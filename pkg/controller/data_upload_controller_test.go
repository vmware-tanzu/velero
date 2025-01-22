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

	"github.com/vmware-tanzu/velero/pkg/nodeagent"

	snapshotv1api "github.com/kubernetes-csi/external-snapshotter/client/v7/apis/volumesnapshot/v1"
	snapshotFake "github.com/kubernetes-csi/external-snapshotter/client/v7/clientset/versioned/fake"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	clientgofake "k8s.io/client-go/kubernetes/fake"
	"k8s.io/utils/clock"
	testclocks "k8s.io/utils/clock/testing"
	ctrl "sigs.k8s.io/controller-runtime"
	kbclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	velerov1api "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	velerov2alpha1api "github.com/vmware-tanzu/velero/pkg/apis/velero/v2alpha1"
	"github.com/vmware-tanzu/velero/pkg/builder"
	"github.com/vmware-tanzu/velero/pkg/datapath"
	datapathmocks "github.com/vmware-tanzu/velero/pkg/datapath/mocks"
	"github.com/vmware-tanzu/velero/pkg/exposer"
	"github.com/vmware-tanzu/velero/pkg/metrics"
	velerotest "github.com/vmware-tanzu/velero/pkg/test"
	"github.com/vmware-tanzu/velero/pkg/uploader"
	"github.com/vmware-tanzu/velero/pkg/util/boolptr"
	"github.com/vmware-tanzu/velero/pkg/util/kube"
)

const dataUploadName = "dataupload-1"

const fakeSnapshotType velerov2alpha1api.SnapshotType = "fake-snapshot"

type FakeClient struct {
	kbclient.Client
	getError       error
	createError    error
	updateError    error
	patchError     error
	updateConflict error
	listError      error
}

func (c *FakeClient) Get(ctx context.Context, key kbclient.ObjectKey, obj kbclient.Object, opts ...kbclient.GetOption) error {
	if c.getError != nil {
		return c.getError
	}

	return c.Client.Get(ctx, key, obj)
}

func (c *FakeClient) Create(ctx context.Context, obj kbclient.Object, opts ...kbclient.CreateOption) error {
	if c.createError != nil {
		return c.createError
	}

	return c.Client.Create(ctx, obj, opts...)
}

func (c *FakeClient) Update(ctx context.Context, obj kbclient.Object, opts ...kbclient.UpdateOption) error {
	if c.updateError != nil {
		return c.updateError
	}

	if c.updateConflict != nil {
		return c.updateConflict
	}

	return c.Client.Update(ctx, obj, opts...)
}

func (c *FakeClient) Patch(ctx context.Context, obj kbclient.Object, patch kbclient.Patch, opts ...kbclient.PatchOption) error {
	if c.patchError != nil {
		return c.patchError
	}

	return c.Client.Patch(ctx, obj, patch, opts...)
}

func (c *FakeClient) List(ctx context.Context, list kbclient.ObjectList, opts ...kbclient.ListOption) error {
	if c.listError != nil {
		return c.listError
	}

	return c.Client.List(ctx, list, opts...)
}

func initDataUploaderReconciler(needError ...bool) (*DataUploadReconciler, error) {
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
			errs[4] = apierrors.NewConflict(velerov2alpha1api.Resource("dataupload"), dataUploadName, errors.New("conflict"))
		} else if k == 5 && isError {
			errs[5] = fmt.Errorf("List error")
		}
	}

	return initDataUploaderReconcilerWithError(errs...)
}

func initDataUploaderReconcilerWithError(needError ...error) (*DataUploadReconciler, error) {
	vscName := "fake-vsc"
	vsObject := &snapshotv1api.VolumeSnapshot{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "fake-volume-snapshot",
			Namespace: "fake-ns",
		},
		Status: &snapshotv1api.VolumeSnapshotStatus{
			BoundVolumeSnapshotContentName: &vscName,
			ReadyToUse:                     boolptr.True(),
			RestoreSize:                    &resource.Quantity{},
		},
	}
	var restoreSize int64
	vscObj := &snapshotv1api.VolumeSnapshotContent{
		ObjectMeta: metav1.ObjectMeta{
			Name: "fake-vsc",
		},
		Spec: snapshotv1api.VolumeSnapshotContentSpec{
			DeletionPolicy: snapshotv1api.VolumeSnapshotContentDelete,
		},
		Status: &snapshotv1api.VolumeSnapshotContentStatus{
			RestoreSize: &restoreSize,
		},
	}

	daemonSet := &appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "velero",
			Name:      "node-agent",
		},
		TypeMeta: metav1.TypeMeta{
			Kind:       "DaemonSet",
			APIVersion: appsv1.SchemeGroupVersion.String(),
		},
		Spec: appsv1.DaemonSetSpec{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
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

	now, err := time.Parse(time.RFC1123, time.RFC1123)
	if err != nil {
		return nil, err
	}
	now = now.Local()
	scheme := runtime.NewScheme()
	err = velerov1api.AddToScheme(scheme)
	if err != nil {
		return nil, err
	}
	err = velerov2alpha1api.AddToScheme(scheme)
	if err != nil {
		return nil, err
	}
	err = corev1.AddToScheme(scheme)
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

	fakeSnapshotClient := snapshotFake.NewSimpleClientset(vsObject, vscObj)
	fakeKubeClient := clientgofake.NewSimpleClientset(daemonSet, node)

	return NewDataUploadReconciler(
		fakeClient,
		nil,
		fakeKubeClient,
		fakeSnapshotClient.SnapshotV1(),
		dataPathMgr,
		nil,
		map[string]nodeagent.BackupPVC{},
		corev1.ResourceRequirements{},
		testclocks.NewFakeClock(now),
		"test-node",
		time.Minute*5,
		velerotest.NewLogger(),
		metrics.NewServerMetrics(),
	), nil
}

func dataUploadBuilder() *builder.DataUploadBuilder {
	csi := &velerov2alpha1api.CSISnapshotSpec{
		SnapshotClass:  "csi-azuredisk-vsc",
		StorageClass:   "default",
		VolumeSnapshot: "fake-volume-snapshot",
	}
	return builder.ForDataUpload(velerov1api.DefaultNamespace, dataUploadName).
		Labels(map[string]string{velerov1api.DataUploadLabel: dataUploadName}).
		BackupStorageLocation("bsl-loc").
		DataMover("velero").
		SnapshotType("CSI").SourceNamespace("fake-ns").SourcePVC("test-pvc").CSISnapshot(csi)
}

type fakeSnapshotExposer struct {
	kubeClient      kbclient.Client
	clock           clock.WithTickerAndDelayedExecution
	ambiguousNodeOS bool
	peekErr         error
}

