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

package kube

import (
	"context"
	"testing"
	"time"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"

	corev1api "k8s.io/api/core/v1"
	storagev1api "k8s.io/api/storage/v1"

	clientTesting "k8s.io/client-go/testing"

	velerotest "github.com/vmware-tanzu/velero/pkg/test"
)

type reactor struct {
	verb        string
	resource    string
	reactorFunc clientTesting.ReactionFunc
}

func TestWaitPVCBound(t *testing.T) {
	pvcObject := &corev1api.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "fake-namespace",
			Name:      "fake-pvc",
		},
	}

	pvcObjectBound := &corev1api.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "fake-namespace",
			Name:      "fake-pvc",
		},
		Spec: corev1api.PersistentVolumeClaimSpec{
			VolumeName: "fake-pv",
		},
	}

	pvObj := &corev1api.PersistentVolume{
		ObjectMeta: metav1.ObjectMeta{
			Name: "fake-pv",
		},
	}

	tests := []struct {
		name          string
		pvcName       string
		pvcNamespace  string
		kubeClientObj []runtime.Object
		kubeReactors  []reactor
		expected      *corev1api.PersistentVolume
		err           string
	}{
		{
			name:         "wait pvc error",
			pvcName:      "fake-pvc",
			pvcNamespace: "fake-namespace",
			err:          "error to wait for rediness of PVC: error to get pvc fake-namespace/fake-pvc: persistentvolumeclaims \"fake-pvc\" not found",
		},
		{
			name:         "wait pvc timeout",
			pvcName:      "fake-pvc",
			pvcNamespace: "fake-namespace",
			kubeClientObj: []runtime.Object{
				pvcObject,
			},
			err: "error to wait for rediness of PVC: timed out waiting for the condition",
		},
		{
			name:         "get pv fail",
			pvcName:      "fake-pvc",
			pvcNamespace: "fake-namespace",
			kubeClientObj: []runtime.Object{
				pvcObjectBound,
			},
			err: "error to get PV: persistentvolumes \"fake-pv\" not found",
		},
		{
			name:         "success",
			pvcName:      "fake-pvc",
			pvcNamespace: "fake-namespace",
			kubeClientObj: []runtime.Object{
				pvcObjectBound,
				pvObj,
			},
			expected: pvObj,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fakeKubeClient := fake.NewSimpleClientset(test.kubeClientObj...)

			for _, reactor := range test.kubeReactors {
				fakeKubeClient.Fake.PrependReactor(reactor.verb, reactor.resource, reactor.reactorFunc)
			}

			var kubeClient kubernetes.Interface = fakeKubeClient

			pv, err := WaitPVCBound(context.Background(), kubeClient.CoreV1(), kubeClient.CoreV1(), test.pvcName, test.pvcNamespace, time.Millisecond)

			if err != nil {
				assert.EqualError(t, err, test.err)
			} else {
				assert.NoError(t, err)
			}

			assert.Equal(t, test.expected, pv)
		})
	}
}

