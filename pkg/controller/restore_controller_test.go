/*
Copyright 2017 Heptio Inc.

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
	"errors"
	"io"
	"io/ioutil"
	"testing"

	testlogger "github.com/sirupsen/logrus/hooks/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	core "k8s.io/client-go/testing"
	"k8s.io/client-go/tools/cache"

	api "github.com/heptio/ark/pkg/apis/ark/v1"
	"github.com/heptio/ark/pkg/generated/clientset/fake"
	informers "github.com/heptio/ark/pkg/generated/informers/externalversions"
	. "github.com/heptio/ark/pkg/util/test"
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
			informerBackups: []*api.Backup{NewTestBackup().WithName("backup-1").Backup},
			expectedRes:     NewTestBackup().WithName("backup-1").Backup,
		},
		{
			name:                "backupSvc has backup",
			backupName:          "backup-1",
			backupServiceBackup: NewTestBackup().WithName("backup-1").Backup,
			expectedRes:         NewTestBackup().WithName("backup-1").Backup,
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
				backupSvc       = &BackupService{}
				logger, _       = testlogger.NewNullLogger()
			)

			c := NewRestoreController(
				sharedInformers.Ark().V1().Restores(),
				client.ArkV1(),
				client.ArkV1(),
				restorer,
				backupSvc,
				"bucket",
				sharedInformers.Ark().V1().Backups(),
				false,
				logger,
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
		expectedRestoreUpdates      []*api.Restore
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
			restore:     NewTestRestore("foo", "bar", api.RestorePhaseInProgress).Restore,
			expectedErr: false,
		},
		{
			name:        "restore with phase Completed does not get processed",
			restore:     NewTestRestore("foo", "bar", api.RestorePhaseCompleted).Restore,
			expectedErr: false,
		},
		{
			name:        "restore with phase FailedValidation does not get processed",
			restore:     NewTestRestore("foo", "bar", api.RestorePhaseFailedValidation).Restore,
			expectedErr: false,
		},
		{
			name:        "restore with both namespace in both includedNamespaces and excludedNamespaces fails validation",
			restore:     NewRestore("foo", "bar", "backup-1", "another-1", "*", api.RestorePhaseNew).WithExcludedNamespace("another-1").Restore,
			backup:      NewTestBackup().WithName("backup-1").Backup,
			expectedErr: false,
			expectedRestoreUpdates: []*api.Restore{
				NewRestore("foo", "bar", "backup-1", "another-1", "*", api.RestorePhaseFailedValidation).WithExcludedNamespace("another-1").
					WithValidationError("Invalid included/excluded namespace lists: excludes list cannot contain an item in the includes list: another-1").
					Restore,
			},
		},
		{
			name:        "restore with resource in both includedResources and excludedResources fails validation",
			restore:     NewRestore("foo", "bar", "backup-1", "*", "a-resource", api.RestorePhaseNew).WithExcludedResource("a-resource").Restore,
			backup:      NewTestBackup().WithName("backup-1").Backup,
			expectedErr: false,
			expectedRestoreUpdates: []*api.Restore{
				NewRestore("foo", "bar", "backup-1", "*", "a-resource", api.RestorePhaseFailedValidation).WithExcludedResource("a-resource").
					WithValidationError("Invalid included/excluded resource lists: excludes list cannot contain an item in the includes list: a-resource").
					Restore,
			},
		},
		{
			name:        "new restore with empty backup name fails validation",
			restore:     NewRestore("foo", "bar", "", "ns-1", "", api.RestorePhaseNew).Restore,
			expectedErr: false,
			expectedRestoreUpdates: []*api.Restore{
				NewRestore("foo", "bar", "", "ns-1", "", api.RestorePhaseFailedValidation).
					WithValidationError("BackupName must be non-empty and correspond to the name of a backup in object storage.").
					Restore,
			},
		},

		{
			name:                        "restore with non-existent backup name fails",
			restore:                     NewTestRestore("foo", "bar", api.RestorePhaseNew).WithBackup("backup-1").WithIncludedNamespace("ns-1").Restore,
			expectedErr:                 false,
			backupServiceGetBackupError: errors.New("no backup here"),
			expectedRestoreUpdates: []*api.Restore{
				NewRestore("foo", "bar", "backup-1", "ns-1", "", api.RestorePhaseInProgress).Restore,
				NewRestore("foo", "bar", "backup-1", "ns-1", "", api.RestorePhaseCompleted).
					WithErrors(api.RestoreResult{
						Ark: []string{"no backup here"},
					}).
					Restore,
			},
		},
		{
			name:          "restorer throwing an error causes the restore to fail",
			restore:       NewRestore("foo", "bar", "backup-1", "ns-1", "", api.RestorePhaseNew).Restore,
			backup:        NewTestBackup().WithName("backup-1").Backup,
			restorerError: errors.New("blarg"),
			expectedErr:   false,
			expectedRestoreUpdates: []*api.Restore{
				NewRestore("foo", "bar", "backup-1", "ns-1", "", api.RestorePhaseInProgress).Restore,
				NewRestore("foo", "bar", "backup-1", "ns-1", "", api.RestorePhaseCompleted).
					WithErrors(api.RestoreResult{
						Namespaces: map[string][]string{
							"ns-1": {"blarg"},
						},
					}).
					Restore,
			},
			expectedRestorerCall: NewRestore("foo", "bar", "backup-1", "ns-1", "", api.RestorePhaseInProgress).Restore,
		},
		{
			name:        "valid restore gets executed",
			restore:     NewRestore("foo", "bar", "backup-1", "ns-1", "", api.RestorePhaseNew).Restore,
			backup:      NewTestBackup().WithName("backup-1").Backup,
			expectedErr: false,
			expectedRestoreUpdates: []*api.Restore{
				NewRestore("foo", "bar", "backup-1", "ns-1", "", api.RestorePhaseInProgress).Restore,
				NewRestore("foo", "bar", "backup-1", "ns-1", "", api.RestorePhaseCompleted).Restore,
			},
			expectedRestorerCall: NewRestore("foo", "bar", "backup-1", "ns-1", "", api.RestorePhaseInProgress).Restore,
		},
		{
			name:                  "valid restore with RestorePVs=true gets executed when allowRestoreSnapshots=true",
			restore:               NewRestore("foo", "bar", "backup-1", "ns-1", "", api.RestorePhaseNew).WithRestorePVs(true).Restore,
			backup:                NewTestBackup().WithName("backup-1").Backup,
			allowRestoreSnapshots: true,
			expectedErr:           false,
			expectedRestoreUpdates: []*api.Restore{
				NewRestore("foo", "bar", "backup-1", "ns-1", "", api.RestorePhaseInProgress).WithRestorePVs(true).Restore,
				NewRestore("foo", "bar", "backup-1", "ns-1", "", api.RestorePhaseCompleted).WithRestorePVs(true).Restore,
			},
			expectedRestorerCall: NewRestore("foo", "bar", "backup-1", "ns-1", "", api.RestorePhaseInProgress).WithRestorePVs(true).Restore,
		},
		{
			name:        "restore with RestorePVs=true fails validation when allowRestoreSnapshots=false",
			restore:     NewRestore("foo", "bar", "backup-1", "ns-1", "", api.RestorePhaseNew).WithRestorePVs(true).Restore,
			backup:      NewTestBackup().WithName("backup-1").Backup,
			expectedErr: false,
			expectedRestoreUpdates: []*api.Restore{
				NewRestore("foo", "bar", "backup-1", "ns-1", "", api.RestorePhaseFailedValidation).
					WithRestorePVs(true).
					WithValidationError("Server is not configured for PV snapshot restores").
					Restore,
			},
		},
		{
			name:        "restoration of nodes is not supported",
			restore:     NewRestore("foo", "bar", "backup-1", "ns-1", "nodes", api.RestorePhaseNew).Restore,
			backup:      NewTestBackup().WithName("backup-1").Backup,
			expectedErr: false,
			expectedRestoreUpdates: []*api.Restore{
				NewRestore("foo", "bar", "backup-1", "ns-1", "nodes", api.RestorePhaseFailedValidation).
					WithValidationError("nodes are a non-restorable resource").
					WithValidationError("Invalid included/excluded resource lists: excludes list cannot contain an item in the includes list: nodes").
					Restore,
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var (
				client          = fake.NewSimpleClientset()
				restorer        = &fakeRestorer{}
				sharedInformers = informers.NewSharedInformerFactory(client, 0)
				backupSvc       = &BackupService{}
				logger, _       = testlogger.NewNullLogger()
			)

			defer restorer.AssertExpectations(t)
			defer backupSvc.AssertExpectations(t)

			c := NewRestoreController(
				sharedInformers.Ark().V1().Restores(),
				client.ArkV1(),
				client.ArkV1(),
				restorer,
				backupSvc,
				"bucket",
				sharedInformers.Ark().V1().Backups(),
				test.allowRestoreSnapshots,
				logger,
			).(*restoreController)

			if test.restore != nil {
				sharedInformers.Ark().V1().Restores().Informer().GetStore().Add(test.restore)

				// this is necessary so the Update() call returns the appropriate object
				client.PrependReactor("update", "restores", func(action core.Action) (bool, runtime.Object, error) {
					obj := action.(core.UpdateAction).GetObject()
					// need to deep copy so we can test the backup state for each call to update
					copy, err := scheme.Scheme.DeepCopy(obj)
					if err != nil {
						return false, nil, err
					}
					ret := copy.(runtime.Object)
					return true, ret, nil
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

			err = c.processRestore(key)
			backupSvc.AssertExpectations(t)
			restorer.AssertExpectations(t)

			assert.Equal(t, test.expectedErr, err != nil, "got error %v", err)

			if test.expectedRestoreUpdates != nil {
				var expectedActions []core.Action

				for _, upd := range test.expectedRestoreUpdates {
					action := core.NewUpdateAction(
						api.SchemeGroupVersion.WithResource("restores"),
						upd.Namespace,
						upd)

					expectedActions = append(expectedActions, action)
				}

				assert.Equal(t, expectedActions, client.Actions())
			}

			if test.expectedRestorerCall == nil {
				assert.Empty(t, restorer.Calls)
				assert.Zero(t, restorer.calledWithArg)
			} else {
				assert.Equal(t, 1, len(restorer.Calls))

				// explicitly capturing the argument passed to Restore myself because
				// I want to validate the called arg as of the time of calling, but
				// the mock stores the pointer, which gets modified after
				assert.Equal(t, *test.expectedRestorerCall, restorer.calledWithArg)
			}
		})
	}
}

func NewRestore(ns, name, backup, includeNS, includeResource string, phase api.RestorePhase) *TestRestore {
	restore := NewTestRestore(ns, name, phase).WithBackup(backup)

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

func (r *fakeRestorer) Restore(restore *api.Restore, backup *api.Backup, backupReader io.Reader, logger io.Writer) (api.RestoreResult, api.RestoreResult) {
	res := r.Called(restore, backup, backupReader, logger)

	r.calledWithArg = *restore

	return res.Get(0).(api.RestoreResult), res.Get(1).(api.RestoreResult)
}
