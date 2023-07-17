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
	corev1api "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"

	clientTesting "k8s.io/client-go/testing"

	velerotest "github.com/vmware-tanzu/velero/pkg/test"
)

func TestEnsureDeletePod(t *testing.T) {
	podObject := &corev1api.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "fake-ns",
			Name:      "fake-pod",
		},
	}

	tests := []struct {
		name      string
		clientObj []runtime.Object
		podName   string
		namespace string
		reactors  []reactor
		err       string
	}{
		{
			name:      "delete fail",
			podName:   "fake-pod",
			namespace: "fake-ns",
			err:       "error to delete pod fake-pod: pods \"fake-pod\" not found",
		},
		{
			name:      "wait fail",
			podName:   "fake-pod",
			namespace: "fake-ns",
			clientObj: []runtime.Object{podObject},
			reactors: []reactor{
				{
					verb:     "get",
					resource: "pods",
					reactorFunc: func(action clientTesting.Action) (handled bool, ret runtime.Object, err error) {
						return true, nil, errors.New("fake-get-error")
					},
				},
			},
			err: "error to assure pod is deleted, fake-pod: error to get pod fake-pod: fake-get-error",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fakeKubeClient := fake.NewSimpleClientset(test.clientObj...)

			for _, reactor := range test.reactors {
				fakeKubeClient.Fake.PrependReactor(reactor.verb, reactor.resource, reactor.reactorFunc)
			}

			var kubeClient kubernetes.Interface = fakeKubeClient

			err := EnsureDeletePod(context.Background(), kubeClient.CoreV1(), test.podName, test.namespace, time.Millisecond)
			if err != nil {
				assert.EqualError(t, err, test.err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestIsPodRunning(t *testing.T) {
	tests := []struct {
		name string
		pod  *corev1api.Pod
		err  string
	}{
		{
			name: "pod is nil",
			err:  "invalid input pod",
		},
		{
			name: "pod is not scheduled",
			pod: &corev1api.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "fake-ns",
					Name:      "fake-pod",
				},
				Status: corev1api.PodStatus{
					Phase: "fake-phase",
				},
			},
			err: "pod is not scheduled, name=fake-pod, namespace=fake-ns, phase=fake-phase",
		},
		{
			name: "pod is not running",
			pod: &corev1api.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "fake-ns",
					Name:      "fake-pod",
				},
				Spec: corev1api.PodSpec{
					NodeName: "fake-node",
				},
				Status: corev1api.PodStatus{
					Phase: "fake-phase",
				},
			},
			err: "pod is not in the expected status, name=fake-pod, namespace=fake-ns, phase=fake-phase: pod is not running",
		},
		{
			name: "pod is being deleted",
			pod: &corev1api.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Namespace:         "fake-ns",
					Name:              "fake-pod",
					DeletionTimestamp: &metav1.Time{Time: time.Now()},
				},
				Spec: corev1api.PodSpec{
					NodeName: "fake-node",
				},
				Status: corev1api.PodStatus{
					Phase: corev1api.PodRunning,
				},
			},
			err: "pod is being terminated, name=fake-pod, namespace=fake-ns, phase=Running",
		},
		{
			name: "success",
			pod: &corev1api.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "fake-ns",
					Name:      "fake-pod",
				},
				Spec: corev1api.PodSpec{
					NodeName: "fake-node",
				},
				Status: corev1api.PodStatus{
					Phase: corev1api.PodRunning,
				},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := IsPodRunning(test.pod)
			if err != nil {
				assert.EqualError(t, err, test.err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestIsPodScheduled(t *testing.T) {
	tests := []struct {
		name string
		pod  *corev1api.Pod
		err  string
	}{
		{
			name: "pod is nil",
			err:  "invalid input pod",
		},
		{
			name: "pod is not scheduled",
			pod: &corev1api.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "fake-ns",
					Name:      "fake-pod",
				},
				Status: corev1api.PodStatus{
					Phase: "fake-phase",
				},
			},
			err: "pod is not scheduled, name=fake-pod, namespace=fake-ns, phase=fake-phase",
		},
		{
			name: "pod is not running or pending",
			pod: &corev1api.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "fake-ns",
					Name:      "fake-pod",
				},
				Spec: corev1api.PodSpec{
					NodeName: "fake-node",
				},
				Status: corev1api.PodStatus{
					Phase: "fake-phase",
				},
			},
			err: "pod is not in the expected status, name=fake-pod, namespace=fake-ns, phase=fake-phase: pod is not running or pending",
		},
		{
			name: "pod is being deleted",
			pod: &corev1api.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Namespace:         "fake-ns",
					Name:              "fake-pod",
					DeletionTimestamp: &metav1.Time{Time: time.Now()},
				},
				Spec: corev1api.PodSpec{
					NodeName: "fake-node",
				},
				Status: corev1api.PodStatus{
					Phase: corev1api.PodRunning,
				},
			},
			err: "pod is being terminated, name=fake-pod, namespace=fake-ns, phase=Running",
		},
		{
			name: "success on running",
			pod: &corev1api.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "fake-ns",
					Name:      "fake-pod",
				},
				Spec: corev1api.PodSpec{
					NodeName: "fake-node",
				},
				Status: corev1api.PodStatus{
					Phase: corev1api.PodRunning,
				},
			},
		},
		{
			name: "success on pending",
			pod: &corev1api.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "fake-ns",
					Name:      "fake-pod",
				},
				Spec: corev1api.PodSpec{
					NodeName: "fake-node",
				},
				Status: corev1api.PodStatus{
					Phase: corev1api.PodPending,
				},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := IsPodScheduled(test.pod)
			if err != nil {
				assert.EqualError(t, err, test.err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestDeletePodIfAny(t *testing.T) {
	tests := []struct {
		name          string
		podName       string
		podNamespace  string
		kubeClientObj []runtime.Object
		kubeReactors  []reactor
		logMessage    string
		logLevel      string
		logError      string
	}{
		{
			name:         "get fail",
			podName:      "fake-pod",
			podNamespace: "fake-namespace",
			logMessage:   "Abort deleting pod, it doesn't exist fake-namespace/fake-pod",
			logLevel:     "level=debug",
		},
		{
			name:         "delete fail",
			podName:      "fake-pod",
			podNamespace: "fake-namespace",
			kubeReactors: []reactor{
				{
					verb:     "delete",
					resource: "pods",
					reactorFunc: func(action clientTesting.Action) (handled bool, ret runtime.Object, err error) {
						return true, nil, errors.New("fake-delete-error")
					},
				},
			},
			logMessage: "Failed to delete pod fake-namespace/fake-pod",
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
			DeletePodIfAny(context.Background(), kubeClient.CoreV1(), test.podName, test.podNamespace, velerotest.NewSingleLogger(&logMessage))

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
