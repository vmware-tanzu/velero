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
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	appsv1api "k8s.io/api/apps/v1"
	corev1api "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	clientgofake "k8s.io/client-go/kubernetes/fake"
	"k8s.io/utils/clock"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	velerov1api "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	"github.com/vmware-tanzu/velero/pkg/builder"
	"github.com/vmware-tanzu/velero/pkg/datapath"
	"github.com/vmware-tanzu/velero/pkg/exposer"
	"github.com/vmware-tanzu/velero/pkg/metrics"
	velerotest "github.com/vmware-tanzu/velero/pkg/test"
	"github.com/vmware-tanzu/velero/pkg/uploader"
	"github.com/vmware-tanzu/velero/pkg/util/kube"
)

const pvbName = "pvb-1"

func initPVBReconciler(needError ...bool) (*PodVolumeBackupReconciler, error) {
	var errs = make([]error, 6)
	for k, isError := range needError {
		if k == 0 && isError {
			errs[0] = fmt.Errorf("Get error")
		} else if k == 1 && isError {
			errs[1] = fmt.Errorf("Create error")
		} else if k == 2 && isError {
			errs[2] = fmt.Errorf("Update error")
		} else if k == 3 && isError {
			errs[3] = fmt.Errorf("Patch error")
		} else if k == 4 && isError {
			errs[4] = apierrors.NewConflict(velerov1api.Resource("podvolumebackup"), pvbName, errors.New("conflict"))
		} else if k == 5 && isError {
			errs[5] = fmt.Errorf("List error")
		}
	}

	return initPVBReconcilerWithError(errs...)
}

func initPVBReconcilerWithError(needError ...error) (*PodVolumeBackupReconciler, error) {
	daemonSet := &appsv1api.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "velero",
			Name:      "node-agent",
		},
		TypeMeta: metav1.TypeMeta{
			Kind:       "DaemonSet",
			APIVersion: appsv1api.SchemeGroupVersion.String(),
		},
		Spec: appsv1api.DaemonSetSpec{
			Template: corev1api.PodTemplateSpec{
				Spec: corev1api.PodSpec{
					Containers: []corev1api.Container{
						{
							Image: "fake-image",
						},
					},
				},
			},
		},
	}

	node := builder.ForNode("fake-node").Labels(map[string]string{kube.NodeOSLabel: kube.NodeOSLinux}).Result()

	dataPathMgr := datapath.NewManager(1)

	scheme := runtime.NewScheme()
	err := velerov1api.AddToScheme(scheme)
	if err != nil {
		return nil, err
	}

	err = corev1api.AddToScheme(scheme)
	if err != nil {
		return nil, err
	}

	fakeClient := &FakeClient{
		Client: fake.NewClientBuilder().WithScheme(scheme).Build(),
	}

	for k := range needError {
		if k == 0 {
			fakeClient.getError = needError[0]
		} else if k == 1 {
			fakeClient.createError = needError[1]
		} else if k == 2 {
			fakeClient.updateError = needError[2]
		} else if k == 3 {
			fakeClient.patchError = needError[3]
		} else if k == 4 {
			fakeClient.updateConflict = needError[4]
		} else if k == 5 {
			fakeClient.listError = needError[5]
		}
	}

	fakeKubeClient := clientgofake.NewSimpleClientset(daemonSet, node)

	return NewPodVolumeBackupReconciler(
		fakeClient,
		nil,
		fakeKubeClient,
		dataPathMgr,
		"test-node",
		time.Minute*5,
		time.Minute,
		corev1api.ResourceRequirements{},
		metrics.NewServerMetrics(),
		velerotest.NewLogger(),
	), nil
}

func pvbBuilder() *builder.PodVolumeBackupBuilder {
	return builder.ForPodVolumeBackup(velerov1api.DefaultNamespace, pvbName).BackupStorageLocation("bsl-loc")
}

type fakePvbExposer struct {
	kubeClient client.Client
	clock      clock.WithTickerAndDelayedExecution
	peekErr    error
	exposeErr  error
	getErr     error
	getNil     bool
}

func (f *fakePvbExposer) Expose(ctx context.Context, ownerObject corev1api.ObjectReference, param exposer.PodVolumeExposeParam) error {
	if f.exposeErr != nil {
		return f.exposeErr
	}

	return nil
}

func (f *fakePvbExposer) GetExposed(context.Context, corev1api.ObjectReference, client.Client, string, time.Duration) (*exposer.ExposeResult, error) {
	if f.getErr != nil {
		return nil, f.getErr
	}

	if f.getNil {
		return nil, nil
	}

	pod := &corev1api.Pod{}

	nodeOS := "linux"
	pNodeOS := &nodeOS

	return &exposer.ExposeResult{ByPod: exposer.ExposeByPod{HostingPod: pod, VolumeName: pvbName, NodeOS: pNodeOS}}, nil
}

