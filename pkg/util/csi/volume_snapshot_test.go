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

package csi

import (
	"context"
	"errors"
	"testing"
	"time"

	snapshotv1api "github.com/kubernetes-csi/external-snapshotter/client/v4/apis/volumesnapshot/v1"
	snapshotFake "github.com/kubernetes-csi/external-snapshotter/client/v4/clientset/versioned/fake"
	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientTesting "k8s.io/client-go/testing"

	"github.com/vmware-tanzu/velero/pkg/util/boolptr"
	"github.com/vmware-tanzu/velero/pkg/util/stringptr"

	velerotest "github.com/vmware-tanzu/velero/pkg/test"
)

type reactor struct {
	verb        string
	resource    string
	reactorFunc clientTesting.ReactionFunc
}

func TestWaitVolumeSnapshotReady(t *testing.T) {
	vscName := "fake-vsc"
	vsObj := &snapshotv1api.VolumeSnapshot{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "fake-vs",
			Namespace: "fake-ns",
		},
		Status: &snapshotv1api.VolumeSnapshotStatus{
			BoundVolumeSnapshotContentName: &vscName,
			ReadyToUse:                     boolptr.True(),
			RestoreSize:                    &resource.Quantity{},
		},
	}

	errMessage := "fake-snapshot-creation-error"

	tests := []struct {
		name      string
		clientObj []runtime.Object
		vsName    string
		namespace string
		err       string
		expect    *snapshotv1api.VolumeSnapshot
	}{
		{
			name:      "get vs error",
			vsName:    "fake-vs-1",
			namespace: "fake-ns-1",
			err:       "error to get volumesnapshot fake-ns-1/fake-vs-1: volumesnapshots.snapshot.storage.k8s.io \"fake-vs-1\" not found",
		},
		{
			name:      "vs status is nil",
			vsName:    "fake-vs",
			namespace: "fake-ns",
			clientObj: []runtime.Object{
				&snapshotv1api.VolumeSnapshot{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "fake-vs",
						Namespace: "fake-ns",
					},
				},
			},
			err: "timed out waiting for the condition",
		},
		{
			name:      "vsc is nil in status",
			vsName:    "fake-vs",
			namespace: "fake-ns",
			clientObj: []runtime.Object{
				&snapshotv1api.VolumeSnapshot{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "fake-vs",
						Namespace: "fake-ns",
					},
					Status: &snapshotv1api.VolumeSnapshotStatus{},
				},
			},
			err: "timed out waiting for the condition",
		},
		{
			name:      "ready to use is nil in status",
			vsName:    "fake-vs",
			namespace: "fake-ns",
			clientObj: []runtime.Object{
				&snapshotv1api.VolumeSnapshot{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "fake-vs",
						Namespace: "fake-ns",
					},
					Status: &snapshotv1api.VolumeSnapshotStatus{
						BoundVolumeSnapshotContentName: &vscName,
					},
				},
			},
			err: "timed out waiting for the condition",
		},
		{
			name:      "ready to use is false",
			vsName:    "fake-vs",
			namespace: "fake-ns",
			clientObj: []runtime.Object{
				&snapshotv1api.VolumeSnapshot{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "fake-vs",
						Namespace: "fake-ns",
					},
					Status: &snapshotv1api.VolumeSnapshotStatus{
						BoundVolumeSnapshotContentName: &vscName,
						ReadyToUse:                     boolptr.False(),
					},
				},
			},
			err: "timed out waiting for the condition",
		},
		{
			name:      "snapshot creation error with message",
			vsName:    "fake-vs",
			namespace: "fake-ns",
			clientObj: []runtime.Object{
				&snapshotv1api.VolumeSnapshot{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "fake-vs",
						Namespace: "fake-ns",
					},
					Status: &snapshotv1api.VolumeSnapshotStatus{
						Error: &snapshotv1api.VolumeSnapshotError{
							Message: &errMessage,
						},
					},
				},
			},
			err: "volume snapshot creation error fake-snapshot-creation-error",
		},
		{
			name:      "snapshot creation error without message",
			vsName:    "fake-vs",
			namespace: "fake-ns",
			clientObj: []runtime.Object{
				&snapshotv1api.VolumeSnapshot{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "fake-vs",
						Namespace: "fake-ns",
					},
					Status: &snapshotv1api.VolumeSnapshotStatus{
						Error: &snapshotv1api.VolumeSnapshotError{},
					},
				},
			},
			err: "volume snapshot creation error " + stringptr.NilString,
		},
		{
			name:      "success",
			vsName:    "fake-vs",
			namespace: "fake-ns",
			clientObj: []runtime.Object{
				vsObj,
			},
			expect: vsObj,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fakeSnapshotClient := snapshotFake.NewSimpleClientset(test.clientObj...)

			vs, err := WaitVolumeSnapshotReady(context.Background(), fakeSnapshotClient.SnapshotV1(), test.vsName, test.namespace, time.Millisecond)
			if err != nil {
				assert.EqualError(t, err, test.err)
			} else {
				assert.NoError(t, err)
			}

			assert.Equal(t, test.expect, vs)
		})
	}
}

