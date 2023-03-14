/*
Copyright The Velero Contributors.

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

	snapshotv1api "github.com/kubernetes-csi/external-snapshotter/client/v4/apis/volumesnapshot/v1"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	kuberrs "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/sets"
	"sigs.k8s.io/controller-runtime/pkg/builder"

	velerov1api "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	"github.com/vmware-tanzu/velero/pkg/features"
	"github.com/vmware-tanzu/velero/pkg/label"
	"github.com/vmware-tanzu/velero/pkg/persistence"
	"github.com/vmware-tanzu/velero/pkg/plugin/clientmgmt"
	"github.com/vmware-tanzu/velero/pkg/util/kube"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	backupSyncReconcilePeriod = time.Minute
)

type backupSyncReconciler struct {
	client                  client.Client
	namespace               string
	defaultBackupSyncPeriod time.Duration
	newPluginManager        func(logrus.FieldLogger) clientmgmt.Manager
	backupStoreGetter       persistence.ObjectBackupStoreGetter
	logger                  logrus.FieldLogger
}

// NewBackupSyncReconciler is used to generate BackupSync reconciler structure.
func NewBackupSyncReconciler(
	client client.Client,
	namespace string,
	defaultBackupSyncPeriod time.Duration,
	newPluginManager func(logrus.FieldLogger) clientmgmt.Manager,
	backupStoreGetter persistence.ObjectBackupStoreGetter,
	logger logrus.FieldLogger) *backupSyncReconciler {
	return &backupSyncReconciler{
		client:                  client,
		namespace:               namespace,
		defaultBackupSyncPeriod: defaultBackupSyncPeriod,
		newPluginManager:        newPluginManager,
		backupStoreGetter:       backupStoreGetter,
		logger:                  logger,
	}
}

// Reconcile syncs between the backups in cluster and backups metadata in object store.
func (b *backupSyncReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := b.logger.WithField("controller", BackupSync)
	log = log.WithField("backupLocation", req.String())
	log.Debug("Begin to sync between backups' metadata in BSL object storage and cluster's existing backups.")

	location := &velerov1api.BackupStorageLocation{}
	err := b.client.Get(ctx, req.NamespacedName, location)
	if err != nil {
		if apierrors.IsNotFound(err) {
			log.Debug("BackupStorageLocation is not found")
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, errors.Wrapf(err, "error getting BackupStorageLocation %s", req.String())
	}

	pluginManager := b.newPluginManager(log)
	defer pluginManager.CleanupClients()

	log.Debug("Checking backup location for backups to sync into cluster")

	backupStore, err := b.backupStoreGetter.Get(location, pluginManager, log)
	if err != nil {
		log.WithError(err).Error("Error getting backup store for this location")
		return ctrl.Result{}, nil
	}

	// get a list of all the backups that are stored in the backup storage location
	res, err := backupStore.ListBackups()
	if err != nil {
		log.WithError(err).Error("Error listing backups in backup store")
		return ctrl.Result{}, nil
	}
	backupStoreBackups := sets.NewString(res...)
	log.WithField("backupCount", len(backupStoreBackups)).Debug("Got backups from backup store")

	// get a list of all the backups that exist as custom resources in the cluster
	var clusterBackupList velerov1api.BackupList
	listOption := client.ListOptions{
		LabelSelector: labels.Everything(),
		Namespace:     b.namespace,
	}

	err = b.client.List(ctx, &clusterBackupList, &listOption)
	if err != nil {
		log.WithError(errors.WithStack(err)).Error("Error getting backups from cluster, proceeding with sync into cluster")
	} else {
		log.WithField("backupCount", len(clusterBackupList.Items)).Debug("Got backups from cluster")
	}

	// get a list of backups that *are* in the backup storage location and *aren't* in the cluster
	clusterBackupsSet := sets.NewString()
	for _, b := range clusterBackupList.Items {
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

		if backup.Status.Phase == velerov1api.BackupPhaseWaitingForPluginOperations ||
			backup.Status.Phase == velerov1api.BackupPhaseWaitingForPluginOperationsPartiallyFailed ||
			backup.Status.Phase == velerov1api.BackupPhaseFinalizing ||
			backup.Status.Phase == velerov1api.BackupPhaseFinalizingPartiallyFailed {

			if backup.Status.Expiration == nil || backup.Status.Expiration.After(time.Now()) {
				log.Debugf("Skipping non-expired incomplete backup %v", backup.Name)
				continue
			}
			log.Debugf("%v Backup is past expiration, syncing for garbage collection", backup.Status.Phase)
			backup.Status.Phase = velerov1api.BackupPhasePartiallyFailed
		}
		backup.Namespace = b.namespace
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
		err = b.client.Create(ctx, backup, &client.CreateOptions{})
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

			err = b.client.Create(ctx, podVolumeBackup, &client.CreateOptions{})
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
			log.Info("Syncing CSI VolumeSnapshotClasses in backup")
			vsClasses, err := backupStore.GetCSIVolumeSnapshotClasses(backupName)
			if err != nil {
				log.WithError(errors.WithStack(err)).Error("Error getting CSI VolumeSnapClasses for this backup from backup store")
				continue
			}
			for _, vsClass := range vsClasses {
				vsClass.ResourceVersion = ""
				err := b.client.Create(ctx, vsClass, &client.CreateOptions{})
				switch {
				case err != nil && kuberrs.IsAlreadyExists(err):
					log.Debugf("VolumeSnapshotClass %s already exists in cluster", vsClass.Name)
					continue
				case err != nil && !kuberrs.IsAlreadyExists(err):
					log.WithError(errors.WithStack(err)).Errorf("Error syncing VolumeSnapshotClass %s into cluster", vsClass.Name)
					continue
				default:
					log.Infof("Created CSI VolumeSnapshotClass %s", vsClass.Name)
				}
			}

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
				err := b.client.Create(ctx, snapCont, &client.CreateOptions{})
				switch {
				case err != nil && kuberrs.IsAlreadyExists(err):
					log.Debugf("volumesnapshotcontent %s already exists in cluster", snapCont.Name)
					continue
				case err != nil && !kuberrs.IsAlreadyExists(err):
					log.WithError(errors.WithStack(err)).Errorf("Error syncing volumesnapshotcontent %s into cluster", snapCont.Name)
					continue
				default:
					log.Infof("Created CSI volumesnapshotcontent %s", snapCont.Name)
				}
			}
		}
	}

	b.deleteOrphanedBackups(ctx, location.Name, backupStoreBackups, log)

	// update the location's last-synced time field
	statusPatch := client.MergeFrom(location.DeepCopy())
	location.Status.LastSyncedTime = &metav1.Time{Time: time.Now().UTC()}
	if err := b.client.Patch(ctx, location, statusPatch); err != nil {
		log.WithError(errors.WithStack(err)).Error("Error patching backup location's last-synced time")
		return ctrl.Result{}, nil
	}

	return ctrl.Result{}, nil
}

// SetupWithManager is used to setup controller and its watching sources.
func (b *backupSyncReconciler) SetupWithManager(mgr ctrl.Manager) error {
	backupSyncSource := kube.NewPeriodicalEnqueueSource(
		b.logger,
		mgr.GetClient(),
		&velerov1api.BackupStorageLocationList{},
		backupSyncReconcilePeriod,
		kube.PeriodicalEnqueueSourceOption{
			OrderFunc: backupSyncSourceOrderFunc,
		},
	)

	gp := kube.NewGenericEventPredicate(func(object client.Object) bool {
		location := object.(*velerov1api.BackupStorageLocation)
		return b.locationFilterFunc(location)
	})

	return ctrl.NewControllerManagedBy(mgr).
		// Filter all BSL events, because this controller is supposed to run periodically, not by event.
		For(&velerov1api.BackupStorageLocation{}, builder.WithPredicates(kube.FalsePredicate{})).
		Watches(backupSyncSource, nil, builder.WithPredicates(gp)).
		Complete(b)
}

// deleteOrphanedBackups deletes backup objects (CRDs) from Kubernetes that have the specified location
// and a phase of Completed, but no corresponding backup in object storage.
func (b *backupSyncReconciler) deleteOrphanedBackups(ctx context.Context, locationName string, backupStoreBackups sets.String, log logrus.FieldLogger) {
	var backupList velerov1api.BackupList
	listOption := client.ListOptions{
		LabelSelector: labels.Set(map[string]string{
			velerov1api.StorageLocationLabel: label.GetValidName(locationName),
		}).AsSelector(),
	}
	err := b.client.List(ctx, &backupList, &listOption)
	if err != nil {
		log.WithError(errors.WithStack(err)).Error("Error listing backups from cluster")
		return
	}

	if len(backupList.Items) == 0 {
		return
	}

	for i, backup := range backupList.Items {
		log = log.WithField("backup", backup.Name)
		if backup.Status.Phase != velerov1api.BackupPhaseCompleted || backupStoreBackups.Has(backup.Name) {
			continue
		}

		if err := b.client.Delete(ctx, &backupList.Items[i], &client.DeleteOptions{}); err != nil {
			log.WithError(errors.WithStack(err)).Error("Error deleting orphaned backup from cluster")
		} else {
			log.Debug("Deleted orphaned backup from cluster")
			b.deleteCSISnapshotsByBackup(ctx, backup.Name, log)
		}
	}
}

func (b *backupSyncReconciler) deleteCSISnapshotsByBackup(ctx context.Context, backupName string, log logrus.FieldLogger) {
	if !features.IsEnabled(velerov1api.CSIFeatureFlag) {
		return
	}
	m := client.MatchingLabels{velerov1api.BackupNameLabel: label.GetValidName(backupName)}
	var vsList snapshotv1api.VolumeSnapshotList
	listOptions := &client.ListOptions{
		LabelSelector: label.NewSelectorForBackup(label.GetValidName(backupName)),
	}
	if err := b.client.List(ctx, &vsList, listOptions); err != nil {
		log.WithError(err).Warnf("Failed to list volumesnapshots for backup: %s, the deletion will be skipped", backupName)
	} else {
		for i, vs := range vsList.Items {
			name := kube.NamespaceAndName(vs.GetObjectMeta())
			log.Debugf("Deleting volumesnapshot %s", name)
			if err := b.client.Delete(context.TODO(), &vsList.Items[i]); err != nil {
				log.WithError(err).Warnf("Failed to delete volumesnapshot %s", name)
			}
		}
	}
	vsc := &snapshotv1api.VolumeSnapshotContent{}
	log.Debugf("Deleting volumesnapshotcontents for backup: %s", backupName)
	if err := b.client.DeleteAllOf(context.TODO(), vsc, m); err != nil {
		log.WithError(err).Warnf("Failed to delete volumesnapshotcontents for backup: %s", backupName)
	}
}

// backupSyncSourceOrderFunc returns a new slice with the default backup location first (if it exists),
// followed by the rest of the locations in no particular order.
func backupSyncSourceOrderFunc(objList client.ObjectList) client.ObjectList {
	inputBSLList := objList.(*velerov1api.BackupStorageLocationList)
	resultBSLList := &velerov1api.BackupStorageLocationList{}
	bslArray := make([]runtime.Object, 0)

	if len(inputBSLList.Items) <= 0 {
		return objList
	}

	for i := range inputBSLList.Items {
		location := inputBSLList.Items[i]

		// sync the default backup storage location first, if it exists
		if location.Spec.Default {
			// put the default location first
			bslArray = append(bslArray, &inputBSLList.Items[i])
			// append everything before the default
			for _, bsl := range inputBSLList.Items[:i] {
				cpBsl := bsl
				bslArray = append(bslArray, &cpBsl)
			}
			// append everything after the default
			for _, bsl := range inputBSLList.Items[i+1:] {
				cpBsl := bsl
				bslArray = append(bslArray, &cpBsl)
			}
			meta.SetList(resultBSLList, bslArray)

			return resultBSLList
		}
	}

	// No default BSL found. Return the input.
	return objList
}

func (b *backupSyncReconciler) locationFilterFunc(location *velerov1api.BackupStorageLocation) bool {
	syncPeriod := b.defaultBackupSyncPeriod
	if location.Spec.BackupSyncPeriod != nil {
		syncPeriod = location.Spec.BackupSyncPeriod.Duration
		if syncPeriod == 0 {
			b.logger.Debug("Backup sync period for this location is set to 0, skipping sync")
			return false
		}

		if syncPeriod < 0 {
			b.logger.Debug("Backup sync period must be non-negative")
			syncPeriod = b.defaultBackupSyncPeriod
		}
	}

	lastSync := location.Status.LastSyncedTime
	if lastSync != nil {
		b.logger.Debug("Checking if backups need to be synced at this time for this location")
		nextSync := lastSync.Add(syncPeriod)
		if time.Now().UTC().Before(nextSync) {
			return false
		}
	}
	return true
}
