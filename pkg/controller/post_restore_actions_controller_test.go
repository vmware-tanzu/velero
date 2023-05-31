/*
Copyright The Velero Contributors.

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

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/types"
	testclocks "k8s.io/utils/clock/testing"
	ctrl "sigs.k8s.io/controller-runtime"

	api "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	"github.com/vmware-tanzu/velero/pkg/builder"
	"github.com/vmware-tanzu/velero/pkg/metrics"
	persistencemocks "github.com/vmware-tanzu/velero/pkg/persistence/mocks"
	"github.com/vmware-tanzu/velero/pkg/plugin/clientmgmt"
	managermocks "github.com/vmware-tanzu/velero/pkg/plugin/mocks"
	"github.com/vmware-tanzu/velero/pkg/plugin/velero"
	velerotest "github.com/vmware-tanzu/velero/pkg/test"
	"github.com/vmware-tanzu/velero/pkg/util/logging"
)

func TestOnlyNeededRestorePhaseIsProcessed(t *testing.T) {
	for _, phase := range []api.RestorePhase{
		api.RestorePhaseNew,
		api.RestorePhaseFailedValidation,
		api.RestorePhaseInProgress,
		api.RestorePhaseWaitingForPluginOperations,
		api.RestorePhaseWaitingForPluginOperationsPartiallyFailed,
		api.RestorePhaseCompleted,
		api.RestorePhasePartiallyFailed,
		api.RestorePhaseFailed,
		api.RestorePhaseFailedPostRestoreActions,
	} {
		t.Run(string(phase), func(t *testing.T) {
			r := postRestoreActionsReconciler{
				logger:    logrus.StandardLogger(),
				client:    velerotest.NewFakeControllerRuntimeClient(t),
				logFormat: logging.FormatText,
			}
			restore := builder.ForRestore(api.DefaultNamespace, "restore-1").
				Phase(phase).Result()
			require.NoError(t, r.client.Create(ctx, restore))
			backupStore.On("GetPostRestoreActions").Return(errors.New("unexpected error"))
			_, err := r.Reconcile(ctx, ctrl.Request{NamespacedName: types.NamespacedName{Namespace: restore.Namespace, Name: restore.Name}})
			require.Nil(t, err)
			restoreAfter := api.Restore{}
			err = r.client.Get(ctx, types.NamespacedName{
				Namespace: restore.Namespace,
				Name:      restore.Name,
			}, &restoreAfter)
			require.Equal(t, phase, restore.Status.Phase)
		})
	}
}

func TestPostRestoreActionsReconcile(t *testing.T) {
	defaultBackupLocation := builder.ForBackupStorageLocation(api.DefaultNamespace, "loc-1").Result()
	defaultBackup := builder.ForBackup(api.DefaultNamespace, "backup-1").Result()
	defaultBackup.Spec.StorageLocation = defaultBackupLocation.Name
	tests := []struct {
		name                     string
		restore                  api.Restore
		expectedPhase            api.RestorePhase
		notFoundRestore          bool
		notFoundBackup           bool
		notFoundBackupLocation   bool
		getPostRestoreActionFail bool
		postRestoreActionFail    bool
	}{
		{
			name: "happy path",
			restore: *builder.ForRestore(api.DefaultNamespace, "restore-1").
				Phase(api.RestorePhaseWaitingForPostRestoreActions).Result(),
			expectedPhase: api.RestorePhaseCompleted,
		},
		{
			name: "not found restore",
			restore: *builder.ForRestore(api.DefaultNamespace, "restore-2").
				Phase(api.RestorePhaseWaitingForPostRestoreActions).Result(),
			notFoundRestore: true,
			expectedPhase:   api.RestorePhaseWaitingForPostRestoreActions,
		},
		{
			name: "not found backup",
			restore: *builder.ForRestore(api.DefaultNamespace, "restore-3").
				Phase(api.RestorePhaseWaitingForPostRestoreActions).Result(),
			notFoundBackup: true,
			expectedPhase:  api.RestorePhaseWaitingForPostRestoreActions,
		},
		{
			name: "not found backup location",
			restore: *builder.ForRestore(api.DefaultNamespace, "restore-4").
				Phase(api.RestorePhaseWaitingForPostRestoreActions).Result(),
			notFoundBackupLocation: true,
			expectedPhase:          api.RestorePhaseWaitingForPostRestoreActions,
		},
		{
			name: "get post restore action fail",
			restore: *builder.ForRestore(api.DefaultNamespace, "restore-5").
				Phase(api.RestorePhaseWaitingForPostRestoreActions).Result(),
			getPostRestoreActionFail: true,
			expectedPhase:            api.RestorePhaseFailedPostRestoreActions,
		},
		{
			name: "post restore action fail",
			restore: *builder.ForRestore(api.DefaultNamespace, "restore-6").
				Phase(api.RestorePhaseWaitingForPostRestoreActions).Result(),
			expectedPhase:         api.RestorePhaseFailedPostRestoreActions,
			postRestoreActionFail: true,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			backupStore := persistencemocks.NewBackupStore(t)
			pluginManager := managermocks.NewManager(t)
			r := &postRestoreActionsReconciler{
				logger:            logrus.StandardLogger(),
				namespace:         api.DefaultNamespace,
				client:            velerotest.NewFakeControllerRuntimeClient(t),
				clock:             testclocks.NewFakeClock(time.Now()),
				logFormat:         logging.FormatText,
				metrics:           metrics.NewServerMetrics(),
				backupStoreGetter: NewFakeSingleObjectBackupStoreGetter(backupStore),
				newPluginManager: func(log logrus.FieldLogger) clientmgmt.Manager {
					return pluginManager
				},
			}

			if !tc.notFoundRestore {
				tc.restore.Spec.BackupName = defaultBackup.Name
				require.NoError(t, r.client.Create(ctx, &tc.restore))
			}
			if !tc.notFoundBackup {
				defaultBackup.ResourceVersion = ""
				require.NoError(t, r.client.Create(ctx, defaultBackup))
			}
			if !tc.notFoundBackupLocation {
				defaultBackupLocation.ResourceVersion = ""
				require.NoError(t, r.client.Create(ctx, defaultBackupLocation))
			}
			if tc.getPostRestoreActionFail {
				pluginManager.On("GetPostRestoreActions").Return(nil, errors.New("error getting post restore actions")).Maybe()
			} else if tc.postRestoreActionFail {
				pluginManager.On("GetPostRestoreActions").Return([]velero.PostRestoreAction{&failPostRestoreAction{}}, nil).Maybe()
			} else {
				pluginManager.On("GetPostRestoreActions").Return(nil, nil).Maybe()
			}
			pluginManager.On("CleanupClients").Return(nil).Maybe()
			backupStore.On("PutPostRestoreLog", mock.Anything, mock.Anything, mock.Anything).Return().Maybe()

			_, _ = r.Reconcile(ctx, ctrl.Request{NamespacedName: types.NamespacedName{Namespace: tc.restore.Namespace, Name: tc.restore.Name}})

			if !tc.notFoundRestore {
				restoreAfter := api.Restore{}
				require.NoError(t, r.client.Get(ctx, types.NamespacedName{
					Namespace: tc.restore.Namespace,
					Name:      tc.restore.Name,
				}, &restoreAfter))

				require.Equal(t, tc.expectedPhase, restoreAfter.Status.Phase)
			}
		})
	}
}

type failPostRestoreAction struct {
}

func (a *failPostRestoreAction) Execute(restore *api.Restore) error {
	return errors.New("some error in post restore action")
}
