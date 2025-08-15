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
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/vmware-tanzu/velero/internal/storage"
	velerov1api "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	"github.com/vmware-tanzu/velero/pkg/builder"
	"github.com/vmware-tanzu/velero/pkg/metrics"
	persistencemocks "github.com/vmware-tanzu/velero/pkg/persistence/mocks"
	"github.com/vmware-tanzu/velero/pkg/plugin/clientmgmt"
	pluginmocks "github.com/vmware-tanzu/velero/pkg/plugin/mocks"
	velerotest "github.com/vmware-tanzu/velero/pkg/test"
)

var _ = Describe("Backup Storage Location Reconciler", func() {
	It("Should successfully patch a backup storage location object status phase according to whether its storage is valid or not", func() {
		tests := []struct {
			backupLocation    *velerov1api.BackupStorageLocation
			isValidError      error
			expectedIsDefault bool
			expectedPhase     velerov1api.BackupStorageLocationPhase
		}{
			{
				backupLocation:    builder.ForBackupStorageLocation("ns-1", "location-1").ValidationFrequency(1 * time.Second).Default(true).Result(),
				isValidError:      nil,
				expectedIsDefault: true,
				expectedPhase:     velerov1api.BackupStorageLocationPhaseAvailable,
			},
			{
				backupLocation:    builder.ForBackupStorageLocation("ns-1", "location-2").ValidationFrequency(1 * time.Second).Result(),
				isValidError:      errors.New("an error"),
				expectedIsDefault: false,
				expectedPhase:     velerov1api.BackupStorageLocationPhaseUnavailable,
			},
		}

		// Setup
		var (
			pluginManager = &pluginmocks.Manager{}
			backupStores  = make(map[string]*persistencemocks.BackupStore)
		)
		pluginManager.On("CleanupClients").Return(nil)

		locations := new(velerov1api.BackupStorageLocationList)
		for i, test := range tests {
			location := test.backupLocation
			locations.Items = append(locations.Items, *location)
			backupStores[location.Name] = &persistencemocks.BackupStore{}
			backupStore := backupStores[location.Name]
			backupStore.On("IsValid").Return(tests[i].isValidError)
		}

		// Setup reconciler
		Expect(velerov1api.AddToScheme(scheme.Scheme)).To(Succeed())
		r := backupStorageLocationReconciler{
			ctx:    ctx,
			client: fake.NewClientBuilder().WithScheme(scheme.Scheme).WithRuntimeObjects(locations).Build(),
			defaultBackupLocationInfo: storage.DefaultBackupLocationInfo{
				StorageLocation:           "location-1",
				ServerValidationFrequency: 0,
			},
			newPluginManager:  func(logrus.FieldLogger) clientmgmt.Manager { return pluginManager },
			backupStoreGetter: NewFakeObjectBackupStoreGetter(backupStores),
			metrics:           metrics.NewServerMetrics(),
			log:               velerotest.NewLogger(),
		}

		// Assertions
		for i, location := range locations.Items {
			actualResult, err := r.Reconcile(ctx, ctrl.Request{
				NamespacedName: types.NamespacedName{Namespace: location.Namespace, Name: location.Name},
			})
			Expect(actualResult).To(BeEquivalentTo(ctrl.Result{}))
			Expect(err).ToNot(HaveOccurred())

			key := client.ObjectKey{Name: location.Name, Namespace: location.Namespace}
			instance := &velerov1api.BackupStorageLocation{}
			err = r.client.Get(ctx, key, instance)
			Expect(err).ToNot(HaveOccurred())
			Expect(instance.Spec.Default).To(BeIdenticalTo(tests[i].expectedIsDefault))
			Expect(instance.Status.Phase).To(BeIdenticalTo(tests[i].expectedPhase))
		}
	})

	It("Should successfully patch a backup storage location object spec default if the BSL is the default one", func() {
		tests := []struct {
			backupLocation    *velerov1api.BackupStorageLocation
			isValidError      error
			expectedIsDefault bool
		}{
			{
				backupLocation:    builder.ForBackupStorageLocation("ns-1", "location-1").ValidationFrequency(1 * time.Second).Default(false).Result(),
				isValidError:      nil,
				expectedIsDefault: false,
			},
			{
				backupLocation:    builder.ForBackupStorageLocation("ns-1", "location-2").ValidationFrequency(1 * time.Second).Default(true).Result(),
				isValidError:      nil,
				expectedIsDefault: true,
			},
		}

		// Setup
		var (
			pluginManager = &pluginmocks.Manager{}
			backupStores  = make(map[string]*persistencemocks.BackupStore)
		)
		pluginManager.On("CleanupClients").Return(nil)

		locations := new(velerov1api.BackupStorageLocationList)
		for i, test := range tests {
			location := test.backupLocation
			locations.Items = append(locations.Items, *location)
			backupStores[location.Name] = &persistencemocks.BackupStore{}
			backupStore := backupStores[location.Name]
			backupStore.On("IsValid").Return(tests[i].isValidError)
		}

		// Setup reconciler
		Expect(velerov1api.AddToScheme(scheme.Scheme)).To(Succeed())
		r := backupStorageLocationReconciler{
			ctx:    ctx,
			client: fake.NewClientBuilder().WithScheme(scheme.Scheme).WithRuntimeObjects(locations).Build(),
			defaultBackupLocationInfo: storage.DefaultBackupLocationInfo{
				StorageLocation:           "default",
				ServerValidationFrequency: 0,
			},
			newPluginManager:  func(logrus.FieldLogger) clientmgmt.Manager { return pluginManager },
			backupStoreGetter: NewFakeObjectBackupStoreGetter(backupStores),
			metrics:           metrics.NewServerMetrics(),
			log:               velerotest.NewLogger(),
		}

		// Assertions
		for i, location := range locations.Items {
			actualResult, err := r.Reconcile(ctx, ctrl.Request{
				NamespacedName: types.NamespacedName{Namespace: location.Namespace, Name: location.Name},
			})
			Expect(actualResult).To(BeEquivalentTo(ctrl.Result{}))
			Expect(err).ToNot(HaveOccurred())

			key := client.ObjectKey{Name: location.Name, Namespace: location.Namespace}
			instance := &velerov1api.BackupStorageLocation{}
			err = r.client.Get(ctx, key, instance)
			Expect(err).ToNot(HaveOccurred())
			Expect(instance.Spec.Default).To(BeIdenticalTo(tests[i].expectedIsDefault))
		}
	})
})

