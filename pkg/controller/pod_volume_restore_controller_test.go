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
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1api "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clocks "k8s.io/utils/clock"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	velerov1api "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	"github.com/vmware-tanzu/velero/pkg/restorehelper"
	"github.com/vmware-tanzu/velero/pkg/test"
)

func TestShouldProcess(t *testing.T) {
	controllerNode := "foo"

	tests := []struct {
		name            string
		obj             *velerov1api.PodVolumeRestore
		pod             *corev1api.Pod
		shouldProcessed bool
	}{
		{
			name: "InProgress phase pvr should not be processed",
			obj: &velerov1api.PodVolumeRestore{
				Status: velerov1api.PodVolumeRestoreStatus{
					Phase: velerov1api.PodVolumeRestorePhaseInProgress,
				},
			},
			shouldProcessed: false,
		},
		{
			name: "Completed phase pvr should not be processed",
			obj: &velerov1api.PodVolumeRestore{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "velero",
					Name:      "pvr-1",
				},
				Status: velerov1api.PodVolumeRestoreStatus{
					Phase: velerov1api.PodVolumeRestorePhaseCompleted,
				},
			},
			shouldProcessed: false,
		},
		{
			name: "Failed phase pvr should not be processed",
			obj: &velerov1api.PodVolumeRestore{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "velero",
					Name:      "pvr-1",
				},
				Status: velerov1api.PodVolumeRestoreStatus{
					Phase: velerov1api.PodVolumeRestorePhaseFailed,
				},
			},
			shouldProcessed: false,
		},
		{
			name: "Unable to get pvr's pod should not be processed",
			obj: &velerov1api.PodVolumeRestore{
				Spec: velerov1api.PodVolumeRestoreSpec{
					Pod: corev1api.ObjectReference{
						Namespace: "ns-1",
						Name:      "pod-1",
					},
				},
				Status: velerov1api.PodVolumeRestoreStatus{
					Phase: "",
				},
			},
			shouldProcessed: false,
		},
		{
			name: "Empty phase pvr with pod on node not running init container should not be processed",
			obj: &velerov1api.PodVolumeRestore{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "velero",
					Name:      "pvr-1",
				},
				Spec: velerov1api.PodVolumeRestoreSpec{
					Pod: corev1api.ObjectReference{
						Namespace: "ns-1",
						Name:      "pod-1",
					},
				},
				Status: velerov1api.PodVolumeRestoreStatus{
					Phase: "",
				},
			},
			pod: &corev1api.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "ns-1",
					Name:      "pod-1",
				},
				Spec: corev1api.PodSpec{
					NodeName: controllerNode,
					InitContainers: []corev1api.Container{
						{
							Name: restorehelper.WaitInitContainer,
						},
					},
				},
				Status: corev1api.PodStatus{
					InitContainerStatuses: []corev1api.ContainerStatus{
						{
							State: corev1api.ContainerState{},
						},
					},
				},
			},
			shouldProcessed: false,
		},
		{
			name: "Empty phase pvr with pod on node running init container should be enqueued",
			obj: &velerov1api.PodVolumeRestore{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "velero",
					Name:      "pvr-1",
				},
				Spec: velerov1api.PodVolumeRestoreSpec{
					Pod: corev1api.ObjectReference{
						Namespace: "ns-1",
						Name:      "pod-1",
					},
				},
				Status: velerov1api.PodVolumeRestoreStatus{
					Phase: "",
				},
			},
			pod: &corev1api.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "ns-1",
					Name:      "pod-1",
				},
				Spec: corev1api.PodSpec{
					NodeName: controllerNode,
					InitContainers: []corev1api.Container{
						{
							Name: restorehelper.WaitInitContainer,
						},
					},
				},
				Status: corev1api.PodStatus{
					InitContainerStatuses: []corev1api.ContainerStatus{
						{
							State: corev1api.ContainerState{
								Running: &corev1api.ContainerStateRunning{
									StartedAt: metav1.Time{Time: time.Now()},
								},
							},
						},
					},
				},
			},
			shouldProcessed: true,
		},
	}

	for _, ts := range tests {
		t.Run(ts.name, func(t *testing.T) {
			ctx := context.Background()

			var objs []runtime.Object
			if ts.obj != nil {
				objs = append(objs, ts.obj)
			}
			if ts.pod != nil {
				objs = append(objs, ts.pod)
			}
			cli := test.NewFakeControllerRuntimeClient(t, objs...)

			c := &PodVolumeRestoreReconciler{
				logger: logrus.New(),
				Client: cli,
				clock:  &clocks.RealClock{},
			}

			shouldProcess, _, _ := c.shouldProcess(ctx, c.logger, ts.obj)
			require.Equal(t, ts.shouldProcessed, shouldProcess)
		})
	}
}