func (f *fakeSnapshotExposer) Expose(ctx context.Context, ownerObject corev1.ObjectReference, param interface{}) error {
	du := velerov2alpha1api.DataUpload{}
	err := f.kubeClient.Get(ctx, kbclient.ObjectKey{
		Name:      dataUploadName,
		Namespace: velerov1api.DefaultNamespace,
	}, &du)
	if err != nil {
		return err
	}

	original := du
	du.Status.Phase = velerov2alpha1api.DataUploadPhasePrepared
	du.Status.StartTimestamp = &metav1.Time{Time: f.clock.Now()}
	f.kubeClient.Patch(ctx, &du, kbclient.MergeFrom(&original))
	return nil
}

func (f *fakeSnapshotExposer) GetExposed(ctx context.Context, du corev1.ObjectReference, tm time.Duration, para interface{}) (*exposer.ExposeResult, error) {
	pod := &corev1.Pod{}
	err := f.kubeClient.Get(ctx, kbclient.ObjectKey{
		Name:      dataUploadName,
		Namespace: velerov1api.DefaultNamespace,
	}, pod)
	if err != nil {
		return nil, err
	}

	nodeOS := "linux"
	pNodeOS := &nodeOS
	if f.ambiguousNodeOS {
		pNodeOS = nil
	}
	return &exposer.ExposeResult{ByPod: exposer.ExposeByPod{HostingPod: pod, VolumeName: dataUploadName, NodeOS: pNodeOS}}, nil
}

func (f *fakeSnapshotExposer) PeekExposed(ctx context.Context, ownerObject corev1.ObjectReference) error {
	return f.peekErr
}

func (f *fakeSnapshotExposer) DiagnoseExpose(context.Context, corev1.ObjectReference) string {
	return ""
}

func (f *fakeSnapshotExposer) CleanUp(context.Context, corev1.ObjectReference, string, string) {
}

type fakeDataUploadFSBR struct {
	du         *velerov2alpha1api.DataUpload
	kubeClient kbclient.Client
	clock      clock.WithTickerAndDelayedExecution
	initErr    error
	startErr   error
}

func (f *fakeDataUploadFSBR) Init(ctx context.Context, param interface{}) error {
	return f.initErr
}

func (f *fakeDataUploadFSBR) StartBackup(source datapath.AccessPoint, uploaderConfigs map[string]string, param interface{}) error {
	return f.startErr
}

func (f *fakeDataUploadFSBR) StartRestore(snapshotID string, target datapath.AccessPoint, uploaderConfigs map[string]string) error {
	return nil
}

func (b *fakeDataUploadFSBR) Cancel() {
}

func (b *fakeDataUploadFSBR) Close(ctx context.Context) {
}