func TestEnsureSingleDefaultBSL(t *testing.T) {
	tests := []struct {
		name               string
		locations          velerov1api.BackupStorageLocationList
		defaultBackupInfo  storage.DefaultBackupLocationInfo
		expectedDefaultSet bool
		expectedError      error
	}{
		{
			name: "MultipleDefaults",
			locations: func() velerov1api.BackupStorageLocationList {
				var locations velerov1api.BackupStorageLocationList
				locations.Items = append(locations.Items, *builder.ForBackupStorageLocation(velerov1api.DefaultNamespace, "location-1").LastValidationTime(time.Now()).Default(true).Result())
				locations.Items = append(locations.Items, *builder.ForBackupStorageLocation(velerov1api.DefaultNamespace, "location-2").LastValidationTime(time.Now().Add(-1 * time.Hour)).Default(true).Result())
				return locations
			}(),
			expectedDefaultSet: true,
			expectedError:      nil,
		},
		{
			name: "NoDefault with exist default bsl in defaultBackupInfo",
			locations: func() velerov1api.BackupStorageLocationList {
				var locations velerov1api.BackupStorageLocationList
				locations.Items = append(locations.Items, *builder.ForBackupStorageLocation(velerov1api.DefaultNamespace, "location-1").Default(false).Result())
				locations.Items = append(locations.Items, *builder.ForBackupStorageLocation(velerov1api.DefaultNamespace, "location-2").Default(false).Result())
				return locations
			}(),
			defaultBackupInfo: storage.DefaultBackupLocationInfo{
				StorageLocation: "location-2",
			},
			expectedDefaultSet: false,
			expectedError:      nil,
		},
		{
			name: "NoDefault with non-exist default bsl in defaultBackupInfo",
			locations: func() velerov1api.BackupStorageLocationList {
				var locations velerov1api.BackupStorageLocationList
				locations.Items = append(locations.Items, *builder.ForBackupStorageLocation(velerov1api.DefaultNamespace, "location-1").Default(false).Result())
				locations.Items = append(locations.Items, *builder.ForBackupStorageLocation(velerov1api.DefaultNamespace, "location-2").Default(false).Result())
				return locations
			}(),
			defaultBackupInfo: storage.DefaultBackupLocationInfo{
				StorageLocation: "location-3",
			},
			expectedDefaultSet: false,
			expectedError:      nil,
		},
		{
			name: "SingleDefault",
			locations: func() velerov1api.BackupStorageLocationList {
				var locations velerov1api.BackupStorageLocationList
				locations.Items = append(locations.Items, *builder.ForBackupStorageLocation(velerov1api.DefaultNamespace, "location-1").Default(true).Result())
				locations.Items = append(locations.Items, *builder.ForBackupStorageLocation(velerov1api.DefaultNamespace, "location-2").Default(false).Result())
				return locations
			}(),
			expectedDefaultSet: true,
			expectedError:      nil,
		},
	}

	for _, test := range tests {
		// Setup reconciler
		require.NoError(t, velerov1api.AddToScheme(scheme.Scheme))
		t.Run(test.name, func(t *testing.T) {
			r := &backupStorageLocationReconciler{
				ctx:                       t.Context(),
				client:                    fake.NewClientBuilder().WithScheme(scheme.Scheme).WithRuntimeObjects(&test.locations).Build(),
				defaultBackupLocationInfo: test.defaultBackupInfo,
				metrics:                   metrics.NewServerMetrics(),
				log:                       velerotest.NewLogger(),
			}
			defaultFound, err := r.ensureSingleDefaultBSL(test.locations)

			assert.Equal(t, test.expectedDefaultSet, defaultFound)
			assert.Equal(t, test.expectedError, err)
		})
	}
}

