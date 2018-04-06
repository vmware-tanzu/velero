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
	"errors"
	"io"
	"io/ioutil"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	core "k8s.io/client-go/testing"
	"k8s.io/client-go/tools/cache"

	api "github.com/heptio/ark/pkg/apis/ark/v1"
	"github.com/heptio/ark/pkg/generated/clientset/versioned/fake"
	informers "github.com/heptio/ark/pkg/generated/informers/externalversions"
	"github.com/heptio/ark/pkg/restore"
	arktest "github.com/heptio/ark/pkg/util/test"
)

func TestFetchBackup(t *testing.T) {
	tests := []struct {
		name                string
		backupName          string
		informerBackups     []*api.Backup
		backupServiceBackup *api.Backup
		backupServiceError  error
		expectedRes         *api.Backup
		expectedErr         bool
	}{
		{
			name:            "lister has backup",
			backupName:      "backup-1",
			informerBackups: []*api.Backup{arktest.NewTestBackup().WithName("backup-1").Backup},
			expectedRes:     arktest.NewTestBackup().WithName("backup-1").Backup,
		},
		{
			name:                "backupSvc has backup",
			backupName:          "backup-1",
			backupServiceBackup: arktest.NewTestBackup().WithName("backup-1").Backup,
			expectedRes:         arktest.NewTestBackup().WithName("backup-1").Backup,
		},
		{
			name:               "no backup",
			backupName:         "backup-1",
			backupServiceError: errors.New("no backup here"),
			expectedErr:        true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var (
				client          = fake.NewSimpleClientset()
				restorer        = &fakeRestorer{}
				sharedInformers = informers.NewSharedInformerFactory(client, 0)
				backupSvc       = &arktest.BackupService{}
				logger          = arktest.NewLogger()
				pluginManager   = &MockManager{}
			)

			c := NewRestoreController(
				api.DefaultNamespace,
				sharedInformers.Ark().V1().Restores(),
				client.ArkV1(),
				client.ArkV1(),
				restorer,
				backupSvc,
				"bucket",
				sharedInformers.Ark().V1().Backups(),
				false,
				logger,
				pluginManager,
			).(*restoreController)

			for _, itm := range test.informerBackups {
				sharedInformers.Ark().V1().Backups().Informer().GetStore().Add(itm)
			}

			if test.backupServiceBackup != nil || test.backupServiceError != nil {
				backupSvc.On("GetBackup", "bucket", test.backupName).Return(test.backupServiceBackup, test.backupServiceError)
			}

			backup, err := c.fetchBackup("bucket", test.backupName)

			if assert.Equal(t, test.expectedErr, err != nil) {
				assert.Equal(t, test.expectedRes, backup)
			}

			backupSvc.AssertExpectations(t)
		})
	}
}

