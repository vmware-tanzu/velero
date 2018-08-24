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
	"time"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"

	kuberrs "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/tools/cache"

	arkv1api "github.com/heptio/ark/pkg/apis/ark/v1"
	"github.com/heptio/ark/pkg/cloudprovider"
	arkv1client "github.com/heptio/ark/pkg/generated/clientset/versioned/typed/ark/v1"
	informers "github.com/heptio/ark/pkg/generated/informers/externalversions/ark/v1"
	listers "github.com/heptio/ark/pkg/generated/listers/ark/v1"
	"github.com/heptio/ark/pkg/plugin"
	"github.com/heptio/ark/pkg/util/kube"
	"github.com/heptio/ark/pkg/util/stringslice"
)

type backupSyncController struct {
	*genericController

	client                      arkv1client.BackupsGetter
	backupLister                listers.BackupLister
	backupStorageLocationLister listers.BackupStorageLocationLister
	namespace                   string
	defaultBackupLocation       string
	newPluginManager            func(logrus.FieldLogger) plugin.Manager
	listCloudBackups            func(logrus.FieldLogger, cloudprovider.ObjectStore, string) ([]*arkv1api.Backup, error)
}

func NewBackupSyncController(
	client arkv1client.BackupsGetter,
	backupInformer informers.BackupInformer,
	backupStorageLocationInformer informers.BackupStorageLocationInformer,
	syncPeriod time.Duration,
	namespace string,
	defaultBackupLocation string,
	pluginRegistry plugin.Registry,
	logger logrus.FieldLogger,
	logLevel logrus.Level,
) Interface {
	if syncPeriod < time.Minute {
		logger.Infof("Provided backup sync period %v is too short. Setting to 1 minute", syncPeriod)
		syncPeriod = time.Minute
	}

	c := &backupSyncController{
		genericController:           newGenericController("backup-sync", logger),
		client:                      client,
		namespace:                   namespace,
		defaultBackupLocation:       defaultBackupLocation,
		backupLister:                backupInformer.Lister(),
		backupStorageLocationLister: backupStorageLocationInformer.Lister(),

		newPluginManager: func(logger logrus.FieldLogger) plugin.Manager {
			return plugin.NewManager(logger, logLevel, pluginRegistry)
		},
		listCloudBackups: cloudprovider.ListBackups,
	}

	c.resyncFunc = c.run
	c.resyncPeriod = syncPeriod
	c.cacheSyncWaiters = []cache.InformerSynced{
		backupInformer.Informer().HasSynced,
		backupStorageLocationInformer.Informer().HasSynced,
	}

	return c
}

const gcFinalizer = "gc.ark.heptio.com"

func (c *backupSyncController) run() {
	c.logger.Info("Syncing backups from backup storage into cluster")

	locations, err := c.backupStorageLocationLister.BackupStorageLocations(c.namespace).List(labels.Everything())
	if err != nil {
		c.logger.WithError(errors.WithStack(err)).Error("Error getting backup storage locations from lister")
		return
	}
	// sync the default location first, if it exists
	locations = orderedBackupLocations(locations, c.defaultBackupLocation)

	pluginManager := c.newPluginManager(c.logger)

	for _, location := range locations {
		log := c.logger.WithField("backupLocation", location.Name)
		log.Info("Syncing backups from backup location")

		objectStore, err := getObjectStoreForLocation(location, pluginManager)
		if err != nil {
			log.WithError(err).Error("Error getting object store for location")
			continue
		}

		backupsInBackupStore, err := c.listCloudBackups(log, objectStore, location.Spec.ObjectStorage.Bucket)
		if err != nil {
			log.WithError(err).Error("Error listing backups in object store")
			continue
		}

		log.WithField("backupCount", len(backupsInBackupStore)).Info("Got backups from object store")

		cloudBackupNames := sets.NewString()
		for _, cloudBackup := range backupsInBackupStore {
			log = log.WithField("backup", kube.NamespaceAndName(cloudBackup))
			log.Debug("Checking cloud backup to see if it needs to be synced into the cluster")

			cloudBackupNames.Insert(cloudBackup.Name)

			// use the controller's namespace when getting the backup because that's where we
			// are syncing backups to, regardless of the namespace of the cloud backup.
			_, err := c.client.Backups(c.namespace).Get(cloudBackup.Name, metav1.GetOptions{})
			if err == nil {
				log.Debug("Backup already exists in cluster")
				continue
			}
			if !kuberrs.IsNotFound(err) {
				log.WithError(errors.WithStack(err)).Error("Error getting backup from client, proceeding with sync into cluster")
			}

			// remove the pre-v0.8.0 gcFinalizer if it exists
			// TODO(1.0): remove this
			cloudBackup.Finalizers = stringslice.Except(cloudBackup.Finalizers, gcFinalizer)
			cloudBackup.Namespace = c.namespace
			cloudBackup.ResourceVersion = ""

			// update the StorageLocation field and label since the name of the location
			// may be different in this cluster than in the cluster that created the
			// backup.
			cloudBackup.Spec.StorageLocation = location.Name
			if cloudBackup.Labels == nil {
				cloudBackup.Labels = make(map[string]string)
			}
			cloudBackup.Labels[arkv1api.StorageLocationLabel] = cloudBackup.Spec.StorageLocation

			_, err = c.client.Backups(cloudBackup.Namespace).Create(cloudBackup)
			switch {
			case err != nil && kuberrs.IsAlreadyExists(err):
				log.Debug("Backup already exists in cluster")
			case err != nil && !kuberrs.IsAlreadyExists(err):
				log.WithError(errors.WithStack(err)).Error("Error syncing backup into cluster")
			default:
				log.Debug("Synced backup into cluster")
			}
		}

		c.deleteOrphanedBackups(location.Name, cloudBackupNames, log)
	}
}

// deleteOrphanedBackups deletes backup objects from Kubernetes that have the specified location
// and a phase of Completed, but no corresponding backup in object storage.
func (c *backupSyncController) deleteOrphanedBackups(locationName string, cloudBackupNames sets.String, log logrus.FieldLogger) {
	locationSelector := labels.Set(map[string]string{
		arkv1api.StorageLocationLabel: locationName,
	}).AsSelector()

	backups, err := c.backupLister.Backups(c.namespace).List(locationSelector)
	if err != nil {
		log.WithError(errors.WithStack(err)).Error("Error listing backups from cluster")
		return
	}
	if len(backups) == 0 {
		return
	}

	for _, backup := range backups {
		log = log.WithField("backup", backup.Name)
		if backup.Status.Phase != arkv1api.BackupPhaseCompleted || cloudBackupNames.Has(backup.Name) {
			continue
		}

		if err := c.client.Backups(backup.Namespace).Delete(backup.Name, nil); err != nil {
			log.WithError(errors.WithStack(err)).Error("Error deleting orphaned backup from cluster")
		} else {
			log.Debug("Deleted orphaned backup from cluster")
		}
	}
}