func TestReconcile(t *testing.T) {
	tests := []struct {
		name                string
		du                  *velerov2alpha1api.DataUpload
		pod                 *corev1.Pod
		pvc                 *corev1.PersistentVolumeClaim
		snapshotExposerList map[velerov2alpha1api.SnapshotType]exposer.SnapshotExposer
		dataMgr             *datapath.Manager
		expectedProcessed   bool
		expected            *velerov2alpha1api.DataUpload
		checkFunc           func(velerov2alpha1api.DataUpload) bool
		expectedRequeue     ctrl.Result
		expectedErrMsg      string
		needErrs            []bool
		removeNode          bool
		ambiguousNodeOS     bool
		peekErr             error
		notCreateFSBR       bool
		fsBRInitErr         error
		fsBRStartErr        error
	}{
		{
			name:            "Dataupload is not initialized",
			du:              builder.ForDataUpload("unknown-ns", "unknown-name").Result(),
			expectedRequeue: ctrl.Result{},
		},
		{
			name:            "Error get Dataupload",
			du:              builder.ForDataUpload(velerov1api.DefaultNamespace, "unknown-name").Result(),
			expectedRequeue: ctrl.Result{},
			expectedErrMsg:  "getting DataUpload: Get error",
			needErrs:        []bool{true, false, false, false},
		},
		{
			name:            "Unsupported data mover type",
			du:              dataUploadBuilder().DataMover("unknown type").Result(),
			expected:        dataUploadBuilder().Phase("").Result(),
			expectedRequeue: ctrl.Result{},
		},
		{
			name:              "Unknown type of snapshot exposer is not initialized",
			du:                dataUploadBuilder().SnapshotType("unknown type").Result(),
			expectedProcessed: true,
			expected:          dataUploadBuilder().Phase(velerov2alpha1api.DataUploadPhaseFailed).Result(),
			expectedRequeue:   ctrl.Result{},
			expectedErrMsg:    "unknown type type of snapshot exposer is not exist",
		},
		{
			name:            "Dataupload should be accepted",
			du:              dataUploadBuilder().Result(),
			pod:             builder.ForPod("fake-ns", dataUploadName).Volumes(&corev1.Volume{Name: "test-pvc"}).Result(),
			pvc:             builder.ForPersistentVolumeClaim("fake-ns", "test-pvc").Result(),
			expected:        dataUploadBuilder().Phase(velerov2alpha1api.DataUploadPhaseAccepted).Result(),
			expectedRequeue: ctrl.Result{},
		},
		{
			name:              "Dataupload should fail to get PVC information",
			du:                dataUploadBuilder().Result(),
			pod:               builder.ForPod("fake-ns", dataUploadName).Volumes(&corev1.Volume{Name: "wrong-pvc"}).Result(),
			expectedProcessed: true,
			expected:          dataUploadBuilder().Phase(velerov2alpha1api.DataUploadPhaseFailed).Result(),
			expectedRequeue:   ctrl.Result{},
			expectedErrMsg:    "failed to get PVC",
		},
		{
			name:              "Dataupload should fail to get PVC attaching node",
			du:                dataUploadBuilder().Result(),
			pod:               builder.ForPod("fake-ns", dataUploadName).Volumes(&corev1.Volume{Name: "test-pvc"}).Result(),
			pvc:               builder.ForPersistentVolumeClaim("fake-ns", "test-pvc").StorageClass("fake-sc").Result(),
			expectedProcessed: true,
			expected:          dataUploadBuilder().Phase(velerov2alpha1api.DataUploadPhaseFailed).Result(),
			expectedRequeue:   ctrl.Result{},
			expectedErrMsg:    "error to get storage class",
		},
		{
			name:              "Dataupload should fail because expected node doesn't exist",
			du:                dataUploadBuilder().Result(),
			pod:               builder.ForPod("fake-ns", dataUploadName).Volumes(&corev1.Volume{Name: "test-pvc"}).Result(),
			pvc:               builder.ForPersistentVolumeClaim("fake-ns", "test-pvc").Result(),
			removeNode:        true,
			expectedProcessed: true,
			expected:          dataUploadBuilder().Phase(velerov2alpha1api.DataUploadPhaseFailed).Result(),
			expectedRequeue:   ctrl.Result{},
			expectedErrMsg:    "no appropriate node to run data upload",
		},
		{
			name:            "Dataupload should be prepared",
			du:              dataUploadBuilder().SnapshotType(fakeSnapshotType).Result(),
			expected:        dataUploadBuilder().Phase(velerov2alpha1api.DataUploadPhasePrepared).Result(),
			expectedRequeue: ctrl.Result{},
		},
		{
			name:            "Dataupload prepared should be completed",
			pod:             builder.ForPod(velerov1api.DefaultNamespace, dataUploadName).Volumes(&corev1.Volume{Name: "dataupload-1"}).Result(),
			du:              dataUploadBuilder().Phase(velerov2alpha1api.DataUploadPhasePrepared).SnapshotType(fakeSnapshotType).Result(),
			expected:        dataUploadBuilder().Phase(velerov2alpha1api.DataUploadPhaseInProgress).Result(),
			expectedRequeue: ctrl.Result{},
		},
		{
			name:              "Dataupload should fail if expose returns ambiguous nodeOS",
			pod:               builder.ForPod(velerov1api.DefaultNamespace, dataUploadName).Volumes(&corev1.Volume{Name: "dataupload-1"}).Result(),
			du:                dataUploadBuilder().Phase(velerov2alpha1api.DataUploadPhasePrepared).SnapshotType(fakeSnapshotType).Result(),
			ambiguousNodeOS:   true,
			expectedProcessed: true,
			expected:          dataUploadBuilder().Phase(velerov2alpha1api.DataUploadPhaseFailed).Result(),
			expectedErrMsg:    "unsupported ambiguous node OS",
		},
		{
			name:            "Dataupload with not enabled cancel",
			pod:             builder.ForPod(velerov1api.DefaultNamespace, dataUploadName).Volumes(&corev1.Volume{Name: "dataupload-1"}).Result(),
			du:              dataUploadBuilder().Phase(velerov2alpha1api.DataUploadPhaseInProgress).SnapshotType(fakeSnapshotType).Cancel(false).Result(),
			expected:        dataUploadBuilder().Phase(velerov2alpha1api.DataUploadPhaseInProgress).Result(),
			expectedRequeue: ctrl.Result{},
		},
		{
			name:            "Dataupload should be cancel",
			pod:             builder.ForPod(velerov1api.DefaultNamespace, dataUploadName).Volumes(&corev1.Volume{Name: "dataupload-1"}).Result(),
			du:              dataUploadBuilder().Phase(velerov2alpha1api.DataUploadPhaseInProgress).SnapshotType(fakeSnapshotType).Cancel(true).Result(),
			expected:        dataUploadBuilder().Phase(velerov2alpha1api.DataUploadPhaseCanceling).Result(),
			expectedRequeue: ctrl.Result{},
		},
		{
			name: "Dataupload should be cancel with match node",
			pod:  builder.ForPod(velerov1api.DefaultNamespace, dataUploadName).Volumes(&corev1.Volume{Name: "dataupload-1"}).Result(),
			du: func() *velerov2alpha1api.DataUpload {
				du := dataUploadBuilder().Phase(velerov2alpha1api.DataUploadPhaseInProgress).SnapshotType(fakeSnapshotType).Cancel(true).Result()
				du.Status.Node = "test-node"
				return du
			}(),
			expectedProcessed: true,
			expected:          dataUploadBuilder().Phase(velerov2alpha1api.DataUploadPhaseCanceled).Result(),
			expectedRequeue:   ctrl.Result{},
			notCreateFSBR:     true,
		},
		{
			name: "Dataupload should not be cancel with mismatch node",
			pod:  builder.ForPod(velerov1api.DefaultNamespace, dataUploadName).Volumes(&corev1.Volume{Name: "dataupload-1"}).Result(),
			du: func() *velerov2alpha1api.DataUpload {
				du := dataUploadBuilder().Phase(velerov2alpha1api.DataUploadPhaseInProgress).SnapshotType(fakeSnapshotType).Cancel(true).Result()
				du.Status.Node = "different_node"
				return du
			}(),
			expected:        dataUploadBuilder().Phase(velerov2alpha1api.DataUploadPhaseInProgress).Result(),
			expectedRequeue: ctrl.Result{},
			notCreateFSBR:   true,
		},
		{
			name:            "runCancelableDataUpload is concurrent limited",
			dataMgr:         datapath.NewManager(0),
			pod:             builder.ForPod(velerov1api.DefaultNamespace, dataUploadName).Volumes(&corev1.Volume{Name: "dataupload-1"}).Result(),
			du:              dataUploadBuilder().Phase(velerov2alpha1api.DataUploadPhasePrepared).SnapshotType(fakeSnapshotType).Result(),
			expected:        dataUploadBuilder().Phase(velerov2alpha1api.DataUploadPhasePrepared).Result(),
			expectedRequeue: ctrl.Result{Requeue: true, RequeueAfter: time.Second * 5},
		},
		{
			name:              "data path init error",
			pod:               builder.ForPod(velerov1api.DefaultNamespace, dataUploadName).Volumes(&corev1.Volume{Name: "dataupload-1"}).Result(),
			du:                dataUploadBuilder().Phase(velerov2alpha1api.DataUploadPhasePrepared).SnapshotType(fakeSnapshotType).Result(),
			fsBRInitErr:       errors.New("fake-data-path-init-error"),
			expectedProcessed: true,
			expected:          dataUploadBuilder().Phase(velerov2alpha1api.DataUploadPhaseFailed).SnapshotType(fakeSnapshotType).Result(),
			expectedErrMsg:    "error initializing asyncBR: fake-data-path-init-error",
		},
		{
			name:            "Unable to update status to in progress for data download",
			pod:             builder.ForPod(velerov1api.DefaultNamespace, dataUploadName).Volumes(&corev1.Volume{Name: "dataupload-1"}).Result(),
			du:              dataUploadBuilder().Phase(velerov2alpha1api.DataUploadPhasePrepared).SnapshotType(fakeSnapshotType).Result(),
			needErrs:        []bool{false, false, false, true},
			expected:        dataUploadBuilder().Phase(velerov2alpha1api.DataUploadPhasePrepared).SnapshotType(fakeSnapshotType).Result(),
			expectedRequeue: ctrl.Result{Requeue: true, RequeueAfter: time.Second * 5},
		},
		{
			name:              "data path start error",
			pod:               builder.ForPod(velerov1api.DefaultNamespace, dataUploadName).Volumes(&corev1.Volume{Name: "dataupload-1"}).Result(),
			du:                dataUploadBuilder().Phase(velerov2alpha1api.DataUploadPhasePrepared).SnapshotType(fakeSnapshotType).Result(),
			fsBRStartErr:      errors.New("fake-data-path-start-error"),
			expectedProcessed: true,
			expected:          dataUploadBuilder().Phase(velerov2alpha1api.DataUploadPhaseFailed).SnapshotType(fakeSnapshotType).Result(),
			expectedErrMsg:    "error starting async backup for pod dataupload-1, volume dataupload-1: fake-data-path-start-error",
		},
		{
			name:     "prepare timeout",
			du:       dataUploadBuilder().Phase(velerov2alpha1api.DataUploadPhaseAccepted).SnapshotType(fakeSnapshotType).AcceptedTimestamp(&metav1.Time{Time: time.Now().Add(-time.Minute * 5)}).Result(),
			expected: dataUploadBuilder().Phase(velerov2alpha1api.DataUploadPhaseFailed).Result(),
		},
		{
			name:              "peek error",
			du:                dataUploadBuilder().Phase(velerov2alpha1api.DataUploadPhaseAccepted).SnapshotType(fakeSnapshotType).Result(),
			peekErr:           errors.New("fake-peek-error"),
			expectedProcessed: true,
			expected:          dataUploadBuilder().Phase(velerov2alpha1api.DataUploadPhaseCanceled).Result(),
		},
		{
			name: "Dataupload with enabled cancel",
			pod:  builder.ForPod(velerov1api.DefaultNamespace, dataUploadName).Volumes(&corev1.Volume{Name: "dataupload-1"}).Result(),
			du: func() *velerov2alpha1api.DataUpload {
				du := dataUploadBuilder().Phase(velerov2alpha1api.DataUploadPhaseAccepted).SnapshotType(fakeSnapshotType).Result()
				controllerutil.AddFinalizer(du, DataUploadDownloadFinalizer)
				du.DeletionTimestamp = &metav1.Time{Time: time.Now()}
				return du
			}(),
			checkFunc: func(du velerov2alpha1api.DataUpload) bool {
				return du.Spec.Cancel
			},
			expected:        dataUploadBuilder().Phase(velerov2alpha1api.DataUploadPhaseAccepted).Result(),
			expectedRequeue: ctrl.Result{},
		},
		{
			name: "Dataupload with remove finalizer and should not be retrieved",
			pod:  builder.ForPod(velerov1api.DefaultNamespace, dataUploadName).Volumes(&corev1.Volume{Name: "dataupload-1"}).Result(),
			du: func() *velerov2alpha1api.DataUpload {
				du := dataUploadBuilder().Phase(velerov2alpha1api.DataUploadPhaseFailed).SnapshotType(fakeSnapshotType).Cancel(true).Result()
				controllerutil.AddFinalizer(du, DataUploadDownloadFinalizer)
				du.DeletionTimestamp = &metav1.Time{Time: time.Now()}
				return du
			}(),
			checkFunc: func(du velerov2alpha1api.DataUpload) bool {
				return !controllerutil.ContainsFinalizer(&du, DataUploadDownloadFinalizer)
			},
			expectedRequeue: ctrl.Result{},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			r, err := initDataUploaderReconciler(test.needErrs...)
			require.NoError(t, err)
			defer func() {
				r.client.Delete(ctx, test.du, &kbclient.DeleteOptions{})
				if test.pod != nil {
					r.client.Delete(ctx, test.pod, &kbclient.DeleteOptions{})
				}
			}()
			ctx := context.Background()
			if test.du.Namespace == velerov1api.DefaultNamespace {
				isDeletionTimestampSet := test.du.DeletionTimestamp != nil
				err = r.client.Create(ctx, test.du)
				require.NoError(t, err)
				// because of the changes introduced by https://github.com/kubernetes-sigs/controller-runtime/commit/7a66d580c0c53504f5b509b45e9300cc18a1cc30
				// the fake client ignores the DeletionTimestamp when calling the Create(),
				// so call Delete() here
				if isDeletionTimestampSet {
					err = r.client.Delete(ctx, test.du)
					require.NoError(t, err)
				}
			}

			if test.pod != nil {
				err = r.client.Create(ctx, test.pod)
				require.NoError(t, err)
			}

			if test.pvc != nil {
				err = r.client.Create(ctx, test.pvc)
				require.NoError(t, err)
			}

			if test.removeNode {
				err = r.kubeClient.CoreV1().Nodes().Delete(ctx, "fake-node", metav1.DeleteOptions{})
				require.NoError(t, err)
			}

			if test.dataMgr != nil {
				r.dataPathMgr = test.dataMgr
			} else {
				r.dataPathMgr = datapath.NewManager(1)
			}

			if test.du.Spec.SnapshotType == fakeSnapshotType {
				r.snapshotExposerList = map[velerov2alpha1api.SnapshotType]exposer.SnapshotExposer{fakeSnapshotType: &fakeSnapshotExposer{r.client, r.Clock, test.ambiguousNodeOS, test.peekErr}}
			} else if test.du.Spec.SnapshotType == velerov2alpha1api.SnapshotTypeCSI {
				r.snapshotExposerList = map[velerov2alpha1api.SnapshotType]exposer.SnapshotExposer{velerov2alpha1api.SnapshotTypeCSI: exposer.NewCSISnapshotExposer(r.kubeClient, r.csiSnapshotClient, velerotest.NewLogger())}
			}
			if !test.notCreateFSBR {
				datapath.MicroServiceBRWatcherCreator = func(kbclient.Client, kubernetes.Interface, manager.Manager, string, string, string, string, string, string, datapath.Callbacks, logrus.FieldLogger) datapath.AsyncBR {
					return &fakeDataUploadFSBR{
						du:         test.du,
						kubeClient: r.client,
						clock:      r.Clock,
						initErr:    test.fsBRInitErr,
						startErr:   test.fsBRStartErr,
					}
				}
			}

			testCreateFsBR := false
			if test.du.Status.Phase == velerov2alpha1api.DataUploadPhaseInProgress && !test.notCreateFSBR {
				if fsBR := r.dataPathMgr.GetAsyncBR(test.du.Name); fsBR == nil {
					testCreateFsBR = true
					_, err := r.dataPathMgr.CreateMicroServiceBRWatcher(ctx, r.client, nil, nil, datapath.TaskTypeBackup, test.du.Name, velerov1api.DefaultNamespace, "", "", "", datapath.Callbacks{OnCancelled: r.OnDataUploadCancelled}, false, velerotest.NewLogger())
					require.NoError(t, err)
				}
			}

			actualResult, err := r.Reconcile(ctx, ctrl.Request{
				NamespacedName: types.NamespacedName{
					Namespace: velerov1api.DefaultNamespace,
					Name:      test.du.Name,
				},
			})

			assert.Equal(t, test.expectedRequeue, actualResult)
			if test.expectedErrMsg == "" {
				require.NoError(t, err)
			} else {
				require.ErrorContains(t, err, test.expectedErrMsg)
			}

			du := velerov2alpha1api.DataUpload{}
			err = r.client.Get(ctx, kbclient.ObjectKey{
				Name:      test.du.Name,
				Namespace: test.du.Namespace,
			}, &du)
			t.Logf("%s: \n %v \n", test.name, du)
			// Assertions
			if test.expected == nil {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, test.expected.Status.Phase, du.Status.Phase)
			}

			if test.expectedProcessed {
				assert.False(t, du.Status.CompletionTimestamp.IsZero())
			}

			if !test.expectedProcessed {
				assert.True(t, du.Status.CompletionTimestamp.IsZero())
			}

			if test.checkFunc != nil {
				assert.True(t, test.checkFunc(du))
			}

			if !testCreateFsBR && du.Status.Phase != velerov2alpha1api.DataUploadPhaseInProgress {
				assert.Nil(t, r.dataPathMgr.GetAsyncBR(test.du.Name))
			}
		})
	}
}

