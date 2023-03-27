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
	"bytes"
	"context"
	"io/ioutil"
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
	"github.com/vmware-tanzu/velero/pkg/plugin/clientmgmt"
	"github.com/vmware-tanzu/velero/pkg/plugin/framework"
	"github.com/vmware-tanzu/velero/pkg/plugin/velero"
	velerotest "github.com/vmware-tanzu/velero/pkg/test"
)

func mockBackupFinalizerReconciler(fakeClient kbclient.Client, fakeClock *testclocks.FakeClock) (*backupFinalizerReconciler, *fakeBackupper) {
	backupper := new(fakeBackupper)
	return NewBackupFinalizerReconciler(
		fakeClient,
		fakeClock,
		backupper,
		func(logrus.FieldLogger) clientmgmt.Manager { return pluginManager },
		NewBackupTracker(),
		NewFakeSingleObjectBackupStoreGetter(backupStore),
		logrus.StandardLogger(),
		metrics.NewServerMetrics(),
	), backupper
}
func TestBackupFinalizerReconcile(t *testing.T) {
	fakeClock := testclocks.NewFakeClock(time.Now())
	metav1Now := metav1.NewTime(fakeClock.Now())

	defaultBackupLocation := builder.ForBackupStorageLocation(velerov1api.DefaultNamespace, "default").Result()

	tests := []struct {
		name             string
		backup           *velerov1api.Backup
		backupOperations []*itemoperation.BackupOperation
		backupLocation   *velerov1api.BackupStorageLocation
		expectError      bool
		expectPhase      velerov1api.BackupPhase
	}{
		{
			name: "Finalizing backup is completed",
			backup: builder.ForBackup(velerov1api.DefaultNamespace, "backup-1").
				StorageLocation("default").
				ObjectMeta(builder.WithUID("foo")).
				StartTimestamp(fakeClock.Now()).
				Phase(velerov1api.BackupPhaseFinalizing).Result(),
			backupLocation: defaultBackupLocation,
			expectPhase:    velerov1api.BackupPhaseCompleted,
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
						PostOperationItems: []velero.ResourceIdentifier{
							{
								GroupResource: kuberesource.Secrets,
								Namespace:     "ns-1",
								Name:          "secret-1",
							},
						},
						OperationID: "operation-1",
					},
					Status: itemoperation.OperationStatus{
						Phase:   itemoperation.OperationPhaseCompleted,
						Created: &metav1Now,
					},
				},
			},
		},
		{
			name: "FinalizingPartiallyFailed backup is partially failed",
			backup: builder.ForBackup(velerov1api.DefaultNamespace, "backup-2").
				StorageLocation("default").
				ObjectMeta(builder.WithUID("foo")).
				StartTimestamp(fakeClock.Now()).
				Phase(velerov1api.BackupPhaseFinalizingPartiallyFailed).Result(),
			backupLocation: defaultBackupLocation,
			expectPhase:    velerov1api.BackupPhasePartiallyFailed,
			backupOperations: []*itemoperation.BackupOperation{
				{
					Spec: itemoperation.BackupOperationSpec{
						BackupName:       "backup-2",
						BackupUID:        "foo",
						BackupItemAction: "foo",
						ResourceIdentifier: velero.ResourceIdentifier{
							GroupResource: kuberesource.Pods,
							Namespace:     "ns-2",
							Name:          "pod-2",
						},
						PostOperationItems: []velero.ResourceIdentifier{
							{
								GroupResource: kuberesource.Secrets,
								Namespace:     "ns-2",
								Name:          "secret-2",
							},
						},
						OperationID: "operation-2",
					},
					Status: itemoperation.OperationStatus{
						Phase:   itemoperation.OperationPhaseCompleted,
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
			reconciler, backupper := mockBackupFinalizerReconciler(fakeClient, fakeClock)
			pluginManager.On("CleanupClients").Return(nil)
			backupStore.On("GetBackupItemOperations", test.backup.Name).Return(test.backupOperations, nil)
			backupStore.On("GetBackupContents", mock.Anything).Return(ioutil.NopCloser(bytes.NewReader([]byte("hello world"))), nil)
			backupStore.On("PutBackupContents", mock.Anything, mock.Anything).Return(nil)
			backupStore.On("PutBackupMetadata", mock.Anything, mock.Anything).Return(nil)
			pluginManager.On("GetBackupItemActionsV2").Return(nil, nil)
			backupper.On("FinalizeBackup", mock.Anything, mock.Anything, mock.Anything, mock.Anything, framework.BackupItemActionResolverV2{}, mock.Anything).Return(nil)
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
