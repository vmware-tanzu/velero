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
	"testing"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	velerov1api "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	"github.com/vmware-tanzu/velero/pkg/builder"
	"github.com/vmware-tanzu/velero/pkg/generated/clientset/versioned/fake"
	informers "github.com/vmware-tanzu/velero/pkg/generated/informers/externalversions"
	"github.com/vmware-tanzu/velero/pkg/persistence"
	persistencemocks "github.com/vmware-tanzu/velero/pkg/persistence/mocks"
	"github.com/vmware-tanzu/velero/pkg/plugin/clientmgmt"
	pluginmocks "github.com/vmware-tanzu/velero/pkg/plugin/mocks"
	velerotest "github.com/vmware-tanzu/velero/pkg/test"
)

func TestBackupStorageLocationControllerRun(t *testing.T) {
	type backupStore struct {
		persistanceStoreIsValidError error
		backupStorageLocation        *velerov1api.BackupStorageLocation
	}

	tests := []struct {
		name, namespace, defaultBackupLocation         string
		backupStores                                   []backupStore
		persistenceStore                               map[string]*persistencemocks.BackupStore
		expectedAvailableBSLs, expectedUnavailableBSLs []string
	}{
		{
			name:                  "all backup storage locations available",
			namespace:             "ns-1",
			defaultBackupLocation: "default",
			backupStores: []backupStore{
				{
					backupStorageLocation:        builder.ForBackupStorageLocation("ns-1", "default").Result(),
					persistanceStoreIsValidError: nil,
				},
				{
					backupStorageLocation:        builder.ForBackupStorageLocation("ns-1", "random").Result(),
					persistanceStoreIsValidError: nil,
				},
			},
			expectedAvailableBSLs:   []string{"default", "random"},
			expectedUnavailableBSLs: []string(nil),
		},
		{
			name:                  "all backup storage locations available, except the default",
			namespace:             "ns-1",
			defaultBackupLocation: "default",
			backupStores: []backupStore{
				{
					backupStorageLocation:        builder.ForBackupStorageLocation("ns-1", "default").Result(),
					persistanceStoreIsValidError: errors.New("this storage is currently invalid"),
				},
				{
					backupStorageLocation:        builder.ForBackupStorageLocation("ns-1", "random").Result(),
					persistanceStoreIsValidError: nil,
				},
			},
			expectedAvailableBSLs:   []string{"random"},
			expectedUnavailableBSLs: []string{"default"},
		},
		{
			name:                  "all backup storage locations unavailable",
			namespace:             "ns-1",
			defaultBackupLocation: "default",
			backupStores: []backupStore{
				{
					backupStorageLocation:        builder.ForBackupStorageLocation("ns-1", "default").Result(),
					persistanceStoreIsValidError: errors.New("this storage is currently invalid"),
				},
				{
					backupStorageLocation:        builder.ForBackupStorageLocation("ns-1", "random").Result(),
					persistanceStoreIsValidError: errors.New("this storage is currently invalid"),
				},
			},
			expectedAvailableBSLs:   []string(nil),
			expectedUnavailableBSLs: []string{"default", "random"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var (
				client          = fake.NewSimpleClientset()
				sharedInformers = informers.NewSharedInformerFactory(client, 0)
				pluginManager   = &pluginmocks.Manager{}
			)
			test.persistenceStore = make(map[string]*persistencemocks.BackupStore)

			c := NewBackupStorageLocationController(
				test.namespace,
				test.defaultBackupLocation,
				client.VeleroV1(),
				sharedInformers.Velero().V1().BackupStorageLocations().Lister(),
				func(logrus.FieldLogger) clientmgmt.Manager { return pluginManager },
				velerotest.NewLogger(),
			).(backupStorageLocationController)

			c.newBackupStore = func(loc *velerov1api.BackupStorageLocation, _ persistence.ObjectStoreGetter, _ logrus.FieldLogger) (persistence.BackupStore, error) {
				// this gets populated just below, prior to exercising the method under test
				// return backupStores[loc.Name], nil
				return test.persistenceStore[loc.Name], nil
			}

			pluginManager.On("CleanupClients").Return(nil)

			for _, storage := range test.backupStores {
				location := storage.backupStorageLocation

				_, err := client.VeleroV1().BackupStorageLocations(location.Namespace).Create(location)
				require.NoError(t, err)

				require.NoError(t, sharedInformers.Velero().V1().BackupStorageLocations().Informer().GetStore().Add(location))

				test.persistenceStore[location.Name] = &persistencemocks.BackupStore{}
			}

			for _, storage := range test.backupStores {
				location := storage.backupStorageLocation

				// populate the mocked persistance store for this location
				store, ok := test.persistenceStore[location.Name]
				require.True(t, ok, "no mock persistance store for location %s", location.Name)

				store.On("IsValid").Return(storage.persistanceStoreIsValidError)
			}

			client.ClearActions()

			c.run()

			backupStorageLocations, err := client.VeleroV1().BackupStorageLocations(test.namespace).List(metav1.ListOptions{})
			require.NoError(t, err)

			var allAvailable, allUnavailable []string
			for _, bsl := range backupStorageLocations.Items {
				if bsl.Status.Phase == velerov1api.BackupStorageLocationPhaseAvailable {
					allAvailable = append(allAvailable, bsl.Name)
				} else {
					allUnavailable = append(allUnavailable, bsl.Name)
				}
			}

			assert.Equal(t, test.expectedAvailableBSLs, allAvailable)
			assert.Equal(t, test.expectedUnavailableBSLs, allUnavailable)
		})
	}
}