func TestOnDataUploadCancelled(t *testing.T) {
	ctx := context.TODO()
	r, err := initDataUploaderReconciler()
	require.NoError(t, err)
	// Create a DataUpload object
	du := dataUploadBuilder().Result()
	namespace := du.Namespace
	duName := du.Name
	// Add the DataUpload object to the fake client
	assert.NoError(t, r.client.Create(ctx, du))

	r.OnDataUploadCancelled(ctx, namespace, duName)
	updatedDu := &velerov2alpha1api.DataUpload{}
	assert.NoError(t, r.client.Get(ctx, types.NamespacedName{Name: duName, Namespace: namespace}, updatedDu))
	assert.Equal(t, velerov2alpha1api.DataUploadPhaseCanceled, updatedDu.Status.Phase)
	assert.False(t, updatedDu.Status.CompletionTimestamp.IsZero())
	assert.False(t, updatedDu.Status.StartTimestamp.IsZero())
}

func TestOnDataUploadProgress(t *testing.T) {
	totalBytes := int64(1024)
	bytesDone := int64(512)
	tests := []struct {
		name     string
		du       *velerov2alpha1api.DataUpload
		progress uploader.Progress
		needErrs []bool
	}{
		{
			name: "patch in progress phase success",
			du:   dataUploadBuilder().Result(),
			progress: uploader.Progress{
				TotalBytes: totalBytes,
				BytesDone:  bytesDone,
			},
		},
		{
			name:     "failed to get dataupload",
			du:       dataUploadBuilder().Result(),
			needErrs: []bool{true, false, false, false},
		},
		{
			name:     "failed to patch dataupload",
			du:       dataUploadBuilder().Result(),
			needErrs: []bool{false, false, false, true},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ctx := context.TODO()

			r, err := initDataUploaderReconciler(test.needErrs...)
			require.NoError(t, err)
			defer func() {
				r.client.Delete(ctx, test.du, &kbclient.DeleteOptions{})
			}()
			// Create a DataUpload object
			du := dataUploadBuilder().Result()
			namespace := du.Namespace
			duName := du.Name
			// Add the DataUpload object to the fake client
			assert.NoError(t, r.client.Create(context.Background(), du))

			// Create a Progress object
			progress := &uploader.Progress{
				TotalBytes: totalBytes,
				BytesDone:  bytesDone,
			}

			// Call the OnDataUploadProgress function
			r.OnDataUploadProgress(ctx, namespace, duName, progress)
			if len(test.needErrs) != 0 && !test.needErrs[0] {
				// Get the updated DataUpload object from the fake client
				updatedDu := &velerov2alpha1api.DataUpload{}
				assert.NoError(t, r.client.Get(ctx, types.NamespacedName{Name: duName, Namespace: namespace}, updatedDu))
				// Assert that the DataUpload object has been updated with the progress
				assert.Equal(t, test.progress.TotalBytes, updatedDu.Status.Progress.TotalBytes)
				assert.Equal(t, test.progress.BytesDone, updatedDu.Status.Progress.BytesDone)
			}
		})
	}
}

