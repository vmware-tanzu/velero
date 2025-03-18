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

	"github.com/vmware-tanzu/velero/pkg/builder"
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
			err: "error to wait for rediness of PVC: context deadline exceeded",
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
		name                       string
		pvcName                    string
		pvcNamespace               string
		kubeClientObj              []runtime.Object
		kubeReactors               []reactor
		expectedPVC                *corev1api.PersistentVolumeClaim
		selectedNode               string
		ignoreWaitForFirstConsumer bool
		err                        string
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
			name:                       "success when ignore wait for first consumer",
			pvcName:                    "fake-pvc-2",
			pvcNamespace:               "fake-namespace",
			ignoreWaitForFirstConsumer: true,
			kubeClientObj: []runtime.Object{
				pvcObjectWithSC,
			},
			expectedPVC: pvcObjectWithSC,
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
			err: "error to wait for PVC: context deadline exceeded",
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

			selectedNode, pvc, err := WaitPVCConsumed(context.Background(), kubeClient.CoreV1(), test.pvcName, test.pvcNamespace, kubeClient.StorageV1(), time.Millisecond, test.ignoreWaitForFirstConsumer)

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

	pvcObject := &corev1api.PersistentVolumeClaim{
		TypeMeta: metav1.TypeMeta{
			Kind: "fake-kind-1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "fake-namespace",
			Name:      "fake-pvc",
			Annotations: map[string]string{
				KubeAnnBindCompleted:     "true",
				KubeAnnBoundByController: "true",
			},
		},
	}

	pvcWithVolume := pvcObject.DeepCopy()
	pvcWithVolume.Spec.VolumeName = "fake-pv"

	tests := []struct {
		name          string
		pvcName       string
		pvcNamespace  string
		pvName        string
		kubeClientObj []runtime.Object
		kubeReactors  []reactor
		logMessage    string
		logLevel      string
		logError      string
		ensureTimeout time.Duration
	}{
		{
			name:         "pvc not found",
			pvcName:      "fake-pvc",
			pvcNamespace: "fake-namespace",
			logMessage:   "Abort deleting PV and PVC, for related PVC doesn't exist, fake-namespace/fake-pvc",
			logLevel:     "level=debug",
		},
		{
			name:         "failed to get pvc",
			pvcName:      "fake-pvc",
			pvcNamespace: "fake-namespace",
			kubeReactors: []reactor{
				{
					verb:     "get",
					resource: "persistentvolumeclaims",
					reactorFunc: func(action clientTesting.Action) (handled bool, ret runtime.Object, err error) {
						return true, nil, errors.New("fake-get-error")
					},
				},
			},
			logMessage: "failed to get pvc fake-namespace/fake-pvc with err fake-get-error",
			logLevel:   "level=warning",
		},
		{
			name:         "pvc has no volume name",
			pvcName:      "fake-pvc",
			pvcNamespace: "fake-namespace",
			pvName:       "fake-pv",
			kubeClientObj: []runtime.Object{
				pvcObject,
				pvObject,
			},
			logMessage: "failed to delete PV, for related PVC fake-namespace/fake-pvc has no bind volume name",
			logLevel:   "level=warning",
		},
		{
			name:         "failed to delete pvc",
			pvcName:      "fake-pvc",
			pvcNamespace: "fake-namespace",
			pvName:       "fake-pv",
			kubeReactors: []reactor{
				{
					verb:     "delete",
					resource: "persistentvolumeclaims",
					reactorFunc: func(action clientTesting.Action) (handled bool, ret runtime.Object, err error) {
						return true, nil, errors.New("fake-delete-error")
					},
				},
			},
			kubeClientObj: []runtime.Object{
				pvcWithVolume,
				pvObject,
			},
			logMessage: "failed to delete pvc fake-namespace/fake-pvc with err error to delete pvc fake-pvc: fake-delete-error",
			logLevel:   "level=warning",
		},
		{
			name:         "failed to get pv",
			pvcName:      "fake-pvc",
			pvcNamespace: "fake-namespace",
			pvName:       "fake-pv",
			kubeReactors: []reactor{
				{
					verb:     "get",
					resource: "persistentvolumes",
					reactorFunc: func(action clientTesting.Action) (handled bool, ret runtime.Object, err error) {
						return true, nil, errors.New("fake-get-error")
					},
				},
			},
			kubeClientObj: []runtime.Object{
				pvcWithVolume,
				pvObject,
			},
			logMessage: "failed to delete PV fake-pv with err fake-get-error",
			logLevel:   "level=warning",
		},
		{
			name:         "set reclaim policy fail",
			pvcName:      "fake-pvc",
			pvcNamespace: "fake-namespace",
			pvName:       "fake-pv",
			kubeClientObj: []runtime.Object{
				pvcWithVolume,
				pvObject,
			},
			kubeReactors: []reactor{
				{
					verb:     "patch",
					resource: "persistentvolumes",
					reactorFunc: func(action clientTesting.Action) (handled bool, ret runtime.Object, err error) {
						return true, pvObject, errors.New("fake-patch-error")
					},
				},
			},
			logMessage: "failed to set reclaim policy of PV fake-pv to delete with err error patching PV: fake-patch-error",
			logLevel:   "level=warning",
			logError:   "fake-patch-error",
		},
		{
			name:         "delete pv pvc success",
			pvcName:      "fake-pvc",
			pvcNamespace: "fake-namespace",
			pvName:       "fake-pv",
			kubeClientObj: []runtime.Object{
				pvcWithVolume,
				pvObject,
			},
		},
		{
			name:         "delete pv pvc success but wait fail",
			pvcName:      "fake-pvc",
			pvcNamespace: "fake-namespace",
			pvName:       "fake-pv",
			kubeClientObj: []runtime.Object{
				pvcWithVolume,
				pvObject,
			},
			kubeReactors: []reactor{
				{
					verb:     "delete",
					resource: "persistentvolumeclaims",
					reactorFunc: func(action clientTesting.Action) (handled bool, ret runtime.Object, err error) {
						return true, pvcWithVolume, nil
					},
				},
			},
			ensureTimeout: time.Second,
			logMessage:    "failed to delete pvc fake-namespace/fake-pvc with err timeout to assure pvc fake-pvc is deleted, finalizers in pvc []",
			logLevel:      "level=warning",
		},
		{
			name:         "delete pv pvc success, wait won't succeed but ensureTimeout is 0",
			pvcName:      "fake-pvc",
			pvcNamespace: "fake-namespace",
			pvName:       "fake-pv",
			kubeClientObj: []runtime.Object{
				pvcWithVolume,
				pvObject,
			},
			kubeReactors: []reactor{
				{
					verb:     "delete",
					resource: "persistentvolumeclaims",
					reactorFunc: func(action clientTesting.Action) (handled bool, ret runtime.Object, err error) {
						return true, pvcWithVolume, nil
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

			logMessage := ""
			DeletePVAndPVCIfAny(context.Background(), kubeClient.CoreV1(), test.pvcName, test.pvcNamespace, test.ensureTimeout, velerotest.NewSingleLogger(&logMessage))

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

	pvcObjectWithFinalizer := &corev1api.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:  "fake-ns",
			Name:       "fake-pvc",
			Finalizers: []string{"fake-finalizer-1", "fake-finalizer-2"},
		},
	}

	tests := []struct {
		name      string
		clientObj []runtime.Object
		pvcName   string
		namespace string
		reactors  []reactor
		timeout   time.Duration
		err       string
	}{
		{
			name:      "delete fail",
			pvcName:   "fake-pvc",
			namespace: "fake-ns",
			err:       "error to delete pvc fake-pvc: persistentvolumeclaims \"fake-pvc\" not found",
		},
		{
			name:      "0 timeout",
			pvcName:   "fake-pvc",
			namespace: "fake-ns",
			clientObj: []runtime.Object{pvcObject},
			reactors: []reactor{
				{
					verb:     "delete",
					resource: "persistentvolumeclaims",
					reactorFunc: func(action clientTesting.Action) (handled bool, ret runtime.Object, err error) {
						return true, pvcObject, nil
					},
				},
			},
		},
		{
			name:      "wait fail",
			pvcName:   "fake-pvc",
			namespace: "fake-ns",
			clientObj: []runtime.Object{pvcObject},
			timeout:   time.Millisecond,
			reactors: []reactor{
				{
					verb:     "get",
					resource: "persistentvolumeclaims",
					reactorFunc: func(action clientTesting.Action) (handled bool, ret runtime.Object, err error) {
						return true, nil, errors.New("fake-get-error")
					},
				},
			},
			err: "error to ensure pvc deleted for fake-pvc: error to get pvc fake-pvc: fake-get-error",
		},
		{
			name:      "wait timeout",
			pvcName:   "fake-pvc",
			namespace: "fake-ns",
			clientObj: []runtime.Object{pvcObjectWithFinalizer},
			timeout:   time.Millisecond,
			reactors: []reactor{
				{
					verb:     "delete",
					resource: "persistentvolumeclaims",
					reactorFunc: func(action clientTesting.Action) (handled bool, ret runtime.Object, err error) {
						return true, pvcObject, nil
					},
				},
			},
			err: "timeout to assure pvc fake-pvc is deleted, finalizers in pvc [fake-finalizer-1 fake-finalizer-2]",
		},
		{
			name:      "wait timeout, no finalizer",
			pvcName:   "fake-pvc",
			namespace: "fake-ns",
			clientObj: []runtime.Object{pvcObject},
			timeout:   time.Millisecond,
			reactors: []reactor{
				{
					verb:     "delete",
					resource: "persistentvolumeclaims",
					reactorFunc: func(action clientTesting.Action) (handled bool, ret runtime.Object, err error) {
						return true, pvcObject, nil
					},
				},
			},
			err: "timeout to assure pvc fake-pvc is deleted, finalizers in pvc []",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fakeKubeClient := fake.NewSimpleClientset(test.clientObj...)

			for _, reactor := range test.reactors {
				fakeKubeClient.Fake.PrependReactor(reactor.verb, reactor.resource, reactor.reactorFunc)
			}

			var kubeClient kubernetes.Interface = fakeKubeClient

			err := EnsureDeletePVC(context.Background(), kubeClient.CoreV1(), test.pvcName, test.namespace, test.timeout)
			if err != nil {
				assert.EqualError(t, err, test.err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestEnsureDeletePV(t *testing.T) {
	pvObject := &corev1api.PersistentVolume{
		ObjectMeta: metav1.ObjectMeta{
			Name: "fake-pv",
		},
	}

	tests := []struct {
		name      string
		clientObj []runtime.Object
		pvName    string
		reactors  []reactor
		timeout   time.Duration
		err       string
	}{
		{
			name:   "get fail",
			pvName: "fake-pv",
			err:    "error to get pv fake-pv: persistentvolumes \"fake-pv\" not found",
		},
		{
			name:      "0 timeout",
			pvName:    "fake-pv",
			clientObj: []runtime.Object{pvObject},
			reactors: []reactor{
				{
					verb:     "get",
					resource: "persistentvolumes",
					reactorFunc: func(action clientTesting.Action) (handled bool, ret runtime.Object, err error) {
						return true, pvObject, nil
					},
				},
			},
		},
		{
			name:      "wait fail",
			pvName:    "fake-pv",
			clientObj: []runtime.Object{pvObject},
			timeout:   time.Millisecond,
			reactors: []reactor{
				{
					verb:     "get",
					resource: "persistentvolumes",
					reactorFunc: func(action clientTesting.Action) (handled bool, ret runtime.Object, err error) {
						return true, nil, errors.New("fake-get-error")
					},
				},
			},
			err: "error to ensure pv is deleted for fake-pv: error to get pv fake-pv: fake-get-error",
		},
		{
			name:      "wait timeout",
			pvName:    "fake-pv",
			clientObj: []runtime.Object{pvObject},
			timeout:   time.Millisecond,
			reactors: []reactor{
				{
					verb:     "get",
					resource: "persistentvolumes",
					reactorFunc: func(action clientTesting.Action) (handled bool, ret runtime.Object, err error) {
						return true, pvObject, nil
					},
				},
			},
			err: "timeout to assure pv fake-pv is deleted",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fakeKubeClient := fake.NewSimpleClientset(test.clientObj...)

			for _, reactor := range test.reactors {
				fakeKubeClient.Fake.PrependReactor(reactor.verb, reactor.resource, reactor.reactorFunc)
			}

			var kubeClient kubernetes.Interface = fakeKubeClient

			err := EnsurePVDeleted(context.Background(), kubeClient.CoreV1(), test.pvName, test.timeout)
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

	pvcObject := &corev1api.PersistentVolumeClaim{
		TypeMeta: metav1.TypeMeta{
			Kind: "fake-kind-1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "fake-ns-1",
			Name:      "fake-pvc-1",
			Annotations: map[string]string{
				KubeAnnBindCompleted:     "true",
				KubeAnnBoundByController: "true",
			},
		},
	}

	tests := []struct {
		name      string
		clientObj []runtime.Object
		pv        *corev1api.PersistentVolume
		pvc       *corev1api.PersistentVolumeClaim
		labels    map[string]string
		reactors  []reactor
		result    *corev1api.PersistentVolume
		err       string
	}{
		{
			name:      "path fail",
			pv:        pvObject,
			pvc:       pvcObject,
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
			pvc:  pvcObject,
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
				Spec: corev1api.PersistentVolumeSpec{
					ClaimRef: &corev1api.ObjectReference{
						Kind:      "fake-kind-1",
						Namespace: "fake-ns-1",
						Name:      "fake-pvc-1",
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

			result, err := ResetPVBinding(context.Background(), kubeClient.CoreV1(), test.pv, test.labels, test.pvc)
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
			err: "error to wait for bound of PV: context deadline exceeded",
		},
		{
			name:   "pvc status not bound",
			pvName: "fake-pv",
			kubeClientObj: []runtime.Object{
				&corev1api.PersistentVolume{
					ObjectMeta: metav1.ObjectMeta{
						Name: "fake-pv",
					},
				},
			},
			err: "error to wait for bound of PV: context deadline exceeded",
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
					Status: corev1api.PersistentVolumeStatus{
						Phase: "Bound",
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
					Status: corev1api.PersistentVolumeStatus{
						Phase: "Bound",
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
					Status: corev1api.PersistentVolumeStatus{
						Phase: "Bound",
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
				Status: corev1api.PersistentVolumeStatus{
					Phase: "Bound",
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

var (
	csiStorageClass = "csi-hostpath-sc"
)

func TestGetPVForPVC(t *testing.T) {
	boundPVC := &corev1api.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-csi-pvc",
			Namespace: "default",
		},
		Spec: corev1api.PersistentVolumeClaimSpec{
			AccessModes: []corev1api.PersistentVolumeAccessMode{corev1api.ReadWriteOnce},
			Resources: corev1api.VolumeResourceRequirements{
				Requests: corev1api.ResourceList{},
			},
			StorageClassName: &csiStorageClass,
			VolumeName:       "test-csi-7d28e566-ade7-4ed6-9e15-2e44d2fbcc08",
		},
		Status: corev1api.PersistentVolumeClaimStatus{
			Phase:       corev1api.ClaimBound,
			AccessModes: []corev1api.PersistentVolumeAccessMode{corev1api.ReadWriteOnce},
			Capacity:    corev1api.ResourceList{},
		},
	}
	matchingPV := &corev1api.PersistentVolume{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-csi-7d28e566-ade7-4ed6-9e15-2e44d2fbcc08",
		},
		Spec: corev1api.PersistentVolumeSpec{
			AccessModes: []corev1api.PersistentVolumeAccessMode{corev1api.ReadWriteOnce},
			Capacity:    corev1api.ResourceList{},
			ClaimRef: &corev1api.ObjectReference{
				Kind:            "PersistentVolumeClaim",
				Name:            "test-csi-pvc",
				Namespace:       "default",
				ResourceVersion: "1027",
				UID:             "7d28e566-ade7-4ed6-9e15-2e44d2fbcc08",
			},
			PersistentVolumeSource: corev1api.PersistentVolumeSource{
				CSI: &corev1api.CSIPersistentVolumeSource{
					Driver: "hostpath.csi.k8s.io",
					FSType: "ext4",
					VolumeAttributes: map[string]string{
						"storage.kubernetes.io/csiProvisionerIdentity": "1582049697841-8081-hostpath.csi.k8s.io",
					},
					VolumeHandle: "e61f2b48-527a-11ea-b54f-cab6317018f1",
				},
			},
			PersistentVolumeReclaimPolicy: corev1api.PersistentVolumeReclaimDelete,
			StorageClassName:              csiStorageClass,
		},
		Status: corev1api.PersistentVolumeStatus{
			Phase: corev1api.VolumeBound,
		},
	}

	pvcWithNoVolumeName := &corev1api.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "no-vol-pvc",
			Namespace: "default",
		},
		Spec: corev1api.PersistentVolumeClaimSpec{
			AccessModes: []corev1api.PersistentVolumeAccessMode{corev1api.ReadWriteOnce},
			Resources: corev1api.VolumeResourceRequirements{
				Requests: corev1api.ResourceList{},
			},
			StorageClassName: &csiStorageClass,
		},
		Status: corev1api.PersistentVolumeClaimStatus{},
	}

	unboundPVC := &corev1api.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "unbound-pvc",
			Namespace: "default",
		},
		Spec: corev1api.PersistentVolumeClaimSpec{
			AccessModes: []corev1api.PersistentVolumeAccessMode{corev1api.ReadWriteOnce},
			Resources: corev1api.VolumeResourceRequirements{
				Requests: corev1api.ResourceList{},
			},
			StorageClassName: &csiStorageClass,
			VolumeName:       "test-csi-7d28e566-ade7-4ed6-9e15-2e44d2fbcc08",
		},
		Status: corev1api.PersistentVolumeClaimStatus{
			Phase:       corev1api.ClaimPending,
			AccessModes: []corev1api.PersistentVolumeAccessMode{corev1api.ReadWriteOnce},
			Capacity:    corev1api.ResourceList{},
		},
	}

	testCases := []struct {
		name        string
		inPVC       *corev1api.PersistentVolumeClaim
		expectError bool
		expectedPV  *corev1api.PersistentVolume
	}{
		{
			name:        "should find PV matching the PVC",
			inPVC:       boundPVC,
			expectError: false,
			expectedPV:  matchingPV,
		},
		{
			name:        "should fail to find PV for PVC with no volumeName",
			inPVC:       pvcWithNoVolumeName,
			expectError: true,
			expectedPV:  nil,
		},
		{
			name:        "should fail to find PV for PVC not in bound phase",
			inPVC:       unboundPVC,
			expectError: true,
			expectedPV:  nil,
		},
	}

	objs := []runtime.Object{boundPVC, matchingPV, pvcWithNoVolumeName, unboundPVC}
	fakeClient := velerotest.NewFakeControllerRuntimeClient(t, objs...)

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actualPV, actualError := GetPVForPVC(tc.inPVC, fakeClient)

			if tc.expectError {
				assert.Error(t, actualError, "Want error; Got nil error")
				assert.Nilf(t, actualPV, "Want PV: nil; Got PV: %q", actualPV)
				return
			}

			assert.NoErrorf(t, actualError, "Want: nil error; Got: %v", actualError)
			assert.Equalf(t, actualPV.Name, tc.expectedPV.Name, "Want PV with name %q; Got PV with name %q", tc.expectedPV.Name, actualPV.Name)
		})
	}
}

func TestGetPVCForPodVolume(t *testing.T) {
	sampleVol := &corev1api.Volume{
		Name: "sample-volume",
		VolumeSource: corev1api.VolumeSource{
			PersistentVolumeClaim: &corev1api.PersistentVolumeClaimVolumeSource{
				ClaimName: "sample-pvc",
			},
		},
	}

	sampleVol2 := &corev1api.Volume{
		Name: "sample-volume",
		VolumeSource: corev1api.VolumeSource{
			PersistentVolumeClaim: &corev1api.PersistentVolumeClaimVolumeSource{
				ClaimName: "sample-pvc-1",
			},
		},
	}

	sampleVol3 := &corev1api.Volume{
		Name:         "sample-volume",
		VolumeSource: corev1api.VolumeSource{},
	}

	samplePod := &corev1api.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "sample-pod",
			Namespace: "sample-ns",
		},

		Spec: corev1api.PodSpec{
			Containers: []corev1api.Container{
				{
					Name:  "sample-container",
					Image: "sample-image",
					VolumeMounts: []corev1api.VolumeMount{
						{
							Name:      "sample-vm",
							MountPath: "/etc/pod-info",
						},
					},
				},
			},
			Volumes: []corev1api.Volume{
				{
					Name: "sample-volume",
					VolumeSource: corev1api.VolumeSource{
						PersistentVolumeClaim: &corev1api.PersistentVolumeClaimVolumeSource{
							ClaimName: "sample-pvc",
						},
					},
				},
			},
		},
	}

	matchingPVC := &corev1api.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "sample-pvc",
			Namespace: "sample-ns",
		},
		Spec: corev1api.PersistentVolumeClaimSpec{
			AccessModes: []corev1api.PersistentVolumeAccessMode{corev1api.ReadWriteOnce},
			Resources: corev1api.VolumeResourceRequirements{
				Requests: corev1api.ResourceList{},
			},
			StorageClassName: &csiStorageClass,
			VolumeName:       "test-csi-7d28e566-ade7-4ed6-9e15-2e44d2fbcc08",
		},
		Status: corev1api.PersistentVolumeClaimStatus{
			Phase:       corev1api.ClaimBound,
			AccessModes: []corev1api.PersistentVolumeAccessMode{corev1api.ReadWriteOnce},
			Capacity:    corev1api.ResourceList{},
		},
	}

	testCases := []struct {
		name          string
		vol           *corev1api.Volume
		pod           *corev1api.Pod
		expectedPVC   *corev1api.PersistentVolumeClaim
		expectedError bool
	}{
		{
			name:          "should find PVC for volume",
			vol:           sampleVol,
			pod:           samplePod,
			expectedPVC:   matchingPVC,
			expectedError: false,
		},
		{
			name:          "should not find PVC for volume not found error case",
			vol:           sampleVol2,
			pod:           samplePod,
			expectedPVC:   nil,
			expectedError: true,
		},
		{
			name:          "should not find PVC vol has no PVC, error case",
			vol:           sampleVol3,
			pod:           samplePod,
			expectedPVC:   nil,
			expectedError: true,
		},
	}

	objs := []runtime.Object{matchingPVC}
	fakeClient := velerotest.NewFakeControllerRuntimeClient(t, objs...)
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actualPVC, actualError := GetPVCForPodVolume(tc.vol, samplePod, fakeClient)
			if tc.expectedError {
				assert.Error(t, actualError, "Want error; Got nil error")
				assert.Nilf(t, actualPVC, "Want PV: nil; Got PV: %q", actualPVC)
				return
			}
			assert.NoErrorf(t, actualError, "Want: nil error; Got: %v", actualError)
			assert.Equalf(t, actualPVC.Name, tc.expectedPVC.Name, "Want PVC with name %q; Got PVC with name %q", tc.expectedPVC.Name, actualPVC)
		})
	}
}

func TestMakePodPVCAttachment(t *testing.T) {
	testCases := []struct {
		name                 string
		volumeName           string
		volumeMode           corev1api.PersistentVolumeMode
		readOnly             bool
		expectedVolumeMount  []corev1api.VolumeMount
		expectedVolumeDevice []corev1api.VolumeDevice
		expectedVolumePath   string
	}{
		{
			name:       "no volume mode specified",
			volumeName: "volume-1",
			readOnly:   true,
			expectedVolumeMount: []corev1api.VolumeMount{
				{
					Name:      "volume-1",
					MountPath: "/volume-1",
					ReadOnly:  true,
				},
			},
			expectedVolumePath: "/volume-1",
		},
		{
			name:       "fs mode specified",
			volumeName: "volume-2",
			volumeMode: corev1api.PersistentVolumeFilesystem,
			readOnly:   true,
			expectedVolumeMount: []corev1api.VolumeMount{
				{
					Name:      "volume-2",
					MountPath: "/volume-2",
					ReadOnly:  true,
				},
			},
			expectedVolumePath: "/volume-2",
		},
		{
			name:       "block volume mode specified",
			volumeName: "volume-3",
			volumeMode: corev1api.PersistentVolumeBlock,
			expectedVolumeDevice: []corev1api.VolumeDevice{
				{
					Name:       "volume-3",
					DevicePath: "/volume-3",
				},
			},
			expectedVolumePath: "/volume-3",
		},
		{
			name:       "fs mode specified with readOnly as false",
			volumeName: "volume-4",
			readOnly:   false,
			volumeMode: corev1api.PersistentVolumeFilesystem,
			expectedVolumeMount: []corev1api.VolumeMount{
				{
					Name:      "volume-4",
					MountPath: "/volume-4",
					ReadOnly:  false,
				},
			},
			expectedVolumePath: "/volume-4",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var volMode *corev1api.PersistentVolumeMode
			if tc.volumeMode != "" {
				volMode = &tc.volumeMode
			}

			mount, device, path := MakePodPVCAttachment(tc.volumeName, volMode, tc.readOnly)

			assert.Equal(t, tc.expectedVolumeMount, mount)
			assert.Equal(t, tc.expectedVolumeDevice, device)
			assert.Equal(t, tc.expectedVolumePath, path)
			if tc.expectedVolumeMount != nil {
				assert.Equal(t, tc.expectedVolumeMount[0].ReadOnly, tc.readOnly)
			}
		})
	}
}

