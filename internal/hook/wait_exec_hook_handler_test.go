/*
Copyright 2020 the Velero contributors.

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

package hook

import (
	"context"
	"testing"
	"time"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/cache"
	fcache "k8s.io/client-go/tools/cache/testing"

	velerov1api "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	"github.com/vmware-tanzu/velero/pkg/builder"
	velerotest "github.com/vmware-tanzu/velero/pkg/test"
)

type fakeListWatchFactory struct {
	lw cache.ListerWatcher
}

func (f *fakeListWatchFactory) NewListWatch(ns string, selector fields.Selector) cache.ListerWatcher {
	return f.lw
}

var _ ListWatchFactory = &fakeListWatchFactory{}

func TestWaitExecHandleHooks(t *testing.T) {
	type change struct {
		// delta to wait since last change applied or pod added
		wait    time.Duration
		updated *v1.Pod
	}
	type expectedExecution struct {
		hook  *velerov1api.ExecHook
		name  string
		error error
		pod   *v1.Pod
	}
	tests := []struct {
		name string
		// Used as argument to HandleHooks and first state added to ListerWatcher
		initialPod         *v1.Pod
		groupResource      string
		byContainer        map[string][]PodExecRestoreHook
		expectedExecutions []expectedExecution
		expectedErrors     []error
		// changes represents the states of the pod over time. It can be used to test a container
		// becoming ready at some point after it is first observed by the controller.
		changes                   []change
		sharedHooksContextTimeout time.Duration
	}{
		{
			name: "should return no error when hook from annotation executes successfully",
			initialPod: builder.ForPod("default", "my-pod").
				ObjectMeta(builder.WithAnnotations(
					podRestoreHookCommandAnnotationKey, "/usr/bin/foo",
					podRestoreHookContainerAnnotationKey, "container1",
					podRestoreHookOnErrorAnnotationKey, string(velerov1api.HookErrorModeContinue),
					podRestoreHookTimeoutAnnotationKey, "1s",
					podRestoreHookWaitTimeoutAnnotationKey, "1m",
				)).
				Containers(&v1.Container{
					Name: "container1",
				}).
				ContainerStatuses(&v1.ContainerStatus{
					Name: "container1",
					State: v1.ContainerState{
						Running: &v1.ContainerStateRunning{},
					},
				}).
				Result(),
			groupResource: "pods",
			byContainer: map[string][]PodExecRestoreHook{
				"container1": {
					{
						HookName:   "<from-annotation>",
						HookSource: "annotation",
						Hook: velerov1api.ExecRestoreHook{
							Container:   "container1",
							Command:     []string{"/usr/bin/foo"},
							OnError:     velerov1api.HookErrorModeContinue,
							ExecTimeout: metav1.Duration{time.Second},
							WaitTimeout: metav1.Duration{time.Minute},
						},
					},
				},
			},
			expectedExecutions: []expectedExecution{
				{
					name: "<from-annotation>",
					hook: &velerov1api.ExecHook{
						Container: "container1",
						Command:   []string{"/usr/bin/foo"},
						OnError:   velerov1api.HookErrorModeContinue,
						Timeout:   metav1.Duration{time.Second},
					},
					error: nil,
					pod: builder.ForPod("default", "my-pod").
						ObjectMeta(builder.WithResourceVersion("1")).
						ObjectMeta(builder.WithAnnotations(
							podRestoreHookCommandAnnotationKey, "/usr/bin/foo",
							podRestoreHookContainerAnnotationKey, "container1",
							podRestoreHookOnErrorAnnotationKey, string(velerov1api.HookErrorModeContinue),
							podRestoreHookTimeoutAnnotationKey, "1s",
							podRestoreHookWaitTimeoutAnnotationKey, "1m",
						)).
						Containers(&v1.Container{
							Name: "container1",
						}).
						ContainerStatuses(&v1.ContainerStatus{
							Name: "container1",
							State: v1.ContainerState{
								Running: &v1.ContainerStateRunning{},
							},
						}).
						Result(),
				},
			},
			expectedErrors: nil,
		},
		{
			name: "should return an error when hook from annotation fails with on error mode fail",
			initialPod: builder.ForPod("default", "my-pod").
				ObjectMeta(builder.WithAnnotations(
					podRestoreHookCommandAnnotationKey, "/usr/bin/foo",
					podRestoreHookContainerAnnotationKey, "container1",
					podRestoreHookOnErrorAnnotationKey, string(velerov1api.HookErrorModeFail),
					podRestoreHookTimeoutAnnotationKey, "1s",
					podRestoreHookWaitTimeoutAnnotationKey, "1m",
				)).
				Containers(&v1.Container{
					Name: "container1",
				}).
				ContainerStatuses(&v1.ContainerStatus{
					Name: "container1",
					State: v1.ContainerState{
						Running: &v1.ContainerStateRunning{},
					},
				}).
				Result(),
			groupResource: "pods",
			byContainer: map[string][]PodExecRestoreHook{
				"container1": {
					{
						HookName:   "<from-annotation>",
						HookSource: "annotation",
						Hook: velerov1api.ExecRestoreHook{
							Container:   "container1",
							Command:     []string{"/usr/bin/foo"},
							OnError:     velerov1api.HookErrorModeFail,
							ExecTimeout: metav1.Duration{time.Second},
							WaitTimeout: metav1.Duration{time.Minute},
						},
					},
				},
			},
			expectedExecutions: []expectedExecution{
				{
					name: "<from-annotation>",
					hook: &velerov1api.ExecHook{
						Container: "container1",
						Command:   []string{"/usr/bin/foo"},
						OnError:   velerov1api.HookErrorModeFail,
						Timeout:   metav1.Duration{time.Second},
					},
					error: errors.New("pod hook error"),
					pod: builder.ForPod("default", "my-pod").
						ObjectMeta(builder.WithResourceVersion("1")).
						ObjectMeta(builder.WithAnnotations(
							podRestoreHookCommandAnnotationKey, "/usr/bin/foo",
							podRestoreHookContainerAnnotationKey, "container1",
							podRestoreHookOnErrorAnnotationKey, string(velerov1api.HookErrorModeFail),
							podRestoreHookTimeoutAnnotationKey, "1s",
							podRestoreHookWaitTimeoutAnnotationKey, "1m",
						)).
						Containers(&v1.Container{
							Name: "container1",
						}).
						ContainerStatuses(&v1.ContainerStatus{
							Name: "container1",
							State: v1.ContainerState{
								Running: &v1.ContainerStateRunning{},
							},
						}).
						Result(),
				},
			},
			expectedErrors: []error{errors.New("pod hook error")},
		},
		{
			name: "should return no error when hook from annotation fails with on error mode continue",
			initialPod: builder.ForPod("default", "my-pod").
				ObjectMeta(builder.WithAnnotations(
					podRestoreHookCommandAnnotationKey, "/usr/bin/foo",
					podRestoreHookContainerAnnotationKey, "container1",
					podRestoreHookOnErrorAnnotationKey, string(velerov1api.HookErrorModeContinue),
					podRestoreHookTimeoutAnnotationKey, "1s",
					podRestoreHookWaitTimeoutAnnotationKey, "1m",
				)).
				Containers(&v1.Container{
					Name: "container1",
				}).
				ContainerStatuses(&v1.ContainerStatus{
					Name: "container1",
					State: v1.ContainerState{
						Running: &v1.ContainerStateRunning{},
					},
				}).
				Result(),
			groupResource: "pods",
			byContainer: map[string][]PodExecRestoreHook{
				"container1": {
					{
						HookName:   "<from-annotation>",
						HookSource: "annotation",
						Hook: velerov1api.ExecRestoreHook{
							Container:   "container1",
							Command:     []string{"/usr/bin/foo"},
							OnError:     velerov1api.HookErrorModeContinue,
							ExecTimeout: metav1.Duration{time.Second},
							WaitTimeout: metav1.Duration{time.Minute},
						},
					},
				},
			},
			expectedExecutions: []expectedExecution{
				{
					name: "<from-annotation>",
					hook: &velerov1api.ExecHook{
						Container: "container1",
						Command:   []string{"/usr/bin/foo"},
						OnError:   velerov1api.HookErrorModeContinue,
						Timeout:   metav1.Duration{time.Second},
					},
					error: errors.New("pod hook error"),
					pod: builder.ForPod("default", "my-pod").
						ObjectMeta(builder.WithResourceVersion("1")).
						ObjectMeta(builder.WithAnnotations(
							podRestoreHookCommandAnnotationKey, "/usr/bin/foo",
							podRestoreHookContainerAnnotationKey, "container1",
							podRestoreHookOnErrorAnnotationKey, string(velerov1api.HookErrorModeContinue),
							podRestoreHookTimeoutAnnotationKey, "1s",
							podRestoreHookWaitTimeoutAnnotationKey, "1m",
						)).
						Containers(&v1.Container{
							Name: "container1",
						}).
						ContainerStatuses(&v1.ContainerStatus{
							Name: "container1",
							State: v1.ContainerState{
								Running: &v1.ContainerStateRunning{},
							},
						}).
						Result(),
				},
			},
			expectedErrors: nil,
		},
		{
			name: "should return no error when hook from annotation executes after 10ms wait for container to start",
			initialPod: builder.ForPod("default", "my-pod").
				ObjectMeta(builder.WithAnnotations(
					podRestoreHookCommandAnnotationKey, "/usr/bin/foo",
					podRestoreHookContainerAnnotationKey, "container1",
					podRestoreHookOnErrorAnnotationKey, string(velerov1api.HookErrorModeContinue),
					podRestoreHookTimeoutAnnotationKey, "1s",
					podRestoreHookWaitTimeoutAnnotationKey, "1m",
				)).
				Containers(&v1.Container{
					Name: "container1",
				}).
				ContainerStatuses(&v1.ContainerStatus{
					Name: "container1",
					State: v1.ContainerState{
						Waiting: &v1.ContainerStateWaiting{},
					},
				}).
				Result(),
			groupResource: "pods",
			byContainer: map[string][]PodExecRestoreHook{
				"container1": {
					{
						HookName:   "<from-annotation>",
						HookSource: "annotation",
						Hook: velerov1api.ExecRestoreHook{
							Container:   "container1",
							Command:     []string{"/usr/bin/foo"},
							OnError:     velerov1api.HookErrorModeContinue,
							ExecTimeout: metav1.Duration{time.Second},
							WaitTimeout: metav1.Duration{time.Minute},
						},
					},
				},
			},
			expectedExecutions: []expectedExecution{
				{
					name: "<from-annotation>",
					hook: &velerov1api.ExecHook{
						Container: "container1",
						Command:   []string{"/usr/bin/foo"},
						OnError:   velerov1api.HookErrorModeContinue,
						Timeout:   metav1.Duration{time.Second},
					},
					error: nil,
					pod: builder.ForPod("default", "my-pod").
						ObjectMeta(builder.WithResourceVersion("2")).
						ObjectMeta(builder.WithAnnotations(
							podRestoreHookCommandAnnotationKey, "/usr/bin/foo",
							podRestoreHookContainerAnnotationKey, "container1",
							podRestoreHookOnErrorAnnotationKey, string(velerov1api.HookErrorModeContinue),
							podRestoreHookTimeoutAnnotationKey, "1s",
							podRestoreHookWaitTimeoutAnnotationKey, "1m",
						)).
						Containers(&v1.Container{
							Name: "container1",
						}).
						ContainerStatuses(&v1.ContainerStatus{
							Name: "container1",
							State: v1.ContainerState{
								Running: &v1.ContainerStateRunning{},
							},
						}).
						Result(),
				},
			},
			expectedErrors: nil,
			changes: []change{
				{
					wait: 10 * time.Millisecond,
					updated: builder.ForPod("default", "my-pod").
						ObjectMeta(builder.WithAnnotations(
							podRestoreHookCommandAnnotationKey, "/usr/bin/foo",
							podRestoreHookContainerAnnotationKey, "container1",
							podRestoreHookOnErrorAnnotationKey, string(velerov1api.HookErrorModeContinue),
							podRestoreHookTimeoutAnnotationKey, "1s",
							podRestoreHookWaitTimeoutAnnotationKey, "1m",
						)).
						Containers(&v1.Container{
							Name: "container1",
						}).
						ContainerStatuses(&v1.ContainerStatus{
							Name: "container1",
							State: v1.ContainerState{
								Running: &v1.ContainerStateRunning{},
							},
						}).
						Result(),
				},
			},
		},
		{
			name:          "should return no error when hook from spec executes successfully",
			groupResource: "pods",
			initialPod: builder.ForPod("default", "my-pod").
				Containers(&v1.Container{
					Name: "container1",
				}).
				ContainerStatuses(&v1.ContainerStatus{
					Name: "container1",
					State: v1.ContainerState{
						Running: &v1.ContainerStateRunning{},
					},
				}).
				Result(),
			expectedErrors: nil,
			byContainer: map[string][]PodExecRestoreHook{
				"container1": {
					{
						HookName:   "my-hook-1",
						HookSource: "backupSpec",
						Hook: velerov1api.ExecRestoreHook{
							Container: "container1",
							Command:   []string{"/usr/bin/foo"},
						},
					},
				},
			},
			expectedExecutions: []expectedExecution{
				{
					name: "my-hook-1",
					hook: &velerov1api.ExecHook{
						Container: "container1",
						Command:   []string{"/usr/bin/foo"},
					},
					pod: builder.ForPod("default", "my-pod").
						ObjectMeta(builder.WithResourceVersion("1")).
						Containers(&v1.Container{
							Name: "container1",
						}).
						ContainerStatuses(&v1.ContainerStatus{
							Name: "container1",
							State: v1.ContainerState{
								Running: &v1.ContainerStateRunning{},
							},
						}).
						Result(),
				},
			},
		},
		{
			name:          "should return no error when spec hook with wait timeout expires with OnError mode Continue",
			groupResource: "pods",
			initialPod: builder.ForPod("default", "my-pod").
				Containers(&v1.Container{
					Name: "container1",
				}).
				ContainerStatuses(&v1.ContainerStatus{
					Name: "container1",
					State: v1.ContainerState{
						Waiting: &v1.ContainerStateWaiting{},
					},
				}).
				Result(),
			expectedErrors: nil,
			byContainer: map[string][]PodExecRestoreHook{
				"container1": {
					{
						HookName:   "my-hook-1",
						HookSource: "backupSpec",
						Hook: velerov1api.ExecRestoreHook{
							Container:   "container1",
							Command:     []string{"/usr/bin/foo"},
							OnError:     velerov1api.HookErrorModeContinue,
							WaitTimeout: metav1.Duration{time.Millisecond},
						},
					},
				},
			},
			expectedExecutions: []expectedExecution{},
		},
		{
			name:          "should return an error when spec hook with wait timeout expires with OnError mode Fail",
			groupResource: "pods",
			initialPod: builder.ForPod("default", "my-pod").
				Containers(&v1.Container{
					Name: "container1",
				}).
				ContainerStatuses(&v1.ContainerStatus{
					Name: "container1",
					State: v1.ContainerState{
						Waiting: &v1.ContainerStateWaiting{},
					},
				}).
				Result(),
			expectedErrors: []error{errors.New("Hook my-hook-1 in container container1 in pod default/my-pod not executed: context deadline exceeded")},
			byContainer: map[string][]PodExecRestoreHook{
				"container1": {
					{
						HookName:   "my-hook-1",
						HookSource: "backupSpec",
						Hook: velerov1api.ExecRestoreHook{
							Container:   "container1",
							Command:     []string{"/usr/bin/foo"},
							OnError:     velerov1api.HookErrorModeFail,
							WaitTimeout: metav1.Duration{time.Millisecond},
						},
					},
				},
			},
			expectedExecutions: []expectedExecution{},
		},
		{
			name:          "should return an error when shared hooks context is canceled before spec hook with OnError mode Fail executes",
			groupResource: "pods",
			initialPod: builder.ForPod("default", "my-pod").
				Containers(&v1.Container{
					Name: "container1",
				}).
				ContainerStatuses(&v1.ContainerStatus{
					Name: "container1",
					State: v1.ContainerState{
						Waiting: &v1.ContainerStateWaiting{},
					},
				}).
				Result(),
			expectedErrors: []error{errors.New("Hook my-hook-1 in container container1 in pod default/my-pod not executed: context deadline exceeded")},
			byContainer: map[string][]PodExecRestoreHook{
				"container1": {
					{
						HookName:   "my-hook-1",
						HookSource: "backupSpec",
						Hook: velerov1api.ExecRestoreHook{
							Container: "container1",
							Command:   []string{"/usr/bin/foo"},
							OnError:   velerov1api.HookErrorModeFail,
						},
					},
				},
			},
			expectedExecutions:        []expectedExecution{},
			sharedHooksContextTimeout: time.Millisecond,
		},
		{
			name:           "should return no error when shared hooks context is canceled before spec hook with OnError mode Continue executes",
			expectedErrors: nil,
			groupResource:  "pods",
			initialPod: builder.ForPod("default", "my-pod").
				Containers(&v1.Container{
					Name: "container1",
				}).
				ContainerStatuses(&v1.ContainerStatus{
					Name: "container1",
					State: v1.ContainerState{
						Waiting: &v1.ContainerStateWaiting{},
					},
				}).
				Result(),
			byContainer: map[string][]PodExecRestoreHook{
				"container1": {
					{
						HookName:   "my-hook-1",
						HookSource: "backupSpec",
						Hook: velerov1api.ExecRestoreHook{
							Container: "container1",
							Command:   []string{"/usr/bin/foo"},
							OnError:   velerov1api.HookErrorModeContinue,
						},
					},
				},
			},
			expectedExecutions:        []expectedExecution{},
			sharedHooksContextTimeout: time.Millisecond,
		},
		{
			name:          "should return no error with 2 spec hooks in 2 different containers, 1st container starts running after 10ms, 2nd container after 20ms, both succeed",
			groupResource: "pods",
			initialPod: builder.ForPod("default", "my-pod").
				Containers(&v1.Container{
					Name: "container1",
				}).
				Containers(&v1.Container{
					Name: "container2",
				}).
				// initially both are waiting
				ContainerStatuses(&v1.ContainerStatus{
					Name: "container1",
					State: v1.ContainerState{
						Waiting: &v1.ContainerStateWaiting{},
					},
				}).
				ContainerStatuses(&v1.ContainerStatus{
					Name: "container2",
					State: v1.ContainerState{
						Waiting: &v1.ContainerStateWaiting{},
					},
				}).
				Result(),
			expectedErrors: nil,
			byContainer: map[string][]PodExecRestoreHook{
				"container1": {
					{
						HookName:   "my-hook-1",
						HookSource: "backupSpec",
						Hook: velerov1api.ExecRestoreHook{
							Container: "container1",
							Command:   []string{"/usr/bin/foo"},
						},
					},
				},
				"container2": {
					{
						HookName:   "my-hook-1",
						HookSource: "backupSpec",
						Hook: velerov1api.ExecRestoreHook{
							Container: "container2",
							Command:   []string{"/usr/bin/bar"},
						},
					},
				},
			},
			expectedExecutions: []expectedExecution{
				{
					name: "my-hook-1",
					hook: &velerov1api.ExecHook{
						Container: "container1",
						Command:   []string{"/usr/bin/foo"},
					},
					error: nil,
					pod: builder.ForPod("default", "my-pod").
						ObjectMeta(builder.WithResourceVersion("2")).
						Containers(&v1.Container{
							Name: "container1",
						}).
						Containers(&v1.Container{
							Name: "container2",
						}).
						ContainerStatuses(&v1.ContainerStatus{
							Name: "container1",
							State: v1.ContainerState{
								Running: &v1.ContainerStateRunning{},
							},
						}).
						// container 2 is still waiting when the first hook executes in container1
						ContainerStatuses(&v1.ContainerStatus{
							Name: "container2",
							State: v1.ContainerState{
								Waiting: &v1.ContainerStateWaiting{},
							},
						}).
						Result(),
				},
				{
					name: "my-hook-1",
					hook: &velerov1api.ExecHook{
						Container: "container2",
						Command:   []string{"/usr/bin/bar"},
					},
					error: nil,
					pod: builder.ForPod("default", "my-pod").
						ObjectMeta(builder.WithResourceVersion("3")).
						Containers(&v1.Container{
							Name: "container1",
						}).
						Containers(&v1.Container{
							Name: "container2",
						}).
						ContainerStatuses(&v1.ContainerStatus{
							Name: "container1",
							State: v1.ContainerState{
								Running: &v1.ContainerStateRunning{},
							},
						}).
						ContainerStatuses(&v1.ContainerStatus{
							Name: "container2",
							State: v1.ContainerState{
								Running: &v1.ContainerStateRunning{},
							},
						}).
						Result(),
				},
			},
			changes: []change{
				// 1st modification: container1 starts running, resourceVersion 2, container2 still waiting
				{
					wait: 10 * time.Millisecond,
					updated: builder.ForPod("default", "my-pod").
						ObjectMeta(builder.WithResourceVersion("2")).
						Containers(&v1.Container{
							Name: "container1",
						}).
						Containers(&v1.Container{
							Name: "container2",
						}).
						ContainerStatuses(&v1.ContainerStatus{
							Name: "container1",
							State: v1.ContainerState{
								Running: &v1.ContainerStateRunning{},
							},
						}).
						ContainerStatuses(&v1.ContainerStatus{
							Name: "container2",
							State: v1.ContainerState{
								Waiting: &v1.ContainerStateWaiting{},
							},
						}).
						Result(),
				},
				// 2nd modification: container2 starts running, resourceVersion 3
				{
					wait: 10 * time.Millisecond,
					updated: builder.ForPod("default", "my-pod").
						ObjectMeta(builder.WithResourceVersion("3")).
						Containers(&v1.Container{
							Name: "container1",
						}).
						Containers(&v1.Container{
							Name: "container2",
						}).
						ContainerStatuses(&v1.ContainerStatus{
							Name: "container1",
							State: v1.ContainerState{
								Running: &v1.ContainerStateRunning{},
							},
						}).
						ContainerStatuses(&v1.ContainerStatus{
							Name: "container2",
							State: v1.ContainerState{
								Running: &v1.ContainerStateRunning{},
							},
						}).
						Result(),
				},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {

			source := fcache.NewFakeControllerSource()
			go func() {
				// This is the state of the pod that will be seen by the AddFunc handler.
				source.Add(test.initialPod)
				// Changes holds the versions of the pod over time. Each of these states
				// will be seen by the UpdateFunc handler.
				for _, change := range test.changes {
					time.Sleep(change.wait)
					source.Modify(change.updated)
				}
			}()

			podCommandExecutor := &velerotest.MockPodCommandExecutor{}
			defer podCommandExecutor.AssertExpectations(t)

			h := &DefaultWaitExecHookHandler{
				PodCommandExecutor: podCommandExecutor,
				ListWatchFactory:   &fakeListWatchFactory{source},
			}

			for _, e := range test.expectedExecutions {
				obj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(e.pod)
				assert.Nil(t, err)
				podCommandExecutor.On("ExecutePodCommand", mock.Anything, obj, e.pod.Namespace, e.pod.Name, e.name, e.hook).Return(e.error)
			}

			ctx := context.Background()
			if test.sharedHooksContextTimeout > 0 {
				ctx, _ = context.WithTimeout(ctx, test.sharedHooksContextTimeout)
			}

			errs := h.HandleHooks(ctx, velerotest.NewLogger(), test.initialPod, test.byContainer)

			// for i, ee := range test.expectedErrors {
			require.Len(t, errs, len(test.expectedErrors))
			for i, ee := range test.expectedErrors {
				assert.EqualError(t, errs[i], ee.Error())
			}
		})
	}
}

func TestPodHasContainer(t *testing.T) {
	tests := []struct {
		name      string
		pod       *v1.Pod
		container string
		expect    bool
	}{
		{
			name:      "has container",
			expect:    true,
			container: "container1",
			pod: builder.ForPod("default", "my-pod").
				Containers(&v1.Container{
					Name: "container1",
				}).
				Result(),
		},
		{
			name:      "does not have container",
			expect:    false,
			container: "container1",
			pod: builder.ForPod("default", "my-pod").
				Containers(&v1.Container{
					Name: "container2",
				}).
				Result(),
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			actual := podHasContainer(test.pod, test.container)
			assert.Equal(t, actual, test.expect)
		})
	}
}

func TestIsContainerRunning(t *testing.T) {
	tests := []struct {
		name      string
		pod       *v1.Pod
		container string
		expect    bool
	}{
		{
			name:      "should return true when running",
			container: "container1",
			expect:    true,
			pod: builder.ForPod("default", "my-pod").
				ContainerStatuses(&v1.ContainerStatus{
					Name: "container1",
					State: v1.ContainerState{
						Running: &v1.ContainerStateRunning{},
					},
				}).
				Result(),
		},
		{
			name:      "should return false when no state is set",
			container: "container1",
			expect:    false,
			pod: builder.ForPod("default", "my-pod").
				ContainerStatuses(&v1.ContainerStatus{
					Name:  "container1",
					State: v1.ContainerState{},
				}).
				Result(),
		},
		{
			name:      "should return false when waiting",
			container: "container1",
			expect:    false,
			pod: builder.ForPod("default", "my-pod").
				ContainerStatuses(&v1.ContainerStatus{
					Name: "container1",
					State: v1.ContainerState{
						Waiting: &v1.ContainerStateWaiting{},
					},
				}).
				Result(),
		},
		{
			name:      "should return true when running and first container is terminated",
			container: "container1",
			expect:    true,
			pod: builder.ForPod("default", "my-pod").
				ContainerStatuses(&v1.ContainerStatus{
					Name: "container0",
					State: v1.ContainerState{
						Terminated: &v1.ContainerStateTerminated{},
					},
				},
					&v1.ContainerStatus{
						Name: "container1",
						State: v1.ContainerState{
							Running: &v1.ContainerStateRunning{},
						},
					}).
				Result(),
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			actual := isContainerRunning(test.pod, test.container)
			assert.Equal(t, actual, test.expect)
		})
	}
}

func TestMaxHookWait(t *testing.T) {
	tests := []struct {
		name        string
		byContainer map[string][]PodExecRestoreHook
		expect      time.Duration
	}{
		{
			name:        "should return 0 for nil map",
			byContainer: nil,
			expect:      0,
		},
		{
			name:   "should return 0 if all hooks are 0 or negative",
			expect: 0,
			byContainer: map[string][]PodExecRestoreHook{
				"container1": {
					{
						Hook: velerov1api.ExecRestoreHook{
							ExecTimeout: metav1.Duration{time.Second},
							WaitTimeout: metav1.Duration{0},
						},
					},
					{
						Hook: velerov1api.ExecRestoreHook{
							WaitTimeout: metav1.Duration{-1},
						},
					},
				},
			},
		},
		{
			name:   "should return biggest wait timeout from multiple hooks in multiple containers",
			expect: time.Hour,
			byContainer: map[string][]PodExecRestoreHook{
				"container1": {
					{
						Hook: velerov1api.ExecRestoreHook{
							WaitTimeout: metav1.Duration{time.Second},
						},
					},
					{
						Hook: velerov1api.ExecRestoreHook{
							WaitTimeout: metav1.Duration{time.Second},
						},
					},
				},
				"container2": {
					{
						Hook: velerov1api.ExecRestoreHook{
							WaitTimeout: metav1.Duration{time.Hour},
						},
					},
					{
						Hook: velerov1api.ExecRestoreHook{
							WaitTimeout: metav1.Duration{time.Minute},
						},
					},
				},
			},
		},
		{
			name:   "should return 0 if any hook does not have a wait timeout",
			expect: 0,
			byContainer: map[string][]PodExecRestoreHook{
				"container1": {
					{
						Hook: velerov1api.ExecRestoreHook{
							ExecTimeout: metav1.Duration{time.Second},
							WaitTimeout: metav1.Duration{time.Second},
						},
					},
					{
						Hook: velerov1api.ExecRestoreHook{
							WaitTimeout: metav1.Duration{0},
						},
					},
				},
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			actual := maxHookWait(test.byContainer)
			assert.Equal(t, actual, test.expect)
		})
	}
}
