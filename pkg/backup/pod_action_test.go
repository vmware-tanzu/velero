/*
Copyright 2018, 2019 the Velero contributors.

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

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/runtime"
	kubefake "k8s.io/client-go/kubernetes/fake"

	"github.com/vmware-tanzu/velero/pkg/builder"
	"github.com/vmware-tanzu/velero/pkg/kuberesource"
	"github.com/vmware-tanzu/velero/pkg/plugin/velero"
	velerotest "github.com/vmware-tanzu/velero/pkg/test"
)

func TestPodActionAppliesTo(t *testing.T) {
	a := NewPodAction(velerotest.NewLogger(), nil)

	actual, err := a.AppliesTo()
	require.NoError(t, err)

	expected := velero.ResourceSelector{
		IncludedResources: []string{"pods"},
	}
	assert.Equal(t, expected, actual)
}

func TestPodActionExecute(t *testing.T) {
	tests := []struct {
		name     string
		pod      runtime.Unstructured
		pvcs     []runtime.Object
		expected []velero.ResourceIdentifier
	}{
		{
			name: "no spec.volumes",
			pod: velerotest.UnstructuredOrDie(`
			{
				"apiVersion": "v1",
				"kind": "Pod",
				"metadata": {
					"namespace": "foo",
					"name": "bar"
				}
			}
			`),
		},
		{
			name: "persistentVolumeClaim without claimName",
			pod: velerotest.UnstructuredOrDie(`
			{
				"apiVersion": "v1",
				"kind": "Pod",
				"metadata": {
					"namespace": "foo",
					"name": "bar"
				},
				"spec": {
					"volumes": [
						{
							"persistentVolumeClaim": {}
						}
					]
				}
			}
			`),
		},
		{
			name: "when a pod references a PVC that does not exist, the PVC is not returned as an additional item",
			pod: velerotest.UnstructuredOrDie(`
			{
				"apiVersion": "v1",
				"kind": "Pod",
				"metadata": {
					"namespace": "foo",
					"name": "bar"
				},
				"spec": {
					"volumes": [
						{
							"persistentVolumeClaim": {"claimName": "non-existent-pvc"}
						}
					]
				}
			}
			`),
			pvcs: []runtime.Object{
				builder.ForPersistentVolumeClaim("foo", "an-existing-pvc").Result(),
				builder.ForPersistentVolumeClaim("different-ns", "non-existent-pvc").Result(),
			},
			expected: nil,
		},
		{
			name: "full test, mix of volume types",
			pod: velerotest.UnstructuredOrDie(`
			{
				"apiVersion": "v1",
				"kind": "Pod",
				"metadata": {
					"namespace": "foo",
					"name": "bar"
				},
				"spec": {
					"volumes": [
						{
							"persistentVolumeClaim": {}
						},
						{
							"emptyDir": {}
						},
						{
							"persistentVolumeClaim": {"claimName": "claim1"}
						},
						{
							"emptyDir": {}
						},
						{
							"persistentVolumeClaim": {"claimName": "claim2"}
						}
					]
				}
			}
			`),
			pvcs: []runtime.Object{
				builder.ForPersistentVolumeClaim("foo", "claim1").Result(),
				builder.ForPersistentVolumeClaim("foo", "claim2").Result(),
			},
			expected: []velero.ResourceIdentifier{
				{GroupResource: kuberesource.PersistentVolumeClaims, Namespace: "foo", Name: "claim1"},
				{GroupResource: kuberesource.PersistentVolumeClaims, Namespace: "foo", Name: "claim2"},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			cs := kubefake.NewSimpleClientset(test.pvcs...)
			a := NewPodAction(velerotest.NewLogger(), cs.CoreV1())

			updated, additionalItems, err := a.Execute(test.pod, nil)
			require.NoError(t, err)
			assert.Equal(t, test.pod, updated)
			assert.Equal(t, test.expected, additionalItems)
		})
	}
}