func TestBSLReconcile(t *testing.T) {
	tests := []struct {
		name          string
		locationList  velerov1api.BackupStorageLocationList
		defaultFound  bool
		expectedError error
	}{
		{
			name:          "NoBSL",
			locationList:  velerov1api.BackupStorageLocationList{},
			defaultFound:  false,
			expectedError: nil,
		},
		{
			name: "BSLNotFound",
			locationList: func() velerov1api.BackupStorageLocationList {
				var locations velerov1api.BackupStorageLocationList
				locations.Items = append(locations.Items, *builder.ForBackupStorageLocation(velerov1api.DefaultNamespace, "location-2").Result())
				return locations
			}(),
			defaultFound:  false,
			expectedError: nil,
		},
	}
	pluginManager := &pluginmocks.Manager{}
	pluginManager.On("CleanupClients").Return(nil)
	for _, test := range tests {
		// Setup reconciler
		require.NoError(t, velerov1api.AddToScheme(scheme.Scheme))
		t.Run(test.name, func(t *testing.T) {
			r := &backupStorageLocationReconciler{
				ctx:              t.Context(),
				client:           fake.NewClientBuilder().WithScheme(scheme.Scheme).WithRuntimeObjects(&test.locationList).Build(),
				newPluginManager: func(logrus.FieldLogger) clientmgmt.Manager { return pluginManager },
				metrics:          metrics.NewServerMetrics(),
				log:              velerotest.NewLogger(),
			}

			result, err := r.Reconcile(t.Context(), ctrl.Request{NamespacedName: types.NamespacedName{Namespace: velerov1api.DefaultNamespace, Name: "location-1"}})
			assert.Equal(t, test.expectedError, err)
			assert.Equal(t, ctrl.Result{}, result)
		})
	}
}