func TestWaitPVCConsumed(t *testing.T) {
	storageClass := "fake-storage-class"
	bindModeImmediate := storagev1api.VolumeBindingImmediate
	bindModeWait := storagev1api.VolumeBindingWaitForFirstConsumer

	pvcObject := &corev1api.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "fake-namespace",
			Name:      "fake-pvc-1",
		},
	}

	pvcObjectWithSC := &corev1api.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "fake-namespace",
			Name:      "fake-pvc-2",
		},
		Spec: corev1api.PersistentVolumeClaimSpec{
			StorageClassName: &storageClass,
		},
	}

	scObjWithoutBindMode := &storagev1api.StorageClass{
		ObjectMeta: metav1.ObjectMeta{
			Name: "fake-storage-class",
		},
	}

	scObjWaitBind := &storagev1api.StorageClass{
		ObjectMeta: metav1.ObjectMeta{
			Name: "fake-storage-class",
		},
		VolumeBindingMode: &bindModeWait,
	}

	scObjWithImmidateBinding := &storagev1api.StorageClass{
		ObjectMeta: metav1.ObjectMeta{
			Name: "fake-storage-class",
		},
		VolumeBindingMode: &bindModeImmediate,
	}

	pvcObjectWithSCAndAnno := &corev1api.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:   "fake-namespace",
			Name:        "fake-pvc-3",
			Annotations: map[string]string{"volume.kubernetes.io/selected-node": "fake-node-1"},
		},
		Spec: corev1api.PersistentVolumeClaimSpec{
			StorageClassName: &storageClass,
		},
	}

	tests := []struct {
		name          string
		pvcName       string
		pvcNamespace  string
		kubeClientObj []runtime.Object
		kubeReactors  []reactor
		expectedPVC   *corev1api.PersistentVolumeClaim
		selectedNode  string
		err           string
	}{
		{
			name:         "get pvc error",
			pvcName:      "fake-pvc",
			pvcNamespace: "fake-namespace",
			err:          "error to wait for PVC: error to get pvc fake-namespace/fake-pvc: persistentvolumeclaims \"fake-pvc\" not found",
		},
		{
			name:         "success when no sc",
			pvcName:      "fake-pvc-1",
			pvcNamespace: "fake-namespace",
			kubeClientObj: []runtime.Object{
				pvcObject,
			},
			expectedPVC: pvcObject,
		},
		{
			name:         "get sc fail",
			pvcName:      "fake-pvc-2",
			pvcNamespace: "fake-namespace",
			kubeClientObj: []runtime.Object{
				pvcObjectWithSC,
			},
			err: "error to wait for PVC: error to get storage class fake-storage-class: storageclasses.storage.k8s.io \"fake-storage-class\" not found",
		},
		{
			name:         "success on sc without binding mode",
			pvcName:      "fake-pvc-2",
			pvcNamespace: "fake-namespace",
			kubeClientObj: []runtime.Object{
				pvcObjectWithSC,
				scObjWithoutBindMode,
			},
			expectedPVC: pvcObjectWithSC,
		},
		{
			name:         "success on sc without immediate binding mode",
			pvcName:      "fake-pvc-2",
			pvcNamespace: "fake-namespace",
			kubeClientObj: []runtime.Object{
				pvcObjectWithSC,
				scObjWithImmidateBinding,
			},
			expectedPVC: pvcObjectWithSC,
		},
		{
			name:         "pvc annotation miss",
			pvcName:      "fake-pvc-2",
			pvcNamespace: "fake-namespace",
			kubeClientObj: []runtime.Object{
				pvcObjectWithSC,
				scObjWaitBind,
			},
			err: "error to wait for PVC: timed out waiting for the condition",
		},
		{
			name:         "success on sc without wait binding mode",
			pvcName:      "fake-pvc-3",
			pvcNamespace: "fake-namespace",
			kubeClientObj: []runtime.Object{
				pvcObjectWithSCAndAnno,
				scObjWaitBind,
			},
			expectedPVC:  pvcObjectWithSCAndAnno,
			selectedNode: "fake-node-1",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fakeKubeClient := fake.NewSimpleClientset(test.kubeClientObj...)

			for _, reactor := range test.kubeReactors {
				fakeKubeClient.Fake.PrependReactor(reactor.verb, reactor.resource, reactor.reactorFunc)
			}

			var kubeClient kubernetes.Interface = fakeKubeClient

			selectedNode, pvc, err := WaitPVCConsumed(context.Background(), kubeClient.CoreV1(), test.pvcName, test.pvcNamespace, kubeClient.StorageV1(), time.Millisecond)

			if err != nil {
				assert.EqualError(t, err, test.err)
			} else {
				assert.NoError(t, err)
			}

			assert.Equal(t, test.expectedPVC, pvc)
			assert.Equal(t, test.selectedNode, selectedNode)
		})
	}
}

