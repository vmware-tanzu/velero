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
	"encoding/json"
	"time"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	kuberrs "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/tools/cache"

	velerov1api "github.com/heptio/velero/pkg/apis/velero/v1"
	velerov1client "github.com/heptio/velero/pkg/generated/clientset/versioned/typed/velero/v1"
	informers "github.com/heptio/velero/pkg/generated/informers/externalversions/velero/v1"
	listers "github.com/heptio/velero/pkg/generated/listers/velero/v1"
	"github.com/heptio/velero/pkg/label"
	"github.com/heptio/velero/pkg/persistence"
	"github.com/heptio/velero/pkg/plugin/clientmgmt"
)

type backupSyncController struct {
	*genericController

	backupClient                velerov1client.BackupsGetter
	backupLocationClient        velerov1client.BackupStorageLocationsGetter
	backupLister                listers.BackupLister
	backupStorageLocationLister listers.BackupStorageLocationLister
	namespace                   string
	defaultBackupLocation       string
	newPluginManager            func(logrus.FieldLogger) clientmgmt.Manager
	newBackupStore              func(*velerov1api.BackupStorageLocation, persistence.ObjectStoreGetter, logrus.FieldLogger) (persistence.BackupStore, error)
}

func NewBackupSyncController(
	backupClient velerov1client.BackupsGetter,
	backupLocationClient velerov1client.BackupStorageLocationsGetter,
	backupInformer informers.BackupInformer,
	backupStorageLocationInformer informers.BackupStorageLocationInformer,
	syncPeriod time.Duration,
	namespace string,
	defaultBackupLocation string,
	newPluginManager func(logrus.FieldLogger) clientmgmt.Manager,
	logger logrus.FieldLogger,
) Interface {
	if syncPeriod < time.Minute {
		logger.Infof("Provided backup sync period %v is too short. Setting to 1 minute", syncPeriod)
		syncPeriod = time.Minute
	}

	c := &backupSyncController{
		genericController:           newGenericController("backup-sync", logger),
		backupClient:                backupClient,
		backupLocationClient:        backupLocationClient,
		namespace:                   namespace,
		defaultBackupLocation:       defaultBackupLocation,
		backupLister:                backupInformer.Lister(),
		backupStorageLocationLister: backupStorageLocationInformer.Lister(),

		// use variables to refer to these functions so they can be
		// replaced with fakes for testing.
		newPluginManager: newPluginManager,
		newBackupStore:   persistence.NewObjectBackupStore,
	}

	c.resyncFunc = c.run
	c.resyncPeriod = syncPeriod
	c.cacheSyncWaiters = []cache.InformerSynced{
		backupInformer.Informer().HasSynced,
		backupStorageLocationInformer.Informer().HasSynced,
	}

	return c
}

func shouldSync(location *velerov1api.BackupStorageLocation, now time.Time, backupStore persistence.BackupStore, log logrus.FieldLogger) (bool, string) {
	log = log.WithFields(map[string]interface{}{
		"lastSyncedRevision": location.Status.LastSyncedRevision,
		"lastSyncedTime":     location.Status.LastSyncedTime.Time.Format(time.RFC1123Z),
	})

	revision, err := backupStore.GetRevision()
	if err != nil {
		log.WithError(err).Debugf("Unable to get backup store's revision file, syncing (this is not an error if a v0.10+ backup has not yet been taken into this location)")
		return true, ""
	}
	log = log.WithField("revision", revision)

	if location.Status.LastSyncedTime.Add(time.Hour).Before(now) {
		log.Debugf("Backup location hasn't been synced in more than %s, syncing", time.Hour)
		return true, revision
	}

	if string(location.Status.LastSyncedRevision) != revision {
		log.Debugf("Backup location hasn't been synced since its last modification, syncing")
		return true, revision
	}

	log.Debugf("Backup location's contents haven't changed since last sync, not syncing")
	return false, ""
}

// orderedBackupLocations returns a new slice with the default backup location first (if it exists),
// followed by the rest of the locations in no particular order.
func orderedBackupLocations(locations []*velerov1api.BackupStorageLocation, defaultLocationName string) []*velerov1api.BackupStorageLocation {
	var result []*velerov1api.BackupStorageLocation

	for i := range locations {
		if locations[i].Name == defaultLocationName {
			// put the default location first
			result = append(result, locations[i])
			// append everything before the default
			result = append(result, locations[:i]...)
			// append everything after the default
			result = append(result, locations[i+1:]...)

			return result
		}
	}

	return locations
}

