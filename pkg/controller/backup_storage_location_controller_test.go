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
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"

	"github.com/vmware-tanzu/velero/internal/storage"
	velerov1api "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	"github.com/vmware-tanzu/velero/pkg/builder"
	"github.com/vmware-tanzu/velero/pkg/persistence"
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
				backupLocation:    builder.ForBackupStorageLocation("ns-1", "location-1").ValidationFrequency(1 * time.Second).Result(),
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
		r := BackupStorageLocationReconciler{
			Ctx:    ctx,
			Client: fake.NewFakeClientWithScheme(scheme.Scheme, locations),
			DefaultBackupLocationInfo: storage.DefaultBackupLocationInfo{
				StorageLocation:           "location-1",
				ServerValidationFrequency: 0,
			},
			NewPluginManager: func(logrus.FieldLogger) clientmgmt.Manager { return pluginManager },
			NewBackupStore: func(loc *velerov1api.BackupStorageLocation, _ persistence.ObjectStoreGetter, _ logrus.FieldLogger) (persistence.BackupStore, error) {
				// this gets populated just below, prior to exercising the method under test
				return backupStores[loc.Name], nil
			},
			Log: velerotest.NewLogger(),
		}

		actualResult, err := r.Reconcile(ctx, ctrl.Request{
			NamespacedName: types.NamespacedName{Namespace: "ns-1"},
		})

		Expect(actualResult).To(BeEquivalentTo(ctrl.Result{Requeue: true}))
		Expect(err).To(BeNil())

		// Assertions
		for i, location := range locations.Items {
			key := client.ObjectKey{Name: location.Name, Namespace: location.Namespace}
			instance := &velerov1api.BackupStorageLocation{}
			err := r.Client.Get(ctx, key, instance)
			Expect(err).To(BeNil())
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
		r := BackupStorageLocationReconciler{
			Ctx:    ctx,
			Client: fake.NewFakeClientWithScheme(scheme.Scheme, locations),
			DefaultBackupLocationInfo: storage.DefaultBackupLocationInfo{
				StorageLocation:           "default",
				ServerValidationFrequency: 0,
			},
			NewPluginManager: func(logrus.FieldLogger) clientmgmt.Manager { return pluginManager },
			NewBackupStore: func(loc *velerov1api.BackupStorageLocation, _ persistence.ObjectStoreGetter, _ logrus.FieldLogger) (persistence.BackupStore, error) {
				// this gets populated just below, prior to exercising the method under test
				return backupStores[loc.Name], nil
			},
			Log: velerotest.NewLogger(),
		}

		actualResult, err := r.Reconcile(ctx, ctrl.Request{
			NamespacedName: types.NamespacedName{Namespace: "ns-1"},
		})

		Expect(actualResult).To(BeEquivalentTo(ctrl.Result{Requeue: true}))
		Expect(err).To(BeNil())

		// Assertions
		for i, location := range locations.Items {
			key := client.ObjectKey{Name: location.Name, Namespace: location.Namespace}
			instance := &velerov1api.BackupStorageLocation{}
			err := r.Client.Get(ctx, key, instance)
			Expect(err).To(BeNil())
			Expect(instance.Spec.Default).To(BeIdenticalTo(tests[i].expectedIsDefault))
		}
	})

	It("Should not patch a backup storage location object status phase if the location's validation frequency is specifically set to zero", func() {
		tests := []struct {
			backupLocation    *velerov1api.BackupStorageLocation
			isValidError      error
			expectedIsDefault bool
			expectedPhase     velerov1api.BackupStorageLocationPhase
			wantErr           bool
		}{
			{
				backupLocation:    builder.ForBackupStorageLocation("ns-1", "location-1").ValidationFrequency(0).LastValidationTime(time.Now()).Result(),
				isValidError:      nil,
				expectedIsDefault: false,
				expectedPhase:     "",
				wantErr:           false,
			},
			{
				backupLocation:    builder.ForBackupStorageLocation("ns-1", "location-2").ValidationFrequency(0).LastValidationTime(time.Now()).Result(),
				isValidError:      nil,
				expectedIsDefault: false,
				expectedPhase:     "",
				wantErr:           false,
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
			backupStores[location.Name].On("IsValid").Return(tests[i].isValidError)
		}

		// Setup reconciler
		Expect(velerov1api.AddToScheme(scheme.Scheme)).To(Succeed())
		r := BackupStorageLocationReconciler{
			Ctx:    ctx,
			Client: fake.NewFakeClientWithScheme(scheme.Scheme, locations),
			DefaultBackupLocationInfo: storage.DefaultBackupLocationInfo{
				StorageLocation:           "default",
				ServerValidationFrequency: 0,
			},
			NewPluginManager: func(logrus.FieldLogger) clientmgmt.Manager { return pluginManager },
			NewBackupStore: func(loc *velerov1api.BackupStorageLocation, _ persistence.ObjectStoreGetter, _ logrus.FieldLogger) (persistence.BackupStore, error) {
				// this gets populated just below, prior to exercising the method under test
				return backupStores[loc.Name], nil
			},
			Log: velerotest.NewLogger(),
		}

		actualResult, err := r.Reconcile(ctx, ctrl.Request{
			NamespacedName: types.NamespacedName{Namespace: "ns-1"},
		})

		Expect(actualResult).To(BeEquivalentTo(ctrl.Result{Requeue: true}))
		Expect(err).To(BeNil())

		// Assertions
		for i, location := range locations.Items {
			key := client.ObjectKey{Name: location.Name, Namespace: location.Namespace}
			instance := &velerov1api.BackupStorageLocation{}
			err := r.Client.Get(ctx, key, instance)
			Expect(err).To(BeNil())
			Expect(instance.Spec.Default).To(BeIdenticalTo(tests[i].expectedIsDefault))
			Expect(instance.Status.Phase).To(BeIdenticalTo(tests[i].expectedPhase))
		}
	})
})
