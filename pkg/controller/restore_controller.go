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
	"sort"
	"sync"
	"time"

	jsonpatch "github.com/evanphx/json-patch"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
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
	"github.com/heptio/ark/pkg/metrics"
	"github.com/heptio/ark/pkg/persistence"
	"github.com/heptio/ark/pkg/plugin"
	"github.com/heptio/ark/pkg/restore"
	"github.com/heptio/ark/pkg/util/boolptr"
	"github.com/heptio/ark/pkg/util/collections"
	kubeutil "github.com/heptio/ark/pkg/util/kube"
	"github.com/heptio/ark/pkg/util/logging"
)

// nonRestorableResources is a blacklist for the restoration process. Any resources
// included here are explicitly excluded from the restoration process.
var nonRestorableResources = []string{
	"nodes",
	"events",
	"events.events.k8s.io",

	// Don't ever restore backups - if appropriate, they'll be synced in from object storage.
	// https://github.com/heptio/ark/issues/622
	"backups.ark.heptio.com",

	// Restores are cluster-specific, and don't have value moving across clusters.
	// https://github.com/heptio/ark/issues/622
	"restores.ark.heptio.com",
}

type restoreController struct {
	namespace                  string
	restoreClient              arkv1client.RestoresGetter
	backupClient               arkv1client.BackupsGetter
	restorer                   restore.Restorer
	pvProviderExists           bool
	backupLister               listers.BackupLister
	backupListerSynced         cache.InformerSynced
	restoreLister              listers.RestoreLister
	restoreListerSynced        cache.InformerSynced
	backupLocationLister       listers.BackupStorageLocationLister
	backupLocationListerSynced cache.InformerSynced
	syncHandler                func(restoreName string) error
	queue                      workqueue.RateLimitingInterface
	logger                     logrus.FieldLogger
	logLevel                   logrus.Level
	defaultBackupLocation      string
	metrics                    *metrics.ServerMetrics

	getBackup            persistence.GetBackupFunc
	downloadBackup       persistence.DownloadBackupFunc
	uploadRestoreLog     persistence.UploadRestoreLogFunc
	uploadRestoreResults persistence.UploadRestoreResultsFunc
	newPluginManager     func(logger logrus.FieldLogger) plugin.Manager
}

