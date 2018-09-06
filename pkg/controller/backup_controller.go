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
	"bytes"
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

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/clock"
	kerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"

	api "github.com/heptio/ark/pkg/apis/ark/v1"
	"github.com/heptio/ark/pkg/backup"
	arkv1client "github.com/heptio/ark/pkg/generated/clientset/versioned/typed/ark/v1"
	informers "github.com/heptio/ark/pkg/generated/informers/externalversions/ark/v1"
	listers "github.com/heptio/ark/pkg/generated/listers/ark/v1"
	"github.com/heptio/ark/pkg/metrics"
	"github.com/heptio/ark/pkg/persistence"
	"github.com/heptio/ark/pkg/plugin"
	"github.com/heptio/ark/pkg/util/collections"
	"github.com/heptio/ark/pkg/util/encode"
	kubeutil "github.com/heptio/ark/pkg/util/kube"
	"github.com/heptio/ark/pkg/util/logging"
)

const backupVersion = 1

type backupController struct {
	backupper                  backup.Backupper
	pvProviderExists           bool
	lister                     listers.BackupLister
	listerSynced               cache.InformerSynced
	client                     arkv1client.BackupsGetter
	syncHandler                func(backupName string) error
	queue                      workqueue.RateLimitingInterface
	clock                      clock.Clock
	logger                     logrus.FieldLogger
	logLevel                   logrus.Level
	newPluginManager           func(logrus.FieldLogger) plugin.Manager
	backupTracker              BackupTracker
	backupLocationLister       listers.BackupStorageLocationLister
	backupLocationListerSynced cache.InformerSynced
	defaultBackupLocation      string
	metrics                    *metrics.ServerMetrics
	newBackupStore             func(*api.BackupStorageLocation, persistence.ObjectStoreGetter, logrus.FieldLogger) (persistence.BackupStore, error)
}

