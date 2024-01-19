/*
Copyright the Velero contributors.

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
	"context"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	testclocks "k8s.io/utils/clock/testing"
	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/stretchr/testify/mock"

	velerov1api "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	"github.com/vmware-tanzu/velero/pkg/builder"
	"github.com/vmware-tanzu/velero/pkg/metrics"
	persistencemocks "github.com/vmware-tanzu/velero/pkg/persistence/mocks"
	"github.com/vmware-tanzu/velero/pkg/plugin/clientmgmt"
	pluginmocks "github.com/vmware-tanzu/velero/pkg/plugin/mocks"
	velerotest "github.com/vmware-tanzu/velero/pkg/test"
	"github.com/vmware-tanzu/velero/pkg/util/results"
)

func TestRestoreFinalizerReconcile(t *testing.T) {
	defaultStorageLocation := builder.ForBackupStorageLocation("velero", "default").Provider("myCloud").Bucket("bucket").Result()
	now, err := time.Parse(time.RFC1123Z, time.RFC1123Z)
	require.NoError(t, err)
	now = now.Local()
	timestamp := metav1.NewTime(now)
	assert.NotNil(t, timestamp)

	rfrTests := []struct {
		name                  string
		restore               *velerov1api.Restore
		backup                *velerov1api.Backup
		location              *velerov1api.BackupStorageLocation
		expectError           bool
		expectPhase           velerov1api.RestorePhase
		expectWarningsCnt     int
		expectErrsCnt         int
		statusCompare         bool
		expectedCompletedTime *metav1.Time
	}{
		{
			name:          "Restore is not awaiting finalization, skip",
			restore:       builder.ForRestore(velerov1api.DefaultNamespace, "restore-1").Phase(velerov1api.RestorePhaseInProgress).Result(),
			expectError:   false,
			expectPhase:   velerov1api.RestorePhaseInProgress,
			statusCompare: false,
		},
		{
			name:                  "Upon completion of all finalization tasks in the 'FinalizingPartiallyFailed' phase, the restore process transit to the 'PartiallyFailed' phase.",
			restore:               builder.ForRestore(velerov1api.DefaultNamespace, "restore-1").Phase(velerov1api.RestorePhaseFinalizingPartiallyFailed).Backup("backup-1").Result(),
			backup:                defaultBackup().StorageLocation("default").Result(),
			location:              defaultStorageLocation,
			expectError:           false,
			expectPhase:           velerov1api.RestorePhasePartiallyFailed,
			statusCompare:         true,
			expectedCompletedTime: &timestamp,
			expectWarningsCnt:     0,
			expectErrsCnt:         0,
		},
		{
			name:                  "Upon completion of all finalization tasks in the 'Finalizing' phase, the restore process transit to the 'Completed' phase.",
			restore:               builder.ForRestore(velerov1api.DefaultNamespace, "restore-1").Phase(velerov1api.RestorePhaseFinalizing).Backup("backup-1").Result(),
			backup:                defaultBackup().StorageLocation("default").Result(),
			location:              defaultStorageLocation,
			expectError:           false,
			expectPhase:           velerov1api.RestorePhaseCompleted,
			statusCompare:         true,
			expectedCompletedTime: &timestamp,
			expectWarningsCnt:     0,
			expectErrsCnt:         0,
		},
		{
			name:        "Backup not exist",
			restore:     builder.ForRestore(velerov1api.DefaultNamespace, "restore-1").Phase(velerov1api.RestorePhaseFinalizing).Backup("backup-2").Result(),
			expectError: false,
		},
		{
			name:          "Restore not exist",
			restore:       builder.ForRestore("unknown", "restore-1").Phase(velerov1api.RestorePhaseFinalizing).Result(),
			expectError:   false,
			statusCompare: false,
		},
	}

	for _, test := range rfrTests {
		t.Run(test.name, func(t *testing.T) {
			if test.restore == nil {
				return
			}

			var (
				fakeClient    = velerotest.NewFakeControllerRuntimeClientBuilder(t).Build()
				logger        = velerotest.NewLogger()
				pluginManager = &pluginmocks.Manager{}
				backupStore   = &persistencemocks.BackupStore{}
			)

			defer func() {
				// reset defaultStorageLocation resourceVersion
				defaultStorageLocation.ObjectMeta.ResourceVersion = ""
			}()

			r := NewRestoreFinalizerReconciler(
				logger,
				velerov1api.DefaultNamespace,
				fakeClient,
				func(logrus.FieldLogger) clientmgmt.Manager { return pluginManager },
				NewFakeSingleObjectBackupStoreGetter(backupStore),
				metrics.NewServerMetrics(),
			)
			r.clock = testclocks.NewFakeClock(now)

			if test.restore != nil && test.restore.Namespace == velerov1api.DefaultNamespace {
				require.NoError(t, r.Client.Create(context.Background(), test.restore))
			}
			if test.backup != nil {
				assert.NoError(t, r.Client.Create(context.Background(), test.backup))
			}
			if test.location != nil {
				require.NoError(t, r.Client.Create(context.Background(), test.location))
			}

			if test.restore != nil {
				pluginManager.On("GetRestoreItemActionsV2").Return(nil, nil)
				pluginManager.On("CleanupClients")
			}

			_, err = r.Reconcile(context.Background(), ctrl.Request{NamespacedName: types.NamespacedName{
				Namespace: test.restore.Namespace,
				Name:      test.restore.Name,
			}})

			assert.Equal(t, test.expectError, err != nil)
			if test.expectError {
				return
			}

			if test.statusCompare {
				restoreAfter := velerov1api.Restore{}
				err = fakeClient.Get(context.TODO(), types.NamespacedName{
					Namespace: test.restore.Namespace,
					Name:      test.restore.Name,
				}, &restoreAfter)

				require.NoError(t, err)

				assert.Equal(t, test.expectPhase, restoreAfter.Status.Phase)
				assert.Equal(t, test.expectErrsCnt, restoreAfter.Status.Errors)
				assert.Equal(t, test.expectWarningsCnt, restoreAfter.Status.Warnings)
				require.True(t, test.expectedCompletedTime.Equal(restoreAfter.Status.CompletionTimestamp))
			}
		})
	}

}

func TestUpdateResult(t *testing.T) {
	var (
		fakeClient    = velerotest.NewFakeControllerRuntimeClientBuilder(t).Build()
		logger        = velerotest.NewLogger()
		pluginManager = &pluginmocks.Manager{}
		backupStore   = &persistencemocks.BackupStore{}
	)

	r := NewRestoreFinalizerReconciler(
		logger,
		velerov1api.DefaultNamespace,
		fakeClient,
		func(logrus.FieldLogger) clientmgmt.Manager { return pluginManager },
		NewFakeSingleObjectBackupStoreGetter(backupStore),
		metrics.NewServerMetrics(),
	)
	restore := builder.ForRestore(velerov1api.DefaultNamespace, "restore-1").Result()
	res := map[string]results.Result{"warnings": {}, "errors": {}}

	backupStore.On("GetRestoreResults", restore.Name).Return(res, nil)
	backupStore.On("PutRestoreResults", mock.Anything, mock.Anything, mock.Anything).Return(nil)

	err := r.updateResults(backupStore, restore, &results.Result{}, &results.Result{})
	require.NoError(t, err)
}