func (f *fakePvbExposer) PeekExposed(ctx context.Context, ownerObject corev1api.ObjectReference) error {
	return f.peekErr
}

func (f *fakePvbExposer) DiagnoseExpose(context.Context, corev1api.ObjectReference) string {
	return ""
}

func (f *fakePvbExposer) CleanUp(context.Context, corev1api.ObjectReference) {
}

func TestPVBReconcile(t *testing.T) {
	tests := []struct {
		name                     string
		pvb                      *velerov1api.PodVolumeBackup
		notCreatePvb             bool
		needDelete               bool
		sportTime                *metav1.Time
		pod                      *corev1api.Pod
		dataMgr                  *datapath.Manager
		needCreateFSBR           bool
		needExclusiveUpdateError error
		needMockExposer          bool
		expected                 *velerov1api.PodVolumeBackup
		expectDeleted            bool
		expectCancelRecord       bool
		needErrs                 []bool
		peekErr                  error
		exposeErr                error
		getExposeErr             error
		getExposeNil             bool
		fsBRInitErr              error
		fsBRStartErr             error
		expectedErr              string
		expectedResult           *ctrl.Result
		expectDataPath           bool
	}{
		{
			name:         "pvb not found",
			pvb:          pvbBuilder().Result(),
			notCreatePvb: true,
		},
		{
			name: "pvb not created in velero default namespace",
			pvb:  builder.ForPodVolumeBackup("test-ns", pvbName).Result(),
		},
		{
			name:        "get pvb fail",
			pvb:         pvbBuilder().Result(),
			needErrs:    []bool{true, false, false, false},
			expectedErr: "getting PVB: Get error",
		},
		{
			name:     "add finalizer to pvb",
			pvb:      pvbBuilder().Result(),
			expected: pvbBuilder().Finalizers([]string{PodVolumeFinalizer}).Result(),
		},
		{
			name:        "add finalizer to pvb failed",
			pvb:         pvbBuilder().Result(),
			needErrs:    []bool{false, false, true, false},
			expectedErr: "error updating PVB with error velero/pvb-1: Update error",
		},
		{
			name:       "pvb is under deletion",
			pvb:        pvbBuilder().Finalizers([]string{PodVolumeFinalizer}).Result(),
			needDelete: true,
			expected:   pvbBuilder().Finalizers([]string{PodVolumeFinalizer}).Cancel(true).Result(),
		},
		{
			name:        "pvb is under deletion but cancel failed",
			pvb:         pvbBuilder().Finalizers([]string{PodVolumeFinalizer}).Result(),
			needErrs:    []bool{false, false, true, false},
			needDelete:  true,
			expectedErr: "error updating PVB with error velero/pvb-1: Update error",
		},
		{
			name:          "pvb is under deletion and in terminal state",
			pvb:           pvbBuilder().Finalizers([]string{PodVolumeFinalizer}).Phase(velerov1api.PodVolumeBackupPhaseFailed).Result(),
			sportTime:     &metav1.Time{Time: time.Now()},
			needDelete:    true,
			expectDeleted: true,
		},
		{
			name:        "pvb is under deletion and in terminal state, but remove finalizer failed",
			pvb:         pvbBuilder().Finalizers([]string{PodVolumeFinalizer}).Phase(velerov1api.PodVolumeBackupPhaseFailed).Result(),
			needErrs:    []bool{false, false, true, false},
			needDelete:  true,
			expectedErr: "error updating PVB with error velero/pvb-1: Update error",
		},
		{
			name:               "delay cancel negative for others",
			pvb:                pvbBuilder().Finalizers([]string{PodVolumeFinalizer}).Cancel(true).Phase(velerov1api.PodVolumeBackupPhasePrepared).Result(),
			sportTime:          &metav1.Time{Time: time.Now()},
			expectCancelRecord: true,
		},
		{
			name:               "delay cancel negative for inProgress",
			pvb:                pvbBuilder().Finalizers([]string{PodVolumeFinalizer}).Cancel(true).Phase(velerov1api.PodVolumeBackupPhaseInProgress).Result(),
			sportTime:          &metav1.Time{Time: time.Now().Add(-time.Minute * 58)},
			expectCancelRecord: true,
		},
		{
			name:      "delay cancel affirmative for others",
			pvb:       pvbBuilder().Finalizers([]string{PodVolumeFinalizer}).Cancel(true).Phase(velerov1api.PodVolumeBackupPhasePrepared).Result(),
			sportTime: &metav1.Time{Time: time.Now().Add(-time.Minute * 5)},
			expected:  pvbBuilder().Finalizers([]string{PodVolumeFinalizer}).Cancel(true).Phase(velerov1api.PodVolumeBackupPhaseCanceled).Result(),
		},
		{
			name:      "delay cancel affirmative for inProgress",
			pvb:       pvbBuilder().Finalizers([]string{PodVolumeFinalizer}).Cancel(true).Phase(velerov1api.PodVolumeBackupPhaseInProgress).Result(),
			sportTime: &metav1.Time{Time: time.Now().Add(-time.Hour)},
			expected:  pvbBuilder().Finalizers([]string{PodVolumeFinalizer}).Cancel(true).Phase(velerov1api.PodVolumeBackupPhaseCanceled).Result(),
		},
		{
			name:               "delay cancel failed",
			pvb:                pvbBuilder().Finalizers([]string{PodVolumeFinalizer}).Cancel(true).Phase(velerov1api.PodVolumeBackupPhaseInProgress).Result(),
			needErrs:           []bool{false, false, true, false},
			sportTime:          &metav1.Time{Time: time.Now().Add(-time.Hour)},
			expected:           pvbBuilder().Finalizers([]string{PodVolumeFinalizer}).Cancel(true).Phase(velerov1api.PodVolumeBackupPhaseInProgress).Result(),
			expectCancelRecord: true,
		},
		{
			name: "Unknown pvb status",
			pvb:  pvbBuilder().Phase("Unknown").Finalizers([]string{PodVolumeFinalizer}).Result(),
		},
		{
			name:        "new pvb but accept failed",
			pvb:         pvbBuilder().Finalizers([]string{PodVolumeFinalizer}).Node("test-node").Result(),
			needErrs:    []bool{false, false, true, false},
			expected:    pvbBuilder().Finalizers([]string{PodVolumeFinalizer}).Result(),
			expectedErr: "error accepting PVB pvb-1: error updating PVB with error velero/pvb-1: Update error",
		},
		{
			name:               "pvb is cancel on accepted",
			pvb:                pvbBuilder().Finalizers([]string{PodVolumeFinalizer}).Node("test-node").Cancel(true).Result(),
			expected:           pvbBuilder().Finalizers([]string{PodVolumeFinalizer}).Cancel(true).Phase(velerov1api.PodVolumeBackupPhaseCanceled).Result(),
			expectCancelRecord: true,
		},
		{
			name:            "pvb expose failed",
			pvb:             pvbBuilder().Finalizers([]string{PodVolumeFinalizer}).Node("test-node").Result(),
			needMockExposer: true,
			exposeErr:       errors.New("fake-expose-error"),
			expected:        pvbBuilder().Finalizers([]string{PodVolumeFinalizer}).Phase(velerov1api.PodVolumeBackupPhaseFailed).Message("error to expose PVB").Result(),
			expectedErr:     "fake-expose-error",
		},
		{
			name:            "pvb succeeds for accepted",
			pvb:             pvbBuilder().Finalizers([]string{PodVolumeFinalizer}).Node("test-node").Result(),
			needMockExposer: true,
			expected:        pvbBuilder().Finalizers([]string{PodVolumeFinalizer}).Phase(velerov1api.PodVolumeBackupPhaseAccepted).Result(),
		},
		{
			name:     "prepare timeout on accepted",
			pvb:      pvbBuilder().Phase(velerov1api.PodVolumeBackupPhaseAccepted).Finalizers([]string{PodVolumeFinalizer}).AcceptedTimestamp(&metav1.Time{Time: time.Now().Add(-time.Minute * 30)}).Result(),
			expected: pvbBuilder().Phase(velerov1api.PodVolumeBackupPhaseFailed).Finalizers([]string{PodVolumeFinalizer}).Phase(velerov1api.PodVolumeBackupPhaseFailed).Message("timeout on preparing PVB").Result(),
		},
		{
			name:            "peek error on accepted",
			pvb:             pvbBuilder().Phase(velerov1api.PodVolumeBackupPhaseAccepted).Finalizers([]string{PodVolumeFinalizer}).Result(),
			needMockExposer: true,
			peekErr:         errors.New("fake-peak-error"),
			expected:        pvbBuilder().Phase(velerov1api.PodVolumeBackupPhaseCanceled).Finalizers([]string{PodVolumeFinalizer}).Phase(velerov1api.PodVolumeBackupPhaseCanceled).Message("found a PVB velero/pvb-1 with expose error: fake-peak-error. mark it as cancel").Result(),
		},
		{
			name:     "cancel on prepared",
			pvb:      pvbBuilder().Phase(velerov1api.PodVolumeBackupPhasePrepared).Finalizers([]string{PodVolumeFinalizer}).Node("test-node").Cancel(true).Result(),
			expected: pvbBuilder().Phase(velerov1api.PodVolumeBackupPhaseCanceled).Finalizers([]string{PodVolumeFinalizer}).Cancel(true).Phase(velerov1api.PodVolumeBackupPhaseCanceled).Result(),
		},
		{
			name:            "Failed to get pvb expose on prepared",
			pvb:             pvbBuilder().Phase(velerov1api.PodVolumeBackupPhasePrepared).Finalizers([]string{PodVolumeFinalizer}).Node("test-node").Result(),
			needMockExposer: true,
			getExposeErr:    errors.New("fake-get-error"),
			expected:        pvbBuilder().Phase(velerov1api.PodVolumeBackupPhaseFailed).Finalizers([]string{PodVolumeFinalizer}).Message("exposed PVB is not ready: fake-get-error").Result(),
			expectedErr:     "fake-get-error",
		},
		{
			name:            "Get nil restore expose on prepared",
			pvb:             pvbBuilder().Phase(velerov1api.PodVolumeBackupPhasePrepared).Finalizers([]string{PodVolumeFinalizer}).Node("test-node").Result(),
			needMockExposer: true,
			getExposeNil:    true,
			expected:        pvbBuilder().Phase(velerov1api.PodVolumeBackupPhaseFailed).Finalizers([]string{PodVolumeFinalizer}).Message("exposed PVB is not ready").Result(),
			expectedErr:     "no expose result is available for the current node",
		},
		{
			name:            "Error in data path is concurrent limited",
			pvb:             pvbBuilder().Phase(velerov1api.PodVolumeBackupPhasePrepared).Finalizers([]string{PodVolumeFinalizer}).Node("test-node").Result(),
			needMockExposer: true,
			dataMgr:         datapath.NewManager(0),
			expectedResult:  &ctrl.Result{Requeue: true, RequeueAfter: time.Second * 5},
		},
		{
			name:            "data path init error",
			pvb:             pvbBuilder().Phase(velerov1api.PodVolumeBackupPhasePrepared).Finalizers([]string{PodVolumeFinalizer}).Node("test-node").Result(),
			needMockExposer: true,
			fsBRInitErr:     errors.New("fake-data-path-init-error"),
			expected:        pvbBuilder().Phase(velerov1api.PodVolumeBackupPhaseFailed).Finalizers([]string{PodVolumeFinalizer}).Message("error initializing data path").Result(),
			expectedErr:     "error initializing asyncBR: fake-data-path-init-error",
		},
		{
			name:            "Unable to update status to in progress for data upload",
			pvb:             pvbBuilder().Phase(velerov1api.PodVolumeBackupPhasePrepared).Finalizers([]string{PodVolumeFinalizer}).Node("test-node").Result(),
			needMockExposer: true,
			needErrs:        []bool{false, false, true, false},
			expected:        pvbBuilder().Phase(velerov1api.PodVolumeBackupPhasePrepared).Finalizers([]string{PodVolumeFinalizer}).Result(),
			expectedResult:  &ctrl.Result{Requeue: true, RequeueAfter: time.Second * 5},
		},
		{
			name:            "data path start error",
			pvb:             pvbBuilder().Phase(velerov1api.PodVolumeBackupPhasePrepared).Finalizers([]string{PodVolumeFinalizer}).Node("test-node").Result(),
			needMockExposer: true,
			fsBRStartErr:    errors.New("fake-data-path-start-error"),
			expected:        pvbBuilder().Phase(velerov1api.PodVolumeBackupPhaseFailed).Finalizers([]string{PodVolumeFinalizer}).Message("error starting data path").Result(),
			expectedErr:     "error starting async backup for pod , volume pvb-1: fake-data-path-start-error",
		},
		{
			name:            "Prepare succeeds",
			pvb:             pvbBuilder().Phase(velerov1api.PodVolumeBackupPhasePrepared).Finalizers([]string{PodVolumeFinalizer}).Node("test-node").Result(),
			needMockExposer: true,
			expected:        pvbBuilder().Phase(velerov1api.PodVolumeBackupPhaseInProgress).Finalizers([]string{PodVolumeFinalizer}).Result(),
			expectDataPath:  true,
		},
		{
			name:     "In progress pvb is not handled by the current node",
			pvb:      pvbBuilder().Phase(velerov1api.PodVolumeBackupPhaseInProgress).Finalizers([]string{PodVolumeFinalizer}).Result(),
			expected: pvbBuilder().Phase(velerov1api.PodVolumeBackupPhaseInProgress).Finalizers([]string{PodVolumeFinalizer}).Result(),
		},
		{
			name:     "In progress pvb is not set as cancel",
			pvb:      pvbBuilder().Phase(velerov1api.PodVolumeBackupPhaseInProgress).Finalizers([]string{PodVolumeFinalizer}).Node("test-node").Result(),
			expected: pvbBuilder().Phase(velerov1api.PodVolumeBackupPhaseInProgress).Finalizers([]string{PodVolumeFinalizer}).Result(),
		},
		{
			name:     "Cancel pvb in progress with empty FSBR",
			pvb:      pvbBuilder().Phase(velerov1api.PodVolumeBackupPhaseInProgress).Cancel(true).Finalizers([]string{PodVolumeFinalizer}).Node("test-node").Result(),
			expected: pvbBuilder().Phase(velerov1api.PodVolumeBackupPhaseCanceled).Cancel(true).Finalizers([]string{PodVolumeFinalizer}).Result(),
		},
		{
			name:               "Cancel pvb in progress and patch pvb error",
			pvb:                pvbBuilder().Phase(velerov1api.PodVolumeBackupPhaseInProgress).Cancel(true).Finalizers([]string{PodVolumeFinalizer}).Node("test-node").Result(),
			needErrs:           []bool{false, false, true, false},
			needCreateFSBR:     true,
			expected:           pvbBuilder().Phase(velerov1api.PodVolumeBackupPhaseInProgress).Cancel(true).Finalizers([]string{PodVolumeFinalizer}).Result(),
			expectedErr:        "error updating PVB with error velero/pvb-1: Update error",
			expectCancelRecord: true,
			expectDataPath:     true,
		},
		{
			name:               "Cancel pvb in progress succeeds",
			pvb:                pvbBuilder().Phase(velerov1api.PodVolumeBackupPhaseInProgress).Cancel(true).Finalizers([]string{PodVolumeFinalizer}).Node("test-node").Result(),
			needCreateFSBR:     true,
			expected:           pvbBuilder().Phase(velerov1api.PodVolumeBackupPhaseCanceling).Cancel(true).Finalizers([]string{PodVolumeFinalizer}).Result(),
			expectDataPath:     true,
			expectCancelRecord: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			r, err := initPVBReconciler(test.needErrs...)
			require.NoError(t, err)

			if !test.notCreatePvb {
				err = r.client.Create(context.Background(), test.pvb)
				require.NoError(t, err)
			}

			if test.needDelete {
				err = r.client.Delete(context.Background(), test.pvb)
				require.NoError(t, err)
			}

			if test.pod != nil {
				err = r.client.Create(ctx, test.pod)
				require.NoError(t, err)
			}

			if test.dataMgr != nil {
				r.dataPathMgr = test.dataMgr
			} else {
				r.dataPathMgr = datapath.NewManager(1)
			}

			if test.sportTime != nil {
				r.cancelledPVB[test.pvb.Name] = test.sportTime.Time
			}

			if test.needMockExposer {
				r.exposer = &fakePvbExposer{r.client, r.clock, test.peekErr, test.exposeErr, test.getExposeErr, test.getExposeNil}
			}

			funcExclusiveUpdatePodVolumeBackup = exclusiveUpdatePodVolumeBackup
			if test.needExclusiveUpdateError != nil {
				funcExclusiveUpdatePodVolumeBackup = func(context.Context, client.Client, *velerov1api.PodVolumeBackup, func(*velerov1api.PodVolumeBackup)) (bool, error) {
					return false, test.needExclusiveUpdateError
				}
			}

			datapath.MicroServiceBRWatcherCreator = func(client.Client, kubernetes.Interface, manager.Manager, string, string, string, string, string, string, datapath.Callbacks, logrus.FieldLogger) datapath.AsyncBR {
				return &fakeFSBR{
					kubeClient: r.client,
					clock:      r.clock,
					initErr:    test.fsBRInitErr,
					startErr:   test.fsBRStartErr,
				}
			}

			if test.needCreateFSBR {
				if fsBR := r.dataPathMgr.GetAsyncBR(test.pvb.Name); fsBR == nil {
					_, err := r.dataPathMgr.CreateMicroServiceBRWatcher(ctx, r.client, nil, nil, datapath.TaskTypeBackup, test.pvb.Name, velerov1api.DefaultNamespace, "", "", "", datapath.Callbacks{OnCancelled: r.OnDataPathCancelled}, false, velerotest.NewLogger())
					require.NoError(t, err)
				}
			}

			actualResult, err := r.Reconcile(ctx, ctrl.Request{
				NamespacedName: types.NamespacedName{
					Namespace: velerov1api.DefaultNamespace,
					Name:      test.pvb.Name,
				},
			})

			if test.expectedErr != "" {
				assert.EqualError(t, err, test.expectedErr)
			} else {
				assert.NoError(t, err)
			}

			if test.expectedResult != nil {
				assert.Equal(t, test.expectedResult.Requeue, actualResult.Requeue)
				assert.Equal(t, test.expectedResult.RequeueAfter, actualResult.RequeueAfter)
			}

			if test.expected != nil || test.expectDeleted {
				pvb := velerov1api.PodVolumeBackup{}
				err = r.client.Get(ctx, client.ObjectKey{
					Name:      test.pvb.Name,
					Namespace: test.pvb.Namespace,
				}, &pvb)

				if test.expectDeleted {
					assert.True(t, apierrors.IsNotFound(err))
				} else {
					require.NoError(t, err)

					assert.Equal(t, test.expected.Status.Phase, pvb.Status.Phase)
					assert.Contains(t, pvb.Status.Message, test.expected.Status.Message)
					assert.Equal(t, pvb.Finalizers, test.expected.Finalizers)
					assert.Equal(t, pvb.Spec.Cancel, test.expected.Spec.Cancel)
				}
			}

			if !test.expectDataPath {
				assert.Nil(t, r.dataPathMgr.GetAsyncBR(test.pvb.Name))
			} else {
				assert.NotNil(t, r.dataPathMgr.GetAsyncBR(test.pvb.Name))
			}

			if test.expectCancelRecord {
				assert.Contains(t, r.cancelledPVB, test.pvb.Name)
			} else {
				assert.Empty(t, r.cancelledPVB)
			}
		})
	}
}

