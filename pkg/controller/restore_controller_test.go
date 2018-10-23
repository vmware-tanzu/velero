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
	"encoding/json"
	"io"
	"io/ioutil"
	"testing"
	"time"

	api "github.com/heptio/ark/pkg/apis/ark/v1"
	"github.com/heptio/ark/pkg/generated/clientset/versioned/fake"
	informers "github.com/heptio/ark/pkg/generated/informers/externalversions"
	listers "github.com/heptio/ark/pkg/generated/listers/ark/v1"
	"github.com/heptio/ark/pkg/metrics"
	"github.com/heptio/ark/pkg/persistence"
	persistencemocks "github.com/heptio/ark/pkg/persistence/mocks"
	"github.com/heptio/ark/pkg/plugin"
	pluginmocks "github.com/heptio/ark/pkg/plugin/mocks"
	"github.com/heptio/ark/pkg/restore"
	"github.com/heptio/ark/pkg/util/collections"
	arktest "github.com/heptio/ark/pkg/util/test"
	"github.com/heptio/ark/pkg/volume"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	core "k8s.io/client-go/testing"
	"k8s.io/client-go/tools/cache"
)

func TestFetchBackupInfo(t *testing.T) {
	tests := []struct {
		name              string
		backupName        string
		informerLocations []*api.BackupStorageLocation
		informerBackups   []*api.Backup
		backupStoreBackup *api.Backup
		backupStoreError  error
		expectedRes       *api.Backup
		expectedErr       bool
	}{
		{
			name:              "lister has backup",
			backupName:        "backup-1",
			informerLocations: []*api.BackupStorageLocation{arktest.NewTestBackupStorageLocation().WithName("default").WithProvider("myCloud").WithObjectStorage("bucket").BackupStorageLocation},
			informerBackups:   []*api.Backup{arktest.NewTestBackup().WithName("backup-1").WithStorageLocation("default").Backup},
			expectedRes:       arktest.NewTestBackup().WithName("backup-1").WithStorageLocation("default").Backup,
		},
		{
			name:              "lister does not have a backup, but backupSvc does",
			backupName:        "backup-1",
			backupStoreBackup: arktest.NewTestBackup().WithName("backup-1").WithStorageLocation("default").Backup,
			informerLocations: []*api.BackupStorageLocation{arktest.NewTestBackupStorageLocation().WithName("default").WithProvider("myCloud").WithObjectStorage("bucket").BackupStorageLocation},
			informerBackups:   []*api.Backup{arktest.NewTestBackup().WithName("backup-1").WithStorageLocation("default").Backup},
			expectedRes:       arktest.NewTestBackup().WithName("backup-1").WithStorageLocation("default").Backup,
		},
		{
			name:             "no backup",
			backupName:       "backup-1",
			backupStoreError: errors.New("no backup here"),
			expectedErr:      true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var (
				client          = fake.NewSimpleClientset()
				restorer        = &fakeRestorer{}
				sharedInformers = informers.NewSharedInformerFactory(client, 0)
				logger          = arktest.NewLogger()
				pluginManager   = &pluginmocks.Manager{}
				backupStore     = &persistencemocks.BackupStore{}
			)

			defer restorer.AssertExpectations(t)
			defer backupStore.AssertExpectations(t)

			c := NewRestoreController(
				api.DefaultNamespace,
				sharedInformers.Ark().V1().Restores(),
				client.ArkV1(),
				client.ArkV1(),
				restorer,
				sharedInformers.Ark().V1().Backups(),
				sharedInformers.Ark().V1().BackupStorageLocations(),
				sharedInformers.Ark().V1().VolumeSnapshotLocations(),
				logger,
				logrus.InfoLevel,
				func(logrus.FieldLogger) plugin.Manager { return pluginManager },
				"default",
				metrics.NewServerMetrics(),
			).(*restoreController)

			c.newBackupStore = func(*api.BackupStorageLocation, persistence.ObjectStoreGetter, logrus.FieldLogger) (persistence.BackupStore, error) {
				return backupStore, nil
			}

			if test.backupStoreError == nil {
				for _, itm := range test.informerLocations {
					sharedInformers.Ark().V1().BackupStorageLocations().Informer().GetStore().Add(itm)
				}

				for _, itm := range test.informerBackups {
					sharedInformers.Ark().V1().Backups().Informer().GetStore().Add(itm)
				}
			}

			if test.backupStoreBackup != nil && test.backupStoreError != nil {
				panic("developer error - only one of backupStoreBackup, backupStoreError can be non-nil")
			}

			if test.backupStoreError != nil {
				// TODO why do I need .Maybe() here?
				backupStore.On("GetBackupMetadata", test.backupName).Return(nil, test.backupStoreError).Maybe()
			}
			if test.backupStoreBackup != nil {
				// TODO why do I need .Maybe() here?
				backupStore.On("GetBackupMetadata", test.backupName).Return(test.backupStoreBackup, nil).Maybe()
			}

			info, err := c.fetchBackupInfo(test.backupName, pluginManager)

			require.Equal(t, test.expectedErr, err != nil)
			assert.Equal(t, test.expectedRes, info.backup)
		})
	}
}

