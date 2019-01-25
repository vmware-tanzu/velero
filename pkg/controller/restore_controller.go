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
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"sort"

	jsonpatch "github.com/evanphx/json-patch"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/tools/cache"

	api "github.com/heptio/velero/pkg/apis/velero/v1"
	velerov1api "github.com/heptio/velero/pkg/apis/velero/v1"
	velerov1client "github.com/heptio/velero/pkg/generated/clientset/versioned/typed/velero/v1"
	informers "github.com/heptio/velero/pkg/generated/informers/externalversions/velero/v1"
	listers "github.com/heptio/velero/pkg/generated/listers/velero/v1"
	"github.com/heptio/velero/pkg/metrics"
	"github.com/heptio/velero/pkg/persistence"
	"github.com/heptio/velero/pkg/plugin"
	"github.com/heptio/velero/pkg/restore"
	"github.com/heptio/velero/pkg/util/collections"
	kubeutil "github.com/heptio/velero/pkg/util/kube"
	"github.com/heptio/velero/pkg/util/logging"
)

// nonRestorableResources is a blacklist for the restoration process. Any resources
// included here are explicitly excluded from the restoration process.
var nonRestorableResources = []string{
	"nodes",
	"events",
	"events.events.k8s.io",

	// Don't ever restore backups - if appropriate, they'll be synced in from object storage.
	// https://github.com/heptio/velero/issues/622
	"backups.ark.heptio.com",
	"backups.velero.io",

	// Restores are cluster-specific, and don't have value moving across clusters.
	// https://github.com/heptio/velero/issues/622
	"restores.ark.heptio.com",
	"restores.velero.io",
}

type restoreController struct {
	*genericController

	namespace              string
	restoreClient          velerov1client.RestoresGetter
	backupClient           velerov1client.BackupsGetter
	restorer               restore.Restorer
	backupLister           listers.BackupLister
	restoreLister          listers.RestoreLister
	backupLocationLister   listers.BackupStorageLocationLister
	snapshotLocationLister listers.VolumeSnapshotLocationLister
	restoreLogLevel        logrus.Level
	defaultBackupLocation  string
	metrics                *metrics.ServerMetrics

	newPluginManager func(logger logrus.FieldLogger) plugin.Manager
	newBackupStore   func(*api.BackupStorageLocation, persistence.ObjectStoreGetter, logrus.FieldLogger) (persistence.BackupStore, error)
}

type restoreResult struct {
	warnings, errors api.RestoreResult
}

func NewRestoreController(
	namespace string,
	restoreInformer informers.RestoreInformer,
	restoreClient velerov1client.RestoresGetter,
	backupClient velerov1client.BackupsGetter,
	restorer restore.Restorer,
	backupInformer informers.BackupInformer,
	backupLocationInformer informers.BackupStorageLocationInformer,
	snapshotLocationInformer informers.VolumeSnapshotLocationInformer,
	logger logrus.FieldLogger,
	restoreLogLevel logrus.Level,
	newPluginManager func(logrus.FieldLogger) plugin.Manager,
	defaultBackupLocation string,
	metrics *metrics.ServerMetrics,
) Interface {
	c := &restoreController{
		genericController:      newGenericController("restore", logger),
		namespace:              namespace,
		restoreClient:          restoreClient,
		backupClient:           backupClient,
		restorer:               restorer,
		backupLister:           backupInformer.Lister(),
		restoreLister:          restoreInformer.Lister(),
		backupLocationLister:   backupLocationInformer.Lister(),
		snapshotLocationLister: snapshotLocationInformer.Lister(),
		restoreLogLevel:        restoreLogLevel,
		defaultBackupLocation:  defaultBackupLocation,
		metrics:                metrics,

		// use variables to refer to these functions so they can be
		// replaced with fakes for testing.
		newPluginManager: newPluginManager,
		newBackupStore:   persistence.NewObjectBackupStore,
	}

	c.syncHandler = c.processRestore
	c.cacheSyncWaiters = append(c.cacheSyncWaiters,
		backupInformer.Informer().HasSynced,
		restoreInformer.Informer().HasSynced,
		backupLocationInformer.Informer().HasSynced,
		snapshotLocationInformer.Informer().HasSynced,
	)

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

func (c *restoreController) processRestore(key string) error {
	log := c.logger.WithField("key", key)

	log.Debug("Running processRestore")
	ns, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		log.WithError(err).Error("unable to process restore: error splitting queue key")
		// Return nil here so we don't try to process the key any more
		return nil
	}

	log.Debug("Getting Restore")
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

	log.Debug("Cloning Restore")
	// store ref to original for creating patch
	original := restore
	// don't modify items in the cache
	restore = restore.DeepCopy()

	pluginManager := c.newPluginManager(log)
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

	log.Debug("Running restore")

	// execution & upload of restore
	restoreRes, restoreFailure := c.runRestore(
		restore,
		actions,
		info,
		pluginManager,
	)

	//TODO(1.0): Remove warnings.Ark
	restore.Status.Warnings = len(restoreRes.warnings.Velero) + len(restoreRes.warnings.Cluster) + len(restoreRes.warnings.Ark)
	for _, w := range restoreRes.warnings.Namespaces {
		restore.Status.Warnings += len(w)
	}

	//TODO (1.0): Remove errors.Ark
	restore.Status.Errors = len(restoreRes.errors.Velero) + len(restoreRes.errors.Cluster) + len(restoreRes.errors.Ark)
	for _, e := range restoreRes.errors.Namespaces {
		restore.Status.Errors += len(e)
	}

	if restoreFailure != nil {
		log.Debug("restore failed")
		restore.Status.Phase = api.RestorePhaseFailed
		restore.Status.FailureReason = restoreFailure.Error()
		c.metrics.RegisterRestoreFailed(backupScheduleName)
	} else {
		log.Debug("restore completed")
		// We got through the restore process without failing validation or restore execution
		restore.Status.Phase = api.RestorePhaseCompleted
		c.metrics.RegisterRestoreSuccess(backupScheduleName)
	}

	log.Debug("Updating Restore final status")
	if _, err = patchRestore(original, restore, c.restoreClient); err != nil {
		log.WithError(errors.WithStack(err)).Info("Error updating Restore final status")
	}

	return nil
}