func TestDiagnosePVC(t *testing.T) {
	testCases := []struct {
		name     string
		pvc      *corev1api.PersistentVolumeClaim
		expected string
	}{
		{
			name: "pvc with all info",
			pvc: &corev1api.PersistentVolumeClaim{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "fake-pvc",
					Namespace: "fake-ns",
				},
				Spec: corev1api.PersistentVolumeClaimSpec{
					VolumeName: "fake-pv",
				},
				Status: corev1api.PersistentVolumeClaimStatus{
					Phase: corev1api.ClaimPending,
				},
			},
			expected: "PVC fake-ns/fake-pvc, phase Pending, binding to fake-pv\n",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			diag := DiagnosePVC(tc.pvc)
			assert.Equal(t, tc.expected, diag)
		})
	}
}

func TestDiagnosePV(t *testing.T) {
	testCases := []struct {
		name     string
		pv       *corev1api.PersistentVolume
		expected string
	}{
		{
			name: "pv with all info",
			pv: &corev1api.PersistentVolume{
				ObjectMeta: metav1.ObjectMeta{
					Name: "fake-pv",
				},
				Status: corev1api.PersistentVolumeStatus{
					Phase:   corev1api.VolumePending,
					Message: "fake-message",
					Reason:  "fake-reason",
				},
			},
			expected: "PV fake-pv, phase Pending, reason fake-reason, message fake-message\n",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			diag := DiagnosePV(tc.pv)
			assert.Equal(t, tc.expected, diag)
		})
	}
}

