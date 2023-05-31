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

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/types"
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

func TestOnlyNeededPhaseIsProcessed(t *testing.T) {
	for _, phase := range []api.BackupPhase{
		api.BackupPhaseNew,
		api.BackupPhaseFailedValidation,
		api.BackupPhaseInProgress,
		api.BackupPhaseWaitingForPluginOperations,
		api.BackupPhaseWaitingForPluginOperationsPartiallyFailed,
		api.BackupPhaseFinalizing,
		api.BackupPhaseFinalizingPartiallyFailed,
		api.BackupPhaseCompleted,
		api.BackupPhasePartiallyFailed,
		api.BackupPhaseFailed,
		api.BackupPhaseDeleting,
		api.BackupPhaseFailedPostBackupActions,
	} {
		t.Run(string(phase), func(t *testing.T) {
			r := postBackupActionsReconciler{
				logger:    logrus.StandardLogger(),
				client:    velerotest.NewFakeControllerRuntimeClient(t),
				logFormat: logging.FormatText,
			}
			backup := builder.ForBackup(api.DefaultNamespace, "backup-1").
				Phase(phase).Result()
			require.NoError(t, r.client.Create(ctx, backup))
			backupStore.On("GetPostBackupActions").Return(errors.New("unexpected error"))
			_, err := r.Reconcile(ctx, ctrl.Request{NamespacedName: types.NamespacedName{Namespace: backup.Namespace, Name: backup.Name}})
			require.Nil(t, err)
			backupAfter := api.Backup{}
			err = r.client.Get(ctx, types.NamespacedName{
				Namespace: backup.Namespace,
				Name:      backup.Name,
			}, &backupAfter)
			require.Equal(t, phase, backup.Status.Phase)
		})
	}
}

func TestPostBackupActionsReconcile(t *testing.T) {
	defaultBackupLocation := builder.ForBackupStorageLocation(api.DefaultNamespace, "loc-1").Result()
	tests := []struct {
		name                    string
		backup                  api.Backup
		expectedPhase           api.BackupPhase
		notFoundBackup          bool
		notFoundBackupLocation  bool
		getPostBackupActionFail bool
		postBackupActionFail    bool
	}{
		{
			name: "happy path",
			backup: *builder.ForBackup(api.DefaultNamespace, "backup-1").
				Phase(api.BackupPhaseWaitingForPostBackupActions).Result(),
			expectedPhase: api.BackupPhaseCompleted,
		},
		{
			name: "not found backup",
			backup: *builder.ForBackup(api.DefaultNamespace, "backup-2").
				Phase(api.BackupPhaseWaitingForPostBackupActions).Result(),
			notFoundBackup: true,
			expectedPhase:  api.BackupPhaseWaitingForPostBackupActions,
		},
		{
			name: "not found backup location",
			backup: *builder.ForBackup(api.DefaultNamespace, "backup-3").
				Phase(api.BackupPhaseWaitingForPostBackupActions).Result(),
			notFoundBackupLocation: true,
			expectedPhase:          api.BackupPhaseWaitingForPostBackupActions,
		},
		{
			name: "get post backup action fail",
			backup: *builder.ForBackup(api.DefaultNamespace, "backup-4").
				Phase(api.BackupPhaseWaitingForPostBackupActions).Result(),
			getPostBackupActionFail: true,
			expectedPhase:           api.BackupPhaseFailedPostBackupActions,
		},
		{
			name: "post backup action fail",
			backup: *builder.ForBackup(api.DefaultNamespace, "backup-5").
				Phase(api.BackupPhaseWaitingForPostBackupActions).Result(),
			expectedPhase:        api.BackupPhaseFailedPostBackupActions,
			postBackupActionFail: true,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			backupStore := persistencemocks.NewBackupStore(t)
			pluginManager := managermocks.NewManager(t)
			r := &postBackupActionsReconciler{
				logger:            logrus.StandardLogger(),
				client:            velerotest.NewFakeControllerRuntimeClient(t),
				logFormat:         logging.FormatText,
				metrics:           metrics.NewServerMetrics(),
				backupStoreGetter: NewFakeSingleObjectBackupStoreGetter(backupStore),
				newPluginManager: func(log logrus.FieldLogger) clientmgmt.Manager {
					return pluginManager
				},
			}

			if !tc.notFoundBackup {
				tc.backup.Spec.StorageLocation = defaultBackupLocation.Name
				require.NoError(t, r.client.Create(ctx, &tc.backup))
			}
			if !tc.notFoundBackupLocation {
				defaultBackupLocation.ResourceVersion = ""
				require.NoError(t, r.client.Create(ctx, defaultBackupLocation))
			}
			if tc.getPostBackupActionFail {
				pluginManager.On("GetPostBackupActions").Return(nil, errors.New("error getting post backup actions")).Maybe()
			} else if tc.postBackupActionFail {
				pluginManager.On("GetPostBackupActions").Return([]velero.PostBackupAction{&failPostBackupAction{}}, nil).Maybe()
			} else {
				pluginManager.On("GetPostBackupActions").Return(nil, nil).Maybe()
			}
			pluginManager.On("CleanupClients").Return(nil).Maybe()
			backupStore.On("PutPostBackupLog", mock.Anything, mock.Anything).Return().Maybe()
			backupStore.On("PutBackupMetadata", mock.Anything, mock.Anything).Return(nil).Maybe()

			_, err := r.Reconcile(ctx, ctrl.Request{NamespacedName: types.NamespacedName{Namespace: tc.backup.Namespace, Name: tc.backup.Name}})
			require.Nil(t, err)

			if !tc.notFoundBackup {
				backupAfter := api.Backup{}
				require.NoError(t, r.client.Get(ctx, types.NamespacedName{
					Namespace: tc.backup.Namespace,
					Name:      tc.backup.Name,
				}, &backupAfter))

				require.Equal(t, tc.expectedPhase, backupAfter.Status.Phase)
			}
		})
	}
}

type failPostBackupAction struct {
}

func (a *failPostBackupAction) Execute(backup *api.Backup) error {
	return errors.New("some error in post backup action")
}
