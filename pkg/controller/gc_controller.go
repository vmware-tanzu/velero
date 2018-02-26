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
	"context"
	"encoding/json"
	"time"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/clock"
	kerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/tools/cache"
	"k8s.io/kubernetes/pkg/util/version"

	api "github.com/heptio/ark/pkg/apis/ark/v1"
	"github.com/heptio/ark/pkg/cloudprovider"
	arkv1client "github.com/heptio/ark/pkg/generated/clientset/versioned/typed/ark/v1"
	informers "github.com/heptio/ark/pkg/generated/informers/externalversions/ark/v1"
	listers "github.com/heptio/ark/pkg/generated/listers/ark/v1"
	"github.com/heptio/ark/pkg/util/kube"
	"github.com/heptio/ark/pkg/util/stringslice"
)

// MinVersionForDelete is the minimum Kubernetes server version that Ark
// requires in order to be able to properly delete backups (including
// the associated snapshots and object storage files). This is because
// Ark uses finalizers on the backup CRD to implement garbage-collection
// and deletion.
var MinVersionForDelete = version.MustParseSemantic("1.7.5")

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
	logger              logrus.FieldLogger
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
	logger logrus.FieldLogger,
) Interface {
	if syncPeriod < time.Minute {
		logger.WithField("syncPeriod", syncPeriod).Info("Provided GC sync period is too short. Setting to 1 minute")
		syncPeriod = time.Minute
	}

	c := &gcController{
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

	backupInformer.Informer().AddEventHandler(
		cache.ResourceEventHandlerFuncs{
			UpdateFunc: c.handleFinalizer,
		},
	)

	return c
}

// handleFinalizer runs garbage-collection on a backup that has the Ark GC
// finalizer and a deletionTimestamp.
func (c *gcController) handleFinalizer(_, newObj interface{}) {
	var (
		backup = newObj.(*api.Backup)
		log    = c.logger.WithField("backup", kube.NamespaceAndName(backup))
	)

	// we're only interested in backups that have a deletionTimestamp and at
	// least one finalizer.
	if backup.DeletionTimestamp == nil || len(backup.Finalizers) == 0 {
		return
	}
	log.Debugf("Backup has finalizers %s", backup.Finalizers)

	if !stringslice.Has(backup.Finalizers, api.GCFinalizer) {
		return
	}

	log.Infof("Garbage-collecting backup")
	if err := c.garbageCollect(backup, log); err != nil {
		// if there were errors deleting related cloud resources, don't
		// delete the backup API object because we don't want to orphan
		// the cloud resources.
		log.WithError(err).Error("Error deleting backup's related objects")
		return
	}

	patchMap := map[string]interface{}{
		"metadata": map[string]interface{}{
			"finalizers":      stringslice.Except(backup.Finalizers, api.GCFinalizer),
			"resourceVersion": backup.ResourceVersion,
		},
	}

	patchBytes, err := json.Marshal(patchMap)
	if err != nil {
		log.WithError(err).Error("Error marshaling finalizers patch")
		return
	}

	if _, err = c.backupClient.Backups(backup.Namespace).Patch(backup.Name, types.MergePatchType, patchBytes); err != nil {
		log.WithError(errors.WithStack(err)).Error("Error patching backup")
	}
}

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
	now := c.clock.Now()
	c.logger.Info("Garbage-collecting expired backups")

	// Go thru API objects and delete expired ones (finalizer will GC their
	// corresponding files/snapshots/restores). Note that we're ignoring backups
	// in object storage that haven't been synced to Kubernetes yet; they'll
	// be processed for GC (if applicable) once they've been synced.
	backups, err := c.backupLister.List(labels.Everything())
	if err != nil {
		c.logger.WithError(errors.WithStack(err)).Error("Error getting all backups")
		return
	}

	for _, backup := range backups {
		log := c.logger.WithField("backup", kube.NamespaceAndName(backup))
		if backup.Status.Expiration.Time.After(now) {
			log.Debug("Backup has not expired yet, skipping")
			continue
		}

		// since backups have a finalizer, this will actually have the effect of setting a deletionTimestamp and calling
		// an update. The update will be handled by this controller and will result in a deletion of the obj storage
		// files and the API object.
		if err := c.backupClient.Backups(backup.Namespace).Delete(backup.Name, &metav1.DeleteOptions{}); err != nil {
			log.WithError(errors.WithStack(err)).Error("Error deleting backup")
		}
	}
}

// garbageCollect prepares for deleting an expired backup by deleting any
// associated backup files, volume snapshots, or restore API objects.
func (c *gcController) garbageCollect(backup *api.Backup, log logrus.FieldLogger) error {
	// if the backup includes snapshots but we don't currently have a PVProvider, we don't
	// want to orphan the snapshots so skip garbage-collection entirely.
	if c.snapshotService == nil && len(backup.Status.VolumeBackups) > 0 {
		return errors.New("cannot garbage-collect backup because it includes snapshots and Ark is not configured with a PersistentVolumeProvider")
	}

	var errs []error

	for _, volumeBackup := range backup.Status.VolumeBackups {
		log.WithField("snapshotID", volumeBackup.SnapshotID).Info("Removing snapshot associated with backup")
		if err := c.snapshotService.DeleteSnapshot(volumeBackup.SnapshotID); err != nil {
			errs = append(errs, errors.Wrapf(err, "error deleting snapshot %s", volumeBackup.SnapshotID))
		}
	}

	log.Info("Removing backup from object storage")
	if err := c.backupService.DeleteBackupDir(c.bucket, backup.Name); err != nil {
		errs = append(errs, errors.Wrap(err, "error deleting backup from object storage"))
	}

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
				restoreLog.WithError(errors.WithStack(err)).Error("Error deleting restore")
			}
		}
	}

	return kerrors.NewAggregate(errs)
}
