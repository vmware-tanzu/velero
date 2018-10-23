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
	"bytes"
	"fmt"
	"io"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/clock"

	"github.com/heptio/ark/pkg/apis/ark/v1"
	pkgbackup "github.com/heptio/ark/pkg/backup"
	"github.com/heptio/ark/pkg/generated/clientset/versioned/fake"
	informers "github.com/heptio/ark/pkg/generated/informers/externalversions"
	"github.com/heptio/ark/pkg/metrics"
	"github.com/heptio/ark/pkg/persistence"
	persistencemocks "github.com/heptio/ark/pkg/persistence/mocks"
	"github.com/heptio/ark/pkg/plugin"
	pluginmocks "github.com/heptio/ark/pkg/plugin/mocks"
	"github.com/heptio/ark/pkg/util/logging"
	arktest "github.com/heptio/ark/pkg/util/test"
)

type fakeBackupper struct {
	mock.Mock
}

func (b *fakeBackupper) Backup(logger logrus.FieldLogger, backup *pkgbackup.Request, backupFile io.Writer, actions []pkgbackup.ItemAction, blockStoreGetter pkgbackup.BlockStoreGetter) error {
	args := b.Called(logger, backup, backupFile, actions, blockStoreGetter)
	return args.Error(0)
}

func TestProcessBackupNonProcessedItems(t *testing.T) {
	tests := []struct {
		name        string
		key         string
		backup      *v1.Backup
		expectedErr string
	}{
		{
			name:        "bad key returns error",
			key:         "bad/key/here",
			expectedErr: "error splitting queue key: unexpected key format: \"bad/key/here\"",
		},
		{
			name:        "backup not found in lister returns error",
			key:         "nonexistent/backup",
			expectedErr: "error getting backup: backup.ark.heptio.com \"backup\" not found",
		},
		{
			name:   "FailedValidation backup is not processed",
			key:    "heptio-ark/backup-1",
			backup: arktest.NewTestBackup().WithName("backup-1").WithPhase(v1.BackupPhaseFailedValidation).Backup,
		},
		{
			name:   "InProgress backup is not processed",
			key:    "heptio-ark/backup-1",
			backup: arktest.NewTestBackup().WithName("backup-1").WithPhase(v1.BackupPhaseInProgress).Backup,
		},
		{
			name:   "Completed backup is not processed",
			key:    "heptio-ark/backup-1",
			backup: arktest.NewTestBackup().WithName("backup-1").WithPhase(v1.BackupPhaseCompleted).Backup,
		},
		{
			name:   "Failed backup is not processed",
			key:    "heptio-ark/backup-1",
			backup: arktest.NewTestBackup().WithName("backup-1").WithPhase(v1.BackupPhaseFailed).Backup,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var (
				sharedInformers = informers.NewSharedInformerFactory(fake.NewSimpleClientset(), 0)
				logger          = logging.DefaultLogger(logrus.DebugLevel)
			)

			c := &backupController{
				genericController: newGenericController("backup-test", logger),
				lister:            sharedInformers.Ark().V1().Backups().Lister(),
			}

			if test.backup != nil {
				require.NoError(t, sharedInformers.Ark().V1().Backups().Informer().GetStore().Add(test.backup))
			}

			err := c.processBackup(test.key)
			if test.expectedErr != "" {
				require.Error(t, err)
				assert.Equal(t, test.expectedErr, err.Error())
			} else {
				assert.Nil(t, err)
			}

			// Any backup that would actually proceed to validation will cause a segfault because this
			// test hasn't set up the necessary controller dependencies for validation/etc. So the lack
			// of segfaults during test execution here imply that backups are not being processed, which
			// is what we expect.
		})
	}
}

