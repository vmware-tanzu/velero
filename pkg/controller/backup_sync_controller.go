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
	"time"

	snapshotterClientSet "github.com/kubernetes-csi/external-snapshotter/client/v4/clientset/versioned"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	kuberrs "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/kubernetes"

	"github.com/vmware-tanzu/velero/internal/storage"
	velerov1api "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	"github.com/vmware-tanzu/velero/pkg/features"
	velerov1client "github.com/vmware-tanzu/velero/pkg/generated/clientset/versioned/typed/velero/v1"
	velerov1listers "github.com/vmware-tanzu/velero/pkg/generated/listers/velero/v1"
	"github.com/vmware-tanzu/velero/pkg/label"
	"github.com/vmware-tanzu/velero/pkg/persistence"
	"github.com/vmware-tanzu/velero/pkg/plugin/clientmgmt"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

type backupSyncController struct {
	*genericController

	backupClient            velerov1client.BackupsGetter
	kbClient                client.Client
	podVolumeBackupClient   velerov1client.PodVolumeBackupsGetter
	backupLister            velerov1listers.BackupLister
	csiSnapshotClient       *snapshotterClientSet.Clientset
	kubeClient              kubernetes.Interface
	namespace               string
	defaultBackupLocation   string
	defaultBackupSyncPeriod time.Duration
	newPluginManager        func(logrus.FieldLogger) clientmgmt.Manager
	newBackupStore          func(*velerov1api.BackupStorageLocation, persistence.ObjectStoreGetter, logrus.FieldLogger) (persistence.BackupStore, error)
}

func NewBackupSyncController(
	backupClient velerov1client.BackupsGetter,
	kbClient client.Client,
	podVolumeBackupClient velerov1client.PodVolumeBackupsGetter,
	backupLister velerov1listers.BackupLister,
	syncPeriod time.Duration,
	namespace string,
	csiSnapshotClient *snapshotterClientSet.Clientset,
	kubeClient kubernetes.Interface,
	defaultBackupLocation string,
	newPluginManager func(logrus.FieldLogger) clientmgmt.Manager,
	logger logrus.FieldLogger,
) Interface {
	if syncPeriod <= 0 {
		syncPeriod = time.Minute
	}
	logger.Infof("Backup sync period is %v", syncPeriod)

	c := &backupSyncController{
		genericController:       newGenericController(BackupSync, logger),
		backupClient:            backupClient,
		kbClient:                kbClient,
		podVolumeBackupClient:   podVolumeBackupClient,
		namespace:               namespace,
		defaultBackupLocation:   defaultBackupLocation,
		defaultBackupSyncPeriod: syncPeriod,
		backupLister:            backupLister,
		csiSnapshotClient:       csiSnapshotClient,
		kubeClient:              kubeClient,

		// use variables to refer to these functions so they can be
		// replaced with fakes for testing.
		newPluginManager: newPluginManager,
		newBackupStore:   persistence.NewObjectBackupStore,
	}

	c.resyncFunc = c.run
	c.resyncPeriod = 30 * time.Second

	return c
}

// orderedBackupLocations returns a new slice with the default backup location first (if it exists),
// followed by the rest of the locations in no particular order.
func orderedBackupLocations(locationList *velerov1api.BackupStorageLocationList, defaultLocationName string) []velerov1api.BackupStorageLocation {
	var result []velerov1api.BackupStorageLocation

	for i := range locationList.Items {
		if locationList.Items[i].Name == defaultLocationName {
			// put the default location first
			result = append(result, locationList.Items[i])
			// append everything before the default
			result = append(result, locationList.Items[:i]...)
			// append everything after the default
			result = append(result, locationList.Items[i+1:]...)

			return result
		}
	}

	return locationList.Items
}