func TestOnPVBCancelled(t *testing.T) {
	ctx := context.TODO()
	r, err := initPVBReconciler()
	require.NoError(t, err)
	pvb := pvbBuilder().Result()
	namespace := pvb.Namespace
	pvbName := pvb.Name

	assert.NoError(t, r.client.Create(ctx, pvb))

	r.OnDataPathCancelled(ctx, namespace, pvbName)
	updatedPvb := &velerov1api.PodVolumeBackup{}
	assert.NoError(t, r.client.Get(ctx, types.NamespacedName{Name: pvbName, Namespace: namespace}, updatedPvb))
	assert.Equal(t, velerov1api.PodVolumeBackupPhaseCanceled, updatedPvb.Status.Phase)
	assert.False(t, updatedPvb.Status.CompletionTimestamp.IsZero())
	assert.False(t, updatedPvb.Status.StartTimestamp.IsZero())
}

func TestOnPVBProgress(t *testing.T) {
	totalBytes := int64(1024)
	bytesDone := int64(512)
	tests := []struct {
		name     string
		pvb      *velerov1api.PodVolumeBackup
		progress uploader.Progress
		needErrs []bool
	}{
		{
			name: "patch in progress phase success",
			pvb:  pvbBuilder().Result(),
			progress: uploader.Progress{
				TotalBytes: totalBytes,
				BytesDone:  bytesDone,
			},
		},
		{
			name:     "failed to get pvb",
			pvb:      pvbBuilder().Result(),
			needErrs: []bool{true, false, false, false},
		},
		{
			name:     "failed to patch pvb",
			pvb:      pvbBuilder().Result(),
			needErrs: []bool{false, false, true, false},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ctx := context.TODO()

			r, err := initPVBReconciler(test.needErrs...)
			require.NoError(t, err)
			defer func() {
				r.client.Delete(ctx, test.pvb, &client.DeleteOptions{})
			}()

			pvb := pvbBuilder().Result()
			namespace := pvb.Namespace
			pvbName := pvb.Name

			assert.NoError(t, r.client.Create(context.Background(), pvb))

			// Create a Progress object
			progress := &uploader.Progress{
				TotalBytes: totalBytes,
				BytesDone:  bytesDone,
			}

			r.OnDataPathProgress(ctx, namespace, pvbName, progress)
			if len(test.needErrs) != 0 && !test.needErrs[0] {

				updatedPvb := &velerov1api.PodVolumeBackup{}
				assert.NoError(t, r.client.Get(ctx, types.NamespacedName{Name: pvbName, Namespace: namespace}, updatedPvb))
				assert.Equal(t, test.progress.TotalBytes, updatedPvb.Status.Progress.TotalBytes)
				assert.Equal(t, test.progress.BytesDone, updatedPvb.Status.Progress.BytesDone)
			}
		})
	}
}

