/*
Copyright The Velero Contributors.

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

	"github.com/apex/log"
	jsonpatch "github.com/evanphx/json-patch"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"golang.org/x/sync/errgroup"
	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/clock"
	kerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/tools/cache"

	"github.com/vmware-tanzu/velero/pkg/util/csi"

	snapshotv1api "github.com/kubernetes-csi/external-snapshotter/client/v4/apis/volumesnapshot/v1"
	snapshotterClientSet "github.com/kubernetes-csi/external-snapshotter/client/v4/clientset/versioned"
	snapshotv1listers "github.com/kubernetes-csi/external-snapshotter/client/v4/listers/volumesnapshot/v1"

	"github.com/vmware-tanzu/velero/internal/storage"
	velerov1api "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	pkgbackup "github.com/vmware-tanzu/velero/pkg/backup"
	"github.com/vmware-tanzu/velero/pkg/discovery"
	"github.com/vmware-tanzu/velero/pkg/features"
	velerov1client "github.com/vmware-tanzu/velero/pkg/generated/clientset/versioned/typed/velero/v1"
	velerov1informers "github.com/vmware-tanzu/velero/pkg/generated/informers/externalversions/velero/v1"
	velerov1listers "github.com/vmware-tanzu/velero/pkg/generated/listers/velero/v1"
	"github.com/vmware-tanzu/velero/pkg/label"
	"github.com/vmware-tanzu/velero/pkg/metrics"
	"github.com/vmware-tanzu/velero/pkg/persistence"
	"github.com/vmware-tanzu/velero/pkg/plugin/clientmgmt"
	"github.com/vmware-tanzu/velero/pkg/plugin/framework"
	"github.com/vmware-tanzu/velero/pkg/util/boolptr"
	"github.com/vmware-tanzu/velero/pkg/util/collections"
	"github.com/vmware-tanzu/velero/pkg/util/encode"
	kubeutil "github.com/vmware-tanzu/velero/pkg/util/kube"
	"github.com/vmware-tanzu/velero/pkg/util/logging"
	"github.com/vmware-tanzu/velero/pkg/volume"

	"sigs.k8s.io/cluster-api/util/patch"
	kbclient "sigs.k8s.io/controller-runtime/pkg/client"
)

type backupController struct {
	*genericController
	discoveryHelper             discovery.Helper
	backupper                   pkgbackup.Backupper
	lister                      velerov1listers.BackupLister
	client                      velerov1client.BackupsGetter
	kbClient                    kbclient.Client
	clock                       clock.Clock
	backupLogLevel              logrus.Level
	newPluginManager            func(logrus.FieldLogger) clientmgmt.Manager
	backupTracker               BackupTracker
	defaultBackupLocation       string
	defaultVolumesToRestic      bool
	defaultBackupTTL            time.Duration
	snapshotLocationLister      velerov1listers.VolumeSnapshotLocationLister
	defaultSnapshotLocations    map[string]string
	metrics                     *metrics.ServerMetrics
	backupStoreGetter           persistence.ObjectBackupStoreGetter
	formatFlag                  logging.Format
	volumeSnapshotLister        snapshotv1listers.VolumeSnapshotLister
	volumeSnapshotClient        *snapshotterClientSet.Clientset
	volumeSnapshotContentLister snapshotv1listers.VolumeSnapshotContentLister
	volumeSnapshotClassLister   snapshotv1listers.VolumeSnapshotClassLister
}

func NewBackupController(
	backupInformer velerov1informers.BackupInformer,
	client velerov1client.BackupsGetter,
	discoveryHelper discovery.Helper,
	backupper pkgbackup.Backupper,
	logger logrus.FieldLogger,
	backupLogLevel logrus.Level,
	newPluginManager func(logrus.FieldLogger) clientmgmt.Manager,
	backupTracker BackupTracker,
	kbClient kbclient.Client,
	defaultBackupLocation string,
	defaultVolumesToRestic bool,
	defaultBackupTTL time.Duration,
	volumeSnapshotLocationLister velerov1listers.VolumeSnapshotLocationLister,
	defaultSnapshotLocations map[string]string,
	metrics *metrics.ServerMetrics,
	formatFlag logging.Format,
	volumeSnapshotLister snapshotv1listers.VolumeSnapshotLister,
	volumeSnapshotClient *snapshotterClientSet.Clientset,
	volumeSnapshotContentLister snapshotv1listers.VolumeSnapshotContentLister,
	volumesnapshotClassLister snapshotv1listers.VolumeSnapshotClassLister,
	backupStoreGetter persistence.ObjectBackupStoreGetter,
) Interface {
	c := &backupController{
		genericController:           newGenericController(Backup, logger),
		discoveryHelper:             discoveryHelper,
		backupper:                   backupper,
		lister:                      backupInformer.Lister(),
		client:                      client,
		clock:                       &clock.RealClock{},
		backupLogLevel:              backupLogLevel,
		newPluginManager:            newPluginManager,
		backupTracker:               backupTracker,
		kbClient:                    kbClient,
		defaultBackupLocation:       defaultBackupLocation,
		defaultVolumesToRestic:      defaultVolumesToRestic,
		defaultBackupTTL:            defaultBackupTTL,
		snapshotLocationLister:      volumeSnapshotLocationLister,
		defaultSnapshotLocations:    defaultSnapshotLocations,
		metrics:                     metrics,
		formatFlag:                  formatFlag,
		volumeSnapshotLister:        volumeSnapshotLister,
		volumeSnapshotClient:        volumeSnapshotClient,
		volumeSnapshotContentLister: volumeSnapshotContentLister,
		volumeSnapshotClassLister:   volumesnapshotClassLister,
		backupStoreGetter:           backupStoreGetter,
	}

	c.syncHandler = c.processBackup
	c.resyncFunc = c.resync
	c.resyncPeriod = time.Minute

	backupInformer.Informer().AddEventHandler(
		cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {
				backup := obj.(*velerov1api.Backup)

				switch backup.Status.Phase {
				case "", velerov1api.BackupPhaseNew, velerov1api.BackupPhaseInProgress:
				default:
					c.logger.WithFields(logrus.Fields{
						"backup": kubeutil.NamespaceAndName(backup),
						"phase":  backup.Status.Phase,
					}).Debug("Backup is not new or in-progress, skipping")
					return
				}

				key, err := cache.MetaNamespaceKeyFunc(backup)
				if err != nil {
					c.logger.WithError(err).WithField(Backup, backup).Error("Error creating queue key, item not added to queue")
					return
				}
				c.queue.Add(key)
			},
		},
	)

	return c
}

func (c *backupController) resync() {
	// recompute backup_total metric
	backups, err := c.lister.List(labels.Everything())
	if err != nil {
		c.logger.Error(err, "Error computing backup_total metric")
	} else {
		c.metrics.SetBackupTotal(int64(len(backups)))
	}

	// recompute backup_last_successful_timestamp metric for each
	// schedule (including the empty schedule, i.e. ad-hoc backups)
	for schedule, timestamp := range getLastSuccessBySchedule(backups) {
		c.metrics.SetBackupLastSuccessfulTimestamp(schedule, timestamp)
	}
}

// getLastSuccessBySchedule finds the most recent completed backup for each schedule
// and returns a map of schedule name -> completion time of the most recent completed
// backup. This map includes an entry for ad-hoc/non-scheduled backups, where the key
// is the empty string.
func getLastSuccessBySchedule(backups []*velerov1api.Backup) map[string]time.Time {
	lastSuccessBySchedule := map[string]time.Time{}
	for _, backup := range backups {
		if backup.Status.Phase != velerov1api.BackupPhaseCompleted {
			continue
		}
		if backup.Status.CompletionTimestamp == nil {
			continue
		}

		schedule := backup.Labels[velerov1api.ScheduleNameLabel]
		timestamp := backup.Status.CompletionTimestamp.Time

		if timestamp.After(lastSuccessBySchedule[schedule]) {
			lastSuccessBySchedule[schedule] = timestamp
		}
	}

	return lastSuccessBySchedule
}

func (c *backupController) processBackup(key string) error {
	log := c.logger.WithField("key", key)

	log.Debug("Running processBackup")
	ns, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		log.WithError(err).Errorf("error splitting key")
		return nil
	}

	log.Debug("Getting backup")
	original, err := c.lister.Backups(ns).Get(name)
	if apierrors.IsNotFound(err) {
		log.Debugf("backup %s not found", name)
		return nil
	}
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
	switch original.Status.Phase {
	case "", velerov1api.BackupPhaseNew:
	case velerov1api.BackupPhaseInProgress:
		// A backup may stay in-progress forever because of
		// 1) the controller restarts during the processing of a backup
		// 2) the backup with in-progress status isn't updated to completed or failed status successfully
		// So we try to mark such Backups as failed to avoid it
		updated := original.DeepCopy()
		updated.Status.Phase = velerov1api.BackupPhaseFailed
		updated.Status.FailureReason = fmt.Sprintf("got a Backup with unexpected status %q, this may be due to a restart of the controller during the backing up, mark it as %q",
			velerov1api.BackupPhaseInProgress, updated.Status.Phase)
		updated.Status.CompletionTimestamp = &metav1.Time{Time: c.clock.Now()}
		_, err = patchBackup(original, updated, c.client)
		if err != nil {
			return errors.Wrapf(err, "error updating Backup status to %s", updated.Status.Phase)
		}
		log.Warn(updated.Status.FailureReason)
		return nil
	default:
		return nil
	}

	log.Debug("Preparing backup request")
	request := c.prepareBackupRequest(original)

	if len(request.Status.ValidationErrors) > 0 {
		request.Status.Phase = velerov1api.BackupPhaseFailedValidation
	} else {
		request.Status.Phase = velerov1api.BackupPhaseInProgress
		request.Status.StartTimestamp = &metav1.Time{Time: c.clock.Now()}
	}

	// update status
	updatedBackup, err := patchBackup(original, request.Backup, c.client)
	if err != nil {
		return errors.Wrapf(err, "error updating Backup status to %s", request.Status.Phase)
	}

	// store ref to just-updated item for creating patch
	original = updatedBackup
	request.Backup = updatedBackup.DeepCopy()

	if request.Status.Phase == velerov1api.BackupPhaseFailedValidation {
		return nil
	}

	c.backupTracker.Add(request.Namespace, request.Name)
	defer c.backupTracker.Delete(request.Namespace, request.Name)

	log.Debug("Running backup")

	backupScheduleName := request.GetLabels()[velerov1api.ScheduleNameLabel]
	c.metrics.RegisterBackupAttempt(backupScheduleName)

	// execution & upload of backup
	if err := c.runBackup(request); err != nil {
		// even though runBackup sets the backup's phase prior
		// to uploading artifacts to object storage, we have to
		// check for an error again here and update the phase if
		// one is found, because there could've been an error
		// while uploading artifacts to object storage, which would
		// result in the backup being Failed.
		log.WithError(err).Error("backup failed")
		request.Status.Phase = velerov1api.BackupPhaseFailed
		request.Status.FailureReason = err.Error()
	}

	switch request.Status.Phase {
	case velerov1api.BackupPhaseCompleted:
		c.metrics.RegisterBackupSuccess(backupScheduleName)
	case velerov1api.BackupPhasePartiallyFailed:
		c.metrics.RegisterBackupPartialFailure(backupScheduleName)
	case velerov1api.BackupPhaseFailed:
		c.metrics.RegisterBackupFailed(backupScheduleName)
	case velerov1api.BackupPhaseFailedValidation:
		c.metrics.RegisterBackupValidationFailure(backupScheduleName)
	}

	log.Debug("Updating backup's final status")
	if _, err := patchBackup(original, request.Backup, c.client); err != nil {
		log.WithError(err).Error("error updating backup's final status")
	}

	return nil
}

func patchBackup(original, updated *velerov1api.Backup, client velerov1client.BackupsGetter) (*velerov1api.Backup, error) {
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

	res, err := client.Backups(original.Namespace).Patch(context.TODO(), original.Name, types.MergePatchType, patchBytes, metav1.PatchOptions{})
	if err != nil {
		return nil, errors.Wrap(err, "error patching backup")
	}

	return res, nil
}

func (c *backupController) prepareBackupRequest(backup *velerov1api.Backup) *pkgbackup.Request {
	request := &pkgbackup.Request{
		Backup: backup.DeepCopy(), // don't modify items in the cache
	}

	// set backup major version - deprecated, use Status.FormatVersion
	request.Status.Version = pkgbackup.BackupVersion

	// set backup major, minor, and patch version
	request.Status.FormatVersion = pkgbackup.BackupFormatVersion

	if request.Spec.TTL.Duration == 0 {
		// set default backup TTL
		request.Spec.TTL.Duration = c.defaultBackupTTL
	}

	// calculate expiration
	request.Status.Expiration = &metav1.Time{Time: c.clock.Now().Add(request.Spec.TTL.Duration)}

	if request.Spec.DefaultVolumesToRestic == nil {
		request.Spec.DefaultVolumesToRestic = &c.defaultVolumesToRestic
	}

	// find which storage location to use
	var serverSpecified bool
	if request.Spec.StorageLocation == "" {
		// when the user doesn't specify a location, use the server default unless there is an existing BSL marked as default
		// TODO(2.0) c.defaultBackupLocation will be deprecated
		request.Spec.StorageLocation = c.defaultBackupLocation

		locationList, err := storage.ListBackupStorageLocations(context.Background(), c.kbClient, request.Namespace)
		if err == nil {
			for _, location := range locationList.Items {
				if location.Spec.Default {
					request.Spec.StorageLocation = location.Name
					break
				}
			}
		}
		serverSpecified = true
	}

	// get the storage location, and store the BackupStorageLocation API obj on the request
	storageLocation := &velerov1api.BackupStorageLocation{}
	if err := c.kbClient.Get(context.Background(), kbclient.ObjectKey{
		Namespace: request.Namespace,
		Name:      request.Spec.StorageLocation,
	}, storageLocation); err != nil {
		if apierrors.IsNotFound(err) {
			if serverSpecified {
				// TODO(2.0) remove this. For now, without mentioning "server default" it could be confusing trying to grasp where the default came from.
				request.Status.ValidationErrors = append(request.Status.ValidationErrors, fmt.Sprintf("an existing backup storage location wasn't specified at backup creation time and the server default '%s' doesn't exist. Please address this issue (see `velero backup-location -h` for options) and create a new backup. Error: %v", request.Spec.StorageLocation, err))
			} else {
				request.Status.ValidationErrors = append(request.Status.ValidationErrors, fmt.Sprintf("an existing backup storage location wasn't specified at backup creation time and the default '%s' wasn't found. Please address this issue (see `velero backup-location -h` for options) and create a new backup. Error: %v", request.Spec.StorageLocation, err))
			}
		} else {
			request.Status.ValidationErrors = append(request.Status.ValidationErrors, fmt.Sprintf("error getting backup storage location: %v", err))
		}
	} else {
		request.StorageLocation = storageLocation

		if request.StorageLocation.Spec.AccessMode == velerov1api.BackupStorageLocationAccessModeReadOnly {
			request.Status.ValidationErrors = append(request.Status.ValidationErrors,
				fmt.Sprintf("backup can't be created because backup storage location %s is currently in read-only mode", request.StorageLocation.Name))
		}
	}

	// add the storage location as a label for easy filtering later.
	if request.Labels == nil {
		request.Labels = make(map[string]string)
	}
	request.Labels[velerov1api.StorageLocationLabel] = label.GetValidName(request.Spec.StorageLocation)

	// validate and get the backup's VolumeSnapshotLocations, and store the
	// VolumeSnapshotLocation API objs on the request
	if locs, errs := c.validateAndGetSnapshotLocations(request.Backup); len(errs) > 0 {
		request.Status.ValidationErrors = append(request.Status.ValidationErrors, errs...)
	} else {
		request.Spec.VolumeSnapshotLocations = nil
		for _, loc := range locs {
			request.Spec.VolumeSnapshotLocations = append(request.Spec.VolumeSnapshotLocations, loc.Name)
			request.SnapshotLocations = append(request.SnapshotLocations, loc)
		}
	}

	// Getting all information of cluster version - useful for future skip-level migration
	if request.Annotations == nil {
		request.Annotations = make(map[string]string)
	}
	request.Annotations[velerov1api.SourceClusterK8sGitVersionAnnotation] = c.discoveryHelper.ServerVersion().String()
	request.Annotations[velerov1api.SourceClusterK8sMajorVersionAnnotation] = c.discoveryHelper.ServerVersion().Major
	request.Annotations[velerov1api.SourceClusterK8sMinorVersionAnnotation] = c.discoveryHelper.ServerVersion().Minor

	// validate the included/excluded resources
	for _, err := range collections.ValidateIncludesExcludes(request.Spec.IncludedResources, request.Spec.ExcludedResources) {
		request.Status.ValidationErrors = append(request.Status.ValidationErrors, fmt.Sprintf("Invalid included/excluded resource lists: %v", err))
	}

	// validate the included/excluded namespaces
	for _, err := range collections.ValidateNamespaceIncludesExcludes(request.Spec.IncludedNamespaces, request.Spec.ExcludedNamespaces) {
		request.Status.ValidationErrors = append(request.Status.ValidationErrors, fmt.Sprintf("Invalid included/excluded namespace lists: %v", err))
	}

	// validate that only one exists orLabelSelector or just labelSelector (singular)
	if request.Spec.OrLabelSelectors != nil && request.Spec.LabelSelector != nil {
		request.Status.ValidationErrors = append(request.Status.ValidationErrors, fmt.Sprintf("encountered labelSelector as well as orLabelSelectors in backup spec, only one can be specified"))
	}

	return request
}

// validateAndGetSnapshotLocations gets a collection of VolumeSnapshotLocation objects that
// this backup will use (returned as a map of provider name -> VSL), and ensures:
// - each location name in .spec.volumeSnapshotLocations exists as a location
// - exactly 1 location per provider
// - a given provider's default location name is added to .spec.volumeSnapshotLocations if one
//   is not explicitly specified for the provider (if there's only one location for the provider,
//   it will automatically be used)
// if backup has snapshotVolume disabled then it returns empty VSL
func (c *backupController) validateAndGetSnapshotLocations(backup *velerov1api.Backup) (map[string]*velerov1api.VolumeSnapshotLocation, []string) {
	errors := []string{}
	providerLocations := make(map[string]*velerov1api.VolumeSnapshotLocation)

	// if snapshotVolume is set to false then we don't need to validate volumesnapshotlocation
	if boolptr.IsSetToFalse(backup.Spec.SnapshotVolumes) {
		return nil, nil
	}

	for _, locationName := range backup.Spec.VolumeSnapshotLocations {
		// validate each locationName exists as a VolumeSnapshotLocation
		location, err := c.snapshotLocationLister.VolumeSnapshotLocations(backup.Namespace).Get(locationName)
		if err != nil {
			if apierrors.IsNotFound(err) {
				errors = append(errors, fmt.Sprintf("a VolumeSnapshotLocation CRD for the location %s with the name specified in the backup spec needs to be created before this snapshot can be executed. Error: %v", locationName, err))
			} else {
				errors = append(errors, fmt.Sprintf("error getting volume snapshot location named %s: %v", locationName, err))
			}
			continue
		}

		// ensure we end up with exactly 1 location *per provider*
		if providerLocation, ok := providerLocations[location.Spec.Provider]; ok {
			// if > 1 location name per provider as in ["aws-us-east-1" | "aws-us-west-1"] (same provider, multiple names)
			if providerLocation.Name != locationName {
				errors = append(errors, fmt.Sprintf("more than one VolumeSnapshotLocation name specified for provider %s: %s; unexpected name was %s", location.Spec.Provider, locationName, providerLocation.Name))
				continue
			}
		} else {
			// keep track of all valid existing locations, per provider
			providerLocations[location.Spec.Provider] = location
		}
	}

	if len(errors) > 0 {
		return nil, errors
	}

	allLocations, err := c.snapshotLocationLister.VolumeSnapshotLocations(backup.Namespace).List(labels.Everything())
	if err != nil {
		errors = append(errors, fmt.Sprintf("error listing volume snapshot locations: %v", err))
		return nil, errors
	}

	// build a map of provider->list of all locations for the provider
	allProviderLocations := make(map[string][]*velerov1api.VolumeSnapshotLocation)
	for i := range allLocations {
		loc := allLocations[i]
		allProviderLocations[loc.Spec.Provider] = append(allProviderLocations[loc.Spec.Provider], loc)
	}

	// go through each provider and make sure we have/can get a VSL
	// for it
	for provider, locations := range allProviderLocations {
		if _, ok := providerLocations[provider]; ok {
			// backup's spec had a location named for this provider
			continue
		}

		if len(locations) > 1 {
			// more than one possible location for the provider: check
			// the defaults
			defaultLocation := c.defaultSnapshotLocations[provider]
			if defaultLocation == "" {
				errors = append(errors, fmt.Sprintf("provider %s has more than one possible volume snapshot location, and none were specified explicitly or as a default", provider))
				continue
			}
			location, err := c.snapshotLocationLister.VolumeSnapshotLocations(backup.Namespace).Get(defaultLocation)
			if err != nil {
				errors = append(errors, fmt.Sprintf("error getting volume snapshot location named %s: %v", defaultLocation, err))
				continue
			}

			providerLocations[provider] = location
			continue
		}

		// exactly one location for the provider: use it
		providerLocations[provider] = locations[0]
	}

	if len(errors) > 0 {
		return nil, errors
	}

	return providerLocations, nil
}

// runBackup runs and uploads a validated backup. Any error returned from this function
// causes the backup to be Failed; if no error is returned, the backup's status's Errors
// field is checked to see if the backup was a partial failure.
func (c *backupController) runBackup(backup *pkgbackup.Request) error {
	c.logger.WithField(Backup, kubeutil.NamespaceAndName(backup)).Info("Setting up backup log")

	logFile, err := ioutil.TempFile("", "")
	if err != nil {
		return errors.Wrap(err, "error creating temp file for backup log")
	}
	gzippedLogFile := gzip.NewWriter(logFile)
	// Assuming we successfully uploaded the log file, this will have already been closed below. It is safe to call
	// close multiple times. If we get an error closing this, there's not really anything we can do about it.
	defer gzippedLogFile.Close()
	defer closeAndRemoveFile(logFile, c.logger.WithField(Backup, kubeutil.NamespaceAndName(backup)))

	// Log the backup to both a backup log file and to stdout. This will help see what happened if the upload of the
	// backup log failed for whatever reason.
	logger := logging.DefaultLogger(c.backupLogLevel, c.formatFlag)
	logger.Out = io.MultiWriter(os.Stdout, gzippedLogFile)

	logCounter := logging.NewLogCounterHook()
	logger.Hooks.Add(logCounter)

	backupLog := logger.WithField(Backup, kubeutil.NamespaceAndName(backup))

	backupLog.Info("Setting up backup temp file")
	backupFile, err := ioutil.TempFile("", "")
	if err != nil {
		return errors.Wrap(err, "error creating temp file for backup")
	}
	defer closeAndRemoveFile(backupFile, backupLog)

	backupLog.Info("Setting up plugin manager")
	pluginManager := c.newPluginManager(backupLog)
	defer pluginManager.CleanupClients()

	backupLog.Info("Getting backup item actions")
	actions, err := pluginManager.GetBackupItemActions()
	if err != nil {
		return err
	}
	itemSnapshotters, err := pluginManager.GetItemSnapshotters()
	if err != nil {
		return err
	}

	backupLog.Info("Setting up backup store to check for backup existence")
	backupStore, err := c.backupStoreGetter.Get(backup.StorageLocation, pluginManager, backupLog)
	if err != nil {
		return err
	}

	exists, err := backupStore.BackupExists(backup.StorageLocation.Spec.StorageType.ObjectStorage.Bucket, backup.Name)
	if exists || err != nil {
		backup.Status.Phase = velerov1api.BackupPhaseFailed
		backup.Status.CompletionTimestamp = &metav1.Time{Time: c.clock.Now()}
		if err != nil {
			return errors.Wrapf(err, "error checking if backup already exists in object storage")
		}
		return errors.Errorf("backup already exists in object storage")
	}

	backupItemActionsResolver := framework.NewBackupItemActionResolver(actions)
	itemSnapshottersResolver := framework.NewItemSnapshotterResolver(itemSnapshotters)

	var fatalErrs []error
	if err := c.backupper.BackupWithResolvers(backupLog, backup, backupFile, backupItemActionsResolver,
		itemSnapshottersResolver, pluginManager); err != nil {
		fatalErrs = append(fatalErrs, err)
	}

	// Empty slices here so that they can be passed in to the persistBackup call later, regardless of whether or not CSI's enabled.
	// This way, we only make the Lister call if the feature flag's on.
	var volumeSnapshots []*snapshotv1api.VolumeSnapshot
	var volumeSnapshotContents []*snapshotv1api.VolumeSnapshotContent
	var volumeSnapshotClasses []*snapshotv1api.VolumeSnapshotClass
	if features.IsEnabled(velerov1api.CSIFeatureFlag) {
		selector := label.NewSelectorForBackup(backup.Name)
		// Listers are wrapped in a nil check out of caution, since they may not be populated based on the
		// EnableCSI feature flag. This is more to guard against programmer error, as they shouldn't be nil
		// when EnableCSI is on.
		if c.volumeSnapshotLister != nil {
			volumeSnapshots, err = c.volumeSnapshotLister.List(selector)
			if err != nil {
				backupLog.Error(err)
			}

			err = c.checkVolumeSnapshotReadyToUse(context.Background(), volumeSnapshots)
			if err != nil {
				backupLog.Errorf("fail to wait VolumeSnapshot change to Ready: %s", err.Error())
			}

			backup.CSISnapshots = volumeSnapshots
		}

		if c.volumeSnapshotContentLister != nil {
			volumeSnapshotContents, err = c.volumeSnapshotContentLister.List(selector)
			if err != nil {
				backupLog.Error(err)
			}
		}
		vsClassSet := sets.NewString()
		for _, vsc := range volumeSnapshotContents {
			// persist the volumesnapshotclasses referenced by vsc
			if c.volumeSnapshotClassLister != nil &&
				vsc.Spec.VolumeSnapshotClassName != nil &&
				!vsClassSet.Has(*vsc.Spec.VolumeSnapshotClassName) {
				if vsClass, err := c.volumeSnapshotClassLister.Get(*vsc.Spec.VolumeSnapshotClassName); err != nil {
					backupLog.Error(err)
				} else {
					vsClassSet.Insert(*vsc.Spec.VolumeSnapshotClassName)
					volumeSnapshotClasses = append(volumeSnapshotClasses, vsClass)
				}
			}
			if err := csi.ResetVolumeSnapshotContent(vsc); err != nil {
				backupLog.Error(err)
			}
		}

		// Delete the VolumeSnapshots created in the backup, when CSI feature is enabled.
		c.deleteVolumeSnapshot(volumeSnapshots, volumeSnapshotContents, *backup, backupLog)

	}

	// Mark completion timestamp before serializing and uploading.
	// Otherwise, the JSON file in object storage has a CompletionTimestamp of 'null'.
	backup.Status.CompletionTimestamp = &metav1.Time{Time: c.clock.Now()}

	backup.Status.VolumeSnapshotsAttempted = len(backup.VolumeSnapshots)
	for _, snap := range backup.VolumeSnapshots {
		if snap.Status.Phase == volume.SnapshotPhaseCompleted {
			backup.Status.VolumeSnapshotsCompleted++
		}
	}

	backup.Status.CSIVolumeSnapshotsAttempted = len(backup.CSISnapshots)
	for _, vs := range backup.CSISnapshots {
		if vs.Status != nil && boolptr.IsSetToTrue(vs.Status.ReadyToUse) {
			backup.Status.CSIVolumeSnapshotsCompleted++
		}
	}

	backup.Status.Warnings = logCounter.GetCount(logrus.WarnLevel)
	backup.Status.Errors = logCounter.GetCount(logrus.ErrorLevel)

	recordBackupMetrics(backupLog, backup.Backup, backupFile, c.metrics)

	if err := gzippedLogFile.Close(); err != nil {
		c.logger.WithField(Backup, kubeutil.NamespaceAndName(backup)).WithError(err).Error("error closing gzippedLogFile")
	}

	// Assign finalize phase as close to end as possible so that any errors
	// logged to backupLog are captured. This is done before uploading the
	// artifacts to object storage so that the JSON representation of the
	// backup in object storage has the terminal phase set.
	switch {
	case len(fatalErrs) > 0:
		backup.Status.Phase = velerov1api.BackupPhaseFailed
	case logCounter.GetCount(logrus.ErrorLevel) > 0:
		backup.Status.Phase = velerov1api.BackupPhasePartiallyFailed
	default:
		backup.Status.Phase = velerov1api.BackupPhaseCompleted
	}

	// re-instantiate the backup store because credentials could have changed since the original
	// instantiation, if this was a long-running backup
	backupLog.Info("Setting up backup store to persist the backup")
	backupStore, err = c.backupStoreGetter.Get(backup.StorageLocation, pluginManager, backupLog)
	if err != nil {
		return err
	}

	if errs := persistBackup(backup, backupFile, logFile, backupStore, c.logger.WithField(Backup, kubeutil.NamespaceAndName(backup)), volumeSnapshots, volumeSnapshotContents, volumeSnapshotClasses); len(errs) > 0 {
		fatalErrs = append(fatalErrs, errs...)
	}

	c.logger.WithField(Backup, kubeutil.NamespaceAndName(backup)).Info("Backup completed")

	// if we return a non-nil error, the calling function will update
	// the backup's phase to Failed.
	return kerrors.NewAggregate(fatalErrs)
}

func recordBackupMetrics(log logrus.FieldLogger, backup *velerov1api.Backup, backupFile *os.File, serverMetrics *metrics.ServerMetrics) {
	backupScheduleName := backup.GetLabels()[velerov1api.ScheduleNameLabel]

	var backupSizeBytes int64
	if backupFileStat, err := backupFile.Stat(); err != nil {
		log.WithError(errors.WithStack(err)).Error("Error getting backup file info")
	} else {
		backupSizeBytes = backupFileStat.Size()
	}
	serverMetrics.SetBackupTarballSizeBytesGauge(backupScheduleName, backupSizeBytes)

	backupDuration := backup.Status.CompletionTimestamp.Time.Sub(backup.Status.StartTimestamp.Time)
	backupDurationSeconds := float64(backupDuration / time.Second)
	serverMetrics.RegisterBackupDuration(backupScheduleName, backupDurationSeconds)
	serverMetrics.RegisterVolumeSnapshotAttempts(backupScheduleName, backup.Status.VolumeSnapshotsAttempted)
	serverMetrics.RegisterVolumeSnapshotSuccesses(backupScheduleName, backup.Status.VolumeSnapshotsCompleted)
	serverMetrics.RegisterVolumeSnapshotFailures(backupScheduleName, backup.Status.VolumeSnapshotsAttempted-backup.Status.VolumeSnapshotsCompleted)

	if features.IsEnabled(velerov1api.CSIFeatureFlag) {
		serverMetrics.RegisterCSISnapshotAttempts(backupScheduleName, backup.Name, backup.Status.CSIVolumeSnapshotsAttempted)
		serverMetrics.RegisterCSISnapshotSuccesses(backupScheduleName, backup.Name, backup.Status.CSIVolumeSnapshotsCompleted)
		serverMetrics.RegisterCSISnapshotFailures(backupScheduleName, backup.Name, backup.Status.CSIVolumeSnapshotsAttempted-backup.Status.CSIVolumeSnapshotsCompleted)
	}

	if backup.Status.Progress != nil {
		serverMetrics.RegisterBackupItemsTotalGauge(backupScheduleName, backup.Status.Progress.TotalItems)
	}
	serverMetrics.RegisterBackupItemsErrorsGauge(backupScheduleName, backup.Status.Errors)
}

func persistBackup(backup *pkgbackup.Request,
	backupContents, backupLog *os.File,
	backupStore persistence.BackupStore,
	log logrus.FieldLogger,
	csiVolumeSnapshots []*snapshotv1api.VolumeSnapshot,
	csiVolumeSnapshotContents []*snapshotv1api.VolumeSnapshotContent,
	csiVolumesnapshotClasses []*snapshotv1api.VolumeSnapshotClass,
) []error {
	persistErrs := []error{}
	backupJSON := new(bytes.Buffer)

	if err := encode.EncodeTo(backup.Backup, "json", backupJSON); err != nil {
		persistErrs = append(persistErrs, errors.Wrap(err, "error encoding backup"))
	}

	// Velero-native volume snapshots (as opposed to CSI ones)
	nativeVolumeSnapshots, errs := encodeToJSONGzip(backup.VolumeSnapshots, "native volumesnapshots list")
	if errs != nil {
		persistErrs = append(persistErrs, errs...)
	}

	podVolumeBackups, errs := encodeToJSONGzip(backup.PodVolumeBackups, "pod volume backups list")
	if errs != nil {
		persistErrs = append(persistErrs, errs...)
	}

	csiSnapshotJSON, errs := encodeToJSONGzip(csiVolumeSnapshots, "csi volume snapshots list")
	if errs != nil {
		persistErrs = append(persistErrs, errs...)
	}

	csiSnapshotContentsJSON, errs := encodeToJSONGzip(csiVolumeSnapshotContents, "csi volume snapshot contents list")
	if errs != nil {
		persistErrs = append(persistErrs, errs...)
	}
	csiSnapshotClassesJSON, errs := encodeToJSONGzip(csiVolumesnapshotClasses, "csi volume snapshot classes list")
	if errs != nil {
		persistErrs = append(persistErrs, errs...)
	}

	backupResourceList, errs := encodeToJSONGzip(backup.BackupResourceList(), "backup resources list")
	if errs != nil {
		persistErrs = append(persistErrs, errs...)
	}

	if len(persistErrs) > 0 {
		// Don't upload the JSON files or backup tarball if encoding to json fails.
		backupJSON = nil
		backupContents = nil
		nativeVolumeSnapshots = nil
		backupResourceList = nil
		csiSnapshotJSON = nil
		csiSnapshotContentsJSON = nil
		csiSnapshotClassesJSON = nil
	}

	backupInfo := persistence.BackupInfo{
		Name:                      backup.Name,
		Metadata:                  backupJSON,
		Contents:                  backupContents,
		Log:                       backupLog,
		PodVolumeBackups:          podVolumeBackups,
		VolumeSnapshots:           nativeVolumeSnapshots,
		BackupResourceList:        backupResourceList,
		CSIVolumeSnapshots:        csiSnapshotJSON,
		CSIVolumeSnapshotContents: csiSnapshotContentsJSON,
		CSIVolumeSnapshotClasses:  csiSnapshotClassesJSON,
	}
	if err := backupStore.PutBackup(backupInfo); err != nil {
		persistErrs = append(persistErrs, err)
	}

	return persistErrs
}

func closeAndRemoveFile(file *os.File, log logrus.FieldLogger) {
	if file == nil {
		log.Debug("Skipping removal of file due to nil file pointer")
		return
	}
	if err := file.Close(); err != nil {
		log.WithError(err).WithField("file", file.Name()).Error("error closing file")
	}
	if err := os.Remove(file.Name()); err != nil {
		log.WithError(err).WithField("file", file.Name()).Error("error removing file")
	}
}

// encodeToJSONGzip takes arbitrary Go data and encodes it to GZip compressed JSON in a buffer, as well as a description of the data to put into an error should encoding fail.
func encodeToJSONGzip(data interface{}, desc string) (*bytes.Buffer, []error) {
	buf := new(bytes.Buffer)
	gzw := gzip.NewWriter(buf)

	// Since both encoding and closing the gzip writer could fail separately and both errors are useful,
	// collect both errors to report back.
	errs := []error{}

	if err := json.NewEncoder(gzw).Encode(data); err != nil {
		errs = append(errs, errors.Wrapf(err, "error encoding %s", desc))
	}
	if err := gzw.Close(); err != nil {
		errs = append(errs, errors.Wrapf(err, "error closing gzip writer for %s", desc))
	}

	if len(errs) > 0 {
		return nil, errs
	}

	return buf, nil
}

// Waiting for VolumeSnapshot ReadyTosue to true is time consuming. Try to make the process parallel by
// using goroutine here instead of waiting in CSI plugin, because it's not easy to make BackupItemAction
// parallel by now. After BackupItemAction parallel is implemented, this logic should be moved to CSI plugin
// as https://github.com/vmware-tanzu/velero-plugin-for-csi/pull/100
func (c *backupController) checkVolumeSnapshotReadyToUse(ctx context.Context, volumesnapshots []*snapshotv1api.VolumeSnapshot) error {
	eg, _ := errgroup.WithContext(ctx)
	timeout := 10 * time.Minute
	interval := 5 * time.Second

	for _, vs := range volumesnapshots {
		volumeSnapshot := vs
		eg.Go(func() error {
			err := wait.PollImmediate(interval, timeout, func() (bool, error) {
				tmpVS, err := c.volumeSnapshotClient.SnapshotV1().VolumeSnapshots(volumeSnapshot.Namespace).Get(ctx, volumeSnapshot.Name, metav1.GetOptions{})
				if err != nil {
					return false, errors.Wrapf(err, fmt.Sprintf("failed to get volumesnapshot %s/%s", volumeSnapshot.Namespace, volumeSnapshot.Name))
				}
				if tmpVS.Status == nil || tmpVS.Status.BoundVolumeSnapshotContentName == nil || !boolptr.IsSetToTrue(tmpVS.Status.ReadyToUse) {
					log.Infof("Waiting for CSI driver to reconcile volumesnapshot %s/%s. Retrying in %ds", volumeSnapshot.Namespace, volumeSnapshot.Name, interval/time.Second)
					return false, nil
				}

				return true, nil
			})
			if err == wait.ErrWaitTimeout {
				log.Errorf("Timed out awaiting reconciliation of volumesnapshot %s/%s", volumeSnapshot.Namespace, volumeSnapshot.Name)
			}
			return err
		})
	}
	return eg.Wait()
}

// deleteVolumeSnapshot delete VolumeSnapshot created during backup.
// This is used to avoid deleting namespace in cluster triggers the VolumeSnapshot deletion,
// which will cause snapshot deletion on cloud provider, then backup cannot restore the PV.
// If DeletionPolicy is Retain, just delete it. If DeletionPolicy is Delete, need to
// change DeletionPolicy to Retain before deleting VS, then change DeletionPolicy back to Delete.
func (c *backupController) deleteVolumeSnapshot(volumeSnapshots []*snapshotv1api.VolumeSnapshot,
	volumeSnapshotContents []*snapshotv1api.VolumeSnapshotContent,
	backup pkgbackup.Request, logger logrus.FieldLogger) {
	var wg sync.WaitGroup
	vscMap := make(map[string]*snapshotv1api.VolumeSnapshotContent)
	for _, vsc := range volumeSnapshotContents {
		vscMap[vsc.Name] = vsc
	}

	for _, vs := range volumeSnapshots {
		wg.Add(1)
		go func(vs *snapshotv1api.VolumeSnapshot) {
			defer wg.Done()
			var vsc *snapshotv1api.VolumeSnapshotContent
			modifyVSCFlag := false
			if vs.Status.BoundVolumeSnapshotContentName != nil &&
				len(*vs.Status.BoundVolumeSnapshotContentName) > 0 {
				vsc = vscMap[*vs.Status.BoundVolumeSnapshotContentName]
				if vsc.Spec.DeletionPolicy == snapshotv1api.VolumeSnapshotContentDelete {
					modifyVSCFlag = true
				}
			}

			// Change VolumeSnapshotContent's DeletionPolicy to Retain before deleting VolumeSnapshot,
			// because VolumeSnapshotContent will be deleted by deleting VolumeSnapshot, when
			// DeletionPolicy is set to Delete, but Velero needs VSC for cleaning snapshot on cloud
			// in backup deletion.
			if modifyVSCFlag {
				logger.Debugf("Patching VolumeSnapshotContent %s", vsc.Name)
				_, err := c.patchVolumeSnapshotContent(vsc, func(req *snapshotv1api.VolumeSnapshotContent) {
					req.Spec.DeletionPolicy = snapshotv1api.VolumeSnapshotContentRetain
				})
				if err != nil {
					logger.Errorf("fail to modify VolumeSnapshotContent %s DeletionPolicy to Retain: %s", vsc.Name, err.Error())
					return
				}

				defer func() {
					logger.Debugf("Start to recreate VolumeSnapshotContent %s", vsc.Name)
					err := c.recreateVolumeSnapshotContent(vsc)
					if err != nil {
						logger.Errorf("fail to recreate VolumeSnapshotContent %s: %s", vsc.Name, err.Error())
					}
				}()
			}

			// Delete VolumeSnapshot from cluster
			logger.Debugf("Deleting VolumeSnapshotContent %s", vsc.Name)
			err := c.volumeSnapshotClient.SnapshotV1().VolumeSnapshots(vs.Namespace).Delete(context.TODO(), vs.Name, metav1.DeleteOptions{})
			if err != nil {
				logger.Errorf("fail to delete VolumeSnapshot %s/%s: %s", vs.Namespace, vs.Name, err.Error())
			}
		}(vs)
	}

	wg.Wait()
}

func (c *backupController) patchVolumeSnapshotContent(req *snapshotv1api.VolumeSnapshotContent, mutate func(*snapshotv1api.VolumeSnapshotContent)) (*snapshotv1api.VolumeSnapshotContent, error) {
	patchHelper, err := patch.NewHelper(req, c.kbClient)
	if err != nil {
		return nil, errors.Wrap(err, "fail to get patch helper.")
	}

	// Mutate
	mutate(req)

	if err := patchHelper.Patch(context.TODO(), req); err != nil {
		return nil, errors.Wrapf(err, "fail to patch VolumeSnapshotContent %s", req.Name)
	}

	return req, nil
}

// recreateVolumeSnapshotContent will delete then re-create VolumeSnapshotContent,
// because some parameter in VolumeSnapshotContent Spec is immutable, e.g. VolumeSnapshotRef
// and Source. Source is updated to let csi-controller thinks the VSC is statically provsisioned with VS.
// Set VolumeSnapshotRef's UID to nil will let the csi-controller finds out the related VS is gone, then
// VSC can be deleted.
func (c *backupController) recreateVolumeSnapshotContent(vsc *snapshotv1api.VolumeSnapshotContent) error {
	timeout := 1 * time.Minute
	interval := 1 * time.Second

	err := c.volumeSnapshotClient.SnapshotV1().VolumeSnapshotContents().Delete(context.TODO(), vsc.Name, metav1.DeleteOptions{})
	if err != nil {
		return errors.Wrapf(err, "fail to delete VolumeSnapshotContent: %s", vsc.Name)
	}

	// Check VolumeSnapshotContents is already deleted, before re-creating it.
	err = wait.PollImmediate(interval, timeout, func() (bool, error) {
		_, err := c.volumeSnapshotClient.SnapshotV1().VolumeSnapshotContents().Get(context.TODO(), vsc.Name, metav1.GetOptions{})
		if err != nil {
			if apierrors.IsNotFound(err) {
				return true, nil
			}
			return false, errors.Wrapf(err, fmt.Sprintf("failed to get VolumeSnapshotContent %s", vsc.Name))
		}
		return false, nil
	})
	if err != nil {
		return errors.Wrapf(err, "fail to retrieve VolumeSnapshotContent %s info", vsc.Name)
	}

	// Make the VolumeSnapshotContent static
	vsc.Spec.Source = snapshotv1api.VolumeSnapshotContentSource{
		SnapshotHandle: vsc.Status.SnapshotHandle,
	}
	// Set VolumeSnapshotRef to none exist one, because VolumeSnapshotContent
	// validation webhook will check whether name and namespace are nil.
	// external-snapshotter needs Source pointing to snapshot and VolumeSnapshot
	// reference's UID to nil to determine the VolumeSnapshotContent is deletable.
	vsc.Spec.VolumeSnapshotRef = v1.ObjectReference{
		APIVersion: snapshotv1api.SchemeGroupVersion.String(),
		Kind:       "VolumeSnapshot",
		Namespace:  "ns-" + string(vsc.UID),
		Name:       "name-" + string(vsc.UID),
	}
	// Revert DeletionPolicy to Delete
	vsc.Spec.DeletionPolicy = snapshotv1api.VolumeSnapshotContentDelete
	// ResourceVersion shouldn't exist for new creation.
	vsc.ResourceVersion = ""
	_, err = c.volumeSnapshotClient.SnapshotV1().VolumeSnapshotContents().Create(context.TODO(), vsc, metav1.CreateOptions{})
	if err != nil {
		return errors.Wrapf(err, "fail to create VolumeSnapshotContent %s", vsc.Name)
	}

	return nil
}
