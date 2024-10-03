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

package exposer

import (
	"context"
	"fmt"
	"testing"
	"time"

	snapshotv1api "github.com/kubernetes-csi/external-snapshotter/client/v7/apis/volumesnapshot/v1"
	snapshotFake "github.com/kubernetes-csi/external-snapshotter/client/v7/clientset/versioned/fake"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	clientTesting "k8s.io/client-go/testing"
	"k8s.io/utils/pointer"
	clientFake "sigs.k8s.io/controller-runtime/pkg/client/fake"

	velerov1 "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	"github.com/vmware-tanzu/velero/pkg/nodeagent"
	velerotest "github.com/vmware-tanzu/velero/pkg/test"
	"github.com/vmware-tanzu/velero/pkg/util/boolptr"
)

type reactor struct {
	verb        string
	resource    string
	reactorFunc clientTesting.ReactionFunc
}

func TestExpose(t *testing.T) {
	vscName := "fake-vsc"
	backup := &velerov1.Backup{
		TypeMeta: metav1.TypeMeta{
			APIVersion: velerov1.SchemeGroupVersion.String(),
			Kind:       "Backup",
		},
		ObjectMeta: metav1.ObjectMeta{
			Namespace: velerov1.DefaultNamespace,
			Name:      "fake-backup",
			UID:       "fake-uid",
		},
	}

	var restoreSize int64 = 123456

	snapshotClass := "fake-snapshot-class"
	vsObject := &snapshotv1api.VolumeSnapshot{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "fake-vs",
			Namespace: "fake-ns",
			Annotations: map[string]string{
				"fake-key-1": "fake-value-1",
				"fake-key-2": "fake-value-2",
			},
		},
		Spec: snapshotv1api.VolumeSnapshotSpec{
			Source: snapshotv1api.VolumeSnapshotSource{
				VolumeSnapshotContentName: &vscName,
			},
			VolumeSnapshotClassName: &snapshotClass,
		},
		Status: &snapshotv1api.VolumeSnapshotStatus{
			BoundVolumeSnapshotContentName: &vscName,
			ReadyToUse:                     boolptr.True(),
			RestoreSize:                    resource.NewQuantity(restoreSize, ""),
		},
	}

	vsObjectWithoutRestoreSize := &snapshotv1api.VolumeSnapshot{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "fake-vs",
			Namespace: "fake-ns",
			Annotations: map[string]string{
				"fake-key-1": "fake-value-1",
				"fake-key-2": "fake-value-2",
			},
		},
		Spec: snapshotv1api.VolumeSnapshotSpec{
			Source: snapshotv1api.VolumeSnapshotSource{
				VolumeSnapshotContentName: &vscName,
			},
			VolumeSnapshotClassName: &snapshotClass,
		},
		Status: &snapshotv1api.VolumeSnapshotStatus{
			BoundVolumeSnapshotContentName: &vscName,
			ReadyToUse:                     boolptr.True(),
		},
	}

	snapshotHandle := "fake-handle"
	vscObj := &snapshotv1api.VolumeSnapshotContent{
		ObjectMeta: metav1.ObjectMeta{
			Name: vscName,
			Annotations: map[string]string{
				"fake-key-3": "fake-value-3",
				"fake-key-4": "fake-value-4",
			},
		},
		Spec: snapshotv1api.VolumeSnapshotContentSpec{
			DeletionPolicy:          snapshotv1api.VolumeSnapshotContentDelete,
			Driver:                  "fake-driver",
			VolumeSnapshotClassName: &snapshotClass,
		},
		Status: &snapshotv1api.VolumeSnapshotContentStatus{
			RestoreSize:    &restoreSize,
			SnapshotHandle: &snapshotHandle,
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
							Name: "node-agent",
						},
					},
				},
			},
		},
	}

	tests := []struct {
		name                          string
		snapshotClientObj             []runtime.Object
		kubeClientObj                 []runtime.Object
		ownerBackup                   *velerov1.Backup
		exposeParam                   CSISnapshotExposeParam
		snapReactors                  []reactor
		kubeReactors                  []reactor
		err                           string
		expectedVolumeSize            *resource.Quantity
		expectedReadOnlyPVC           bool
		expectedBackupPVCStorageClass string
	}{
		{
			name:        "wait vs ready fail",
			ownerBackup: backup,
			exposeParam: CSISnapshotExposeParam{
				SnapshotName:     "fake-vs",
				OperationTimeout: time.Millisecond,
				ExposeTimeout:    time.Millisecond,
			},
			err: "error wait volume snapshot ready: error to get VolumeSnapshot /fake-vs: volumesnapshots.snapshot.storage.k8s.io \"fake-vs\" not found",
		},
		{
			name:        "get vsc fail",
			ownerBackup: backup,
			exposeParam: CSISnapshotExposeParam{
				SnapshotName:     "fake-vs",
				SourceNamespace:  "fake-ns",
				OperationTimeout: time.Millisecond,
				ExposeTimeout:    time.Millisecond,
			},
			snapshotClientObj: []runtime.Object{
				vsObject,
			},
			err: "error to get volume snapshot content: error getting volume snapshot content from API: volumesnapshotcontents.snapshot.storage.k8s.io \"fake-vsc\" not found",
		},
		{
			name:        "delete vs fail",
			ownerBackup: backup,
			exposeParam: CSISnapshotExposeParam{
				SnapshotName:     "fake-vs",
				SourceNamespace:  "fake-ns",
				OperationTimeout: time.Millisecond,
				ExposeTimeout:    time.Millisecond,
			},
			snapshotClientObj: []runtime.Object{
				vsObject,
				vscObj,
			},
			snapReactors: []reactor{
				{
					verb:     "delete",
					resource: "volumesnapshots",
					reactorFunc: func(action clientTesting.Action) (handled bool, ret runtime.Object, err error) {
						return true, nil, errors.New("fake-delete-error")
					},
				},
			},
			err: "error to delete volume snapshot: error to delete volume snapshot: fake-delete-error",
		},
		{
			name:        "delete vsc fail",
			ownerBackup: backup,
			exposeParam: CSISnapshotExposeParam{
				SnapshotName:     "fake-vs",
				SourceNamespace:  "fake-ns",
				OperationTimeout: time.Millisecond,
				ExposeTimeout:    time.Millisecond,
			},
			snapshotClientObj: []runtime.Object{
				vsObject,
				vscObj,
			},
			snapReactors: []reactor{
				{
					verb:     "delete",
					resource: "volumesnapshotcontents",
					reactorFunc: func(action clientTesting.Action) (handled bool, ret runtime.Object, err error) {
						return true, nil, errors.New("fake-delete-error")
					},
				},
			},
			err: "error to delete volume snapshot content: error to delete volume snapshot content: fake-delete-error",
		},
		{
			name:        "create backup vs fail",
			ownerBackup: backup,
			exposeParam: CSISnapshotExposeParam{
				SnapshotName:     "fake-vs",
				SourceNamespace:  "fake-ns",
				OperationTimeout: time.Millisecond,
				ExposeTimeout:    time.Millisecond,
			},
			snapshotClientObj: []runtime.Object{
				vsObject,
				vscObj,
			},
			snapReactors: []reactor{
				{
					verb:     "create",
					resource: "volumesnapshots",
					reactorFunc: func(action clientTesting.Action) (handled bool, ret runtime.Object, err error) {
						return true, nil, errors.New("fake-create-error")
					},
				},
			},
			err: "error to create backup volume snapshot: fake-create-error",
		},
		{
			name:        "create backup vsc fail",
			ownerBackup: backup,
			exposeParam: CSISnapshotExposeParam{
				SnapshotName:     "fake-vs",
				SourceNamespace:  "fake-ns",
				OperationTimeout: time.Millisecond,
				ExposeTimeout:    time.Millisecond,
			},
			snapshotClientObj: []runtime.Object{
				vsObject,
				vscObj,
			},
			snapReactors: []reactor{
				{
					verb:     "create",
					resource: "volumesnapshotcontents",
					reactorFunc: func(action clientTesting.Action) (handled bool, ret runtime.Object, err error) {
						return true, nil, errors.New("fake-create-error")
					},
				},
			},
			err: "error to create backup volume snapshot content: fake-create-error",
		},
		{
			name:        "create backup pvc fail, invalid access mode",
			ownerBackup: backup,
			exposeParam: CSISnapshotExposeParam{
				SnapshotName:    "fake-vs",
				SourceNamespace: "fake-ns",
				AccessMode:      "fake-mode",
			},
			snapshotClientObj: []runtime.Object{
				vsObject,
				vscObj,
			},
			err: "error to create backup pvc: unsupported access mode fake-mode",
		},
		{
			name:        "create backup pvc fail",
			ownerBackup: backup,
			exposeParam: CSISnapshotExposeParam{
				SnapshotName:     "fake-vs",
				SourceNamespace:  "fake-ns",
				OperationTimeout: time.Millisecond,
				ExposeTimeout:    time.Millisecond,
				AccessMode:       AccessModeFileSystem,
			},
			snapshotClientObj: []runtime.Object{
				vsObject,
				vscObj,
			},
			kubeReactors: []reactor{
				{
					verb:     "create",
					resource: "persistentvolumeclaims",
					reactorFunc: func(action clientTesting.Action) (handled bool, ret runtime.Object, err error) {
						return true, nil, errors.New("fake-create-error")
					},
				},
			},
			err: "error to create backup pvc: error to create pvc: fake-create-error",
		},
		{
			name:        "create backup pod fail",
			ownerBackup: backup,
			exposeParam: CSISnapshotExposeParam{
				SnapshotName:     "fake-vs",
				SourceNamespace:  "fake-ns",
				AccessMode:       AccessModeFileSystem,
				OperationTimeout: time.Millisecond,
				ExposeTimeout:    time.Millisecond,
			},
			snapshotClientObj: []runtime.Object{
				vsObject,
				vscObj,
			},
			kubeClientObj: []runtime.Object{
				daemonSet,
			},
			kubeReactors: []reactor{
				{
					verb:     "create",
					resource: "pods",
					reactorFunc: func(action clientTesting.Action) (handled bool, ret runtime.Object, err error) {
						return true, nil, errors.New("fake-create-error")
					},
				},
			},
			err: "error to create backup pod: fake-create-error",
		},
		{
			name:        "success",
			ownerBackup: backup,
			exposeParam: CSISnapshotExposeParam{
				SnapshotName:     "fake-vs",
				SourceNamespace:  "fake-ns",
				AccessMode:       AccessModeFileSystem,
				OperationTimeout: time.Millisecond,
				ExposeTimeout:    time.Millisecond,
			},
			snapshotClientObj: []runtime.Object{
				vsObject,
				vscObj,
			},
			kubeClientObj: []runtime.Object{
				daemonSet,
			},
		},
		{
			name:        "restore size from exposeParam",
			ownerBackup: backup,
			exposeParam: CSISnapshotExposeParam{
				SnapshotName:     "fake-vs",
				SourceNamespace:  "fake-ns",
				AccessMode:       AccessModeFileSystem,
				OperationTimeout: time.Millisecond,
				ExposeTimeout:    time.Millisecond,
				VolumeSize:       *resource.NewQuantity(567890, ""),
			},
			snapshotClientObj: []runtime.Object{
				vsObjectWithoutRestoreSize,
				vscObj,
			},
			kubeClientObj: []runtime.Object{
				daemonSet,
			},
			expectedVolumeSize: resource.NewQuantity(567890, ""),
		},
		{
			name:        "backupPod mounts read only backupPVC",
			ownerBackup: backup,
			exposeParam: CSISnapshotExposeParam{
				SnapshotName:     "fake-vs",
				SourceNamespace:  "fake-ns",
				StorageClass:     "fake-sc",
				AccessMode:       AccessModeFileSystem,
				OperationTimeout: time.Millisecond,
				ExposeTimeout:    time.Millisecond,
				BackupPVCConfig: map[string]nodeagent.BackupPVC{
					"fake-sc": {
						StorageClass: "fake-sc-read-only",
						ReadOnly:     true,
					},
				},
			},
			snapshotClientObj: []runtime.Object{
				vsObject,
				vscObj,
			},
			kubeClientObj: []runtime.Object{
				daemonSet,
			},
			expectedReadOnlyPVC: true,
		},
		{
			name:        "backupPod mounts read only backupPVC and storageClass specified in backupPVC config",
			ownerBackup: backup,
			exposeParam: CSISnapshotExposeParam{
				SnapshotName:     "fake-vs",
				SourceNamespace:  "fake-ns",
				StorageClass:     "fake-sc",
				AccessMode:       AccessModeFileSystem,
				OperationTimeout: time.Millisecond,
				ExposeTimeout:    time.Millisecond,
				BackupPVCConfig: map[string]nodeagent.BackupPVC{
					"fake-sc": {
						StorageClass: "fake-sc-read-only",
						ReadOnly:     true,
					},
				},
			},
			snapshotClientObj: []runtime.Object{
				vsObject,
				vscObj,
			},
			kubeClientObj: []runtime.Object{
				daemonSet,
			},
			expectedReadOnlyPVC:           true,
			expectedBackupPVCStorageClass: "fake-sc-read-only",
		},
		{
			name:        "backupPod mounts backupPVC with storageClass specified in backupPVC config",
			ownerBackup: backup,
			exposeParam: CSISnapshotExposeParam{
				SnapshotName:     "fake-vs",
				SourceNamespace:  "fake-ns",
				StorageClass:     "fake-sc",
				AccessMode:       AccessModeFileSystem,
				OperationTimeout: time.Millisecond,
				ExposeTimeout:    time.Millisecond,
				BackupPVCConfig: map[string]nodeagent.BackupPVC{
					"fake-sc": {
						StorageClass: "fake-sc-read-only",
					},
				},
			},
			snapshotClientObj: []runtime.Object{
				vsObject,
				vscObj,
			},
			kubeClientObj: []runtime.Object{
				daemonSet,
			},
			expectedBackupPVCStorageClass: "fake-sc-read-only",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fakeSnapshotClient := snapshotFake.NewSimpleClientset(test.snapshotClientObj...)
			fakeKubeClient := fake.NewSimpleClientset(test.kubeClientObj...)

			for _, reactor := range test.snapReactors {
				fakeSnapshotClient.Fake.PrependReactor(reactor.verb, reactor.resource, reactor.reactorFunc)
			}

			for _, reactor := range test.kubeReactors {
				fakeKubeClient.Fake.PrependReactor(reactor.verb, reactor.resource, reactor.reactorFunc)
			}

			exposer := csiSnapshotExposer{
				kubeClient:        fakeKubeClient,
				csiSnapshotClient: fakeSnapshotClient.SnapshotV1(),
				log:               velerotest.NewLogger(),
			}

			var ownerObject corev1.ObjectReference
			if test.ownerBackup != nil {
				ownerObject = corev1.ObjectReference{
					Kind:       test.ownerBackup.Kind,
					Namespace:  test.ownerBackup.Namespace,
					Name:       test.ownerBackup.Name,
					UID:        test.ownerBackup.UID,
					APIVersion: test.ownerBackup.APIVersion,
				}
			}

			err := exposer.Expose(context.Background(), ownerObject, &test.exposeParam)
			if err == nil {
				assert.NoError(t, err)

				_, err = exposer.kubeClient.CoreV1().Pods(ownerObject.Namespace).Get(context.Background(), ownerObject.Name, metav1.GetOptions{})
				assert.NoError(t, err)

				backupPVC, err := exposer.kubeClient.CoreV1().PersistentVolumeClaims(ownerObject.Namespace).Get(context.Background(), ownerObject.Name, metav1.GetOptions{})
				assert.NoError(t, err)

				expectedVS, err := exposer.csiSnapshotClient.VolumeSnapshots(ownerObject.Namespace).Get(context.Background(), ownerObject.Name, metav1.GetOptions{})
				assert.NoError(t, err)

				expectedVSC, err := exposer.csiSnapshotClient.VolumeSnapshotContents().Get(context.Background(), ownerObject.Name, metav1.GetOptions{})
				assert.NoError(t, err)

				assert.Equal(t, expectedVS.Annotations, vsObject.Annotations)
				assert.Equal(t, *expectedVS.Spec.VolumeSnapshotClassName, *vsObject.Spec.VolumeSnapshotClassName)
				assert.Equal(t, expectedVSC.Name, *expectedVS.Spec.Source.VolumeSnapshotContentName)

				assert.Equal(t, expectedVSC.Annotations, vscObj.Annotations)
				assert.Equal(t, expectedVSC.Spec.DeletionPolicy, vscObj.Spec.DeletionPolicy)
				assert.Equal(t, expectedVSC.Spec.Driver, vscObj.Spec.Driver)
				assert.Equal(t, *expectedVSC.Spec.VolumeSnapshotClassName, *vscObj.Spec.VolumeSnapshotClassName)
				assert.Equal(t, *expectedVSC.Spec.SourceVolumeMode, *vscObj.Spec.SourceVolumeMode)

				if test.expectedVolumeSize != nil {
					assert.Equal(t, *test.expectedVolumeSize, backupPVC.Spec.Resources.Requests[corev1.ResourceStorage])
				} else {
					assert.Equal(t, *resource.NewQuantity(restoreSize, ""), backupPVC.Spec.Resources.Requests[corev1.ResourceStorage])
				}

				if test.expectedReadOnlyPVC {
					gotReadOnlyAccessMode := false
					for _, accessMode := range backupPVC.Spec.AccessModes {
						if accessMode == corev1.ReadOnlyMany {
							gotReadOnlyAccessMode = true
						}
					}
					assert.Equal(t, test.expectedReadOnlyPVC, gotReadOnlyAccessMode)
				}

				if test.expectedBackupPVCStorageClass != "" {
					assert.Equal(t, test.expectedBackupPVCStorageClass, *backupPVC.Spec.StorageClassName)
				}
			} else {
				assert.EqualError(t, err, test.err)
			}
		})
	}
}

