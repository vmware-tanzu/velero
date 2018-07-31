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
	"time"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"

	kuberrs "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/tools/cache"

	"github.com/heptio/ark/pkg/cloudprovider"
	arkv1client "github.com/heptio/ark/pkg/generated/clientset/versioned/typed/ark/v1"
	informers "github.com/heptio/ark/pkg/generated/informers/externalversions/ark/v1"
	listers "github.com/heptio/ark/pkg/generated/listers/ark/v1"
	"github.com/heptio/ark/pkg/util/kube"
	"github.com/heptio/ark/pkg/util/stringslice"
)

type backupSyncController struct {
	client               arkv1client.BackupsGetter
	cloudBackupLister    cloudprovider.BackupLister
	bucket               string
	syncPeriod           time.Duration
	namespace            string
	backupLister         listers.BackupLister
	backupInformerSynced cache.InformerSynced
	logger               logrus.FieldLogger
}

func NewBackupSyncController(
	client arkv1client.BackupsGetter,
	cloudBackupLister cloudprovider.BackupLister,
	bucket string,
	syncPeriod time.Duration,
	namespace string,
	backupInformer informers.BackupInformer,
	logger logrus.FieldLogger,
) Interface {
	if syncPeriod < time.Minute {
		logger.Infof("Provided backup sync period %v is too short. Setting to 1 minute", syncPeriod)
		syncPeriod = time.Minute
	}
	return &backupSyncController{
		client:               client,
		cloudBackupLister:    cloudBackupLister,
		bucket:               bucket,
		syncPeriod:           syncPeriod,
		namespace:            namespace,
		backupLister:         backupInformer.Lister(),
		backupInformerSynced: backupInformer.Informer().HasSynced,
		logger:               logger,
	}
}

// Run is a blocking function that continually runs the object storage -> Ark API
// sync process according to the controller's syncPeriod. It will return when it
// receives on the ctx.Done() channel.
func (c *backupSyncController) Run(ctx context.Context, workers int) error {
	c.logger.Info("Running backup sync controller")
	c.logger.Info("Waiting for caches to sync")
	if !cache.WaitForCacheSync(ctx.Done(), c.backupInformerSynced) {
		return errors.New("timed out waiting for caches to sync")
	}
	c.logger.Info("Caches are synced")
	wait.Until(c.run, c.syncPeriod, ctx.Done())
	return nil
}

const gcFinalizer = "gc.ark.heptio.com"

func (c *backupSyncController) run() {
	c.logger.Info("Syncing backups from object storage")
	backups, err := c.cloudBackupLister.ListBackups(c.bucket)
	if err != nil {
		c.logger.WithError(err).Error("error listing backups")
		return
	}
	c.logger.WithField("backupCount", len(backups)).Info("Got backups from object storage")

	cloudBackupNames := sets.NewString()
	for _, cloudBackup := range backups {
		logContext := c.logger.WithField("backup", kube.NamespaceAndName(cloudBackup))
		logContext.Info("Syncing backup")

		cloudBackupNames.Insert(cloudBackup.Name)

		// If we're syncing backups made by pre-0.8.0 versions, the server removes all finalizers
		// faster than the sync finishes. Just process them as we find them.
		cloudBackup.Finalizers = stringslice.Except(cloudBackup.Finalizers, gcFinalizer)

		cloudBackup.Namespace = c.namespace
		cloudBackup.ResourceVersion = ""

		// Backup only if backup does not exist in Kubernetes or if we are not able to get the backup for any reason.
		_, err := c.client.Backups(cloudBackup.Namespace).Get(cloudBackup.Name, metav1.GetOptions{})
		if err != nil {
			if !kuberrs.IsNotFound(err) {
				logContext.WithError(errors.WithStack(err)).Error("Error getting backup from client, proceeding with backup sync")
			}

			if _, err := c.client.Backups(cloudBackup.Namespace).Create(cloudBackup); err != nil && !kuberrs.IsAlreadyExists(err) {
				logContext.WithError(errors.WithStack(err)).Error("Error syncing backup from object storage")
			}
		}
	}

	c.deleteUnused(cloudBackupNames)
	return
}

// deleteUnused deletes backup objects from Kubernetes if there is no corresponding backup in the object storage.
func (c *backupSyncController) deleteUnused(cloudBackupNames sets.String) {
	// Backups objects in Kubernetes
	backups, err := c.backupLister.Backups(c.namespace).List(labels.Everything())
	if err != nil {
		c.logger.WithError(errors.WithStack(err)).Error("Error listing backup from Kubernetes")
	}
	if len(backups) == 0 {
		return
	}

	// For each backup object in Kubernetes, verify if has a corresponding backup in the object storage. If not, delete it.
	for _, backup := range backups {
		if !cloudBackupNames.Has(backup.Name) {
			if err := c.client.Backups(backup.Namespace).Delete(backup.Name, nil); err != nil {
				c.logger.WithError(errors.WithStack(err)).Error("Error deleting unused backup from Kubernetes")
			} else {
				c.logger.Debugf("Deleted backup: %s", backup.Name)
			}
		}
	}

	return
}