func TestOnPvbFailed(t *testing.T) {
	ctx := context.TODO()
	r, err := initPVBReconciler()
	require.NoError(t, err)

	pvb := pvbBuilder().Result()
	namespace := pvb.Namespace
	pvbName := pvb.Name

	assert.NoError(t, r.client.Create(ctx, pvb))

	r.OnDataPathFailed(ctx, namespace, pvbName, fmt.Errorf("Failed to handle %v", pvbName))
	updatedPvb := &velerov1api.PodVolumeBackup{}
	assert.NoError(t, r.client.Get(ctx, types.NamespacedName{Name: pvbName, Namespace: namespace}, updatedPvb))
	assert.Equal(t, velerov1api.PodVolumeBackupPhaseFailed, updatedPvb.Status.Phase)
	assert.False(t, updatedPvb.Status.CompletionTimestamp.IsZero())
	assert.False(t, updatedPvb.Status.StartTimestamp.IsZero())
}

func TestOnPvbCompleted(t *testing.T) {
	ctx := context.TODO()
	r, err := initPVBReconciler()
	require.NoError(t, err)

	now := time.Now()
	pvb := pvbBuilder().StartTimestamp(&metav1.Time{Time: now.Add(-time.Minute)}).CompletionTimestamp(&metav1.Time{Time: now}).OwnerReference(metav1.OwnerReference{Name: "test-backup"}).Result()
	namespace := pvb.Namespace
	pvbName := pvb.Name

	assert.NoError(t, r.client.Create(ctx, pvb))

	r.OnDataPathCompleted(ctx, namespace, pvbName, datapath.Result{})
	updatedPvb := &velerov1api.PodVolumeBackup{}
	assert.NoError(t, r.client.Get(ctx, types.NamespacedName{Name: pvbName, Namespace: namespace}, updatedPvb))
	assert.Equal(t, velerov1api.PodVolumeBackupPhaseCompleted, updatedPvb.Status.Phase)
	assert.False(t, updatedPvb.Status.CompletionTimestamp.IsZero())
}

