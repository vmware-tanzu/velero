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

	snapshotv1api "github.com/kubernetes-csi/external-snapshotter/client/v4/apis/volumesnapshot/v1"
	snapshotFake "github.com/kubernetes-csi/external-snapshotter/client/v4/clientset/versioned/fake"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	clientgofake "k8s.io/client-go/kubernetes/fake"
	"k8s.io/utils/clock"
	testclocks "k8s.io/utils/clock/testing"
	ctrl "sigs.k8s.io/controller-runtime"
	kbclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/vmware-tanzu/velero/internal/credentials"
	velerov1api "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	velerov2alpha1api "github.com/vmware-tanzu/velero/pkg/apis/velero/v2alpha1"
	"github.com/vmware-tanzu/velero/pkg/builder"
	"github.com/vmware-tanzu/velero/pkg/datapath"
	"github.com/vmware-tanzu/velero/pkg/exposer"
	"github.com/vmware-tanzu/velero/pkg/metrics"
	"github.com/vmware-tanzu/velero/pkg/repository"
	velerotest "github.com/vmware-tanzu/velero/pkg/test"
	"github.com/vmware-tanzu/velero/pkg/uploader"
	"github.com/vmware-tanzu/velero/pkg/util/boolptr"
)

const dataUploadName = "dataupload-1"

const fakeSnapshotType velerov2alpha1api.SnapshotType = "fake-snapshot"

type FakeClient struct {
	kbclient.Client
	getError    error
	createError error
	updateError error
	patchError  error
}

func (c *FakeClient) Get(ctx context.Context, key kbclient.ObjectKey, obj kbclient.Object) error {
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

	return c.Client.Update(ctx, obj, opts...)
}

func (c *FakeClient) Patch(ctx context.Context, obj kbclient.Object, patch kbclient.Patch, opts ...kbclient.PatchOption) error {
	if c.patchError != nil {
		return c.patchError
	}

	return c.Client.Patch(ctx, obj, patch, opts...)
}

