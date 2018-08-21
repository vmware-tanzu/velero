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
	"testing"
	"time"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/sirupsen/logrus"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	core "k8s.io/client-go/testing"

	arkv1api "github.com/heptio/ark/pkg/apis/ark/v1"
	"github.com/heptio/ark/pkg/cloudprovider"
	"github.com/heptio/ark/pkg/generated/clientset/versioned/fake"
	informers "github.com/heptio/ark/pkg/generated/informers/externalversions"
	"github.com/heptio/ark/pkg/plugin"
	pluginmocks "github.com/heptio/ark/pkg/plugin/mocks"
	"github.com/heptio/ark/pkg/util/stringslice"
	arktest "github.com/heptio/ark/pkg/util/test"
	"github.com/stretchr/testify/assert"
)

func defaultLocationsList(namespace string) []*arkv1api.BackupStorageLocation {
	return []*arkv1api.BackupStorageLocation{
		{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: namespace,
				Name:      "location-1",
			},
			Spec: arkv1api.BackupStorageLocationSpec{
				Provider: "objStoreProvider",
				StorageType: arkv1api.StorageType{
					ObjectStorage: &arkv1api.ObjectStorageLocation{
						Bucket: "bucket-1",
					},
				},
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: namespace,
				Name:      "location-2",
			},
			Spec: arkv1api.BackupStorageLocationSpec{
				Provider: "objStoreProvider",
				StorageType: arkv1api.StorageType{
					ObjectStorage: &arkv1api.ObjectStorageLocation{
						Bucket: "bucket-2",
					},
				},
			},
		},
	}
}

