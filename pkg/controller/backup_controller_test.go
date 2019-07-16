/*
Copyright 2017, 2019 the Velero contributors.

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

	velerov1api "github.com/heptio/velero/pkg/apis/velero/v1"
	pkgbackup "github.com/heptio/velero/pkg/backup"
	"github.com/heptio/velero/pkg/generated/clientset/versioned/fake"
	informers "github.com/heptio/velero/pkg/generated/informers/externalversions"
	"github.com/heptio/velero/pkg/metrics"
	"github.com/heptio/velero/pkg/persistence"
	persistencemocks "github.com/heptio/velero/pkg/persistence/mocks"
	"github.com/heptio/velero/pkg/plugin/clientmgmt"
	pluginmocks "github.com/heptio/velero/pkg/plugin/mocks"
	"github.com/heptio/velero/pkg/plugin/velero"
	"github.com/heptio/velero/pkg/util/logging"
	velerotest "github.com/heptio/velero/pkg/util/test"
)

type fakeBackupper struct {
	mock.Mock
}

func (b *fakeBackupper) Backup(logger logrus.FieldLogger, backup *pkgbackup.Request, backupFile io.Writer, actions []velero.BackupItemAction, volumeSnapshotterGetter pkgbackup.VolumeSnapshotterGetter) error {
	args := b.Called(logger, backup, backupFile, actions, volumeSnapshotterGetter)
	return args.Error(0)
}

func defaultBackup() *pkgbackup.Builder {
	return pkgbackup.NewNamedBackupBuilder(velerov1api.DefaultNamespace, "backup-1")
}

func TestProcessBackupNonProcessedItems(t *testing.T) {
	tests := []struct {
		name   string
		key    string
		backup *velerov1api.Backup
	}{
		{
			name: "bad key does not return error",
			key:  "bad/key/here",
		},
		{
			name: "backup not found in lister does not return error",
			key:  "nonexistent/backup",
		},
		{
			name:   "FailedValidation backup is not processed",
			key:    "velero/backup-1",
			backup: defaultBackup().Phase(velerov1api.BackupPhaseFailedValidation).Backup(),
		},
		{
			name:   "InProgress backup is not processed",
			key:    "velero/backup-1",
			backup: defaultBackup().Phase(velerov1api.BackupPhaseInProgress).Backup(),
		},
		{
			name:   "Completed backup is not processed",
			key:    "velero/backup-1",
			backup: defaultBackup().Phase(velerov1api.BackupPhaseCompleted).Backup(),
		},
		{
			name:   "Failed backup is not processed",
			key:    "velero/backup-1",
			backup: defaultBackup().Phase(velerov1api.BackupPhaseFailed).Backup(),
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
				lister:            sharedInformers.Velero().V1().Backups().Lister(),
			}

			if test.backup != nil {
				require.NoError(t, sharedInformers.Velero().V1().Backups().Informer().GetStore().Add(test.backup))
			}

			err := c.processBackup(test.key)
			assert.Nil(t, err)

			// Any backup that would actually proceed to validation will cause a segfault because this
			// test hasn't set up the necessary controller dependencies for validation/etc. So the lack
			// of segfaults during test execution here imply that backups are not being processed, which
			// is what we expect.
		})
	}
}

func TestProcessBackupValidationFailures(t *testing.T) {
	defaultBackupLocation := velerotest.NewTestBackupStorageLocation().WithName("loc-1").BackupStorageLocation

	tests := []struct {
		name           string
		backup         *velerov1api.Backup
		backupLocation *velerov1api.BackupStorageLocation
		expectedErrs   []string
	}{
		{
			name:           "invalid included/excluded resources fails validation",
			backup:         defaultBackup().IncludedResources("foo").ExcludedResources("foo").Backup(),
			backupLocation: defaultBackupLocation,
			expectedErrs:   []string{"Invalid included/excluded resource lists: excludes list cannot contain an item in the includes list: foo"},
		},
		{
			name:           "invalid included/excluded namespaces fails validation",
			backup:         defaultBackup().IncludedNamespaces("foo").ExcludedNamespaces("foo").Backup(),
			backupLocation: defaultBackupLocation,
			expectedErrs:   []string{"Invalid included/excluded namespace lists: excludes list cannot contain an item in the includes list: foo"},
		},
		{
			name:         "non-existent backup location fails validation",
			backup:       defaultBackup().StorageLocation("nonexistent").Backup(),
			expectedErrs: []string{"a BackupStorageLocation CRD with the name specified in the backup spec needs to be created before this backup can be executed. Error: backupstoragelocation.velero.io \"nonexistent\" not found"},
		},
		{
			name:           "backup for read-only backup location fails validation",
			backup:         defaultBackup().StorageLocation("read-only").Backup(),
			backupLocation: velerotest.NewTestBackupStorageLocation().WithName("read-only").WithAccessMode(velerov1api.BackupStorageLocationAccessModeReadOnly).BackupStorageLocation,
			expectedErrs:   []string{"backup can't be created because backup storage location read-only is currently in read-only mode"},
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
				genericController:      newGenericController("backup-test", logger),
				client:                 clientset.VeleroV1(),
				lister:                 sharedInformers.Velero().V1().Backups().Lister(),
				backupLocationLister:   sharedInformers.Velero().V1().BackupStorageLocations().Lister(),
				snapshotLocationLister: sharedInformers.Velero().V1().VolumeSnapshotLocations().Lister(),
				defaultBackupLocation:  defaultBackupLocation.Name,
				clock:                  &clock.RealClock{},
			}

			require.NotNil(t, test.backup)
			require.NoError(t, sharedInformers.Velero().V1().Backups().Informer().GetStore().Add(test.backup))

			if test.backupLocation != nil {
				_, err := clientset.VeleroV1().BackupStorageLocations(test.backupLocation.Namespace).Create(test.backupLocation)
				require.NoError(t, err)

				require.NoError(t, sharedInformers.Velero().V1().BackupStorageLocations().Informer().GetStore().Add(test.backupLocation))
			}

			require.NoError(t, c.processBackup(fmt.Sprintf("%s/%s", test.backup.Namespace, test.backup.Name)))

			res, err := clientset.VeleroV1().Backups(test.backup.Namespace).Get(test.backup.Name, metav1.GetOptions{})
			require.NoError(t, err)

			assert.Equal(t, velerov1api.BackupPhaseFailedValidation, res.Status.Phase)
			assert.Equal(t, test.expectedErrs, res.Status.ValidationErrors)

			// Any backup that would actually proceed to processing will cause a segfault because this
			// test hasn't set up the necessary controller dependencies for running backups. So the lack
			// of segfaults during test execution here imply that backups are not being processed, which
			// is what we expect.
		})
	}
}

func TestBackupLocationLabel(t *testing.T) {
	tests := []struct {
		name                   string
		backup                 *velerov1api.Backup
		backupLocation         *velerov1api.BackupStorageLocation
		expectedBackupLocation string
	}{
		{
			name:                   "valid backup location name should be used as a label",
			backup:                 defaultBackup().Backup(),
			backupLocation:         velerotest.NewTestBackupStorageLocation().WithName("loc-1").BackupStorageLocation,
			expectedBackupLocation: "loc-1",
		},
		{
			name:   "invalid storage location name should be handled while creating label",
			backup: defaultBackup().Backup(),
			backupLocation: velerotest.NewTestBackupStorageLocation().
				WithName("defaultdefaultdefaultdefaultdefaultdefaultdefaultdefaultdefaultdefault").BackupStorageLocation,
			expectedBackupLocation: "defaultdefaultdefaultdefaultdefaultdefaultdefaultdefaultd58343f",
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
				genericController:      newGenericController("backup-test", logger),
				client:                 clientset.VeleroV1(),
				lister:                 sharedInformers.Velero().V1().Backups().Lister(),
				backupLocationLister:   sharedInformers.Velero().V1().BackupStorageLocations().Lister(),
				snapshotLocationLister: sharedInformers.Velero().V1().VolumeSnapshotLocations().Lister(),
				defaultBackupLocation:  test.backupLocation.Name,
				clock:                  &clock.RealClock{},
			}

			res := c.prepareBackupRequest(test.backup)
			assert.NotNil(t, res)
			assert.Equal(t, test.expectedBackupLocation, res.Labels[velerov1api.StorageLocationLabel])
		})
	}
}

func TestDefaultBackupTTL(t *testing.T) {
	var (
		defaultBackupTTL = metav1.Duration{Duration: 24 * 30 * time.Hour}
	)

	now, err := time.Parse(time.RFC1123Z, time.RFC1123Z)
	require.NoError(t, err)
	now = now.Local()

	tests := []struct {
		name               string
		backup             *velerov1api.Backup
		backupLocation     *velerov1api.BackupStorageLocation
		expectedTTL        metav1.Duration
		expectedExpiration metav1.Time
	}{
		{
			name:               "backup with no TTL specified",
			backup:             defaultBackup().Backup(),
			expectedTTL:        defaultBackupTTL,
			expectedExpiration: metav1.NewTime(now.Add(defaultBackupTTL.Duration)),
		},
		{
			name:               "backup with TTL specified",
			backup:             defaultBackup().TTL(time.Hour).Backup(),
			expectedTTL:        metav1.Duration{Duration: 1 * time.Hour},
			expectedExpiration: metav1.NewTime(now.Add(1 * time.Hour)),
		},
	}

	for _, test := range tests {
		var (
			clientset       = fake.NewSimpleClientset(test.backup)
			logger          = logging.DefaultLogger(logrus.DebugLevel)
			sharedInformers = informers.NewSharedInformerFactory(clientset, 0)
		)

		t.Run(test.name, func(t *testing.T) {
			c := &backupController{
				genericController:      newGenericController("backup-test", logger),
				backupLocationLister:   sharedInformers.Velero().V1().BackupStorageLocations().Lister(),
				snapshotLocationLister: sharedInformers.Velero().V1().VolumeSnapshotLocations().Lister(),
				defaultBackupTTL:       defaultBackupTTL.Duration,
				clock:                  clock.NewFakeClock(now),
			}

			res := c.prepareBackupRequest(test.backup)
			assert.NotNil(t, res)
			assert.Equal(t, test.expectedTTL, res.Spec.TTL)
			assert.Equal(t, test.expectedExpiration, res.Status.Expiration)
		})
	}
}

func TestProcessBackupCompletions(t *testing.T) {
	defaultBackupLocation := velerotest.NewTestBackupStorageLocation().WithName("loc-1").WithObjectStorage("store-1").BackupStorageLocation

	now, err := time.Parse(time.RFC1123Z, time.RFC1123Z)
	require.NoError(t, err)
	now = now.Local()

	tests := []struct {
		name                string
		backup              *velerov1api.Backup
		backupLocation      *velerov1api.BackupStorageLocation
		expectedResult      *velerov1api.Backup
		backupExists        bool
		existenceCheckError error
	}{
		// Completed
		{
			name:           "backup with no backup location gets the default",
			backup:         defaultBackup().Backup(),
			backupLocation: defaultBackupLocation,
			expectedResult: &velerov1api.Backup{
				TypeMeta: metav1.TypeMeta{
					Kind:       "Backup",
					APIVersion: "velero.io/v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Namespace: velerov1api.DefaultNamespace,
					Name:      "backup-1",
					Labels: map[string]string{
						"velero.io/storage-location": "loc-1",
					},
				},
				Spec: velerov1api.BackupSpec{
					StorageLocation: defaultBackupLocation.Name,
				},
				Status: velerov1api.BackupStatus{
					Phase:               velerov1api.BackupPhaseCompleted,
					Version:             1,
					StartTimestamp:      metav1.NewTime(now),
					CompletionTimestamp: metav1.NewTime(now),
					Expiration:          metav1.NewTime(now),
				},
			},
		},
		{
			name:           "backup with a specific backup location keeps it",
			backup:         defaultBackup().StorageLocation("alt-loc").Backup(),
			backupLocation: velerotest.NewTestBackupStorageLocation().WithName("alt-loc").WithObjectStorage("store-1").BackupStorageLocation,
			expectedResult: &velerov1api.Backup{
				TypeMeta: metav1.TypeMeta{
					Kind:       "Backup",
					APIVersion: "velero.io/v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Namespace: velerov1api.DefaultNamespace,
					Name:      "backup-1",
					Labels: map[string]string{
						"velero.io/storage-location": "alt-loc",
					},
				},
				Spec: velerov1api.BackupSpec{
					StorageLocation: "alt-loc",
				},
				Status: velerov1api.BackupStatus{
					Phase:               velerov1api.BackupPhaseCompleted,
					Version:             1,
					StartTimestamp:      metav1.NewTime(now),
					CompletionTimestamp: metav1.NewTime(now),
					Expiration:          metav1.NewTime(now),
				},
			},
		},
		{
			name:   "backup for a location with ReadWrite access mode gets processed",
			backup: defaultBackup().StorageLocation("read-write").Backup(),
			backupLocation: velerotest.NewTestBackupStorageLocation().
				WithName("read-write").
				WithObjectStorage("store-1").
				WithAccessMode(velerov1api.BackupStorageLocationAccessModeReadWrite).
				BackupStorageLocation,
			expectedResult: &velerov1api.Backup{
				TypeMeta: metav1.TypeMeta{
					Kind:       "Backup",
					APIVersion: "velero.io/v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Namespace: velerov1api.DefaultNamespace,
					Name:      "backup-1",
					Labels: map[string]string{
						"velero.io/storage-location": "read-write",
					},
				},
				Spec: velerov1api.BackupSpec{
					StorageLocation: "read-write",
				},
				Status: velerov1api.BackupStatus{
					Phase:               velerov1api.BackupPhaseCompleted,
					Version:             1,
					StartTimestamp:      metav1.NewTime(now),
					CompletionTimestamp: metav1.NewTime(now),
					Expiration:          metav1.NewTime(now),
				},
			},
		},
		{
			name:           "backup with a TTL has expiration set",
			backup:         defaultBackup().TTL(10 * time.Minute).Backup(),
			backupLocation: defaultBackupLocation,
			expectedResult: &velerov1api.Backup{
				TypeMeta: metav1.TypeMeta{
					Kind:       "Backup",
					APIVersion: "velero.io/v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Namespace: velerov1api.DefaultNamespace,
					Name:      "backup-1",
					Labels: map[string]string{
						"velero.io/storage-location": "loc-1",
					},
				},
				Spec: velerov1api.BackupSpec{
					TTL:             metav1.Duration{Duration: 10 * time.Minute},
					StorageLocation: defaultBackupLocation.Name,
				},
				Status: velerov1api.BackupStatus{
					Phase:               velerov1api.BackupPhaseCompleted,
					Version:             1,
					Expiration:          metav1.NewTime(now.Add(10 * time.Minute)),
					StartTimestamp:      metav1.NewTime(now),
					CompletionTimestamp: metav1.NewTime(now),
				},
			},
		},
		{
			name:           "backup without an existing backup will succeed",
			backupExists:   false,
			backup:         defaultBackup().Backup(),
			backupLocation: defaultBackupLocation,
			expectedResult: &velerov1api.Backup{
				TypeMeta: metav1.TypeMeta{
					Kind:       "Backup",
					APIVersion: "velero.io/v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Namespace: velerov1api.DefaultNamespace,
					Name:      "backup-1",
					Labels: map[string]string{
						"velero.io/storage-location": "loc-1",
					},
				},
				Spec: velerov1api.BackupSpec{
					StorageLocation: defaultBackupLocation.Name,
				},
				Status: velerov1api.BackupStatus{
					Phase:               velerov1api.BackupPhaseCompleted,
					Version:             1,
					StartTimestamp:      metav1.NewTime(now),
					CompletionTimestamp: metav1.NewTime(now),
					Expiration:          metav1.NewTime(now),
				},
			},
		},

		// Failed
		{
			name:           "backup with existing backup will fail",
			backupExists:   true,
			backup:         defaultBackup().Backup(),
			backupLocation: defaultBackupLocation,
			expectedResult: &velerov1api.Backup{
				TypeMeta: metav1.TypeMeta{
					Kind:       "Backup",
					APIVersion: "velero.io/v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Namespace: velerov1api.DefaultNamespace,
					Name:      "backup-1",
					Labels: map[string]string{
						"velero.io/storage-location": "loc-1",
					},
				},
				Spec: velerov1api.BackupSpec{
					StorageLocation: defaultBackupLocation.Name,
				},
				Status: velerov1api.BackupStatus{
					Phase:               velerov1api.BackupPhaseFailed,
					Version:             1,
					StartTimestamp:      metav1.NewTime(now),
					CompletionTimestamp: metav1.NewTime(now),
					Expiration:          metav1.NewTime(now),
				},
			},
		},
		{
			name:                "error when checking if backup exists will cause backup to fail",
			backup:              defaultBackup().Backup(),
			existenceCheckError: errors.New("Backup already exists in object storage"),
			backupLocation:      defaultBackupLocation,
			expectedResult: &velerov1api.Backup{
				TypeMeta: metav1.TypeMeta{
					Kind:       "Backup",
					APIVersion: "velero.io/v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Namespace: velerov1api.DefaultNamespace,
					Name:      "backup-1",
					Labels: map[string]string{
						"velero.io/storage-location": "loc-1",
					},
				},
				Spec: velerov1api.BackupSpec{
					StorageLocation: defaultBackupLocation.Name,
				},
				Status: velerov1api.BackupStatus{
					Phase:               velerov1api.BackupPhaseFailed,
					Version:             1,
					StartTimestamp:      metav1.NewTime(now),
					CompletionTimestamp: metav1.NewTime(now),
					Expiration:          metav1.NewTime(now),
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
				genericController:      newGenericController("backup-test", logger),
				client:                 clientset.VeleroV1(),
				lister:                 sharedInformers.Velero().V1().Backups().Lister(),
				listerPodVolumes:       sharedInformers.Velero().V1().PodVolumeBackups().Lister(),
				backupLocationLister:   sharedInformers.Velero().V1().BackupStorageLocations().Lister(),
				snapshotLocationLister: sharedInformers.Velero().V1().VolumeSnapshotLocations().Lister(),
				defaultBackupLocation:  defaultBackupLocation.Name,
				backupTracker:          NewBackupTracker(),
				metrics:                metrics.NewServerMetrics(),
				clock:                  clock.NewFakeClock(now),
				newPluginManager:       func(logrus.FieldLogger) clientmgmt.Manager { return pluginManager },
				newBackupStore: func(*velerov1api.BackupStorageLocation, persistence.ObjectStoreGetter, logrus.FieldLogger) (persistence.BackupStore, error) {
					return backupStore, nil
				},
				backupper: backupper,
			}

			pluginManager.On("GetBackupItemActions").Return(nil, nil)
			pluginManager.On("CleanupClients").Return(nil)

			backupper.On("Backup", mock.Anything, mock.Anything, mock.Anything, []velero.BackupItemAction(nil), pluginManager).Return(nil)

			// Ensure we have a CompletionTimestamp when uploading.
			// Failures will display the bytes in buf.
			completionTimestampIsPresent := func(buf *bytes.Buffer) bool {
				return strings.Contains(buf.String(), `"completionTimestamp": "2006-01-02T22:04:05Z"`)
			}
			backupStore.On("BackupExists", test.backupLocation.Spec.StorageType.ObjectStorage.Bucket, test.backup.Name).Return(test.backupExists, test.existenceCheckError)
			backupStore.On("PutBackup", test.backup.Name, mock.MatchedBy(completionTimestampIsPresent), mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)

			// add the test's backup to the informer/lister store
			require.NotNil(t, test.backup)
			require.NoError(t, sharedInformers.Velero().V1().Backups().Informer().GetStore().Add(test.backup))

			// add the default backup storage location to the clientset and the informer/lister store
			_, err := clientset.VeleroV1().BackupStorageLocations(defaultBackupLocation.Namespace).Create(defaultBackupLocation)
			require.NoError(t, err)

			require.NoError(t, sharedInformers.Velero().V1().BackupStorageLocations().Informer().GetStore().Add(defaultBackupLocation))

			// add the test's backup storage location to the clientset and the informer/lister store
			// if it's different than the default
			if test.backupLocation != nil && test.backupLocation != defaultBackupLocation {
				_, err := clientset.VeleroV1().BackupStorageLocations(test.backupLocation.Namespace).Create(test.backupLocation)
				require.NoError(t, err)

				require.NoError(t, sharedInformers.Velero().V1().BackupStorageLocations().Informer().GetStore().Add(test.backupLocation))
			}

			require.NoError(t, c.processBackup(fmt.Sprintf("%s/%s", test.backup.Namespace, test.backup.Name)))

			res, err := clientset.VeleroV1().Backups(test.backup.Namespace).Get(test.backup.Name, metav1.GetOptions{})
			require.NoError(t, err)

			assert.Equal(t, test.expectedResult, res)
		})
	}
}

func TestValidateAndGetSnapshotLocations(t *testing.T) {
	tests := []struct {
		name                                string
		backup                              *velerov1api.Backup
		locations                           []*velerotest.TestVolumeSnapshotLocation
		defaultLocations                    map[string]string
		expectedVolumeSnapshotLocationNames []string // adding these in the expected order will allow to test with better msgs in case of a test failure
		expectedErrors                      string
		expectedSuccess                     bool
	}{
		{
			name:   "location name does not correspond to any existing location",
			backup: defaultBackup().Phase(velerov1api.BackupPhaseNew).VolumeSnapshotLocations("random-name").Backup(),
			locations: []*velerotest.TestVolumeSnapshotLocation{
				velerotest.NewTestVolumeSnapshotLocation().WithProvider("aws").WithName("aws-us-east-1"),
				velerotest.NewTestVolumeSnapshotLocation().WithProvider("aws").WithName("aws-us-west-1"),
				velerotest.NewTestVolumeSnapshotLocation().WithProvider("fake-provider").WithName("some-name"),
			},
			expectedErrors: "a VolumeSnapshotLocation CRD for the location random-name with the name specified in the backup spec needs to be created before this snapshot can be executed. Error: volumesnapshotlocation.velero.io \"random-name\" not found", expectedSuccess: false,
		},
		{
			name:   "duplicate locationName per provider: should filter out dups",
			backup: defaultBackup().Phase(velerov1api.BackupPhaseNew).VolumeSnapshotLocations("aws-us-west-1", "aws-us-west-1").Backup(),
			locations: []*velerotest.TestVolumeSnapshotLocation{
				velerotest.NewTestVolumeSnapshotLocation().WithProvider("aws").WithName("aws-us-east-1"),
				velerotest.NewTestVolumeSnapshotLocation().WithProvider("aws").WithName("aws-us-west-1"),
			},
			expectedVolumeSnapshotLocationNames: []string{"aws-us-west-1"},
			expectedSuccess:                     true,
		},
		{
			name:   "multiple non-dupe location names per provider should error",
			backup: defaultBackup().Phase(velerov1api.BackupPhaseNew).VolumeSnapshotLocations("aws-us-east-1", "aws-us-west-1").Backup(),
			locations: []*velerotest.TestVolumeSnapshotLocation{
				velerotest.NewTestVolumeSnapshotLocation().WithProvider("aws").WithName("aws-us-east-1"),
				velerotest.NewTestVolumeSnapshotLocation().WithProvider("aws").WithName("aws-us-west-1"),
				velerotest.NewTestVolumeSnapshotLocation().WithProvider("fake-provider").WithName("some-name"),
			},
			expectedErrors:  "more than one VolumeSnapshotLocation name specified for provider aws: aws-us-west-1; unexpected name was aws-us-east-1",
			expectedSuccess: false,
		},
		{
			name:   "no location name for the provider exists, only one VSL for the provider: use it",
			backup: defaultBackup().Phase(velerov1api.BackupPhaseNew).Backup(),
			locations: []*velerotest.TestVolumeSnapshotLocation{
				velerotest.NewTestVolumeSnapshotLocation().WithProvider("aws").WithName("aws-us-east-1"),
			},
			expectedVolumeSnapshotLocationNames: []string{"aws-us-east-1"},
			expectedSuccess:                     true,
		},
		{
			name:   "no location name for the provider exists, no default, more than one VSL for the provider: error",
			backup: defaultBackup().Phase(velerov1api.BackupPhaseNew).Backup(),
			locations: []*velerotest.TestVolumeSnapshotLocation{
				velerotest.NewTestVolumeSnapshotLocation().WithProvider("aws").WithName("aws-us-east-1"),
				velerotest.NewTestVolumeSnapshotLocation().WithProvider("aws").WithName("aws-us-west-1"),
			},
			expectedErrors: "provider aws has more than one possible volume snapshot location, and none were specified explicitly or as a default",
		},
		{
			name:             "no location name for the provider exists, more than one VSL for the provider: the provider's default should be added",
			backup:           defaultBackup().Phase(velerov1api.BackupPhaseNew).Backup(),
			defaultLocations: map[string]string{"aws": "aws-us-east-1"},
			locations: []*velerotest.TestVolumeSnapshotLocation{
				velerotest.NewTestVolumeSnapshotLocation().WithName("aws-us-east-1").WithProvider("aws"),
				velerotest.NewTestVolumeSnapshotLocation().WithName("aws-us-west-1").WithProvider("aws"),
			},
			expectedVolumeSnapshotLocationNames: []string{"aws-us-east-1"},
			expectedSuccess:                     true,
		},
		{
			name:            "no existing location name and no default location name given",
			backup:          defaultBackup().Phase(velerov1api.BackupPhaseNew).Backup(),
			expectedSuccess: true,
		},
		{
			name:             "multiple location names for a provider, default location name for another provider",
			backup:           defaultBackup().Phase(velerov1api.BackupPhaseNew).VolumeSnapshotLocations("aws-us-west-1", "aws-us-west-1").Backup(),
			defaultLocations: map[string]string{"fake-provider": "some-name"},
			locations: []*velerotest.TestVolumeSnapshotLocation{
				velerotest.NewTestVolumeSnapshotLocation().WithProvider("aws").WithName("aws-us-west-1"),
				velerotest.NewTestVolumeSnapshotLocation().WithProvider("fake-provider").WithName("some-name"),
			},
			expectedVolumeSnapshotLocationNames: []string{"aws-us-west-1", "some-name"},
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
				snapshotLocationLister:   sharedInformers.Velero().V1().VolumeSnapshotLocations().Lister(),
				defaultSnapshotLocations: test.defaultLocations,
			}

			// set up a Backup object to represent what we expect to be passed to backupper.Backup()
			backup := test.backup.DeepCopy()
			backup.Spec.VolumeSnapshotLocations = test.backup.Spec.VolumeSnapshotLocations
			for _, location := range test.locations {
				require.NoError(t, sharedInformers.Velero().V1().VolumeSnapshotLocations().Informer().GetStore().Add(location.VolumeSnapshotLocation))
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