func TestFindPvbForPod(t *testing.T) {
	r, err := initPVBReconciler()
	require.NoError(t, err)
	tests := []struct {
		name      string
		pvb       *velerov1api.PodVolumeBackup
		pod       *corev1api.Pod
		checkFunc func(*velerov1api.PodVolumeBackup, []reconcile.Request)
	}{
		{
			name: "find pvb for pod",
			pvb:  pvbBuilder().Phase(velerov1api.PodVolumeBackupPhaseAccepted).Result(),
			pod:  builder.ForPod(velerov1api.DefaultNamespace, pvbName).Labels(map[string]string{velerov1api.PVBLabel: pvbName}).Status(corev1api.PodStatus{Phase: corev1api.PodRunning}).Result(),
			checkFunc: func(pvb *velerov1api.PodVolumeBackup, requests []reconcile.Request) {
				// Assert that the function returns a single request
				assert.Len(t, requests, 1)
				// Assert that the request contains the correct namespaced name
				assert.Equal(t, pvb.Namespace, requests[0].Namespace)
				assert.Equal(t, pvb.Name, requests[0].Name)
			},
		}, {
			name: "no selected label found for pod",
			pvb:  pvbBuilder().Phase(velerov1api.PodVolumeBackupPhaseAccepted).Result(),
			pod:  builder.ForPod(velerov1api.DefaultNamespace, pvbName).Result(),
			checkFunc: func(pvb *velerov1api.PodVolumeBackup, requests []reconcile.Request) {
				// Assert that the function returns a single request
				assert.Empty(t, requests)
			},
		}, {
			name: "no matched pod",
			pvb:  pvbBuilder().Phase(velerov1api.PodVolumeBackupPhaseAccepted).Result(),
			pod:  builder.ForPod(velerov1api.DefaultNamespace, pvbName).Labels(map[string]string{velerov1api.PVBLabel: "non-existing-pvb"}).Result(),
			checkFunc: func(pvb *velerov1api.PodVolumeBackup, requests []reconcile.Request) {
				assert.Empty(t, requests)
			},
		},
		{
			name: "pvb not accepte",
			pvb:  pvbBuilder().Phase(velerov1api.PodVolumeBackupPhaseInProgress).Result(),
			pod:  builder.ForPod(velerov1api.DefaultNamespace, pvbName).Labels(map[string]string{velerov1api.PVBLabel: pvbName}).Result(),
			checkFunc: func(pvb *velerov1api.PodVolumeBackup, requests []reconcile.Request) {
				assert.Empty(t, requests)
			},
		},
	}
	for _, test := range tests {
		ctx := context.Background()
		assert.NoError(t, r.client.Create(ctx, test.pod))
		assert.NoError(t, r.client.Create(ctx, test.pvb))

		requests := r.findPVBForPod(context.Background(), test.pod)
		test.checkFunc(test.pvb, requests)
		r.client.Delete(ctx, test.pvb, &client.DeleteOptions{})
		if test.pod != nil {
			r.client.Delete(ctx, test.pod, &client.DeleteOptions{})
		}
	}
}

