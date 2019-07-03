/*
Copyright 2017, 2019 the Velero contributors.

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
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	discoveryfake "k8s.io/client-go/discovery/fake"
	kubefake "k8s.io/client-go/kubernetes/fake"

	"github.com/heptio/velero/pkg/discovery"
	"github.com/heptio/velero/pkg/test"
	"github.com/heptio/velero/pkg/util/collections"
	velerotest "github.com/heptio/velero/pkg/util/test"
)

func TestPrioritizeResources(t *testing.T) {
	tests := []struct {
		name         string
		apiResources map[string][]string
		priorities   []string
		includes     []string
		excludes     []string
		expected     []string
	}{
		{
			name: "priorities & ordering are correctly applied",
			apiResources: map[string][]string{
				"v1": {"aaa", "bbb", "configmaps", "ddd", "namespaces", "ooo", "pods", "sss"},
			},
			priorities: []string{"namespaces", "configmaps", "pods"},
			includes:   []string{"*"},
			expected:   []string{"namespaces", "configmaps", "pods", "aaa", "bbb", "ddd", "ooo", "sss"},
		},
		{
			name: "includes are correctly applied",
			apiResources: map[string][]string{
				"v1": {"aaa", "bbb", "configmaps", "ddd", "namespaces", "ooo", "pods", "sss"},
			},
			priorities: []string{"namespaces", "configmaps", "pods"},
			includes:   []string{"namespaces", "aaa", "sss"},
			expected:   []string{"namespaces", "aaa", "sss"},
		},
		{
			name: "excludes are correctly applied",
			apiResources: map[string][]string{
				"v1": {"aaa", "bbb", "configmaps", "ddd", "namespaces", "ooo", "pods", "sss"},
			},
			priorities: []string{"namespaces", "configmaps", "pods"},
			includes:   []string{"*"},
			excludes:   []string{"ooo", "pods"},
			expected:   []string{"namespaces", "configmaps", "aaa", "bbb", "ddd", "sss"},
		},
	}

	logger := velerotest.NewLogger()

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			discoveryClient := &test.DiscoveryClient{
				FakeDiscovery: kubefake.NewSimpleClientset().Discovery().(*discoveryfake.FakeDiscovery),
			}

			helper, err := discovery.NewHelper(discoveryClient, logger)
			require.NoError(t, err)

			// add all the test case's API resources to the discovery client
			for gvString, resources := range tc.apiResources {
				gv, err := schema.ParseGroupVersion(gvString)
				require.NoError(t, err)

				for _, resource := range resources {
					discoveryClient.WithAPIResource(&test.APIResource{
						Group:   gv.Group,
						Version: gv.Version,
						Name:    resource,
					})
				}
			}

			require.NoError(t, helper.Refresh())

			includesExcludes := collections.NewIncludesExcludes().Includes(tc.includes...).Excludes(tc.excludes...)

			result, err := prioritizeResources(helper, tc.priorities, includesExcludes, logger)
			require.NoError(t, err)

			require.Equal(t, len(tc.expected), len(result))

			for i := range result {
				if e, a := tc.expected[i], result[i].Resource; e != a {
					t.Errorf("index %d, expected %s, got %s", i, e, a)
				}
			}
		})
	}
}

func TestResetMetadataAndStatus(t *testing.T) {
	tests := []struct {
		name        string
		obj         *unstructured.Unstructured
		expectedErr bool
		expectedRes *unstructured.Unstructured
	}{
		{
			name:        "no metadata causes error",
			obj:         NewTestUnstructured().Unstructured,
			expectedErr: true,
		},
		{
			name:        "keep name, namespace, labels, annotations only",
			obj:         NewTestUnstructured().WithMetadata("name", "blah", "namespace", "labels", "annotations", "foo").Unstructured,
			expectedErr: false,
			expectedRes: NewTestUnstructured().WithMetadata("name", "namespace", "labels", "annotations").Unstructured,
		},
		{
			name:        "don't keep status",
			obj:         NewTestUnstructured().WithMetadata().WithStatus().Unstructured,
			expectedErr: false,
			expectedRes: NewTestUnstructured().WithMetadata().Unstructured,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			res, err := resetMetadataAndStatus(test.obj)

			if assert.Equal(t, test.expectedErr, err != nil) {
				assert.Equal(t, test.expectedRes, res)
			}
		})
	}
}

func TestIsCompleted(t *testing.T) {
	tests := []struct {
		name          string
		expected      bool
		content       string
		groupResource schema.GroupResource
		expectedErr   bool
	}{
		{
			name:          "Failed pods are complete",
			expected:      true,
			content:       `{"apiVersion":"v1","kind":"Pod","metadata":{"namespace":"ns","name":"pod1"}, "status": {"phase": "Failed"}}`,
			groupResource: schema.GroupResource{Group: "", Resource: "pods"},
		},
		{
			name:          "Succeeded pods are complete",
			expected:      true,
			content:       `{"apiVersion":"v1","kind":"Pod","metadata":{"namespace":"ns","name":"pod1"}, "status": {"phase": "Succeeded"}}`,
			groupResource: schema.GroupResource{Group: "", Resource: "pods"},
		},
		{
			name:          "Pending pods aren't complete",
			expected:      false,
			content:       `{"apiVersion":"v1","kind":"Pod","metadata":{"namespace":"ns","name":"pod1"}, "status": {"phase": "Pending"}}`,
			groupResource: schema.GroupResource{Group: "", Resource: "pods"},
		},
		{
			name:          "Running pods aren't complete",
			expected:      false,
			content:       `{"apiVersion":"v1","kind":"Pod","metadata":{"namespace":"ns","name":"pod1"}, "status": {"phase": "Running"}}`,
			groupResource: schema.GroupResource{Group: "", Resource: "pods"},
		},
		{
			name:          "Jobs without a completion time aren't complete",
			expected:      false,
			content:       `{"apiVersion":"v1","kind":"Pod","metadata":{"namespace":"ns","name":"pod1"}}`,
			groupResource: schema.GroupResource{Group: "batch", Resource: "jobs"},
		},
		{
			name:          "Jobs with a completion time are completed",
			expected:      true,
			content:       `{"apiVersion":"v1","kind":"Pod","metadata":{"namespace":"ns","name":"pod1"}, "status": {"completionTime": "bar"}}`,
			groupResource: schema.GroupResource{Group: "batch", Resource: "jobs"},
		},
		{
			name:          "Jobs with an empty completion time are not completed",
			expected:      false,
			content:       `{"apiVersion":"v1","kind":"Pod","metadata":{"namespace":"ns","name":"pod1"}, "status": {"completionTime": ""}}`,
			groupResource: schema.GroupResource{Group: "batch", Resource: "jobs"},
		},
		{
			name:          "Something not a pod or a job may actually be complete, but we're not concerned with that",
			expected:      false,
			content:       `{"apiVersion": "v1", "kind": "Namespace", "metadata": {"name": "ns"}, "status": {"completionTime": "bar", "phase":"Completed"}}`,
			groupResource: schema.GroupResource{Group: "", Resource: "namespaces"},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			u := velerotest.UnstructuredOrDie(test.content)
			backup, err := isCompleted(u, test.groupResource)

			if assert.Equal(t, test.expectedErr, err != nil) {
				assert.Equal(t, test.expected, backup)
			}
		})
	}
}

func TestGetItemFilePath(t *testing.T) {
	res := getItemFilePath("root", "resource", "", "item")
	assert.Equal(t, "root/resources/resource/cluster/item.json", res)

	res = getItemFilePath("root", "resource", "namespace", "item")
	assert.Equal(t, "root/resources/resource/namespaces/namespace/item.json", res)
}

type testUnstructured struct {
	*unstructured.Unstructured
}

func NewTestUnstructured() *testUnstructured {
	obj := &testUnstructured{
		Unstructured: &unstructured.Unstructured{
			Object: make(map[string]interface{}),
		},
	}

	return obj
}

func (obj *testUnstructured) WithAPIVersion(v string) *testUnstructured {
	obj.Object["apiVersion"] = v
	return obj
}

func (obj *testUnstructured) WithKind(k string) *testUnstructured {
	obj.Object["kind"] = k
	return obj
}

func (obj *testUnstructured) WithMetadata(fields ...string) *testUnstructured {
	return obj.withMap("metadata", fields...)
}

func (obj *testUnstructured) WithSpec(fields ...string) *testUnstructured {
	if _, found := obj.Object["spec"]; found {
		panic("spec already set - you probably didn't mean to do this twice!")
	}
	return obj.withMap("spec", fields...)
}

func (obj *testUnstructured) WithStatus(fields ...string) *testUnstructured {
	return obj.withMap("status", fields...)
}

func (obj *testUnstructured) WithMetadataField(field string, value interface{}) *testUnstructured {
	return obj.withMapEntry("metadata", field, value)
}

func (obj *testUnstructured) WithSpecField(field string, value interface{}) *testUnstructured {
	return obj.withMapEntry("spec", field, value)
}

func (obj *testUnstructured) WithStatusField(field string, value interface{}) *testUnstructured {
	return obj.withMapEntry("status", field, value)
}

func (obj *testUnstructured) WithAnnotations(fields ...string) *testUnstructured {
	vals := map[string]string{}
	for _, field := range fields {
		vals[field] = "foo"
	}

	return obj.WithAnnotationValues(vals)
}

func (obj *testUnstructured) WithAnnotationValues(fieldVals map[string]string) *testUnstructured {
	annotations := make(map[string]interface{})
	for field, val := range fieldVals {
		annotations[field] = val
	}

	obj = obj.WithMetadataField("annotations", annotations)

	return obj
}

func (obj *testUnstructured) WithNamespace(ns string) *testUnstructured {
	return obj.WithMetadataField("namespace", ns)
}

func (obj *testUnstructured) WithName(name string) *testUnstructured {
	return obj.WithMetadataField("name", name)
}

func (obj *testUnstructured) ToJSON() []byte {
	bytes, err := json.Marshal(obj.Object)
	if err != nil {
		panic(err)
	}
	return bytes
}

func (obj *testUnstructured) withMap(name string, fields ...string) *testUnstructured {
	m := make(map[string]interface{})
	obj.Object[name] = m

	for _, field := range fields {
		m[field] = "foo"
	}

	return obj
}

func (obj *testUnstructured) withMapEntry(mapName, field string, value interface{}) *testUnstructured {
	var m map[string]interface{}

	if res, ok := obj.Unstructured.Object[mapName]; !ok {
		m = make(map[string]interface{})
		obj.Unstructured.Object[mapName] = m
	} else {
		m = res.(map[string]interface{})
	}

	m[field] = value

	return obj
}