func TestProcessBackupValidationFailures(t *testing.T) {
	defaultBackupLocation := arktest.NewTestBackupStorageLocation().WithName("loc-1").BackupStorageLocation

	tests := []struct {
		name           string
		backup         *v1.Backup
		backupLocation *v1.BackupStorageLocation
		expectedErrs   []string
	}{
		{
			name:           "invalid included/excluded resources fails validation",
			backup:         arktest.NewTestBackup().WithName("backup-1").WithIncludedResources("foo").WithExcludedResources("foo").Backup,
			backupLocation: defaultBackupLocation,
			expectedErrs:   []string{"Invalid included/excluded resource lists: excludes list cannot contain an item in the includes list: foo"},
		},
		{
			name:           "invalid included/excluded namespaces fails validation",
			backup:         arktest.NewTestBackup().WithName("backup-1").WithIncludedNamespaces("foo").WithExcludedNamespaces("foo").Backup,
			backupLocation: defaultBackupLocation,
			expectedErrs:   []string{"Invalid included/excluded namespace lists: excludes list cannot contain an item in the includes list: foo"},
		},
		{
			name:         "non-existent backup location fails validation",
			backup:       arktest.NewTestBackup().WithName("backup-1").WithStorageLocation("nonexistent").Backup,
			expectedErrs: []string{"Error getting backup storage location: backupstoragelocation.ark.heptio.com \"nonexistent\" not found"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var (
				clientset       = fake.NewSimpleClientset(test.backup)
				sharedInformers = informers.NewSharedInformerFactory(clientset, 0)
				logger          = logging.DefaultLogger(logrus.DebugLevel)
			)

			c := &backupController{
				genericController:     newGenericController("backup-test", logger),
				client:                clientset.ArkV1(),
				lister:                sharedInformers.Ark().V1().Backups().Lister(),
				backupLocationLister:  sharedInformers.Ark().V1().BackupStorageLocations().Lister(),
				defaultBackupLocation: defaultBackupLocation.Name,
			}

			require.NotNil(t, test.backup)
			require.NoError(t, sharedInformers.Ark().V1().Backups().Informer().GetStore().Add(test.backup))

			if test.backupLocation != nil {
				_, err := clientset.ArkV1().BackupStorageLocations(test.backupLocation.Namespace).Create(test.backupLocation)
				require.NoError(t, err)

				require.NoError(t, sharedInformers.Ark().V1().BackupStorageLocations().Informer().GetStore().Add(test.backupLocation))
			}

			require.NoError(t, c.processBackup(fmt.Sprintf("%s/%s", test.backup.Namespace, test.backup.Name)))

			res, err := clientset.ArkV1().Backups(test.backup.Namespace).Get(test.backup.Name, metav1.GetOptions{})
			require.NoError(t, err)

			assert.Equal(t, v1.BackupPhaseFailedValidation, res.Status.Phase)
			assert.Equal(t, test.expectedErrs, res.Status.ValidationErrors)

			// Any backup that would actually proceed to processing will cause a segfault because this
			// test hasn't set up the necessary controller dependencies for running backups. So the lack
			// of segfaults during test execution here imply that backups are not being processed, which
			// is what we expect.
		})
	}
}

