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
	"errors"
	"time"

	"github.com/golang/glog"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/clock"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/tools/cache"

	"github.com/heptio/ark/pkg/cloudprovider"
	arkv1client "github.com/heptio/ark/pkg/generated/clientset/typed/ark/v1"
	informers "github.com/heptio/ark/pkg/generated/informers/externalversions/ark/v1"
	listers "github.com/heptio/ark/pkg/generated/listers/ark/v1"
)

// gcController removes expired backup content from object storage.
type gcController struct {
	backupService   cloudprovider.BackupService
	snapshotService cloudprovider.SnapshotService
	bucket          string
	syncPeriod      time.Duration
	clock           clock.Clock
	lister          listers.BackupLister
	listerSynced    cache.InformerSynced
	client          arkv1client.BackupsGetter
}

// NewGCController constructs a new gcController.
func NewGCController(
	backupService cloudprovider.BackupService,
	snapshotService cloudprovider.SnapshotService,
	bucket string,
	syncPeriod time.Duration,
	backupInformer informers.BackupInformer,
	client arkv1client.BackupsGetter,
) Interface {
	if syncPeriod < time.Minute {
		glog.Infof("GC sync period %v is too short. Setting to 1 minute", syncPeriod)
		syncPeriod = time.Minute
	}

	return &gcController{
		backupService:   backupService,
		snapshotService: snapshotService,
		bucket:          bucket,
		syncPeriod:      syncPeriod,
		clock:           clock.RealClock{},
		lister:          backupInformer.Lister(),
		listerSynced:    backupInformer.Informer().HasSynced,
		client:          client,
	}
}

var _ Interface = &gcController{}

// Run is a blocking function that runs a single worker to garbage-collect backups
// from object/block storage and the Ark API. It will return when it receives on the
// ctx.Done() channel.
func (c *gcController) Run(ctx context.Context, workers int) error {
	glog.Info("Waiting for caches to sync")
	if !cache.WaitForCacheSync(ctx.Done(), c.listerSynced) {
		return errors.New("timed out waiting for caches to sync")
	}
	glog.Info("Caches are synced")

	wait.Until(c.run, c.syncPeriod, ctx.Done())
	return nil
}

func (c *gcController) run() {
	c.cleanBackups()
}

// cleanBackups deletes expired backups.
func (c *gcController) cleanBackups() {
	backups, err := c.backupService.GetAllBackups(c.bucket)
	if err != nil {
		glog.Errorf("error getting all backups: %v", err)
		return
	}

	now := c.clock.Now()
	glog.Infof("garbage-collecting backups that have expired as of %v", now)

	// GC backup files and associated snapshots/API objects. Note that deletion from object
	// storage should happen first because otherwise there's a possibility the backup sync
	// controller would re-create the API object after deletion.
	for _, backup := range backups {
		if backup.Status.Expiration.Time.Before(now) {
			glog.Infof("Removing backup %s/%s", backup.Namespace, backup.Name)
			if err := c.backupService.DeleteBackup(c.bucket, backup.Name); err != nil {
				glog.Errorf("error deleting backup %s/%s: %v", backup.Namespace, backup.Name, err)
			}

			for _, volumeBackup := range backup.Status.VolumeBackups {
				glog.Infof("Removing snapshot %s associated with backup %s/%s", volumeBackup.SnapshotID, backup.Namespace, backup.Name)
				if err := c.snapshotService.DeleteSnapshot(volumeBackup.SnapshotID); err != nil {
					glog.Errorf("error deleting snapshot %v: %v", volumeBackup.SnapshotID, err)
				}
			}

			glog.Infof("Removing backup API object %s/%s", backup.Namespace, backup.Name)
			if err := c.client.Backups(backup.Namespace).Delete(backup.Name, &metav1.DeleteOptions{}); err != nil {
				glog.Errorf("error deleting backup API object %s/%s: %v", backup.Namespace, backup.Name, err)
			}
		} else {
			glog.Infof("Backup %s/%s has not expired yet, skipping", backup.Namespace, backup.Name)
		}
	}

	// also GC any Backup API objects without files in object storage
	apiBackups, err := c.lister.List(labels.NewSelector())
	if err != nil {
		glog.Errorf("error getting all backup API objects: %v", err)
	}

	for _, backup := range apiBackups {
		if backup.Status.Expiration.Time.Before(now) {
			glog.Infof("Removing backup API object %s/%s", backup.Namespace, backup.Name)
			if err := c.client.Backups(backup.Namespace).Delete(backup.Name, &metav1.DeleteOptions{}); err != nil {
				glog.Errorf("error deleting backup API object %s/%s: %v", backup.Namespace, backup.Name, err)
			}
		} else {
			glog.Infof("Backup %s/%s has not expired yet, skipping", backup.Namespace, backup.Name)
		}
	}
}