func NewRestoreController(
	namespace string,
	restoreInformer informers.RestoreInformer,
	restoreClient arkv1client.RestoresGetter,
	backupClient arkv1client.BackupsGetter,
	restorer restore.Restorer,
	backupInformer informers.BackupInformer,
	backupLocationInformer informers.BackupStorageLocationInformer,
	pvProviderExists bool,
	logger logrus.FieldLogger,
	logLevel logrus.Level,
	newPluginManager func(logrus.FieldLogger) plugin.Manager,
	defaultBackupLocation string,
	metrics *metrics.ServerMetrics,
) Interface {
	c := &restoreController{
		namespace:                  namespace,
		restoreClient:              restoreClient,
		backupClient:               backupClient,
		restorer:                   restorer,
		pvProviderExists:           pvProviderExists,
		backupLister:               backupInformer.Lister(),
		backupListerSynced:         backupInformer.Informer().HasSynced,
		restoreLister:              restoreInformer.Lister(),
		restoreListerSynced:        restoreInformer.Informer().HasSynced,
		backupLocationLister:       backupLocationInformer.Lister(),
		backupLocationListerSynced: backupLocationInformer.Informer().HasSynced,
		queue:                 workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "restore"),
		logger:                logger,
		logLevel:              logLevel,
		defaultBackupLocation: defaultBackupLocation,
		metrics:               metrics,

		// use variables to refer to these functions so they can be
		// replaced with fakes for testing.
		newPluginManager:     newPluginManager,
		getBackup:            persistence.GetBackup,
		downloadBackup:       persistence.DownloadBackup,
		uploadRestoreLog:     persistence.UploadRestoreLog,
		uploadRestoreResults: persistence.UploadRestoreResults,
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
func (c *restoreController) Run(ctx context.Context, numWorkers int) error {
	var wg sync.WaitGroup

	defer func() {
		c.logger.Info("Waiting for workers to finish their work")

		c.queue.ShutDown()

		// We have to wait here in the deferred function instead of at the bottom of the function body
		// because we have to shut down the queue in order for the workers to shut down gracefully, and
		// we want to shut down the queue via defer and not at the end of the body.
		wg.Wait()

		c.logger.Info("All workers have finished")
	}()

	c.logger.Info("Starting RestoreController")
	defer c.logger.Info("Shutting down RestoreController")

	c.logger.Info("Waiting for caches to sync")
	if !cache.WaitForCacheSync(ctx.Done(), c.backupListerSynced, c.restoreListerSynced, c.backupLocationListerSynced) {
		return errors.New("timed out waiting for caches to sync")
	}
	c.logger.Info("Caches are synced")

	wg.Add(numWorkers)
	for i := 0; i < numWorkers; i++ {
		go func() {
			wait.Until(c.runWorker, time.Second, ctx.Done())
			wg.Done()
		}()
	}

	<-ctx.Done()

	return nil
}

func (c *restoreController) runWorker() {
	// continually take items off the queue (waits if it's
	// empty) until we get a shutdown signal from the queue
	for c.processNextWorkItem() {
	}
}

func (c *restoreController) processNextWorkItem() bool {
	key, quit := c.queue.Get()
	if quit {
		return false
	}
	// always call done on this item, since if it fails we'll add
	// it back with rate-limiting below
	defer c.queue.Done(key)

	err := c.syncHandler(key.(string))
	if err == nil {
		// If you had no error, tell the queue to stop tracking history for your key. This will reset
		// things like failure counts for per-item rate limiting.
		c.queue.Forget(key)
		return true
	}

	c.logger.WithError(err).WithField("key", key).Error("Error in syncHandler, re-adding item to queue")
	// we had an error processing the item so add it back
	// into the queue for re-processing with rate-limiting
	c.queue.AddRateLimited(key)

	return true
}

func (c *restoreController) processRestore(key string) error {
	logContext := c.logger.WithField("key", key)

	logContext.Debug("Running processRestore")
	ns, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		logContext.WithError(err).Error("unable to process restore: error splitting queue key")
		// Return nil here so we don't try to process the key any more
		return nil
	}

	logContext.Debug("Getting Restore")
	restore, err := c.restoreLister.Restores(ns).Get(name)
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

	pluginManager := c.newPluginManager(logContext)
	defer pluginManager.CleanupClients()

	actions, err := pluginManager.GetRestoreItemActions()
	if err != nil {
		return errors.Wrap(err, "error initializing restore item actions")
	}

	// validate the restore and fetch the backup
	info := c.validateAndComplete(restore, pluginManager)
	backupScheduleName := restore.Spec.ScheduleName
	// Register attempts after validation so we don't have to fetch the backup multiple times
	c.metrics.RegisterRestoreAttempt(backupScheduleName)

	if len(restore.Status.ValidationErrors) > 0 {
		restore.Status.Phase = api.RestorePhaseFailedValidation
		c.metrics.RegisterRestoreValidationFailed(backupScheduleName)
	} else {
		restore.Status.Phase = api.RestorePhaseInProgress
	}

	// patch to update status and persist to API
	updatedRestore, err := patchRestore(original, restore, c.restoreClient)
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
	restoreWarnings, restoreErrors, restoreFailure := c.runRestore(
		restore,
		actions,
		info,
	)

	restore.Status.Warnings = len(restoreWarnings.Ark) + len(restoreWarnings.Cluster)
	for _, w := range restoreWarnings.Namespaces {
		restore.Status.Warnings += len(w)
	}

	restore.Status.Errors = len(restoreErrors.Ark) + len(restoreErrors.Cluster)
	for _, e := range restoreErrors.Namespaces {
		restore.Status.Errors += len(e)
	}

	if restoreFailure != nil {
		logContext.Debug("restore failed")
		restore.Status.Phase = api.RestorePhaseFailed
		restore.Status.FailureReason = restoreFailure.Error()
		c.metrics.RegisterRestoreFailed(backupScheduleName)
	} else {
		logContext.Debug("restore completed")
		// We got through the restore process without failing validation or restore execution
		restore.Status.Phase = api.RestorePhaseCompleted
		c.metrics.RegisterRestoreSuccess(backupScheduleName)
	}

	logContext.Debug("Updating Restore final status")
	if _, err = patchRestore(original, restore, c.restoreClient); err != nil {
		logContext.WithError(errors.WithStack(err)).Info("Error updating Restore final status")
	}

	return nil
}

