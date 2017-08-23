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
	"errors"
	"io"
	"testing"

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

func TestProcessRestore(t *testing.T) {
	tests := []struct {
		name                   string
		restoreKey             string
		restore                *api.Restore
		backup                 *api.Backup
		restorerError          error
		allowRestoreSnapshots  bool
		expectedErr            bool
		expectedRestoreUpdates []*api.Restore
		expectedRestorerCall   *api.Restore
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
			name:        "new restore with empty backup name fails validation",
			restore:     NewTestRestore("foo", "bar", api.RestorePhaseNew).WithRestorableNamespace("ns-1").Restore,
			expectedErr: false,
			expectedRestoreUpdates: []*api.Restore{
				NewTestRestore("foo", "bar", api.RestorePhaseFailedValidation).
					WithRestorableNamespace("ns-1").
					WithValidationError("BackupName must be non-empty and correspond to the name of a backup in object storage.").Restore,
			},
		},
		{
			name:        "restore with non-existent backup name fails",
			restore:     NewTestRestore("foo", "bar", api.RestorePhaseNew).WithBackup("backup-1").WithRestorableNamespace("ns-1").Restore,
			expectedErr: false,
			expectedRestoreUpdates: []*api.Restore{
				NewTestRestore("foo", "bar", api.RestorePhaseInProgress).WithBackup("backup-1").WithRestorableNamespace("ns-1").Restore,
				NewTestRestore("foo", "bar", api.RestorePhaseCompleted).
					WithBackup("backup-1").
					WithRestorableNamespace("ns-1").
					WithErrors(api.RestoreResult{
						Cluster: []string{"backup.ark.heptio.com \"backup-1\" not found"},
					}).
					Restore,
			},
		},
		{
			name:          "restorer throwing an error causes the restore to fail",
			restore:       NewTestRestore("foo", "bar", api.RestorePhaseNew).WithBackup("backup-1").WithRestorableNamespace("ns-1").Restore,
			backup:        NewTestBackup().WithName("backup-1").Backup,
			restorerError: errors.New("blarg"),
			expectedErr:   false,
			expectedRestoreUpdates: []*api.Restore{
				NewTestRestore("foo", "bar", api.RestorePhaseInProgress).WithBackup("backup-1").WithRestorableNamespace("ns-1").Restore,
				NewTestRestore("foo", "bar", api.RestorePhaseCompleted).
					WithBackup("backup-1").
					WithRestorableNamespace("ns-1").
					WithErrors(api.RestoreResult{
						Namespaces: map[string][]string{
							"ns-1": {"blarg"},
						},
					}).Restore,
			},
			expectedRestorerCall: NewTestRestore("foo", "bar", api.RestorePhaseInProgress).WithBackup("backup-1").WithRestorableNamespace("ns-1").Restore,
		},
		{
			name:        "valid restore gets executed",
			restore:     NewTestRestore("foo", "bar", api.RestorePhaseNew).WithBackup("backup-1").WithRestorableNamespace("ns-1").Restore,
			backup:      NewTestBackup().WithName("backup-1").Backup,
			expectedErr: false,
			expectedRestoreUpdates: []*api.Restore{
				NewTestRestore("foo", "bar", api.RestorePhaseInProgress).WithBackup("backup-1").WithRestorableNamespace("ns-1").Restore,
				NewTestRestore("foo", "bar", api.RestorePhaseCompleted).WithBackup("backup-1").WithRestorableNamespace("ns-1").Restore,
			},
			expectedRestorerCall: NewTestRestore("foo", "bar", api.RestorePhaseInProgress).WithBackup("backup-1").WithRestorableNamespace("ns-1").Restore,
		},
		{
			name:        "restore with no restorable namespaces gets defaulted to *",
			restore:     NewTestRestore("foo", "bar", api.RestorePhaseNew).WithBackup("backup-1").Restore,
			backup:      NewTestBackup().WithName("backup-1").Backup,
			expectedErr: false,
			expectedRestoreUpdates: []*api.Restore{
				NewTestRestore("foo", "bar", api.RestorePhaseInProgress).WithBackup("backup-1").WithRestorableNamespace("*").Restore,
				NewTestRestore("foo", "bar", api.RestorePhaseCompleted).WithBackup("backup-1").WithRestorableNamespace("*").Restore,
			},
			expectedRestorerCall: NewTestRestore("foo", "bar", api.RestorePhaseInProgress).WithBackup("backup-1").WithRestorableNamespace("*").Restore,
		},
		{
			name:                  "valid restore with RestorePVs=true gets executed when allowRestoreSnapshots=true",
			restore:               NewTestRestore("foo", "bar", api.RestorePhaseNew).WithBackup("backup-1").WithRestorableNamespace("ns-1").WithRestorePVs(true).Restore,
			backup:                NewTestBackup().WithName("backup-1").Backup,
			allowRestoreSnapshots: true,
			expectedErr:           false,
			expectedRestoreUpdates: []*api.Restore{
				NewTestRestore("foo", "bar", api.RestorePhaseInProgress).WithBackup("backup-1").WithRestorableNamespace("ns-1").WithRestorePVs(true).Restore,
				NewTestRestore("foo", "bar", api.RestorePhaseCompleted).WithBackup("backup-1").WithRestorableNamespace("ns-1").WithRestorePVs(true).Restore,
			},
			expectedRestorerCall: NewTestRestore("foo", "bar", api.RestorePhaseInProgress).WithBackup("backup-1").WithRestorableNamespace("ns-1").WithRestorePVs(true).Restore,
		},
		{
			name:        "restore with RestorePVs=true fails validation when allowRestoreSnapshots=false",
			restore:     NewTestRestore("foo", "bar", api.RestorePhaseNew).WithBackup("backup-1").WithRestorableNamespace("ns-1").WithRestorePVs(true).Restore,
			backup:      NewTestBackup().WithName("backup-1").Backup,
			expectedErr: false,
			expectedRestoreUpdates: []*api.Restore{
				NewTestRestore("foo", "bar", api.RestorePhaseFailedValidation).WithBackup("backup-1").WithRestorableNamespace("ns-1").WithRestorePVs(true).
					WithValidationError("Server is not configured for PV snapshot restores").Restore,
			},
		},
	}

	// flag.Set("logtostderr", "true")
	// flag.Set("v", "4")

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {

			var (
				client          = fake.NewSimpleClientset()
				restorer        = &fakeRestorer{}
				sharedInformers = informers.NewSharedInformerFactory(client, 0)
				backupSvc       = &fakeBackupService{}
			)

			c := NewRestoreController(
				sharedInformers.Ark().V1().Restores(),
				client.ArkV1(),
				client.ArkV1(),
				restorer,
				backupSvc,
				"bucket",
				sharedInformers.Ark().V1().Backups(),
				test.allowRestoreSnapshots,
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
			restorer.On("Restore", mock.Anything, mock.Anything, mock.Anything).Return(warnings, errors)

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

			err = c.processRestore(key)

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

type fakeRestorer struct {
	mock.Mock
	calledWithArg api.Restore
}

func (r *fakeRestorer) Restore(restore *api.Restore, backup *api.Backup, backupReader io.Reader) (api.RestoreResult, api.RestoreResult) {
	res := r.Called(restore, backup, backupReader)

	r.calledWithArg = *restore

	return res.Get(0).(api.RestoreResult), res.Get(1).(api.RestoreResult)
}