func TestGetVolumeSnapshotContentForVolumeSnapshot(t *testing.T) {
	vscName := "fake-vsc"
	vsObj := &snapshotv1api.VolumeSnapshot{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "fake-vs",
			Namespace: "fake-ns",
		},
		Status: &snapshotv1api.VolumeSnapshotStatus{
			BoundVolumeSnapshotContentName: &vscName,
			ReadyToUse:                     boolptr.True(),
			RestoreSize:                    &resource.Quantity{},
		},
	}

	vscObj := &snapshotv1api.VolumeSnapshotContent{
		ObjectMeta: metav1.ObjectMeta{
			Name: "fake-vsc",
		},
	}

	tests := []struct {
		name        string
		snapshotObj *snapshotv1api.VolumeSnapshot
		clientObj   []runtime.Object
		vsName      string
		namespace   string
		err         string
		expect      *snapshotv1api.VolumeSnapshotContent
	}{
		{
			name:      "vs status is nil",
			vsName:    "fake-vs",
			namespace: "fake-ns",
			snapshotObj: &snapshotv1api.VolumeSnapshot{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "fake-vs",
					Namespace: "fake-ns",
				},
			},
			err: "invalid snapshot info in volume snapshot fake-vs",
		},
		{
			name:      "vsc is nil in status",
			vsName:    "fake-vs",
			namespace: "fake-ns",
			snapshotObj: &snapshotv1api.VolumeSnapshot{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "fake-vs",
					Namespace: "fake-ns",
				},
			},
			err: "invalid snapshot info in volume snapshot fake-vs",
		},
		{
			name:        "get vsc fail",
			vsName:      "fake-vs",
			namespace:   "fake-ns",
			snapshotObj: vsObj,
			err:         "error getting volume snapshot content from API: volumesnapshotcontents.snapshot.storage.k8s.io \"fake-vsc\" not found",
		},
		{
			name:        "success",
			vsName:      "fake-vs",
			namespace:   "fake-ns",
			snapshotObj: vsObj,
			clientObj:   []runtime.Object{vscObj},
			expect:      vscObj,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fakeSnapshotClient := snapshotFake.NewSimpleClientset(test.clientObj...)

			vs, err := GetVolumeSnapshotContentForVolumeSnapshot(test.snapshotObj, fakeSnapshotClient.SnapshotV1())
			if err != nil {
				assert.EqualError(t, err, test.err)
			} else {
				assert.NoError(t, err)
			}

			assert.Equal(t, test.expect, vs)
		})
	}
}