func (c *backupSyncController) run() {
	c.logger.Debug("Checking for existing backup storage locations to sync into cluster")

	locationList, err := storage.ListBackupStorageLocations(context.Background(), c.kbClient, c.namespace)
	if err != nil {
		c.logger.WithError(err).Error("No backup storage locations found, at least one is required")
		return
	}

	// sync the default backup storage location first, if it exists
	for _, location := range locationList.Items {
		if location.Spec.Default {
			c.defaultBackupLocation = location.Name
			break
		}
	}
	locations := orderedBackupLocations(&locationList, c.defaultBackupLocation)

	pluginManager := c.newPluginManager(c.logger)
	defer pluginManager.CleanupClients()

	for _, location := range locations {
		log := c.logger.WithField("backupLocation", location.Name)

		syncPeriod := c.defaultBackupSyncPeriod
		if location.Spec.BackupSyncPeriod != nil {
			syncPeriod = location.Spec.BackupSyncPeriod.Duration
			if syncPeriod == 0 {
				log.Debug("Backup sync period for this location is set to 0, skipping sync")
				continue
			}

			if syncPeriod < 0 {
				log.Debug("Backup sync period must be non-negative")
				syncPeriod = c.defaultBackupSyncPeriod
			}
		}

		lastSync := location.Status.LastSyncedTime
		if lastSync != nil {
			log.Debug("Checking if backups need to be synced at this time for this location")
			nextSync := lastSync.Add(syncPeriod)
			if time.Now().UTC().Before(nextSync) {
				continue
			}
		}

		log.Debug("Checking backup location for backups to sync into cluster")

		backupStore, err := c.newBackupStore(&location, pluginManager, log)
		if err != nil {
			log.WithError(err).Error("Error getting backup store for this location")
			continue
		}

		// get a list of all the backups that are stored in the backup storage location
		res, err := backupStore.ListBackups()
		if err != nil {
			log.WithError(err).Error("Error listing backups in backup store")
			continue
		}
		backupStoreBackups := sets.NewString(res...)
		log.WithField("backupCount", len(backupStoreBackups)).Debug("Got backups from backup store")

		// get a list of all the backups that exist as custom resources in the cluster
		clusterBackups, err := c.backupLister.Backups(c.namespace).List(labels.Everything())
		if err != nil {
			log.WithError(errors.WithStack(err)).Error("Error getting backups from cluster, proceeding with sync into cluster")
		} else {
			log.WithField("backupCount", len(clusterBackups)).Debug("Got backups from cluster")
		}

		// get a list of backups that *are* in the backup storage location and *aren't* in the cluster
		clusterBackupsSet := sets.NewString()
		for _, b := range clusterBackups {
			clusterBackupsSet.Insert(b.Name)
		}
		backupsToSync := backupStoreBackups.Difference(clusterBackupsSet)

		if count := backupsToSync.Len(); count > 0 {
			log.Infof("Found %v backups in the backup location that do not exist in the cluster and need to be synced", count)
		} else {
			log.Debug("No backups found in the backup location that need to be synced into the cluster")
		}

		// sync each backup
		for backupName := range backupsToSync {
			log = log.WithField("backup", backupName)
			log.Info("Attempting to sync backup into cluster")

			backup, err := backupStore.GetBackupMetadata(backupName)
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

			// attempt to create backup custom resource via API
			backup, err = c.backupClient.Backups(backup.Namespace).Create(context.TODO(), backup, metav1.CreateOptions{})
			switch {
			case err != nil && kuberrs.IsAlreadyExists(err):
				log.Debug("Backup already exists in cluster")
				continue
			case err != nil && !kuberrs.IsAlreadyExists(err):
				log.WithError(errors.WithStack(err)).Error("Error syncing backup into cluster")
				continue
			default:
				log.Info("Successfully synced backup into cluster")
			}

			// process the pod volume backups from object store, if any
			podVolumeBackups, err := backupStore.GetPodVolumeBackups(backupName)
			if err != nil {
				log.WithError(errors.WithStack(err)).Error("Error getting pod volume backups for this backup from backup store")
				continue
			}

			for _, podVolumeBackup := range podVolumeBackups {
				log := log.WithField("podVolumeBackup", podVolumeBackup.Name)
				log.Debug("Checking this pod volume backup to see if it needs to be synced into the cluster")

				for i, ownerRef := range podVolumeBackup.OwnerReferences {
					if ownerRef.APIVersion == velerov1api.SchemeGroupVersion.String() && ownerRef.Kind == "Backup" && ownerRef.Name == backup.Name {
						log.WithField("uid", backup.UID).Debugf("Updating pod volume backup's owner reference UID")
						podVolumeBackup.OwnerReferences[i].UID = backup.UID
					}
				}

				if _, ok := podVolumeBackup.Labels[velerov1api.BackupUIDLabel]; ok {
					podVolumeBackup.Labels[velerov1api.BackupUIDLabel] = string(backup.UID)
				}

				podVolumeBackup.Namespace = backup.Namespace
				podVolumeBackup.ResourceVersion = ""

				_, err = c.podVolumeBackupClient.PodVolumeBackups(backup.Namespace).Create(context.TODO(), podVolumeBackup, metav1.CreateOptions{})
				switch {
				case err != nil && kuberrs.IsAlreadyExists(err):
					log.Debug("Pod volume backup already exists in cluster")
					continue
				case err != nil && !kuberrs.IsAlreadyExists(err):
					log.WithError(errors.WithStack(err)).Error("Error syncing pod volume backup into cluster")
					continue
				default:
					log.Debug("Synced pod volume backup into cluster")
				}
			}

			if features.IsEnabled(velerov1api.CSIFeatureFlag) {
				// we are syncing these objects only to ensure that the storage snapshots are cleaned up
				// on backup deletion or expiry.
				log.Info("Syncing CSI volumesnapshotcontents in backup")
				snapConts, err := backupStore.GetCSIVolumeSnapshotContents(backupName)
				if err != nil {
					log.WithError(errors.WithStack(err)).Error("Error getting CSI volumesnapshotcontents for this backup from backup store")
					continue
				}

				log.Infof("Syncing %d CSI volumesnapshotcontents in backup", len(snapConts))
				for _, snapCont := range snapConts {
					// TODO: Reset ResourceVersion prior to persisting VolumeSnapshotContents
					snapCont.ResourceVersion = ""
					created, err := c.csiSnapshotClient.SnapshotV1beta1().VolumeSnapshotContents().Create(context.TODO(), snapCont, metav1.CreateOptions{})
					switch {
					case err != nil && kuberrs.IsAlreadyExists(err):
						log.Debugf("volumesnapshotcontent %s already exists in cluster", snapCont.Name)
						continue
					case err != nil && !kuberrs.IsAlreadyExists(err):
						log.WithError(errors.WithStack(err)).Errorf("Error syncing volumesnapshotcontent %s into cluster", snapCont.Name)
						continue
					default:
						log.Infof("Created CSI volumesnapshotcontent %s", created.Name)
					}
				}
			}
		}

		c.deleteOrphanedBackups(location.Name, backupStoreBackups, log)

		// update the location's last-synced time field
		statusPatch := client.MergeFrom(location.DeepCopyObject())
		location.Status.LastSyncedTime = &metav1.Time{Time: time.Now().UTC()}
		if err := c.kbClient.Status().Patch(context.Background(), &location, statusPatch); err != nil {
			log.WithError(errors.WithStack(err)).Error("Error patching backup location's last-synced time")
			continue
		}
	}
}

// deleteOrphanedBackups deletes backup objects (CRDs) from Kubernetes that have the specified location
// and a phase of Completed, but no corresponding backup in object storage.
func (c *backupSyncController) deleteOrphanedBackups(locationName string, backupStoreBackups sets.String, log logrus.FieldLogger) {
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
		if backup.Status.Phase != velerov1api.BackupPhaseCompleted || backupStoreBackups.Has(backup.Name) {
			continue
		}

		if err := c.backupClient.Backups(backup.Namespace).Delete(context.TODO(), backup.Name, metav1.DeleteOptions{}); err != nil {
			log.WithError(errors.WithStack(err)).Error("Error deleting orphaned backup from cluster")
		} else {
			log.Debug("Deleted orphaned backup from cluster")
		}
	}
}