func TestOnDataUploadFailed(t *testing.T) {
	ctx := context.TODO()
	r, err := initDataUploaderReconciler()
	require.NoError(t, err)

	// Create a DataUpload object
	du := dataUploadBuilder().Result()
	namespace := du.Namespace
	duName := du.Name
	// Add the DataUpload object to the fake client
	assert.NoError(t, r.client.Create(ctx, du))
	r.snapshotExposerList = map[velerov2alpha1api.SnapshotType]exposer.SnapshotExposer{velerov2alpha1api.SnapshotTypeCSI: exposer.NewCSISnapshotExposer(r.kubeClient, r.csiSnapshotClient, velerotest.NewLogger())}
	r.OnDataUploadFailed(ctx, namespace, duName, fmt.Errorf("Failed to handle %v", duName))
	updatedDu := &velerov2alpha1api.DataUpload{}
	assert.NoError(t, r.client.Get(ctx, types.NamespacedName{Name: duName, Namespace: namespace}, updatedDu))
	assert.Equal(t, velerov2alpha1api.DataUploadPhaseFailed, updatedDu.Status.Phase)
	assert.False(t, updatedDu.Status.CompletionTimestamp.IsZero())
	assert.False(t, updatedDu.Status.StartTimestamp.IsZero())
}

func TestOnDataUploadCompleted(t *testing.T) {
	ctx := context.TODO()
	r, err := initDataUploaderReconciler()
	require.NoError(t, err)
	// Create a DataUpload object
	du := dataUploadBuilder().Result()
	namespace := du.Namespace
	duName := du.Name
	// Add the DataUpload object to the fake client
	assert.NoError(t, r.client.Create(ctx, du))
	r.snapshotExposerList = map[velerov2alpha1api.SnapshotType]exposer.SnapshotExposer{velerov2alpha1api.SnapshotTypeCSI: exposer.NewCSISnapshotExposer(r.kubeClient, r.csiSnapshotClient, velerotest.NewLogger())}
	r.OnDataUploadCompleted(ctx, namespace, duName, datapath.Result{})
	updatedDu := &velerov2alpha1api.DataUpload{}
	assert.NoError(t, r.client.Get(ctx, types.NamespacedName{Name: duName, Namespace: namespace}, updatedDu))
	assert.Equal(t, velerov2alpha1api.DataUploadPhaseCompleted, updatedDu.Status.Phase)
	assert.False(t, updatedDu.Status.CompletionTimestamp.IsZero())
}

