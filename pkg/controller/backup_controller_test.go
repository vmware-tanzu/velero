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
	"bytes"
	"context"
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
	"k8s.io/apimachinery/pkg/version"
	kbclient "sigs.k8s.io/controller-runtime/pkg/client"

	velerov1api "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	pkgbackup "github.com/vmware-tanzu/velero/pkg/backup"
	"github.com/vmware-tanzu/velero/pkg/builder"
	"github.com/vmware-tanzu/velero/pkg/discovery"
	"github.com/vmware-tanzu/velero/pkg/generated/clientset/versioned/fake"
	informers "github.com/vmware-tanzu/velero/pkg/generated/informers/externalversions"
	"github.com/vmware-tanzu/velero/pkg/metrics"
	"github.com/vmware-tanzu/velero/pkg/persistence"
	persistencemocks "github.com/vmware-tanzu/velero/pkg/persistence/mocks"
	"github.com/vmware-tanzu/velero/pkg/plugin/clientmgmt"
	pluginmocks "github.com/vmware-tanzu/velero/pkg/plugin/mocks"
	"github.com/vmware-tanzu/velero/pkg/plugin/velero"
	velerotest "github.com/vmware-tanzu/velero/pkg/test"
	"github.com/vmware-tanzu/velero/pkg/util/boolptr"
	"github.com/vmware-tanzu/velero/pkg/util/logging"
)

type fakeBackupper struct {
	mock.Mock
}

func (b *fakeBackupper) Backup(logger logrus.FieldLogger, backup *pkgbackup.Request, backupFile io.Writer, actions []velero.BackupItemAction, volumeSnapshotterGetter pkgbackup.VolumeSnapshotterGetter) error {
	args := b.Called(logger, backup, backupFile, actions, volumeSnapshotterGetter)
	return args.Error(0)
}

