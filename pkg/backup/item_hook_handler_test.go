/*
Copyright 2017 the Heptio Ark contributors.

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

package backup

import (
	"testing"
	"time"

	"github.com/heptio/ark/pkg/apis/ark/v1"
	"github.com/heptio/ark/pkg/util/collections"
	arktest "github.com/heptio/ark/pkg/util/test"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

type mockItemHookHandler struct {
	mock.Mock
}

func (h *mockItemHookHandler) handleHooks(log *logrus.Entry, groupResource schema.GroupResource, obj runtime.Unstructured, resourceHooks []resourceHook) error {
	args := h.Called(log, groupResource, obj, resourceHooks)
	return args.Error(0)
}

func TestHandleHooksSkips(t *testing.T) {
	tests := []struct {
		name          string
		groupResource string
		item          runtime.Unstructured
		hooks         []resourceHook
	}{
		{
			name:          "not a pod",
			groupResource: "widget.group",
		},
		{
			name: "pod without annotation / no spec hooks",
			item: unstructuredOrDie(
				`
				{
					"apiVersion": "v1",
					"kind": "Pod",
					"metadata": {
						"namespace": "ns",
						"name": "foo"
					}
				}
				`,
			),
		},
		{
			name:          "spec hooks not applicable",
			groupResource: "pods",
			item: unstructuredOrDie(
				`
				{
					"apiVersion": "v1",
					"kind": "Pod",
					"metadata": {
						"namespace": "ns",
						"name": "foo",
						"labels": {
							"color": "blue"
						}
					}
				}
				`,
			),
			hooks: []resourceHook{
				{
					name:       "ns exclude",
					namespaces: collections.NewIncludesExcludes().Excludes("ns"),
				},
				{
					name:      "resource exclude",
					resources: collections.NewIncludesExcludes().Includes("widgets.group"),
				},
				{
					name:          "label selector mismatch",
					labelSelector: parseLabelSelectorOrDie("color=green"),
				},
				{
					name: "missing exec hook",
					hooks: []v1.BackupResourceHook{
						{},
						{},
					},
				},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			podCommandExecutor := &mockPodCommandExecutor{}
			defer podCommandExecutor.AssertExpectations(t)

			h := &defaultItemHookHandler{
				podCommandExecutor: podCommandExecutor,
			}

			groupResource := schema.ParseGroupResource(test.groupResource)
			err := h.handleHooks(arktest.NewLogger(), groupResource, test.item, test.hooks)
			assert.NoError(t, err)
		})
	}
}

func TestHandleHooksPodFromPodAnnotation(t *testing.T) {
	tests := []struct {
		name                  string
		groupResource         string
		item                  runtime.Unstructured
		hooks                 []resourceHook
		hookErrorsByContainer map[string]error
		expectedError         error
		expectedPodHook       *v1.ExecHook
		expectedPodHookError  error
	}{
		{
			name:          "pod, no annotation, spec (multiple hooks) = run spec",
			groupResource: "pods",
			item: unstructuredOrDie(`
		{
			"apiVersion": "v1",
			"kind": "Pod",
			"metadata": {
				"namespace": "ns",
				"name": "name"
			}
		}`),
			hooks: []resourceHook{
				{
					name: "hook1",
					hooks: []v1.BackupResourceHook{
						{
							Exec: &v1.ExecHook{
								Container: "1a",
								Command:   []string{"1a"},
							},
						},
						{
							Exec: &v1.ExecHook{
								Container: "1b",
								Command:   []string{"1b"},
							},
						},
					},
				},
				{
					name: "hook2",
					hooks: []v1.BackupResourceHook{
						{
							Exec: &v1.ExecHook{
								Container: "2a",
								Command:   []string{"2a"},
							},
						},
						{
							Exec: &v1.ExecHook{
								Container: "2b",
								Command:   []string{"2b"},
							},
						},
					},
				},
			},
		},
		{
			name:          "pod, annotation, no spec = run annotation",
			groupResource: "pods",
			item: unstructuredOrDie(`
		{
			"apiVersion": "v1",
			"kind": "Pod",
			"metadata": {
				"namespace": "ns",
				"name": "name",
				"annotations": {
					"hook.backup.ark.heptio.com/container": "c",
					"hook.backup.ark.heptio.com/command": "/bin/ls"
				}
			}
		}`),
			expectedPodHook: &v1.ExecHook{
				Container: "c",
				Command:   []string{"/bin/ls"},
			},
		},
		{
			name:          "pod, annotation & spec = run annotation",
			groupResource: "pods",
			item: unstructuredOrDie(`
		{
			"apiVersion": "v1",
			"kind": "Pod",
			"metadata": {
				"namespace": "ns",
				"name": "name",
				"annotations": {
					"hook.backup.ark.heptio.com/container": "c",
					"hook.backup.ark.heptio.com/command": "/bin/ls"
				}
			}
		}`),
			expectedPodHook: &v1.ExecHook{
				Container: "c",
				Command:   []string{"/bin/ls"},
			},
			hooks: []resourceHook{
				{
					name: "hook1",
					hooks: []v1.BackupResourceHook{
						{
							Exec: &v1.ExecHook{
								Container: "1a",
								Command:   []string{"1a"},
							},
						},
					},
				},
			},
		},
		{
			name:          "pod, annotation, onError=fail = return error",
			groupResource: "pods",
			item: unstructuredOrDie(`
		{
			"apiVersion": "v1",
			"kind": "Pod",
			"metadata": {
				"namespace": "ns",
				"name": "name",
				"annotations": {
					"hook.backup.ark.heptio.com/container": "c",
					"hook.backup.ark.heptio.com/command": "/bin/ls",
					"hook.backup.ark.heptio.com/on-error": "Fail"
				}
			}
		}`),
			expectedPodHook: &v1.ExecHook{
				Container: "c",
				Command:   []string{"/bin/ls"},
				OnError:   v1.HookErrorModeFail,
			},
			expectedPodHookError: errors.New("pod hook error"),
			expectedError:        errors.New("pod hook error"),
		},
		{
			name:          "pod, annotation, onError=continue = return nil",
			groupResource: "pods",
			item: unstructuredOrDie(`
		{
			"apiVersion": "v1",
			"kind": "Pod",
			"metadata": {
				"namespace": "ns",
				"name": "name",
				"annotations": {
					"hook.backup.ark.heptio.com/container": "c",
					"hook.backup.ark.heptio.com/command": "/bin/ls",
					"hook.backup.ark.heptio.com/on-error": "Continue"
				}
			}
		}`),
			expectedPodHook: &v1.ExecHook{
				Container: "c",
				Command:   []string{"/bin/ls"},
				OnError:   v1.HookErrorModeContinue,
			},
			expectedPodHookError: errors.New("pod hook error"),
			expectedError:        nil,
		},
		{
			name:          "pod, spec, onError=fail = don't run other hooks",
			groupResource: "pods",
			item: unstructuredOrDie(`
		{
			"apiVersion": "v1",
			"kind": "Pod",
			"metadata": {
				"namespace": "ns",
				"name": "name"
			}
		}`),
			hooks: []resourceHook{
				{
					name: "hook1",
					hooks: []v1.BackupResourceHook{
						{
							Exec: &v1.ExecHook{
								Container: "1a",
								Command:   []string{"1a"},
								OnError:   v1.HookErrorModeContinue,
							},
						},
						{
							Exec: &v1.ExecHook{
								Container: "1b",
								Command:   []string{"1b"},
							},
						},
					},
				},
				{
					name: "hook2",
					hooks: []v1.BackupResourceHook{
						{
							Exec: &v1.ExecHook{
								Container: "2",
								Command:   []string{"2"},
								OnError:   v1.HookErrorModeFail,
							},
						},
					},
				},
				{
					name: "hook3",
					hooks: []v1.BackupResourceHook{
						{
							Exec: &v1.ExecHook{
								Container: "3",
								Command:   []string{"3"},
							},
						},
					},
				},
			},
			hookErrorsByContainer: map[string]error{
				"1a": errors.New("1a error, but continue"),
				"2":  errors.New("2 error, fail"),
			},
			expectedError: errors.New("2 error, fail"),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			podCommandExecutor := &mockPodCommandExecutor{}
			defer podCommandExecutor.AssertExpectations(t)

			h := &defaultItemHookHandler{
				podCommandExecutor: podCommandExecutor,
			}

			if test.expectedPodHook != nil {
				podCommandExecutor.On("executePodCommand", mock.Anything, test.item.UnstructuredContent(), "ns", "name", "<from-annotation>", test.expectedPodHook).Return(test.expectedPodHookError)
			} else {
			hookLoop:
				for _, resourceHook := range test.hooks {
					for _, hook := range resourceHook.hooks {
						hookError := test.hookErrorsByContainer[hook.Exec.Container]
						podCommandExecutor.On("executePodCommand", mock.Anything, test.item.UnstructuredContent(), "ns", "name", resourceHook.name, hook.Exec).Return(hookError)
						if hookError != nil && hook.Exec.OnError == v1.HookErrorModeFail {
							break hookLoop
						}
					}
				}
			}

			groupResource := schema.ParseGroupResource(test.groupResource)
			err := h.handleHooks(arktest.NewLogger(), groupResource, test.item, test.hooks)

			if test.expectedError != nil {
				assert.EqualError(t, err, test.expectedError.Error())
				return
			}

			require.NoError(t, err)
		})
	}
}

func TestGetPodExecHookFromAnnotations(t *testing.T) {
	tests := []struct {
		name         string
		annotations  map[string]string
		expectedHook *v1.ExecHook
	}{
		{
			name:         "missing command annotation",
			expectedHook: nil,
		},
		{
			name: "malformed command json array",
			annotations: map[string]string{
				podBackupHookCommandAnnotationKey: "[blarg",
			},
			expectedHook: &v1.ExecHook{
				Command: []string{"[blarg"},
			},
		},
		{
			name: "valid command json array",
			annotations: map[string]string{
				podBackupHookCommandAnnotationKey: `["a","b","c"]`,
			},
			expectedHook: &v1.ExecHook{
				Command: []string{"a", "b", "c"},
			},
		},
		{
			name: "command as a string",
			annotations: map[string]string{
				podBackupHookCommandAnnotationKey: "/usr/bin/foo",
			},
			expectedHook: &v1.ExecHook{
				Command: []string{"/usr/bin/foo"},
			},
		},
		{
			name: "hook mode set to continue",
			annotations: map[string]string{
				podBackupHookCommandAnnotationKey: "/usr/bin/foo",
				podBackupHookOnErrorAnnotationKey: string(v1.HookErrorModeContinue),
			},
			expectedHook: &v1.ExecHook{
				Command: []string{"/usr/bin/foo"},
				OnError: v1.HookErrorModeContinue,
			},
		},
		{
			name: "hook mode set to fail",
			annotations: map[string]string{
				podBackupHookCommandAnnotationKey: "/usr/bin/foo",
				podBackupHookOnErrorAnnotationKey: string(v1.HookErrorModeFail),
			},
			expectedHook: &v1.ExecHook{
				Command: []string{"/usr/bin/foo"},
				OnError: v1.HookErrorModeFail,
			},
		},
		{
			name: "use the specified timeout",
			annotations: map[string]string{
				podBackupHookCommandAnnotationKey: "/usr/bin/foo",
				podBackupHookTimeoutAnnotationKey: "5m3s",
			},
			expectedHook: &v1.ExecHook{
				Command: []string{"/usr/bin/foo"},
				Timeout: metav1.Duration{Duration: 5*time.Minute + 3*time.Second},
			},
		},
		{
			name: "invalid timeout is ignored",
			annotations: map[string]string{
				podBackupHookCommandAnnotationKey: "/usr/bin/foo",
				podBackupHookTimeoutAnnotationKey: "invalid",
			},
			expectedHook: &v1.ExecHook{
				Command: []string{"/usr/bin/foo"},
			},
		},
		{
			name: "use the specified container",
			annotations: map[string]string{
				podBackupHookContainerAnnotationKey: "some-container",
				podBackupHookCommandAnnotationKey:   "/usr/bin/foo",
			},
			expectedHook: &v1.ExecHook{
				Container: "some-container",
				Command:   []string{"/usr/bin/foo"},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			hook := getPodExecHookFromAnnotations(test.annotations)
			assert.Equal(t, test.expectedHook, hook)
		})
	}
}

func TestResourceHookApplicableTo(t *testing.T) {
	tests := []struct {
		name               string
		includedNamespaces []string
		excludedNamespaces []string
		includedResources  []string
		excludedResources  []string
		labelSelector      string
		namespace          string
		resource           schema.GroupResource
		labels             labels.Set
		expected           bool
	}{
		{
			name:      "allow anything",
			namespace: "foo",
			resource:  schema.GroupResource{Group: "foo", Resource: "bar"},
			expected:  true,
		},
		{
			name:               "namespace in included list",
			includedNamespaces: []string{"a", "b"},
			excludedNamespaces: []string{"c", "d"},
			namespace:          "b",
			expected:           true,
		},
		{
			name:               "namespace not in included list",
			includedNamespaces: []string{"a", "b"},
			namespace:          "c",
			expected:           false,
		},
		{
			name:               "namespace excluded",
			excludedNamespaces: []string{"a", "b"},
			namespace:          "a",
			expected:           false,
		},
		{
			name:              "resource in included list",
			includedResources: []string{"foo.a", "bar.b"},
			excludedResources: []string{"baz.c"},
			resource:          schema.GroupResource{Group: "a", Resource: "foo"},
			expected:          true,
		},
		{
			name:              "resource not in included list",
			includedResources: []string{"foo.a", "bar.b"},
			resource:          schema.GroupResource{Group: "c", Resource: "baz"},
			expected:          false,
		},
		{
			name:              "resource excluded",
			excludedResources: []string{"foo.a", "bar.b"},
			resource:          schema.GroupResource{Group: "b", Resource: "bar"},
			expected:          false,
		},
		{
			name:          "label selector matches",
			labelSelector: "a=b",
			labels:        labels.Set{"a": "b"},
			expected:      true,
		},
		{
			name:          "label selector doesn't match",
			labelSelector: "a=b",
			labels:        labels.Set{"a": "c"},
			expected:      false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			h := resourceHook{
				namespaces: collections.NewIncludesExcludes().Includes(test.includedNamespaces...).Excludes(test.excludedNamespaces...),
				resources:  collections.NewIncludesExcludes().Includes(test.includedResources...).Excludes(test.excludedResources...),
			}
			if test.labelSelector != "" {
				selector, err := labels.Parse(test.labelSelector)
				require.NoError(t, err)
				h.labelSelector = selector
			}

			result := h.applicableTo(test.resource, test.namespace, test.labels)
			assert.Equal(t, test.expected, result)
		})
	}
}