func TestGetExpose(t *testing.T) {
	backup := &velerov1.Backup{
		TypeMeta: metav1.TypeMeta{
			APIVersion: velerov1.SchemeGroupVersion.String(),
			Kind:       "Backup",
		},
		ObjectMeta: metav1.ObjectMeta{
			Namespace: velerov1.DefaultNamespace,
			Name:      "fake-backup",
			UID:       "fake-uid",
		},
	}

	backupPod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: backup.Namespace,
			Name:      backup.Name,
		},
		Spec: corev1.PodSpec{
			Volumes: []corev1.Volume{
				{
					Name: "fake-volume",
				},
				{
					Name: "fake-volume-2",
				},
				{
					Name: string(backup.UID),
				},
			},
		},
	}

	backupPodWithoutVolume := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: backup.Namespace,
			Name:      backup.Name,
		},
		Spec: corev1.PodSpec{
			Volumes: []corev1.Volume{
				{
					Name: "fake-volume-1",
				},
				{
					Name: "fake-volume-2",
				},
			},
		},
	}

	backupPVC := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: backup.Namespace,
			Name:      backup.Name,
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			VolumeName: "fake-pv-name",
		},
	}

	backupPV := &corev1.PersistentVolume{
		ObjectMeta: metav1.ObjectMeta{
			Name: "fake-pv-name",
		},
	}

	scheme := runtime.NewScheme()
	corev1.AddToScheme(scheme)

	tests := []struct {
		name            string
		kubeClientObj   []runtime.Object
		ownerBackup     *velerov1.Backup
		exposeWaitParam CSISnapshotExposeWaitParam
		Timeout         time.Duration
		err             string
		expectedResult  *ExposeResult
	}{
		{
			name:        "backup pod is not found",
			ownerBackup: backup,
			exposeWaitParam: CSISnapshotExposeWaitParam{
				NodeName: "fake-node",
			},
		},
		{
			name:        "wait pvc bound fail",
			ownerBackup: backup,
			exposeWaitParam: CSISnapshotExposeWaitParam{
				NodeName: "fake-node",
			},
			kubeClientObj: []runtime.Object{
				backupPod,
			},
			Timeout: time.Second,
			err:     "error to wait backup PVC bound, fake-backup: error to wait for rediness of PVC: error to get pvc velero/fake-backup: persistentvolumeclaims \"fake-backup\" not found",
		},
		{
			name:        "backup volume not found in pod",
			ownerBackup: backup,
			exposeWaitParam: CSISnapshotExposeWaitParam{
				NodeName: "fake-node",
			},
			kubeClientObj: []runtime.Object{
				backupPodWithoutVolume,
				backupPVC,
				backupPV,
			},
			Timeout: time.Second,
			err:     "backup pod fake-backup doesn't have the expected backup volume",
		},
		{
			name:        "succeed",
			ownerBackup: backup,
			exposeWaitParam: CSISnapshotExposeWaitParam{
				NodeName: "fake-node",
			},
			kubeClientObj: []runtime.Object{
				backupPod,
				backupPVC,
				backupPV,
			},
			Timeout: time.Second,
			expectedResult: &ExposeResult{
				ByPod: ExposeByPod{
					HostingPod: backupPod,
					VolumeName: string(backup.UID),
				},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fakeKubeClient := fake.NewSimpleClientset(test.kubeClientObj...)

			fakeClientBuilder := clientFake.NewClientBuilder()
			fakeClientBuilder = fakeClientBuilder.WithScheme(scheme)

			fakeClient := fakeClientBuilder.WithRuntimeObjects(test.kubeClientObj...).Build()

			exposer := csiSnapshotExposer{
				kubeClient: fakeKubeClient,
				log:        velerotest.NewLogger(),
			}

			var ownerObject corev1.ObjectReference
			if test.ownerBackup != nil {
				ownerObject = corev1.ObjectReference{
					Kind:       test.ownerBackup.Kind,
					Namespace:  test.ownerBackup.Namespace,
					Name:       test.ownerBackup.Name,
					UID:        test.ownerBackup.UID,
					APIVersion: test.ownerBackup.APIVersion,
				}
			}

			test.exposeWaitParam.NodeClient = fakeClient

			result, err := exposer.GetExposed(context.Background(), ownerObject, test.Timeout, &test.exposeWaitParam)
			if test.err == "" {
				assert.NoError(t, err)

				if test.expectedResult == nil {
					assert.Nil(t, result)
				} else {
					assert.NoError(t, err)
					assert.Equal(t, test.expectedResult.ByPod.VolumeName, result.ByPod.VolumeName)
					assert.Equal(t, test.expectedResult.ByPod.HostingPod.Name, result.ByPod.HostingPod.Name)
				}
			} else {
				assert.EqualError(t, err, test.err)
			}
		})
	}
}