type backupInfo struct {
	backup      *api.Backup
	backupStore persistence.BackupStore
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

	// validate that exactly one of BackupName and ScheduleName have been specified
	if !backupXorScheduleProvided(restore) {
		restore.Status.ValidationErrors = append(restore.Status.ValidationErrors, "Either a backup or schedule must be specified as a source for the restore, but not both")
		return backupInfo{}
	}

	// if ScheduleName is specified, fill in BackupName with the most recent successful backup from
	// the schedule
	if restore.Spec.ScheduleName != "" {
		selector := labels.SelectorFromSet(labels.Set(map[string]string{
			velerov1api.ScheduleNameLabel: restore.Spec.ScheduleName,
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

	// Ensure that we have either .status.volumeBackups (for pre-v0.10 backups) OR a
	// volumesnapshots.json.gz file in obj storage (for v0.10+ backups), but not both.
	// If we have .status.volumeBackups, ensure that there's only one volume snapshot
	// location configured.
	if info.backup.Status.VolumeBackups != nil {
		snapshots, err := info.backupStore.GetBackupVolumeSnapshots(info.backup.Name)
		if err != nil {
			restore.Status.ValidationErrors = append(restore.Status.ValidationErrors, errors.Wrap(err, "Error checking for volumesnapshots file").Error())
		} else if len(snapshots) > 0 {
			restore.Status.ValidationErrors = append(restore.Status.ValidationErrors, "Backup must not have both .status.volumeBackups and a volumesnapshots.json.gz file in object storage")
		} else {
			locations, err := c.snapshotLocationLister.VolumeSnapshotLocations(restore.Namespace).List(labels.Everything())
			if err != nil {
				restore.Status.ValidationErrors = append(restore.Status.ValidationErrors, errors.Wrap(err, "Error listing volume snapshot locations").Error())
			} else if len(locations) > 1 {
				restore.Status.ValidationErrors = append(restore.Status.ValidationErrors, "Cannot restore backup with .status.volumeBackups when more than one volume snapshot location exists")
			}
		}
	}

	// Fill in the ScheduleName so it's easier to consume for metrics.
	if restore.Spec.ScheduleName == "" {
		restore.Spec.ScheduleName = info.backup.GetLabels()[velerov1api.ScheduleNameLabel]
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
// find it, it returns an error.
func (c *restoreController) fetchBackupInfo(backupName string, pluginManager plugin.Manager) (backupInfo, error) {
	backup, err := c.backupLister.Backups(c.namespace).Get(backupName)
	if err != nil {
		return backupInfo{}, err
	}

	location, err := c.backupLocationLister.BackupStorageLocations(c.namespace).Get(backup.Spec.StorageLocation)
	if err != nil {
		return backupInfo{}, errors.WithStack(err)
	}

	backupStore, err := c.newBackupStore(location, pluginManager, c.logger)
	if err != nil {
		return backupInfo{}, err
	}

	return backupInfo{
		backup:      backup,
		backupStore: backupStore,
	}, nil
}

func (c *restoreController) runRestore(
	restore *api.Restore,
	actions []restore.ItemAction,
	info backupInfo,
	pluginManager plugin.Manager,
) (restoreResult, error) {
	var restoreWarnings, restoreErrors api.RestoreResult
	var restoreFailure error
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
		restoreErrors.Velero = append(restoreErrors.Velero, err.Error())
		return restoreResult{warnings: restoreWarnings, errors: restoreErrors}, restoreFailure
	}
	gzippedLogFile := gzip.NewWriter(logFile)
	// Assuming we successfully uploaded the log file, this will have already been closed below. It is safe to call
	// close multiple times. If we get an error closing this, there's not really anything we can do about it.
	defer gzippedLogFile.Close()
	defer closeAndRemoveFile(logFile, c.logger)

	// Log the backup to both a backup log file and to stdout. This will help see what happened if the upload of the
	// backup log failed for whatever reason.
	logger := logging.DefaultLogger(c.restoreLogLevel)
	logger.Out = io.MultiWriter(os.Stdout, gzippedLogFile)
	log := logger.WithFields(
		logrus.Fields{
			"restore": kubeutil.NamespaceAndName(restore),
			"backup":  restore.Spec.BackupName,
		})

	backupFile, err := downloadToTempFile(restore.Spec.BackupName, info.backupStore, c.logger)
	if err != nil {
		log.WithError(err).Error("Error downloading backup")
		restoreErrors.Velero = append(restoreErrors.Velero, err.Error())
		restoreFailure = err
		return restoreResult{warnings: restoreWarnings, errors: restoreErrors}, restoreFailure
	}
	defer closeAndRemoveFile(backupFile, c.logger)

	resultsFile, err := ioutil.TempFile("", "")
	if err != nil {
		log.WithError(errors.WithStack(err)).Error("Error creating results temp file")
		restoreErrors.Velero = append(restoreErrors.Velero, err.Error())
		restoreFailure = err
		return restoreResult{warnings: restoreWarnings, errors: restoreErrors}, restoreFailure
	}
	defer closeAndRemoveFile(resultsFile, c.logger)

	volumeSnapshots, err := info.backupStore.GetBackupVolumeSnapshots(restore.Spec.BackupName)
	if err != nil {
		log.WithError(errors.WithStack(err)).Error("Error fetching volume snapshots")
		restoreErrors.Velero = append(restoreErrors.Velero, err.Error())
		restoreFailure = err
		return restoreResult{warnings: restoreWarnings, errors: restoreErrors}, restoreFailure
	}

	// Any return statement above this line means a total restore failure
	// Some failures after this line *may* be a total restore failure
	log.Info("starting restore")
	restoreWarnings, restoreErrors = c.restorer.Restore(log, restore, info.backup, volumeSnapshots, backupFile, actions, c.snapshotLocationLister, pluginManager)
	log.Info("restore completed")

	// Try to upload the log file. This is best-effort. If we fail, we'll add to the velero errors.
	if err := gzippedLogFile.Close(); err != nil {
		c.logger.WithError(err).Error("error closing gzippedLogFile")
	}
	// Reset the offset to 0 for reading
	if _, err = logFile.Seek(0, 0); err != nil {
		restoreErrors.Velero = append(restoreErrors.Velero, fmt.Sprintf("error resetting log file offset to 0: %v", err))
		return restoreResult{warnings: restoreWarnings, errors: restoreErrors}, restoreFailure
	}

	if err := info.backupStore.PutRestoreLog(restore.Spec.BackupName, restore.Name, logFile); err != nil {
		restoreErrors.Ark = append(restoreErrors.Ark, fmt.Sprintf("error uploading log file to backup storage: %v", err))
	}

	m := map[string]api.RestoreResult{
		"warnings": restoreWarnings,
		"errors":   restoreErrors,
	}

	gzippedResultsFile := gzip.NewWriter(resultsFile)

	if err := json.NewEncoder(gzippedResultsFile).Encode(m); err != nil {
		log.WithError(errors.WithStack(err)).Error("Error encoding restore results")
		return restoreResult{warnings: restoreWarnings, errors: restoreErrors}, restoreFailure
	}
	gzippedResultsFile.Close()

	if _, err = resultsFile.Seek(0, 0); err != nil {
		log.WithError(errors.WithStack(err)).Error("Error resetting results file offset to 0")
		return restoreResult{warnings: restoreWarnings, errors: restoreErrors}, restoreFailure
	}
	if err := info.backupStore.PutRestoreResults(restore.Spec.BackupName, restore.Name, resultsFile); err != nil {
		log.WithError(errors.WithStack(err)).Error("Error uploading results file to backup storage")
	}

	return restoreResult{warnings: restoreWarnings, errors: restoreErrors}, restoreFailure
}

func downloadToTempFile(
	backupName string,
	backupStore persistence.BackupStore,
	logger logrus.FieldLogger,
) (*os.File, error) {
	readCloser, err := backupStore.GetBackupContents(backupName)
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

	log := logger.WithField("backup", backupName)

	log.WithFields(logrus.Fields{
		"fileName": file.Name(),
		"bytes":    n,
	}).Debug("Copied Backup to file")

	if _, err := file.Seek(0, 0); err != nil {
		return nil, errors.Wrap(err, "error resetting Backup file offset")
	}

	return file, nil
}

func patchRestore(original, updated *api.Restore, client velerov1client.RestoresGetter) (*api.Restore, error) {
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