func initDataUploaderReconciler(needError ...bool) (*DataUploadReconciler, error) {
	var errs []error = make([]error, 4)
	if len(needError) == 4 {
		if needError[0] {
			errs[0] = fmt.Errorf("Get error")
		}

		if needError[1] {
			errs[1] = fmt.Errorf("Create error")
		}

		if needError[2] {
			errs[2] = fmt.Errorf("Update error")
		}

		if needError[3] {
			errs[3] = fmt.Errorf("Patch error")
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
		Spec: appsv1.DaemonSetSpec{},
	}

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

	if len(needError) == 4 {
		fakeClient.getError = needError[0]
		fakeClient.createError = needError[1]
		fakeClient.updateError = needError[2]
		fakeClient.patchError = needError[3]
	}

	fakeSnapshotClient := snapshotFake.NewSimpleClientset(vsObject, vscObj)
	fakeKubeClient := clientgofake.NewSimpleClientset(daemonSet)
	fakeFS := velerotest.NewFakeFileSystem()
	pathGlob := fmt.Sprintf("/host_pods/%s/volumes/*/%s", "", dataUploadName)
	_, err = fakeFS.Create(pathGlob)
	if err != nil {
		return nil, err
	}

	credentialFileStore, err := credentials.NewNamespacedFileStore(
		fakeClient,
		velerov1api.DefaultNamespace,
		"/tmp/credentials",
		fakeFS,
	)
	if err != nil {
		return nil, err
	}
	return NewDataUploadReconciler(fakeClient, fakeKubeClient, fakeSnapshotClient.SnapshotV1(), nil,
		testclocks.NewFakeClock(now), &credentials.CredentialGetter{FromFile: credentialFileStore}, "test_node", fakeFS, time.Minute*5, velerotest.NewLogger(), metrics.NewServerMetrics()), nil
}

func dataUploadBuilder() *builder.DataUploadBuilder {
	csi := &velerov2alpha1api.CSISnapshotSpec{
		SnapshotClass:  "csi-azuredisk-vsc",
		StorageClass:   "default",
		VolumeSnapshot: "fake-volume-snapshot",
	}
	return builder.ForDataUpload(velerov1api.DefaultNamespace, dataUploadName).
		BackupStorageLocation("bsl-loc").
		DataMover("velero").
		SnapshotType("CSI").SourceNamespace("fake-ns").SourcePVC("test-pvc").CSISnapshot(csi)
}

type fakeSnapshotExposer struct {
	kubeClient kbclient.Client
	clock      clock.WithTickerAndDelayedExecution
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
	return &exposer.ExposeResult{ByPod: exposer.ExposeByPod{HostingPod: pod, VolumeName: dataUploadName}}, nil
}

func (f *fakeSnapshotExposer) CleanUp(context.Context, corev1.ObjectReference, string, string) {
}

type fakeDataUploadFSBR struct {
	du         *velerov2alpha1api.DataUpload
	kubeClient kbclient.Client
	clock      clock.WithTickerAndDelayedExecution
}

func (f *fakeDataUploadFSBR) Init(ctx context.Context, bslName string, sourceNamespace string, uploaderType string, repositoryType string, repoIdentifier string, repositoryEnsurer *repository.Ensurer, credentialGetter *credentials.CredentialGetter) error {
	return nil
}

func (f *fakeDataUploadFSBR) StartBackup(source datapath.AccessPoint, realSource string, parentSnapshot string, forceFull bool, tags map[string]string) error {
	du := f.du
	original := f.du.DeepCopy()
	du.Status.Phase = velerov2alpha1api.DataUploadPhaseCompleted
	du.Status.CompletionTimestamp = &metav1.Time{Time: f.clock.Now()}
	f.kubeClient.Patch(context.Background(), du, kbclient.MergeFrom(original))

	return nil
}

func (f *fakeDataUploadFSBR) StartRestore(snapshotID string, target datapath.AccessPoint) error {
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
		snapshotExposerList map[velerov2alpha1api.SnapshotType]exposer.SnapshotExposer
		dataMgr             *datapath.Manager
		expectedProcessed   bool
		expected            *velerov2alpha1api.DataUpload
		expectedRequeue     ctrl.Result
		expectedErrMsg      string
		needErrs            []bool
	}{
		{
			name:              "Dataupload is not initialized",
			du:                builder.ForDataUpload("unknown-ns", "unknown-name").Result(),
			expectedProcessed: false,
			expected:          nil,
			expectedRequeue:   ctrl.Result{},
		}, {
			name:              "Error get Dataupload",
			du:                builder.ForDataUpload(velerov1api.DefaultNamespace, "unknown-name").Result(),
			expectedProcessed: false,
			expected:          nil,
			expectedRequeue:   ctrl.Result{},
			expectedErrMsg:    "getting DataUpload: Get error",
			needErrs:          []bool{true, false, false, false},
		}, {
			name:              "Unsupported data mover type",
			du:                dataUploadBuilder().DataMover("unknown type").Result(),
			expectedProcessed: false,
			expected:          dataUploadBuilder().Phase("").Result(),
			expectedRequeue:   ctrl.Result{},
		}, {
			name:              "Unknown type of snapshot exposer is not initialized",
			du:                dataUploadBuilder().SnapshotType("unknown type").Result(),
			expectedProcessed: true,
			expected:          dataUploadBuilder().Phase(velerov2alpha1api.DataUploadPhaseFailed).Result(),
			expectedRequeue:   ctrl.Result{},
			expectedErrMsg:    "unknown type type of snapshot exposer is not exist",
		}, {
			name:              "Dataupload should be accepted",
			du:                dataUploadBuilder().Result(),
			pod:               builder.ForPod(velerov1api.DefaultNamespace, dataUploadName).Volumes(&corev1.Volume{Name: "dataupload-1"}).Result(),
			expectedProcessed: false,
			expected:          dataUploadBuilder().Phase(velerov2alpha1api.DataUploadPhaseAccepted).Result(),
			expectedRequeue:   ctrl.Result{},
		},
		{
			name:              "Dataupload should be prepared",
			du:                dataUploadBuilder().SnapshotType(fakeSnapshotType).Result(),
			expectedProcessed: false,
			expected:          dataUploadBuilder().Phase(velerov2alpha1api.DataUploadPhasePrepared).Result(),
			expectedRequeue:   ctrl.Result{},
		}, {
			name:              "Dataupload prepared should be completed",
			pod:               builder.ForPod(velerov1api.DefaultNamespace, dataUploadName).Volumes(&corev1.Volume{Name: "dataupload-1"}).Result(),
			du:                dataUploadBuilder().Phase(velerov2alpha1api.DataUploadPhasePrepared).SnapshotType(fakeSnapshotType).Result(),
			expectedProcessed: true,
			expected:          dataUploadBuilder().Phase(velerov2alpha1api.DataUploadPhaseCompleted).Result(),
			expectedRequeue:   ctrl.Result{},
		},
		{
			name:              "Dataupload with not enabled cancel",
			pod:               builder.ForPod(velerov1api.DefaultNamespace, dataUploadName).Volumes(&corev1.Volume{Name: "dataupload-1"}).Result(),
			du:                dataUploadBuilder().Phase(velerov2alpha1api.DataUploadPhaseInProgress).SnapshotType(fakeSnapshotType).Cancel(false).Result(),
			expectedProcessed: false,
			expected:          dataUploadBuilder().Phase(velerov2alpha1api.DataUploadPhaseInProgress).Result(),
			expectedRequeue:   ctrl.Result{},
		},
		{
			name:              "Dataupload should be cancel",
			pod:               builder.ForPod(velerov1api.DefaultNamespace, dataUploadName).Volumes(&corev1.Volume{Name: "dataupload-1"}).Result(),
			du:                dataUploadBuilder().Phase(velerov2alpha1api.DataUploadPhaseInProgress).SnapshotType(fakeSnapshotType).Cancel(true).Result(),
			expectedProcessed: false,
			expected:          dataUploadBuilder().Phase(velerov2alpha1api.DataUploadPhaseCanceling).Result(),
			expectedRequeue:   ctrl.Result{},
		},
		{
			name:              "runCancelableDataUpload is concurrent limited",
			dataMgr:           datapath.NewManager(0),
			pod:               builder.ForPod(velerov1api.DefaultNamespace, dataUploadName).Volumes(&corev1.Volume{Name: "dataupload-1"}).Result(),
			du:                dataUploadBuilder().Phase(velerov2alpha1api.DataUploadPhasePrepared).SnapshotType(fakeSnapshotType).Result(),
			expectedProcessed: false,
			expected:          dataUploadBuilder().Phase(velerov2alpha1api.DataUploadPhasePrepared).Result(),
			expectedRequeue:   ctrl.Result{Requeue: true, RequeueAfter: time.Minute},
		},
		{
			name:     "prepare timeout",
			du:       dataUploadBuilder().Phase(velerov2alpha1api.DataUploadPhaseAccepted).SnapshotType(fakeSnapshotType).StartTimestamp(&metav1.Time{Time: time.Now().Add(-time.Minute * 5)}).Result(),
			expected: dataUploadBuilder().Phase(velerov2alpha1api.DataUploadPhaseFailed).Result(),
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
				err = r.client.Create(ctx, test.du)
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

			if test.du.Spec.SnapshotType == fakeSnapshotType {
				r.snapshotExposerList = map[velerov2alpha1api.SnapshotType]exposer.SnapshotExposer{fakeSnapshotType: &fakeSnapshotExposer{r.client, r.Clock}}
			} else if test.du.Spec.SnapshotType == velerov2alpha1api.SnapshotTypeCSI {
				r.snapshotExposerList = map[velerov2alpha1api.SnapshotType]exposer.SnapshotExposer{velerov2alpha1api.SnapshotTypeCSI: exposer.NewCSISnapshotExposer(r.kubeClient, r.csiSnapshotClient, velerotest.NewLogger())}
			}

			datapath.FSBRCreator = func(string, string, kbclient.Client, string, datapath.Callbacks, logrus.FieldLogger) datapath.AsyncBR {
				return &fakeDataUploadFSBR{
					du:         test.du,
					kubeClient: r.client,
					clock:      r.Clock,
				}
			}

			if test.du.Status.Phase == velerov2alpha1api.DataUploadPhaseInProgress {
				if fsBR := r.dataPathMgr.GetAsyncBR(test.du.Name); fsBR == nil {
					_, err := r.dataPathMgr.CreateFileSystemBR(test.du.Name, pVBRRequestor, ctx, r.client, velerov1api.DefaultNamespace, datapath.Callbacks{OnCancelled: r.OnDataUploadCancelled}, velerotest.NewLogger())
					require.NoError(t, err)
				}
			}

			actualResult, err := r.Reconcile(ctx, ctrl.Request{
				NamespacedName: types.NamespacedName{
					Namespace: velerov1api.DefaultNamespace,
					Name:      test.du.Name,
				},
			})

			assert.Equal(t, actualResult, test.expectedRequeue)
			if test.expectedErrMsg == "" {
				require.NoError(t, err)
			} else {
				assert.Contains(t, err.Error(), test.expectedErrMsg)
			}

			du := velerov2alpha1api.DataUpload{}
			err = r.client.Get(ctx, kbclient.ObjectKey{
				Name:      test.du.Name,
				Namespace: test.du.Namespace,
			}, &du)
			t.Logf("%s: \n %v \n", test.name, du)
			// Assertions
			if test.expected == nil {
				assert.Equal(t, err != nil, true)
			} else {
				require.NoError(t, err)
				assert.Equal(t, du.Status.Phase, test.expected.Status.Phase)
			}

			if test.expectedProcessed {
				assert.Equal(t, du.Status.CompletionTimestamp.IsZero(), false)
			}

			if !test.expectedProcessed {
				assert.Equal(t, du.Status.CompletionTimestamp.IsZero(), true)
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
	assert.Equal(t, updatedDu.Status.CompletionTimestamp.IsZero(), false)
	assert.Equal(t, updatedDu.Status.StartTimestamp.IsZero(), false)
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
	assert.Equal(t, updatedDu.Status.CompletionTimestamp.IsZero(), false)
	assert.Equal(t, updatedDu.Status.StartTimestamp.IsZero(), false)
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
	assert.Equal(t, updatedDu.Status.CompletionTimestamp.IsZero(), false)
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
			pod:  builder.ForPod(velerov1api.DefaultNamespace, dataUploadName).Labels(map[string]string{velerov1api.DataUploadLabel: dataUploadName}).Result(),
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
		requests := r.findDataUploadForPod(test.pod)
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