func TestFindDataUploadForPod(t *testing.T) {
	r, err := initDataUploaderReconciler()
	require.NoError(t, err)
	tests := []struct {
		name      string
		du        *velerov2alpha1api.DataUpload
		pod       *corev1.Pod
		checkFunc func(*velerov2alpha1api.DataUpload, []reconcile.Request)
	}{
		{
			name: "find dataUpload for pod",
			du:   dataUploadBuilder().Phase(velerov2alpha1api.DataUploadPhaseAccepted).Result(),
			pod:  builder.ForPod(velerov1api.DefaultNamespace, dataUploadName).Labels(map[string]string{velerov1api.DataUploadLabel: dataUploadName}).Status(corev1.PodStatus{Phase: corev1.PodRunning}).Result(),
			checkFunc: func(du *velerov2alpha1api.DataUpload, requests []reconcile.Request) {
				// Assert that the function returns a single request
				assert.Len(t, requests, 1)
				// Assert that the request contains the correct namespaced name
				assert.Equal(t, du.Namespace, requests[0].Namespace)
				assert.Equal(t, du.Name, requests[0].Name)
			},
		}, {
			name: "no selected label found for pod",
			du:   dataUploadBuilder().Phase(velerov2alpha1api.DataUploadPhaseAccepted).Result(),
			pod:  builder.ForPod(velerov1api.DefaultNamespace, dataUploadName).Result(),
			checkFunc: func(du *velerov2alpha1api.DataUpload, requests []reconcile.Request) {
				// Assert that the function returns a single request
				assert.Empty(t, requests)
			},
		}, {
			name: "no matched pod",
			du:   dataUploadBuilder().Phase(velerov2alpha1api.DataUploadPhaseAccepted).Result(),
			pod:  builder.ForPod(velerov1api.DefaultNamespace, dataUploadName).Labels(map[string]string{velerov1api.DataUploadLabel: "non-existing-dataupload"}).Result(),
			checkFunc: func(du *velerov2alpha1api.DataUpload, requests []reconcile.Request) {
				assert.Empty(t, requests)
			},
		},
		{
			name: "dataUpload not accepte",
			du:   dataUploadBuilder().Phase(velerov2alpha1api.DataUploadPhaseInProgress).Result(),
			pod:  builder.ForPod(velerov1api.DefaultNamespace, dataUploadName).Labels(map[string]string{velerov1api.DataUploadLabel: dataUploadName}).Result(),
			checkFunc: func(du *velerov2alpha1api.DataUpload, requests []reconcile.Request) {
				assert.Empty(t, requests)
			},
		},
	}
	for _, test := range tests {
		ctx := context.Background()
		assert.NoError(t, r.client.Create(ctx, test.pod))
		assert.NoError(t, r.client.Create(ctx, test.du))
		// Call the findDataUploadForPod function
		requests := r.findDataUploadForPod(context.Background(), test.pod)
		test.checkFunc(test.du, requests)
		r.client.Delete(ctx, test.du, &kbclient.DeleteOptions{})
		if test.pod != nil {
			r.client.Delete(ctx, test.pod, &kbclient.DeleteOptions{})
		}
	}
}

type fakeAPIStatus struct {
	reason metav1.StatusReason
}

func (f *fakeAPIStatus) Status() metav1.Status {
	return metav1.Status{
		Reason: f.reason,
	}
}

func (f *fakeAPIStatus) Error() string {
	return string(f.reason)
}

func TestAcceptDataUpload(t *testing.T) {
	tests := []struct {
		name        string
		du          *velerov2alpha1api.DataUpload
		needErrs    []error
		succeeded   bool
		expectedErr string
	}{
		{
			name:        "update fail",
			du:          dataUploadBuilder().Result(),
			needErrs:    []error{nil, nil, fmt.Errorf("fake-update-error"), nil},
			expectedErr: "fake-update-error",
		},
		{
			name:     "accepted by others",
			du:       dataUploadBuilder().Result(),
			needErrs: []error{nil, nil, &fakeAPIStatus{metav1.StatusReasonConflict}, nil},
		},
		{
			name:      "succeed",
			du:        dataUploadBuilder().Result(),
			needErrs:  []error{nil, nil, nil, nil},
			succeeded: true,
		},
	}
	for _, test := range tests {
		ctx := context.Background()
		r, err := initDataUploaderReconcilerWithError(test.needErrs...)
		require.NoError(t, err)

		err = r.client.Create(ctx, test.du)
		require.NoError(t, err)

		succeeded, err := r.acceptDataUpload(ctx, test.du)
		assert.Equal(t, test.succeeded, succeeded)
		if test.expectedErr == "" {
			assert.NoError(t, err)
		} else {
			assert.EqualError(t, err, test.expectedErr)
		}
	}
}

func TestOnDuPrepareTimeout(t *testing.T) {
	tests := []struct {
		name     string
		du       *velerov2alpha1api.DataUpload
		needErrs []error
		expected *velerov2alpha1api.DataUpload
	}{
		{
			name:     "update fail",
			du:       dataUploadBuilder().Result(),
			needErrs: []error{nil, nil, fmt.Errorf("fake-update-error"), nil},
			expected: dataUploadBuilder().Result(),
		},
		{
			name:     "update interrupted",
			du:       dataUploadBuilder().Result(),
			needErrs: []error{nil, nil, &fakeAPIStatus{metav1.StatusReasonConflict}, nil},
			expected: dataUploadBuilder().Result(),
		},
		{
			name:     "succeed",
			du:       dataUploadBuilder().Result(),
			needErrs: []error{nil, nil, nil, nil},
			expected: dataUploadBuilder().Phase(velerov2alpha1api.DataUploadPhaseFailed).Result(),
		},
	}
	for _, test := range tests {
		ctx := context.Background()
		r, err := initDataUploaderReconcilerWithError(test.needErrs...)
		require.NoError(t, err)

		err = r.client.Create(ctx, test.du)
		require.NoError(t, err)

		r.onPrepareTimeout(ctx, test.du)

		du := velerov2alpha1api.DataUpload{}
		_ = r.client.Get(ctx, kbclient.ObjectKey{
			Name:      test.du.Name,
			Namespace: test.du.Namespace,
		}, &du)

		assert.Equal(t, test.expected.Status.Phase, du.Status.Phase)
	}
}