func TestBackupSyncControllerRun(t *testing.T) {
	tests := []struct {
		name            string
		namespace       string
		locations       []*arkv1api.BackupStorageLocation
		cloudBackups    map[string][]*arkv1api.Backup
		existingBackups []*arkv1api.Backup
	}{
		{
			name: "no cloud backups",
		},
		{
			name:      "normal case",
			namespace: "ns-1",
			locations: defaultLocationsList("ns-1"),
			cloudBackups: map[string][]*arkv1api.Backup{
				"bucket-1": {
					arktest.NewTestBackup().WithNamespace("ns-1").WithName("backup-1").Backup,
					arktest.NewTestBackup().WithNamespace("ns-1").WithName("backup-2").Backup,
				},
				"bucket-2": {
					arktest.NewTestBackup().WithNamespace("ns-1").WithName("backup-3").Backup,
				},
			},
		},
		{
			name:      "gcFinalizer (only) gets removed on sync",
			namespace: "ns-1",
			locations: defaultLocationsList("ns-1"),
			cloudBackups: map[string][]*arkv1api.Backup{
				"bucket-1": {
					arktest.NewTestBackup().WithNamespace("ns-1").WithFinalizers("a-finalizer", gcFinalizer, "some-other-finalizer").Backup,
				},
			},
		},
		{
			name:      "all synced backups get created in Ark server's namespace",
			namespace: "heptio-ark",
			locations: defaultLocationsList("heptio-ark"),
			cloudBackups: map[string][]*arkv1api.Backup{
				"bucket-1": {
					arktest.NewTestBackup().WithNamespace("ns-1").WithName("backup-1").Backup,
					arktest.NewTestBackup().WithNamespace("ns-1").WithName("backup-2").Backup,
				},
				"bucket-2": {
					arktest.NewTestBackup().WithNamespace("ns-2").WithName("backup-3").Backup,
					arktest.NewTestBackup().WithNamespace("heptio-ark").WithName("backup-4").Backup,
				},
			},
		},
		{
			name:      "new backups get synced when some cloud backups already exist in the cluster",
			namespace: "ns-1",
			locations: defaultLocationsList("ns-1"),
			cloudBackups: map[string][]*arkv1api.Backup{
				"bucket-1": {
					arktest.NewTestBackup().WithNamespace("ns-1").WithName("backup-1").Backup,
					arktest.NewTestBackup().WithNamespace("ns-1").WithName("backup-2").Backup,
				},
				"bucket-2": {
					arktest.NewTestBackup().WithNamespace("ns-1").WithName("backup-3").Backup,
					arktest.NewTestBackup().WithNamespace("ns-1").WithName("backup-4").Backup,
				},
			},
			existingBackups: []*arkv1api.Backup{
				// add a label to each existing backup so we can differentiate it from the cloud
				// backup during verification
				arktest.NewTestBackup().WithNamespace("ns-1").WithName("backup-1").WithLabel("i-exist", "true").Backup,
				arktest.NewTestBackup().WithNamespace("ns-1").WithName("backup-3").WithLabel("i-exist", "true").Backup,
			},
		},
		{
			name:      "backup storage location names and labels get updated",
			namespace: "ns-1",
			locations: defaultLocationsList("ns-1"),
			cloudBackups: map[string][]*arkv1api.Backup{
				"bucket-1": {
					arktest.NewTestBackup().WithNamespace("ns-1").WithName("backup-1").WithStorageLocation("foo").WithLabel(arkv1api.StorageLocationLabel, "foo").Backup,
					arktest.NewTestBackup().WithNamespace("ns-1").WithName("backup-2").Backup,
				},
				"bucket-2": {
					arktest.NewTestBackup().WithNamespace("ns-1").WithName("backup-3").WithStorageLocation("bar").WithLabel(arkv1api.StorageLocationLabel, "bar").Backup,
				},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var (
				client          = fake.NewSimpleClientset()
				sharedInformers = informers.NewSharedInformerFactory(client, 0)
				pluginManager   = &pluginmocks.Manager{}
				objectStore     = &arktest.ObjectStore{}
			)

			c := NewBackupSyncController(
				client.ArkV1(),
				sharedInformers.Ark().V1().Backups(),
				sharedInformers.Ark().V1().BackupStorageLocations(),
				time.Duration(0),
				test.namespace,
				nil, // pluginRegistry
				arktest.NewLogger(),
				logrus.DebugLevel,
			).(*backupSyncController)

			c.newPluginManager = func(_ logrus.FieldLogger) plugin.Manager { return pluginManager }
			pluginManager.On("GetObjectStore", "objStoreProvider").Return(objectStore, nil)
			pluginManager.On("CleanupClients").Return(nil)
			objectStore.On("Init", mock.Anything).Return(nil)

			for _, location := range test.locations {
				require.NoError(t, sharedInformers.Ark().V1().BackupStorageLocations().Informer().GetStore().Add(location))
			}

			c.listCloudBackups = func(_ logrus.FieldLogger, _ cloudprovider.ObjectStore, bucket string) ([]*arkv1api.Backup, error) {
				backups, ok := test.cloudBackups[bucket]
				if !ok {
					return nil, errors.New("bucket not found")
				}

				return backups, nil
			}

			for _, existingBackup := range test.existingBackups {
				require.NoError(t, sharedInformers.Ark().V1().Backups().Informer().GetStore().Add(existingBackup))

				_, err := client.ArkV1().Backups(test.namespace).Create(existingBackup)
				require.NoError(t, err)
			}
			client.ClearActions()

			c.run()

			for bucket, backups := range test.cloudBackups {
				for _, cloudBackup := range backups {
					obj, err := client.ArkV1().Backups(test.namespace).Get(cloudBackup.Name, metav1.GetOptions{})
					require.NoError(t, err)

					// did this cloud backup already exist in the cluster?
					var existing *arkv1api.Backup
					for _, obj := range test.existingBackups {
						if obj.Name == cloudBackup.Name {
							existing = obj
							break
						}
					}

					if existing != nil {
						// if this cloud backup already exists in the cluster, make sure that what we get from the
						// client is the existing backup, not the cloud one.
						assert.Equal(t, existing, obj)
					} else {
						// verify that the GC finalizer is removed
						assert.Equal(t, stringslice.Except(cloudBackup.Finalizers, gcFinalizer), obj.Finalizers)

						// verify that the storage location field and label are set properly
						for _, location := range test.locations {
							if location.Spec.ObjectStorage.Bucket == bucket {
								assert.Equal(t, location.Name, obj.Spec.StorageLocation)
								assert.Equal(t, location.Name, obj.Labels[arkv1api.StorageLocationLabel])
								break
							}
						}
					}
				}
			}
		})
	}
}

