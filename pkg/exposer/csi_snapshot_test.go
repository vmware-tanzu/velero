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
	"testing"
	"time"

	snapshotv1api "github.com/kubernetes-csi/external-snapshotter/client/v4/apis/volumesnapshot/v1"
	snapshotFake "github.com/kubernetes-csi/external-snapshotter/client/v4/clientset/versioned/fake"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	clientTesting "k8s.io/client-go/testing"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"

	velerov1 "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
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
			RestoreSize:                    &resource.Quantity{},
		},
	}

	var restoreSize int64
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
		Spec: appsv1.DaemonSetSpec{},
	}

	tests := []struct {
		name              string
		snapshotClientObj []runtime.Object
		kubeClientObj     []runtime.Object
		ownerBackup       *velerov1.Backup
		exposeParam       CSISnapshotExposeParam
		snapReactors      []reactor
		kubeReactors      []reactor
		err               string
	}{
		{
			name:        "wait vs ready fail",
			ownerBackup: backup,
			exposeParam: CSISnapshotExposeParam{
				SnapshotName: "fake-vs",
				Timeout:      time.Millisecond,
			},
			err: "error wait volume snapshot ready: error to get volumesnapshot /fake-vs: volumesnapshots.snapshot.storage.k8s.io \"fake-vs\" not found",
		},
		{
			name:        "get vsc fail",
			ownerBackup: backup,
			exposeParam: CSISnapshotExposeParam{
				SnapshotName:    "fake-vs",
				SourceNamespace: "fake-ns",
				Timeout:         time.Millisecond,
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
				SnapshotName:    "fake-vs",
				SourceNamespace: "fake-ns",
				Timeout:         time.Millisecond,
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
				SnapshotName:    "fake-vs",
				SourceNamespace: "fake-ns",
				Timeout:         time.Millisecond,
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
				SnapshotName:    "fake-vs",
				SourceNamespace: "fake-ns",
				Timeout:         time.Millisecond,
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
				SnapshotName:    "fake-vs",
				SourceNamespace: "fake-ns",
				Timeout:         time.Millisecond,
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
				SnapshotName:    "fake-vs",
				SourceNamespace: "fake-ns",
				Timeout:         time.Millisecond,
				AccessMode:      AccessModeFileSystem,
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
				SnapshotName:    "fake-vs",
				SourceNamespace: "fake-ns",
				AccessMode:      AccessModeFileSystem,
				Timeout:         time.Millisecond,
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
				SnapshotName:    "fake-vs",
				SourceNamespace: "fake-ns",
				AccessMode:      AccessModeFileSystem,
				Timeout:         time.Millisecond,
			},
			snapshotClientObj: []runtime.Object{
				vsObject,
				vscObj,
			},
			kubeClientObj: []runtime.Object{
				daemonSet,
			},
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

				_, err = exposer.kubeClient.CoreV1().PersistentVolumeClaims(ownerObject.Namespace).Get(context.Background(), ownerObject.Name, metav1.GetOptions{})
				assert.NoError(t, err)

				expectedVS, err := exposer.csiSnapshotClient.VolumeSnapshots(ownerObject.Namespace).Get(context.Background(), ownerObject.Name, metav1.GetOptions{})
				assert.NoError(t, err)

				expectedVSC, err := exposer.csiSnapshotClient.VolumeSnapshotContents().Get(context.Background(), ownerObject.Name, metav1.GetOptions{})
				assert.NoError(t, err)

				assert.Equal(t, expectedVS.Annotations, vsObject.Annotations)
				assert.Equal(t, *expectedVS.Spec.VolumeSnapshotClassName, *vsObject.Spec.VolumeSnapshotClassName)
				assert.Equal(t, *expectedVS.Spec.Source.VolumeSnapshotContentName, expectedVSC.Name)

				assert.Equal(t, expectedVSC.Annotations, vscObj.Annotations)
				assert.Equal(t, expectedVSC.Spec.DeletionPolicy, vscObj.Spec.DeletionPolicy)
				assert.Equal(t, expectedVSC.Spec.Driver, vscObj.Spec.Driver)
				assert.Equal(t, *expectedVSC.Spec.VolumeSnapshotClassName, *vscObj.Spec.VolumeSnapshotClassName)
			} else {
				assert.EqualError(t, err, test.err)
			}

		})
	}
}