func TestProcessRestoreSkips(t *testing.T) {
	tests := []struct {
		name        string
		restoreKey  string
		restore     *api.Restore
		expectError bool
	}{
		{
			name:       "invalid key returns error",
			restoreKey: "invalid/key/value",
		},
		{
			name:        "missing restore returns error",
			restoreKey:  "foo/bar",
			expectError: true,
		},
		{
			name:       "restore with phase InProgress does not get processed",
			restoreKey: "foo/bar",
			restore:    arktest.NewTestRestore("foo", "bar", api.RestorePhaseInProgress).Restore,
		},
		{
			name:       "restore with phase Completed does not get processed",
			restoreKey: "foo/bar",
			restore:    arktest.NewTestRestore("foo", "bar", api.RestorePhaseCompleted).Restore,
		},
		{
			name:       "restore with phase FailedValidation does not get processed",
			restoreKey: "foo/bar",
			restore:    arktest.NewTestRestore("foo", "bar", api.RestorePhaseFailedValidation).Restore,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var (
				client          = fake.NewSimpleClientset()
				restorer        = &fakeRestorer{}
				sharedInformers = informers.NewSharedInformerFactory(client, 0)
				logger          = arktest.NewLogger()
			)

			c := NewRestoreController(
				api.DefaultNamespace,
				sharedInformers.Ark().V1().Restores(),
				client.ArkV1(),
				client.ArkV1(),
				restorer,
				sharedInformers.Ark().V1().Backups(),
				sharedInformers.Ark().V1().BackupStorageLocations(),
				sharedInformers.Ark().V1().VolumeSnapshotLocations(),
				logger,
				logrus.InfoLevel,
				nil,
				"default",
				metrics.NewServerMetrics(),
			).(*restoreController)

			if test.restore != nil {
				sharedInformers.Ark().V1().Restores().Informer().GetStore().Add(test.restore)
			}

			err := c.processRestore(test.restoreKey)

			assert.Equal(t, test.expectError, err != nil)
		})
	}
}

