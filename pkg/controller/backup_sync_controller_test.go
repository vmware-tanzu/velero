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
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	core "k8s.io/client-go/testing"

	"github.com/heptio/ark/pkg/apis/ark/v1"
	"github.com/heptio/ark/pkg/generated/clientset/versioned/fake"
	"github.com/heptio/ark/pkg/util/stringslice"
	arktest "github.com/heptio/ark/pkg/util/test"
)

func TestBackupSyncControllerRun(t *testing.T) {
	tests := []struct {
		name               string
		getAllBackupsError error
		cloudBackups       []*v1.Backup
	}{
		{
			name: "no cloud backups",
		},
		{
			name:               "backup service returns error on GetAllBackups",
			getAllBackupsError: errors.New("getAllBackups"),
		},
		{
			name: "normal case",
			cloudBackups: []*v1.Backup{
				arktest.NewTestBackup().WithNamespace("ns-1").WithName("backup-1").Backup,
				arktest.NewTestBackup().WithNamespace("ns-1").WithName("backup-2").Backup,
				arktest.NewTestBackup().WithNamespace("ns-2").WithName("backup-3").Backup,
			},
		},
		{
			name: "Finalizer gets removed on sync",
			cloudBackups: []*v1.Backup{
				arktest.NewTestBackup().WithNamespace("ns-1").WithFinalizers(gcFinalizer).Backup,
			},
		},
		{
			name: "Only target finalizer is removed",
			cloudBackups: []*v1.Backup{
				arktest.NewTestBackup().WithNamespace("ns-1").WithFinalizers(gcFinalizer, "blah").Backup,
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var (
				bs     = &arktest.BackupService{}
				client = fake.NewSimpleClientset()
				logger = arktest.NewLogger()
			)

			c := NewBackupSyncController(
				client.ArkV1(),
				bs,
				"bucket",
				time.Duration(0),
				logger,
			).(*backupSyncController)

			bs.On("GetAllBackups", "bucket").Return(test.cloudBackups, test.getAllBackupsError)

			c.run()

			expectedActions := make([]core.Action, 0)

			// we only expect creates for items within the target bucket
			for _, cloudBackup := range test.cloudBackups {
				// Verify that the run function stripped the GC finalizer
				assert.False(t, stringslice.Has(cloudBackup.Finalizers, gcFinalizer))
				action := core.NewCreateAction(
					v1.SchemeGroupVersion.WithResource("backups"),
					cloudBackup.Namespace,
					cloudBackup,
				)

				expectedActions = append(expectedActions, action)
			}

			assert.Equal(t, expectedActions, client.Actions())
			bs.AssertExpectations(t)
		})
	}
}
