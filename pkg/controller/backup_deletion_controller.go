/*
Copyright 2020 the Velero contributors.

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
	"encoding/json"
	"fmt"
	"time"

	jsonpatch "github.com/evanphx/json-patch"
	snapshotterClientSet "github.com/kubernetes-csi/external-snapshotter/client/v4/clientset/versioned"
	snapshotter "github.com/kubernetes-csi/external-snapshotter/client/v4/clientset/versioned/typed/volumesnapshot/v1beta1"
	snapshotv1beta1listers "github.com/kubernetes-csi/external-snapshotter/client/v4/listers/volumesnapshot/v1beta1"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/clock"
	kubeerrs "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/client-go/tools/cache"

	"github.com/vmware-tanzu/velero/internal/delete"
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
	"github.com/vmware-tanzu/velero/pkg/plugin/velero"
	"github.com/vmware-tanzu/velero/pkg/restic"
	"github.com/vmware-tanzu/velero/pkg/util/filesystem"
	"github.com/vmware-tanzu/velero/pkg/util/kube"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

const resticTimeout = time.Minute

type backupDeletionController struct {
	*genericController

	deleteBackupRequestClient velerov1client.DeleteBackupRequestsGetter
	deleteBackupRequestLister velerov1listers.DeleteBackupRequestLister
	backupClient              velerov1client.BackupsGetter
	restoreLister             velerov1listers.RestoreLister
	restoreClient             velerov1client.RestoresGetter
	backupTracker             BackupTracker
	resticMgr                 restic.RepositoryManager
	podvolumeBackupLister     velerov1listers.PodVolumeBackupLister
	kbClient                  client.Client
	snapshotLocationLister    velerov1listers.VolumeSnapshotLocationLister
	csiSnapshotLister         snapshotv1beta1listers.VolumeSnapshotLister
	csiSnapshotContentLister  snapshotv1beta1listers.VolumeSnapshotContentLister
	csiSnapshotClient         *snapshotterClientSet.Clientset
	processRequestFunc        func(*velerov1api.DeleteBackupRequest) error
	clock                     clock.Clock
	newPluginManager          func(logrus.FieldLogger) clientmgmt.Manager
	newBackupStore            func(*velerov1api.BackupStorageLocation, persistence.ObjectStoreGetter, logrus.FieldLogger) (persistence.BackupStore, error)
	metrics                   *metrics.ServerMetrics
	helper                    discovery.Helper
}

// NewBackupDeletionController creates a new backup deletion controller.
func NewBackupDeletionController(
	logger logrus.FieldLogger,
	deleteBackupRequestInformer velerov1informers.DeleteBackupRequestInformer,
	deleteBackupRequestClient velerov1client.DeleteBackupRequestsGetter,
	backupClient velerov1client.BackupsGetter,
	restoreLister velerov1listers.RestoreLister,
	restoreClient velerov1client.RestoresGetter,
	backupTracker BackupTracker,
	resticMgr restic.RepositoryManager,
	podvolumeBackupLister velerov1listers.PodVolumeBackupLister,
	kbClient client.Client,
	snapshotLocationLister velerov1listers.VolumeSnapshotLocationLister,
	csiSnapshotLister snapshotv1beta1listers.VolumeSnapshotLister,
	csiSnapshotContentLister snapshotv1beta1listers.VolumeSnapshotContentLister,
	csiSnapshotClient *snapshotterClientSet.Clientset,
	newPluginManager func(logrus.FieldLogger) clientmgmt.Manager,
	metrics *metrics.ServerMetrics,
	helper discovery.Helper,
) Interface {
	c := &backupDeletionController{
		genericController:         newGenericController(BackupDeletion, logger),
		deleteBackupRequestClient: deleteBackupRequestClient,
		deleteBackupRequestLister: deleteBackupRequestInformer.Lister(),
		backupClient:              backupClient,
		restoreLister:             restoreLister,
		restoreClient:             restoreClient,
		backupTracker:             backupTracker,
		resticMgr:                 resticMgr,
		podvolumeBackupLister:     podvolumeBackupLister,
		kbClient:                  kbClient,
		snapshotLocationLister:    snapshotLocationLister,
		csiSnapshotLister:         csiSnapshotLister,
		csiSnapshotContentLister:  csiSnapshotContentLister,
		csiSnapshotClient:         csiSnapshotClient,
		metrics:                   metrics,
		helper:                    helper,
		// use variables to refer to these functions so they can be
		// replaced with fakes for testing.
		newPluginManager: newPluginManager,
		newBackupStore:   persistence.NewObjectBackupStore,

		clock: &clock.RealClock{},
	}

	c.syncHandler = c.processQueueItem
	c.processRequestFunc = c.processRequest

	deleteBackupRequestInformer.Informer().AddEventHandler(
		cache.ResourceEventHandlerFuncs{
			AddFunc: c.enqueue,
		},
	)

	c.resyncPeriod = time.Hour
	c.resyncFunc = c.deleteExpiredRequests

	return c
}

func (c *backupDeletionController) processQueueItem(key string) error {
	log := c.logger.WithField("key", key)
	log.Debug("Running processItem")

	ns, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		return errors.Wrap(err, "error splitting queue key")
	}

	req, err := c.deleteBackupRequestLister.DeleteBackupRequests(ns).Get(name)
	if apierrors.IsNotFound(err) {
		log.Debug("Unable to find DeleteBackupRequest")
		return nil
	}
	if err != nil {
		return errors.Wrap(err, "error getting DeleteBackupRequest")
	}

	switch req.Status.Phase {
	case velerov1api.DeleteBackupRequestPhaseProcessed:
		// Don't do anything because it's already been processed
	default:
		// Don't mutate the shared cache
		reqCopy := req.DeepCopy()
		return c.processRequestFunc(reqCopy)
	}

	return nil
}

func (c *backupDeletionController) processRequest(req *velerov1api.DeleteBackupRequest) error {
	log := c.logger.WithFields(logrus.Fields{
		"namespace": req.Namespace,
		"name":      req.Name,
		"backup":    req.Spec.BackupName,
	})

	var err error

	// Make sure we have the backup name
	if req.Spec.BackupName == "" {
		_, err = c.patchDeleteBackupRequest(req, func(r *velerov1api.DeleteBackupRequest) {
			r.Status.Phase = velerov1api.DeleteBackupRequestPhaseProcessed
			r.Status.Errors = []string{"spec.backupName is required"}
		})
		return err
	}

	// Remove any existing deletion requests for this backup so we only have
	// one at a time
	if errs := c.deleteExistingDeletionRequests(req, log); errs != nil {
		return kubeerrs.NewAggregate(errs)
	}

	// Don't allow deleting an in-progress backup
	if c.backupTracker.Contains(req.Namespace, req.Spec.BackupName) {
		_, err = c.patchDeleteBackupRequest(req, func(r *velerov1api.DeleteBackupRequest) {
			r.Status.Phase = velerov1api.DeleteBackupRequestPhaseProcessed
			r.Status.Errors = []string{"backup is still in progress"}
		})

		return err
	}

	// Get the backup we're trying to delete
	backup, err := c.backupClient.Backups(req.Namespace).Get(context.TODO(), req.Spec.BackupName, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		// Couldn't find backup - update status to Processed and record the not-found error
		req, err = c.patchDeleteBackupRequest(req, func(r *velerov1api.DeleteBackupRequest) {
			r.Status.Phase = velerov1api.DeleteBackupRequestPhaseProcessed
			r.Status.Errors = []string{"backup not found"}
		})

		return err
	}
	if err != nil {
		return errors.Wrap(err, "error getting backup")
	}

	// Don't allow deleting backups in read-only storage locations
	location := &velerov1api.BackupStorageLocation{}
	if err := c.kbClient.Get(context.Background(), client.ObjectKey{
		Namespace: backup.Namespace,
		Name:      backup.Spec.StorageLocation,
	}, location); err != nil {
		if apierrors.IsNotFound(err) {
			_, err := c.patchDeleteBackupRequest(req, func(r *velerov1api.DeleteBackupRequest) {
				r.Status.Phase = velerov1api.DeleteBackupRequestPhaseProcessed
				r.Status.Errors = append(r.Status.Errors, fmt.Sprintf("backup storage location %s not found", backup.Spec.StorageLocation))
			})
			return err
		}
		return errors.Wrap(err, "error getting backup storage location")
	}

	if location.Spec.AccessMode == velerov1api.BackupStorageLocationAccessModeReadOnly {
		_, err := c.patchDeleteBackupRequest(req, func(r *velerov1api.DeleteBackupRequest) {
			r.Status.Phase = velerov1api.DeleteBackupRequestPhaseProcessed
			r.Status.Errors = append(r.Status.Errors, fmt.Sprintf("cannot delete backup because backup storage location %s is currently in read-only mode", location.Name))
		})
		return err
	}

	// if the request object has no labels defined, initialise an empty map since
	// we will be updating labels
	if req.Labels == nil {
		req.Labels = map[string]string{}
	}

	// Update status to InProgress and set backup-name label if needed
	req, err = c.patchDeleteBackupRequest(req, func(r *velerov1api.DeleteBackupRequest) {
		r.Status.Phase = velerov1api.DeleteBackupRequestPhaseInProgress

		if req.Labels[velerov1api.BackupNameLabel] == "" {
			req.Labels[velerov1api.BackupNameLabel] = label.GetValidName(req.Spec.BackupName)
		}
	})
	if err != nil {
		return err
	}

	// Set backup-uid label if needed
	if req.Labels[velerov1api.BackupUIDLabel] == "" {
		req, err = c.patchDeleteBackupRequest(req, func(r *velerov1api.DeleteBackupRequest) {
			req.Labels[velerov1api.BackupUIDLabel] = string(backup.UID)
		})
		if err != nil {
			return err
		}
	}

	// Set backup status to Deleting
	backup, err = c.patchBackup(backup, func(b *velerov1api.Backup) {
		b.Status.Phase = velerov1api.BackupPhaseDeleting
	})
	if err != nil {
		log.WithError(errors.WithStack(err)).Error("Error setting backup phase to deleting")
		return err
	}

	backupScheduleName := backup.GetLabels()[velerov1api.ScheduleNameLabel]
	c.metrics.RegisterBackupDeletionAttempt(backupScheduleName)

	var errs []string

	pluginManager := c.newPluginManager(log)
	defer pluginManager.CleanupClients()

	backupStore, err := c.newBackupStore(location, pluginManager, log)
	if err != nil {
		errs = append(errs, err.Error())
	}

	actions, err := pluginManager.GetDeleteItemActions()
	log.Debugf("%d actions before invoking actions", len(actions))
	if err != nil {
		return errors.Wrap(err, "error getting delete item actions")
	}
	// don't defer CleanupClients here, since it was already called above.

	if len(actions) > 0 {
		// Download the tarball
		backupFile, err := downloadToTempFile(backup.Name, backupStore, log)
		defer closeAndRemoveFile(backupFile, c.logger)

		if err != nil {
			log.WithError(err).Errorf("Unable to download tarball for backup %s, skipping associated DeleteItemAction plugins", backup.Name)
		} else {
			ctx := &delete.Context{
				Backup:          backup,
				BackupReader:    backupFile,
				Actions:         actions,
				Log:             c.logger,
				DiscoveryHelper: c.helper,
				Filesystem:      filesystem.NewFileSystem(),
			}

			// Optimization: wrap in a gofunc? Would be useful for large backups with lots of objects.
			// but what do we do with the error returned? We can't just swallow it as that may lead to dangling resources.
			err = delete.InvokeDeleteActions(ctx)
			if err != nil {
				return errors.Wrap(err, "error invoking delete item actions")
			}
		}
	}

	if backupStore != nil {
		log.Info("Removing PV snapshots")

		if snapshots, err := backupStore.GetBackupVolumeSnapshots(backup.Name); err != nil {
			errs = append(errs, errors.Wrap(err, "error getting backup's volume snapshots").Error())
		} else {
			volumeSnapshotters := make(map[string]velero.VolumeSnapshotter)

			for _, snapshot := range snapshots {
				log.WithField("providerSnapshotID", snapshot.Status.ProviderSnapshotID).Info("Removing snapshot associated with backup")

				volumeSnapshotter, ok := volumeSnapshotters[snapshot.Spec.Location]
				if !ok {
					if volumeSnapshotter, err = volumeSnapshotterForSnapshotLocation(backup.Namespace, snapshot.Spec.Location, c.snapshotLocationLister, pluginManager); err != nil {
						errs = append(errs, err.Error())
						continue
					}
					volumeSnapshotters[snapshot.Spec.Location] = volumeSnapshotter
				}

				if err := volumeSnapshotter.DeleteSnapshot(snapshot.Status.ProviderSnapshotID); err != nil {
					errs = append(errs, errors.Wrapf(err, "error deleting snapshot %s", snapshot.Status.ProviderSnapshotID).Error())
				}
			}
		}
	}

	log.Info("Removing restic snapshots")
	if deleteErrs := c.deleteResticSnapshots(backup); len(deleteErrs) > 0 {
		for _, err := range deleteErrs {
			errs = append(errs, err.Error())
		}
	}

	if backupStore != nil {
		log.Info("Removing backup from backup storage")
		if err := backupStore.DeleteBackup(backup.Name); err != nil {
			errs = append(errs, err.Error())
		}
	}

	if features.IsEnabled(velerov1api.CSIFeatureFlag) {
		log.Info("Removing CSI volumesnapshots")
		if csiErrs := deleteCSIVolumeSnapshots(backup.Name, c.csiSnapshotLister, c.csiSnapshotClient.SnapshotV1beta1(), log); len(csiErrs) > 0 {
			for _, err := range csiErrs {
				errs = append(errs, err.Error())
			}
		}

		log.Info("Removing CSI volumesnapshotcontents")
		if csiErrs := deleteCSIVolumeSnapshotContents(backup.Name, c.csiSnapshotContentLister, c.csiSnapshotClient.SnapshotV1beta1(), log); len(csiErrs) > 0 {
			for _, err := range csiErrs {
				errs = append(errs, err.Error())
			}
		}
	}

	log.Info("Removing restores")
	if restores, err := c.restoreLister.Restores(backup.Namespace).List(labels.Everything()); err != nil {
		log.WithError(errors.WithStack(err)).Error("Error listing restore API objects")
	} else {
		for _, restore := range restores {
			if restore.Spec.BackupName != backup.Name {
				continue
			}

			restoreLog := log.WithField("restore", kube.NamespaceAndName(restore))

			restoreLog.Info("Deleting restore log/results from backup storage")
			if err := backupStore.DeleteRestore(restore.Name); err != nil {
				errs = append(errs, err.Error())
				// if we couldn't delete the restore files, don't delete the API object
				continue
			}

			restoreLog.Info("Deleting restore referencing backup")
			if err := c.restoreClient.Restores(restore.Namespace).Delete(context.TODO(), restore.Name, metav1.DeleteOptions{}); err != nil {
				errs = append(errs, errors.Wrapf(err, "error deleting restore %s", kube.NamespaceAndName(restore)).Error())
			}
		}
	}

	if len(errs) == 0 {
		// Only try to delete the backup object from kube if everything preceding went smoothly
		err = c.backupClient.Backups(backup.Namespace).Delete(context.TODO(), backup.Name, metav1.DeleteOptions{})
		if err != nil {
			errs = append(errs, errors.Wrapf(err, "error deleting backup %s", kube.NamespaceAndName(backup)).Error())
		}
	}

	if len(errs) == 0 {
		c.metrics.RegisterBackupDeletionSuccess(backupScheduleName)
	} else {
		c.metrics.RegisterBackupDeletionFailed(backupScheduleName)
	}

	// Update status to processed and record errors
	req, err = c.patchDeleteBackupRequest(req, func(r *velerov1api.DeleteBackupRequest) {
		r.Status.Phase = velerov1api.DeleteBackupRequestPhaseProcessed
		r.Status.Errors = errs
	})
	if err != nil {
		return err
	}

	// Everything deleted correctly, so we can delete all DeleteBackupRequests for this backup
	if len(errs) == 0 {
		listOptions := pkgbackup.NewDeleteBackupRequestListOptions(backup.Name, string(backup.UID))
		err = c.deleteBackupRequestClient.DeleteBackupRequests(req.Namespace).DeleteCollection(context.TODO(), metav1.DeleteOptions{}, listOptions)
		if err != nil {
			// If this errors, all we can do is log it.
			c.logger.WithField("backup", kube.NamespaceAndName(backup)).Error("error deleting all associated DeleteBackupRequests after successfully deleting the backup")
		}
	}

	return nil
}

func volumeSnapshotterForSnapshotLocation(
	namespace, snapshotLocationName string,
	snapshotLocationLister velerov1listers.VolumeSnapshotLocationLister,
	pluginManager clientmgmt.Manager,
) (velero.VolumeSnapshotter, error) {
	snapshotLocation, err := snapshotLocationLister.VolumeSnapshotLocations(namespace).Get(snapshotLocationName)
	if err != nil {
		return nil, errors.Wrapf(err, "error getting volume snapshot location %s", snapshotLocationName)
	}

	volumeSnapshotter, err := pluginManager.GetVolumeSnapshotter(snapshotLocation.Spec.Provider)
	if err != nil {
		return nil, errors.Wrapf(err, "error getting volume snapshotter for provider %s", snapshotLocation.Spec.Provider)
	}

	if err = volumeSnapshotter.Init(snapshotLocation.Spec.Config); err != nil {
		return nil, errors.Wrapf(err, "error initializing volume snapshotter for volume snapshot location %s", snapshotLocationName)
	}

	return volumeSnapshotter, nil
}

func (c *backupDeletionController) deleteExistingDeletionRequests(req *velerov1api.DeleteBackupRequest, log logrus.FieldLogger) []error {
	log.Info("Removing existing deletion requests for backup")
	selector := label.NewSelectorForBackup(req.Spec.BackupName)
	dbrs, err := c.deleteBackupRequestLister.DeleteBackupRequests(req.Namespace).List(selector)
	if err != nil {
		return []error{errors.Wrap(err, "error listing existing DeleteBackupRequests for backup")}
	}

	var errs []error
	for _, dbr := range dbrs {
		if dbr.Name == req.Name {
			continue
		}

		if err := c.deleteBackupRequestClient.DeleteBackupRequests(req.Namespace).Delete(context.TODO(), dbr.Name, metav1.DeleteOptions{}); err != nil {
			errs = append(errs, errors.WithStack(err))
		}
	}

	return errs
}

func (c *backupDeletionController) deleteResticSnapshots(backup *velerov1api.Backup) []error {
	if c.resticMgr == nil {
		return nil
	}

	snapshots, err := restic.GetSnapshotsInBackup(backup, c.podvolumeBackupLister)
	if err != nil {
		return []error{err}
	}

	ctx, cancelFunc := context.WithTimeout(context.Background(), resticTimeout)
	defer cancelFunc()

	var errs []error
	for _, snapshot := range snapshots {
		if err := c.resticMgr.Forget(ctx, snapshot); err != nil {
			errs = append(errs, err)
		}
	}

	return errs
}

func setVolumeSnapshotContentDeletionPolicy(vscName string, csiClient snapshotter.SnapshotV1beta1Interface, log *logrus.Entry) error {
	log.Infof("Setting DeletionPolicy of CSI volumesnapshotcontent %s to Delete", vscName)
	pb := []byte(`{"spec":{"deletionPolicy":"Delete"}}`)
	_, err := csiClient.VolumeSnapshotContents().Patch(context.TODO(), vscName, types.MergePatchType, pb, metav1.PatchOptions{})
	if err != nil {
		return err
	}
	return nil
}

func deleteCSIVolumeSnapshots(backupName string, csiSnapshotLister snapshotv1beta1listers.VolumeSnapshotLister,
	csiClient snapshotter.SnapshotV1beta1Interface, log *logrus.Entry) []error {
	errs := []error{}

	selector := label.NewSelectorForBackup(backupName)
	csiVolSnaps, err := csiSnapshotLister.List(selector)
	if err != nil {
		return []error{err}
	}

	log.Infof("Deleting %d CSI volumesnapshots", len(csiVolSnaps))
	for _, csiVS := range csiVolSnaps {
		log.Infof("Deleting CSI volumesnapshot %s/%s", csiVS.Namespace, csiVS.Name)
		if csiVS.Status != nil && csiVS.Status.BoundVolumeSnapshotContentName != nil {
			// we patch the DeletionPolicy of the volumesnapshotcontent to set it to Delete.
			// This ensures that the volume snapshot in the storage provider is also deleted.
			err := setVolumeSnapshotContentDeletionPolicy(*csiVS.Status.BoundVolumeSnapshotContentName, csiClient, log)
			if err != nil && !apierrors.IsNotFound(err) {
				log.Errorf("Skipping deletion of volumesnapshot %s/%s", csiVS.Namespace, csiVS.Name)
				errs = append(errs, err)
				continue
			}
		}
		err := csiClient.VolumeSnapshots(csiVS.Namespace).Delete(context.TODO(), csiVS.Name, metav1.DeleteOptions{})
		if err != nil {
			errs = append(errs, err)
		}
	}
	return errs
}

func deleteCSIVolumeSnapshotContents(backupName string, csiVSCLister snapshotv1beta1listers.VolumeSnapshotContentLister,
	csiClient snapshotter.SnapshotV1beta1Interface, log *logrus.Entry) []error {
	errs := []error{}
	selector := label.NewSelectorForBackup(backupName)
	csiVolSnapConts, err := csiVSCLister.List(selector)
	if err != nil {
		return []error{err}
	}
	// It is possible that by the time deleteCSIVolumeSnapshotContents is called after deleteCSIVolumeSnapshots
	// that deletion of VSCs hasn't been completed, by the snapshot-controller (one of the CSI components).
	// For that reason the csiVSCLister returned VSCs that are yet to be deleted. To handle this scenario,
	// we swallow `IsNotFound` errors from the setVolumeSnapshotContentDeletionPolicy function and the
	// csiClient.VolumeSnapshotContents().Delete(...)
	log.Infof("Deleting %d CSI volumesnapshotcontents", len(csiVolSnapConts))
	for _, snapCont := range csiVolSnapConts {
		err := setVolumeSnapshotContentDeletionPolicy(snapCont.Name, csiClient, log)
		if err != nil && !apierrors.IsNotFound(err) {
			log.Errorf("Failed to set DeletionPolicy on volumesnapshotcontent %s. Skipping deletion", snapCont.Name)
			errs = append(errs, err)
			continue
		}
		if apierrors.IsNotFound(err) {
			log.Infof("volumesnapshotcontent %s not found", snapCont.Name)
			continue
		}
		log.Infof("Deleting volumesnapshotcontent %s", snapCont.Name)
		err = csiClient.VolumeSnapshotContents().Delete(context.TODO(), snapCont.Name, metav1.DeleteOptions{})
		if err != nil && !apierrors.IsNotFound(err) {
			errs = append(errs, err)
		}
	}

	return errs
}

const deleteBackupRequestMaxAge = 24 * time.Hour

func (c *backupDeletionController) deleteExpiredRequests() {
	c.logger.Info("Checking for expired DeleteBackupRequests")
	defer c.logger.Info("Done checking for expired DeleteBackupRequests")

	// Our shared informer factory filters on a single namespace, so asking for all is ok here.
	requests, err := c.deleteBackupRequestLister.List(labels.Everything())
	if err != nil {
		c.logger.WithError(err).Error("unable to check for expired DeleteBackupRequests")
		return
	}

	now := c.clock.Now()

	for _, req := range requests {
		if req.Status.Phase != velerov1api.DeleteBackupRequestPhaseProcessed {
			continue
		}

		age := now.Sub(req.CreationTimestamp.Time)
		if age >= deleteBackupRequestMaxAge {
			reqLog := c.logger.WithFields(logrus.Fields{"namespace": req.Namespace, "name": req.Name})
			reqLog.Info("Deleting expired DeleteBackupRequest")

			err = c.deleteBackupRequestClient.DeleteBackupRequests(req.Namespace).Delete(context.TODO(), req.Name, metav1.DeleteOptions{})
			if err != nil {
				reqLog.WithError(err).Error("Error deleting DeleteBackupRequest")
			}
		}
	}
}

func (c *backupDeletionController) patchDeleteBackupRequest(req *velerov1api.DeleteBackupRequest, mutate func(*velerov1api.DeleteBackupRequest)) (*velerov1api.DeleteBackupRequest, error) {
	// Record original json
	oldData, err := json.Marshal(req)
	if err != nil {
		return nil, errors.Wrap(err, "error marshalling original DeleteBackupRequest")
	}

	// Mutate
	mutate(req)

	// Record new json
	newData, err := json.Marshal(req)
	if err != nil {
		return nil, errors.Wrap(err, "error marshalling updated DeleteBackupRequest")
	}

	patchBytes, err := jsonpatch.CreateMergePatch(oldData, newData)
	if err != nil {
		return nil, errors.Wrap(err, "error creating json merge patch for DeleteBackupRequest")
	}

	req, err = c.deleteBackupRequestClient.DeleteBackupRequests(req.Namespace).Patch(context.TODO(), req.Name, types.MergePatchType, patchBytes, metav1.PatchOptions{})
	if err != nil {
		return nil, errors.Wrap(err, "error patching DeleteBackupRequest")
	}

	return req, nil
}

func (c *backupDeletionController) patchBackup(backup *velerov1api.Backup, mutate func(*velerov1api.Backup)) (*velerov1api.Backup, error) {
	// Record original json
	oldData, err := json.Marshal(backup)
	if err != nil {
		return nil, errors.Wrap(err, "error marshalling original Backup")
	}

	// Mutate
	mutate(backup)

	// Record new json
	newData, err := json.Marshal(backup)
	if err != nil {
		return nil, errors.Wrap(err, "error marshalling updated Backup")
	}

	patchBytes, err := jsonpatch.CreateMergePatch(oldData, newData)
	if err != nil {
		return nil, errors.Wrap(err, "error creating json merge patch for Backup")
	}

	backup, err = c.backupClient.Backups(backup.Namespace).Patch(context.TODO(), backup.Name, types.MergePatchType, patchBytes, metav1.PatchOptions{})
	if err != nil {
		return nil, errors.Wrap(err, "error patching Backup")
	}

	return backup, nil
}