type backupInfo struct {
	bucketName  string
	backup      *api.Backup
	objectStore cloudprovider.ObjectStore
}

func (c *restoreController) validateAndComplete(restore *api.Restore, pluginManager plugin.Manager) backupInfo {
	// add non-restorable resources to restore's excluded resources
	excludedResources := sets.NewString(restore.Spec.ExcludedResources...)
	for _, nonrestorable := range nonRestorableResources {
		if !excludedResources.Has(nonrestorable) {
			restore.Spec.ExcludedResources = append(restore.Spec.ExcludedResources, nonrestorable)
		}
	}

	// validate that included resources don't contain any non-restorable resources
	includedResources := sets.NewString(restore.Spec.IncludedResources...)
	for _, nonRestorableResource := range nonRestorableResources {
		if includedResources.Has(nonRestorableResource) {
			restore.Status.ValidationErrors = append(restore.Status.ValidationErrors, fmt.Sprintf("%v are non-restorable resources", nonRestorableResource))
		}
	}

	// validate included/excluded resources
	for _, err := range collections.ValidateIncludesExcludes(restore.Spec.IncludedResources, restore.Spec.ExcludedResources) {
		restore.Status.ValidationErrors = append(restore.Status.ValidationErrors, fmt.Sprintf("Invalid included/excluded resource lists: %v", err))
	}

	// validate included/excluded namespaces
	for _, err := range collections.ValidateIncludesExcludes(restore.Spec.IncludedNamespaces, restore.Spec.ExcludedNamespaces) {
		restore.Status.ValidationErrors = append(restore.Status.ValidationErrors, fmt.Sprintf("Invalid included/excluded namespace lists: %v", err))
	}

	// validate that PV provider exists if we're restoring PVs
	if boolptr.IsSetToTrue(restore.Spec.RestorePVs) && !c.pvProviderExists {
		restore.Status.ValidationErrors = append(restore.Status.ValidationErrors, "Server is not configured for PV snapshot restores")
	}

	// validate that exactly one of BackupName and ScheduleName have been specified
	if !backupXorScheduleProvided(restore) {
		restore.Status.ValidationErrors = append(restore.Status.ValidationErrors, "Either a backup or schedule must be specified as a source for the restore, but not both")
		return backupInfo{}
	}

	// if ScheduleName is specified, fill in BackupName with the most recent successful backup from
	// the schedule
	if restore.Spec.ScheduleName != "" {
		selector := labels.SelectorFromSet(labels.Set(map[string]string{
			"ark-schedule": restore.Spec.ScheduleName,
		}))

		backups, err := c.backupLister.Backups(c.namespace).List(selector)
		if err != nil {
			restore.Status.ValidationErrors = append(restore.Status.ValidationErrors, "Unable to list backups for schedule")
			return backupInfo{}
		}
		if len(backups) == 0 {
			restore.Status.ValidationErrors = append(restore.Status.ValidationErrors, "No backups found for schedule")
		}

		if backup := mostRecentCompletedBackup(backups); backup != nil {
			restore.Spec.BackupName = backup.Name
		} else {
			restore.Status.ValidationErrors = append(restore.Status.ValidationErrors, "No completed backups found for schedule")
			return backupInfo{}
		}
	}

	info, err := c.fetchBackupInfo(restore.Spec.BackupName, pluginManager)
	if err != nil {
		restore.Status.ValidationErrors = append(restore.Status.ValidationErrors, fmt.Sprintf("Error retrieving backup: %v", err))
		return backupInfo{}
	}

	// Fill in the ScheduleName so it's easier to consume for metrics.
	if restore.Spec.ScheduleName == "" {
		restore.Spec.ScheduleName = info.backup.GetLabels()["ark-schedule"]
	}

	return info
}