func defaultBackup() *builder.BackupBuilder {
	return builder.ForBackup(velerov1api.DefaultNamespace, "backup-1")
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
			backup: defaultBackup().Phase(velerov1api.BackupPhaseFailedValidation).Result(),
		},
		{
			name:   "InProgress backup is not processed",
			key:    "velero/backup-1",
			backup: defaultBackup().Phase(velerov1api.BackupPhaseInProgress).Result(),
		},
		{
			name:   "Completed backup is not processed",
			key:    "velero/backup-1",
			backup: defaultBackup().Phase(velerov1api.BackupPhaseCompleted).Result(),
		},
		{
			name:   "Failed backup is not processed",
			key:    "velero/backup-1",
			backup: defaultBackup().Phase(velerov1api.BackupPhaseFailed).Result(),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			formatFlag := logging.FormatText
			var (
				sharedInformers = informers.NewSharedInformerFactory(fake.NewSimpleClientset(), 0)
				logger          = logging.DefaultLogger(logrus.DebugLevel, formatFlag)
			)

			c := &backupController{
				genericController: newGenericController("backup-test", logger),
				lister:            sharedInformers.Velero().V1().Backups().Lister(),
				formatFlag:        formatFlag,
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
	defaultBackupLocation := builder.ForBackupStorageLocation("velero", "loc-1").Result()

	tests := []struct {
		name           string
		backup         *velerov1api.Backup
		backupLocation *velerov1api.BackupStorageLocation
		expectedErrs   []string
	}{
		{
			name:           "invalid included/excluded resources fails validation",
			backup:         defaultBackup().IncludedResources("foo").ExcludedResources("foo").Result(),
			backupLocation: defaultBackupLocation,
			expectedErrs:   []string{"Invalid included/excluded resource lists: excludes list cannot contain an item in the includes list: foo"},
		},
		{
			name:           "invalid included/excluded namespaces fails validation",
			backup:         defaultBackup().IncludedNamespaces("foo").ExcludedNamespaces("foo").Result(),
			backupLocation: defaultBackupLocation,
			expectedErrs:   []string{"Invalid included/excluded namespace lists: excludes list cannot contain an item in the includes list: foo"},
		},
		{
			name:         "non-existent backup location fails validation",
			backup:       defaultBackup().StorageLocation("nonexistent").Result(),
			expectedErrs: []string{"an existing backup storage location wasn't specified at backup creation time and the default 'nonexistent' wasn't found. Please address this issue (see `velero backup-location -h` for options) and create a new backup. Error: backupstoragelocations.velero.io \"nonexistent\" not found"},
		},
		{
			name:           "backup for read-only backup location fails validation",
			backup:         defaultBackup().StorageLocation("read-only").Result(),
			backupLocation: builder.ForBackupStorageLocation("velero", "read-only").AccessMode(velerov1api.BackupStorageLocationAccessModeReadOnly).Result(),
			expectedErrs:   []string{"backup can't be created because backup storage location read-only is currently in read-only mode"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			formatFlag := logging.FormatText
			var (
				clientset       = fake.NewSimpleClientset(test.backup)
				sharedInformers = informers.NewSharedInformerFactory(clientset, 0)
				logger          = logging.DefaultLogger(logrus.DebugLevel, formatFlag)
			)

			apiServer := velerotest.NewAPIServer(t)
			discoveryHelper, err := discovery.NewHelper(apiServer.DiscoveryClient, logger)
			require.NoError(t, err)

			var fakeClient kbclient.Client
			if test.backupLocation != nil {
				fakeClient = newFakeClient(t, test.backupLocation)
			} else {
				fakeClient = newFakeClient(t)
			}

			c := &backupController{
				genericController:      newGenericController("backup-test", logger),
				discoveryHelper:        discoveryHelper,
				client:                 clientset.VeleroV1(),
				lister:                 sharedInformers.Velero().V1().Backups().Lister(),
				kbClient:               fakeClient,
				snapshotLocationLister: sharedInformers.Velero().V1().VolumeSnapshotLocations().Lister(),
				defaultBackupLocation:  defaultBackupLocation.Name,
				clock:                  &clock.RealClock{},
				formatFlag:             formatFlag,
			}

			require.NotNil(t, test.backup)
			require.NoError(t, sharedInformers.Velero().V1().Backups().Informer().GetStore().Add(test.backup))

			require.NoError(t, c.processBackup(fmt.Sprintf("%s/%s", test.backup.Namespace, test.backup.Name)))

			res, err := clientset.VeleroV1().Backups(test.backup.Namespace).Get(context.TODO(), test.backup.Name, metav1.GetOptions{})
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
			backup:                 defaultBackup().Result(),
			backupLocation:         builder.ForBackupStorageLocation("velero", "loc-1").Result(),
			expectedBackupLocation: "loc-1",
		},
		{
			name:                   "invalid storage location name should be handled while creating label",
			backup:                 defaultBackup().Result(),
			backupLocation:         builder.ForBackupStorageLocation("velero", "defaultdefaultdefaultdefaultdefaultdefaultdefaultdefaultdefaultdefault").Result(),
			expectedBackupLocation: "defaultdefaultdefaultdefaultdefaultdefaultdefaultdefaultd58343f",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			formatFlag := logging.FormatText

			var (
				clientset       = fake.NewSimpleClientset(test.backup)
				sharedInformers = informers.NewSharedInformerFactory(clientset, 0)
				logger          = logging.DefaultLogger(logrus.DebugLevel, formatFlag)
				fakeClient      = newFakeClient(t)
			)

			apiServer := velerotest.NewAPIServer(t)
			discoveryHelper, err := discovery.NewHelper(apiServer.DiscoveryClient, logger)
			require.NoError(t, err)

			c := &backupController{
				genericController:      newGenericController("backup-test", logger),
				discoveryHelper:        discoveryHelper,
				client:                 clientset.VeleroV1(),
				lister:                 sharedInformers.Velero().V1().Backups().Lister(),
				kbClient:               fakeClient,
				snapshotLocationLister: sharedInformers.Velero().V1().VolumeSnapshotLocations().Lister(),
				defaultBackupLocation:  test.backupLocation.Name,
				clock:                  &clock.RealClock{},
				formatFlag:             formatFlag,
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
			backup:             defaultBackup().Result(),
			expectedTTL:        defaultBackupTTL,
			expectedExpiration: metav1.NewTime(now.Add(defaultBackupTTL.Duration)),
		},
		{
			name:               "backup with TTL specified",
			backup:             defaultBackup().TTL(time.Hour).Result(),
			expectedTTL:        metav1.Duration{Duration: 1 * time.Hour},
			expectedExpiration: metav1.NewTime(now.Add(1 * time.Hour)),
		},
	}

	for _, test := range tests {
		formatFlag := logging.FormatText
		var (
			clientset       = fake.NewSimpleClientset(test.backup)
			fakeClient      = newFakeClient(t)
			logger          = logging.DefaultLogger(logrus.DebugLevel, formatFlag)
			sharedInformers = informers.NewSharedInformerFactory(clientset, 0)
		)

		t.Run(test.name, func(t *testing.T) {

			apiServer := velerotest.NewAPIServer(t)
			discoveryHelper, err := discovery.NewHelper(apiServer.DiscoveryClient, logger)
			require.NoError(t, err)

			c := &backupController{
				genericController:      newGenericController("backup-test", logger),
				discoveryHelper:        discoveryHelper,
				kbClient:               fakeClient,
				snapshotLocationLister: sharedInformers.Velero().V1().VolumeSnapshotLocations().Lister(),
				defaultBackupTTL:       defaultBackupTTL.Duration,
				clock:                  clock.NewFakeClock(now),
				formatFlag:             formatFlag,
			}

			res := c.prepareBackupRequest(test.backup)
			assert.NotNil(t, res)
			assert.Equal(t, test.expectedTTL, res.Spec.TTL)
			assert.Equal(t, test.expectedExpiration, *res.Status.Expiration)
		})
	}
}

func TestProcessBackupCompletions(t *testing.T) {
	defaultBackupLocation := builder.ForBackupStorageLocation("velero", "loc-1").Default(true).Bucket("store-1").Result()

	now, err := time.Parse(time.RFC1123Z, time.RFC1123Z)
	require.NoError(t, err)
	now = now.Local()
	timestamp := metav1.NewTime(now)

	tests := []struct {
		name                   string
		backup                 *velerov1api.Backup
		backupLocation         *velerov1api.BackupStorageLocation
		defaultVolumesToRestic bool
		expectedResult         *velerov1api.Backup
		backupExists           bool
		existenceCheckError    error
	}{
		// Completed
		{
			name:                   "backup with no backup location gets the default",
			backup:                 defaultBackup().Result(),
			backupLocation:         defaultBackupLocation,
			defaultVolumesToRestic: true,
			expectedResult: &velerov1api.Backup{
				TypeMeta: metav1.TypeMeta{
					Kind:       "Backup",
					APIVersion: "velero.io/v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Namespace: velerov1api.DefaultNamespace,
					Name:      "backup-1",
					Annotations: map[string]string{
						"velero.io/source-cluster-k8s-major-version": "1",
						"velero.io/source-cluster-k8s-minor-version": "16",
						"velero.io/source-cluster-k8s-gitversion":    "v1.16.4",
					},
					Labels: map[string]string{
						"velero.io/storage-location": "loc-1",
					},
				},
				Spec: velerov1api.BackupSpec{
					StorageLocation:        defaultBackupLocation.Name,
					DefaultVolumesToRestic: boolptr.True(),
				},
				Status: velerov1api.BackupStatus{
					Phase:               velerov1api.BackupPhaseCompleted,
					Version:             1,
					FormatVersion:       "1.1.0",
					StartTimestamp:      &timestamp,
					CompletionTimestamp: &timestamp,
					Expiration:          &timestamp,
				},
			},
		},
		{
			name:                   "backup with a specific backup location keeps it",
			backup:                 defaultBackup().StorageLocation("alt-loc").Result(),
			backupLocation:         builder.ForBackupStorageLocation("velero", "alt-loc").Bucket("store-1").Result(),
			defaultVolumesToRestic: false,
			expectedResult: &velerov1api.Backup{
				TypeMeta: metav1.TypeMeta{
					Kind:       "Backup",
					APIVersion: "velero.io/v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Namespace: velerov1api.DefaultNamespace,
					Name:      "backup-1",
					Annotations: map[string]string{
						"velero.io/source-cluster-k8s-major-version": "1",
						"velero.io/source-cluster-k8s-minor-version": "16",
						"velero.io/source-cluster-k8s-gitversion":    "v1.16.4",
					},
					Labels: map[string]string{
						"velero.io/storage-location": "alt-loc",
					},
				},
				Spec: velerov1api.BackupSpec{
					StorageLocation:        "alt-loc",
					DefaultVolumesToRestic: boolptr.False(),
				},
				Status: velerov1api.BackupStatus{
					Phase:               velerov1api.BackupPhaseCompleted,
					Version:             1,
					FormatVersion:       "1.1.0",
					StartTimestamp:      &timestamp,
					CompletionTimestamp: &timestamp,
					Expiration:          &timestamp,
				},
			},
		},
		{
			name:   "backup for a location with ReadWrite access mode gets processed",
			backup: defaultBackup().StorageLocation("read-write").Result(),
			backupLocation: builder.ForBackupStorageLocation("velero", "read-write").
				Bucket("store-1").
				AccessMode(velerov1api.BackupStorageLocationAccessModeReadWrite).
				Result(),
			defaultVolumesToRestic: true,
			expectedResult: &velerov1api.Backup{
				TypeMeta: metav1.TypeMeta{
					Kind:       "Backup",
					APIVersion: "velero.io/v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Namespace: velerov1api.DefaultNamespace,
					Name:      "backup-1",
					Annotations: map[string]string{
						"velero.io/source-cluster-k8s-major-version": "1",
						"velero.io/source-cluster-k8s-minor-version": "16",
						"velero.io/source-cluster-k8s-gitversion":    "v1.16.4",
					},
					Labels: map[string]string{
						"velero.io/storage-location": "read-write",
					},
				},
				Spec: velerov1api.BackupSpec{
					StorageLocation:        "read-write",
					DefaultVolumesToRestic: boolptr.True(),
				},
				Status: velerov1api.BackupStatus{
					Phase:               velerov1api.BackupPhaseCompleted,
					Version:             1,
					FormatVersion:       "1.1.0",
					StartTimestamp:      &timestamp,
					CompletionTimestamp: &timestamp,
					Expiration:          &timestamp,
				},
			},
		},
		{
			name:                   "backup with a TTL has expiration set",
			backup:                 defaultBackup().TTL(10 * time.Minute).Result(),
			backupLocation:         defaultBackupLocation,
			defaultVolumesToRestic: false,
			expectedResult: &velerov1api.Backup{
				TypeMeta: metav1.TypeMeta{
					Kind:       "Backup",
					APIVersion: "velero.io/v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Namespace: velerov1api.DefaultNamespace,
					Name:      "backup-1",
					Annotations: map[string]string{
						"velero.io/source-cluster-k8s-major-version": "1",
						"velero.io/source-cluster-k8s-minor-version": "16",
						"velero.io/source-cluster-k8s-gitversion":    "v1.16.4",
					},
					Labels: map[string]string{
						"velero.io/storage-location": "loc-1",
					},
				},
				Spec: velerov1api.BackupSpec{
					TTL:                    metav1.Duration{Duration: 10 * time.Minute},
					StorageLocation:        defaultBackupLocation.Name,
					DefaultVolumesToRestic: boolptr.False(),
				},
				Status: velerov1api.BackupStatus{
					Phase:               velerov1api.BackupPhaseCompleted,
					Version:             1,
					FormatVersion:       "1.1.0",
					Expiration:          &metav1.Time{now.Add(10 * time.Minute)},
					StartTimestamp:      &timestamp,
					CompletionTimestamp: &timestamp,
				},
			},
		},
		{
			name:                   "backup without an existing backup will succeed",
			backupExists:           false,
			backup:                 defaultBackup().Result(),
			backupLocation:         defaultBackupLocation,
			defaultVolumesToRestic: true,
			expectedResult: &velerov1api.Backup{
				TypeMeta: metav1.TypeMeta{
					Kind:       "Backup",
					APIVersion: "velero.io/v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Namespace: velerov1api.DefaultNamespace,
					Name:      "backup-1",
					Annotations: map[string]string{
						"velero.io/source-cluster-k8s-major-version": "1",
						"velero.io/source-cluster-k8s-minor-version": "16",
						"velero.io/source-cluster-k8s-gitversion":    "v1.16.4",
					},
					Labels: map[string]string{
						"velero.io/storage-location": "loc-1",
					},
				},
				Spec: velerov1api.BackupSpec{
					StorageLocation:        defaultBackupLocation.Name,
					DefaultVolumesToRestic: boolptr.True(),
				},
				Status: velerov1api.BackupStatus{
					Phase:               velerov1api.BackupPhaseCompleted,
					Version:             1,
					FormatVersion:       "1.1.0",
					StartTimestamp:      &timestamp,
					CompletionTimestamp: &timestamp,
					Expiration:          &timestamp,
				},
			},
		},
		{
			name:           "backup specifying a false value for 'DefaultVolumesToRestic' keeps it",
			backupExists:   false,
			backup:         defaultBackup().DefaultVolumesToRestic(false).Result(),
			backupLocation: defaultBackupLocation,
			// value set in the controller is different from that specified in the backup
			defaultVolumesToRestic: true,
			expectedResult: &velerov1api.Backup{
				TypeMeta: metav1.TypeMeta{
					Kind:       "Backup",
					APIVersion: "velero.io/v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Namespace: velerov1api.DefaultNamespace,
					Name:      "backup-1",
					Annotations: map[string]string{
						"velero.io/source-cluster-k8s-major-version": "1",
						"velero.io/source-cluster-k8s-minor-version": "16",
						"velero.io/source-cluster-k8s-gitversion":    "v1.16.4",
					},
					Labels: map[string]string{
						"velero.io/storage-location": "loc-1",
					},
				},
				Spec: velerov1api.BackupSpec{
					StorageLocation:        defaultBackupLocation.Name,
					DefaultVolumesToRestic: boolptr.False(),
				},
				Status: velerov1api.BackupStatus{
					Phase:               velerov1api.BackupPhaseCompleted,
					Version:             1,
					FormatVersion:       "1.1.0",
					StartTimestamp:      &timestamp,
					CompletionTimestamp: &timestamp,
					Expiration:          &timestamp,
				},
			},
		},
		{
			name:           "backup specifying a true value for 'DefaultVolumesToRestic' keeps it",
			backupExists:   false,
			backup:         defaultBackup().DefaultVolumesToRestic(true).Result(),
			backupLocation: defaultBackupLocation,
			// value set in the controller is different from that specified in the backup
			defaultVolumesToRestic: false,
			expectedResult: &velerov1api.Backup{
				TypeMeta: metav1.TypeMeta{
					Kind:       "Backup",
					APIVersion: "velero.io/v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Namespace: velerov1api.DefaultNamespace,
					Name:      "backup-1",
					Annotations: map[string]string{
						"velero.io/source-cluster-k8s-major-version": "1",
						"velero.io/source-cluster-k8s-minor-version": "16",
						"velero.io/source-cluster-k8s-gitversion":    "v1.16.4",
					},
					Labels: map[string]string{
						"velero.io/storage-location": "loc-1",
					},
				},
				Spec: velerov1api.BackupSpec{
					StorageLocation:        defaultBackupLocation.Name,
					DefaultVolumesToRestic: boolptr.True(),
				},
				Status: velerov1api.BackupStatus{
					Phase:               velerov1api.BackupPhaseCompleted,
					Version:             1,
					FormatVersion:       "1.1.0",
					StartTimestamp:      &timestamp,
					CompletionTimestamp: &timestamp,
					Expiration:          &timestamp,
				},
			},
		},
		{
			name:           "backup specifying no value for 'DefaultVolumesToRestic' gets the default true value",
			backupExists:   false,
			backup:         defaultBackup().Result(),
			backupLocation: defaultBackupLocation,
			// value set in the controller is different from that specified in the backup
			defaultVolumesToRestic: true,
			expectedResult: &velerov1api.Backup{
				TypeMeta: metav1.TypeMeta{
					Kind:       "Backup",
					APIVersion: "velero.io/v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Namespace: velerov1api.DefaultNamespace,
					Name:      "backup-1",
					Annotations: map[string]string{
						"velero.io/source-cluster-k8s-major-version": "1",
						"velero.io/source-cluster-k8s-minor-version": "16",
						"velero.io/source-cluster-k8s-gitversion":    "v1.16.4",
					},
					Labels: map[string]string{
						"velero.io/storage-location": "loc-1",
					},
				},
				Spec: velerov1api.BackupSpec{
					StorageLocation:        defaultBackupLocation.Name,
					DefaultVolumesToRestic: boolptr.True(),
				},
				Status: velerov1api.BackupStatus{
					Phase:               velerov1api.BackupPhaseCompleted,
					Version:             1,
					FormatVersion:       "1.1.0",
					StartTimestamp:      &timestamp,
					CompletionTimestamp: &timestamp,
					Expiration:          &timestamp,
				},
			},
		},
		{
			name:           "backup specifying no value for 'DefaultVolumesToRestic' gets the default false value",
			backupExists:   false,
			backup:         defaultBackup().Result(),
			backupLocation: defaultBackupLocation,
			// value set in the controller is different from that specified in the backup
			defaultVolumesToRestic: false,
			expectedResult: &velerov1api.Backup{
				TypeMeta: metav1.TypeMeta{
					Kind:       "Backup",
					APIVersion: "velero.io/v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Namespace: velerov1api.DefaultNamespace,
					Name:      "backup-1",
					Annotations: map[string]string{
						"velero.io/source-cluster-k8s-major-version": "1",
						"velero.io/source-cluster-k8s-minor-version": "16",
						"velero.io/source-cluster-k8s-gitversion":    "v1.16.4",
					},
					Labels: map[string]string{
						"velero.io/storage-location": "loc-1",
					},
				},
				Spec: velerov1api.BackupSpec{
					StorageLocation:        defaultBackupLocation.Name,
					DefaultVolumesToRestic: boolptr.False(),
				},
				Status: velerov1api.BackupStatus{
					Phase:               velerov1api.BackupPhaseCompleted,
					Version:             1,
					FormatVersion:       "1.1.0",
					StartTimestamp:      &timestamp,
					CompletionTimestamp: &timestamp,
					Expiration:          &timestamp,
				},
			},
		},

		// Failed
		{
			name:                   "backup with existing backup will fail",
			backupExists:           true,
			backup:                 defaultBackup().Result(),
			backupLocation:         defaultBackupLocation,
			defaultVolumesToRestic: true,
			expectedResult: &velerov1api.Backup{
				TypeMeta: metav1.TypeMeta{
					Kind:       "Backup",
					APIVersion: "velero.io/v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Namespace: velerov1api.DefaultNamespace,
					Name:      "backup-1",
					Annotations: map[string]string{
						"velero.io/source-cluster-k8s-major-version": "1",
						"velero.io/source-cluster-k8s-minor-version": "16",
						"velero.io/source-cluster-k8s-gitversion":    "v1.16.4",
					},
					Labels: map[string]string{
						"velero.io/storage-location": "loc-1",
					},
				},
				Spec: velerov1api.BackupSpec{
					StorageLocation:        defaultBackupLocation.Name,
					DefaultVolumesToRestic: boolptr.True(),
				},
				Status: velerov1api.BackupStatus{
					Phase:               velerov1api.BackupPhaseFailed,
					Version:             1,
					FormatVersion:       "1.1.0",
					StartTimestamp:      &timestamp,
					CompletionTimestamp: &timestamp,
					Expiration:          &timestamp,
				},
			},
		},
		{
			name:                   "error when checking if backup exists will cause backup to fail",
			backup:                 defaultBackup().Result(),
			existenceCheckError:    errors.New("Backup already exists in object storage"),
			backupLocation:         defaultBackupLocation,
			defaultVolumesToRestic: true,
			expectedResult: &velerov1api.Backup{
				TypeMeta: metav1.TypeMeta{
					Kind:       "Backup",
					APIVersion: "velero.io/v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Namespace: velerov1api.DefaultNamespace,
					Name:      "backup-1",
					Annotations: map[string]string{
						"velero.io/source-cluster-k8s-major-version": "1",
						"velero.io/source-cluster-k8s-minor-version": "16",
						"velero.io/source-cluster-k8s-gitversion":    "v1.16.4",
					},
					Labels: map[string]string{
						"velero.io/storage-location": "loc-1",
					},
				},
				Spec: velerov1api.BackupSpec{
					StorageLocation:        defaultBackupLocation.Name,
					DefaultVolumesToRestic: boolptr.True(),
				},
				Status: velerov1api.BackupStatus{
					Phase:               velerov1api.BackupPhaseFailed,
					Version:             1,
					FormatVersion:       "1.1.0",
					StartTimestamp:      &timestamp,
					CompletionTimestamp: &timestamp,
					Expiration:          &timestamp,
				},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			formatFlag := logging.FormatText
			var (
				clientset       = fake.NewSimpleClientset(test.backup)
				sharedInformers = informers.NewSharedInformerFactory(clientset, 0)
				logger          = logging.DefaultLogger(logrus.DebugLevel, formatFlag)
				pluginManager   = new(pluginmocks.Manager)
				backupStore     = new(persistencemocks.BackupStore)
				backupper       = new(fakeBackupper)
			)

			var fakeClient kbclient.Client
			// add the test's backup storage location if it's different than the default
			if test.backupLocation != nil && test.backupLocation != defaultBackupLocation {
				fakeClient = newFakeClient(t, test.backupLocation)
			} else {
				fakeClient = newFakeClient(t)
			}

			apiServer := velerotest.NewAPIServer(t)

			apiServer.DiscoveryClient.FakedServerVersion = &version.Info{
				Major:        "1",
				Minor:        "16",
				GitVersion:   "v1.16.4",
				GitCommit:    "FakeTest",
				GitTreeState: "",
				BuildDate:    "",
				GoVersion:    "",
				Compiler:     "",
				Platform:     "",
			}

			discoveryHelper, err := discovery.NewHelper(apiServer.DiscoveryClient, logger)
			require.NoError(t, err)

			c := &backupController{
				genericController:      newGenericController("backup-test", logger),
				discoveryHelper:        discoveryHelper,
				client:                 clientset.VeleroV1(),
				lister:                 sharedInformers.Velero().V1().Backups().Lister(),
				kbClient:               fakeClient,
				snapshotLocationLister: sharedInformers.Velero().V1().VolumeSnapshotLocations().Lister(),
				defaultBackupLocation:  defaultBackupLocation.Name,
				defaultVolumesToRestic: test.defaultVolumesToRestic,
				backupTracker:          NewBackupTracker(),
				metrics:                metrics.NewServerMetrics(),
				clock:                  clock.NewFakeClock(now),
				newPluginManager:       func(logrus.FieldLogger) clientmgmt.Manager { return pluginManager },
				newBackupStore: func(*velerov1api.BackupStorageLocation, persistence.ObjectStoreGetter, logrus.FieldLogger) (persistence.BackupStore, error) {
					return backupStore, nil
				},
				backupper:  backupper,
				formatFlag: formatFlag,
			}

			pluginManager.On("GetBackupItemActions").Return(nil, nil)
			pluginManager.On("CleanupClients").Return(nil)
			backupper.On("Backup", mock.Anything, mock.Anything, mock.Anything, []velero.BackupItemAction(nil), pluginManager).Return(nil)
			backupStore.On("BackupExists", test.backupLocation.Spec.StorageType.ObjectStorage.Bucket, test.backup.Name).Return(test.backupExists, test.existenceCheckError)

			// Ensure we have a CompletionTimestamp when uploading and that the backup name matches the backup in the object store.
			// Failures will display the bytes in buf.
			hasNameAndCompletionTimestamp := func(info persistence.BackupInfo) bool {
				buf := new(bytes.Buffer)
				buf.ReadFrom(info.Metadata)
				return info.Name == test.backup.Name &&
					strings.Contains(buf.String(), `"completionTimestamp": "2006-01-02T22:04:05Z"`)
			}
			backupStore.On("PutBackup", mock.MatchedBy(hasNameAndCompletionTimestamp)).Return(nil)

			// add the test's backup to the informer/lister store
			require.NotNil(t, test.backup)
			require.NoError(t, sharedInformers.Velero().V1().Backups().Informer().GetStore().Add(test.backup))

			// add the default backup storage location to the clientset and the informer/lister store
			require.NoError(t, fakeClient.Create(context.Background(), defaultBackupLocation))

			require.NoError(t, c.processBackup(fmt.Sprintf("%s/%s", test.backup.Namespace, test.backup.Name)))

			res, err := clientset.VeleroV1().Backups(test.backup.Namespace).Get(context.TODO(), test.backup.Name, metav1.GetOptions{})
			require.NoError(t, err)

			assert.Equal(t, test.expectedResult, res)

			// reset defaultBackupLocation resourceVersion
			defaultBackupLocation.ObjectMeta.ResourceVersion = ""
		})
	}
}

func TestValidateAndGetSnapshotLocations(t *testing.T) {
	tests := []struct {
		name                                string
		backup                              *velerov1api.Backup
		locations                           []*velerov1api.VolumeSnapshotLocation
		defaultLocations                    map[string]string
		expectedVolumeSnapshotLocationNames []string // adding these in the expected order will allow to test with better msgs in case of a test failure
		expectedErrors                      string
		expectedSuccess                     bool
	}{
		{
			name:   "location name does not correspond to any existing location",
			backup: defaultBackup().Phase(velerov1api.BackupPhaseNew).VolumeSnapshotLocations("random-name").Result(),
			locations: []*velerov1api.VolumeSnapshotLocation{
				builder.ForVolumeSnapshotLocation(velerov1api.DefaultNamespace, "aws-us-east-1").Provider("aws").Result(),
				builder.ForVolumeSnapshotLocation(velerov1api.DefaultNamespace, "aws-us-west-1").Provider("aws").Result(),
				builder.ForVolumeSnapshotLocation(velerov1api.DefaultNamespace, "some-name").Provider("fake-provider").Result(),
			},
			expectedErrors: "a VolumeSnapshotLocation CRD for the location random-name with the name specified in the backup spec needs to be created before this snapshot can be executed. Error: volumesnapshotlocation.velero.io \"random-name\" not found", expectedSuccess: false,
		},
		{
			name:   "duplicate locationName per provider: should filter out dups",
			backup: defaultBackup().Phase(velerov1api.BackupPhaseNew).VolumeSnapshotLocations("aws-us-west-1", "aws-us-west-1").Result(),
			locations: []*velerov1api.VolumeSnapshotLocation{
				builder.ForVolumeSnapshotLocation(velerov1api.DefaultNamespace, "aws-us-east-1").Provider("aws").Result(),
				builder.ForVolumeSnapshotLocation(velerov1api.DefaultNamespace, "aws-us-west-1").Provider("aws").Result(),
			},
			expectedVolumeSnapshotLocationNames: []string{"aws-us-west-1"},
			expectedSuccess:                     true,
		},
		{
			name:   "multiple non-dupe location names per provider should error",
			backup: defaultBackup().Phase(velerov1api.BackupPhaseNew).VolumeSnapshotLocations("aws-us-east-1", "aws-us-west-1").Result(),
			locations: []*velerov1api.VolumeSnapshotLocation{
				builder.ForVolumeSnapshotLocation(velerov1api.DefaultNamespace, "aws-us-east-1").Provider("aws").Result(),
				builder.ForVolumeSnapshotLocation(velerov1api.DefaultNamespace, "aws-us-west-1").Provider("aws").Result(),
				builder.ForVolumeSnapshotLocation(velerov1api.DefaultNamespace, "some-name").Provider("fake-provider").Result(),
			},
			expectedErrors:  "more than one VolumeSnapshotLocation name specified for provider aws: aws-us-west-1; unexpected name was aws-us-east-1",
			expectedSuccess: false,
		},
		{
			name:   "no location name for the provider exists, only one VSL for the provider: use it",
			backup: defaultBackup().Phase(velerov1api.BackupPhaseNew).Result(),
			locations: []*velerov1api.VolumeSnapshotLocation{
				builder.ForVolumeSnapshotLocation(velerov1api.DefaultNamespace, "aws-us-east-1").Provider("aws").Result(),
			},
			expectedVolumeSnapshotLocationNames: []string{"aws-us-east-1"},
			expectedSuccess:                     true,
		},
		{
			name:   "no location name for the provider exists, no default, more than one VSL for the provider: error",
			backup: defaultBackup().Phase(velerov1api.BackupPhaseNew).Result(),
			locations: []*velerov1api.VolumeSnapshotLocation{
				builder.ForVolumeSnapshotLocation(velerov1api.DefaultNamespace, "aws-us-east-1").Provider("aws").Result(),
				builder.ForVolumeSnapshotLocation(velerov1api.DefaultNamespace, "aws-us-west-1").Provider("aws").Result(),
			},
			expectedErrors: "provider aws has more than one possible volume snapshot location, and none were specified explicitly or as a default",
		},
		{
			name:             "no location name for the provider exists, more than one VSL for the provider: the provider's default should be added",
			backup:           defaultBackup().Phase(velerov1api.BackupPhaseNew).Result(),
			defaultLocations: map[string]string{"aws": "aws-us-east-1"},
			locations: []*velerov1api.VolumeSnapshotLocation{
				builder.ForVolumeSnapshotLocation(velerov1api.DefaultNamespace, "aws-us-east-1").Provider("aws").Result(),
				builder.ForVolumeSnapshotLocation(velerov1api.DefaultNamespace, "aws-us-west-1").Provider("aws").Result(),
			},
			expectedVolumeSnapshotLocationNames: []string{"aws-us-east-1"},
			expectedSuccess:                     true,
		},
		{
			name:            "no existing location name and no default location name given",
			backup:          defaultBackup().Phase(velerov1api.BackupPhaseNew).Result(),
			expectedSuccess: true,
		},
		{
			name:             "multiple location names for a provider, default location name for another provider",
			backup:           defaultBackup().Phase(velerov1api.BackupPhaseNew).VolumeSnapshotLocations("aws-us-west-1", "aws-us-west-1").Result(),
			defaultLocations: map[string]string{"fake-provider": "some-name"},
			locations: []*velerov1api.VolumeSnapshotLocation{
				builder.ForVolumeSnapshotLocation(velerov1api.DefaultNamespace, "aws-us-west-1").Provider("aws").Result(),
				builder.ForVolumeSnapshotLocation(velerov1api.DefaultNamespace, "some-name").Provider("fake-provider").Result(),
			},
			expectedVolumeSnapshotLocationNames: []string{"aws-us-west-1", "some-name"},
			expectedSuccess:                     true,
		},
		{
			name:   "location name does not correspond to any existing location and snapshotvolume disabled; should return empty VSL and no error",
			backup: defaultBackup().Phase(velerov1api.BackupPhaseNew).VolumeSnapshotLocations("random-name").SnapshotVolumes(false).Result(),
			locations: []*velerov1api.VolumeSnapshotLocation{
				builder.ForVolumeSnapshotLocation(velerov1api.DefaultNamespace, "aws-us-east-1").Provider("aws").Result(),
				builder.ForVolumeSnapshotLocation(velerov1api.DefaultNamespace, "aws-us-west-1").Provider("aws").Result(),
				builder.ForVolumeSnapshotLocation(velerov1api.DefaultNamespace, "some-name").Provider("fake-provider").Result(),
			},
			expectedVolumeSnapshotLocationNames: nil,
			expectedSuccess:                     true,
		},
		{
			name:   "duplicate locationName per provider and snapshotvolume disabled; should return empty VSL and no error",
			backup: defaultBackup().Phase(velerov1api.BackupPhaseNew).VolumeSnapshotLocations("aws-us-west-1", "aws-us-west-1").SnapshotVolumes(false).Result(),
			locations: []*velerov1api.VolumeSnapshotLocation{
				builder.ForVolumeSnapshotLocation(velerov1api.DefaultNamespace, "aws-us-east-1").Provider("aws").Result(),
				builder.ForVolumeSnapshotLocation(velerov1api.DefaultNamespace, "aws-us-west-1").Provider("aws").Result(),
			},
			expectedVolumeSnapshotLocationNames: nil,
			expectedSuccess:                     true,
		},
		{
			name:   "no location name for the provider exists, only one VSL created and snapshotvolume disabled; should return empty VSL and no error",
			backup: defaultBackup().Phase(velerov1api.BackupPhaseNew).SnapshotVolumes(false).Result(),
			locations: []*velerov1api.VolumeSnapshotLocation{
				builder.ForVolumeSnapshotLocation(velerov1api.DefaultNamespace, "aws-us-east-1").Provider("aws").Result(),
			},
			expectedVolumeSnapshotLocationNames: nil,
			expectedSuccess:                     true,
		},
		{
			name:   "multiple location names for a provider, no default location and backup has no location defined, but snapshotvolume disabled, should return empty VSL and no error",
			backup: defaultBackup().Phase(velerov1api.BackupPhaseNew).SnapshotVolumes(false).Result(),
			locations: []*velerov1api.VolumeSnapshotLocation{
				builder.ForVolumeSnapshotLocation(velerov1api.DefaultNamespace, "aws-us-west-1").Provider("aws").Result(),
				builder.ForVolumeSnapshotLocation(velerov1api.DefaultNamespace, "aws-us-east-1").Provider("aws").Result(),
			},
			expectedVolumeSnapshotLocationNames: nil,
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
				require.NoError(t, sharedInformers.Velero().V1().VolumeSnapshotLocations().Informer().GetStore().Add(location))
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

// Test_getLastSuccessBySchedule verifies that the getLastSuccessBySchedule helper function correctly returns
// the completion timestamp of the most recent completed backup for each schedule, including an entry for ad-hoc
// or non-scheduled backups.
func Test_getLastSuccessBySchedule(t *testing.T) {
	buildBackup := func(phase velerov1api.BackupPhase, completion time.Time, schedule string) *velerov1api.Backup {
		b := builder.ForBackup("", "").
			ObjectMeta(builder.WithLabels(velerov1api.ScheduleNameLabel, schedule)).
			Phase(phase)

		if !completion.IsZero() {
			b.CompletionTimestamp(completion)
		}

		return b.Result()
	}

	// create a static "base time" that can be used to easily construct completion timestamps
	// by using the .Add(...) method.
	baseTime, err := time.Parse(time.RFC1123, time.RFC1123)
	require.NoError(t, err)

	tests := []struct {
		name    string
		backups []*velerov1api.Backup
		want    map[string]time.Time
	}{
		{
			name:    "when backups is nil, an empty map is returned",
			backups: nil,
			want:    map[string]time.Time{},
		},
		{
			name:    "when backups is empty, an empty map is returned",
			backups: []*velerov1api.Backup{},
			want:    map[string]time.Time{},
		},
		{
			name: "when multiple completed backups for a schedule exist, the latest one is returned",
			backups: []*velerov1api.Backup{
				buildBackup(velerov1api.BackupPhaseCompleted, baseTime, "schedule-1"),
				buildBackup(velerov1api.BackupPhaseCompleted, baseTime.Add(time.Second), "schedule-1"),
				buildBackup(velerov1api.BackupPhaseCompleted, baseTime.Add(-time.Second), "schedule-1"),
			},
			want: map[string]time.Time{
				"schedule-1": baseTime.Add(time.Second),
			},
		},
		{
			name: "when the most recent backup for a schedule is Failed, the timestamp of the most recent Completed one is returned",
			backups: []*velerov1api.Backup{
				buildBackup(velerov1api.BackupPhaseCompleted, baseTime, "schedule-1"),
				buildBackup(velerov1api.BackupPhaseFailed, baseTime.Add(time.Second), "schedule-1"),
				buildBackup(velerov1api.BackupPhaseCompleted, baseTime.Add(-time.Second), "schedule-1"),
			},
			want: map[string]time.Time{
				"schedule-1": baseTime,
			},
		},
		{
			name: "when there are no Completed backups for a schedule, it's not returned",
			backups: []*velerov1api.Backup{
				buildBackup(velerov1api.BackupPhaseInProgress, baseTime, "schedule-1"),
				buildBackup(velerov1api.BackupPhaseFailed, baseTime.Add(time.Second), "schedule-1"),
				buildBackup(velerov1api.BackupPhasePartiallyFailed, baseTime.Add(-time.Second), "schedule-1"),
			},
			want: map[string]time.Time{},
		},
		{
			name: "when backups exist without a schedule, the most recent Completed one is returned",
			backups: []*velerov1api.Backup{
				buildBackup(velerov1api.BackupPhaseCompleted, baseTime, ""),
				buildBackup(velerov1api.BackupPhaseFailed, baseTime.Add(time.Second), ""),
				buildBackup(velerov1api.BackupPhaseCompleted, baseTime.Add(-time.Second), ""),
			},
			want: map[string]time.Time{
				"": baseTime,
			},
		},
		{
			name: "when backups exist for multiple schedules, the most recent Completed timestamp for each schedule is returned",
			backups: []*velerov1api.Backup{
				// ad-hoc backups (no schedule)
				buildBackup(velerov1api.BackupPhaseCompleted, baseTime.Add(30*time.Minute), ""),
				buildBackup(velerov1api.BackupPhaseFailed, baseTime.Add(time.Hour), ""),
				buildBackup(velerov1api.BackupPhaseCompleted, baseTime.Add(-time.Second), ""),

				// schedule-1
				buildBackup(velerov1api.BackupPhaseCompleted, baseTime, "schedule-1"),
				buildBackup(velerov1api.BackupPhaseFailed, baseTime.Add(time.Second), "schedule-1"),
				buildBackup(velerov1api.BackupPhaseCompleted, baseTime.Add(-time.Second), "schedule-1"),

				// schedule-2
				buildBackup(velerov1api.BackupPhaseCompleted, baseTime.Add(24*time.Hour), "schedule-2"),
				buildBackup(velerov1api.BackupPhaseCompleted, baseTime.Add(48*time.Hour), "schedule-2"),
				buildBackup(velerov1api.BackupPhaseCompleted, baseTime.Add(72*time.Hour), "schedule-2"),

				// schedule-3
				buildBackup(velerov1api.BackupPhaseNew, baseTime, "schedule-3"),
				buildBackup(velerov1api.BackupPhaseInProgress, baseTime.Add(time.Minute), "schedule-3"),
				buildBackup(velerov1api.BackupPhasePartiallyFailed, baseTime.Add(2*time.Minute), "schedule-3"),
			},
			want: map[string]time.Time{
				"":           baseTime.Add(30 * time.Minute),
				"schedule-1": baseTime,
				"schedule-2": baseTime.Add(72 * time.Hour),
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, getLastSuccessBySchedule(tc.backups))
		})
	}
}