func TestDeleteOrphanedBackups(t *testing.T) {
	tests := []struct {
		name            string
		cloudBackups    sets.String
		k8sBackups      []*arktest.TestBackup
		namespace       string
		expectedDeletes sets.String
	}{
		{
			name:         "no overlapping backups",
			namespace:    "ns-1",
			cloudBackups: sets.NewString("backup-1", "backup-2", "backup-3"),
			k8sBackups: []*arktest.TestBackup{
				arktest.NewTestBackup().WithNamespace("ns-1").WithName("backupA").WithLabel(arkv1api.StorageLocationLabel, "default").WithPhase(arkv1api.BackupPhaseCompleted),
				arktest.NewTestBackup().WithNamespace("ns-1").WithName("backupB").WithLabel(arkv1api.StorageLocationLabel, "default").WithPhase(arkv1api.BackupPhaseCompleted),
				arktest.NewTestBackup().WithNamespace("ns-1").WithName("backupC").WithLabel(arkv1api.StorageLocationLabel, "default").WithPhase(arkv1api.BackupPhaseCompleted),
			},
			expectedDeletes: sets.NewString("backupA", "backupB", "backupC"),
		},
		{
			name:         "some overlapping backups",
			namespace:    "ns-1",
			cloudBackups: sets.NewString("backup-1", "backup-2", "backup-3"),
			k8sBackups: []*arktest.TestBackup{
				arktest.NewTestBackup().WithNamespace("ns-1").WithName("backup-1").WithLabel(arkv1api.StorageLocationLabel, "default").WithPhase(arkv1api.BackupPhaseCompleted),
				arktest.NewTestBackup().WithNamespace("ns-1").WithName("backup-2").WithLabel(arkv1api.StorageLocationLabel, "default").WithPhase(arkv1api.BackupPhaseCompleted),
				arktest.NewTestBackup().WithNamespace("ns-1").WithName("backup-C").WithLabel(arkv1api.StorageLocationLabel, "default").WithPhase(arkv1api.BackupPhaseCompleted),
			},
			expectedDeletes: sets.NewString("backup-C"),
		},
		{
			name:         "all overlapping backups",
			namespace:    "ns-1",
			cloudBackups: sets.NewString("backup-1", "backup-2", "backup-3"),
			k8sBackups: []*arktest.TestBackup{
				arktest.NewTestBackup().WithNamespace("ns-1").WithName("backup-1").WithLabel(arkv1api.StorageLocationLabel, "default").WithPhase(arkv1api.BackupPhaseCompleted),
				arktest.NewTestBackup().WithNamespace("ns-1").WithName("backup-2").WithLabel(arkv1api.StorageLocationLabel, "default").WithPhase(arkv1api.BackupPhaseCompleted),
				arktest.NewTestBackup().WithNamespace("ns-1").WithName("backup-3").WithLabel(arkv1api.StorageLocationLabel, "default").WithPhase(arkv1api.BackupPhaseCompleted),
			},
			expectedDeletes: sets.NewString(),
		},
		{
			name:         "no overlapping backups but including backups that are not complete",
			namespace:    "ns-1",
			cloudBackups: sets.NewString("backup-1", "backup-2", "backup-3"),
			k8sBackups: []*arktest.TestBackup{
				arktest.NewTestBackup().WithNamespace("ns-1").WithName("backupA").WithLabel(arkv1api.StorageLocationLabel, "default").WithPhase(arkv1api.BackupPhaseCompleted),
				arktest.NewTestBackup().WithNamespace("ns-1").WithName("Deleting").WithLabel(arkv1api.StorageLocationLabel, "default").WithPhase(arkv1api.BackupPhaseDeleting),
				arktest.NewTestBackup().WithNamespace("ns-1").WithName("Failed").WithLabel(arkv1api.StorageLocationLabel, "default").WithPhase(arkv1api.BackupPhaseFailed),
				arktest.NewTestBackup().WithNamespace("ns-1").WithName("FailedValidation").WithLabel(arkv1api.StorageLocationLabel, "default").WithPhase(arkv1api.BackupPhaseFailedValidation),
				arktest.NewTestBackup().WithNamespace("ns-1").WithName("InProgress").WithLabel(arkv1api.StorageLocationLabel, "default").WithPhase(arkv1api.BackupPhaseInProgress),
				arktest.NewTestBackup().WithNamespace("ns-1").WithName("New").WithLabel(arkv1api.StorageLocationLabel, "default").WithPhase(arkv1api.BackupPhaseNew),
			},
			expectedDeletes: sets.NewString("backupA"),
		},
		{
			name:         "all overlapping backups and all backups that are not complete",
			namespace:    "ns-1",
			cloudBackups: sets.NewString("backup-1", "backup-2", "backup-3"),
			k8sBackups: []*arktest.TestBackup{
				arktest.NewTestBackup().WithNamespace("ns-1").WithName("backup-1").WithLabel(arkv1api.StorageLocationLabel, "default").WithPhase(arkv1api.BackupPhaseFailed),
				arktest.NewTestBackup().WithNamespace("ns-1").WithName("backup-2").WithLabel(arkv1api.StorageLocationLabel, "default").WithPhase(arkv1api.BackupPhaseFailedValidation),
				arktest.NewTestBackup().WithNamespace("ns-1").WithName("backup-3").WithLabel(arkv1api.StorageLocationLabel, "default").WithPhase(arkv1api.BackupPhaseInProgress),
			},
			expectedDeletes: sets.NewString(),
		},
		{
			name:         "no completed backups in other locations are deleted",
			namespace:    "ns-1",
			cloudBackups: sets.NewString("backup-1", "backup-2", "backup-3"),
			k8sBackups: []*arktest.TestBackup{
				arktest.NewTestBackup().WithNamespace("ns-1").WithName("backup-1").WithLabel(arkv1api.StorageLocationLabel, "default").WithPhase(arkv1api.BackupPhaseCompleted),
				arktest.NewTestBackup().WithNamespace("ns-1").WithName("backup-2").WithLabel(arkv1api.StorageLocationLabel, "default").WithPhase(arkv1api.BackupPhaseCompleted),
				arktest.NewTestBackup().WithNamespace("ns-1").WithName("backup-C").WithLabel(arkv1api.StorageLocationLabel, "default").WithPhase(arkv1api.BackupPhaseCompleted),
				arktest.NewTestBackup().WithNamespace("ns-1").WithName("backup-4").WithLabel(arkv1api.StorageLocationLabel, "alternate").WithPhase(arkv1api.BackupPhaseCompleted),
				arktest.NewTestBackup().WithNamespace("ns-1").WithName("backup-5").WithLabel(arkv1api.StorageLocationLabel, "alternate").WithPhase(arkv1api.BackupPhaseCompleted),
				arktest.NewTestBackup().WithNamespace("ns-1").WithName("backup-6").WithLabel(arkv1api.StorageLocationLabel, "alternate").WithPhase(arkv1api.BackupPhaseCompleted),
			},
			expectedDeletes: sets.NewString("backup-C"),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var (
				client          = fake.NewSimpleClientset()
				sharedInformers = informers.NewSharedInformerFactory(client, 0)
			)

			c := NewBackupSyncController(
				client.ArkV1(),
				sharedInformers.Ark().V1().Backups(),
				sharedInformers.Ark().V1().BackupStorageLocations(),
				time.Duration(0),
				test.namespace,
				nil, // pluginRegistry
				arktest.NewLogger(),
				logrus.InfoLevel,
			).(*backupSyncController)

			expectedDeleteActions := make([]core.Action, 0)

			for _, backup := range test.k8sBackups {
				// add test backup to informer
				require.NoError(t, sharedInformers.Ark().V1().Backups().Informer().GetStore().Add(backup.Backup), "Error adding backup to informer")

				// add test backup to client
				_, err := client.Ark().Backups(test.namespace).Create(backup.Backup)
				require.NoError(t, err, "Error adding backup to clientset")

				// if we expect this backup to be deleted, set up the expected DeleteAction
				if test.expectedDeletes.Has(backup.Name) {
					actionDelete := core.NewDeleteAction(
						arkv1api.SchemeGroupVersion.WithResource("backups"),
						test.namespace,
						backup.Name,
					)
					expectedDeleteActions = append(expectedDeleteActions, actionDelete)
				}
			}

			c.deleteOrphanedBackups("default", test.cloudBackups, arktest.NewLogger())

			numBackups, err := numBackups(t, client, c.namespace)
			assert.NoError(t, err)

			expected := len(test.k8sBackups) - len(test.expectedDeletes)
			assert.Equal(t, expected, numBackups)

			arktest.CompareActions(t, expectedDeleteActions, getDeleteActions(client.Actions()))
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
	existingK8SBackups, err := c.ArkV1().Backups(ns).List(metav1.ListOptions{})
	if err != nil {
		return 0, err
	}

	return len(existingK8SBackups.Items), nil
}
