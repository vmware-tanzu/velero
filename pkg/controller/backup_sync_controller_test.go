/*
Copyright 2017, 2020 the Velero contributors.

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
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/validation"
	core "k8s.io/client-go/testing"

	velerov1api "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	"github.com/vmware-tanzu/velero/pkg/builder"
	"github.com/vmware-tanzu/velero/pkg/generated/clientset/versioned/fake"
	informers "github.com/vmware-tanzu/velero/pkg/generated/informers/externalversions"
	"github.com/vmware-tanzu/velero/pkg/label"
	"github.com/vmware-tanzu/velero/pkg/persistence"
	persistencemocks "github.com/vmware-tanzu/velero/pkg/persistence/mocks"
	"github.com/vmware-tanzu/velero/pkg/plugin/clientmgmt"
	pluginmocks "github.com/vmware-tanzu/velero/pkg/plugin/mocks"
	velerotest "github.com/vmware-tanzu/velero/pkg/test"
)

func defaultLocationsList(namespace string) []*velerov1api.BackupStorageLocation {
	return []*velerov1api.BackupStorageLocation{
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
						Bucket: "bucket-2",
					},
				},
			},
		},
	}
}

func defaultLocationsListWithLongerLocationName(namespace string) []*velerov1api.BackupStorageLocation {
	return []*velerov1api.BackupStorageLocation{
		{
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
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: namespace,
				Name:      "the-really-long-location-name-that-is-much-more-than-63-characters-2",
			},
			Spec: velerov1api.BackupStorageLocationSpec{
				Provider: "objStoreProvider",
				StorageType: velerov1api.StorageType{
					ObjectStorage: &velerov1api.ObjectStorageLocation{
						Bucket: "bucket-2",
					},
				},
			},
		},
	}
}

func TestBackupSyncControllerRun(t *testing.T) {
	type cloudBackupData struct {
		backup           *velerov1api.Backup
		podVolumeBackups []*velerov1api.PodVolumeBackup
	}

	tests := []struct {
		name                     string
		namespace                string
		locations                []*velerov1api.BackupStorageLocation
		cloudBuckets             map[string][]*cloudBackupData
		existingBackups          []*velerov1api.Backup
		existingPodVolumeBackups []*velerov1api.PodVolumeBackup
		longLocationNameEnabled  bool
	}{
		{
			name: "no cloud backups",
		},
		{
			name:      "normal case",
			namespace: "ns-1",
			locations: defaultLocationsList("ns-1"),
			cloudBuckets: map[string][]*cloudBackupData{
				"bucket-1": {
					&cloudBackupData{
						backup: builder.ForBackup("ns-1", "backup-1").Result(),
					},
					&cloudBackupData{
						backup: builder.ForBackup("ns-1", "backup-2").Result(),
					},
				},
				"bucket-2": {
					&cloudBackupData{
						backup: builder.ForBackup("ns-1", "backup-3").Result(),
					},
				},
			},
		},
		{
			name:      "all synced backups get created in Velero server's namespace",
			namespace: "velero",
			locations: defaultLocationsList("velero"),
			cloudBuckets: map[string][]*cloudBackupData{
				"bucket-1": {
					&cloudBackupData{
						backup: builder.ForBackup("ns-1", "backup-1").Result(),
					},
					&cloudBackupData{
						backup: builder.ForBackup("ns-1", "backup-2").Result(),
					},
				},
				"bucket-2": {
					&cloudBackupData{
						backup: builder.ForBackup("ns-2", "backup-3").Result(),
					},
					&cloudBackupData{
						backup: builder.ForBackup("velero", "backup-4").Result(),
					},
				},
			},
		},
		{
			name:      "new backups get synced when some cloud backups already exist in the cluster",
			namespace: "ns-1",
			locations: defaultLocationsList("ns-1"),
			cloudBuckets: map[string][]*cloudBackupData{
				"bucket-1": {
					&cloudBackupData{
						backup: builder.ForBackup("ns-1", "backup-1").Result(),
					},
					&cloudBackupData{
						backup: builder.ForBackup("ns-1", "backup-2").Result(),
					},
				},
				"bucket-2": {
					&cloudBackupData{
						backup: builder.ForBackup("ns-1", "backup-3").Result(),
					},
					&cloudBackupData{
						backup: builder.ForBackup("ns-1", "backup-4").Result(),
					},
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
			locations: defaultLocationsList("ns-1"),
			cloudBuckets: map[string][]*cloudBackupData{
				"bucket-1": {
					&cloudBackupData{
						backup: builder.ForBackup("ns-1", "backup-1").Result(),
					},
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
			locations: defaultLocationsList("ns-1"),
			cloudBuckets: map[string][]*cloudBackupData{
				"bucket-1": {
					&cloudBackupData{
						backup: builder.ForBackup("ns-1", "backup-1").StorageLocation("foo").ObjectMeta(builder.WithLabels(velerov1api.StorageLocationLabel, "foo")).Result(),
					},
					&cloudBackupData{
						backup: builder.ForBackup("ns-1", "backup-2").Result(),
					},
				},
				"bucket-2": {
					&cloudBackupData{
						backup: builder.ForBackup("ns-1", "backup-3").StorageLocation("bar").ObjectMeta(builder.WithLabels(velerov1api.StorageLocationLabel, "bar")).Result(),
					},
				},
			},
		},
		{
			name:                    "backup storage location names and labels get updated with location name greater than 63 chars",
			namespace:               "ns-1",
			locations:               defaultLocationsListWithLongerLocationName("ns-1"),
			longLocationNameEnabled: true,
			cloudBuckets: map[string][]*cloudBackupData{
				"bucket-1": {
					&cloudBackupData{
						backup: builder.ForBackup("ns-1", "backup-1").StorageLocation("foo").ObjectMeta(builder.WithLabels(velerov1api.StorageLocationLabel, "foo")).Result(),
					},
					&cloudBackupData{
						backup: builder.ForBackup("ns-1", "backup-2").Result(),
					},
				},
				"bucket-2": {
					&cloudBackupData{
						backup: builder.ForBackup("ns-1", "backup-3").StorageLocation("bar").ObjectMeta(builder.WithLabels(velerov1api.StorageLocationLabel, "bar")).Result(),
					},
				},
			},
		},
		{
			name:      "all synced backups and pod volume backups get created in Velero server's namespace",
			namespace: "ns-1",
			locations: defaultLocationsList("ns-1"),
			cloudBuckets: map[string][]*cloudBackupData{
				"bucket-1": {
					&cloudBackupData{
						backup: builder.ForBackup("ns-1", "backup-1").Result(),
						podVolumeBackups: []*velerov1api.PodVolumeBackup{
							builder.ForPodVolumeBackup("ns-1", "pvb-1").Result(),
						},
					},
					&cloudBackupData{
						backup: builder.ForBackup("ns-1", "backup-2").Result(),
						podVolumeBackups: []*velerov1api.PodVolumeBackup{
							builder.ForPodVolumeBackup("ns-1", "pvb-2").Result(),
						},
					},
				},
				"bucket-2": {
					&cloudBackupData{
						backup: builder.ForBackup("ns-1", "backup-3").Result(),
					},
					&cloudBackupData{
						backup: builder.ForBackup("ns-1", "backup-4").Result(),
						podVolumeBackups: []*velerov1api.PodVolumeBackup{
							builder.ForPodVolumeBackup("ns-1", "pvb-1").Result(),
							builder.ForPodVolumeBackup("ns-1", "pvb-2").Result(),
							builder.ForPodVolumeBackup("ns-1", "pvb-3").Result(),
						},
					},
				},
			},
		},
		{
			name:      "new pod volume backups get synched when some pod volume backups already exist in the cluster",
			namespace: "ns-1",
			locations: defaultLocationsList("ns-1"),
			cloudBuckets: map[string][]*cloudBackupData{
				"bucket-1": {
					&cloudBackupData{
						backup: builder.ForBackup("ns-1", "backup-1").Result(),
						podVolumeBackups: []*velerov1api.PodVolumeBackup{
							builder.ForPodVolumeBackup("ns-1", "pvb-1").Result(),
						},
					},
					&cloudBackupData{
						backup: builder.ForBackup("ns-1", "backup-2").Result(),
						podVolumeBackups: []*velerov1api.PodVolumeBackup{
							builder.ForPodVolumeBackup("ns-1", "pvb-3").Result(),
						},
					},
				},
				"bucket-2": {
					&cloudBackupData{
						backup: builder.ForBackup("ns-1", "backup-3").Result(),
					},
					&cloudBackupData{
						backup: builder.ForBackup("ns-1", "backup-4").Result(),
						podVolumeBackups: []*velerov1api.PodVolumeBackup{
							builder.ForPodVolumeBackup("ns-1", "pvb-1").Result(),
							builder.ForPodVolumeBackup("ns-1", "pvb-5").Result(),
							builder.ForPodVolumeBackup("ns-1", "pvb-6").Result(),
						},
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
		t.Run(test.name, func(t *testing.T) {
			var (
				client          = fake.NewSimpleClientset()
				fakeClient      = newFakeClient(t)
				sharedInformers = informers.NewSharedInformerFactory(client, 0)
				pluginManager   = &pluginmocks.Manager{}
				backupStores    = make(map[string]*persistencemocks.BackupStore)
			)

			c := NewBackupSyncController(
				client.VeleroV1(),
				fakeClient,
				client.VeleroV1(),
				sharedInformers.Velero().V1().Backups().Lister(),
				time.Duration(0),
				test.namespace,
				nil, // csiSnapshotClient
				nil, // kubeClient
				"",
				func(logrus.FieldLogger) clientmgmt.Manager { return pluginManager },
				velerotest.NewLogger(),
			).(*backupSyncController)

			c.newBackupStore = func(loc *velerov1api.BackupStorageLocation, _ persistence.ObjectStoreGetter, _ logrus.FieldLogger) (persistence.BackupStore, error) {
				// this gets populated just below, prior to exercising the method under test
				return backupStores[loc.Name], nil
			}

			pluginManager.On("CleanupClients").Return(nil)

			for _, location := range test.locations {
				require.NoError(t, fakeClient.Create(context.Background(), location))
				backupStores[location.Name] = &persistencemocks.BackupStore{}
			}

			for _, location := range test.locations {
				backupStore, ok := backupStores[location.Name]
				require.True(t, ok, "no mock backup store for location %s", location.Name)

				var backupNames []string
				for _, bucket := range test.cloudBuckets[location.Spec.ObjectStorage.Bucket] {
					backupNames = append(backupNames, bucket.backup.Name)
					backupStore.On("GetBackupMetadata", bucket.backup.Name).Return(bucket.backup, nil)
					backupStore.On("GetPodVolumeBackups", bucket.backup.Name).Return(bucket.podVolumeBackups, nil)
				}
				backupStore.On("ListBackups").Return(backupNames, nil)
			}

			for _, existingBackup := range test.existingBackups {
				require.NoError(t, sharedInformers.Velero().V1().Backups().Informer().GetStore().Add(existingBackup))

				_, err := client.VeleroV1().Backups(test.namespace).Create(context.TODO(), existingBackup, metav1.CreateOptions{})
				require.NoError(t, err)
			}

			for _, existingPodVolumeBackup := range test.existingPodVolumeBackups {
				require.NoError(t, sharedInformers.Velero().V1().PodVolumeBackups().Informer().GetStore().Add(existingPodVolumeBackup))

				_, err := client.VeleroV1().PodVolumeBackups(test.namespace).Create(context.TODO(), existingPodVolumeBackup, metav1.CreateOptions{})
				require.NoError(t, err)
			}
			client.ClearActions()

			c.run()

			for bucket, backupDataSet := range test.cloudBuckets {
				// figure out which location this bucket is for; we need this for verification
				// purposes later
				var location *velerov1api.BackupStorageLocation
				for _, loc := range test.locations {
					if loc.Spec.ObjectStorage.Bucket == bucket {
						location = loc
						break
					}
				}
				require.NotNil(t, location)

				// process the cloud backups
				for _, cloudBackupData := range backupDataSet {
					obj, err := client.VeleroV1().Backups(test.namespace).Get(context.TODO(), cloudBackupData.backup.Name, metav1.GetOptions{})
					require.NoError(t, err)

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
						expected.Spec.StorageLocation = location.Name

						assert.Equal(t, expected, obj)
					} else {
						// verify that the storage location field and label are set properly
						assert.Equal(t, location.Name, obj.Spec.StorageLocation)

						locationName := location.Name
						if test.longLocationNameEnabled {
							locationName = label.GetValidName(locationName)
						}
						assert.Equal(t, locationName, obj.Labels[velerov1api.StorageLocationLabel])
						assert.Equal(t, true, len(obj.Labels[velerov1api.StorageLocationLabel]) <= validation.DNS1035LabelMaxLength)
					}

					// process the cloud pod volume backups for this backup, if any
					for _, podVolumeBackup := range cloudBackupData.podVolumeBackups {
						objPodVolumeBackup, err := client.VeleroV1().PodVolumeBackups(test.namespace).Get(context.TODO(), podVolumeBackup.Name, metav1.GetOptions{})
						require.NoError(t, err)

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
							assert.Equal(t, expected, objPodVolumeBackup)
						}
					}
				}
			}
		})
	}
}

func TestDeleteOrphanedBackups(t *testing.T) {
	baseBuilder := func(name string) *builder.BackupBuilder {
		return builder.ForBackup("ns-1", name).ObjectMeta(builder.WithLabels(velerov1api.StorageLocationLabel, "default"))
	}

	tests := []struct {
		name            string
		cloudBackups    sets.String
		k8sBackups      []*velerov1api.Backup
		namespace       string
		expectedDeletes sets.String
	}{
		{
			name:         "no overlapping backups",
			namespace:    "ns-1",
			cloudBackups: sets.NewString("backup-1", "backup-2", "backup-3"),
			k8sBackups: []*velerov1api.Backup{
				baseBuilder("backupA").Phase(velerov1api.BackupPhaseCompleted).Result(),
				baseBuilder("backupB").Phase(velerov1api.BackupPhaseCompleted).Result(),
				baseBuilder("backupC").Phase(velerov1api.BackupPhaseCompleted).Result(),
			},
			expectedDeletes: sets.NewString("backupA", "backupB", "backupC"),
		},
		{
			name:         "some overlapping backups",
			namespace:    "ns-1",
			cloudBackups: sets.NewString("backup-1", "backup-2", "backup-3"),
			k8sBackups: []*velerov1api.Backup{
				baseBuilder("backup-1").Phase(velerov1api.BackupPhaseCompleted).Result(),
				baseBuilder("backup-2").Phase(velerov1api.BackupPhaseCompleted).Result(),
				baseBuilder("backup-C").Phase(velerov1api.BackupPhaseCompleted).Result(),
			},
			expectedDeletes: sets.NewString("backup-C"),
		},
		{
			name:         "all overlapping backups",
			namespace:    "ns-1",
			cloudBackups: sets.NewString("backup-1", "backup-2", "backup-3"),
			k8sBackups: []*velerov1api.Backup{
				baseBuilder("backup-1").Phase(velerov1api.BackupPhaseCompleted).Result(),
				baseBuilder("backup-2").Phase(velerov1api.BackupPhaseCompleted).Result(),
				baseBuilder("backup-3").Phase(velerov1api.BackupPhaseCompleted).Result(),
			},
			expectedDeletes: sets.NewString(),
		},
		{
			name:         "no overlapping backups but including backups that are not complete",
			namespace:    "ns-1",
			cloudBackups: sets.NewString("backup-1", "backup-2", "backup-3"),
			k8sBackups: []*velerov1api.Backup{
				baseBuilder("backupA").Phase(velerov1api.BackupPhaseCompleted).Result(),
				baseBuilder("Deleting").Phase(velerov1api.BackupPhaseDeleting).Result(),
				baseBuilder("Failed").Phase(velerov1api.BackupPhaseFailed).Result(),
				baseBuilder("FailedValidation").Phase(velerov1api.BackupPhaseFailedValidation).Result(),
				baseBuilder("InProgress").Phase(velerov1api.BackupPhaseInProgress).Result(),
				baseBuilder("New").Phase(velerov1api.BackupPhaseNew).Result(),
			},
			expectedDeletes: sets.NewString("backupA"),
		},
		{
			name:         "all overlapping backups and all backups that are not complete",
			namespace:    "ns-1",
			cloudBackups: sets.NewString("backup-1", "backup-2", "backup-3"),
			k8sBackups: []*velerov1api.Backup{
				baseBuilder("backup-1").Phase(velerov1api.BackupPhaseFailed).Result(),
				baseBuilder("backup-2").Phase(velerov1api.BackupPhaseFailedValidation).Result(),
				baseBuilder("backup-3").Phase(velerov1api.BackupPhaseInProgress).Result(),
			},
			expectedDeletes: sets.NewString(),
		},
		{
			name:         "no completed backups in other locations are deleted",
			namespace:    "ns-1",
			cloudBackups: sets.NewString("backup-1", "backup-2", "backup-3"),
			k8sBackups: []*velerov1api.Backup{
				baseBuilder("backup-1").Phase(velerov1api.BackupPhaseCompleted).Result(),
				baseBuilder("backup-2").Phase(velerov1api.BackupPhaseCompleted).Result(),
				baseBuilder("backup-C").Phase(velerov1api.BackupPhaseCompleted).Result(),

				baseBuilder("backup-4").ObjectMeta(builder.WithLabels(velerov1api.StorageLocationLabel, "alternate")).Phase(velerov1api.BackupPhaseCompleted).Result(),
				baseBuilder("backup-5").ObjectMeta(builder.WithLabels(velerov1api.StorageLocationLabel, "alternate")).Phase(velerov1api.BackupPhaseCompleted).Result(),
				baseBuilder("backup-6").ObjectMeta(builder.WithLabels(velerov1api.StorageLocationLabel, "alternate")).Phase(velerov1api.BackupPhaseCompleted).Result(),
			},
			expectedDeletes: sets.NewString("backup-C"),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var (
				client          = fake.NewSimpleClientset()
				fakeClient      = newFakeClient(t)
				sharedInformers = informers.NewSharedInformerFactory(client, 0)
			)

			c := NewBackupSyncController(
				client.VeleroV1(),
				fakeClient,
				client.VeleroV1(),
				sharedInformers.Velero().V1().Backups().Lister(),
				time.Duration(0),
				test.namespace,
				nil, // csiSnapshotClient
				nil, // kubeClient
				"",
				nil, // new plugin manager func
				velerotest.NewLogger(),
			).(*backupSyncController)

			expectedDeleteActions := make([]core.Action, 0)

			for _, backup := range test.k8sBackups {
				// add test backup to informer
				require.NoError(t, sharedInformers.Velero().V1().Backups().Informer().GetStore().Add(backup), "Error adding backup to informer")

				// add test backup to client
				_, err := client.VeleroV1().Backups(test.namespace).Create(context.TODO(), backup, metav1.CreateOptions{})
				require.NoError(t, err, "Error adding backup to clientset")

				// if we expect this backup to be deleted, set up the expected DeleteAction
				if test.expectedDeletes.Has(backup.Name) {
					actionDelete := core.NewDeleteAction(
						velerov1api.SchemeGroupVersion.WithResource("backups"),
						test.namespace,
						backup.Name,
					)
					expectedDeleteActions = append(expectedDeleteActions, actionDelete)
				}
			}

			c.deleteOrphanedBackups("default", test.cloudBackups, velerotest.NewLogger())

			numBackups, err := numBackups(t, client, c.namespace)
			assert.NoError(t, err)

			expected := len(test.k8sBackups) - len(test.expectedDeletes)
			assert.Equal(t, expected, numBackups)

			velerotest.CompareActions(t, expectedDeleteActions, getDeleteActions(client.Actions()))
		})
	}
}

func TestStorageLabelsInDeleteOrphanedBackups(t *testing.T) {
	longLabelName := "the-really-long-location-name-that-is-much-more-than-63-characters"
	tests := []struct {
		name            string
		cloudBackups    sets.String
		k8sBackups      []*velerov1api.Backup
		namespace       string
		expectedDeletes sets.String
	}{
		{
			name:         "some overlapping backups",
			namespace:    "ns-1",
			cloudBackups: sets.NewString("backup-1", "backup-2", "backup-3"),
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
			},
			expectedDeletes: sets.NewString("backup-C"),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var (
				client          = fake.NewSimpleClientset()
				fakeClient      = newFakeClient(t)
				sharedInformers = informers.NewSharedInformerFactory(client, 0)
			)

			c := NewBackupSyncController(
				client.VeleroV1(),
				fakeClient,
				client.VeleroV1(),
				sharedInformers.Velero().V1().Backups().Lister(),
				time.Duration(0),
				test.namespace,
				nil, // csiSnapshotClient
				nil, // kubeClient
				"",
				nil, // new plugin manager func
				velerotest.NewLogger(),
			).(*backupSyncController)

			expectedDeleteActions := make([]core.Action, 0)

			for _, backup := range test.k8sBackups {
				// add test backup to informer
				require.NoError(t, sharedInformers.Velero().V1().Backups().Informer().GetStore().Add(backup), "Error adding backup to informer")

				// add test backup to client
				_, err := client.VeleroV1().Backups(test.namespace).Create(context.TODO(), backup, metav1.CreateOptions{})
				require.NoError(t, err, "Error adding backup to clientset")

				// if we expect this backup to be deleted, set up the expected DeleteAction
				if test.expectedDeletes.Has(backup.Name) {
					actionDelete := core.NewDeleteAction(
						velerov1api.SchemeGroupVersion.WithResource("backups"),
						test.namespace,
						backup.Name,
					)
					expectedDeleteActions = append(expectedDeleteActions, actionDelete)
				}
			}

			c.deleteOrphanedBackups(longLabelName, test.cloudBackups, velerotest.NewLogger())

			numBackups, err := numBackups(t, client, c.namespace)
			assert.NoError(t, err)

			expected := len(test.k8sBackups) - len(test.expectedDeletes)
			assert.Equal(t, expected, numBackups)

			velerotest.CompareActions(t, expectedDeleteActions, getDeleteActions(client.Actions()))
		})
	}
}

func getDeleteActions(actions []core.Action) []core.Action {
	var deleteActions []core.Action
	for _, action := range actions {
		if action.GetVerb() == "delete" {
			deleteActions = append(deleteActions, action)
		}
	}
	return deleteActions
}

func numBackups(t *testing.T, c *fake.Clientset, ns string) (int, error) {
	t.Helper()
	existingK8SBackups, err := c.VeleroV1().Backups(ns).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return 0, err
	}

	return len(existingK8SBackups.Items), nil
}

func numPodVolumeBackups(t *testing.T, c *fake.Clientset, ns string) (int, error) {
	t.Helper()
	existingK8SPodvolumeBackups, err := c.VeleroV1().PodVolumeBackups(ns).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return 0, err
	}

	return len(existingK8SPodvolumeBackups.Items), nil
}