func TestGetPVCAttachingNodeOS(t *testing.T) {
	storageClass := "fake-storage-class"
	nodeNoOSLabel := builder.ForNode("fake-node").Result()
	nodeWindows := builder.ForNode("fake-node").Labels(map[string]string{"kubernetes.io/os": "windows"}).Result()

	pvcObj := &corev1api.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "fake-namespace",
			Name:      "fake-pvc",
		},
	}

	blockMode := corev1api.PersistentVolumeBlock
	pvcObjBlockMode := &corev1api.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "fake-namespace",
			Name:      "fake-pvc",
		},
		Spec: corev1api.PersistentVolumeClaimSpec{
			VolumeMode: &blockMode,
		},
	}

	pvcObjWithNode := &corev1api.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:   "fake-namespace",
			Name:        "fake-pvc",
			Annotations: map[string]string{KubeAnnSelectedNode: "fake-node"},
		},
	}

	pvcObjWithVolume := &corev1api.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "fake-namespace",
			Name:      "fake-pvc",
		},
		Spec: corev1api.PersistentVolumeClaimSpec{
			VolumeName: "fake-volume-name",
		},
	}

	pvcObjWithStorageClass := &corev1api.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "fake-namespace",
			Name:      "fake-pvc",
		},
		Spec: corev1api.PersistentVolumeClaimSpec{
			StorageClassName: &storageClass,
		},
	}

	pvName := "fake-volume-name"
	pvcObjWithAll := &corev1api.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:   "fake-namespace",
			Name:        "fake-pvc",
			Annotations: map[string]string{KubeAnnSelectedNode: "fake-node"},
		},
		Spec: corev1api.PersistentVolumeClaimSpec{
			VolumeName:       pvName,
			StorageClassName: &storageClass,
		},
	}

	pvcObjWithVolumeSC := &corev1api.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "fake-namespace",
			Name:      "fake-pvc",
		},
		Spec: corev1api.PersistentVolumeClaimSpec{
			VolumeName:       pvName,
			StorageClassName: &storageClass,
		},
	}

	scObjWithoutFSType := &storagev1api.StorageClass{
		ObjectMeta: metav1.ObjectMeta{
			Name: "fake-storage-class",
		},
	}

	scObjWithFSType := &storagev1api.StorageClass{
		ObjectMeta: metav1.ObjectMeta{
			Name: "fake-storage-class",
		},
		Parameters: map[string]string{"csi.storage.k8s.io/fstype": "ntfs"},
	}

	volAttachEmpty := &storagev1api.VolumeAttachment{
		ObjectMeta: metav1.ObjectMeta{
			Name: "fake-volume-attach-1",
		},
	}

	volAttachWithVolume := &storagev1api.VolumeAttachment{
		ObjectMeta: metav1.ObjectMeta{
			Name: "fake-volume-attach-2",
		},
		Spec: storagev1api.VolumeAttachmentSpec{
			Source: storagev1api.VolumeAttachmentSource{
				PersistentVolumeName: &pvName,
			},
		},
	}

	otherPVName := "other-volume-name"
	volAttachWithOtherVolume := &storagev1api.VolumeAttachment{
		ObjectMeta: metav1.ObjectMeta{
			Name: "fake-volume-attach-3",
		},
		Spec: storagev1api.VolumeAttachmentSpec{
			Source: storagev1api.VolumeAttachmentSource{
				PersistentVolumeName: &otherPVName,
			},
		},
	}

	tests := []struct {
		name           string
		pvc            *corev1api.PersistentVolumeClaim
		kubeClientObj  []runtime.Object
		expectedNodeOS string
		err            string
	}{
		{
			name:           "no selected node, volume name and storage class",
			pvc:            pvcObj,
			expectedNodeOS: NodeOSLinux,
		},
		{
			name: "node doesn't exist",
			pvc:  pvcObjWithNode,
			err:  "error to get os from node fake-node for PVC fake-namespace/fake-pvc: error getting node fake-node: nodes \"fake-node\" not found",
		},
		{
			name: "node without os label",
			pvc:  pvcObjWithNode,
			kubeClientObj: []runtime.Object{
				nodeNoOSLabel,
			},
			expectedNodeOS: NodeOSLinux,
		},
		{
			name:           "no attach volume",
			pvc:            pvcObjWithVolume,
			expectedNodeOS: NodeOSLinux,
		},
		{
			name: "sc doesn't exist",
			pvc:  pvcObjWithStorageClass,
			err:  "error to get storage class fake-storage-class: storageclasses.storage.k8s.io \"fake-storage-class\" not found",
		},
		{
			name: "volume attachment not exist",
			pvc:  pvcObjWithVolume,
			kubeClientObj: []runtime.Object{
				nodeWindows,
				scObjWithFSType,
				volAttachEmpty,
				volAttachWithOtherVolume,
			},
			expectedNodeOS: NodeOSLinux,
		},
		{
			name: "sc without fsType",
			pvc:  pvcObjWithStorageClass,
			kubeClientObj: []runtime.Object{
				scObjWithoutFSType,
			},
			expectedNodeOS: NodeOSLinux,
		},
		{
			name: "deduce from node os",
			pvc:  pvcObjWithAll,
			kubeClientObj: []runtime.Object{
				nodeWindows,
				scObjWithFSType,
			},
			expectedNodeOS: NodeOSWindows,
		},
		{
			name: "deduce from sc",
			pvc:  pvcObjWithAll,
			kubeClientObj: []runtime.Object{
				nodeNoOSLabel,
				scObjWithFSType,
			},
			expectedNodeOS: NodeOSWindows,
		},
		{
			name: "deduce from attached node os",
			pvc:  pvcObjWithVolumeSC,
			kubeClientObj: []runtime.Object{
				nodeWindows,
				scObjWithFSType,
				volAttachEmpty,
				volAttachWithOtherVolume,
				volAttachWithVolume,
			},
			expectedNodeOS: NodeOSWindows,
		},
		{
			name:           "block access",
			pvc:            pvcObjBlockMode,
			expectedNodeOS: NodeOSLinux,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fakeKubeClient := fake.NewSimpleClientset(test.kubeClientObj...)

			var kubeClient kubernetes.Interface = fakeKubeClient

			nodeOS, err := GetPVCAttachingNodeOS(test.pvc, kubeClient.CoreV1(), kubeClient.StorageV1(), velerotest.NewLogger())

			if err != nil {
				assert.EqualError(t, err, test.err)
			} else {
				assert.NoError(t, err)
			}

			assert.Equal(t, test.expectedNodeOS, nodeOS)
		})
	}
}