func TestIsInitContainerRunning(t *testing.T) {
	tests := []struct {
		name     string
		pod      *corev1api.Pod
		expected bool
	}{
		{
			name: "pod with no init containers should return false",
			pod: &corev1api.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "ns-1",
					Name:      "pod-1",
				},
			},
			expected: false,
		},
		{
			name: "pod with running init container that's not restore init should return false",
			pod: &corev1api.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "ns-1",
					Name:      "pod-1",
				},
				Spec: corev1api.PodSpec{
					InitContainers: []corev1api.Container{
						{
							Name: "non-restore-init",
						},
					},
				},
				Status: corev1api.PodStatus{
					InitContainerStatuses: []corev1api.ContainerStatus{
						{
							State: corev1api.ContainerState{
								Running: &corev1api.ContainerStateRunning{StartedAt: metav1.Time{Time: time.Now()}},
							},
						},
					},
				},
			},
			expected: false,
		},
		{
			name: "pod with running init container that's not first should still work",
			pod: &corev1api.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "ns-1",
					Name:      "pod-1",
				},
				Spec: corev1api.PodSpec{
					InitContainers: []corev1api.Container{
						{
							Name: "non-restore-init",
						},
						{
							Name: restorehelper.WaitInitContainer,
						},
					},
				},
				Status: corev1api.PodStatus{
					InitContainerStatuses: []corev1api.ContainerStatus{
						{
							State: corev1api.ContainerState{
								Running: &corev1api.ContainerStateRunning{StartedAt: metav1.Time{Time: time.Now()}},
							},
						},
						{
							State: corev1api.ContainerState{
								Running: &corev1api.ContainerStateRunning{StartedAt: metav1.Time{Time: time.Now()}},
							},
						},
					},
				},
			},
			expected: true,
		},
		{
			name: "pod with init container as first initContainer that's not running should return false",
			pod: &corev1api.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "ns-1",
					Name:      "pod-1",
				},
				Spec: corev1api.PodSpec{
					InitContainers: []corev1api.Container{
						{
							Name: restorehelper.WaitInitContainer,
						},
						{
							Name: "non-restore-init",
						},
					},
				},
				Status: corev1api.PodStatus{
					InitContainerStatuses: []corev1api.ContainerStatus{
						{
							State: corev1api.ContainerState{},
						},
						{
							State: corev1api.ContainerState{
								Running: &corev1api.ContainerStateRunning{StartedAt: metav1.Time{Time: time.Now()}},
							},
						},
					},
				},
			},
			expected: false,
		},
		{
			name: "pod with running init container as first initContainer should return true",
			pod: &corev1api.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "ns-1",
					Name:      "pod-1",
				},
				Spec: corev1api.PodSpec{
					InitContainers: []corev1api.Container{
						{
							Name: restorehelper.WaitInitContainer,
						},
						{
							Name: "non-restore-init",
						},
					},
				},
				Status: corev1api.PodStatus{
					InitContainerStatuses: []corev1api.ContainerStatus{
						{
							State: corev1api.ContainerState{
								Running: &corev1api.ContainerStateRunning{StartedAt: metav1.Time{Time: time.Now()}},
							},
						},
						{
							State: corev1api.ContainerState{
								Running: &corev1api.ContainerStateRunning{StartedAt: metav1.Time{Time: time.Now()}},
							},
						},
					},
				},
			},
			expected: true,
		},
		{
			name: "pod with init container with empty InitContainerStatuses should return 0",
			pod: &corev1api.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "ns-1",
					Name:      "pod-1",
				},
				Spec: corev1api.PodSpec{
					InitContainers: []corev1api.Container{
						{
							Name: restorehelper.WaitInitContainer,
						},
					},
				},
				Status: corev1api.PodStatus{
					InitContainerStatuses: []corev1api.ContainerStatus{},
				},
			},
			expected: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.expected, isInitContainerRunning(test.pod))
		})
	}
}

