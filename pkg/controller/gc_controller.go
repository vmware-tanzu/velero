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
	"time"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/clock"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/tools/cache"

	api "github.com/heptio/ark/pkg/apis/ark/v1"
	"github.com/heptio/ark/pkg/cloudprovider"
	arkv1client "github.com/heptio/ark/pkg/generated/clientset/versioned/typed/ark/v1"
	informers "github.com/heptio/ark/pkg/generated/informers/externalversions/ark/v1"
	listers "github.com/heptio/ark/pkg/generated/listers/ark/v1"
	"github.com/heptio/ark/pkg/util/kube"
)

// gcController removes expired backup content from object storage.
type gcController struct {
	backupService       cloudprovider.BackupService
	snapshotService     cloudprovider.SnapshotService
	bucket              string
	syncPeriod          time.Duration
	clock               clock.Clock
	backupLister        listers.BackupLister
	backupListerSynced  cache.InformerSynced
	backupClient        arkv1client.BackupsGetter
	restoreLister       listers.RestoreLister
	restoreListerSynced cache.InformerSynced
	restoreClient       arkv1client.RestoresGetter
	logger              *logrus.Logger
}

// NewGCController constructs a new gcController.
func NewGCController(
	backupService cloudprovider.BackupService,
	snapshotService cloudprovider.SnapshotService,
	bucket string,
	syncPeriod time.Duration,
	backupInformer informers.BackupInformer,
	backupClient arkv1client.BackupsGetter,
	restoreInformer informers.RestoreInformer,
	restoreClient arkv1client.RestoresGetter,
	logger *logrus.Logger,
) Interface {
	if syncPeriod < time.Minute {
		logger.WithField("syncPeriod", syncPeriod).Info("Provided GC sync period is too short. Setting to 1 minute")
		syncPeriod = time.Minute
	}

	return &gcController{
		backupService:       backupService,
		snapshotService:     snapshotService,
		bucket:              bucket,
		syncPeriod:          syncPeriod,
		clock:               clock.RealClock{},
		backupLister:        backupInformer.Lister(),
		backupListerSynced:  backupInformer.Informer().HasSynced,
		backupClient:        backupClient,
		restoreLister:       restoreInformer.Lister(),
		restoreListerSynced: restoreInformer.Informer().HasSynced,
		restoreClient:       restoreClient,
		logger:              logger,
	}
}

var _ Interface = &gcController{}

// Run is a blocking function that runs a single worker to garbage-collect backups
// from object/block storage and the Ark API. It will return when it receives on the
// ctx.Done() channel.
func (c *gcController) Run(ctx context.Context, workers int) error {
	c.logger.Info("Waiting for caches to sync")
	if !cache.WaitForCacheSync(ctx.Done(), c.backupListerSynced, c.restoreListerSynced) {
		return errors.New("timed out waiting for caches to sync")
	}
	c.logger.Info("Caches are synced")

	wait.Until(c.run, c.syncPeriod, ctx.Done())
	return nil
}

func (c *gcController) run() {
	c.processBackups()
}

// garbageCollectBackup removes an expired backup by deleting any associated backup files (if
// deleteBackupFiles = true), volume snapshots, restore API objects, and the backup API object
// itself.
func (c *gcController) garbageCollectBackup(backup *api.Backup, deleteBackupFiles bool) {
	logContext := c.logger.WithField("backup", kube.NamespaceAndName(backup))

	// if the backup includes snapshots but we don't currently have a PVProvider, we don't
	// want to orphan the snapshots so skip garbage-collection entirely.
	if c.snapshotService == nil && len(backup.Status.VolumeBackups) > 0 {
		logContext.Warning("Cannot garbage-collect backup because backup includes snapshots and server is not configured with PersistentVolumeProvider")
		return
	}

	// The GC process is primarily intended to delete expired cloud resources (file in object
	// storage, snapshots). If we fail to delete any of these, we don't delete the Backup API
	// object or metadata file in object storage so that we don't orphan the cloud resources.
	deletionFailure := false

	for _, volumeBackup := range backup.Status.VolumeBackups {
		logContext.WithField("snapshotID", volumeBackup.SnapshotID).Info("Removing snapshot associated with backup")
		if err := c.snapshotService.DeleteSnapshot(volumeBackup.SnapshotID); err != nil {
			logContext.WithError(err).WithField("snapshotID", volumeBackup.SnapshotID).Error("Error deleting snapshot")
			deletionFailure = true
		}
	}

	// If applicable, delete everything in the backup dir in object storage *before* deleting the API object
	// because otherwise the backup sync controller could re-sync the backup from object storage.
	if deleteBackupFiles {
		logContext.Info("Removing backup from object storage")
		if err := c.backupService.DeleteBackupDir(c.bucket, backup.Name); err != nil {
			logContext.WithError(err).Error("Error deleting backup")
			deletionFailure = true
		}
	}

	logContext.Info("Getting restore API objects referencing backup")
	if restores, err := c.restoreLister.Restores(backup.Namespace).List(labels.Everything()); err != nil {
		logContext.WithError(errors.WithStack(err)).Error("Error getting Restore API objects")
	} else {
		for _, restore := range restores {
			if restore.Spec.BackupName == backup.Name {
				logContext.WithField("restore", kube.NamespaceAndName(restore)).Info("Removing Restore API object referencing Backup")
				if err := c.restoreClient.Restores(restore.Namespace).Delete(restore.Name, &metav1.DeleteOptions{}); err != nil {
					logContext.WithError(errors.WithStack(err)).WithField("restore", kube.NamespaceAndName(restore)).
						Error("Error deleting Restore API object")
				}
			}
		}
	}

	if deletionFailure {
		logContext.Warning("Backup will not be deleted due to errors deleting related object storage files(s) and/or volume snapshots")
		return
	}

	logContext.Info("Removing Backup API object")
	if err := c.backupClient.Backups(backup.Namespace).Delete(backup.Name, &metav1.DeleteOptions{}); err != nil {
		logContext.WithError(errors.WithStack(err)).Error("Error deleting Backup API object")
	}
}

// garbageCollectBackups checks backups for expiration and triggers garbage-collection for the expired
// ones.
func (c *gcController) garbageCollectBackups(backups []*api.Backup, expiration time.Time, deleteBackupFiles bool) {
	for _, backup := range backups {
		if backup.Status.Expiration.Time.After(expiration) {
			c.logger.WithField("backup", kube.NamespaceAndName(backup)).Info("Backup has not expired yet, skipping")
			continue
		}

		c.garbageCollectBackup(backup, deleteBackupFiles)
	}
}

// processBackups gets backups from object storage and the API and submits
// them for garbage-collection.
func (c *gcController) processBackups() {
	now := c.clock.Now()
	c.logger.WithField("now", now).Info("Garbage-collecting backups that have expired as of now")

	// GC backups in object storage. We do this in addition
	// to GC'ing API objects to prevent orphan backup files.
	backups, err := c.backupService.GetAllBackups(c.bucket)
	if err != nil {
		c.logger.WithError(err).Error("Error getting all backups from object storage")
		return
	}
	c.garbageCollectBackups(backups, now, true)

	// GC backups without files in object storage
	apiBackups, err := c.backupLister.List(labels.Everything())
	if err != nil {
		c.logger.WithError(errors.WithStack(err)).Error("Error getting all Backup API objects")
		return
	}
	c.garbageCollectBackups(apiBackups, now, false)
}
