/*
Copyright the Velero contributors.

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
	"fmt"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	snapshotv1api "github.com/kubernetes-csi/external-snapshotter/client/v7/apis/volumesnapshot/v1"
	"github.com/sirupsen/logrus"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/validation"
	testclocks "k8s.io/utils/clock/testing"
	ctrl "sigs.k8s.io/controller-runtime"
	ctrlClient "sigs.k8s.io/controller-runtime/pkg/client"
	ctrlfake "sigs.k8s.io/controller-runtime/pkg/client/fake"

	velerov1api "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	"github.com/vmware-tanzu/velero/pkg/builder"
	"github.com/vmware-tanzu/velero/pkg/label"
	persistencemocks "github.com/vmware-tanzu/velero/pkg/persistence/mocks"
	"github.com/vmware-tanzu/velero/pkg/plugin/clientmgmt"
	pluginmocks "github.com/vmware-tanzu/velero/pkg/plugin/mocks"
	velerotest "github.com/vmware-tanzu/velero/pkg/test"
)

func defaultLocation(namespace string) *velerov1api.BackupStorageLocation {
	return &velerov1api.BackupStorageLocation{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      "location-1",
		},
		Spec: velerov1api.BackupStorageLocationSpec{
			Provider: "objStoreProvider",
			StorageType: velerov1api.StorageType{
				ObjectStorage: &velerov1api.ObjectStorageLocation{
					Bucket: "bucket-1",
				},
			},
			Default: true,
		},
	}
}

func defaultLocationsList(namespace string) []*velerov1api.BackupStorageLocation {
	return []*velerov1api.BackupStorageLocation{
		{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: namespace,
				Name:      "location-0",
			},
			Spec: velerov1api.BackupStorageLocationSpec{
				Provider: "objStoreProvider",
				StorageType: velerov1api.StorageType{
					ObjectStorage: &velerov1api.ObjectStorageLocation{
						Bucket: "bucket-1",
					},
				},
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: namespace,
				Name:      "location-1",
			},
			Spec: velerov1api.BackupStorageLocationSpec{
				Provider: "objStoreProvider",
				StorageType: velerov1api.StorageType{
					ObjectStorage: &velerov1api.ObjectStorageLocation{
						Bucket: "bucket-1",
					},
				},
				Default: true,
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: namespace,
				Name:      "location-2",
			},
			Spec: velerov1api.BackupStorageLocationSpec{
				Provider: "objStoreProvider",
				StorageType: velerov1api.StorageType{
					ObjectStorage: &velerov1api.ObjectStorageLocation{
						Bucket: "bucket-1",
					},
				},
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: namespace,
				Name:      "location-3",
			},
			Spec: velerov1api.BackupStorageLocationSpec{
				Provider: "objStoreProvider",
				StorageType: velerov1api.StorageType{
					ObjectStorage: &velerov1api.ObjectStorageLocation{
						Bucket: "bucket-1",
					},
				},
			},
		},
	}
}

func defaultLocationWithLongerLocationName(namespace string) *velerov1api.BackupStorageLocation {
	return &velerov1api.BackupStorageLocation{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      "the-really-long-location-name-that-is-much-more-than-63-characters-1",
		},
		Spec: velerov1api.BackupStorageLocationSpec{
			Provider: "objStoreProvider",
			StorageType: velerov1api.StorageType{
				ObjectStorage: &velerov1api.ObjectStorageLocation{
					Bucket: "bucket-1",
				},
			},
		},
	}
}

func numBackups(c ctrlClient.WithWatch) (int, error) {
	var existingK8SBackups velerov1api.BackupList
	err := c.List(context.TODO(), &existingK8SBackups, &ctrlClient.ListOptions{})
	if err != nil {
		return 0, err
	}

	return len(existingK8SBackups.Items), nil
}

var _ = Describe("Backup Sync Reconciler", func() {
	It("Test Backup Sync Reconciler basic function", func() {
		fakeClock := testclocks.NewFakeClock(time.Now())
		type cloudBackupData struct {
			backup               *velerov1api.Backup
			podVolumeBackups     []*velerov1api.PodVolumeBackup
			backupShouldSkipSync bool // backups waiting for plugin operations should not sync
		}

		tests := []struct {
			name                     string
			namespace                string
			location                 *velerov1api.BackupStorageLocation
			cloudBackups             []*cloudBackupData
			existingBackups          []*velerov1api.Backup
			existingPodVolumeBackups []*velerov1api.PodVolumeBackup
			longLocationNameEnabled  bool
		}{
			{
				name:      "no cloud backups",
				namespace: "ns-1",
				location:  defaultLocation("ns-1"),
			},
			{
				name:      "normal case",
				namespace: "ns-1",
				location:  defaultLocation("ns-1"),
				cloudBackups: []*cloudBackupData{
					{
						backup: builder.ForBackup("ns-1", "backup-1").Result(),
					},
					{
						backup: builder.ForBackup("ns-1", "backup-2").Result(),
					},
				},
			},
			{
				name:      "backups waiting for plugin operations aren't synced",
				namespace: "ns-1",
				location:  defaultLocation("ns-1"),
				cloudBackups: []*cloudBackupData{
					{
						backup: builder.ForBackup("ns-1", "backup-1").
							Phase(velerov1api.BackupPhaseWaitingForPluginOperations).Result(),
						backupShouldSkipSync: true,
					},
					{
						backup: builder.ForBackup("ns-1", "backup-2").
							Phase(velerov1api.BackupPhaseWaitingForPluginOperationsPartiallyFailed).Result(),
						backupShouldSkipSync: true,
					},
					{
						backup: builder.ForBackup("ns-1", "backup-3").
							Phase(velerov1api.BackupPhaseWaitingForPluginOperations).Result(),
						podVolumeBackups: []*velerov1api.PodVolumeBackup{
							builder.ForPodVolumeBackup("ns-1", "pvb-1").Result(),
						},
						backupShouldSkipSync: true,
					},
					{
						backup: builder.ForBackup("ns-1", "backup-4").
							Phase(velerov1api.BackupPhaseFinalizing).Result(),
						backupShouldSkipSync: true,
					},
					{
						backup: builder.ForBackup("ns-1", "backup-5").
							Phase(velerov1api.BackupPhaseFinalizingPartiallyFailed).Result(),
						backupShouldSkipSync: true,
					},
					{
						backup: builder.ForBackup("ns-1", "backup-6").
							Phase(velerov1api.BackupPhaseFinalizing).Result(),
						podVolumeBackups: []*velerov1api.PodVolumeBackup{
							builder.ForPodVolumeBackup("ns-1", "pvb-2").Result(),
						},
						backupShouldSkipSync: true,
					},
				},
			},
			{
				name:      "expired backups waiting for plugin operations are synced",
				namespace: "ns-1",
				location:  defaultLocation("ns-1"),
				cloudBackups: []*cloudBackupData{
					{
						backup: builder.ForBackup("ns-1", "backup-1").
							Phase(velerov1api.BackupPhaseWaitingForPluginOperations).
							Expiration(fakeClock.Now().Add(-time.Hour)).Result(),
						backupShouldSkipSync: true,
					},
					{
						backup: builder.ForBackup("ns-1", "backup-2").
							Phase(velerov1api.BackupPhaseWaitingForPluginOperationsPartiallyFailed).
							Expiration(fakeClock.Now().Add(-time.Hour)).Result(),
						backupShouldSkipSync: true,
					},
					{
						backup: builder.ForBackup("ns-1", "backup-3").
							Phase(velerov1api.BackupPhaseWaitingForPluginOperations).
							Expiration(fakeClock.Now().Add(-time.Hour)).Result(),
						podVolumeBackups: []*velerov1api.PodVolumeBackup{
							builder.ForPodVolumeBackup("ns-1", "pvb-1").Result(),
						},
						backupShouldSkipSync: true,
					},
					{
						backup: builder.ForBackup("ns-1", "backup-4").
							Phase(velerov1api.BackupPhaseFinalizing).
							Expiration(fakeClock.Now().Add(-time.Hour)).Result(),
						backupShouldSkipSync: true,
					},
					{
						backup: builder.ForBackup("ns-1", "backup-5").
							Phase(velerov1api.BackupPhaseFinalizingPartiallyFailed).
							Expiration(fakeClock.Now().Add(-time.Hour)).Result(),
						backupShouldSkipSync: true,
					},
					{
						backup: builder.ForBackup("ns-1", "backup-6").
							Phase(velerov1api.BackupPhaseFinalizing).
							Expiration(fakeClock.Now().Add(-time.Hour)).Result(),
						podVolumeBackups: []*velerov1api.PodVolumeBackup{
							builder.ForPodVolumeBackup("ns-1", "pvb-2").Result(),
						},
						backupShouldSkipSync: true,
					},
				},
			},
			{
				name:      "all synced backups get created in Velero server's namespace",
				namespace: "velero",
				location:  defaultLocation("velero"),
				cloudBackups: []*cloudBackupData{
					{
						backup: builder.ForBackup("ns-1", "backup-1").Result(),
					},
					{
						backup: builder.ForBackup("ns-1", "backup-2").Result(),
					},
				},
			},
			{
				name:      "new backups get synced when some cloud backups already exist in the cluster",
				namespace: "ns-1",
				location:  defaultLocation("ns-1"),
				cloudBackups: []*cloudBackupData{
					{
						backup: builder.ForBackup("ns-1", "backup-1").Result(),
					},
					{
						backup: builder.ForBackup("ns-1", "backup-2").Result(),
					},
				},
				existingBackups: []*velerov1api.Backup{
					// add a label to each existing backup so we can differentiate it from the cloud
					// backup during verification
					builder.ForBackup("ns-1", "backup-1").StorageLocation("location-1").ObjectMeta(builder.WithLabels("i-exist", "true")).Result(),
					builder.ForBackup("ns-1", "backup-3").StorageLocation("location-2").ObjectMeta(builder.WithLabels("i-exist", "true")).Result(),
				},
			},
			{
				name:      "existing backups without a StorageLocation get it filled in",
				namespace: "ns-1",
				location:  defaultLocation("ns-1"),
				cloudBackups: []*cloudBackupData{
					{
						backup: builder.ForBackup("ns-1", "backup-1").Result(),
					},
				},
				existingBackups: []*velerov1api.Backup{
					// add a label to each existing backup so we can differentiate it from the cloud
					// backup during verification
					builder.ForBackup("ns-1", "backup-1").ObjectMeta(builder.WithLabels("i-exist", "true")).StorageLocation("location-1").Result(),
				},
			},
			{
				name:      "backup storage location names and labels get updated",
				namespace: "ns-1",
				location:  defaultLocation("ns-1"),
				cloudBackups: []*cloudBackupData{
					{
						backup: builder.ForBackup("ns-1", "backup-1").StorageLocation("foo").ObjectMeta(builder.WithLabels(velerov1api.StorageLocationLabel, "foo")).Result(),
					},
					{
						backup: builder.ForBackup("ns-1", "backup-2").Result(),
					},
				},
			},
			{
				name:                    "backup storage location names and labels get updated with location name greater than 63 chars",
				namespace:               "ns-1",
				location:                defaultLocationWithLongerLocationName("ns-1"),
				longLocationNameEnabled: true,
				cloudBackups: []*cloudBackupData{
					{
						backup: builder.ForBackup("ns-1", "backup-1").StorageLocation("foo").ObjectMeta(builder.WithLabels(velerov1api.StorageLocationLabel, "foo")).Result(),
					},
					{
						backup: builder.ForBackup("ns-1", "backup-2").Result(),
					},
				},
			},
			{
				name:      "all synced backups and pod volume backups get created in Velero server's namespace",
				namespace: "ns-1",
				location:  defaultLocation("ns-1"),
				cloudBackups: []*cloudBackupData{
					{
						backup: builder.ForBackup("ns-1", "backup-1").Result(),
						podVolumeBackups: []*velerov1api.PodVolumeBackup{
							builder.ForPodVolumeBackup("ns-1", "pvb-1").Result(),
						},
					},
					{
						backup: builder.ForBackup("ns-1", "backup-2").Result(),
						podVolumeBackups: []*velerov1api.PodVolumeBackup{
							builder.ForPodVolumeBackup("ns-1", "pvb-2").Result(),
						},
					},
				},
			},
			{
				name:      "new pod volume backups get synched when some pod volume backups already exist in the cluster",
				namespace: "ns-1",
				location:  defaultLocation("ns-1"),
				cloudBackups: []*cloudBackupData{
					{
						backup: builder.ForBackup("ns-1", "backup-1").Result(),
						podVolumeBackups: []*velerov1api.PodVolumeBackup{
							builder.ForPodVolumeBackup("ns-1", "pvb-1").Result(),
						},
					},
					{
						backup: builder.ForBackup("ns-1", "backup-2").Result(),
						podVolumeBackups: []*velerov1api.PodVolumeBackup{
							builder.ForPodVolumeBackup("ns-1", "pvb-3").Result(),
						},
					},
				},
				existingPodVolumeBackups: []*velerov1api.PodVolumeBackup{
					builder.ForPodVolumeBackup("ns-1", "pvb-1").Result(),
					builder.ForPodVolumeBackup("ns-1", "pvb-2").Result(),
				},
			},
		}

		for _, test := range tests {
			var (
				client        = ctrlfake.NewClientBuilder().Build()
				pluginManager = &pluginmocks.Manager{}
				backupStores  = make(map[string]*persistencemocks.BackupStore)
			)

			pluginManager.On("CleanupClients").Return(nil)
			r := backupSyncReconciler{
				client:                  client,
				namespace:               test.namespace,
				defaultBackupSyncPeriod: time.Second * 10,
				newPluginManager:        func(logrus.FieldLogger) clientmgmt.Manager { return pluginManager },
				backupStoreGetter:       NewFakeObjectBackupStoreGetter(backupStores),
				logger:                  velerotest.NewLogger(),
			}

			if test.location != nil {
				Expect(r.client.Create(ctx, test.location)).ShouldNot(HaveOccurred())
				backupStores[test.location.Name] = &persistencemocks.BackupStore{}

				backupStore, ok := backupStores[test.location.Name]
				Expect(ok).To(BeTrue(), "no mock backup store for location %s", test.location.Name)

				var backupNames []string
				for _, backup := range test.cloudBackups {
					backupNames = append(backupNames, backup.backup.Name)
					backupStore.On("GetBackupMetadata", backup.backup.Name).Return(backup.backup, nil)
					backupStore.On("GetPodVolumeBackups", backup.backup.Name).Return(backup.podVolumeBackups, nil)
					backupStore.On("BackupExists", "bucket-1", backup.backup.Name).Return(true, nil)
					backupStore.On("GetCSIVolumeSnapshotClasses", backup.backup.Name).Return([]*snapshotv1api.VolumeSnapshotClass{}, nil)
					backupStore.On("GetCSIVolumeSnapshotContents", backup.backup.Name).Return([]*snapshotv1api.VolumeSnapshotContent{}, nil)
				}
				backupStore.On("ListBackups").Return(backupNames, nil)
			}

			for _, existingBackup := range test.existingBackups {
				err := client.Create(context.TODO(), existingBackup, &ctrlClient.CreateOptions{})
				Expect(err).ShouldNot(HaveOccurred())
			}

			for _, existingPodVolumeBackup := range test.existingPodVolumeBackups {
				err := client.Create(context.TODO(), existingPodVolumeBackup, &ctrlClient.CreateOptions{})
				Expect(err).ShouldNot(HaveOccurred())
			}

			actualResult, err := r.Reconcile(ctx, ctrl.Request{
				NamespacedName: types.NamespacedName{Namespace: test.location.Namespace, Name: test.location.Name},
			})

			Expect(actualResult).To(BeEquivalentTo(ctrl.Result{}))
			Expect(err).ToNot(HaveOccurred())

			// process the cloud backups
			for _, cloudBackupData := range test.cloudBackups {
				obj := &velerov1api.Backup{}
				err := client.Get(
					context.TODO(),
					types.NamespacedName{
						Namespace: cloudBackupData.backup.Namespace,
						Name:      cloudBackupData.backup.Name},
					obj)
				if cloudBackupData.backupShouldSkipSync &&
					(cloudBackupData.backup.Status.Expiration == nil ||
						cloudBackupData.backup.Status.Expiration.After(fakeClock.Now())) {
					Expect(apierrors.IsNotFound(err)).To(BeTrue())
				} else {
					Expect(err).ToNot(HaveOccurred())

					// did this cloud backup already exist in the cluster?
					var existing *velerov1api.Backup
					for _, obj := range test.existingBackups {
						if obj.Name == cloudBackupData.backup.Name {
							existing = obj
							break
						}
					}

					if existing != nil {
						// if this cloud backup already exists in the cluster, make sure that what we get from the
						// client is the existing backup, not the cloud one.

						// verify that the in-cluster backup has its storage location populated, if it's not already.
						expected := existing.DeepCopy()
						expected.Spec.StorageLocation = test.location.Name

						Expect(expected).To(BeEquivalentTo(obj))
					} else {
						// verify that the storage location field and label are set properly
						Expect(test.location.Name).To(BeEquivalentTo(obj.Spec.StorageLocation))

						locationName := test.location.Name
						if test.longLocationNameEnabled {
							locationName = label.GetValidName(locationName)
						}
						Expect(locationName).To(BeEquivalentTo(obj.Labels[velerov1api.StorageLocationLabel]))
						Expect(len(obj.Labels[velerov1api.StorageLocationLabel])).To(BeNumerically("<=", validation.DNS1035LabelMaxLength))
					}
				}

				// process the cloud pod volume backups for this backup, if any
				for _, podVolumeBackup := range cloudBackupData.podVolumeBackups {
					objPodVolumeBackup := &velerov1api.PodVolumeBackup{}
					err := client.Get(
						context.TODO(),
						types.NamespacedName{
							Namespace: podVolumeBackup.Namespace,
							Name:      podVolumeBackup.Name,
						},
						objPodVolumeBackup)
					if cloudBackupData.backupShouldSkipSync &&
						(cloudBackupData.backup.Status.Expiration == nil ||
							cloudBackupData.backup.Status.Expiration.After(fakeClock.Now())) {
						Expect(apierrors.IsNotFound(err)).To(BeTrue())
					} else {
						Expect(err).ShouldNot(HaveOccurred())

						// did this cloud pod volume backup already exist in the cluster?
						var existingPodVolumeBackup *velerov1api.PodVolumeBackup
						for _, objPodVolumeBackup := range test.existingPodVolumeBackups {
							if objPodVolumeBackup.Name == podVolumeBackup.Name {
								existingPodVolumeBackup = objPodVolumeBackup
								break
							}
						}

						if existingPodVolumeBackup != nil {
							// if this cloud pod volume backup already exists in the cluster, make sure that what we get from the
							// client is the existing backup, not the cloud one.
							expected := existingPodVolumeBackup.DeepCopy()
							Expect(expected).To(BeEquivalentTo(objPodVolumeBackup))
						}
					}
				}
			}
		}
	})

	It("Test deleting orphaned backups.", func() {
		longLabelName := "the-really-long-location-name-that-is-much-more-than-63-characters"

		baseBuilder := func(name string) *builder.BackupBuilder {
			return builder.ForBackup("ns-1", name).ObjectMeta(builder.WithLabels(velerov1api.StorageLocationLabel, "default"))
		}

		tests := []struct {
			name            string
			cloudBackups    sets.Set[string]
			k8sBackups      []*velerov1api.Backup
			namespace       string
			expectedDeletes sets.Set[string]
			useLongBSLName  bool
		}{
			{
				name:         "no overlapping backups",
				namespace:    "ns-1",
				cloudBackups: sets.New[string]("backup-1", "backup-2", "backup-3"),
				k8sBackups: []*velerov1api.Backup{
					baseBuilder("backupA").Phase(velerov1api.BackupPhaseCompleted).Result(),
					baseBuilder("backupB").Phase(velerov1api.BackupPhaseCompleted).Result(),
					baseBuilder("backupC").Phase(velerov1api.BackupPhasePartiallyFailed).Result(),
				},
				expectedDeletes: sets.New[string]("backupA", "backupB", "backupC"),
			},
			{
				name:         "some overlapping backups",
				namespace:    "ns-1",
				cloudBackups: sets.New[string]("backup-1", "backup-2", "backup-3"),
				k8sBackups: []*velerov1api.Backup{
					baseBuilder("backup-1").Phase(velerov1api.BackupPhaseCompleted).Result(),
					baseBuilder("backup-2").Phase(velerov1api.BackupPhaseCompleted).Result(),
					baseBuilder("backup-B").Phase(velerov1api.BackupPhaseCompleted).Result(),
					baseBuilder("backup-C").Phase(velerov1api.BackupPhasePartiallyFailed).Result(),
				},
				expectedDeletes: sets.New[string]("backup-B", "backup-C"),
			},
			{
				name:         "all overlapping backups",
				namespace:    "ns-1",
				cloudBackups: sets.New[string]("backup-1", "backup-2", "backup-3"),
				k8sBackups: []*velerov1api.Backup{
					baseBuilder("backup-1").Phase(velerov1api.BackupPhaseCompleted).Result(),
					baseBuilder("backup-2").Phase(velerov1api.BackupPhaseCompleted).Result(),
					baseBuilder("backup-3").Phase(velerov1api.BackupPhasePartiallyFailed).Result(),
				},
				expectedDeletes: sets.New[string](),
			},
			{
				name:         "no overlapping backups but including backups that are not complete",
				namespace:    "ns-1",
				cloudBackups: sets.New[string]("backup-1", "backup-2", "backup-3"),
				k8sBackups: []*velerov1api.Backup{
					baseBuilder("backupA").Phase(velerov1api.BackupPhaseCompleted).Result(),
					baseBuilder("backupB").Phase(velerov1api.BackupPhasePartiallyFailed).Result(),
					baseBuilder("Deleting").Phase(velerov1api.BackupPhaseDeleting).Result(),
					baseBuilder("Failed").Phase(velerov1api.BackupPhaseFailed).Result(),
					baseBuilder("FailedValidation").Phase(velerov1api.BackupPhaseFailedValidation).Result(),
					baseBuilder("InProgress").Phase(velerov1api.BackupPhaseInProgress).Result(),
					baseBuilder("New").Phase(velerov1api.BackupPhaseNew).Result(),
				},
				expectedDeletes: sets.New[string]("backupA", "backupB"),
			},
			{
				name:         "all overlapping backups and all backups that are not complete",
				namespace:    "ns-1",
				cloudBackups: sets.New[string]("backup-1", "backup-2", "backup-3"),
				k8sBackups: []*velerov1api.Backup{
					baseBuilder("backup-1").Phase(velerov1api.BackupPhaseFailed).Result(),
					baseBuilder("backup-2").Phase(velerov1api.BackupPhaseFailedValidation).Result(),
					baseBuilder("backup-3").Phase(velerov1api.BackupPhaseInProgress).Result(),
				},
				expectedDeletes: sets.New[string](),
			},
			{
				name:         "no completed backups in other locations are deleted",
				namespace:    "ns-1",
				cloudBackups: sets.New[string]("backup-1", "backup-2", "backup-3"),
				k8sBackups: []*velerov1api.Backup{
					baseBuilder("backup-1").Phase(velerov1api.BackupPhaseCompleted).Result(),
					baseBuilder("backup-2").Phase(velerov1api.BackupPhaseCompleted).Result(),
					baseBuilder("backup-C").Phase(velerov1api.BackupPhaseCompleted).Result(),
					baseBuilder("backup-D").Phase(velerov1api.BackupPhasePartiallyFailed).Result(),

					baseBuilder("backup-4").ObjectMeta(builder.WithLabels(velerov1api.StorageLocationLabel, "alternate")).Phase(velerov1api.BackupPhaseCompleted).Result(),
					baseBuilder("backup-5").ObjectMeta(builder.WithLabels(velerov1api.StorageLocationLabel, "alternate")).Phase(velerov1api.BackupPhaseCompleted).Result(),
					baseBuilder("backup-6").ObjectMeta(builder.WithLabels(velerov1api.StorageLocationLabel, "alternate")).Phase(velerov1api.BackupPhasePartiallyFailed).Result(),
				},
				expectedDeletes: sets.New[string]("backup-C", "backup-D"),
			},
			{
				name:         "some overlapping backups",
				namespace:    "ns-1",
				cloudBackups: sets.New[string]("backup-1", "backup-2", "backup-3"),
				k8sBackups: []*velerov1api.Backup{
					builder.ForBackup("ns-1", "backup-1").
						ObjectMeta(
							builder.WithLabels(velerov1api.StorageLocationLabel, "the-really-long-location-name-that-is-much-more-than-63-c69e779"),
						).
						Phase(velerov1api.BackupPhaseCompleted).
						Result(),
					builder.ForBackup("ns-1", "backup-2").
						ObjectMeta(
							builder.WithLabels(velerov1api.StorageLocationLabel, "the-really-long-location-name-that-is-much-more-than-63-c69e779"),
						).
						Phase(velerov1api.BackupPhaseCompleted).
						Result(),
					builder.ForBackup("ns-1", "backup-C").
						ObjectMeta(
							builder.WithLabels(velerov1api.StorageLocationLabel, "the-really-long-location-name-that-is-much-more-than-63-c69e779"),
						).
						Phase(velerov1api.BackupPhaseCompleted).
						Result(),
					builder.ForBackup("ns-1", "backup-D").
						ObjectMeta(
							builder.WithLabels(velerov1api.StorageLocationLabel, "the-really-long-location-name-that-is-much-more-than-63-c69e779"),
						).
						Phase(velerov1api.BackupPhasePartiallyFailed).
						Result(),
				},
				expectedDeletes: sets.New[string]("backup-C", "backup-D"),
				useLongBSLName:  true,
			},
		}

		for _, test := range tests {
			var (
				client        = ctrlfake.NewClientBuilder().Build()
				pluginManager = &pluginmocks.Manager{}
				backupStores  = make(map[string]*persistencemocks.BackupStore)
			)

			r := backupSyncReconciler{
				client:                  client,
				namespace:               test.namespace,
				defaultBackupSyncPeriod: time.Second * 10,
				newPluginManager:        func(logrus.FieldLogger) clientmgmt.Manager { return pluginManager },
				backupStoreGetter:       NewFakeObjectBackupStoreGetter(backupStores),
				logger:                  velerotest.NewLogger(),
			}

			for _, backup := range test.k8sBackups {
				// add test backup to client
				err := client.Create(context.TODO(), backup, &ctrlClient.CreateOptions{})
				Expect(err).ShouldNot(HaveOccurred())
			}

			bslName := "default"
			if test.useLongBSLName {
				bslName = longLabelName
			}
			r.deleteOrphanedBackups(ctx, bslName, test.cloudBackups, velerotest.NewLogger())

			numBackups, err := numBackups(client)
			Expect(err).ShouldNot(HaveOccurred())

			fmt.Println("")

			expected := len(test.k8sBackups) - len(test.expectedDeletes)
			Expect(expected).To(BeEquivalentTo(numBackups))
		}
	})

	It("Test moving default BSL at the head of BSL array.", func() {
		locationList := &velerov1api.BackupStorageLocationList{}
		objArray := make([]runtime.Object, 0)

		// Generate BSL array.
		locations := defaultLocationsList("velero")
		for _, bsl := range locations {
			objArray = append(objArray, bsl)
		}

		meta.SetList(locationList, objArray)

		testObjList := backupSyncSourceOrderFunc(locationList)
		testObjArray, err := meta.ExtractList(testObjList)
		Expect(err).ShouldNot(HaveOccurred())

		expectLocation := testObjArray[0].(*velerov1api.BackupStorageLocation)
		Expect(expectLocation.Spec.Default).To(BeEquivalentTo(true))

		// If BSL list without default BSL is passed in, the output should be same with input.
		locationList.Items = testObjList.(*velerov1api.BackupStorageLocationList).Items[1:]
		testObjList = backupSyncSourceOrderFunc(locationList)
		Expect(testObjList).To(BeEquivalentTo(locationList))

	})

	When("testing validateOwnerReferences", func() {

		testCases := []struct {
			name               string
			backup             *velerov1api.Backup
			toCreate           []ctrlClient.Object
			expectedReferences []metav1.OwnerReference
		}{
			{
				name: "handles empty owner references",
				backup: &velerov1api.Backup{
					ObjectMeta: metav1.ObjectMeta{
						OwnerReferences: []metav1.OwnerReference{},
					},
				},
				expectedReferences: []metav1.OwnerReference{},
			},
			{
				name: "handles missing schedule",
				backup: &velerov1api.Backup{
					ObjectMeta: metav1.ObjectMeta{
						OwnerReferences: []metav1.OwnerReference{
							{
								Kind: "Schedule",
								Name: "some name",
							},
						},
					},
				},
				expectedReferences: []metav1.OwnerReference{},
			},
			{
				name: "handles existing reference",
				backup: &velerov1api.Backup{
					ObjectMeta: metav1.ObjectMeta{
						OwnerReferences: []metav1.OwnerReference{
							{
								Kind: "Schedule",
								Name: "existing-schedule",
							},
						},
						Namespace: "test-namespace",
					},
				},
				toCreate: []ctrlClient.Object{
					&velerov1api.Schedule{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "existing-schedule",
							Namespace: "test-namespace",
						},
					},
				},
				expectedReferences: []metav1.OwnerReference{
					{
						Kind: "Schedule",
						Name: "existing-schedule",
					},
				},
			},
			{
				name: "handles existing mismatched UID",
				backup: &velerov1api.Backup{
					ObjectMeta: metav1.ObjectMeta{
						OwnerReferences: []metav1.OwnerReference{
							{
								Kind: "Schedule",
								Name: "existing-schedule",
								UID:  "backup-UID",
							},
						},
						Namespace: "test-namespace",
					},
				},
				toCreate: []ctrlClient.Object{
					&velerov1api.Schedule{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "existing-schedule",
							Namespace: "test-namespace",
							UID:       "schedule-UID",
						},
					},
				},
				expectedReferences: []metav1.OwnerReference{},
			},
			{
				name: "handles multiple references",
				backup: &velerov1api.Backup{
					ObjectMeta: metav1.ObjectMeta{
						OwnerReferences: []metav1.OwnerReference{
							{
								Kind: "Schedule",
								Name: "existing-schedule",
								UID:  "1",
							},
							{
								Kind: "Schedule",
								Name: "missing-schedule",
								UID:  "2",
							},
							{
								Kind: "Schedule",
								Name: "mismatched-uid-schedule",
								UID:  "3",
							},
							{
								Kind: "Schedule",
								Name: "another-existing-schedule",
								UID:  "4",
							},
						},
						Namespace: "test-namespace",
					},
				},
				toCreate: []ctrlClient.Object{
					&velerov1api.Schedule{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "existing-schedule",
							Namespace: "test-namespace",
							UID:       "1",
						},
					},
					&velerov1api.Schedule{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "mismatched-uid-schedule",
							Namespace: "test-namespace",
							UID:       "not-3",
						},
					},
					&velerov1api.Schedule{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "another-existing-schedule",
							Namespace: "test-namespace",
							UID:       "4",
						},
					},
				},
				expectedReferences: []metav1.OwnerReference{
					{
						Kind: "Schedule",
						Name: "existing-schedule",
						UID:  "1",
					},
					{
						Kind: "Schedule",
						Name: "another-existing-schedule",
						UID:  "4",
					},
				},
			},
		}
		for _, test := range testCases {
			test := test
			It(test.name, func() {
				logger := velerotest.NewLogger()
				b := backupSyncReconciler{
					client: ctrlfake.NewClientBuilder().Build(),
				}

				//create all required schedules as needed.
				for _, creatable := range test.toCreate {
					err := b.client.Create(context.Background(), creatable)
					Expect(err).ShouldNot(HaveOccurred())
				}

				references := b.filterBackupOwnerReferences(context.Background(), test.backup, logger)
				Expect(references).To(BeEquivalentTo(test.expectedReferences))
			})
		}
	})
})