func TestProcessRestore(t *testing.T) {
	tests := []struct {
		name                            string
		restoreKey                      string
		location                        *api.BackupStorageLocation
		restore                         *api.Restore
		backup                          *api.Backup
		restorerError                   error
		expectedErr                     bool
		expectedPhase                   string
		expectedValidationErrors        []string
		expectedRestoreErrors           int
		expectedRestorerCall            *api.Restore
		backupStoreGetBackupMetadataErr error
		backupStoreGetBackupContentsErr error
		putRestoreLogErr                error
		expectedFinalPhase              string
	}{
		{
			name:                     "restore with both namespace in both includedNamespaces and excludedNamespaces fails validation",
			location:                 arktest.NewTestBackupStorageLocation().WithName("default").WithProvider("myCloud").WithObjectStorage("bucket").BackupStorageLocation,
			restore:                  NewRestore("foo", "bar", "backup-1", "another-1", "*", api.RestorePhaseNew).WithExcludedNamespace("another-1").Restore,
			backup:                   arktest.NewTestBackup().WithName("backup-1").WithStorageLocation("default").Backup,
			expectedErr:              false,
			expectedPhase:            string(api.RestorePhaseFailedValidation),
			expectedValidationErrors: []string{"Invalid included/excluded namespace lists: excludes list cannot contain an item in the includes list: another-1"},
		},
		{
			name:                     "restore with resource in both includedResources and excludedResources fails validation",
			location:                 arktest.NewTestBackupStorageLocation().WithName("default").WithProvider("myCloud").WithObjectStorage("bucket").BackupStorageLocation,
			restore:                  NewRestore("foo", "bar", "backup-1", "*", "a-resource", api.RestorePhaseNew).WithExcludedResource("a-resource").Restore,
			backup:                   arktest.NewTestBackup().WithName("backup-1").WithStorageLocation("default").Backup,
			expectedErr:              false,
			expectedPhase:            string(api.RestorePhaseFailedValidation),
			expectedValidationErrors: []string{"Invalid included/excluded resource lists: excludes list cannot contain an item in the includes list: a-resource"},
		},
		{
			name:                     "new restore with empty backup and schedule names fails validation",
			restore:                  NewRestore("foo", "bar", "", "ns-1", "", api.RestorePhaseNew).Restore,
			expectedErr:              false,
			expectedPhase:            string(api.RestorePhaseFailedValidation),
			expectedValidationErrors: []string{"Either a backup or schedule must be specified as a source for the restore, but not both"},
		},
		{
			name:                     "new restore with backup and schedule names provided fails validation",
			restore:                  NewRestore("foo", "bar", "backup-1", "ns-1", "", api.RestorePhaseNew).WithSchedule("sched-1").Restore,
			expectedErr:              false,
			expectedPhase:            string(api.RestorePhaseFailedValidation),
			expectedValidationErrors: []string{"Either a backup or schedule must be specified as a source for the restore, but not both"},
		},
		{
			name:     "valid restore with schedule name gets executed",
			location: arktest.NewTestBackupStorageLocation().WithName("default").WithProvider("myCloud").WithObjectStorage("bucket").BackupStorageLocation,
			restore:  NewRestore("foo", "bar", "", "ns-1", "", api.RestorePhaseNew).WithSchedule("sched-1").Restore,
			backup: arktest.
				NewTestBackup().
				WithName("backup-1").
				WithStorageLocation("default").
				WithLabel("ark-schedule", "sched-1").
				WithPhase(api.BackupPhaseCompleted).
				Backup,
			expectedErr:          false,
			expectedPhase:        string(api.RestorePhaseInProgress),
			expectedRestorerCall: NewRestore("foo", "bar", "backup-1", "ns-1", "", api.RestorePhaseInProgress).WithSchedule("sched-1").Restore,
		},
		{
			name:                            "restore with non-existent backup name fails",
			restore:                         NewRestore("foo", "bar", "backup-1", "ns-1", "*", api.RestorePhaseNew).Restore,
			expectedErr:                     false,
			expectedPhase:                   string(api.RestorePhaseFailedValidation),
			expectedValidationErrors:        []string{"Error retrieving backup: not able to fetch from backup storage"},
			backupStoreGetBackupMetadataErr: errors.New("no backup here"),
		},
		{
			name:                  "restorer throwing an error causes the restore to fail",
			location:              arktest.NewTestBackupStorageLocation().WithName("default").WithProvider("myCloud").WithObjectStorage("bucket").BackupStorageLocation,
			restore:               NewRestore("foo", "bar", "backup-1", "ns-1", "", api.RestorePhaseNew).Restore,
			backup:                arktest.NewTestBackup().WithName("backup-1").WithStorageLocation("default").Backup,
			restorerError:         errors.New("blarg"),
			expectedErr:           false,
			expectedPhase:         string(api.RestorePhaseInProgress),
			expectedRestoreErrors: 1,
			expectedRestorerCall:  NewRestore("foo", "bar", "backup-1", "ns-1", "", api.RestorePhaseInProgress).Restore,
		},
		{
			name:                 "valid restore gets executed",
			location:             arktest.NewTestBackupStorageLocation().WithName("default").WithProvider("myCloud").WithObjectStorage("bucket").BackupStorageLocation,
			restore:              NewRestore("foo", "bar", "backup-1", "ns-1", "", api.RestorePhaseNew).Restore,
			backup:               arktest.NewTestBackup().WithName("backup-1").WithStorageLocation("default").Backup,
			expectedErr:          false,
			expectedPhase:        string(api.RestorePhaseInProgress),
			expectedRestorerCall: NewRestore("foo", "bar", "backup-1", "ns-1", "", api.RestorePhaseInProgress).Restore,
		},
		{
			name:          "restoration of nodes is not supported",
			location:      arktest.NewTestBackupStorageLocation().WithName("default").WithProvider("myCloud").WithObjectStorage("bucket").BackupStorageLocation,
			restore:       NewRestore("foo", "bar", "backup-1", "ns-1", "nodes", api.RestorePhaseNew).Restore,
			backup:        arktest.NewTestBackup().WithName("backup-1").WithStorageLocation("default").Backup,
			expectedErr:   false,
			expectedPhase: string(api.RestorePhaseFailedValidation),
			expectedValidationErrors: []string{
				"nodes are non-restorable resources",
				"Invalid included/excluded resource lists: excludes list cannot contain an item in the includes list: nodes",
			},
		},
		{
			name:          "restoration of events is not supported",
			location:      arktest.NewTestBackupStorageLocation().WithName("default").WithProvider("myCloud").WithObjectStorage("bucket").BackupStorageLocation,
			restore:       NewRestore("foo", "bar", "backup-1", "ns-1", "events", api.RestorePhaseNew).Restore,
			backup:        arktest.NewTestBackup().WithName("backup-1").WithStorageLocation("default").Backup,
			expectedErr:   false,
			expectedPhase: string(api.RestorePhaseFailedValidation),
			expectedValidationErrors: []string{
				"events are non-restorable resources",
				"Invalid included/excluded resource lists: excludes list cannot contain an item in the includes list: events",
			},
		},
		{
			name:          "restoration of events.events.k8s.io is not supported",
			location:      arktest.NewTestBackupStorageLocation().WithName("default").WithProvider("myCloud").WithObjectStorage("bucket").BackupStorageLocation,
			restore:       NewRestore("foo", "bar", "backup-1", "ns-1", "events.events.k8s.io", api.RestorePhaseNew).Restore,
			backup:        arktest.NewTestBackup().WithName("backup-1").WithStorageLocation("default").Backup,
			expectedErr:   false,
			expectedPhase: string(api.RestorePhaseFailedValidation),
			expectedValidationErrors: []string{
				"events.events.k8s.io are non-restorable resources",
				"Invalid included/excluded resource lists: excludes list cannot contain an item in the includes list: events.events.k8s.io",
			},
		},
		{
			name:          "restoration of backups.ark.heptio.com is not supported",
			location:      arktest.NewTestBackupStorageLocation().WithName("default").WithProvider("myCloud").WithObjectStorage("bucket").BackupStorageLocation,
			restore:       NewRestore("foo", "bar", "backup-1", "ns-1", "backups.ark.heptio.com", api.RestorePhaseNew).Restore,
			backup:        arktest.NewTestBackup().WithName("backup-1").WithStorageLocation("default").Backup,
			expectedErr:   false,
			expectedPhase: string(api.RestorePhaseFailedValidation),
			expectedValidationErrors: []string{
				"backups.ark.heptio.com are non-restorable resources",
				"Invalid included/excluded resource lists: excludes list cannot contain an item in the includes list: backups.ark.heptio.com",
			},
		},
		{
			name:          "restoration of restores.ark.heptio.com is not supported",
			location:      arktest.NewTestBackupStorageLocation().WithName("default").WithProvider("myCloud").WithObjectStorage("bucket").BackupStorageLocation,
			restore:       NewRestore("foo", "bar", "backup-1", "ns-1", "restores.ark.heptio.com", api.RestorePhaseNew).Restore,
			backup:        arktest.NewTestBackup().WithName("backup-1").WithStorageLocation("default").Backup,
			expectedErr:   false,
			expectedPhase: string(api.RestorePhaseFailedValidation),
			expectedValidationErrors: []string{
				"restores.ark.heptio.com are non-restorable resources",
				"Invalid included/excluded resource lists: excludes list cannot contain an item in the includes list: restores.ark.heptio.com",
			},
		},
		{
			name:                            "backup download error results in failed restore",
			location:                        arktest.NewTestBackupStorageLocation().WithName("default").WithProvider("myCloud").WithObjectStorage("bucket").BackupStorageLocation,
			restore:                         NewRestore(api.DefaultNamespace, "bar", "backup-1", "ns-1", "", api.RestorePhaseNew).Restore,
			expectedPhase:                   string(api.RestorePhaseInProgress),
			expectedFinalPhase:              string(api.RestorePhaseFailed),
			backupStoreGetBackupContentsErr: errors.New("Couldn't download backup"),
			backup: arktest.NewTestBackup().WithName("backup-1").WithStorageLocation("default").Backup,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var (
				client          = fake.NewSimpleClientset()
				restorer        = &fakeRestorer{}
				sharedInformers = informers.NewSharedInformerFactory(client, 0)
				logger          = arktest.NewLogger()
				pluginManager   = &pluginmocks.Manager{}
				backupStore     = &persistencemocks.BackupStore{}
			)

			defer restorer.AssertExpectations(t)
			defer backupStore.AssertExpectations(t)

			c := NewRestoreController(
				api.DefaultNamespace,
				sharedInformers.Ark().V1().Restores(),
				client.ArkV1(),
				client.ArkV1(),
				restorer,
				sharedInformers.Ark().V1().Backups(),
				sharedInformers.Ark().V1().BackupStorageLocations(),
				sharedInformers.Ark().V1().VolumeSnapshotLocations(),
				logger,
				logrus.InfoLevel,
				func(logrus.FieldLogger) plugin.Manager { return pluginManager },
				"default",
				metrics.NewServerMetrics(),
			).(*restoreController)

			c.newBackupStore = func(*api.BackupStorageLocation, persistence.ObjectStoreGetter, logrus.FieldLogger) (persistence.BackupStore, error) {
				return backupStore, nil
			}

			if test.location != nil {
				sharedInformers.Ark().V1().BackupStorageLocations().Informer().GetStore().Add(test.location)
			}
			if test.backup != nil {
				sharedInformers.Ark().V1().Backups().Informer().GetStore().Add(test.backup)
			}

			if test.restore != nil {
				sharedInformers.Ark().V1().Restores().Informer().GetStore().Add(test.restore)

				// this is necessary so the Patch() call returns the appropriate object
				client.PrependReactor("patch", "restores", func(action core.Action) (bool, runtime.Object, error) {
					if test.restore == nil {
						return true, nil, nil
					}

					patch := action.(core.PatchAction).GetPatch()
					patchMap := make(map[string]interface{})

					if err := json.Unmarshal(patch, &patchMap); err != nil {
						t.Logf("error unmarshalling patch: %s\n", err)
						return false, nil, err
					}

					phase, err := collections.GetString(patchMap, "status.phase")
					if err != nil {
						t.Logf("error getting status.phase: %s\n", err)
						return false, nil, err
					}

					res := test.restore.DeepCopy()

					// these are the fields that we expect to be set by
					// the controller

					res.Status.Phase = api.RestorePhase(phase)

					if backupName, err := collections.GetString(patchMap, "spec.backupName"); err == nil {
						res.Spec.BackupName = backupName
					}

					return true, res, nil
				})
			}

			if test.backup != nil {
				sharedInformers.Ark().V1().Backups().Informer().GetStore().Add(test.backup)
			}

			var warnings, errors api.RestoreResult
			if test.restorerError != nil {
				errors.Namespaces = map[string][]string{"ns-1": {test.restorerError.Error()}}
			}
			if test.putRestoreLogErr != nil {
				errors.Ark = append(errors.Ark, "error uploading log file to object storage: "+test.putRestoreLogErr.Error())
			}
			if test.expectedRestorerCall != nil {
				backupStore.On("GetBackupContents", test.backup.Name).Return(ioutil.NopCloser(bytes.NewReader([]byte("hello world"))), nil)

				restorer.On("Restore", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(warnings, errors)

				backupStore.On("PutRestoreLog", test.backup.Name, test.restore.Name, mock.Anything).Return(test.putRestoreLogErr)

				backupStore.On("PutRestoreResults", test.backup.Name, test.restore.Name, mock.Anything).Return(nil)

				volumeSnapshots := []*volume.Snapshot{
					{
						Spec: volume.SnapshotSpec{
							PersistentVolumeName: "test-pv",
							BackupName:           test.backup.Name,
						},
					},
				}
				backupStore.On("GetBackupVolumeSnapshots", test.backup.Name).Return(volumeSnapshots, nil)
			}

			var (
				key = test.restoreKey
				err error
			)
			if key == "" && test.restore != nil {
				key, err = cache.MetaNamespaceKeyFunc(test.restore)
				if err != nil {
					panic(err)
				}
			}

			if test.backupStoreGetBackupMetadataErr != nil {
				// TODO why do I need .Maybe() here?
				backupStore.On("GetBackupMetadata", test.restore.Spec.BackupName).Return(nil, test.backupStoreGetBackupMetadataErr).Maybe()
			}

			if test.backupStoreGetBackupContentsErr != nil {
				// TODO why do I need .Maybe() here?
				backupStore.On("GetBackupContents", test.restore.Spec.BackupName).Return(nil, test.backupStoreGetBackupContentsErr).Maybe()
			}

			if test.restore != nil {
				pluginManager.On("GetRestoreItemActions").Return(nil, nil)
				pluginManager.On("CleanupClients")
			}

			err = c.processRestore(key)

			assert.Equal(t, test.expectedErr, err != nil, "got error %v", err)
			actions := client.Actions()

			if test.expectedPhase == "" {
				require.Equal(t, 0, len(actions), "len(actions) should be zero")
				return
			}

			// structs and func for decoding patch content
			type SpecPatch struct {
				BackupName string `json:"backupName"`
			}

			type StatusPatch struct {
				Phase            api.RestorePhase `json:"phase"`
				ValidationErrors []string         `json:"validationErrors"`
				Errors           int              `json:"errors"`
			}

			type Patch struct {
				Spec   SpecPatch   `json:"spec,omitempty"`
				Status StatusPatch `json:"status"`
			}

			decode := func(decoder *json.Decoder) (interface{}, error) {
				actual := new(Patch)
				err := decoder.Decode(actual)

				return *actual, err
			}

			// validate Patch call 1 (setting phase, validation errs)
			require.True(t, len(actions) > 0, "len(actions) is too small")

			expected := Patch{
				Status: StatusPatch{
					Phase:            api.RestorePhase(test.expectedPhase),
					ValidationErrors: test.expectedValidationErrors,
				},
			}

			if test.restore.Spec.ScheduleName != "" && test.backup != nil {
				expected.Spec = SpecPatch{
					BackupName: test.backup.Name,
				}
			}

			arktest.ValidatePatch(t, actions[0], expected, decode)

			// if we don't expect a restore, validate it wasn't called and exit the test
			if test.expectedRestorerCall == nil {
				assert.Empty(t, restorer.Calls)
				assert.Zero(t, restorer.calledWithArg)
				return
			}
			assert.Equal(t, 1, len(restorer.Calls))

			// validate Patch call 2 (setting phase)

			expected = Patch{
				Status: StatusPatch{
					Phase:  api.RestorePhaseCompleted,
					Errors: test.expectedRestoreErrors,
				},
			}
			// Override our default expectations if the case requires it
			if test.expectedFinalPhase != "" {
				expected = Patch{
					Status: StatusPatch{
						Phase:  api.RestorePhaseCompleted,
						Errors: test.expectedRestoreErrors,
					},
				}
			}

			arktest.ValidatePatch(t, actions[1], expected, decode)

			// explicitly capturing the argument passed to Restore myself because
			// I want to validate the called arg as of the time of calling, but
			// the mock stores the pointer, which gets modified after
			assert.Equal(t, *test.expectedRestorerCall, restorer.calledWithArg)
		})
	}
}