func TestEnsureDeleteVS(t *testing.T) {
	vsObj := &snapshotv1api.VolumeSnapshot{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "fake-vs",
			Namespace: "fake-ns",
		},
	}

	tests := []struct {
		name      string
		clientObj []runtime.Object
		vsName    string
		namespace string
		reactors  []reactor
		err       string
	}{
		{
			name:      "delete fail",
			vsName:    "fake-vs",
			namespace: "fake-ns",
			err:       "error to delete volume snapshot: volumesnapshots.snapshot.storage.k8s.io \"fake-vs\" not found",
		},
		{
			name:      "wait fail",
			vsName:    "fake-vs",
			namespace: "fake-ns",
			clientObj: []runtime.Object{vsObj},
			reactors: []reactor{
				{
					verb:     "get",
					resource: "volumesnapshots",
					reactorFunc: func(action clientTesting.Action) (handled bool, ret runtime.Object, err error) {
						return true, nil, errors.New("fake-get-error")
					},
				},
			},
			err: "error to assure VolumeSnapshot is deleted, fake-vs: error to get VolumeSnapshot fake-vs: fake-get-error",
		},
		{
			name:      "success",
			vsName:    "fake-vs",
			namespace: "fake-ns",
			clientObj: []runtime.Object{vsObj},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fakeSnapshotClient := snapshotFake.NewSimpleClientset(test.clientObj...)

			for _, reactor := range test.reactors {
				fakeSnapshotClient.Fake.PrependReactor(reactor.verb, reactor.resource, reactor.reactorFunc)
			}

			err := EnsureDeleteVS(context.Background(), fakeSnapshotClient.SnapshotV1(), test.vsName, test.namespace, time.Millisecond)
			if err != nil {
				assert.EqualError(t, err, test.err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestEnsureDeleteVSC(t *testing.T) {
	vscObj := &snapshotv1api.VolumeSnapshotContent{
		ObjectMeta: metav1.ObjectMeta{
			Name: "fake-vsc",
		},
	}

	tests := []struct {
		name      string
		clientObj []runtime.Object
		reactors  []reactor
		vscName   string
		err       string
	}{
		{
			name:    "delete fail",
			vscName: "fake-vsc",
			err:     "error to delete volume snapshot content: volumesnapshotcontents.snapshot.storage.k8s.io \"fake-vsc\" not found",
		},
		{
			name:      "wait fail",
			vscName:   "fake-vsc",
			clientObj: []runtime.Object{vscObj},
			reactors: []reactor{
				{
					verb:     "get",
					resource: "volumesnapshotcontents",
					reactorFunc: func(action clientTesting.Action) (handled bool, ret runtime.Object, err error) {
						return true, nil, errors.New("fake-get-error")
					},
				},
			},
			err: "error to assure VolumeSnapshotContent is deleted, fake-vsc: error to get VolumeSnapshotContent fake-vsc: fake-get-error",
		},
		{
			name:      "success",
			vscName:   "fake-vsc",
			clientObj: []runtime.Object{vscObj},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fakeSnapshotClient := snapshotFake.NewSimpleClientset(test.clientObj...)

			for _, reactor := range test.reactors {
				fakeSnapshotClient.Fake.PrependReactor(reactor.verb, reactor.resource, reactor.reactorFunc)
			}

			err := EnsureDeleteVSC(context.Background(), fakeSnapshotClient.SnapshotV1(), test.vscName, time.Millisecond)
			if err != nil {
				assert.EqualError(t, err, test.err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestDeleteVolumeSnapshotContentIfAny(t *testing.T) {
	tests := []struct {
		name       string
		clientObj  []runtime.Object
		reactors   []reactor
		vscName    string
		logMessage string
		logLevel   string
		logError   string
	}{
		{
			name:       "vsc not exist",
			vscName:    "fake-vsc",
			logMessage: "Abort deleting VSC, it doesn't exist fake-vsc",
			logLevel:   "level=debug",
		},
		{
			name:    "deleete fail",
			vscName: "fake-vsc",
			reactors: []reactor{
				{
					verb:     "delete",
					resource: "volumesnapshotcontents",
					reactorFunc: func(action clientTesting.Action) (handled bool, ret runtime.Object, err error) {
						return true, nil, errors.New("fake-delete-error")
					},
				},
			},
			logMessage: "Failed to delete volume snapshot content fake-vsc",
			logLevel:   "level=error",
			logError:   "error=fake-delete-error",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fakeSnapshotClient := snapshotFake.NewSimpleClientset(test.clientObj...)

			for _, reactor := range test.reactors {
				fakeSnapshotClient.Fake.PrependReactor(reactor.verb, reactor.resource, reactor.reactorFunc)
			}

			logMessage := ""

			DeleteVolumeSnapshotContentIfAny(context.Background(), fakeSnapshotClient.SnapshotV1(), test.vscName, velerotest.NewSingleLogger(&logMessage))

			if len(test.logMessage) > 0 {
				assert.Contains(t, logMessage, test.logMessage)
			}

			if len(test.logLevel) > 0 {
				assert.Contains(t, logMessage, test.logLevel)
			}

			if len(test.logError) > 0 {
				assert.Contains(t, logMessage, test.logError)
			}
		})
	}
}

func TestDeleteVolumeSnapshotIfAny(t *testing.T) {
	tests := []struct {
		name        string
		clientObj   []runtime.Object
		reactors    []reactor
		vsName      string
		vsNamespace string
		logMessage  string
		logLevel    string
		logError    string
	}{
		{
			name:        "vs not exist",
			vsName:      "fake-vs",
			vsNamespace: "fake-ns",
			logMessage:  "Abort deleting volume snapshot, it doesn't exist fake-ns/fake-vs",
			logLevel:    "level=debug",
		},
		{
			name:        "delete fail",
			vsName:      "fake-vs",
			vsNamespace: "fake-ns",
			reactors: []reactor{
				{
					verb:     "delete",
					resource: "volumesnapshots",
					reactorFunc: func(action clientTesting.Action) (handled bool, ret runtime.Object, err error) {
						return true, nil, errors.New("fake-delete-error")
					},
				},
			},
			logMessage: "Failed to delete volume snapshot fake-ns/fake-vs",
			logLevel:   "level=error",
			logError:   "error=fake-delete-error",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fakeSnapshotClient := snapshotFake.NewSimpleClientset(test.clientObj...)

			for _, reactor := range test.reactors {
				fakeSnapshotClient.Fake.PrependReactor(reactor.verb, reactor.resource, reactor.reactorFunc)
			}

			logMessage := ""

			DeleteVolumeSnapshotIfAny(context.Background(), fakeSnapshotClient.SnapshotV1(), test.vsName, test.vsNamespace, velerotest.NewSingleLogger(&logMessage))

			if len(test.logMessage) > 0 {
				assert.Contains(t, logMessage, test.logMessage)
			}

			if len(test.logLevel) > 0 {
				assert.Contains(t, logMessage, test.logLevel)
			}

			if len(test.logError) > 0 {
				assert.Contains(t, logMessage, test.logError)
			}
		})
	}
}

func TestRetainVSC(t *testing.T) {
	vscObj := &snapshotv1api.VolumeSnapshotContent{
		ObjectMeta: metav1.ObjectMeta{
			Name: "fake-vsc",
		},
	}

	tests := []struct {
		name      string
		clientObj []runtime.Object
		reactors  []reactor
		vsc       *snapshotv1api.VolumeSnapshotContent
		updated   *snapshotv1api.VolumeSnapshotContent
		err       string
	}{
		{
			name: "already retained",
			vsc: &snapshotv1api.VolumeSnapshotContent{
				ObjectMeta: metav1.ObjectMeta{
					Name: "fake-vsc",
				},
				Spec: snapshotv1api.VolumeSnapshotContentSpec{
					DeletionPolicy: snapshotv1api.VolumeSnapshotContentRetain,
				},
			},
		},
		{
			name: "path vsc fail",
			vsc: &snapshotv1api.VolumeSnapshotContent{
				ObjectMeta: metav1.ObjectMeta{
					Name: "fake-vsc",
				},
				Spec: snapshotv1api.VolumeSnapshotContentSpec{
					DeletionPolicy: snapshotv1api.VolumeSnapshotContentDelete,
				},
			},
			reactors: []reactor{
				{
					verb:     "patch",
					resource: "volumesnapshotcontents",
					reactorFunc: func(action clientTesting.Action) (handled bool, ret runtime.Object, err error) {
						return true, nil, errors.New("fake-patch-error")
					},
				},
			},
			err: "error patching VSC: fake-patch-error",
		},
		{
			name: "success",
			vsc: &snapshotv1api.VolumeSnapshotContent{
				ObjectMeta: metav1.ObjectMeta{
					Name: "fake-vsc",
				},
				Spec: snapshotv1api.VolumeSnapshotContentSpec{
					DeletionPolicy: snapshotv1api.VolumeSnapshotContentDelete,
				},
			},
			clientObj: []runtime.Object{vscObj},
			updated: &snapshotv1api.VolumeSnapshotContent{
				ObjectMeta: metav1.ObjectMeta{
					Name: "fake-vsc",
				},
				Spec: snapshotv1api.VolumeSnapshotContentSpec{
					DeletionPolicy: snapshotv1api.VolumeSnapshotContentRetain,
				},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fakeSnapshotClient := snapshotFake.NewSimpleClientset(test.clientObj...)

			for _, reactor := range test.reactors {
				fakeSnapshotClient.Fake.PrependReactor(reactor.verb, reactor.resource, reactor.reactorFunc)
			}

			returned, err := RetainVSC(context.Background(), fakeSnapshotClient.SnapshotV1(), test.vsc)

			if len(test.err) == 0 {
				assert.NoError(t, err)
			} else {
				assert.EqualError(t, err, test.err)
			}

			if test.updated != nil {
				assert.Equal(t, *test.updated, *returned)
			} else {
				assert.Nil(t, returned)
			}
		})
	}
}