func TestAcceptPvb(t *testing.T) {
	tests := []struct {
		name        string
		pvb         *velerov1api.PodVolumeBackup
		needErrs    []error
		expectedErr string
	}{
		{
			name:        "update fail",
			pvb:         pvbBuilder().Result(),
			needErrs:    []error{nil, nil, fmt.Errorf("fake-update-error"), nil},
			expectedErr: "fake-update-error",
		},
		{
			name:     "accepted by others",
			pvb:      pvbBuilder().Result(),
			needErrs: []error{nil, nil, &fakeAPIStatus{metav1.StatusReasonConflict}, nil},
		},
		{
			name:     "succeed",
			pvb:      pvbBuilder().Result(),
			needErrs: []error{nil, nil, nil, nil},
		},
	}
	for _, test := range tests {
		ctx := context.Background()
		r, err := initPVBReconcilerWithError(test.needErrs...)
		require.NoError(t, err)

		err = r.client.Create(ctx, test.pvb)
		require.NoError(t, err)

		err = r.acceptPodVolumeBackup(ctx, test.pvb)
		if test.expectedErr == "" {
			assert.NoError(t, err)
		} else {
			assert.EqualError(t, err, test.expectedErr)
		}
	}
}

func TestOnPvbPrepareTimeout(t *testing.T) {
	tests := []struct {
		name     string
		pvb      *velerov1api.PodVolumeBackup
		needErrs []error
		expected *velerov1api.PodVolumeBackup
	}{
		{
			name:     "update fail",
			pvb:      pvbBuilder().Result(),
			needErrs: []error{nil, nil, fmt.Errorf("fake-update-error"), nil},
			expected: pvbBuilder().Result(),
		},
		{
			name:     "update interrupted",
			pvb:      pvbBuilder().Result(),
			needErrs: []error{nil, nil, &fakeAPIStatus{metav1.StatusReasonConflict}, nil},
			expected: pvbBuilder().Result(),
		},
		{
			name:     "succeed",
			pvb:      pvbBuilder().Result(),
			needErrs: []error{nil, nil, nil, nil},
			expected: pvbBuilder().Phase(velerov1api.PodVolumeBackupPhaseFailed).Result(),
		},
	}
	for _, test := range tests {
		ctx := context.Background()
		r, err := initPVBReconcilerWithError(test.needErrs...)
		require.NoError(t, err)

		err = r.client.Create(ctx, test.pvb)
		require.NoError(t, err)

		r.onPrepareTimeout(ctx, test.pvb)

		pvb := velerov1api.PodVolumeBackup{}
		_ = r.client.Get(ctx, client.ObjectKey{
			Name:      test.pvb.Name,
			Namespace: test.pvb.Namespace,
		}, &pvb)

		assert.Equal(t, test.expected.Status.Phase, pvb.Status.Phase)
	}
}