func TestTryCancelDataUpload(t *testing.T) {
	tests := []struct {
		name        string
		dd          *velerov2alpha1api.DataUpload
		needErrs    []error
		succeeded   bool
		expectedErr string
	}{
		{
			name:     "update fail",
			dd:       dataUploadBuilder().Result(),
			needErrs: []error{nil, nil, fmt.Errorf("fake-update-error"), nil},
		},
		{
			name:     "cancel by others",
			dd:       dataUploadBuilder().Result(),
			needErrs: []error{nil, nil, &fakeAPIStatus{metav1.StatusReasonConflict}, nil},
		},
		{
			name:      "succeed",
			dd:        dataUploadBuilder().Result(),
			needErrs:  []error{nil, nil, nil, nil},
			succeeded: true,
		},
	}
	for _, test := range tests {
		ctx := context.Background()
		r, err := initDataUploaderReconcilerWithError(test.needErrs...)
		require.NoError(t, err)

		err = r.client.Create(ctx, test.dd)
		require.NoError(t, err)

		r.tryCancelAcceptedDataUpload(ctx, test.dd, "")

		if test.expectedErr == "" {
			assert.NoError(t, err)
		} else {
			assert.EqualError(t, err, test.expectedErr)
		}
	}
}

func TestUpdateDataUploadWithRetry(t *testing.T) {
	namespacedName := types.NamespacedName{
		Name:      dataUploadName,
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
			r, err := initDataUploaderReconciler(tc.needErrs...)
			require.NoError(t, err)
			err = r.client.Create(ctx, dataUploadBuilder().Result())
			require.NoError(t, err)
			updateFunc := func(dataDownload *velerov2alpha1api.DataUpload) bool {
				if tc.noChange {
					return false
				}

				dataDownload.Spec.Cancel = true
				return true
			}
			err = UpdateDataUploadWithRetry(ctx, r.client, namespacedName, velerotest.NewLogger().WithField("name", tc.Name), updateFunc)
			if tc.ExpectErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

type duResumeTestHelper struct {
	resumeErr    error
	getExposeErr error
	exposeResult *exposer.ExposeResult
	asyncBR      datapath.AsyncBR
}

func (dt *duResumeTestHelper) resumeCancellableDataPath(_ *DataUploadReconciler, _ context.Context, _ *velerov2alpha1api.DataUpload, _ logrus.FieldLogger) error {
	return dt.resumeErr
}

func (dt *duResumeTestHelper) Expose(context.Context, corev1.ObjectReference, interface{}) error {
	return nil
}

func (dt *duResumeTestHelper) GetExposed(context.Context, corev1.ObjectReference, time.Duration, interface{}) (*exposer.ExposeResult, error) {
	return dt.exposeResult, dt.getExposeErr
}

func (dt *duResumeTestHelper) PeekExposed(context.Context, corev1.ObjectReference) error {
	return nil
}

func (dt *duResumeTestHelper) DiagnoseExpose(context.Context, corev1.ObjectReference) string {
	return ""
}

func (dt *duResumeTestHelper) CleanUp(context.Context, corev1.ObjectReference, string, string) {}

func (dt *duResumeTestHelper) newMicroServiceBRWatcher(kbclient.Client, kubernetes.Interface, manager.Manager, string, string, string, string, string, string,
	datapath.Callbacks, logrus.FieldLogger) datapath.AsyncBR {
	return dt.asyncBR
}

func TestAttemptDataUploadResume(t *testing.T) {
	tests := []struct {
		name                  string
		dataUploads           []velerov2alpha1api.DataUpload
		du                    *velerov2alpha1api.DataUpload
		needErrs              []bool
		acceptedDataUploads   []string
		prepareddDataUploads  []string
		cancelledDataUploads  []string
		inProgressDataUploads []string
		resumeErr             error
		expectedError         string
	}{
		{
			name:                 "accepted DataUpload in other node",
			du:                   dataUploadBuilder().Phase(velerov2alpha1api.DataUploadPhaseAccepted).Result(),
			cancelledDataUploads: []string{dataUploadName},
			acceptedDataUploads:  []string{dataUploadName},
		},
		{
			name:                 "accepted DataUpload in the current node",
			du:                   dataUploadBuilder().Phase(velerov2alpha1api.DataUploadPhaseAccepted).AcceptedByNode("node-1").Result(),
			cancelledDataUploads: []string{dataUploadName},
			acceptedDataUploads:  []string{dataUploadName},
		},
		{
			name:                 "accepted DataUpload in the current node but canceled",
			du:                   dataUploadBuilder().Phase(velerov2alpha1api.DataUploadPhaseAccepted).AcceptedByNode("node-1").Cancel(true).Result(),
			cancelledDataUploads: []string{dataUploadName},
			acceptedDataUploads:  []string{dataUploadName},
		},
		{
			name:                "accepted DataUpload in the current node but update error",
			du:                  dataUploadBuilder().Phase(velerov2alpha1api.DataUploadPhaseAccepted).AcceptedByNode("node-1").Result(),
			needErrs:            []bool{false, false, true, false, false, false},
			acceptedDataUploads: []string{dataUploadName},
		},
		{
			name:                 "prepared DataUpload",
			du:                   dataUploadBuilder().Phase(velerov2alpha1api.DataUploadPhasePrepared).Result(),
			prepareddDataUploads: []string{dataUploadName},
		},
		{
			name:                  "InProgress DataUpload, not the current node",
			du:                    dataUploadBuilder().Phase(velerov2alpha1api.DataUploadPhaseInProgress).Result(),
			inProgressDataUploads: []string{dataUploadName},
		},
		{
			name:                  "InProgress DataUpload, resume error and update error",
			du:                    dataUploadBuilder().Phase(velerov2alpha1api.DataUploadPhaseInProgress).Node("node-1").Result(),
			needErrs:              []bool{false, false, true, false, false, false},
			resumeErr:             errors.New("fake-resume-error"),
			inProgressDataUploads: []string{dataUploadName},
		},
		{
			name:                  "InProgress DataUpload, resume error and update succeed",
			du:                    dataUploadBuilder().Phase(velerov2alpha1api.DataUploadPhaseInProgress).Node("node-1").Result(),
			resumeErr:             errors.New("fake-resume-error"),
			cancelledDataUploads:  []string{dataUploadName},
			inProgressDataUploads: []string{dataUploadName},
		},
		{
			name:                  "InProgress DataUpload and resume succeed",
			du:                    dataUploadBuilder().Phase(velerov2alpha1api.DataUploadPhaseInProgress).Node("node-1").Result(),
			inProgressDataUploads: []string{dataUploadName},
		},
		{
			name:          "Error",
			needErrs:      []bool{false, false, false, false, false, true},
			du:            dataUploadBuilder().Phase(velerov2alpha1api.DataUploadPhasePrepared).Result(),
			expectedError: "error to list datauploads: List error",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ctx := context.TODO()
			r, err := initDataUploaderReconciler(test.needErrs...)
			r.nodeName = "node-1"
			require.NoError(t, err)

			assert.NoError(t, r.client.Create(ctx, test.du))

			dt := &duResumeTestHelper{
				resumeErr: test.resumeErr,
			}

			funcResumeCancellableDataBackup = dt.resumeCancellableDataPath

			// Run the test
			err = r.AttemptDataUploadResume(ctx, r.logger.WithField("name", test.name), test.du.Namespace)

			if test.expectedError != "" {
				assert.EqualError(t, err, test.expectedError)
			} else {
				assert.NoError(t, err)

				// Verify DataUploads marked as Canceled
				for _, duName := range test.cancelledDataUploads {
					dataUpload := &velerov2alpha1api.DataUpload{}
					err := r.client.Get(context.Background(), types.NamespacedName{Namespace: "velero", Name: duName}, dataUpload)
					require.NoError(t, err)
					assert.True(t, dataUpload.Spec.Cancel)
				}
				// Verify DataUploads marked as Accepted
				for _, duName := range test.acceptedDataUploads {
					dataUpload := &velerov2alpha1api.DataUpload{}
					err := r.client.Get(context.Background(), types.NamespacedName{Namespace: "velero", Name: duName}, dataUpload)
					require.NoError(t, err)
					assert.Equal(t, velerov2alpha1api.DataUploadPhaseAccepted, dataUpload.Status.Phase)
				}
				// Verify DataUploads marked as Prepared
				for _, duName := range test.prepareddDataUploads {
					dataUpload := &velerov2alpha1api.DataUpload{}
					err := r.client.Get(context.Background(), types.NamespacedName{Namespace: "velero", Name: duName}, dataUpload)
					require.NoError(t, err)
					assert.Equal(t, velerov2alpha1api.DataUploadPhasePrepared, dataUpload.Status.Phase)
				}
				// Verify DataUploads marked as InProgress
				for _, duName := range test.inProgressDataUploads {
					dataUpload := &velerov2alpha1api.DataUpload{}
					err := r.client.Get(context.Background(), types.NamespacedName{Namespace: "velero", Name: duName}, dataUpload)
					require.NoError(t, err)
					assert.Equal(t, velerov2alpha1api.DataUploadPhaseInProgress, dataUpload.Status.Phase)
				}
			}
		})
	}
}

func TestResumeCancellableBackup(t *testing.T) {
	tests := []struct {
		name             string
		dataUploads      []velerov2alpha1api.DataUpload
		du               *velerov2alpha1api.DataUpload
		getExposeErr     error
		exposeResult     *exposer.ExposeResult
		createWatcherErr error
		initWatcherErr   error
		startWatcherErr  error
		mockInit         bool
		mockStart        bool
		mockClose        bool
		expectedError    string
	}{
		{
			name:          "not find exposer",
			du:            dataUploadBuilder().Phase(velerov2alpha1api.DataUploadPhaseInProgress).SnapshotType("").Result(),
			expectedError: fmt.Sprintf("error to find exposer for du %s", dataUploadName),
		},
		{
			name:          "get expose failed",
			du:            dataUploadBuilder().Phase(velerov2alpha1api.DataUploadPhaseInProgress).SnapshotType(velerov2alpha1api.SnapshotTypeCSI).Result(),
			getExposeErr:  errors.New("fake-expose-error"),
			expectedError: fmt.Sprintf("error to get exposed snapshot for du %s: fake-expose-error", dataUploadName),
		},
		{
			name:          "no expose",
			du:            dataUploadBuilder().Phase(velerov2alpha1api.DataUploadPhaseAccepted).Node("node-1").Result(),
			expectedError: fmt.Sprintf("expose info missed for du %s", dataUploadName),
		},
		{
			name: "watcher init error",
			du:   dataUploadBuilder().Phase(velerov2alpha1api.DataUploadPhaseAccepted).Node("node-1").Result(),
			exposeResult: &exposer.ExposeResult{
				ByPod: exposer.ExposeByPod{
					HostingPod: &corev1.Pod{},
				},
			},
			mockInit:       true,
			mockClose:      true,
			initWatcherErr: errors.New("fake-init-watcher-error"),
			expectedError:  fmt.Sprintf("error to init asyncBR watcher for du %s: fake-init-watcher-error", dataUploadName),
		},
		{
			name: "start watcher error",
			du:   dataUploadBuilder().Phase(velerov2alpha1api.DataUploadPhaseAccepted).Node("node-1").Result(),
			exposeResult: &exposer.ExposeResult{
				ByPod: exposer.ExposeByPod{
					HostingPod: &corev1.Pod{},
				},
			},
			mockInit:        true,
			mockStart:       true,
			mockClose:       true,
			startWatcherErr: errors.New("fake-start-watcher-error"),
			expectedError:   fmt.Sprintf("error to resume asyncBR watcher for du %s: fake-start-watcher-error", dataUploadName),
		},
		{
			name: "succeed",
			du:   dataUploadBuilder().Phase(velerov2alpha1api.DataUploadPhaseAccepted).Node("node-1").Result(),
			exposeResult: &exposer.ExposeResult{
				ByPod: exposer.ExposeByPod{
					HostingPod: &corev1.Pod{},
				},
			},
			mockInit:  true,
			mockStart: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ctx := context.TODO()
			r, err := initDataUploaderReconciler()
			r.nodeName = "node-1"
			require.NoError(t, err)

			mockAsyncBR := datapathmocks.NewAsyncBR(t)

			if test.mockInit {
				mockAsyncBR.On("Init", mock.Anything, mock.Anything).Return(test.initWatcherErr)
			}

			if test.mockStart {
				mockAsyncBR.On("StartBackup", mock.Anything, mock.Anything, mock.Anything).Return(test.startWatcherErr)
			}

			if test.mockClose {
				mockAsyncBR.On("Close", mock.Anything).Return()
			}

			dt := &duResumeTestHelper{
				getExposeErr: test.getExposeErr,
				exposeResult: test.exposeResult,
				asyncBR:      mockAsyncBR,
			}

			r.snapshotExposerList[velerov2alpha1api.SnapshotTypeCSI] = dt

			datapath.MicroServiceBRWatcherCreator = dt.newMicroServiceBRWatcher

			err = r.resumeCancellableDataPath(ctx, test.du, velerotest.NewLogger())
			if test.expectedError != "" {
				assert.EqualError(t, err, test.expectedError)
			}
		})
	}
}