func TestProcessBackupCompletions(t *testing.T) {
	defaultBackupLocation := arktest.NewTestBackupStorageLocation().WithName("loc-1").BackupStorageLocation

	now, err := time.Parse(time.RFC1123Z, time.RFC1123Z)
	require.NoError(t, err)
	now = now.Local()

	tests := []struct {
		name           string
		backup         *v1.Backup
		backupLocation *v1.BackupStorageLocation
		expectedResult *v1.Backup
	}{
		{
			name:           "backup with no backup location gets the default",
			backup:         arktest.NewTestBackup().WithName("backup-1").Backup,
			backupLocation: defaultBackupLocation,
			expectedResult: &v1.Backup{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: v1.DefaultNamespace,
					Name:      "backup-1",
					Labels: map[string]string{
						"ark.heptio.com/storage-location": "loc-1",
					},
				},
				Spec: v1.BackupSpec{
					StorageLocation: defaultBackupLocation.Name,
				},
				Status: v1.BackupStatus{
					Phase:               v1.BackupPhaseCompleted,
					Version:             1,
					StartTimestamp:      metav1.NewTime(now),
					CompletionTimestamp: metav1.NewTime(now),
				},
			},
		},
		{
			name:           "backup with a specific backup location keeps it",
			backup:         arktest.NewTestBackup().WithName("backup-1").WithStorageLocation("alt-loc").Backup,
			backupLocation: arktest.NewTestBackupStorageLocation().WithName("alt-loc").BackupStorageLocation,
			expectedResult: &v1.Backup{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: v1.DefaultNamespace,
					Name:      "backup-1",
					Labels: map[string]string{
						"ark.heptio.com/storage-location": "alt-loc",
					},
				},
				Spec: v1.BackupSpec{
					StorageLocation: "alt-loc",
				},
				Status: v1.BackupStatus{
					Phase:               v1.BackupPhaseCompleted,
					Version:             1,
					StartTimestamp:      metav1.NewTime(now),
					CompletionTimestamp: metav1.NewTime(now),
				},
			},
		},
		{
			name:           "backup with a TTL has expiration set",
			backup:         arktest.NewTestBackup().WithName("backup-1").WithTTL(10 * time.Minute).Backup,
			backupLocation: defaultBackupLocation,
			expectedResult: &v1.Backup{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: v1.DefaultNamespace,
					Name:      "backup-1",
					Labels: map[string]string{
						"ark.heptio.com/storage-location": "loc-1",
					},
				},
				Spec: v1.BackupSpec{
					TTL:             metav1.Duration{Duration: 10 * time.Minute},
					StorageLocation: defaultBackupLocation.Name,
				},
				Status: v1.BackupStatus{
					Phase:               v1.BackupPhaseCompleted,
					Version:             1,
					Expiration:          metav1.NewTime(now.Add(10 * time.Minute)),
					StartTimestamp:      metav1.NewTime(now),
					CompletionTimestamp: metav1.NewTime(now),
				},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var (
				clientset       = fake.NewSimpleClientset(test.backup)
				sharedInformers = informers.NewSharedInformerFactory(clientset, 0)
				logger          = logging.DefaultLogger(logrus.DebugLevel)
				pluginManager   = new(pluginmocks.Manager)
				backupStore     = new(persistencemocks.BackupStore)
				backupper       = new(fakeBackupper)
			)

			c := &backupController{
				genericController:     newGenericController("backup-test", logger),
				client:                clientset.ArkV1(),
				lister:                sharedInformers.Ark().V1().Backups().Lister(),
				backupLocationLister:  sharedInformers.Ark().V1().BackupStorageLocations().Lister(),
				defaultBackupLocation: defaultBackupLocation.Name,
				backupTracker:         NewBackupTracker(),
				metrics:               metrics.NewServerMetrics(),
				clock:                 clock.NewFakeClock(now),
				newPluginManager:      func(logrus.FieldLogger) plugin.Manager { return pluginManager },
				newBackupStore: func(*v1.BackupStorageLocation, persistence.ObjectStoreGetter, logrus.FieldLogger) (persistence.BackupStore, error) {
					return backupStore, nil
				},
				backupper: backupper,
			}

			pluginManager.On("GetBackupItemActions").Return(nil, nil)
			pluginManager.On("CleanupClients").Return(nil)

			backupper.On("Backup", mock.Anything, mock.Anything, mock.Anything, []pkgbackup.ItemAction(nil), pluginManager).Return(nil)

			// Ensure we have a CompletionTimestamp when uploading.
			// Failures will display the bytes in buf.
			completionTimestampIsPresent := func(buf *bytes.Buffer) bool {
				return strings.Contains(buf.String(), `"completionTimestamp": "2006-01-02T22:04:05Z"`)
			}
			backupStore.On("PutBackup", test.backup.Name, mock.MatchedBy(completionTimestampIsPresent), mock.Anything, mock.Anything, mock.Anything).Return(nil)

			// add the test's backup to the informer/lister store
			require.NotNil(t, test.backup)
			require.NoError(t, sharedInformers.Ark().V1().Backups().Informer().GetStore().Add(test.backup))

			// add the default backup storage location to the clientset and the informer/lister store
			_, err := clientset.ArkV1().BackupStorageLocations(defaultBackupLocation.Namespace).Create(defaultBackupLocation)
			require.NoError(t, err)

			require.NoError(t, sharedInformers.Ark().V1().BackupStorageLocations().Informer().GetStore().Add(defaultBackupLocation))

			// add the test's backup storage location to the clientset and the informer/lister store
			// if it's different than the default
			if test.backupLocation != nil && test.backupLocation != defaultBackupLocation {
				_, err := clientset.ArkV1().BackupStorageLocations(test.backupLocation.Namespace).Create(test.backupLocation)
				require.NoError(t, err)

				require.NoError(t, sharedInformers.Ark().V1().BackupStorageLocations().Informer().GetStore().Add(test.backupLocation))
			}

			require.NoError(t, c.processBackup(fmt.Sprintf("%s/%s", test.backup.Namespace, test.backup.Name)))

			res, err := clientset.ArkV1().Backups(test.backup.Namespace).Get(test.backup.Name, metav1.GetOptions{})
			require.NoError(t, err)

			assert.Equal(t, test.expectedResult, res)
		})
	}
}

