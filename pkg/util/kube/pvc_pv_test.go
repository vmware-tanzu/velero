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

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"

	corev1api "k8s.io/api/core/v1"

	clientTesting "k8s.io/client-go/testing"
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
			name:    "wait pvc error",
			pvcName: "fake-pvc",
			err:     "error to wait for rediness of PVC: failed to get pvc /fake-pvc: persistentvolumeclaims \"fake-pvc\" not found",
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
