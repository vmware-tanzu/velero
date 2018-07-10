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
	"testing"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/sets"
	core "k8s.io/client-go/testing"

	"github.com/heptio/ark/pkg/apis/ark/v1"
	"github.com/heptio/ark/pkg/generated/clientset/versioned/fake"
	"github.com/heptio/ark/pkg/util/stringslice"
	arktest "github.com/heptio/ark/pkg/util/test"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
)

func TestBackupSyncControllerRun(t *testing.T) {
	tests := []struct {
		name               string
		getAllBackupsError error
		cloudBackups       []*v1.Backup
		namespace          string
		existingBackups    sets.String
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
				arktest.NewTestBackup().WithNamespace("ns-1").WithName("backup-3").Backup,
			},
			namespace: "ns-1",
		},
		{
			name: "Finalizer gets removed on sync",
			cloudBackups: []*v1.Backup{
				arktest.NewTestBackup().WithNamespace("ns-1").WithFinalizers(gcFinalizer).Backup,
			},
			namespace: "ns-1",
		},
		{
			name: "Only target finalizer is removed",
			cloudBackups: []*v1.Backup{
				arktest.NewTestBackup().WithNamespace("ns-1").WithFinalizers(gcFinalizer, "blah").Backup,
			},
			namespace: "ns-1",
		},
		{
			name: "backups get created in Ark server's namespace",
			cloudBackups: []*v1.Backup{
				arktest.NewTestBackup().WithNamespace("ns-1").WithName("backup-1").Backup,
				arktest.NewTestBackup().WithNamespace("ns-2").WithName("backup-2").Backup,
			},
			namespace: "heptio-ark",
		},
		{
			name: "normal case with backups that already exist in Kubernetes",
			cloudBackups: []*v1.Backup{
				arktest.NewTestBackup().WithNamespace("ns-1").WithName("backup-1").Backup,
				arktest.NewTestBackup().WithNamespace("ns-1").WithName("backup-2").Backup,
				arktest.NewTestBackup().WithNamespace("ns-1").WithName("backup-3").Backup,
			},
			existingBackups: sets.NewString("backup-2", "backup-3"),
			namespace:       "ns-1",
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
				test.namespace,
				logger,
			).(*backupSyncController)

			bs.On("GetAllBackups", "bucket").Return(test.cloudBackups, test.getAllBackupsError)

			expectedActions := make([]core.Action, 0)

			client.PrependReactor("get", "backups", func(action core.Action) (bool, runtime.Object, error) {
				getAction := action.(core.GetAction)
				if test.existingBackups.Has(getAction.GetName()) {
					return true, nil, nil
				}
				// We return nil in place of the found backup object because
				// we exclusively check for the error and don't use the object
				// returned by the Get / Backups call.
				return true, nil, apierrors.NewNotFound(v1.SchemeGroupVersion.WithResource("backups").GroupResource(), getAction.GetName())
			})

			c.run()

			// we only expect creates for items within the target bucket
			for _, cloudBackup := range test.cloudBackups {
				// Verify that the run function stripped the GC finalizer
				assert.False(t, stringslice.Has(cloudBackup.Finalizers, gcFinalizer))
				assert.Equal(t, test.namespace, cloudBackup.Namespace)

				actionGet := core.NewGetAction(
					v1.SchemeGroupVersion.WithResource("backups"),
					test.namespace,
					cloudBackup.Name,
				)
				expectedActions = append(expectedActions, actionGet)

				if test.existingBackups.Has(cloudBackup.Name) {
					continue
				}
				actionCreate := core.NewCreateAction(
					v1.SchemeGroupVersion.WithResource("backups"),
					test.namespace,
					cloudBackup,
				)
				expectedActions = append(expectedActions, actionCreate)
			}

			assert.Equal(t, expectedActions, client.Actions())
			bs.AssertExpectations(t)
		})
	}
}
