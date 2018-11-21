/*
Copyright 2018 the Heptio Ark contributors.

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
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1api "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
	corev1listers "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"

	arkv1api "github.com/heptio/ark/pkg/apis/ark/v1"
	arkfake "github.com/heptio/ark/pkg/generated/clientset/versioned/fake"
	arkinformers "github.com/heptio/ark/pkg/generated/informers/externalversions"
	arkv1listers "github.com/heptio/ark/pkg/generated/listers/ark/v1"
	"github.com/heptio/ark/pkg/restic"
	arktest "github.com/heptio/ark/pkg/util/test"
)

func TestPVRHandler(t *testing.T) {
	controllerNode := "foo"

	tests := []struct {
		name          string
		obj           *arkv1api.PodVolumeRestore
		pod           *corev1api.Pod
		shouldEnqueue bool
	}{
		{
			name: "InProgress phase pvr should not be enqueued",
			obj: &arkv1api.PodVolumeRestore{
				Status: arkv1api.PodVolumeRestoreStatus{
					Phase: arkv1api.PodVolumeRestorePhaseInProgress,
				},
			},
			shouldEnqueue: false,
		},
		{
			name: "Completed phase pvr should not be enqueued",
			obj: &arkv1api.PodVolumeRestore{
				Status: arkv1api.PodVolumeRestoreStatus{
					Phase: arkv1api.PodVolumeRestorePhaseCompleted,
				},
			},
			shouldEnqueue: false,
		},
		{
			name: "Failed phase pvr should not be enqueued",
			obj: &arkv1api.PodVolumeRestore{
				Status: arkv1api.PodVolumeRestoreStatus{
					Phase: arkv1api.PodVolumeRestorePhaseFailed,
				},
			},
			shouldEnqueue: false,
		},
		{
			name: "Unable to get pvr's pod should not be enqueued",
			obj: &arkv1api.PodVolumeRestore{
				Spec: arkv1api.PodVolumeRestoreSpec{
					Pod: corev1api.ObjectReference{
						Namespace: "ns-1",
						Name:      "pod-1",
					},
				},
				Status: arkv1api.PodVolumeRestoreStatus{
					Phase: "",
				},
			},
			shouldEnqueue: false,
		},
		{
			name: "Empty phase pvr with pod not on node running init container should not be enqueued",
			obj: &arkv1api.PodVolumeRestore{
				Spec: arkv1api.PodVolumeRestoreSpec{
					Pod: corev1api.ObjectReference{
						Namespace: "ns-1",
						Name:      "pod-1",
					},
				},
				Status: arkv1api.PodVolumeRestoreStatus{
					Phase: "",
				},
			},
			pod: &corev1api.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "ns-1",
					Name:      "pod-1",
				},
				Spec: corev1api.PodSpec{
					NodeName: "some-other-node",
					InitContainers: []corev1api.Container{
						{
							Name: restic.InitContainer,
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
			shouldEnqueue: false,
		},
		{
			name: "Empty phase pvr with pod on node not running init container should not be enqueued",
			obj: &arkv1api.PodVolumeRestore{
				Spec: arkv1api.PodVolumeRestoreSpec{
					Pod: corev1api.ObjectReference{
						Namespace: "ns-1",
						Name:      "pod-1",
					},
				},
				Status: arkv1api.PodVolumeRestoreStatus{
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
							Name: restic.InitContainer,
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
			shouldEnqueue: false,
		},
		{
			name: "Empty phase pvr with pod on node running init container should be enqueued",
			obj: &arkv1api.PodVolumeRestore{
				Spec: arkv1api.PodVolumeRestoreSpec{
					Pod: corev1api.ObjectReference{
						Namespace: "ns-1",
						Name:      "pod-1",
					},
				},
				Status: arkv1api.PodVolumeRestoreStatus{
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
							Name: restic.InitContainer,
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
			shouldEnqueue: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var (
				podInformer = cache.NewSharedIndexInformer(nil, new(corev1api.Pod), 0, cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc})
				c           = &podVolumeRestoreController{
					genericController: newGenericController("pod-volume-restore", arktest.NewLogger()),
					podLister:         corev1listers.NewPodLister(podInformer.GetIndexer()),
					nodeName:          controllerNode,
				}
			)

			if test.pod != nil {
				require.NoError(t, podInformer.GetStore().Add(test.pod))
			}

			c.pvrHandler(test.obj)

			if !test.shouldEnqueue {
				assert.Equal(t, 0, c.queue.Len())
				return
			}

			require.Equal(t, 1, c.queue.Len())
		})
	}
}

func TestPodHandler(t *testing.T) {
	controllerNode := "foo"

	tests := []struct {
		name              string
		pod               *corev1api.Pod
		podVolumeRestores []*arkv1api.PodVolumeRestore
		expectedEnqueues  sets.String
	}{
		{
			name: "pod on controller node running restic init container with multiple PVRs has new ones enqueued",
			pod: &corev1api.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "ns-1",
					Name:      "pod-1",
					UID:       types.UID("uid"),
				},
				Spec: corev1api.PodSpec{
					NodeName: controllerNode,
					InitContainers: []corev1api.Container{
						{
							Name: restic.InitContainer,
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
			podVolumeRestores: []*arkv1api.PodVolumeRestore{
				{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "ns-1",
						Name:      "pvr-1",
						Labels: map[string]string{
							arkv1api.PodUIDLabel: "uid",
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "ns-1",
						Name:      "pvr-2",
						Labels: map[string]string{
							arkv1api.PodUIDLabel: "uid",
						},
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "ns-1",
						Name:      "pvr-3",
						Labels: map[string]string{
							arkv1api.PodUIDLabel: "uid",
						},
					},
					Status: arkv1api.PodVolumeRestoreStatus{
						Phase: arkv1api.PodVolumeRestorePhaseInProgress,
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "ns-1",
						Name:      "pvr-4",
						Labels: map[string]string{
							arkv1api.PodUIDLabel: "some-other-pod",
						},
					},
				},
			},
			expectedEnqueues: sets.NewString("ns-1/pvr-1", "ns-1/pvr-2"),
		},
		{
			name: "pod on controller node not running restic init container doesn't have PVRs enqueued",
			pod: &corev1api.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "ns-1",
					Name:      "pod-1",
					UID:       types.UID("uid"),
				},
				Spec: corev1api.PodSpec{
					NodeName: controllerNode,
					InitContainers: []corev1api.Container{
						{
							Name: restic.InitContainer,
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
			podVolumeRestores: []*arkv1api.PodVolumeRestore{
				{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "ns-1",
						Name:      "pvr-1",
						Labels: map[string]string{
							arkv1api.PodUIDLabel: "uid",
						},
					},
				},
			},
		},
		{
			name: "pod not running on controller node doesn't have PVRs enqueued",
			pod: &corev1api.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "ns-1",
					Name:      "pod-1",
					UID:       types.UID("uid"),
				},
				Spec: corev1api.PodSpec{
					NodeName: "some-other-node",
					InitContainers: []corev1api.Container{
						{
							Name: restic.InitContainer,
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
			podVolumeRestores: []*arkv1api.PodVolumeRestore{
				{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "ns-1",
						Name:      "pvr-1",
						Labels: map[string]string{
							arkv1api.PodUIDLabel: "uid",
						},
					},
				},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var (
				client      = arkfake.NewSimpleClientset()
				informers   = arkinformers.NewSharedInformerFactory(client, 0)
				pvrInformer = informers.Ark().V1().PodVolumeRestores()
				c           = &podVolumeRestoreController{
					genericController:      newGenericController("pod-volume-restore", arktest.NewLogger()),
					podVolumeRestoreLister: arkv1listers.NewPodVolumeRestoreLister(pvrInformer.Informer().GetIndexer()),
					nodeName:               controllerNode,
				}
			)

			if len(test.podVolumeRestores) > 0 {
				for _, pvr := range test.podVolumeRestores {
					require.NoError(t, pvrInformer.Informer().GetStore().Add(pvr))
				}
			}

			c.podHandler(test.pod)

			require.Equal(t, len(test.expectedEnqueues), c.queue.Len())

			itemCount := c.queue.Len()

			for i := 0; i < itemCount; i++ {
				item, _ := c.queue.Get()
				assert.True(t, test.expectedEnqueues.Has(item.(string)))
			}
		})
	}
}

func TestIsPVRNew(t *testing.T) {
	pvr := &arkv1api.PodVolumeRestore{}

	expectationByStatus := map[arkv1api.PodVolumeRestorePhase]bool{
		"":                                       true,
		arkv1api.PodVolumeRestorePhaseNew:        true,
		arkv1api.PodVolumeRestorePhaseInProgress: false,
		arkv1api.PodVolumeRestorePhaseCompleted:  false,
		arkv1api.PodVolumeRestorePhaseFailed:     false,
	}

	for phase, expected := range expectationByStatus {
		pvr.Status.Phase = phase
		assert.Equal(t, expected, isPVRNew(pvr))
	}
}

func TestIsPodOnNode(t *testing.T) {
	pod := &corev1api.Pod{}
	assert.False(t, isPodOnNode(pod, "bar"))

	pod.Spec.NodeName = "foo"
	assert.False(t, isPodOnNode(pod, "bar"))

	pod.Spec.NodeName = "bar"
	assert.True(t, isPodOnNode(pod, "bar"))
}

func TestIsResticContainerRunning(t *testing.T) {
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
			name: "pod with running init container that's not restic should return false",
			pod: &corev1api.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "ns-1",
					Name:      "pod-1",
				},
				Spec: corev1api.PodSpec{
					InitContainers: []corev1api.Container{
						{
							Name: "non-restic-init",
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
			name: "pod with running restic init container that's not first should return false",
			pod: &corev1api.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "ns-1",
					Name:      "pod-1",
				},
				Spec: corev1api.PodSpec{
					InitContainers: []corev1api.Container{
						{
							Name: "non-restic-init",
						},
						{
							Name: restic.InitContainer,
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
			expected: false,
		},
		{
			name: "pod with restic init container as first initContainer that's not running should return false",
			pod: &corev1api.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "ns-1",
					Name:      "pod-1",
				},
				Spec: corev1api.PodSpec{
					InitContainers: []corev1api.Container{
						{
							Name: restic.InitContainer,
						},
						{
							Name: "non-restic-init",
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
			name: "pod with running restic init container as first initContainer should return true",
			pod: &corev1api.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "ns-1",
					Name:      "pod-1",
				},
				Spec: corev1api.PodSpec{
					InitContainers: []corev1api.Container{
						{
							Name: restic.InitContainer,
						},
						{
							Name: "non-restic-init",
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
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.expected, isResticInitContainerRunning(test.pod))
		})
	}
}
