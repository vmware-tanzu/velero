/*
Copyright 2022 the Velero contributors.

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
	"github.com/stretchr/testify/require"
	corev1api "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"

	velerov1api "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	"github.com/vmware-tanzu/velero/pkg/plugin/velero"
	velerotest "github.com/vmware-tanzu/velero/pkg/test"
)

func TestNamespaceActionAppliesTo(t *testing.T) {
	action := NewNamespaceAction(velerotest.NewLogger())
	actual, err := action.AppliesTo()
	require.NoError(t, err)
	assert.Equal(t, velero.ResourceSelector{IncludedResources: []string{"namespaces"}}, actual)
}

func TestNamespaceActionExecute(t *testing.T) {
	tests := []struct {
		name             string
		namespace        string
		namespaceMapping map[string]string
		expected         string
	}{
		{
			name:             "namespace mapping disabled",
			namespace:        "foo",
			namespaceMapping: map[string]string{},
			expected:         "foo",
		},
		{
			name:             "namespace mapping enabled",
			namespace:        "foo",
			namespaceMapping: map[string]string{"foo": "bar", "fizz": "buzz"},
			expected:         "bar",
		},
		{
			name:             "namespace mapping enabled, not included namespace remains unchanged",
			namespace:        "xyz",
			namespaceMapping: map[string]string{"foo": "bar", "fizz": "buzz"},
			expected:         "xyz",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			namespace := corev1api.Namespace{}
			namespace.SetName(tc.namespace)

			namespaceUnstructured, err := runtime.DefaultUnstructuredConverter.ToUnstructured(&namespace)
			require.NoError(t, err)

			action := NewNamespaceAction(velerotest.NewLogger())
			res, err := action.Execute(&velero.RestoreItemActionExecuteInput{
				Item:           &unstructured.Unstructured{Object: namespaceUnstructured},
				ItemFromBackup: &unstructured.Unstructured{Object: namespaceUnstructured},
				Restore: &velerov1api.Restore{
					Spec: velerov1api.RestoreSpec{
						NamespaceMapping: tc.namespaceMapping,
					},
				},
			})
			require.NoError(t, err)

			var actual *corev1api.Namespace
			err = runtime.DefaultUnstructuredConverter.FromUnstructured(res.UpdatedItem.UnstructuredContent(), &actual)
			require.NoError(t, err)

			assert.Equal(t, tc.expected, actual.GetName())
		})
	}
}