func TestDeletePVCIfAny(t *testing.T) {
	tests := []struct {
		name          string
		pvcName       string
		pvcNamespace  string
		kubeClientObj []runtime.Object
		kubeReactors  []reactor
		logMessage    string
		logLevel      string
		logError      string
	}{
		{
			name:         "get fail",
			pvcName:      "fake-pvc",
			pvcNamespace: "fake-namespace",
			logMessage:   "Abort deleting PVC, it doesn't exist, fake-namespace/fake-pvc",
			logLevel:     "level=debug",
		},
		{
			name:         "delete fail",
			pvcName:      "fake-pvc",
			pvcNamespace: "fake-namespace",
			kubeReactors: []reactor{
				{
					verb:     "delete",
					resource: "persistentvolumeclaims",
					reactorFunc: func(action clientTesting.Action) (handled bool, ret runtime.Object, err error) {
						return true, nil, errors.New("fake-delete-error")
					},
				},
			},
			logMessage: "Failed to delete pvc fake-namespace/fake-pvc",
			logLevel:   "level=error",
			logError:   "error=fake-delete-error",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fakeKubeClient := fake.NewSimpleClientset(test.kubeClientObj...)

			for _, reactor := range test.kubeReactors {
				fakeKubeClient.Fake.PrependReactor(reactor.verb, reactor.resource, reactor.reactorFunc)
			}

			var kubeClient kubernetes.Interface = fakeKubeClient

			logMessage := ""
			DeletePVCIfAny(context.Background(), kubeClient.CoreV1(), test.pvcName, test.pvcNamespace, velerotest.NewSingleLogger(&logMessage))

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

func TestDeletePVIfAny(t *testing.T) {
	tests := []struct {
		name          string
		pvName        string
		kubeClientObj []runtime.Object
		kubeReactors  []reactor
		logMessage    string
		logLevel      string
		logError      string
	}{
		{
			name:       "get fail",
			pvName:     "fake-pv",
			logMessage: "Abort deleting PV, it doesn't exist, fake-pv",
			logLevel:   "level=debug",
		},
		{
			name:   "delete fail",
			pvName: "fake-pv",
			kubeReactors: []reactor{
				{
					verb:     "delete",
					resource: "persistentvolumes",
					reactorFunc: func(action clientTesting.Action) (handled bool, ret runtime.Object, err error) {
						return true, nil, errors.New("fake-delete-error")
					},
				},
			},
			logMessage: "Failed to delete PV fake-pv",
			logLevel:   "level=error",
			logError:   "error=fake-delete-error",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fakeKubeClient := fake.NewSimpleClientset(test.kubeClientObj...)

			for _, reactor := range test.kubeReactors {
				fakeKubeClient.Fake.PrependReactor(reactor.verb, reactor.resource, reactor.reactorFunc)
			}

			var kubeClient kubernetes.Interface = fakeKubeClient

			logMessage := ""
			DeletePVIfAny(context.Background(), kubeClient.CoreV1(), test.pvName, velerotest.NewSingleLogger(&logMessage))

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

func TestEnsureDeletePVC(t *testing.T) {
	pvcObject := &corev1api.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "fake-ns",
			Name:      "fake-pvc",
		},
	}

	tests := []struct {
		name      string
		clientObj []runtime.Object
		pvcName   string
		namespace string
		reactors  []reactor
		err       string
	}{
		{
			name:      "delete fail",
			pvcName:   "fake-pvc",
			namespace: "fake-ns",
			err:       "error to delete pvc fake-pvc: persistentvolumeclaims \"fake-pvc\" not found",
		},
		{
			name:      "wait fail",
			pvcName:   "fake-pvc",
			namespace: "fake-ns",
			clientObj: []runtime.Object{pvcObject},
			reactors: []reactor{
				{
					verb:     "get",
					resource: "persistentvolumeclaims",
					reactorFunc: func(action clientTesting.Action) (handled bool, ret runtime.Object, err error) {
						return true, nil, errors.New("fake-get-error")
					},
				},
			},
			err: "error to retrieve pvc info for fake-pvc: error to get pvc fake-pvc: fake-get-error",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fakeKubeClient := fake.NewSimpleClientset(test.clientObj...)

			for _, reactor := range test.reactors {
				fakeKubeClient.Fake.PrependReactor(reactor.verb, reactor.resource, reactor.reactorFunc)
			}

			var kubeClient kubernetes.Interface = fakeKubeClient

			err := EnsureDeletePVC(context.Background(), kubeClient.CoreV1(), test.pvcName, test.namespace, time.Millisecond)
			if err != nil {
				assert.EqualError(t, err, test.err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestRebindPVC(t *testing.T) {
	pvcObject := &corev1api.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "fake-ns",
			Name:      "fake-pvc",
			Annotations: map[string]string{
				KubeAnnBindCompleted:     "true",
				KubeAnnBoundByController: "true",
			},
		},
	}

	tests := []struct {
		name      string
		clientObj []runtime.Object
		pvc       *corev1api.PersistentVolumeClaim
		pv        string
		reactors  []reactor
		result    *corev1api.PersistentVolumeClaim
		err       string
	}{
		{
			name:      "path fail",
			pvc:       pvcObject,
			pv:        "fake-pv",
			clientObj: []runtime.Object{pvcObject},
			reactors: []reactor{
				{
					verb:     "patch",
					resource: "persistentvolumeclaims",
					reactorFunc: func(action clientTesting.Action) (handled bool, ret runtime.Object, err error) {
						return true, nil, errors.New("fake-patch-error")
					},
				},
			},
			err: "error patching PVC: fake-patch-error",
		},
		{
			name:      "succeed",
			pvc:       pvcObject,
			pv:        "fake-pv",
			clientObj: []runtime.Object{pvcObject},
			result: &corev1api.PersistentVolumeClaim{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "fake-ns",
					Name:      "fake-pvc",
				},
				Spec: corev1api.PersistentVolumeClaimSpec{
					VolumeName: "fake-pv",
				},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fakeKubeClient := fake.NewSimpleClientset(test.clientObj...)

			for _, reactor := range test.reactors {
				fakeKubeClient.Fake.PrependReactor(reactor.verb, reactor.resource, reactor.reactorFunc)
			}

			var kubeClient kubernetes.Interface = fakeKubeClient

			result, err := RebindPVC(context.Background(), kubeClient.CoreV1(), test.pvc, test.pv)
			if err != nil {
				assert.EqualError(t, err, test.err)
			} else {
				assert.NoError(t, err)
			}

			assert.Equal(t, test.result, result)
		})
	}
}

func TestResetPVBinding(t *testing.T) {
	pvObject := &corev1api.PersistentVolume{
		ObjectMeta: metav1.ObjectMeta{
			Name: "fake-pv",
			Annotations: map[string]string{
				KubeAnnBoundByController: "true",
			},
		},
		Spec: corev1api.PersistentVolumeSpec{
			ClaimRef: &corev1api.ObjectReference{
				Kind:      "fake-kind",
				Namespace: "fake-ns",
				Name:      "fake-pvc",
			},
		},
	}

	tests := []struct {
		name      string
		clientObj []runtime.Object
		pv        *corev1api.PersistentVolume
		labels    map[string]string
		reactors  []reactor
		result    *corev1api.PersistentVolume
		err       string
	}{
		{
			name:      "path fail",
			pv:        pvObject,
			clientObj: []runtime.Object{pvObject},
			reactors: []reactor{
				{
					verb:     "patch",
					resource: "persistentvolumes",
					reactorFunc: func(action clientTesting.Action) (handled bool, ret runtime.Object, err error) {
						return true, nil, errors.New("fake-patch-error")
					},
				},
			},
			err: "error patching PV: fake-patch-error",
		},
		{
			name: "succeed",
			pv:   pvObject,
			labels: map[string]string{
				"fake-label-1": "fake-value-1",
				"fake-label-2": "fake-value-2",
			},
			clientObj: []runtime.Object{pvObject},
			result: &corev1api.PersistentVolume{
				ObjectMeta: metav1.ObjectMeta{
					Name: "fake-pv",
					Labels: map[string]string{
						"fake-label-1": "fake-value-1",
						"fake-label-2": "fake-value-2",
					},
				},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fakeKubeClient := fake.NewSimpleClientset(test.clientObj...)

			for _, reactor := range test.reactors {
				fakeKubeClient.Fake.PrependReactor(reactor.verb, reactor.resource, reactor.reactorFunc)
			}

			var kubeClient kubernetes.Interface = fakeKubeClient

			result, err := ResetPVBinding(context.Background(), kubeClient.CoreV1(), test.pv, test.labels)
			if err != nil {
				assert.EqualError(t, err, test.err)
			} else {
				assert.NoError(t, err)
			}

			assert.Equal(t, test.result, result)
		})
	}
}

func TestSetPVReclaimPolicy(t *testing.T) {
	pvObject := &corev1api.PersistentVolume{
		ObjectMeta: metav1.ObjectMeta{
			Name: "fake-pv",
		},
		Spec: corev1api.PersistentVolumeSpec{
			PersistentVolumeReclaimPolicy: corev1api.PersistentVolumeReclaimRetain,
		},
	}

	tests := []struct {
		name      string
		clientObj []runtime.Object
		pv        *corev1api.PersistentVolume
		policy    corev1api.PersistentVolumeReclaimPolicy
		reactors  []reactor
		result    *corev1api.PersistentVolume
		err       string
	}{
		{
			name:   "policy not changed",
			pv:     pvObject,
			policy: corev1api.PersistentVolumeReclaimRetain,
		},
		{
			name:      "path fail",
			pv:        pvObject,
			policy:    corev1api.PersistentVolumeReclaimDelete,
			clientObj: []runtime.Object{pvObject},
			reactors: []reactor{
				{
					verb:     "patch",
					resource: "persistentvolumes",
					reactorFunc: func(action clientTesting.Action) (handled bool, ret runtime.Object, err error) {
						return true, nil, errors.New("fake-patch-error")
					},
				},
			},
			err: "error patching PV: fake-patch-error",
		},
		{
			name:      "succeed",
			pv:        pvObject,
			policy:    corev1api.PersistentVolumeReclaimDelete,
			clientObj: []runtime.Object{pvObject},
			result: &corev1api.PersistentVolume{
				ObjectMeta: metav1.ObjectMeta{
					Name: "fake-pv",
				},
				Spec: corev1api.PersistentVolumeSpec{
					PersistentVolumeReclaimPolicy: corev1api.PersistentVolumeReclaimDelete,
				},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fakeKubeClient := fake.NewSimpleClientset(test.clientObj...)

			for _, reactor := range test.reactors {
				fakeKubeClient.Fake.PrependReactor(reactor.verb, reactor.resource, reactor.reactorFunc)
			}

			var kubeClient kubernetes.Interface = fakeKubeClient

			result, err := SetPVReclaimPolicy(context.Background(), kubeClient.CoreV1(), test.pv, test.policy)
			if err != nil {
				assert.EqualError(t, err, test.err)
			} else {
				assert.NoError(t, err)
			}

			assert.Equal(t, test.result, result)
		})
	}
}

func TestWaitPVBound(t *testing.T) {
	tests := []struct {
		name          string
		pvName        string
		pvcName       string
		pvcNamespace  string
		kubeClientObj []runtime.Object
		kubeReactors  []reactor
		expectedPV    *corev1api.PersistentVolume
		err           string
	}{
		{
			name:   "get pv error",
			pvName: "fake-pv",
			err:    "error to wait for bound of PV: failed to get pv fake-pv: persistentvolumes \"fake-pv\" not found",
		},
		{
			name:   "pvc claimRef miss",
			pvName: "fake-pv",
			kubeClientObj: []runtime.Object{
				&corev1api.PersistentVolume{
					ObjectMeta: metav1.ObjectMeta{
						Name: "fake-pv",
					},
				},
			},
			err: "error to wait for bound of PV: timed out waiting for the condition",
		},
		{
			name:         "pvc claimRef pvc name mismatch",
			pvName:       "fake-pv",
			pvcName:      "fake-pvc",
			pvcNamespace: "fake-ns",
			kubeClientObj: []runtime.Object{
				&corev1api.PersistentVolume{
					ObjectMeta: metav1.ObjectMeta{
						Name: "fake-pv",
					},
					Spec: corev1api.PersistentVolumeSpec{
						ClaimRef: &corev1api.ObjectReference{
							Kind:      "fake-kind",
							Namespace: "fake-ns",
							Name:      "fake-pvc-1",
						},
					},
				},
			},
			err: "error to wait for bound of PV: pv has been bound by unexpected pvc fake-ns/fake-pvc-1",
		},
		{
			name:         "pvc claimRef pvc namespace mismatch",
			pvName:       "fake-pv",
			pvcName:      "fake-pvc",
			pvcNamespace: "fake-ns",
			kubeClientObj: []runtime.Object{
				&corev1api.PersistentVolume{
					ObjectMeta: metav1.ObjectMeta{
						Name: "fake-pv",
					},
					Spec: corev1api.PersistentVolumeSpec{
						ClaimRef: &corev1api.ObjectReference{
							Kind:      "fake-kind",
							Namespace: "fake-ns-1",
							Name:      "fake-pvc",
						},
					},
				},
			},
			err: "error to wait for bound of PV: pv has been bound by unexpected pvc fake-ns-1/fake-pvc",
		},
		{
			name:         "success",
			pvName:       "fake-pv",
			pvcName:      "fake-pvc",
			pvcNamespace: "fake-ns",
			kubeClientObj: []runtime.Object{
				&corev1api.PersistentVolume{
					ObjectMeta: metav1.ObjectMeta{
						Name: "fake-pv",
					},
					Spec: corev1api.PersistentVolumeSpec{
						ClaimRef: &corev1api.ObjectReference{
							Kind:      "fake-kind",
							Name:      "fake-pvc",
							Namespace: "fake-ns",
						},
					},
				},
			},
			expectedPV: &corev1api.PersistentVolume{
				ObjectMeta: metav1.ObjectMeta{
					Name: "fake-pv",
				},
				Spec: corev1api.PersistentVolumeSpec{
					ClaimRef: &corev1api.ObjectReference{
						Kind:      "fake-kind",
						Name:      "fake-pvc",
						Namespace: "fake-ns",
					},
				},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fakeKubeClient := fake.NewSimpleClientset(test.kubeClientObj...)

			for _, reactor := range test.kubeReactors {
				fakeKubeClient.Fake.PrependReactor(reactor.verb, reactor.resource, reactor.reactorFunc)
			}

			var kubeClient kubernetes.Interface = fakeKubeClient

			pv, err := WaitPVBound(context.Background(), kubeClient.CoreV1(), test.pvName, test.pvcName, test.pvcNamespace, time.Millisecond)

			if err != nil {
				assert.EqualError(t, err, test.err)
			} else {
				assert.NoError(t, err)
			}

			assert.Equal(t, test.expectedPV, pv)
		})
	}
}

func TestIsPVCBound(t *testing.T) {
	tests := []struct {
		name   string
		pvc    *corev1api.PersistentVolumeClaim
		expect bool
	}{
		{
			name: "expect bound",
			pvc: &corev1api.PersistentVolumeClaim{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "fake-ns",
					Name:      "fake-pvc",
				},
				Spec: corev1api.PersistentVolumeClaimSpec{
					VolumeName: "fake-volume",
				},
			},
			expect: true,
		},
		{
			name: "expect not bound",
			pvc: &corev1api.PersistentVolumeClaim{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "fake-ns",
					Name:      "fake-pvc",
				},
			},
			expect: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result := IsPVCBound(test.pvc)

			assert.Equal(t, test.expect, result)
		})
	}
}