func TestProcessRestore(t *testing.T) {
	tests := []struct {
		name                        string
		restoreKey                  string
		restore                     *api.Restore
		backup                      *api.Backup
		restorerError               error
		allowRestoreSnapshots       bool
		expectedErr                 bool
		expectedPhase               string
		expectedValidationErrors    []string
		expectedRestoreErrors       int
		expectedRestorerCall        *api.Restore
		backupServiceGetBackupError error
		uploadLogError              error
	}{
		{
			name:        "invalid key returns error",
			restoreKey:  "invalid/key/value",
			expectedErr: true,
		},
		{
			name:        "missing restore returns error",
			restoreKey:  "foo/bar",
			expectedErr: true,
		},
		{
			name:        "restore with phase InProgress does not get processed",
			restore:     arktest.NewTestRestore("foo", "bar", api.RestorePhaseInProgress).Restore,
			expectedErr: false,
		},
		{
			name:        "restore with phase Completed does not get processed",
			restore:     arktest.NewTestRestore("foo", "bar", api.RestorePhaseCompleted).Restore,
			expectedErr: false,
		},
		{
			name:        "restore with phase FailedValidation does not get processed",
			restore:     arktest.NewTestRestore("foo", "bar", api.RestorePhaseFailedValidation).Restore,
			expectedErr: false,
		},
		{
			name:                     "restore with both namespace in both includedNamespaces and excludedNamespaces fails validation",
			restore:                  NewRestore("foo", "bar", "backup-1", "another-1", "*", api.RestorePhaseNew).WithExcludedNamespace("another-1").Restore,
			backup:                   arktest.NewTestBackup().WithName("backup-1").Backup,
			expectedErr:              false,
			expectedPhase:            string(api.RestorePhaseFailedValidation),
			expectedValidationErrors: []string{"Invalid included/excluded namespace lists: excludes list cannot contain an item in the includes list: another-1"},
		},
		{
			name:                     "restore with resource in both includedResources and excludedResources fails validation",
			restore:                  NewRestore("foo", "bar", "backup-1", "*", "a-resource", api.RestorePhaseNew).WithExcludedResource("a-resource").Restore,
			backup:                   arktest.NewTestBackup().WithName("backup-1").Backup,
			expectedErr:              false,
			expectedPhase:            string(api.RestorePhaseFailedValidation),
			expectedValidationErrors: []string{"Invalid included/excluded resource lists: excludes list cannot contain an item in the includes list: a-resource"},
		},
		{
			name:                     "new restore with empty backup name fails validation",
			restore:                  NewRestore("foo", "bar", "", "ns-1", "", api.RestorePhaseNew).Restore,
			expectedErr:              false,
			expectedPhase:            string(api.RestorePhaseFailedValidation),
			expectedValidationErrors: []string{"BackupName must be non-empty and correspond to the name of a backup in object storage."},
		},

		{
			name:                        "restore with non-existent backup name fails",
			restore:                     arktest.NewTestRestore("foo", "bar", api.RestorePhaseNew).WithBackup("backup-1").WithIncludedNamespace("ns-1").Restore,
			expectedErr:                 false,
			expectedPhase:               string(api.RestorePhaseFailedValidation),
			expectedValidationErrors:    []string{"Error retrieving backup: no backup here"},
			backupServiceGetBackupError: errors.New("no backup here"),
		},
		{
			name:                  "restorer throwing an error causes the restore to fail",
			restore:               NewRestore("foo", "bar", "backup-1", "ns-1", "", api.RestorePhaseNew).Restore,
			backup:                arktest.NewTestBackup().WithName("backup-1").Backup,
			restorerError:         errors.New("blarg"),
			expectedErr:           false,
			expectedPhase:         string(api.RestorePhaseInProgress),
			expectedRestoreErrors: 1,
			expectedRestorerCall:  NewRestore("foo", "bar", "backup-1", "ns-1", "", api.RestorePhaseInProgress).Restore,
		},
		{
			name:                 "valid restore gets executed",
			restore:              NewRestore("foo", "bar", "backup-1", "ns-1", "", api.RestorePhaseNew).Restore,
			backup:               arktest.NewTestBackup().WithName("backup-1").Backup,
			expectedErr:          false,
			expectedPhase:        string(api.RestorePhaseInProgress),
			expectedRestorerCall: NewRestore("foo", "bar", "backup-1", "ns-1", "", api.RestorePhaseInProgress).Restore,
		},
		{
			name:                  "valid restore with RestorePVs=true gets executed when allowRestoreSnapshots=true",
			restore:               NewRestore("foo", "bar", "backup-1", "ns-1", "", api.RestorePhaseNew).WithRestorePVs(true).Restore,
			backup:                arktest.NewTestBackup().WithName("backup-1").Backup,
			allowRestoreSnapshots: true,
			expectedErr:           false,
			expectedPhase:         string(api.RestorePhaseInProgress),
			expectedRestorerCall:  NewRestore("foo", "bar", "backup-1", "ns-1", "", api.RestorePhaseInProgress).WithRestorePVs(true).Restore,
		},
		{
			name:                     "restore with RestorePVs=true fails validation when allowRestoreSnapshots=false",
			restore:                  NewRestore("foo", "bar", "backup-1", "ns-1", "", api.RestorePhaseNew).WithRestorePVs(true).Restore,
			backup:                   arktest.NewTestBackup().WithName("backup-1").Backup,
			expectedErr:              false,
			expectedPhase:            string(api.RestorePhaseFailedValidation),
			expectedValidationErrors: []string{"Server is not configured for PV snapshot restores"},
		},
		{
			name:          "restoration of nodes is not supported",
			restore:       NewRestore("foo", "bar", "backup-1", "ns-1", "nodes", api.RestorePhaseNew).Restore,
			backup:        arktest.NewTestBackup().WithName("backup-1").Backup,
			expectedErr:   false,
			expectedPhase: string(api.RestorePhaseFailedValidation),
			expectedValidationErrors: []string{
				"nodes are non-restorable resources",
				"Invalid included/excluded resource lists: excludes list cannot contain an item in the includes list: nodes",
			},
		},
		{
			name:          "restoration of events is not supported",
			restore:       NewRestore("foo", "bar", "backup-1", "ns-1", "events", api.RestorePhaseNew).Restore,
			backup:        arktest.NewTestBackup().WithName("backup-1").Backup,
			expectedErr:   false,
			expectedPhase: string(api.RestorePhaseFailedValidation),
			expectedValidationErrors: []string{
				"events are non-restorable resources",
				"Invalid included/excluded resource lists: excludes list cannot contain an item in the includes list: events",
			},
		},
		{
			name:          "restoration of events.events.k8s.io is not supported",
			restore:       NewRestore("foo", "bar", "backup-1", "ns-1", "events.events.k8s.io", api.RestorePhaseNew).Restore,
			backup:        arktest.NewTestBackup().WithName("backup-1").Backup,
			expectedErr:   false,
			expectedPhase: string(api.RestorePhaseFailedValidation),
			expectedValidationErrors: []string{
				"events.events.k8s.io are non-restorable resources",
				"Invalid included/excluded resource lists: excludes list cannot contain an item in the includes list: events.events.k8s.io",
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var (
				client          = fake.NewSimpleClientset()
				restorer        = &fakeRestorer{}
				sharedInformers = informers.NewSharedInformerFactory(client, 0)
				backupSvc       = &arktest.BackupService{}
				logger          = arktest.NewLogger()
				pluginManager   = &MockManager{}
			)

			defer restorer.AssertExpectations(t)
			defer backupSvc.AssertExpectations(t)

			c := NewRestoreController(
				api.DefaultNamespace,
				sharedInformers.Ark().V1().Restores(),
				client.ArkV1(),
				client.ArkV1(),
				restorer,
				backupSvc,
				"bucket",
				sharedInformers.Ark().V1().Backups(),
				test.allowRestoreSnapshots,
				logger,
				pluginManager,
			).(*restoreController)

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

					phase, found := unstructured.NestedString(patchMap, "status", "phase")
					if !found {
						err := errors.New("unable to get status.phase")
						t.Log(err.Error())
						return false, nil, err
					}

					res := test.restore.DeepCopy()

					// these are the fields that we expect to be set by
					// the controller

					res.Status.Phase = api.RestorePhase(phase)

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
			if test.uploadLogError != nil {
				errors.Ark = append(errors.Ark, "error uploading log file to object storage: "+test.uploadLogError.Error())
			}
			if test.expectedRestorerCall != nil {
				downloadedBackup := ioutil.NopCloser(bytes.NewReader([]byte("hello world")))
				backupSvc.On("DownloadBackup", mock.Anything, mock.Anything).Return(downloadedBackup, nil)
				restorer.On("Restore", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(warnings, errors)
				backupSvc.On("UploadRestoreLog", "bucket", test.restore.Spec.BackupName, test.restore.Name, mock.Anything).Return(test.uploadLogError)
				backupSvc.On("UploadRestoreResults", "bucket", test.restore.Spec.BackupName, test.restore.Name, mock.Anything).Return(nil)
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

			if test.backupServiceGetBackupError != nil {
				backupSvc.On("GetBackup", "bucket", test.restore.Spec.BackupName).Return(nil, test.backupServiceGetBackupError)
			}

			if test.restore != nil {
				pluginManager.On("GetRestoreItemActions", test.restore.Name).Return(nil, nil)
				pluginManager.On("CloseRestoreItemActions", test.restore.Name).Return(nil)
			}

			err = c.processRestore(key)
			backupSvc.AssertExpectations(t)
			restorer.AssertExpectations(t)

			assert.Equal(t, test.expectedErr, err != nil, "got error %v", err)

			actions := client.Actions()

			if test.expectedPhase == "" {
				require.Equal(t, 0, len(actions), "len(actions) should be zero")
				return
			}

			// validate Patch call 1 (setting phase, validation errs)
			require.True(t, len(actions) > 0, "len(actions) is too small")

			patchAction, ok := actions[0].(core.PatchAction)
			require.True(t, ok, "action is not a PatchAction")

			patch := make(map[string]interface{})
			require.NoError(t, json.Unmarshal(patchAction.GetPatch(), &patch), "cannot unmarshal patch")

			expectedStatusKeys := 1

			assert.True(t, hasKeyAndVal(patch, test.expectedPhase, "status", "phase"), "patch's status.phase does not match")

			if len(test.expectedValidationErrors) > 0 {
				errs, found := unstructured.NestedSlice(patch, "status", "validationErrors")
				require.True(t, found, "error getting patch's status.validationErrors")

				var errStrings []string
				for _, err := range errs {
					errStrings = append(errStrings, err.(string))
				}

				assert.Equal(t, test.expectedValidationErrors, errStrings, "patch's status.validationErrors does not match")

				expectedStatusKeys++
			}

			res, _ := unstructured.NestedMap(patch, "status")
			assert.Equal(t, expectedStatusKeys, len(res), "patch's status has the wrong number of keys")

			// if we don't expect a restore, validate it wasn't called and exit the test
			if test.expectedRestorerCall == nil {
				assert.Empty(t, restorer.Calls)
				assert.Zero(t, restorer.calledWithArg)
				return
			}
			assert.Equal(t, 1, len(restorer.Calls))

			// validate Patch call 2 (setting phase)
			patchAction, ok = actions[1].(core.PatchAction)
			require.True(t, ok, "action is not a PatchAction")

			require.NoError(t, json.Unmarshal(patchAction.GetPatch(), &patch), "cannot unmarshal patch")

			assert.Equal(t, 1, len(patch), "patch has wrong number of keys")

			res, _ = unstructured.NestedMap(patch, "status")
			expectedStatusKeys = 1

			assert.True(t, hasKeyAndVal(patch, string(api.RestorePhaseCompleted), "status", "phase"), "patch's status.phase does not match")

			if test.expectedRestoreErrors != 0 {
				assert.True(t, hasKeyAndVal(patch, float64(test.expectedRestoreErrors), "status", "errors"), "patch's status.errors does not match")
				expectedStatusKeys++
			}

			assert.Equal(t, expectedStatusKeys, len(res), "patch's status has wrong number of keys")

			// explicitly capturing the argument passed to Restore myself because
			// I want to validate the called arg as of the time of calling, but
			// the mock stores the pointer, which gets modified after
			assert.Equal(t, *test.expectedRestorerCall, restorer.calledWithArg)
		})
	}
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
		restore.WithExcludedResource(n)
	}

	return restore
}

type fakeRestorer struct {
	mock.Mock
	calledWithArg api.Restore
}

func (r *fakeRestorer) Restore(
	restore *api.Restore,
	backup *api.Backup,
	backupReader io.Reader,
	logger io.Writer,
	actions []restore.ItemAction,
) (api.RestoreResult, api.RestoreResult) {
	res := r.Called(restore, backup, backupReader, logger)

	r.calledWithArg = *restore

	return res.Get(0).(api.RestoreResult), res.Get(1).(api.RestoreResult)
}
