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

package restoreaction

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"

	velerotest "github.com/heptio/velero/pkg/util/test"
)

func TestJobActionExecute(t *testing.T) {
	tests := []struct {
		name        string
		obj         runtime.Unstructured
		expectedErr bool
		expectedRes runtime.Unstructured
	}{
		{
			name: "missing spec.selector and/or spec.template should not error",
			obj: NewTestUnstructured().WithName("job-1").
				WithSpec().
				Unstructured,
			expectedErr: false,
			expectedRes: NewTestUnstructured().WithName("job-1").
				WithSpec().
				Unstructured,
		},
		{
			name: "missing spec.selector.matchLabels should not error",
			obj: NewTestUnstructured().WithName("job-1").
				WithSpecField("selector", map[string]interface{}{}).
				Unstructured,
			expectedErr: false,
			expectedRes: NewTestUnstructured().WithName("job-1").
				WithSpecField("selector", map[string]interface{}{}).
				Unstructured,
		},
		{
			name: "spec.selector.matchLabels[controller-uid] is removed",
			obj: NewTestUnstructured().WithName("job-1").
				WithSpecField("selector", map[string]interface{}{
					"matchLabels": map[string]interface{}{
						"controller-uid": "foo",
						"hello":          "world",
					},
				}).
				Unstructured,
			expectedErr: false,
			expectedRes: NewTestUnstructured().WithName("job-1").
				WithSpecField("selector", map[string]interface{}{
					"matchLabels": map[string]interface{}{
						"hello": "world",
					},
				}).
				Unstructured,
		},
		{
			name: "missing spec.template.metadata should not error",
			obj: NewTestUnstructured().WithName("job-1").
				WithSpecField("template", map[string]interface{}{}).
				Unstructured,
			expectedErr: false,
			expectedRes: NewTestUnstructured().WithName("job-1").
				WithSpecField("template", map[string]interface{}{}).
				Unstructured,
		},
		{
			name: "missing spec.template.metadata.labels should not error",
			obj: NewTestUnstructured().WithName("job-1").
				WithSpecField("template", map[string]interface{}{
					"metadata": map[string]interface{}{},
				}).
				Unstructured,
			expectedErr: false,
			expectedRes: NewTestUnstructured().WithName("job-1").
				WithSpecField("template", map[string]interface{}{
					"metadata": map[string]interface{}{},
				}).
				Unstructured,
		},
		{
			name: "spec.template.metadata.labels[controller-uid] is removed",
			obj: NewTestUnstructured().WithName("job-1").
				WithSpecField("template", map[string]interface{}{
					"metadata": map[string]interface{}{
						"labels": map[string]interface{}{
							"controller-uid": "foo",
							"hello":          "world",
						},
					},
				}).
				Unstructured,
			expectedErr: false,
			expectedRes: NewTestUnstructured().WithName("job-1").
				WithSpecField("template", map[string]interface{}{
					"metadata": map[string]interface{}{
						"labels": map[string]interface{}{
							"hello": "world",
						},
					},
				}).
				Unstructured,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			action := NewJobAction(velerotest.NewLogger())

			res, _, err := action.Execute(test.obj, nil)

			if assert.Equal(t, test.expectedErr, err != nil) {
				assert.Equal(t, test.expectedRes, res)
			}
		})
	}
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
