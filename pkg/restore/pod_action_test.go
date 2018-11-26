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

package restore

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/runtime"

	arktest "github.com/heptio/ark/pkg/util/test"
)

func TestPodActionExecute(t *testing.T) {
	tests := []struct {
		name        string
		obj         runtime.Unstructured
		expectedErr bool
		expectedRes runtime.Unstructured
	}{
		{
			name:        "no spec should error",
			obj:         NewTestUnstructured().WithName("pod-1").Unstructured,
			expectedErr: true,
		},
		{
			name: "nodeName (only) should be deleted from spec",
			obj: NewTestUnstructured().WithName("pod-1").WithSpec("nodeName", "foo").
				WithSpecField("containers", []interface{}{}).
				Unstructured,
			expectedErr: false,
			expectedRes: NewTestUnstructured().WithName("pod-1").WithSpec("foo").
				WithSpecField("containers", []interface{}{}).
				Unstructured,
		},
		{
			name: "priority (only) should be deleted from spec",
			obj: NewTestUnstructured().WithName("pod-1").WithSpec("priority", "foo").
				WithSpecField("containers", []interface{}{}).
				Unstructured,
			expectedErr: false,
			expectedRes: NewTestUnstructured().WithName("pod-1").WithSpec("foo").
				WithSpecField("containers", []interface{}{}).
				Unstructured,
		},
		{
			name: "volumes matching prefix <service account name>-token- should be deleted",
			obj: NewTestUnstructured().WithName("pod-1").
				WithSpec("serviceAccountName", "foo").
				WithSpecField("volumes", []interface{}{
					map[string]interface{}{"name": "foo"},
					map[string]interface{}{"name": "foo-token-foo"},
				}).
				WithSpecField("containers", []interface{}{}).
				Unstructured,
			expectedErr: false,
			expectedRes: NewTestUnstructured().WithName("pod-1").
				WithSpec("serviceAccountName", "foo").
				WithSpecField("volumes", []interface{}{
					map[string]interface{}{"name": "foo"},
				}).
				WithSpecField("containers", []interface{}{}).
				Unstructured,
		},
		{
			name: "container volumeMounts matching prefix <service account name>-token- should be deleted",
			obj: NewTestUnstructured().WithName("svc-1").
				WithSpec("serviceAccountName", "foo").
				WithSpecField("volumes", []interface{}{
					map[string]interface{}{"name": "foo"},
					map[string]interface{}{"name": "foo-token-foo"},
				}).
				WithSpecField("containers", []interface{}{
					map[string]interface{}{
						"volumeMounts": []interface{}{
							map[string]interface{}{"name": "foo"},
							map[string]interface{}{"name": "foo-token-foo"},
						},
					},
				}).
				Unstructured,
			expectedErr: false,
			expectedRes: NewTestUnstructured().WithName("svc-1").
				WithSpec("serviceAccountName", "foo").
				WithSpecField("volumes", []interface{}{
					map[string]interface{}{"name": "foo"},
				}).
				WithSpecField("containers", []interface{}{
					map[string]interface{}{
						"volumeMounts": []interface{}{
							map[string]interface{}{"name": "foo"},
						},
					},
				}).
				Unstructured,
		},
		{
			name: "initContainer volumeMounts matching prefix <service account name>-token- should be deleted",
			obj: NewTestUnstructured().WithName("svc-1").
				WithSpec("serviceAccountName", "foo").
				WithSpecField("containers", []interface{}{}).
				WithSpecField("volumes", []interface{}{
					map[string]interface{}{"name": "foo"},
					map[string]interface{}{"name": "foo-token-foo"},
				}).
				WithSpecField("initContainers", []interface{}{
					map[string]interface{}{
						"volumeMounts": []interface{}{
							map[string]interface{}{"name": "foo"},
							map[string]interface{}{"name": "foo-token-foo"},
						},
					},
				}).
				Unstructured,
			expectedErr: false,
			expectedRes: NewTestUnstructured().WithName("svc-1").
				WithSpec("serviceAccountName", "foo").
				WithSpecField("containers", []interface{}{}).
				WithSpecField("volumes", []interface{}{
					map[string]interface{}{"name": "foo"},
				}).
				WithSpecField("initContainers", []interface{}{
					map[string]interface{}{
						"volumeMounts": []interface{}{
							map[string]interface{}{"name": "foo"},
						},
					},
				}).
				Unstructured,
		},
		{
			name: "containers and initContainers with no volume mounts should not error",
			obj: NewTestUnstructured().WithName("pod-1").
				WithSpec("serviceAccountName", "foo").
				WithSpecField("volumes", []interface{}{
					map[string]interface{}{"name": "foo"},
					map[string]interface{}{"name": "foo-token-foo"},
				}).
				WithSpecField("containers", []interface{}{}).
				WithSpecField("initContainers", []interface{}{}).
				Unstructured,
			expectedErr: false,
			expectedRes: NewTestUnstructured().WithName("pod-1").
				WithSpec("serviceAccountName", "foo").
				WithSpecField("volumes", []interface{}{
					map[string]interface{}{"name": "foo"},
				}).
				WithSpecField("containers", []interface{}{}).
				WithSpecField("initContainers", []interface{}{}).
				Unstructured,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			action := NewPodAction(arktest.NewLogger())

			res, warning, err := action.Execute(test.obj, nil)

			assert.Nil(t, warning)

			if test.expectedErr {
				assert.NotNil(t, err, "expected an error")
			} else {
				assert.Nil(t, err, "expected no error, got %v", err)
			}

			assert.Equal(t, test.expectedRes, res)
		})
	}
}