func TestValidateAndComplete(t *testing.T) {
	tests := []struct {
		name              string
		storageLocation   *api.BackupStorageLocation
		snapshotLocations []*api.VolumeSnapshotLocation
		backup            *api.Backup
		volumeSnapshots   []*volume.Snapshot
		restore           *api.Restore
		expectedErrs      []string
	}{
		{
			name:            "backup with .status.volumeBackups and no volumesnapshots.json file does not error",
			storageLocation: arktest.NewTestBackupStorageLocation().WithName("loc-1").BackupStorageLocation,
			backup:          arktest.NewTestBackup().WithName("backup-1").WithStorageLocation("loc-1").WithSnapshot("pv-1", "snap-1").Backup,
			volumeSnapshots: nil,
			restore:         arktest.NewDefaultTestRestore().WithBackup("backup-1").Restore,
			expectedErrs:    nil,
		},
		{
			name:            "backup with no .status.volumeBackups and volumesnapshots.json file does not error",
			storageLocation: arktest.NewTestBackupStorageLocation().WithName("loc-1").BackupStorageLocation,
			backup:          arktest.NewTestBackup().WithName("backup-1").WithStorageLocation("loc-1").Backup,
			volumeSnapshots: []*volume.Snapshot{{}},
			restore:         arktest.NewDefaultTestRestore().WithBackup("backup-1").Restore,
			expectedErrs:    nil,
		},
		{
			name:            "backup with both .status.volumeBackups and volumesnapshots.json file errors",
			storageLocation: arktest.NewTestBackupStorageLocation().WithName("loc-1").BackupStorageLocation,
			backup:          arktest.NewTestBackup().WithName("backup-1").WithStorageLocation("loc-1").WithSnapshot("pv-1", "snap-1").Backup,
			volumeSnapshots: []*volume.Snapshot{{}},
			restore:         arktest.NewDefaultTestRestore().WithBackup("backup-1").Restore,
			expectedErrs:    []string{"Backup must not have both .status.volumeBackups and a volumesnapshots.json.gz file in object storage"},
		},
		{
			name:            "backup with .status.volumeBackups, and >1 volume snapshot locations exist, errors",
			storageLocation: arktest.NewTestBackupStorageLocation().WithName("loc-1").BackupStorageLocation,
			snapshotLocations: []*api.VolumeSnapshotLocation{
				{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: api.DefaultNamespace,
						Name:      "vsl-1",
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: api.DefaultNamespace,
						Name:      "vsl-2",
					},
				},
			},
			backup:       arktest.NewTestBackup().WithName("backup-1").WithStorageLocation("loc-1").WithSnapshot("pv-1", "snap-1").Backup,
			restore:      arktest.NewDefaultTestRestore().WithBackup("backup-1").Restore,
			expectedErrs: []string{"Cannot restore backup with .status.volumeBackups when more than one volume snapshot location exists"},
		},
		{
			name:            "backup with .status.volumeBackups, and 1 volume snapshot location exists, does not error",
			storageLocation: arktest.NewTestBackupStorageLocation().WithName("loc-1").BackupStorageLocation,
			snapshotLocations: []*api.VolumeSnapshotLocation{
				{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: api.DefaultNamespace,
						Name:      "vsl-1",
					},
				},
			},
			backup:       arktest.NewTestBackup().WithName("backup-1").WithStorageLocation("loc-1").WithSnapshot("pv-1", "snap-1").Backup,
			restore:      arktest.NewDefaultTestRestore().WithBackup("backup-1").Restore,
			expectedErrs: nil,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var (
				clientset       = fake.NewSimpleClientset()
				sharedInformers = informers.NewSharedInformerFactory(clientset, 0)
				logger          = arktest.NewLogger()
				backupStore     = &persistencemocks.BackupStore{}
				controller      = &restoreController{
					genericController: &genericController{
						logger: logger,
					},
					namespace:              api.DefaultNamespace,
					backupLister:           sharedInformers.Ark().V1().Backups().Lister(),
					backupLocationLister:   sharedInformers.Ark().V1().BackupStorageLocations().Lister(),
					snapshotLocationLister: sharedInformers.Ark().V1().VolumeSnapshotLocations().Lister(),
					newBackupStore: func(*api.BackupStorageLocation, persistence.ObjectStoreGetter, logrus.FieldLogger) (persistence.BackupStore, error) {
						return backupStore, nil
					},
				}
			)

			require.NoError(t, sharedInformers.Ark().V1().BackupStorageLocations().Informer().GetStore().Add(tc.storageLocation))
			require.NoError(t, sharedInformers.Ark().V1().Backups().Informer().GetStore().Add(tc.backup))
			for _, loc := range tc.snapshotLocations {
				require.NoError(t, sharedInformers.Ark().V1().VolumeSnapshotLocations().Informer().GetStore().Add(loc))
			}
			backupStore.On("GetBackupVolumeSnapshots", tc.backup.Name).Return(tc.volumeSnapshots, nil)

			controller.validateAndComplete(tc.restore, nil)

			assert.Equal(t, tc.expectedErrs, tc.restore.Status.ValidationErrors)
		})
	}

}