func (c *backupSyncController) run() {
	c.logger.Debug("Checking for existing backup storage locations to sync into cluster")

	locations, err := c.backupStorageLocationLister.BackupStorageLocations(c.namespace).List(labels.Everything())
	if err != nil {
		c.logger.WithError(errors.WithStack(err)).Error("Error getting backup storage locations from lister")
		return
	}
	// sync the default location first, if it exists
	locations = orderedBackupLocations(locations, c.defaultBackupLocation)

	pluginManager := c.newPluginManager(c.logger)
	defer pluginManager.CleanupClients()

	for _, location := range locations {
		log := c.logger.WithField("backupLocation", location.Name)

		backupStore, err := c.newBackupStore(location, pluginManager, log)
		if err != nil {
			log.WithError(err).Error("Error getting backup store for location")
			continue
		}

		ok, revision := shouldSync(location, time.Now().UTC(), backupStore, log)
		if !ok {
			continue
		}
		log.Infof("Syncing contents of backup store into cluster")

		res, err := backupStore.ListBackups()
		if err != nil {
			log.WithError(err).Error("Error listing backups in backup store")
			continue
		}
		backupStoreBackups := sets.NewString(res...)
		log.WithField("backupCount", len(backupStoreBackups)).Info("Got backups from backup store")

		for backupName := range backupStoreBackups {
			log = log.WithField("backup", backupName)
			log.Debug("Checking backup store backup to see if it needs to be synced into the cluster")

			// use the controller's namespace when getting the backup because that's where we
			// are syncing backups to, regardless of the namespace of the cloud backup.
			backup, err := c.backupClient.Backups(c.namespace).Get(backupName, metav1.GetOptions{})
			if err == nil {
				log.Debug("Backup already exists in cluster")

				if backup.Spec.StorageLocation != "" {
					continue
				}

				// pre-v0.10 backups won't initially have a .spec.storageLocation so fill it in
				log.Debug("Patching backup's .spec.storageLocation because it's missing")
				if err := patchStorageLocation(backup, c.backupClient.Backups(c.namespace), location.Name); err != nil {
					log.WithError(err).Error("Error patching backup's .spec.storageLocation")
				}

				continue
			}
			if !kuberrs.IsNotFound(err) {
				log.WithError(errors.WithStack(err)).Error("Error getting backup from client, proceeding with sync into cluster")
			}

			backup, err = backupStore.GetBackupMetadata(backupName)
			if err != nil {
				log.WithError(errors.WithStack(err)).Error("Error getting backup metadata from backup store")
				continue
			}

			backup.Namespace = c.namespace
			backup.ResourceVersion = ""

			// update the StorageLocation field and label since the name of the location
			// may be different in this cluster than in the cluster that created the
			// backup.
			backup.Spec.StorageLocation = location.Name
			if backup.Labels == nil {
				backup.Labels = make(map[string]string)
			}
			backup.Labels[velerov1api.StorageLocationLabel] = label.GetValidName(backup.Spec.StorageLocation)

			_, err = c.backupClient.Backups(backup.Namespace).Create(backup)
			switch {
			case err != nil && kuberrs.IsAlreadyExists(err):
				log.Debug("Backup already exists in cluster")
				continue
			case err != nil && !kuberrs.IsAlreadyExists(err):
				log.WithError(errors.WithStack(err)).Error("Error syncing backup into cluster")
				continue
			default:
				log.Debug("Synced backup into cluster")
			}
		}

		c.deleteOrphanedBackups(location.Name, backupStoreBackups, log)

		// update the location's status's last-synced fields
		patch := map[string]interface{}{
			"status": map[string]interface{}{
				"lastSyncedTime":     time.Now().UTC(),
				"lastSyncedRevision": revision,
			},
		}

		patchBytes, err := json.Marshal(patch)
		if err != nil {
			log.WithError(errors.WithStack(err)).Error("Error marshaling last-synced patch to JSON")
			continue
		}

		if _, err = c.backupLocationClient.BackupStorageLocations(c.namespace).Patch(
			location.Name,
			types.MergePatchType,
			patchBytes,
		); err != nil {
			log.WithError(errors.WithStack(err)).Error("Error patching backup location's last-synced time and revision")
			continue
		}
	}
}

func patchStorageLocation(backup *velerov1api.Backup, client velerov1client.BackupInterface, location string) error {
	patch := map[string]interface{}{
		"spec": map[string]interface{}{
			"storageLocation": location,
		},
	}

	patchBytes, err := json.Marshal(patch)
	if err != nil {
		return errors.WithStack(err)
	}

	if _, err := client.Patch(backup.Name, types.MergePatchType, patchBytes); err != nil {
		return errors.WithStack(err)
	}

	return nil
}

// deleteOrphanedBackups deletes backup objects from Kubernetes that have the specified location
// and a phase of Completed, but no corresponding backup in object storage.
func (c *backupSyncController) deleteOrphanedBackups(locationName string, cloudBackupNames sets.String, log logrus.FieldLogger) {
	locationSelector := labels.Set(map[string]string{
		velerov1api.StorageLocationLabel: label.GetValidName(locationName),
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
		if backup.Status.Phase != velerov1api.BackupPhaseCompleted || cloudBackupNames.Has(backup.Name) {
			continue
		}

		if err := c.backupClient.Backups(backup.Namespace).Delete(backup.Name, nil); err != nil {
			log.WithError(errors.WithStack(err)).Error("Error deleting orphaned backup from cluster")
		} else {
			log.Debug("Deleted orphaned backup from cluster")
		}
	}
}
