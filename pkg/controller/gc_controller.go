/*
Copyright 2017 the Velero contributors.

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
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/clock"
	"k8s.io/client-go/tools/cache"

	"sigs.k8s.io/controller-runtime/pkg/client"

	velerov1api "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	pkgbackup "github.com/vmware-tanzu/velero/pkg/backup"
	velerov1client "github.com/vmware-tanzu/velero/pkg/generated/clientset/versioned/typed/velero/v1"
	velerov1informers "github.com/vmware-tanzu/velero/pkg/generated/informers/externalversions/velero/v1"
	velerov1listers "github.com/vmware-tanzu/velero/pkg/generated/listers/velero/v1"
	"github.com/vmware-tanzu/velero/pkg/label"
)

const (
	GCSyncPeriod = 60 * time.Minute
)

// gcController creates DeleteBackupRequests for expired backups.
type gcController struct {
	*genericController

	backupLister              velerov1listers.BackupLister
	deleteBackupRequestLister velerov1listers.DeleteBackupRequestLister
	deleteBackupRequestClient velerov1client.DeleteBackupRequestsGetter
	kbClient                  client.Client

	clock clock.Clock
}

// NewGCController constructs a new gcController.
func NewGCController(
	logger logrus.FieldLogger,
	backupInformer velerov1informers.BackupInformer,
	deleteBackupRequestLister velerov1listers.DeleteBackupRequestLister,
	deleteBackupRequestClient velerov1client.DeleteBackupRequestsGetter,
	kbClient client.Client,
) Interface {
	c := &gcController{
		genericController:         newGenericController(GarbageCollection, logger),
		clock:                     clock.RealClock{},
		backupLister:              backupInformer.Lister(),
		deleteBackupRequestLister: deleteBackupRequestLister,
		deleteBackupRequestClient: deleteBackupRequestClient,
		kbClient:                  kbClient,
	}

	c.syncHandler = c.processQueueItem
	c.resyncPeriod = GCSyncPeriod
	c.resyncFunc = c.enqueueAllBackups

	backupInformer.Informer().AddEventHandler(
		cache.ResourceEventHandlerFuncs{
			AddFunc:    c.enqueue,
			UpdateFunc: func(_, obj interface{}) { c.enqueue(obj) },
		},
	)

	return c
}

// enqueueAllBackups lists all backups from cache and enqueues all of them so we can check each one
// for expiration.
func (c *gcController) enqueueAllBackups() {
	c.logger.Debug("gcController.enqueueAllBackups")

	backups, err := c.backupLister.List(labels.Everything())
	if err != nil {
		c.logger.WithError(errors.WithStack(err)).Error("error listing backups")
		return
	}

	for _, backup := range backups {
		c.enqueue(backup)
	}
}

func (c *gcController) processQueueItem(key string) error {
	log := c.logger.WithField("backup", key)

	ns, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		return errors.Wrap(err, "error splitting queue key")
	}

	backup, err := c.backupLister.Backups(ns).Get(name)
	if apierrors.IsNotFound(err) {
		log.Debug("Unable to find backup")
		return nil
	}
	if err != nil {
		return errors.Wrap(err, "error getting backup")
	}

	log = c.logger.WithFields(
		logrus.Fields{
			"backup":     key,
			"expiration": backup.Status.Expiration,
		},
	)

	now := c.clock.Now()

	if backup.Status.Expiration == nil || backup.Status.Expiration.After(now) {
		log.Debug("Backup has not expired yet, skipping")
		return nil
	}

	log.Info("Backup has expired")

	loc := &velerov1api.BackupStorageLocation{}
	if err := c.kbClient.Get(context.Background(), client.ObjectKey{
		Namespace: ns,
		Name:      backup.Spec.StorageLocation,
	}, loc); err != nil {
		if apierrors.IsNotFound(err) {
			log.Warnf("Backup cannot be garbage-collected because backup storage location %s does not exist", backup.Spec.StorageLocation)
		}
		return errors.Wrap(err, "error getting backup storage location")
	}

	if loc.Spec.AccessMode == velerov1api.BackupStorageLocationAccessModeReadOnly {
		log.Infof("Backup cannot be garbage-collected because backup storage location %s is currently in read-only mode", loc.Name)
		return nil
	}

	selector := labels.SelectorFromSet(labels.Set(map[string]string{
		velerov1api.BackupNameLabel: label.GetValidName(backup.Name),
		velerov1api.BackupUIDLabel:  string(backup.UID),
	}))

	dbrs, err := c.deleteBackupRequestLister.DeleteBackupRequests(ns).List(selector)
	if err != nil {
		return errors.Wrap(err, "error listing existing DeleteBackupRequests for backup")
	}

	// if there's an existing unprocessed deletion request for this backup, don't create
	// another one
	for _, dbr := range dbrs {
		switch dbr.Status.Phase {
		case "", velerov1api.DeleteBackupRequestPhaseNew, velerov1api.DeleteBackupRequestPhaseInProgress:
			log.Info("Backup already has a pending deletion request")
			return nil
		}
	}

	log.Info("Creating a new deletion request")
	req := pkgbackup.NewDeleteBackupRequest(backup.Name, string(backup.UID))

	if _, err = c.deleteBackupRequestClient.DeleteBackupRequests(ns).Create(context.TODO(), req, metav1.CreateOptions{}); err != nil {
		return errors.Wrap(err, "error creating DeleteBackupRequest")
	}

	return nil
}