func TestvalidateAndCompleteWhenScheduleNameSpecified(t *testing.T) {
	var (
		client          = fake.NewSimpleClientset()
		sharedInformers = informers.NewSharedInformerFactory(client, 0)
		logger          = arktest.NewLogger()
		pluginManager   = &pluginmocks.Manager{}
	)

	c := NewRestoreController(
		api.DefaultNamespace,
		sharedInformers.Ark().V1().Restores(),
		client.ArkV1(),
		client.ArkV1(),
		nil,
		sharedInformers.Ark().V1().Backups(),
		sharedInformers.Ark().V1().BackupStorageLocations(),
		sharedInformers.Ark().V1().VolumeSnapshotLocations(),
		logger,
		logrus.DebugLevel,
		nil,
		"default",
		nil,
	).(*restoreController)

	restore := &api.Restore{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: api.DefaultNamespace,
			Name:      "restore-1",
		},
		Spec: api.RestoreSpec{
			ScheduleName: "schedule-1",
		},
	}

	// no backups created from the schedule: fail validation
	require.NoError(t, sharedInformers.Ark().V1().Backups().Informer().GetStore().Add(arktest.
		NewTestBackup().
		WithName("backup-1").
		WithLabel("ark-schedule", "non-matching-schedule").
		WithPhase(api.BackupPhaseCompleted).
		Backup,
	))

	errs := c.validateAndComplete(restore, pluginManager)
	assert.Equal(t, []string{"No backups found for schedule"}, errs)
	assert.Empty(t, restore.Spec.BackupName)

	// no completed backups created from the schedule: fail validation
	require.NoError(t, sharedInformers.Ark().V1().Backups().Informer().GetStore().Add(arktest.
		NewTestBackup().
		WithName("backup-2").
		WithLabel("ark-schedule", "schedule-1").
		WithPhase(api.BackupPhaseInProgress).
		Backup,
	))

	errs = c.validateAndComplete(restore, pluginManager)
	assert.Equal(t, []string{"No completed backups found for schedule"}, errs)
	assert.Empty(t, restore.Spec.BackupName)

	// multiple completed backups created from the schedule: use most recent
	now := time.Now()

	require.NoError(t, sharedInformers.Ark().V1().Backups().Informer().GetStore().Add(arktest.
		NewTestBackup().
		WithName("foo").
		WithLabel("ark-schedule", "schedule-1").
		WithPhase(api.BackupPhaseCompleted).
		WithStartTimestamp(now).
		Backup,
	))
	require.NoError(t, sharedInformers.Ark().V1().Backups().Informer().GetStore().Add(arktest.
		NewTestBackup().
		WithName("bar").
		WithLabel("ark-schedule", "schedule-1").
		WithPhase(api.BackupPhaseCompleted).
		WithStartTimestamp(now.Add(time.Second)).
		Backup,
	))

	errs = c.validateAndComplete(restore, pluginManager)
	assert.Nil(t, errs)
	assert.Equal(t, "bar", restore.Spec.BackupName)
}

