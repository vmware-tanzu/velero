/*
Copyright 2017 Heptio Inc.

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
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"sync"
	"time"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"

	api "github.com/heptio/ark/pkg/apis/ark/v1"
	"github.com/heptio/ark/pkg/cloudprovider"
	"github.com/heptio/ark/pkg/generated/clientset/scheme"
	arkv1client "github.com/heptio/ark/pkg/generated/clientset/typed/ark/v1"
	informers "github.com/heptio/ark/pkg/generated/informers/externalversions/ark/v1"
	listers "github.com/heptio/ark/pkg/generated/listers/ark/v1"
	"github.com/heptio/ark/pkg/restore"
	"github.com/heptio/ark/pkg/util/collections"
	kubeutil "github.com/heptio/ark/pkg/util/kube"
)

// nonRestorableResources is a blacklist for the restoration process. Any resources
// included here are explicitly excluded from the restoration process.
var nonRestorableResources = []string{"nodes"}

type restoreController struct {
	restoreClient       arkv1client.RestoresGetter
	backupClient        arkv1client.BackupsGetter
	restorer            restore.Restorer
	backupService       cloudprovider.BackupService
	bucket              string
	pvProviderExists    bool
	backupLister        listers.BackupLister
	backupListerSynced  cache.InformerSynced
	restoreLister       listers.RestoreLister
	restoreListerSynced cache.InformerSynced
	syncHandler         func(restoreName string) error
	queue               workqueue.RateLimitingInterface
	logger              *logrus.Logger
}

func NewRestoreController(
	restoreInformer informers.RestoreInformer,
	restoreClient arkv1client.RestoresGetter,
	backupClient arkv1client.BackupsGetter,
	restorer restore.Restorer,
	backupService cloudprovider.BackupService,
	bucket string,
	backupInformer informers.BackupInformer,
	pvProviderExists bool,
	logger *logrus.Logger,
) Interface {
	c := &restoreController{
		restoreClient:       restoreClient,
		backupClient:        backupClient,
		restorer:            restorer,
		backupService:       backupService,
		bucket:              bucket,
		pvProviderExists:    pvProviderExists,
		backupLister:        backupInformer.Lister(),
		backupListerSynced:  backupInformer.Informer().HasSynced,
		restoreLister:       restoreInformer.Lister(),
		restoreListerSynced: restoreInformer.Informer().HasSynced,
		queue:               workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "restore"),
		logger:              logger,
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
		return errors.Wrap(err, "error splitting queue key")
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
	// don't modify items in the cache
	restore, err = cloneRestore(restore)
	if err != nil {
		return err
	}

	excludedResources := sets.NewString(restore.Spec.ExcludedResources...)
	for _, nonrestorable := range nonRestorableResources {
		if !excludedResources.Has(nonrestorable) {
			restore.Spec.ExcludedResources = append(restore.Spec.ExcludedResources, nonrestorable)
		}
	}

	// validation
	if restore.Status.ValidationErrors = controller.getValidationErrors(restore); len(restore.Status.ValidationErrors) > 0 {
		restore.Status.Phase = api.RestorePhaseFailedValidation
	} else {
		restore.Status.Phase = api.RestorePhaseInProgress
	}

	// update status
	updatedRestore, err := controller.restoreClient.Restores(ns).Update(restore)
	if err != nil {
		return errors.Wrapf(err, "error updating Restore phase to %s", restore.Status.Phase)
	}
	restore = updatedRestore

	if restore.Status.Phase == api.RestorePhaseFailedValidation {
		return nil
	}

	logContext.Debug("Running restore")
	// execution & upload of restore
	restore.Status.Warnings, restore.Status.Errors = controller.runRestore(restore, controller.bucket)

	logContext.Debug("restore completed")
	restore.Status.Phase = api.RestorePhaseCompleted

	logContext.Debug("Updating Restore final status")
	if _, err = controller.restoreClient.Restores(ns).Update(restore); err != nil {
		logContext.WithError(errors.WithStack(err)).Info("Error updating Restore final status")
	}

	return nil
}

func cloneRestore(in interface{}) (*api.Restore, error) {
	clone, err := scheme.Scheme.DeepCopy(in)
	if err != nil {
		return nil, errors.Wrap(err, "error deep-copying Restore")
	}

	out, ok := clone.(*api.Restore)
	if !ok {
		return nil, errors.Errorf("unexpected type: %T", clone)
	}

	return out, nil
}

func (controller *restoreController) getValidationErrors(itm *api.Restore) []string {
	var validationErrors []string

	if itm.Spec.BackupName == "" {
		validationErrors = append(validationErrors, "BackupName must be non-empty and correspond to the name of a backup in object storage.")
	}

	includedResources := sets.NewString(itm.Spec.IncludedResources...)
	for _, nonRestorableResource := range nonRestorableResources {
		if includedResources.Has(nonRestorableResource) {
			validationErrors = append(validationErrors, fmt.Sprintf("%v are a non-restorable resource", nonRestorableResource))
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

func (controller *restoreController) fetchBackup(bucket, name string) (*api.Backup, error) {
	backup, err := controller.backupLister.Backups(api.DefaultNamespace).Get(name)
	if err == nil {
		return backup, nil
	}

	if !apierrors.IsNotFound(err) {
		return nil, errors.WithStack(err)
	}

	logContext := controller.logger.WithField("backupName", name)

	logContext.Debug("Backup not found in backupLister, checking object storage directly")
	backup, err = controller.backupService.GetBackup(bucket, name)
	if err != nil {
		return nil, err
	}

	// ResourceVersion needs to be cleared in order to create the object in the API
	backup.ResourceVersion = ""

	created, createErr := controller.backupClient.Backups(api.DefaultNamespace).Create(backup)
	if createErr != nil {
		logContext.WithError(errors.WithStack(createErr)).Error("Unable to create API object for Backup")
	} else {
		backup = created
	}

	return backup, nil
}

func (controller *restoreController) runRestore(restore *api.Restore, bucket string) (warnings, restoreErrors api.RestoreResult) {
	logContext := controller.logger.WithField("restore", kubeutil.NamespaceAndName(restore))

	backup, err := controller.fetchBackup(bucket, restore.Spec.BackupName)
	if err != nil {
		logContext.WithError(err).WithField("backup", restore.Spec.BackupName).Error("Error getting backup")
		restoreErrors.Ark = append(restoreErrors.Ark, err.Error())
		return
	}

	tmpFile, err := downloadToTempFile(restore.Spec.BackupName, controller.backupService, bucket, controller.logger)
	if err != nil {
		logContext.WithError(err).WithField("backup", restore.Spec.BackupName).Error("Error downloading backup")
		restoreErrors.Ark = append(restoreErrors.Ark, err.Error())
		return
	}

	logFile, err := ioutil.TempFile("", "")
	if err != nil {
		logContext.WithError(errors.WithStack(err)).Error("Error creating log temp file")
		restoreErrors.Ark = append(restoreErrors.Ark, err.Error())
		return
	}

	defer func() {
		if err := tmpFile.Close(); err != nil {
			logContext.WithError(errors.WithStack(err)).WithField("file", tmpFile.Name()).Error("Error closing file")
		}

		if err := os.Remove(tmpFile.Name()); err != nil {
			logContext.WithError(errors.WithStack(err)).WithField("file", tmpFile.Name()).Error("Error removing file")
		}

		if err := logFile.Close(); err != nil {
			logContext.WithError(errors.WithStack(err)).WithField("file", logFile.Name()).Error("Error closing file")
		}

		if err := os.Remove(logFile.Name()); err != nil {
			logContext.WithError(errors.WithStack(err)).WithField("file", logFile.Name()).Error("Error removing file")
		}
	}()

	warnings, restoreErrors = controller.restorer.Restore(restore, backup, tmpFile, logFile)

	// Try to upload the log file. This is best-effort. If we fail, we'll add to the ark errors.

	// Reset the offset to 0 for reading
	if _, err = logFile.Seek(0, 0); err != nil {
		restoreErrors.Ark = append(restoreErrors.Ark, fmt.Sprintf("error resetting log file offset to 0: %v", err))
		return
	}

	if err := controller.backupService.UploadRestoreLog(bucket, restore.Spec.BackupName, restore.Name, logFile); err != nil {
		restoreErrors.Ark = append(restoreErrors.Ark, fmt.Sprintf("error uploading log file to object storage: %v", err))
	}

	return
}

func downloadToTempFile(backupName string, backupService cloudprovider.BackupService, bucket string, logger *logrus.Logger) (*os.File, error) {
	readCloser, err := backupService.DownloadBackup(bucket, backupName)
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