// backupXorScheduleProvided returns true if exactly one of BackupName and
// ScheduleName are non-empty for the restore, or false otherwise.
func backupXorScheduleProvided(restore *api.Restore) bool {
	if restore.Spec.BackupName != "" && restore.Spec.ScheduleName != "" {
		return false
	}

	if restore.Spec.BackupName == "" && restore.Spec.ScheduleName == "" {
		return false
	}

	return true
}

// mostRecentCompletedBackup returns the most recent backup that's
// completed from a list of backups.
func mostRecentCompletedBackup(backups []*api.Backup) *api.Backup {
	sort.Slice(backups, func(i, j int) bool {
		// Use .After() because we want descending sort.
		return backups[i].Status.StartTimestamp.After(backups[j].Status.StartTimestamp.Time)
	})

	for _, backup := range backups {
		if backup.Status.Phase == api.BackupPhaseCompleted {
			return backup
		}
	}

	return nil
}

// fetchBackupInfo checks the backup lister for a backup that matches the given name. If it doesn't
// find it, it tries to retrieve it from one of the backup storage locations.
func (c *restoreController) fetchBackupInfo(backupName string, pluginManager plugin.Manager) (backupInfo, error) {
	var info backupInfo
	var err error
	info.backup, err = c.backupLister.Backups(c.namespace).Get(backupName)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return backupInfo{}, errors.WithStack(err)
		}

		logContext := c.logger.WithField("backupName", backupName)
		logContext.Debug("Backup not found in backupLister, checking each backup location directly, starting with default...")
		return c.fetchFromBackupStorage(backupName, pluginManager)
	}

	location, err := c.backupLocationLister.BackupStorageLocations(c.namespace).Get(info.backup.Spec.StorageLocation)
	if err != nil {
		return backupInfo{}, errors.WithStack(err)
	}

	info.objectStore, err = getObjectStoreForLocation(location, pluginManager)
	if err != nil {
		return backupInfo{}, errors.Wrap(err, "error initializing object store")
	}
	info.bucketName = location.Spec.ObjectStorage.Bucket

	return info, nil
}

// fetchFromBackupStorage checks each backup storage location, starting with the default,
// looking for a backup that matches the given backup name.
func (c *restoreController) fetchFromBackupStorage(backupName string, pluginManager plugin.Manager) (backupInfo, error) {
	locations, err := c.backupLocationLister.BackupStorageLocations(c.namespace).List(labels.Everything())
	if err != nil {
		return backupInfo{}, errors.WithStack(err)
	}

	orderedLocations := orderedBackupLocations(locations, c.defaultBackupLocation)

	logContext := c.logger.WithField("backupName", backupName)
	for _, location := range orderedLocations {
		info, err := c.backupInfoForLocation(location, backupName, pluginManager)
		if err != nil {
			logContext.WithField("locationName", location.Name).WithError(err).Error("Unable to fetch backup from object storage location")
			continue
		}
		return info, nil
	}

	return backupInfo{}, errors.New("not able to fetch from backup storage")
}

// orderedBackupLocations returns a new slice with the default backup location first (if it exists),
// followed by the rest of the locations in no particular order.
func orderedBackupLocations(locations []*api.BackupStorageLocation, defaultLocationName string) []*api.BackupStorageLocation {
	var result []*api.BackupStorageLocation

	for i := range locations {
		if locations[i].Name == defaultLocationName {
			// put the default location first
			result = append(result, locations[i])
			// append everything before the default
			result = append(result, locations[:i]...)
			// append everything after the default
			result = append(result, locations[i+1:]...)

			return result
		}
	}

	return locations
}

