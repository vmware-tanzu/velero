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
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	testclocks "k8s.io/utils/clock/testing"
	ctrl "sigs.k8s.io/controller-runtime"
	kbclient "sigs.k8s.io/controller-runtime/pkg/client"

	velerov1api "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	"github.com/vmware-tanzu/velero/pkg/builder"
	"github.com/vmware-tanzu/velero/pkg/itemoperation"
	"github.com/vmware-tanzu/velero/pkg/kuberesource"
	"github.com/vmware-tanzu/velero/pkg/metrics"
	persistencemocks "github.com/vmware-tanzu/velero/pkg/persistence/mocks"
	"github.com/vmware-tanzu/velero/pkg/plugin/clientmgmt"
	pluginmocks "github.com/vmware-tanzu/velero/pkg/plugin/mocks"
	"github.com/vmware-tanzu/velero/pkg/plugin/velero"
	biav2mocks "github.com/vmware-tanzu/velero/pkg/plugin/velero/mocks/backupitemaction/v2"
	velerotest "github.com/vmware-tanzu/velero/pkg/test"
)

var (
	pluginManager = &pluginmocks.Manager{}
	backupStore   = &persistencemocks.BackupStore{}
	bia           = &biav2mocks.BackupItemAction{}
)

func mockAsyncBackupOperationsReconciler(fakeClient kbclient.Client, fakeClock *testclocks.FakeClock, freq time.Duration) (*asyncBackupOperationsReconciler, *BackupItemOperationsMap) {
	abor, biaMap := NewAsyncBackupOperationsReconciler(
		logrus.StandardLogger(),
		fakeClient,
		freq,
		func(logrus.FieldLogger) clientmgmt.Manager { return pluginManager },
		NewFakeSingleObjectBackupStoreGetter(backupStore),
		metrics.NewServerMetrics(),
	)
	abor.clock = fakeClock
	return abor, biaMap
}