func TestValidateAndGetSnapshotLocations(t *testing.T) {
	defaultLocationsAWS := map[string]*v1.VolumeSnapshotLocation{
		"aws": arktest.NewTestVolumeSnapshotLocation().WithName("aws-us-east-2").VolumeSnapshotLocation,
	}
	defaultLocationsFake := map[string]*v1.VolumeSnapshotLocation{
		"fake-provider": arktest.NewTestVolumeSnapshotLocation().WithName("some-name").VolumeSnapshotLocation,
	}

	multipleLocationNames := []string{"aws-us-west-1", "aws-us-east-1"}

	multipleLocation1 := arktest.LocationInfo{
		Name:     multipleLocationNames[0],
		Provider: "aws",
		Config:   map[string]string{"region": "us-west-1"},
	}
	multipleLocation2 := arktest.LocationInfo{
		Name:     multipleLocationNames[1],
		Provider: "aws",
		Config:   map[string]string{"region": "us-west-1"},
	}

	multipleLocationList := []arktest.LocationInfo{multipleLocation1, multipleLocation2}

	dupLocationNames := []string{"aws-us-west-1", "aws-us-west-1"}
	dupLocation1 := arktest.LocationInfo{
		Name:     dupLocationNames[0],
		Provider: "aws",
		Config:   map[string]string{"region": "us-west-1"},
	}
	dupLocation2 := arktest.LocationInfo{
		Name:     dupLocationNames[0],
		Provider: "aws",
		Config:   map[string]string{"region": "us-west-1"},
	}
	dupLocationList := []arktest.LocationInfo{dupLocation1, dupLocation2}

	tests := []struct {
		name                                string
		backup                              *arktest.TestBackup
		locations                           []*arktest.TestVolumeSnapshotLocation
		defaultLocations                    map[string]*v1.VolumeSnapshotLocation
		expectedVolumeSnapshotLocationNames []string // adding these in the expected order will allow to test with better msgs in case of a test failure
		expectedErrors                      string
		expectedSuccess                     bool
	}{
		{
			name:            "location name does not correspond to any existing location",
			backup:          arktest.NewTestBackup().WithName("backup1").WithPhase(v1.BackupPhaseNew).WithVolumeSnapshotLocations("random-name"),
			locations:       arktest.NewTestVolumeSnapshotLocation().WithName(dupLocationNames[0]).WithProviderConfig(dupLocationList),
			expectedErrors:  "error getting volume snapshot location named random-name: volumesnapshotlocation.ark.heptio.com \"random-name\" not found",
			expectedSuccess: false,
		},
		{
			name:                                "duplicate locationName per provider: should filter out dups",
			backup:                              arktest.NewTestBackup().WithName("backup1").WithPhase(v1.BackupPhaseNew).WithVolumeSnapshotLocations(dupLocationNames...),
			locations:                           arktest.NewTestVolumeSnapshotLocation().WithName(dupLocationNames[0]).WithProviderConfig(dupLocationList),
			expectedVolumeSnapshotLocationNames: []string{dupLocationNames[0]},
			expectedSuccess:                     true,
		},
		{
			name:            "multiple location names per provider",
			backup:          arktest.NewTestBackup().WithName("backup1").WithPhase(v1.BackupPhaseNew).WithVolumeSnapshotLocations(multipleLocationNames...),
			locations:       arktest.NewTestVolumeSnapshotLocation().WithName(multipleLocationNames[0]).WithProviderConfig(multipleLocationList),
			expectedErrors:  "more than one VolumeSnapshotLocation name specified for provider aws: aws-us-east-1; unexpected name was aws-us-west-1",
			expectedSuccess: false,
		},
		{
			name:                                "no location name for the provider exists: the provider's default should be added",
			backup:                              arktest.NewTestBackup().WithName("backup1").WithPhase(v1.BackupPhaseNew),
			defaultLocations:                    defaultLocationsAWS,
			expectedVolumeSnapshotLocationNames: []string{defaultLocationsAWS["aws"].Name},
			expectedSuccess:                     true,
		},
		{
			name:            "no existing location name and no default location name given",
			backup:          arktest.NewTestBackup().WithName("backup1").WithPhase(v1.BackupPhaseNew),
			expectedSuccess: true,
		},
		{
			name:                                "multiple location names for a provider, default location name for another provider",
			backup:                              arktest.NewTestBackup().WithName("backup1").WithPhase(v1.BackupPhaseNew).WithVolumeSnapshotLocations(dupLocationNames...),
			locations:                           arktest.NewTestVolumeSnapshotLocation().WithName(dupLocationNames[0]).WithProviderConfig(dupLocationList),
			defaultLocations:                    defaultLocationsFake,
			expectedVolumeSnapshotLocationNames: []string{dupLocationNames[0], defaultLocationsFake["fake-provider"].Name},
			expectedSuccess:                     true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var (
				client          = fake.NewSimpleClientset()
				sharedInformers = informers.NewSharedInformerFactory(client, 0)
			)

			c := &backupController{
				snapshotLocationLister:   sharedInformers.Ark().V1().VolumeSnapshotLocations().Lister(),
				defaultSnapshotLocations: test.defaultLocations,
			}

			// set up a Backup object to represent what we expect to be passed to backupper.Backup()
			backup := test.backup.DeepCopy()
			backup.Spec.VolumeSnapshotLocations = test.backup.Spec.VolumeSnapshotLocations
			for _, location := range test.locations {
				require.NoError(t, sharedInformers.Ark().V1().VolumeSnapshotLocations().Informer().GetStore().Add(location.VolumeSnapshotLocation))
			}

			providerLocations, errs := c.validateAndGetSnapshotLocations(backup)
			if test.expectedSuccess {
				for _, err := range errs {
					require.NoError(t, errors.New(err), "validateAndGetSnapshotLocations unexpected error: %v", err)
				}

				var locations []string
				for _, loc := range providerLocations {
					locations = append(locations, loc.Name)
				}

				sort.Strings(test.expectedVolumeSnapshotLocationNames)
				sort.Strings(locations)
				require.Equal(t, test.expectedVolumeSnapshotLocationNames, locations)
			} else {
				if len(errs) == 0 {
					require.Error(t, nil, "validateAndGetSnapshotLocations expected error")
				}
				require.Contains(t, errs, test.expectedErrors)
			}
		})
	}
}
