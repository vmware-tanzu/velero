/*Copyright 2020 the Velero contributors.

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

package velero

import (
	"context"
	"errors"
	"testing"
	"time"

	. "github.com/onsi/gomega"
	"github.com/sirupsen/logrus"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	velerov1api "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	"github.com/vmware-tanzu/velero/pkg/builder"
	"github.com/vmware-tanzu/velero/pkg/generated/clientset/versioned/scheme"
	"github.com/vmware-tanzu/velero/pkg/persistence"
	persistencemocks "github.com/vmware-tanzu/velero/pkg/persistence/mocks"
	"github.com/vmware-tanzu/velero/pkg/plugin/clientmgmt"
	pluginmocks "github.com/vmware-tanzu/velero/pkg/plugin/mocks"
	velerotest "github.com/vmware-tanzu/velero/pkg/test"
)

func TestIsReadyToValidate(t *testing.T) {
	tests := []struct {
		name                             string
		serverDefaultValidationFrequency time.Duration
		backupLocation                   *velerov1api.BackupStorageLocation
		ready                            bool
	}{
		{
			name:                             "don't validate, since frequency is set to zero",
			serverDefaultValidationFrequency: 0,
			backupLocation:                   builder.ForBackupStorageLocation("ns-1", "location-2").ValidationFrequency(0).Result(),
			ready:                            false,
		},
		{
			name:                             "validate as per location setting, as that takes precedence, and always if it has never been validated before regardless of the frequency setting",
			serverDefaultValidationFrequency: 0,
			backupLocation:                   builder.ForBackupStorageLocation("ns-1", "location-1").ValidationFrequency(1 * time.Hour).Result(),
			ready:                            true,
		},
		{
			name:                             "don't validate as per location setting, as it is set to zero and that takes precedence",
			serverDefaultValidationFrequency: 1,
			backupLocation:                   builder.ForBackupStorageLocation("ns-1", "location-1").ValidationFrequency(0).Result(),
			ready:                            false,
		},
		{
			name:                             "validate as per default setting when location setting is not set",
			serverDefaultValidationFrequency: 1,
			backupLocation:                   builder.ForBackupStorageLocation("ns-1", "location-2").Result(),
			ready:                            true,
		},
		{
			name:                             "don't validate when default setting is set to zero and the location setting is not set",
			serverDefaultValidationFrequency: 0,
			backupLocation:                   builder.ForBackupStorageLocation("ns-1", "location-2").Result(),
			ready:                            false,
		},
		{
			name:                             "don't validate when now is before the NEXT validation time (validation frequency + last validation time)",
			serverDefaultValidationFrequency: 0,
			backupLocation:                   builder.ForBackupStorageLocation("ns-1", "location-1").ValidationFrequency(1 * time.Second).LastValidationTime(time.Now()).Result(),
			ready:                            false,
		},
		{
			name:                             "validate when now is equal to the NEXT validation time (validation frequency + last validation time)",
			serverDefaultValidationFrequency: 0,
			backupLocation:                   builder.ForBackupStorageLocation("ns-1", "location-2").ValidationFrequency(1 * time.Second).LastValidationTime(time.Now().Add(-1 * time.Second)).Result(),
			ready:                            true,
		},
		{
			name:                             "validate when now is after the NEXT validation time (validation frequency + last validation time)",
			serverDefaultValidationFrequency: 0,
			backupLocation:                   builder.ForBackupStorageLocation("ns-1", "location-2").ValidationFrequency(1 * time.Second).LastValidationTime(time.Now().Add(-2 * time.Second)).Result(),
			ready:                            true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			locationStore := LocationStore{
				location: tt.backupLocation,
				defaultLocationInfo: DefaultBackupLocationInfo{
					DefaultStoreValidationFrequency: tt.serverDefaultValidationFrequency,
				},
				log: velerotest.NewLogger(),
			}

			g.Expect(locationStore.IsReadyToValidate()).To(BeIdenticalTo(tt.ready))
		})
	}
}

func TestIsValid(t *testing.T) {
	tests := []struct {
		name           string
		backupLocation *velerov1api.BackupStorageLocation
		isValidError   error
	}{
		{
			name:           "do not expect an error when store is valid",
			backupLocation: builder.ForBackupStorageLocation("ns-1", "location-1").Result(),
			isValidError:   nil,
		},
		{
			name:           "expect an error when store is not valid",
			backupLocation: builder.ForBackupStorageLocation("ns-1", "location-1").Result(),
			isValidError:   errors.New("an error"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			var (
				pluginManager = &pluginmocks.Manager{}
				backupStores  = make(map[string]*persistencemocks.BackupStore)
			)
			pluginManager.On("CleanupClients").Return(nil)

			location := tt.backupLocation
			backupStores[location.Name] = &persistencemocks.BackupStore{}
			backupStore := backupStores[location.Name]
			backupStore.On("IsValid").Return(tt.isValidError)

			storeManager := BackupStoreManager{
				NewPluginManager: func(logrus.FieldLogger) clientmgmt.Manager { return pluginManager },
				NewBackupStore: func(loc *velerov1api.BackupStorageLocation, _ persistence.ObjectStoreGetter, _ logrus.FieldLogger) (persistence.BackupStore, error) {
					return backupStores[loc.Name], nil
				},
			}

			locationStore, err := NewLocationStore(storeManager, DefaultBackupLocationInfo{}, tt.backupLocation, velerotest.NewLogger())
			g.Expect(err).To(BeNil())

			actual := locationStore.IsValid()
			if tt.isValidError != nil {
				g.Expect(actual).NotTo(BeNil())
			} else {
				g.Expect(actual).To(BeNil())
			}
		})
	}
}

func TestListBackupStorageLocations(t *testing.T) {
	tests := []struct {
		name            string
		backupLocations *velerov1api.BackupStorageLocationList
		expectError     bool
	}{
		{
			name: "1 existing location does not return an error",
			backupLocations: &velerov1api.BackupStorageLocationList{
				Items: []velerov1api.BackupStorageLocation{
					*builder.ForBackupStorageLocation("ns-1", "location-1").Result(),
				},
			},
			expectError: false,
		},
		{
			name: "multiple existing location does not return an error",
			backupLocations: &velerov1api.BackupStorageLocationList{
				Items: []velerov1api.BackupStorageLocation{
					*builder.ForBackupStorageLocation("ns-1", "location-1").Result(),
					*builder.ForBackupStorageLocation("ns-1", "location-2").Result(),
					*builder.ForBackupStorageLocation("ns-1", "location-3").Result(),
				},
			},
			expectError: false,
		},
		{
			name:            "no existing locations returns an error",
			backupLocations: &velerov1api.BackupStorageLocationList{},
			expectError:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			client := fake.NewFakeClientWithScheme(scheme.Scheme, tt.backupLocations)
			if tt.expectError {
				_, err := ListBackupStorageLocations(context.Background(), client, "ns-1")
				g.Expect(err).NotTo(BeNil())
			} else {
				_, err := ListBackupStorageLocations(context.Background(), client, "ns-1")
				g.Expect(err).To(BeNil())
			}
		})
	}
}