func TestTryCancelPvb(t *testing.T) {
	tests := []struct {
		name        string
		pvb         *velerov1api.PodVolumeBackup
		needErrs    []error
		succeeded   bool
		expectedErr string
	}{
		{
			name:     "update fail",
			pvb:      pvbBuilder().Result(),
			needErrs: []error{nil, nil, fmt.Errorf("fake-update-error"), nil},
		},
		{
			name:     "cancel by others",
			pvb:      pvbBuilder().Result(),
			needErrs: []error{nil, nil, &fakeAPIStatus{metav1.StatusReasonConflict}, nil},
		},
		{
			name:      "succeed",
			pvb:       pvbBuilder().Result(),
			needErrs:  []error{nil, nil, nil, nil},
			succeeded: true,
		},
	}
	for _, test := range tests {
		ctx := context.Background()
		r, err := initPVBReconcilerWithError(test.needErrs...)
		require.NoError(t, err)

		err = r.client.Create(ctx, test.pvb)
		require.NoError(t, err)

		r.tryCancelPodVolumeBackup(ctx, test.pvb, "")

		if test.expectedErr == "" {
			assert.NoError(t, err)
		} else {
			assert.EqualError(t, err, test.expectedErr)
		}
	}
}

func TestUpdatePvbWithRetry(t *testing.T) {
	namespacedName := types.NamespacedName{
		Name:      pvbName,
		Namespace: "velero",
	}

	// Define test cases
	testCases := []struct {
		Name      string
		needErrs  []bool
		noChange  bool
		ExpectErr bool
	}{
		{
			Name: "SuccessOnFirstAttempt",
		},
		{
			Name:      "Error get",
			needErrs:  []bool{true, false, false, false, false},
			ExpectErr: true,
		},
		{
			Name:      "Error update",
			needErrs:  []bool{false, false, true, false, false},
			ExpectErr: true,
		},
		{
			Name:     "no change",
			noChange: true,
			needErrs: []bool{false, false, true, false, false},
		},
		{
			Name:      "Conflict with error timeout",
			needErrs:  []bool{false, false, false, false, true},
			ExpectErr: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.Name, func(t *testing.T) {
			ctx, cancelFunc := context.WithTimeout(context.TODO(), time.Second*5)
			defer cancelFunc()
			r, err := initPVBReconciler(tc.needErrs...)
			require.NoError(t, err)
			err = r.client.Create(ctx, pvbBuilder().Result())
			require.NoError(t, err)
			updateFunc := func(pvb *velerov1api.PodVolumeBackup) bool {
				if tc.noChange {
					return false
				}

				pvb.Spec.Cancel = true
				return true
			}
			err = UpdatePVBWithRetry(ctx, r.client, namespacedName, velerotest.NewLogger().WithField("name", tc.Name), updateFunc)
			if tc.ExpectErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