func NewBackupController(
	backupInformer informers.BackupInformer,
	client arkv1client.BackupsGetter,
	backupper backup.Backupper,
	pvProviderExists bool,
	logger logrus.FieldLogger,
	logLevel logrus.Level,
	newPluginManager func(logrus.FieldLogger) plugin.Manager,
	backupTracker BackupTracker,
	backupLocationInformer informers.BackupStorageLocationInformer,
	defaultBackupLocation string,
	metrics *metrics.ServerMetrics,
) Interface {
	c := &backupController{
		backupper:                  backupper,
		pvProviderExists:           pvProviderExists,
		lister:                     backupInformer.Lister(),
		listerSynced:               backupInformer.Informer().HasSynced,
		client:                     client,
		queue:                      workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "backup"),
		clock:                      &clock.RealClock{},
		logger:                     logger,
		logLevel:                   logLevel,
		newPluginManager:           newPluginManager,
		backupTracker:              backupTracker,
		backupLocationLister:       backupLocationInformer.Lister(),
		backupLocationListerSynced: backupLocationInformer.Informer().HasSynced,
		defaultBackupLocation:      defaultBackupLocation,
		metrics:                    metrics,

		newBackupStore: persistence.NewObjectBackupStore,
	}

	c.syncHandler = c.processBackup

	backupInformer.Informer().AddEventHandler(
		cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {
				backup := obj.(*api.Backup)

				switch backup.Status.Phase {
				case "", api.BackupPhaseNew:
					// only process new backups
				default:
					c.logger.WithFields(logrus.Fields{
						"backup": kubeutil.NamespaceAndName(backup),
						"phase":  backup.Status.Phase,
					}).Debug("Backup is not new, skipping")
					return
				}

				key, err := cache.MetaNamespaceKeyFunc(backup)
				if err != nil {
					c.logger.WithError(err).WithField("backup", backup).Error("Error creating queue key, item not added to queue")
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
func (controller *backupController) Run(ctx context.Context, numWorkers int) error {
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

	controller.logger.Info("Starting BackupController")
	defer controller.logger.Info("Shutting down BackupController")

	controller.logger.Info("Waiting for caches to sync")
	if !cache.WaitForCacheSync(ctx.Done(), controller.listerSynced, controller.backupLocationListerSynced) {
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

func (controller *backupController) runWorker() {
	// continually take items off the queue (waits if it's
	// empty) until we get a shutdown signal from the queue
	for controller.processNextWorkItem() {
	}
}

func (controller *backupController) processNextWorkItem() bool {
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

func (controller *backupController) processBackup(key string) error {
	logContext := controller.logger.WithField("key", key)

	logContext.Debug("Running processBackup")
	ns, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		return errors.Wrap(err, "error splitting queue key")
	}

	logContext.Debug("Getting backup")
	backup, err := controller.lister.Backups(ns).Get(name)
	if err != nil {
		return errors.Wrap(err, "error getting backup")
	}

	// Double-check we have the correct phase. In the unlikely event that multiple controller
	// instances are running, it's possible for controller A to succeed in changing the phase to
	// InProgress, while controller B's attempt to patch the phase fails. When controller B
	// reprocesses the same backup, it will either show up as New (informer hasn't seen the update
	// yet) or as InProgress. In the former case, the patch attempt will fail again, until the
	// informer sees the update. In the latter case, after the informer has seen the update to
	// InProgress, we still need this check so we can return nil to indicate we've finished processing
	// this key (even though it was a no-op).
	switch backup.Status.Phase {
	case "", api.BackupPhaseNew:
		// only process new backups
	default:
		return nil
	}

	logContext.Debug("Cloning backup")
	// store ref to original for creating patch
	original := backup
	// don't modify items in the cache
	backup = backup.DeepCopy()

	// set backup version
	backup.Status.Version = backupVersion

	// calculate expiration
	if backup.Spec.TTL.Duration > 0 {
		backup.Status.Expiration = metav1.NewTime(controller.clock.Now().Add(backup.Spec.TTL.Duration))
	}

	var backupLocation *api.BackupStorageLocation
	// validation
	if backupLocation, backup.Status.ValidationErrors = controller.getLocationAndValidate(backup, controller.defaultBackupLocation); len(backup.Status.ValidationErrors) > 0 {
		backup.Status.Phase = api.BackupPhaseFailedValidation
	} else {
		backup.Status.Phase = api.BackupPhaseInProgress
	}

	// update status
	updatedBackup, err := patchBackup(original, backup, controller.client)
	if err != nil {
		return errors.Wrapf(err, "error updating Backup status to %s", backup.Status.Phase)
	}
	// store ref to just-updated item for creating patch
	original = updatedBackup
	backup = updatedBackup.DeepCopy()

	if backup.Status.Phase == api.BackupPhaseFailedValidation {
		return nil
	}

	controller.backupTracker.Add(backup.Namespace, backup.Name)
	defer controller.backupTracker.Delete(backup.Namespace, backup.Name)

	logContext.Debug("Running backup")
	// execution & upload of backup
	backupScheduleName := backup.GetLabels()["ark-schedule"]
	controller.metrics.RegisterBackupAttempt(backupScheduleName)

	if err := controller.runBackup(backup, backupLocation); err != nil {
		logContext.WithError(err).Error("backup failed")
		backup.Status.Phase = api.BackupPhaseFailed
		controller.metrics.RegisterBackupFailed(backupScheduleName)
	} else {
		controller.metrics.RegisterBackupSuccess(backupScheduleName)
	}

	logContext.Debug("Updating backup's final status")
	if _, err := patchBackup(original, backup, controller.client); err != nil {
		logContext.WithError(err).Error("error updating backup's final status")
	}

	return nil
}

func patchBackup(original, updated *api.Backup, client arkv1client.BackupsGetter) (*api.Backup, error) {
	origBytes, err := json.Marshal(original)
	if err != nil {
		return nil, errors.Wrap(err, "error marshalling original backup")
	}

	updatedBytes, err := json.Marshal(updated)
	if err != nil {
		return nil, errors.Wrap(err, "error marshalling updated backup")
	}

	patchBytes, err := jsonpatch.CreateMergePatch(origBytes, updatedBytes)
	if err != nil {
		return nil, errors.Wrap(err, "error creating json merge patch for backup")
	}

	res, err := client.Backups(original.Namespace).Patch(original.Name, types.MergePatchType, patchBytes)
	if err != nil {
		return nil, errors.Wrap(err, "error patching backup")
	}

	return res, nil
}

func (controller *backupController) getLocationAndValidate(itm *api.Backup, defaultBackupLocation string) (*api.BackupStorageLocation, []string) {
	var validationErrors []string

	for _, err := range collections.ValidateIncludesExcludes(itm.Spec.IncludedResources, itm.Spec.ExcludedResources) {
		validationErrors = append(validationErrors, fmt.Sprintf("Invalid included/excluded resource lists: %v", err))
	}

	for _, err := range collections.ValidateIncludesExcludes(itm.Spec.IncludedNamespaces, itm.Spec.ExcludedNamespaces) {
		validationErrors = append(validationErrors, fmt.Sprintf("Invalid included/excluded namespace lists: %v", err))
	}

	if !controller.pvProviderExists && itm.Spec.SnapshotVolumes != nil && *itm.Spec.SnapshotVolumes {
		validationErrors = append(validationErrors, "Server is not configured for PV snapshots")
	}

	if itm.Spec.StorageLocation == "" {
		itm.Spec.StorageLocation = defaultBackupLocation
	}

	// add the storage location as a label for easy filtering later.
	if itm.Labels == nil {
		itm.Labels = make(map[string]string)
	}
	itm.Labels[api.StorageLocationLabel] = itm.Spec.StorageLocation

	var backupLocation *api.BackupStorageLocation
	backupLocation, err := controller.backupLocationLister.BackupStorageLocations(itm.Namespace).Get(itm.Spec.StorageLocation)
	if err != nil {
		validationErrors = append(validationErrors, fmt.Sprintf("Error getting backup storage location: %v", err))
	}

	return backupLocation, validationErrors
}

func (controller *backupController) runBackup(backup *api.Backup, backupLocation *api.BackupStorageLocation) error {
	log := controller.logger.WithField("backup", kubeutil.NamespaceAndName(backup))
	log.Info("Starting backup")
	backup.Status.StartTimestamp.Time = controller.clock.Now()

	logFile, err := ioutil.TempFile("", "")
	if err != nil {
		return errors.Wrap(err, "error creating temp file for backup log")
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
	log = logger.WithField("backup", kubeutil.NamespaceAndName(backup))

	log.Info("Starting backup")

	backupFile, err := ioutil.TempFile("", "")
	if err != nil {
		return errors.Wrap(err, "error creating temp file for backup")
	}
	defer closeAndRemoveFile(backupFile, log)

	pluginManager := controller.newPluginManager(log)
	defer pluginManager.CleanupClients()

	actions, err := pluginManager.GetBackupItemActions()
	if err != nil {
		return err
	}

	backupStore, err := controller.newBackupStore(backupLocation, pluginManager, log)
	if err != nil {
		return err
	}

	var errs []error

	var backupJSONToUpload, backupFileToUpload io.Reader

	// Do the actual backup
	if err := controller.backupper.Backup(log, backup, backupFile, actions); err != nil {
		errs = append(errs, err)

		backup.Status.Phase = api.BackupPhaseFailed
	} else {
		backup.Status.Phase = api.BackupPhaseCompleted
	}

	// Mark completion timestamp before serializing and uploading.
	// Otherwise, the JSON file in object storage has a CompletionTimestamp of 'null'.
	backup.Status.CompletionTimestamp.Time = controller.clock.Now()

	backupJSON := new(bytes.Buffer)
	if err := encode.EncodeTo(backup, "json", backupJSON); err != nil {
		errs = append(errs, errors.Wrap(err, "error encoding backup"))
	} else {
		// Only upload the json and backup tarball if encoding to json succeeded.
		backupJSONToUpload = backupJSON
		backupFileToUpload = backupFile
	}

	var backupSizeBytes int64
	if backupFileStat, err := backupFile.Stat(); err != nil {
		errs = append(errs, errors.Wrap(err, "error getting file info"))
	} else {
		backupSizeBytes = backupFileStat.Size()
	}

	if err := gzippedLogFile.Close(); err != nil {
		controller.logger.WithError(err).Error("error closing gzippedLogFile")
	}

	if err := backupStore.PutBackup(backup.Name, backupJSONToUpload, backupFileToUpload, logFile); err != nil {
		errs = append(errs, err)
	}

	backupScheduleName := backup.GetLabels()["ark-schedule"]
	controller.metrics.SetBackupTarballSizeBytesGauge(backupScheduleName, backupSizeBytes)

	backupDuration := backup.Status.CompletionTimestamp.Time.Sub(backup.Status.StartTimestamp.Time)
	backupDurationSeconds := float64(backupDuration / time.Second)
	controller.metrics.RegisterBackupDuration(backupScheduleName, backupDurationSeconds)

	log.Info("Backup completed")

	return kerrors.NewAggregate(errs)
}

func closeAndRemoveFile(file *os.File, log logrus.FieldLogger) {
	if err := file.Close(); err != nil {
		log.WithError(err).WithField("file", file.Name()).Error("error closing file")
	}
	if err := os.Remove(file.Name()); err != nil {
		log.WithError(err).WithField("file", file.Name()).Error("error removing file")
	}
}