func TestPeekExpose(t *testing.T) {
	backup := &velerov1.Backup{
		TypeMeta: metav1.TypeMeta{
			APIVersion: velerov1.SchemeGroupVersion.String(),
			Kind:       "Backup",
		},
		ObjectMeta: metav1.ObjectMeta{
			Namespace: velerov1.DefaultNamespace,
			Name:      "fake-backup",
			UID:       "fake-uid",
		},
	}

	backupPodUrecoverable := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: backup.Namespace,
			Name:      backup.Name,
		},
		Status: corev1.PodStatus{
			Phase: corev1.PodFailed,
		},
	}

	backupPod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: backup.Namespace,
			Name:      backup.Name,
		},
	}

	scheme := runtime.NewScheme()
	corev1.AddToScheme(scheme)

	tests := []struct {
		name          string
		kubeClientObj []runtime.Object
		ownerBackup   *velerov1.Backup
		err           string
	}{
		{
			name:        "backup pod is not found",
			ownerBackup: backup,
		},
		{
			name:        "pod is unrecoverable",
			ownerBackup: backup,
			kubeClientObj: []runtime.Object{
				backupPodUrecoverable,
			},
			err: "Pod is in abnormal state [Failed], message []",
		},
		{
			name:        "succeed",
			ownerBackup: backup,
			kubeClientObj: []runtime.Object{
				backupPod,
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fakeKubeClient := fake.NewSimpleClientset(test.kubeClientObj...)

			exposer := csiSnapshotExposer{
				kubeClient: fakeKubeClient,
				log:        velerotest.NewLogger(),
			}

			var ownerObject corev1.ObjectReference
			if test.ownerBackup != nil {
				ownerObject = corev1.ObjectReference{
					Kind:       test.ownerBackup.Kind,
					Namespace:  test.ownerBackup.Namespace,
					Name:       test.ownerBackup.Name,
					UID:        test.ownerBackup.UID,
					APIVersion: test.ownerBackup.APIVersion,
				}
			}

			err := exposer.PeekExposed(context.Background(), ownerObject)
			if test.err == "" {
				assert.NoError(t, err)
			} else {
				assert.EqualError(t, err, test.err)
			}
		})
	}
}