func TestBackupXorScheduleProvided(t *testing.T) {
	r := &api.Restore{}
	assert.False(t, backupXorScheduleProvided(r))

	r.Spec.BackupName = "backup-1"
	r.Spec.ScheduleName = "schedule-1"
	assert.False(t, backupXorScheduleProvided(r))

	r.Spec.BackupName = "backup-1"
	r.Spec.ScheduleName = ""
	assert.True(t, backupXorScheduleProvided(r))

	r.Spec.BackupName = ""
	r.Spec.ScheduleName = "schedule-1"
	assert.True(t, backupXorScheduleProvided(r))

}

func TestMostRecentCompletedBackup(t *testing.T) {
	backups := []*api.Backup{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: "a",
			},
			Status: api.BackupStatus{
				Phase: "",
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: "b",
			},
			Status: api.BackupStatus{
				Phase: api.BackupPhaseNew,
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: "c",
			},
			Status: api.BackupStatus{
				Phase: api.BackupPhaseInProgress,
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: "d",
			},
			Status: api.BackupStatus{
				Phase: api.BackupPhaseFailedValidation,
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: "e",
			},
			Status: api.BackupStatus{
				Phase: api.BackupPhaseFailed,
			},
		},
	}

	assert.Nil(t, mostRecentCompletedBackup(backups))

	now := time.Now()

	backups = append(backups, &api.Backup{
		ObjectMeta: metav1.ObjectMeta{
			Name: "foo",
		},
		Status: api.BackupStatus{
			Phase:          api.BackupPhaseCompleted,
			StartTimestamp: metav1.Time{Time: now},
		},
	})

	expected := &api.Backup{
		ObjectMeta: metav1.ObjectMeta{
			Name: "bar",
		},
		Status: api.BackupStatus{
			Phase:          api.BackupPhaseCompleted,
			StartTimestamp: metav1.Time{Time: now.Add(time.Second)},
		},
	}
	backups = append(backups, expected)

	assert.Equal(t, expected, mostRecentCompletedBackup(backups))
}

func NewRestore(ns, name, backup, includeNS, includeResource string, phase api.RestorePhase) *arktest.TestRestore {
	restore := arktest.NewTestRestore(ns, name, phase).WithBackup(backup)

	if includeNS != "" {
		restore = restore.WithIncludedNamespace(includeNS)
	}

	if includeResource != "" {
		restore = restore.WithIncludedResource(includeResource)
	}

	for _, n := range nonRestorableResources {
		restore = restore.WithExcludedResource(n)
	}

	return restore
}

type fakeRestorer struct {
	mock.Mock
	calledWithArg api.Restore
}

func (r *fakeRestorer) Restore(
	log logrus.FieldLogger,
	restore *api.Restore,
	backup *api.Backup,
	volumeSnapshots []*volume.Snapshot,
	backupReader io.Reader,
	actions []restore.ItemAction,
	snapshotLocationLister listers.VolumeSnapshotLocationLister,
	blockStoreGetter restore.BlockStoreGetter,
) (api.RestoreResult, api.RestoreResult) {
	res := r.Called(log, restore, backup, backupReader, actions)

	r.calledWithArg = *restore

	return res.Get(0).(api.RestoreResult), res.Get(1).(api.RestoreResult)
}