func TestGetInitContainerIndex(t *testing.T) {
	tests := []struct {
		name     string
		pod      *corev1api.Pod
		expected int
	}{
		{
			name: "init container is not present return -1",
			pod: &corev1api.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "ns-1",
					Name:      "pod-1",
				},
			},
			expected: -1,
		},
		{
			name: "pod with no init container return -1",
			pod: &corev1api.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "ns-1",
					Name:      "pod-1",
				},
				Spec: corev1api.PodSpec{
					InitContainers: []corev1api.Container{
						{
							Name: "non-restore-init",
						},
					},
				},
			},
			expected: -1,
		},
		{
			name: "pod with container as second initContainern should return 1",
			pod: &corev1api.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "ns-1",
					Name:      "pod-1",
				},
				Spec: corev1api.PodSpec{
					InitContainers: []corev1api.Container{
						{
							Name: "non-restore-init",
						},
						{
							Name: restorehelper.WaitInitContainer,
						},
					},
				},
			},
			expected: 1,
		},
		{
			name: "pod with init container as first initContainer should return 0",
			pod: &corev1api.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "ns-1",
					Name:      "pod-1",
				},
				Spec: corev1api.PodSpec{
					InitContainers: []corev1api.Container{
						{
							Name: restorehelper.WaitInitContainer,
						},
						{
							Name: "non-restore-init",
						},
					},
				},
			},
			expected: 0,
		},
		{
			name: "pod with init container as first initContainer should return 0",
			pod: &corev1api.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "ns-1",
					Name:      "pod-1",
				},
				Spec: corev1api.PodSpec{
					InitContainers: []corev1api.Container{
						{
							Name: restorehelper.WaitInitContainer,
						},
						{
							Name: "non-restore-init",
						},
					},
				},
			},
			expected: 0,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.expected, getInitContainerIndex(test.pod))
		})
	}
}

func TestFindVolumeRestoresForPod(t *testing.T) {
	pod := &corev1api.Pod{}
	pod.UID = "uid"

	scheme := runtime.NewScheme()
	scheme.AddKnownTypes(velerov1api.SchemeGroupVersion, &velerov1api.PodVolumeRestore{}, &velerov1api.PodVolumeRestoreList{})
	clientBuilder := fake.NewClientBuilder().WithScheme(scheme)

	// no matching PVR
	reconciler := &PodVolumeRestoreReconciler{
		Client: clientBuilder.Build(),
		logger: logrus.New(),
	}
	requests := reconciler.findVolumeRestoresForPod(pod)
	assert.Len(t, requests, 0)

	// contain one matching PVR
	reconciler.Client = clientBuilder.WithLists(&velerov1api.PodVolumeRestoreList{
		Items: []velerov1api.PodVolumeRestore{
			{
				ObjectMeta: metav1.ObjectMeta{
					Name: "pvr1",
					Labels: map[string]string{
						velerov1api.PodUIDLabel: string(pod.GetUID()),
					},
				},
			},
			{
				ObjectMeta: metav1.ObjectMeta{
					Name: "pvr2",
					Labels: map[string]string{
						velerov1api.PodUIDLabel: "non-matching-uid",
					},
				},
			},
		},
	}).Build()
	requests = reconciler.findVolumeRestoresForPod(pod)
	assert.Len(t, requests, 1)
}