func Test_csiSnapshotExposer_createBackupPVC(t *testing.T) {
	backup := &velerov1.Backup{
		TypeMeta: metav1.TypeMeta{
			APIVersion: velerov1.SchemeGroupVersion.String(),
			Kind:       "Backup",
		},
		ObjectMeta: metav1.ObjectMeta{
			Namespace: velerov1.DefaultNamespace,
			Name:      "fake-backup",
			UID:       "fake-uid",
		},
	}

	dataSource := &corev1.TypedLocalObjectReference{
		APIGroup: &snapshotv1api.SchemeGroupVersion.Group,
		Kind:     "VolumeSnapshot",
		Name:     "fake-snapshot",
	}
	volumeMode := corev1.PersistentVolumeFilesystem

	backupPVC := corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: velerov1.DefaultNamespace,
			Name:      "fake-backup",
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: backup.APIVersion,
					Kind:       backup.Kind,
					Name:       backup.Name,
					UID:        backup.UID,
					Controller: pointer.BoolPtr(true),
				},
			},
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes: []corev1.PersistentVolumeAccessMode{
				corev1.ReadWriteOnce,
			},
			VolumeMode:       &volumeMode,
			DataSource:       dataSource,
			DataSourceRef:    nil,
			StorageClassName: pointer.String("fake-storage-class"),
			Resources: corev1.VolumeResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceStorage: resource.MustParse("1Gi"),
				},
			},
		},
	}

	backupPVCReadOnly := corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: velerov1.DefaultNamespace,
			Name:      "fake-backup",
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: backup.APIVersion,
					Kind:       backup.Kind,
					Name:       backup.Name,
					UID:        backup.UID,
					Controller: pointer.BoolPtr(true),
				},
			},
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes: []corev1.PersistentVolumeAccessMode{
				corev1.ReadOnlyMany,
			},
			VolumeMode:       &volumeMode,
			DataSource:       dataSource,
			DataSourceRef:    nil,
			StorageClassName: pointer.String("fake-storage-class"),
			Resources: corev1.VolumeResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceStorage: resource.MustParse("1Gi"),
				},
			},
		},
	}

	tests := []struct {
		name              string
		ownerBackup       *velerov1.Backup
		backupVS          string
		storageClass      string
		accessMode        string
		resource          resource.Quantity
		readOnly          bool
		kubeClientObj     []runtime.Object
		snapshotClientObj []runtime.Object
		want              *corev1.PersistentVolumeClaim
		wantErr           assert.ErrorAssertionFunc
	}{
		{
			name:         "backupPVC gets created successfully with parameters from source PVC",
			ownerBackup:  backup,
			backupVS:     "fake-snapshot",
			storageClass: "fake-storage-class",
			accessMode:   AccessModeFileSystem,
			resource:     resource.MustParse("1Gi"),
			readOnly:     false,
			want:         &backupPVC,
			wantErr:      assert.NoError,
		},
		{
			name:         "backupPVC gets created successfully with parameters from source PVC but accessMode from backupPVC Config as read only",
			ownerBackup:  backup,
			backupVS:     "fake-snapshot",
			storageClass: "fake-storage-class",
			accessMode:   AccessModeFileSystem,
			resource:     resource.MustParse("1Gi"),
			readOnly:     true,
			want:         &backupPVCReadOnly,
			wantErr:      assert.NoError,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fakeKubeClient := fake.NewSimpleClientset(tt.kubeClientObj...)
			fakeSnapshotClient := snapshotFake.NewSimpleClientset(tt.snapshotClientObj...)
			e := &csiSnapshotExposer{
				kubeClient:        fakeKubeClient,
				csiSnapshotClient: fakeSnapshotClient.SnapshotV1(),
				log:               velerotest.NewLogger(),
			}
			var ownerObject corev1.ObjectReference
			if tt.ownerBackup != nil {
				ownerObject = corev1.ObjectReference{
					Kind:       tt.ownerBackup.Kind,
					Namespace:  tt.ownerBackup.Namespace,
					Name:       tt.ownerBackup.Name,
					UID:        tt.ownerBackup.UID,
					APIVersion: tt.ownerBackup.APIVersion,
				}
			}
			got, err := e.createBackupPVC(context.Background(), ownerObject, tt.backupVS, tt.storageClass, tt.accessMode, tt.resource, tt.readOnly)
			if !tt.wantErr(t, err, fmt.Sprintf("createBackupPVC(%v, %v, %v, %v, %v, %v)", ownerObject, tt.backupVS, tt.storageClass, tt.accessMode, tt.resource, tt.readOnly)) {
				return
			}
			assert.Equalf(t, tt.want, got, "createBackupPVC(%v, %v, %v, %v, %v, %v)", ownerObject, tt.backupVS, tt.storageClass, tt.accessMode, tt.resource, tt.readOnly)
		})
	}
}