func TestAsyncBackupOperationsReconcile(t *testing.T) {
	fakeClock := testclocks.NewFakeClock(time.Now())
	metav1Now := metav1.NewTime(fakeClock.Now())

	defaultBackupLocation := builder.ForBackupStorageLocation(velerov1api.DefaultNamespace, "default").Result()

	tests := []struct {
		name              string
		backup            *velerov1api.Backup
		backupOperations  []*itemoperation.BackupOperation
		backupLocation    *velerov1api.BackupStorageLocation
		operationComplete bool
		operationErr      string
		expectError       bool
		expectPhase       velerov1api.BackupPhase
	}{
		{
			name: "WaitingForPluginOperations backup with completed operations is FinalizingAfterPluginOperations",
			backup: builder.ForBackup(velerov1api.DefaultNamespace, "backup-1").
				StorageLocation("default").
				ItemOperationTimeout(60 * time.Minute).
				ObjectMeta(builder.WithUID("foo")).
				Phase(velerov1api.BackupPhaseWaitingForPluginOperations).Result(),
			backupLocation:    defaultBackupLocation,
			operationComplete: true,
			expectPhase:       velerov1api.BackupPhaseFinalizingAfterPluginOperations,
			backupOperations: []*itemoperation.BackupOperation{
				{
					Spec: itemoperation.BackupOperationSpec{
						BackupName:       "backup-1",
						BackupUID:        "foo",
						BackupItemAction: "foo",
						ResourceIdentifier: velero.ResourceIdentifier{
							GroupResource: kuberesource.Pods,
							Namespace:     "ns-1",
							Name:          "pod-1",
						},
						OperationID: "operation-1",
					},
					Status: itemoperation.OperationStatus{
						Phase:   itemoperation.OperationPhaseInProgress,
						Created: &metav1Now,
					},
				},
			},
		},
		{
			name: "WaitingForPluginOperations backup with incomplete operations is still incomplete",
			backup: builder.ForBackup(velerov1api.DefaultNamespace, "backup-2").
				StorageLocation("default").
				ItemOperationTimeout(60 * time.Minute).
				ObjectMeta(builder.WithUID("foo")).
				Phase(velerov1api.BackupPhaseWaitingForPluginOperations).Result(),
			backupLocation:    defaultBackupLocation,
			operationComplete: false,
			expectPhase:       velerov1api.BackupPhaseWaitingForPluginOperations,
			backupOperations: []*itemoperation.BackupOperation{
				{
					Spec: itemoperation.BackupOperationSpec{
						BackupName:       "backup-2",
						BackupUID:        "foo-2",
						BackupItemAction: "foo-2",
						ResourceIdentifier: velero.ResourceIdentifier{
							GroupResource: kuberesource.Pods,
							Namespace:     "ns-1",
							Name:          "pod-1",
						},
						OperationID: "operation-2",
					},
					Status: itemoperation.OperationStatus{
						Phase:   itemoperation.OperationPhaseInProgress,
						Created: &metav1Now,
					},
				},
			},
		},
		{
			name: "WaitingForPluginOperations backup with completed failed operations is FinalizingAfterPluginOperationsPartiallyFailed",
			backup: builder.ForBackup(velerov1api.DefaultNamespace, "backup-3").
				StorageLocation("default").
				ItemOperationTimeout(60 * time.Minute).
				ObjectMeta(builder.WithUID("foo")).
				Phase(velerov1api.BackupPhaseWaitingForPluginOperations).Result(),
			backupLocation:    defaultBackupLocation,
			operationComplete: true,
			operationErr:      "failed",
			expectPhase:       velerov1api.BackupPhaseFinalizingAfterPluginOperationsPartiallyFailed,
			backupOperations: []*itemoperation.BackupOperation{
				{
					Spec: itemoperation.BackupOperationSpec{
						BackupName:       "backup-3",
						BackupUID:        "foo-3",
						BackupItemAction: "foo-3",
						ResourceIdentifier: velero.ResourceIdentifier{
							GroupResource: kuberesource.Pods,
							Namespace:     "ns-1",
							Name:          "pod-1",
						},
						OperationID: "operation-3",
					},
					Status: itemoperation.OperationStatus{
						Phase:   itemoperation.OperationPhaseInProgress,
						Created: &metav1Now,
					},
				},
			},
		},
		{
			name: "WaitingForPluginOperationsPartiallyFailed backup with completed operations is FinalizingAfterPluginOperationsPartiallyFailed",
			backup: builder.ForBackup(velerov1api.DefaultNamespace, "backup-1").
				StorageLocation("default").
				ItemOperationTimeout(60 * time.Minute).
				ObjectMeta(builder.WithUID("foo")).
				Phase(velerov1api.BackupPhaseWaitingForPluginOperationsPartiallyFailed).Result(),
			backupLocation:    defaultBackupLocation,
			operationComplete: true,
			expectPhase:       velerov1api.BackupPhaseFinalizingAfterPluginOperationsPartiallyFailed,
			backupOperations: []*itemoperation.BackupOperation{
				{
					Spec: itemoperation.BackupOperationSpec{
						BackupName:       "backup-4",
						BackupUID:        "foo-4",
						BackupItemAction: "foo-4",
						ResourceIdentifier: velero.ResourceIdentifier{
							GroupResource: kuberesource.Pods,
							Namespace:     "ns-1",
							Name:          "pod-1",
						},
						OperationID: "operation-4",
					},
					Status: itemoperation.OperationStatus{
						Phase:   itemoperation.OperationPhaseInProgress,
						Created: &metav1Now,
					},
				},
			},
		},
		{
			name: "WaitingForPluginOperationsPartiallyFailed backup with incomplete operations is still incomplete",
			backup: builder.ForBackup(velerov1api.DefaultNamespace, "backup-2").
				StorageLocation("default").
				ItemOperationTimeout(60 * time.Minute).
				ObjectMeta(builder.WithUID("foo")).
				Phase(velerov1api.BackupPhaseWaitingForPluginOperationsPartiallyFailed).Result(),
			backupLocation:    defaultBackupLocation,
			operationComplete: false,
			expectPhase:       velerov1api.BackupPhaseWaitingForPluginOperationsPartiallyFailed,
			backupOperations: []*itemoperation.BackupOperation{
				{
					Spec: itemoperation.BackupOperationSpec{
						BackupName:       "backup-5",
						BackupUID:        "foo-5",
						BackupItemAction: "foo-5",
						ResourceIdentifier: velero.ResourceIdentifier{
							GroupResource: kuberesource.Pods,
							Namespace:     "ns-1",
							Name:          "pod-1",
						},
						OperationID: "operation-5",
					},
					Status: itemoperation.OperationStatus{
						Phase:   itemoperation.OperationPhaseInProgress,
						Created: &metav1Now,
					},
				},
			},
		},
		{
			name: "WaitingForPluginOperationsPartiallyFailed backup with completed failed operations is FinalizingAfterPluginOperationsPartiallyFailed",
			backup: builder.ForBackup(velerov1api.DefaultNamespace, "backup-3").
				StorageLocation("default").
				ItemOperationTimeout(60 * time.Minute).
				ObjectMeta(builder.WithUID("foo")).
				Phase(velerov1api.BackupPhaseWaitingForPluginOperationsPartiallyFailed).Result(),
			backupLocation:    defaultBackupLocation,
			operationComplete: true,
			operationErr:      "failed",
			expectPhase:       velerov1api.BackupPhaseFinalizingAfterPluginOperationsPartiallyFailed,
			backupOperations: []*itemoperation.BackupOperation{
				{
					Spec: itemoperation.BackupOperationSpec{
						BackupName:       "backup-6",
						BackupUID:        "foo-6",
						BackupItemAction: "foo-6",
						ResourceIdentifier: velero.ResourceIdentifier{
							GroupResource: kuberesource.Pods,
							Namespace:     "ns-1",
							Name:          "pod-1",
						},
						OperationID: "operation-6",
					},
					Status: itemoperation.OperationStatus{
						Phase:   itemoperation.OperationPhaseInProgress,
						Created: &metav1Now,
					},
				},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if test.backup == nil {
				return
			}

			initObjs := []runtime.Object{}
			initObjs = append(initObjs, test.backup)

			if test.backupLocation != nil {
				initObjs = append(initObjs, test.backupLocation)
			}

			fakeClient := velerotest.NewFakeControllerRuntimeClient(t, initObjs...)
			reconciler, _ := mockAsyncBackupOperationsReconciler(fakeClient, fakeClock, defaultAsyncBackupOperationsFrequency)
			pluginManager.On("CleanupClients").Return(nil)
			backupStore.On("GetBackupItemOperations", test.backup.Name).Return(test.backupOperations, nil)
			backupStore.On("PutBackupItemOperations", mock.Anything, mock.Anything).Return(nil)
			backupStore.On("PutBackupMetadata", mock.Anything, mock.Anything).Return(nil)
			for _, operation := range test.backupOperations {
				bia.On("Progress", operation.Spec.OperationID, mock.Anything).
					Return(velero.OperationProgress{
						Completed: test.operationComplete,
						Err:       test.operationErr,
					}, nil)
				pluginManager.On("GetBackupItemActionV2", operation.Spec.BackupItemAction).Return(bia, nil)
			}
			_, err := reconciler.Reconcile(context.TODO(), ctrl.Request{NamespacedName: types.NamespacedName{Namespace: test.backup.Namespace, Name: test.backup.Name}})
			gotErr := err != nil
			assert.Equal(t, test.expectError, gotErr)

			backupAfter := velerov1api.Backup{}
			err = fakeClient.Get(context.TODO(), types.NamespacedName{
				Namespace: test.backup.Namespace,
				Name:      test.backup.Name,
			}, &backupAfter)

			require.NoError(t, err)
			assert.Equal(t, test.expectPhase, backupAfter.Status.Phase)
		})
	}
}
