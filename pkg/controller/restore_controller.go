/*
Copyright 2017 the Heptio Ark contributors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controller

import (
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"sync"
	"time"

	jsonpatch "github.com/evanphx/json-patch"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"

	api "github.com/heptio/ark/pkg/apis/ark/v1"
	"github.com/heptio/ark/pkg/cloudprovider"
	arkv1client "github.com/heptio/ark/pkg/generated/clientset/versioned/typed/ark/v1"
	informers "github.com/heptio/ark/pkg/generated/informers/externalversions/ark/v1"
	listers "github.com/heptio/ark/pkg/generated/listers/ark/v1"
	"github.com/heptio/ark/pkg/plugin"
	"github.com/heptio/ark/pkg/restore"
	"github.com/heptio/ark/pkg/util/collections"
	kubeutil "github.com/heptio/ark/pkg/util/kube"
	"github.com/heptio/ark/pkg/util/logging"
)

// nonRestorableResources is a blacklist for the restoration process. Any resources
// included here are explicitly excluded from the restoration process.
var nonRestorableResources = []string{"nodes", "events", "events.events.k8s.io"}

type backupDownloader interface {
	downloadBackup(backupName string) (io.ReadCloser, error)
}

type objectStoreBackupDownloader struct {
	objectStore cloudprovider.ObjectStore
	bucket      string
}

func newObjectStoreBackupDownloader(objectStore cloudprovider.ObjectStore, bucket string) backupDownloader {
	return &objectStoreBackupDownloader{
		objectStore: objectStore,
		bucket:      bucket,
	}
}

func (o *objectStoreBackupDownloader) downloadBackup(backupName string) (io.ReadCloser, error) {
	return cloudprovider.DownloadBackup(o.objectStore, o.bucket, backupName)
}

type restoreUploader interface {
	uploadRestoreLog(backupName, restoreName string, logFile io.Reader) error
	uploadRestoreResults(backupName, restoreName string, resultsFile io.Reader) error
}

type objectStoreRestoreUploader struct {
	objectStore cloudprovider.ObjectStore
	bucket      string
}

func newObjectStoreRestoreUploader(objectStore cloudprovider.ObjectStore, bucket string) restoreUploader {
	return &objectStoreRestoreUploader{
		objectStore: objectStore,
		bucket:      bucket,
	}
}

func (o *objectStoreRestoreUploader) uploadRestoreLog(backupName, restoreName string, logFile io.Reader) error {
	return cloudprovider.UploadRestoreLog(o.objectStore, o.bucket, backupName, restoreName, logFile)
}

func (o *objectStoreRestoreUploader) uploadRestoreResults(backupName, restoreName string, resultsFile io.Reader) error {
	return cloudprovider.UploadRestoreResults(o.objectStore, o.bucket, backupName, restoreName, resultsFile)
}

type restoreController struct {
	namespace           string
	restoreClient       arkv1client.RestoresGetter
	backupClient        arkv1client.BackupsGetter
	restorer            restore.Restorer
	objectStoreConfig   api.CloudProviderConfig
	bucket              string
	pvProviderExists    bool
	backupLister        listers.BackupLister
	backupListerSynced  cache.InformerSynced
	restoreLister       listers.RestoreLister
	restoreListerSynced cache.InformerSynced
	syncHandler         func(restoreName string) error
	queue               workqueue.RateLimitingInterface
	logger              logrus.FieldLogger
	logLevel            logrus.Level
	pluginRegistry      plugin.Registry

	newPluginManager    func(logger logrus.FieldLogger, logLevel logrus.Level, pluginRegistry plugin.Registry) plugin.Manager
	newBackupGetter     func(logger logrus.FieldLogger, objectStore cloudprovider.ObjectStore) cloudprovider.XXXBackupGetter
	newBackupDownloader func(objectStore cloudprovider.ObjectStore, bucket string) backupDownloader
	newRestoreUploader  func(objectStore cloudprovider.ObjectStore, bucket string) restoreUploader
}

func NewRestoreController(
	namespace string,
	restoreInformer informers.RestoreInformer,
	restoreClient arkv1client.RestoresGetter,
	backupClient arkv1client.BackupsGetter,
	restorer restore.Restorer,
	objectStoreConfig api.CloudProviderConfig,
	bucket string,
	backupInformer informers.BackupInformer,
	pvProviderExists bool,
	logger logrus.FieldLogger,
	logLevel logrus.Level,
	pluginRegistry plugin.Registry,
) Interface {
	c := &restoreController{
		namespace:           namespace,
		restoreClient:       restoreClient,
		backupClient:        backupClient,
		restorer:            restorer,
		objectStoreConfig:   objectStoreConfig,
		bucket:              bucket,
		pvProviderExists:    pvProviderExists,
		backupLister:        backupInformer.Lister(),
		backupListerSynced:  backupInformer.Informer().HasSynced,
		restoreLister:       restoreInformer.Lister(),
		restoreListerSynced: restoreInformer.Informer().HasSynced,
		queue:               workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "restore"),
		logger:              logger,
		logLevel:            logLevel,
		pluginRegistry:      pluginRegistry,

		newPluginManager: func(logger logrus.FieldLogger, logLevel logrus.Level, pluginRegistry plugin.Registry) plugin.Manager {
			return plugin.NewManager(logger, logLevel, pluginRegistry)
		},
		newBackupGetter: func(logger logrus.FieldLogger, objectStore cloudprovider.ObjectStore) cloudprovider.XXXBackupGetter {
			return cloudprovider.NewLiveXXXBackupGetter(logger, objectStore)
		},
		newBackupDownloader: func(objectStore cloudprovider.ObjectStore, bucket string) backupDownloader {
			return newObjectStoreBackupDownloader(objectStore, bucket)
		},
		newRestoreUploader: func(objectStore cloudprovider.ObjectStore, bucket string) restoreUploader {
			return newObjectStoreRestoreUploader(objectStore, bucket)
		},
	}

	c.syncHandler = c.processRestore

	restoreInformer.Informer().AddEventHandler(
		cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {
				restore := obj.(*api.Restore)

				switch restore.Status.Phase {
				case "", api.RestorePhaseNew:
					// only process new restores
				default:
					c.logger.WithFields(logrus.Fields{
						"restore": kubeutil.NamespaceAndName(restore),
						"phase":   restore.Status.Phase,
					}).Debug("Restore is not new, skipping")
					return
				}

				key, err := cache.MetaNamespaceKeyFunc(restore)
				if err != nil {
					c.logger.WithError(errors.WithStack(err)).WithField("restore", restore).Error("Error creating queue key, item not added to queue")
					return
				}
				c.queue.Add(key)
			},
		},
	)

	return c
}

// Run is a blocking function that runs the specified number of worker goroutines
// to process items in the work queue. It will return when it receives on the
// ctx.Done() channel.
func (controller *restoreController) Run(ctx context.Context, numWorkers int) error {
	var wg sync.WaitGroup

	defer func() {
		controller.logger.Info("Waiting for workers to finish their work")

		controller.queue.ShutDown()

		// We have to wait here in the deferred function instead of at the bottom of the function body
		// because we have to shut down the queue in order for the workers to shut down gracefully, and
		// we want to shut down the queue via defer and not at the end of the body.
		wg.Wait()

		controller.logger.Info("All workers have finished")
	}()

	controller.logger.Info("Starting RestoreController")
	defer controller.logger.Info("Shutting down RestoreController")

	controller.logger.Info("Waiting for caches to sync")
	if !cache.WaitForCacheSync(ctx.Done(), controller.backupListerSynced, controller.restoreListerSynced) {
		return errors.New("timed out waiting for caches to sync")
	}
	controller.logger.Info("Caches are synced")

	wg.Add(numWorkers)
	for i := 0; i < numWorkers; i++ {
		go func() {
			wait.Until(controller.runWorker, time.Second, ctx.Done())
			wg.Done()
		}()
	}

	<-ctx.Done()

	return nil
}

func (controller *restoreController) runWorker() {
	// continually take items off the queue (waits if it's
	// empty) until we get a shutdown signal from the queue
	for controller.processNextWorkItem() {
	}
}

func (controller *restoreController) processNextWorkItem() bool {
	key, quit := controller.queue.Get()
	if quit {
		return false
	}
	// always call done on this item, since if it fails we'll add
	// it back with rate-limiting below
	defer controller.queue.Done(key)

	err := controller.syncHandler(key.(string))
	if err == nil {
		// If you had no error, tell the queue to stop tracking history for your key. This will reset
		// things like failure counts for per-item rate limiting.
		controller.queue.Forget(key)
		return true
	}

	controller.logger.WithError(err).WithField("key", key).Error("Error in syncHandler, re-adding item to queue")
	// we had an error processing the item so add it back
	// into the queue for re-processing with rate-limiting
	controller.queue.AddRateLimited(key)

	return true
}

func (controller *restoreController) processRestore(key string) error {
	logContext := controller.logger.WithField("key", key)

	logContext.Debug("Running processRestore")
	ns, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		logContext.WithError(err).Error("unable to process restore: error splitting queue key")
		// Return nil here so we don't try to process the key any more
		return nil
	}

	logContext.Debug("Getting Restore")
	restore, err := controller.restoreLister.Restores(ns).Get(name)
	if err != nil {
		return errors.Wrap(err, "error getting Restore")
	}

	// TODO I think this is now unnecessary. We only initially place
	// item with Phase = ("" | New) into the queue. Items will only get
	// re-queued if syncHandler returns an error, which will only
	// happen if there's an error updating Phase from its initial
	// state to something else. So any time it's re-queued it will
	// still have its initial state, which we've already confirmed
	// is ("" | New)
	switch restore.Status.Phase {
	case "", api.RestorePhaseNew:
		// only process new restores
	default:
		return nil
	}

	logContext.Debug("Cloning Restore")
	// store ref to original for creating patch
	original := restore
	// don't modify items in the cache
	restore = restore.DeepCopy()

	excludedResources := sets.NewString(restore.Spec.ExcludedResources...)
	for _, nonrestorable := range nonRestorableResources {
		if !excludedResources.Has(nonrestorable) {
			restore.Spec.ExcludedResources = append(restore.Spec.ExcludedResources, nonrestorable)
		}
	}

	pluginManager := controller.newPluginManager(logContext, logContext.Level, controller.pluginRegistry)
	defer pluginManager.CleanupClients()

	objectStore, err := getObjectStore(controller.objectStoreConfig, pluginManager)
	if err != nil {
		return errors.Wrap(err, "error initializing object store")
	}
	backupGetter := controller.newBackupGetter(logContext, objectStore)
	backupDownloader := controller.newBackupDownloader(objectStore, controller.bucket)
	restoreUploader := controller.newRestoreUploader(objectStore, controller.bucket)

	actions, err := pluginManager.GetRestoreItemActions()
	if err != nil {
		return errors.Wrap(err, "error initializing restore item actions")
	}

	// validation
	if restore.Status.ValidationErrors = controller.getValidationErrors(backupGetter, restore); len(restore.Status.ValidationErrors) > 0 {
		restore.Status.Phase = api.RestorePhaseFailedValidation
	} else {
		restore.Status.Phase = api.RestorePhaseInProgress
	}

	// update status
	updatedRestore, err := patchRestore(original, restore, controller.restoreClient)
	if err != nil {
		return errors.Wrapf(err, "error updating Restore phase to %s", restore.Status.Phase)
	}
	// store ref to just-updated item for creating patch
	original = updatedRestore
	restore = updatedRestore.DeepCopy()

	if restore.Status.Phase == api.RestorePhaseFailedValidation {
		return nil
	}

	logContext.Debug("Running restore")
	// execution & upload of restore
	restoreWarnings, restoreErrors := controller.runRestore(
		restore,
		actions,
		backupGetter,
		backupDownloader,
		restoreUploader,
	)

	restore.Status.Warnings = len(restoreWarnings.Ark) + len(restoreWarnings.Cluster)
	for _, w := range restoreWarnings.Namespaces {
		restore.Status.Warnings += len(w)
	}

	restore.Status.Errors = len(restoreErrors.Ark) + len(restoreErrors.Cluster)
	for _, e := range restoreErrors.Namespaces {
		restore.Status.Errors += len(e)
	}

	logContext.Debug("restore completed")
	restore.Status.Phase = api.RestorePhaseCompleted

	logContext.Debug("Updating Restore final status")
	if _, err = patchRestore(original, restore, controller.restoreClient); err != nil {
		logContext.WithError(errors.WithStack(err)).Info("Error updating Restore final status")
	}

	return nil
}

func (controller *restoreController) getValidationErrors(backupGetter cloudprovider.XXXBackupGetter, itm *api.Restore) []string {
	var validationErrors []string

	if itm.Spec.BackupName == "" {
		validationErrors = append(validationErrors, "BackupName must be non-empty and correspond to the name of a backup in object storage.")
	} else if _, err := controller.fetchBackup(backupGetter, itm.Spec.BackupName); err != nil {
		validationErrors = append(validationErrors, fmt.Sprintf("Error retrieving backup: %v", err))
	}

	includedResources := sets.NewString(itm.Spec.IncludedResources...)
	for _, nonRestorableResource := range nonRestorableResources {
		if includedResources.Has(nonRestorableResource) {
			validationErrors = append(validationErrors, fmt.Sprintf("%v are non-restorable resources", nonRestorableResource))
		}
	}

	for _, err := range collections.ValidateIncludesExcludes(itm.Spec.IncludedNamespaces, itm.Spec.ExcludedNamespaces) {
		validationErrors = append(validationErrors, fmt.Sprintf("Invalid included/excluded namespace lists: %v", err))
	}

	for _, err := range collections.ValidateIncludesExcludes(itm.Spec.IncludedResources, itm.Spec.ExcludedResources) {
		validationErrors = append(validationErrors, fmt.Sprintf("Invalid included/excluded resource lists: %v", err))
	}

	if !controller.pvProviderExists && itm.Spec.RestorePVs != nil && *itm.Spec.RestorePVs {
		validationErrors = append(validationErrors, "Server is not configured for PV snapshot restores")
	}

	return validationErrors
}

func (controller *restoreController) fetchBackup(backupGetter cloudprovider.XXXBackupGetter, name string) (*api.Backup, error) {
	backup, err := controller.backupLister.Backups(controller.namespace).Get(name)
	if err == nil {
		return backup, nil
	}

	if !apierrors.IsNotFound(err) {
		return nil, errors.WithStack(err)
	}

	logContext := controller.logger.WithField("backupName", name)

	logContext.Debug("Backup not found in backupLister, checking object storage directly")
	backup, err = backupGetter.GetBackup(controller.bucket, name)
	if err != nil {
		return nil, err
	}

	// ResourceVersion needs to be cleared in order to create the object in the API
	backup.ResourceVersion = ""
	// Clear out the namespace too, just in case
	backup.Namespace = ""

	created, createErr := controller.backupClient.Backups(controller.namespace).Create(backup)
	if createErr != nil {
		logContext.WithError(errors.WithStack(createErr)).Error("Unable to create API object for Backup")
	} else {
		backup = created
	}

	return backup, nil
}

func (controller *restoreController) runRestore(
	restore *api.Restore,
	actions []restore.ItemAction,
	backupGetter cloudprovider.XXXBackupGetter,
	backupDownloader backupDownloader,
	restoreUploader restoreUploader,
) (restoreWarnings, restoreErrors api.RestoreResult) {
	logFile, err := ioutil.TempFile("", "")
	if err != nil {
		controller.logger.
			WithFields(
				logrus.Fields{
					"restore": kubeutil.NamespaceAndName(restore),
					"backup":  restore.Spec.BackupName,
				},
			).
			WithError(errors.WithStack(err)).
			Error("Error creating log temp file")
		restoreErrors.Ark = append(restoreErrors.Ark, err.Error())
		return
	}
	gzippedLogFile := gzip.NewWriter(logFile)
	// Assuming we successfully uploaded the log file, this will have already been closed below. It is safe to call
	// close multiple times. If we get an error closing this, there's not really anything we can do about it.
	defer gzippedLogFile.Close()
	defer closeAndRemoveFile(logFile, controller.logger)

	// Log the backup to both a backup log file and to stdout. This will help see what happened if the upload of the
	// backup log failed for whatever reason.
	logger := logging.DefaultLogger(controller.logLevel)
	logger.Out = io.MultiWriter(os.Stdout, gzippedLogFile)
	logContext := logger.WithFields(
		logrus.Fields{
			"restore": kubeutil.NamespaceAndName(restore),
			"backup":  restore.Spec.BackupName,
		})

	backup, err := controller.fetchBackup(backupGetter, restore.Spec.BackupName)
	if err != nil {
		logContext.WithError(err).Error("Error getting backup")
		restoreErrors.Ark = append(restoreErrors.Ark, err.Error())
		return
	}

	backupFile, err := downloadToTempFile(restore.Spec.BackupName, backupDownloader, controller.logger)
	if err != nil {
		logContext.WithError(err).Error("Error downloading backup")
		restoreErrors.Ark = append(restoreErrors.Ark, err.Error())
		return
	}
	defer closeAndRemoveFile(backupFile, controller.logger)

	resultsFile, err := ioutil.TempFile("", "")
	if err != nil {
		logContext.WithError(errors.WithStack(err)).Error("Error creating results temp file")
		restoreErrors.Ark = append(restoreErrors.Ark, err.Error())
		return
	}
	defer closeAndRemoveFile(resultsFile, controller.logger)

	logContext.Info("starting restore")
	restoreWarnings, restoreErrors = controller.restorer.Restore(logContext, restore, backup, backupFile, actions)
	logContext.Info("restore completed")

	// Try to upload the log file. This is best-effort. If we fail, we'll add to the ark errors.
	if err := gzippedLogFile.Close(); err != nil {
		controller.logger.WithError(err).Error("error closing gzippedLogFile")
	}
	// Reset the offset to 0 for reading
	if _, err = logFile.Seek(0, 0); err != nil {
		restoreErrors.Ark = append(restoreErrors.Ark, fmt.Sprintf("error resetting log file offset to 0: %v", err))
		return
	}

	if err := restoreUploader.uploadRestoreLog(restore.Spec.BackupName, restore.Name, logFile); err != nil {
		restoreErrors.Ark = append(restoreErrors.Ark, fmt.Sprintf("error uploading log file to object storage: %v", err))
	}

	m := map[string]api.RestoreResult{
		"warnings": restoreWarnings,
		"errors":   restoreErrors,
	}

	gzippedResultsFile := gzip.NewWriter(resultsFile)

	if err := json.NewEncoder(gzippedResultsFile).Encode(m); err != nil {
		logContext.WithError(errors.WithStack(err)).Error("Error encoding restore results")
		return
	}
	gzippedResultsFile.Close()

	if _, err = resultsFile.Seek(0, 0); err != nil {
		logContext.WithError(errors.WithStack(err)).Error("Error resetting results file offset to 0")
		return
	}
	if err := restoreUploader.uploadRestoreResults(restore.Spec.BackupName, restore.Name, resultsFile); err != nil {
		logContext.WithError(errors.WithStack(err)).Error("Error uploading results files to object storage")
	}

	return
}

func downloadToTempFile(backupName string, backupDownloader backupDownloader, logger logrus.FieldLogger) (*os.File, error) {
	readCloser, err := backupDownloader.downloadBackup(backupName)
	if err != nil {
		return nil, err
	}
	defer readCloser.Close()

	file, err := ioutil.TempFile("", backupName)
	if err != nil {
		return nil, errors.Wrap(err, "error creating Backup temp file")
	}

	n, err := io.Copy(file, readCloser)
	if err != nil {
		return nil, errors.Wrap(err, "error copying Backup to temp file")
	}

	logContext := logger.WithField("backup", backupName)

	logContext.WithFields(logrus.Fields{
		"fileName": file.Name(),
		"bytes":    n,
	}).Debug("Copied Backup to file")

	if _, err := file.Seek(0, 0); err != nil {
		return nil, errors.Wrap(err, "error resetting Backup file offset")
	}

	return file, nil
}

func patchRestore(original, updated *api.Restore, client arkv1client.RestoresGetter) (*api.Restore, error) {
	origBytes, err := json.Marshal(original)
	if err != nil {
		return nil, errors.Wrap(err, "error marshalling original restore")
	}

	updatedBytes, err := json.Marshal(updated)
	if err != nil {
		return nil, errors.Wrap(err, "error marshalling updated restore")
	}

	patchBytes, err := jsonpatch.CreateMergePatch(origBytes, updatedBytes)
	if err != nil {
		return nil, errors.Wrap(err, "error creating json merge patch for restore")
	}

	res, err := client.Restores(original.Namespace).Patch(original.Name, types.MergePatchType, patchBytes)
	if err != nil {
		return nil, errors.Wrap(err, "error patching restore")
	}

	return res, nil
}
