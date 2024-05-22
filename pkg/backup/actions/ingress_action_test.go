/*
Copyright the Velero contributors.

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

package actions

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/vmware-tanzu/velero/pkg/kuberesource"
	"github.com/vmware-tanzu/velero/pkg/plugin/velero"
	velerotest "github.com/vmware-tanzu/velero/pkg/test"
)

func TestIngressActionAppliesTo(t *testing.T) {
	a := NewIngressAction(velerotest.NewLogger())

	actual, err := a.AppliesTo()
	require.NoError(t, err)

	expected := velero.ResourceSelector{
		IncludedResources: []string{"ingresses"},
	}
	assert.Equal(t, expected, actual)
}

func TestIngressActionExecute(t *testing.T) {
	tests := []struct {
		name     string
		ing      runtime.Unstructured
		expected []velero.ResourceIdentifier
	}{
		{
			name: "no spec.ingressClass",
			ing: velerotest.UnstructuredOrDie(`
			{
				"apiVersion": "networking.k8s.io/v1",
				"kind": "Ingress",
				"metadata": {
					"namespace": "ns",
					"name": "ing-1"
				}
			}
			`),
		},
		{
			name: "spec.ingressClass equals to nil string",
			ing: velerotest.UnstructuredOrDie(`
			{
				"apiVersion": "networking.k8s.io/v1",
				"kind": "Ingress",
				"metadata": {
					"namespace": "ns",
					"name": "ing-2"
				},
				"spec": {
					"ingressClassName": ""
				}
			}
			`),
		},
		{
			name: "spec.ingressClass exists",
			ing: velerotest.UnstructuredOrDie(`
			{
				"apiVersion": "networking.k8s.io/v1",
				"kind": "Ingress",
				"metadata": {
					"namespace": "ns",
					"name": "ing-3"
				},
				"spec": {
					"ingressClassName": "ingressclass"
				}
			}
			`),
			expected: []velero.ResourceIdentifier{
				{GroupResource: kuberesource.IngressClasses, Name: "ingressclass"},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			a := NewIngressAction(velerotest.NewLogger())

			updated, additionalItems, err := a.Execute(test.ing, nil)
			require.NoError(t, err)
			assert.Equal(t, test.ing, updated)
			assert.Equal(t, test.expected, additionalItems)
		})
	}
}
