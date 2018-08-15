/*
Copyright 2018 the Heptio Ark contributors.

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
	"time"

	jsonpatch "github.com/evanphx/json-patch"
	"github.com/heptio/ark/pkg/apis/ark/v1"
	pkgbackup "github.com/heptio/ark/pkg/backup"
	"github.com/heptio/ark/pkg/cloudprovider"
	arkv1client "github.com/heptio/ark/pkg/generated/clientset/versioned/typed/ark/v1"
	informers "github.com/heptio/ark/pkg/generated/informers/externalversions/ark/v1"
	listers "github.com/heptio/ark/pkg/generated/listers/ark/v1"
	"github.com/heptio/ark/pkg/plugin"
	"github.com/heptio/ark/pkg/restic"
	"github.com/heptio/ark/pkg/util/kube"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/clock"
	kubeerrs "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/client-go/tools/cache"
)

const resticTimeout = time.Minute

type backupDeletionController struct {
	*genericController

	deleteBackupRequestClient arkv1client.DeleteBackupRequestsGetter
	deleteBackupRequestLister listers.DeleteBackupRequestLister
	backupClient              arkv1client.BackupsGetter
	blockStore                cloudprovider.BlockStore
	restoreLister             listers.RestoreLister
	restoreClient             arkv1client.RestoresGetter
	backupTracker             BackupTracker
	resticMgr                 restic.RepositoryManager
	podvolumeBackupLister     listers.PodVolumeBackupLister
	backupLocationLister      listers.BackupStorageLocationLister
	pluginRegistry            plugin.Registry
	deleteBackupDir           cloudprovider.DeleteBackupDirFunc
	processRequestFunc        func(*v1.DeleteBackupRequest) error
	clock                     clock.Clock
	newPluginManager          func(logger logrus.FieldLogger, logLevel logrus.Level, pluginRegistry plugin.Registry) plugin.Manager
}

// NewBackupDeletionController creates a new backup deletion controller.
func NewBackupDeletionController(
	logger logrus.FieldLogger,
	deleteBackupRequestInformer informers.DeleteBackupRequestInformer,
	deleteBackupRequestClient arkv1client.DeleteBackupRequestsGetter,
	backupClient arkv1client.BackupsGetter,
	blockStore cloudprovider.BlockStore,
	restoreInformer informers.RestoreInformer,
	restoreClient arkv1client.RestoresGetter,
	backupTracker BackupTracker,
	resticMgr restic.RepositoryManager,
	podvolumeBackupInformer informers.PodVolumeBackupInformer,
	backupLocationInformer informers.BackupStorageLocationInformer,
	pluginRegistry plugin.Registry,
) Interface {
	c := &backupDeletionController{
		genericController:         newGenericController("backup-deletion", logger),
		deleteBackupRequestClient: deleteBackupRequestClient,
		deleteBackupRequestLister: deleteBackupRequestInformer.Lister(),
		backupClient:              backupClient,
		blockStore:                blockStore,
		restoreLister:             restoreInformer.Lister(),
		restoreClient:             restoreClient,
		backupTracker:             backupTracker,
		resticMgr:                 resticMgr,
		podvolumeBackupLister:     podvolumeBackupInformer.Lister(),
		backupLocationLister:      backupLocationInformer.Lister(),
		pluginRegistry:            pluginRegistry,

		// use variables to refer to these functions so they can be
		// replaced with fakes for testing.
		deleteBackupDir:  cloudprovider.DeleteBackupDir,
		newPluginManager: plugin.NewManager,

		clock: &clock.RealClock{},
	}

	c.syncHandler = c.processQueueItem
	c.cacheSyncWaiters = append(
		c.cacheSyncWaiters,
		deleteBackupRequestInformer.Informer().HasSynced,
		restoreInformer.Informer().HasSynced,
		podvolumeBackupInformer.Informer().HasSynced,
		backupLocationInformer.Informer().HasSynced,
	)
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
	case v1.DeleteBackupRequestPhaseProcessed:
		// Don't do anything because it's already been processed
	default:
		// Don't mutate the shared cache
		reqCopy := req.DeepCopy()
		return c.processRequestFunc(reqCopy)
	}

	return nil
}

func (c *backupDeletionController) processRequest(req *v1.DeleteBackupRequest) error {
	log := c.logger.WithFields(logrus.Fields{
		"namespace": req.Namespace,
		"name":      req.Name,
		"backup":    req.Spec.BackupName,
	})

	var err error

	// Make sure we have the backup name
	if req.Spec.BackupName == "" {
		_, err = c.patchDeleteBackupRequest(req, func(r *v1.DeleteBackupRequest) {
			r.Status.Phase = v1.DeleteBackupRequestPhaseProcessed
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
		_, err = c.patchDeleteBackupRequest(req, func(r *v1.DeleteBackupRequest) {
			r.Status.Phase = v1.DeleteBackupRequestPhaseProcessed
			r.Status.Errors = []string{"backup is still in progress"}
		})

		return err
	}

	// Update status to InProgress and set backup-name label if needed
	req, err = c.patchDeleteBackupRequest(req, func(r *v1.DeleteBackupRequest) {
		r.Status.Phase = v1.DeleteBackupRequestPhaseInProgress

		if req.Labels[v1.BackupNameLabel] == "" {
			req.Labels[v1.BackupNameLabel] = req.Spec.BackupName
		}
	})
	if err != nil {
		return err
	}

	// Get the backup we're trying to delete
	backup, err := c.backupClient.Backups(req.Namespace).Get(req.Spec.BackupName, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		// Couldn't find backup - update status to Processed and record the not-found error
		req, err = c.patchDeleteBackupRequest(req, func(r *v1.DeleteBackupRequest) {
			r.Status.Phase = v1.DeleteBackupRequestPhaseProcessed
			r.Status.Errors = []string{"backup not found"}
		})

		return err
	}
	if err != nil {
		return errors.Wrap(err, "error getting Backup")
	}

	// Set backup-uid label if needed
	if req.Labels[v1.BackupUIDLabel] == "" {
		req, err = c.patchDeleteBackupRequest(req, func(r *v1.DeleteBackupRequest) {
			req.Labels[v1.BackupUIDLabel] = string(backup.UID)
		})
		if err != nil {
			return err
		}
	}

	// If the backup includes snapshots but we don't currently have a PVProvider, we don't
	// want to orphan the snapshots so skip deletion.
	if c.blockStore == nil && len(backup.Status.VolumeBackups) > 0 {
		req, err = c.patchDeleteBackupRequest(req, func(r *v1.DeleteBackupRequest) {
			r.Status.Phase = v1.DeleteBackupRequestPhaseProcessed
			r.Status.Errors = []string{"unable to delete backup because it includes PV snapshots and Ark is not configured with a PersistentVolumeProvider"}
		})

		return err
	}

	// Set backup status to Deleting
	backup, err = c.patchBackup(backup, func(b *v1.Backup) {
		b.Status.Phase = v1.BackupPhaseDeleting
	})
	if err != nil {
		log.WithError(errors.WithStack(err)).Error("Error setting backup phase to deleting")
		return err
	}

	var errs []string

	log.Info("Removing PV snapshots")
	for _, volumeBackup := range backup.Status.VolumeBackups {
		log.WithField("snapshotID", volumeBackup.SnapshotID).Info("Removing snapshot associated with backup")
		if err := c.blockStore.DeleteSnapshot(volumeBackup.SnapshotID); err != nil {
			errs = append(errs, errors.Wrapf(err, "error deleting snapshot %s", volumeBackup.SnapshotID).Error())
		}
	}

	log.Info("Removing restic snapshots")
	if deleteErrs := c.deleteResticSnapshots(backup); len(deleteErrs) > 0 {
		for _, err := range deleteErrs {
			errs = append(errs, err.Error())
		}
	}

	log.Info("Removing backup from backup storage")
	if err := c.deleteBackupFromStorage(backup, log); err != nil {
		errs = append(errs, err.Error())
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

			restoreLog.Info("Deleting restore referencing backup")
			if err := c.restoreClient.Restores(restore.Namespace).Delete(restore.Name, &metav1.DeleteOptions{}); err != nil {
				errs = append(errs, errors.Wrapf(err, "error deleting restore %s", kube.NamespaceAndName(restore)).Error())
			}
		}
	}

	if len(errs) == 0 {
		// Only try to delete the backup object from kube if everything preceding went smoothly
		err = c.backupClient.Backups(backup.Namespace).Delete(backup.Name, nil)
		if err != nil {
			errs = append(errs, errors.Wrapf(err, "error deleting backup %s", kube.NamespaceAndName(backup)).Error())
		}
	}

	// Update status to processed and record errors
	req, err = c.patchDeleteBackupRequest(req, func(r *v1.DeleteBackupRequest) {
		r.Status.Phase = v1.DeleteBackupRequestPhaseProcessed
		r.Status.Errors = errs
	})
	if err != nil {
		return err
	}

	// Everything deleted correctly, so we can delete all DeleteBackupRequests for this backup
	if len(errs) == 0 {
		listOptions := pkgbackup.NewDeleteBackupRequestListOptions(backup.Name, string(backup.UID))
		err = c.deleteBackupRequestClient.DeleteBackupRequests(req.Namespace).DeleteCollection(nil, listOptions)
		if err != nil {
			// If this errors, all we can do is log it.
			c.logger.WithField("backup", kube.NamespaceAndName(backup)).Error("error deleting all associated DeleteBackupRequests after successfully deleting the backup")
		}
	}

	return nil
}

func (c *backupDeletionController) deleteBackupFromStorage(backup *v1.Backup, log *logrus.Entry) error {
	pluginManager := c.newPluginManager(log, log.Level, c.pluginRegistry)
	defer pluginManager.CleanupClients()

	backupLocation, err := c.backupLocationLister.BackupStorageLocations(backup.Namespace).Get(backup.Spec.StorageLocation)
	if err != nil {
		return errors.WithStack(err)
	}

	objectStore, err := getObjectStoreForLocation(backupLocation, pluginManager)
	if err != nil {
		return err
	}

	if err := c.deleteBackupDir(log, objectStore, backupLocation.Spec.ObjectStorage.Bucket, backup.Name); err != nil {
		return errors.Wrap(err, "error deleting backup from backup storage")
	}

	return nil
}

func (c *backupDeletionController) deleteExistingDeletionRequests(req *v1.DeleteBackupRequest, log logrus.FieldLogger) []error {
	log.Info("Removing existing deletion requests for backup")
	selector := labels.SelectorFromSet(labels.Set(map[string]string{
		v1.BackupNameLabel: req.Spec.BackupName,
	}))
	dbrs, err := c.deleteBackupRequestLister.DeleteBackupRequests(req.Namespace).List(selector)
	if err != nil {
		return []error{errors.Wrap(err, "error listing existing DeleteBackupRequests for backup")}
	}

	var errs []error
	for _, dbr := range dbrs {
		if dbr.Name == req.Name {
			continue
		}

		if err := c.deleteBackupRequestClient.DeleteBackupRequests(req.Namespace).Delete(dbr.Name, nil); err != nil {
			errs = append(errs, errors.WithStack(err))
		}
	}

	return errs
}

func (c *backupDeletionController) deleteResticSnapshots(backup *v1.Backup) []error {
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
		if req.Status.Phase != v1.DeleteBackupRequestPhaseProcessed {
			continue
		}

		age := now.Sub(req.CreationTimestamp.Time)
		if age >= deleteBackupRequestMaxAge {
			reqLog := c.logger.WithFields(logrus.Fields{"namespace": req.Namespace, "name": req.Name})
			reqLog.Info("Deleting expired DeleteBackupRequest")

			err = c.deleteBackupRequestClient.DeleteBackupRequests(req.Namespace).Delete(req.Name, nil)
			if err != nil {
				reqLog.WithError(err).Error("Error deleting DeleteBackupRequest")
			}
		}
	}
}

func (c *backupDeletionController) patchDeleteBackupRequest(req *v1.DeleteBackupRequest, mutate func(*v1.DeleteBackupRequest)) (*v1.DeleteBackupRequest, error) {
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

	req, err = c.deleteBackupRequestClient.DeleteBackupRequests(req.Namespace).Patch(req.Name, types.MergePatchType, patchBytes)
	if err != nil {
		return nil, errors.Wrap(err, "error patching DeleteBackupRequest")
	}

	return req, nil
}

func (c *backupDeletionController) patchBackup(backup *v1.Backup, mutate func(*v1.Backup)) (*v1.Backup, error) {
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

	backup, err = c.backupClient.Backups(backup.Namespace).Patch(backup.Name, types.MergePatchType, patchBytes)
	if err != nil {
		return nil, errors.Wrap(err, "error patching Backup")
	}

	return backup, nil
}