func (c *restoreController) backupInfoForLocation(location *api.BackupStorageLocation, backupName string, pluginManager plugin.Manager) (backupInfo, error) {
	objectStore, err := getObjectStoreForLocation(location, pluginManager)
	if err != nil {
		return backupInfo{}, err
	}

	backup, err := c.getBackup(objectStore, location.Spec.ObjectStorage.Bucket, backupName)
	if err != nil {
		return backupInfo{}, err
	}

	// ResourceVersion needs to be cleared in order to create the object in the API
	backup.ResourceVersion = ""
	// Clear out the namespace, in case the backup was made in a different cluster, with a different namespace
	backup.Namespace = ""

	backupCreated, err := c.backupClient.Backups(c.namespace).Create(backup)
	if err != nil {
		return backupInfo{}, errors.WithStack(err)
	}

	return backupInfo{
		bucketName:  location.Spec.ObjectStorage.Bucket,
		backup:      backupCreated,
		objectStore: objectStore,
	}, nil
}

func (c *restoreController) runRestore(
	restore *api.Restore,
	actions []restore.ItemAction,
	info backupInfo,
) (restoreWarnings, restoreErrors api.RestoreResult, restoreFailure error) {
	logFile, err := ioutil.TempFile("", "")
	if err != nil {
		c.logger.
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
	defer closeAndRemoveFile(logFile, c.logger)

	// Log the backup to both a backup log file and to stdout. This will help see what happened if the upload of the
	// backup log failed for whatever reason.
	logger := logging.DefaultLogger(c.logLevel)
	logger.Out = io.MultiWriter(os.Stdout, gzippedLogFile)
	logContext := logger.WithFields(
		logrus.Fields{
			"restore": kubeutil.NamespaceAndName(restore),
			"backup":  restore.Spec.BackupName,
		})

	backupFile, err := downloadToTempFile(info.objectStore, info.bucketName, restore.Spec.BackupName, c.downloadBackup, c.logger)
	if err != nil {
		logContext.WithError(err).Error("Error downloading backup")
		restoreErrors.Ark = append(restoreErrors.Ark, err.Error())
		restoreFailure = err
		return
	}
	defer closeAndRemoveFile(backupFile, c.logger)

	resultsFile, err := ioutil.TempFile("", "")
	if err != nil {
		logContext.WithError(errors.WithStack(err)).Error("Error creating results temp file")
		restoreErrors.Ark = append(restoreErrors.Ark, err.Error())
		restoreFailure = err
		return
	}
	defer closeAndRemoveFile(resultsFile, c.logger)

	// Any return statement above this line means a total restore failure
	// Some failures after this line *may* be a total restore failure
	logContext.Info("starting restore")
	restoreWarnings, restoreErrors = c.restorer.Restore(logContext, restore, info.backup, backupFile, actions)
	logContext.Info("restore completed")

	// Try to upload the log file. This is best-effort. If we fail, we'll add to the ark errors.
	if err := gzippedLogFile.Close(); err != nil {
		c.logger.WithError(err).Error("error closing gzippedLogFile")
	}
	// Reset the offset to 0 for reading
	if _, err = logFile.Seek(0, 0); err != nil {
		restoreErrors.Ark = append(restoreErrors.Ark, fmt.Sprintf("error resetting log file offset to 0: %v", err))
		return
	}

	if err := c.uploadRestoreLog(info.objectStore, info.bucketName, restore.Spec.BackupName, restore.Name, logFile); err != nil {
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
	if err := c.uploadRestoreResults(info.objectStore, info.bucketName, restore.Spec.BackupName, restore.Name, resultsFile); err != nil {
		logContext.WithError(errors.WithStack(err)).Error("Error uploading results files to object storage")
	}

	return
}

func downloadToTempFile(
	objectStore cloudprovider.ObjectStore,
	bucket, backupName string,
	downloadBackup persistence.DownloadBackupFunc,
	logger logrus.FieldLogger,
) (*os.File, error) {
	readCloser, err := downloadBackup(objectStore, bucket, backupName)
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
