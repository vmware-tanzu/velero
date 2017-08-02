/*
Copyright 2017 Heptio Inc.

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

package restorers

import (
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/stretchr/testify/assert"
)

func TestResetMetadataAndStatus(t *testing.T) {
	tests := []struct {
		name            string
		obj             runtime.Unstructured
		keepAnnotations bool
		expectedErr     bool
		expectedRes     runtime.Unstructured
	}{
		{
			name:            "no metadata causes error",
			obj:             NewTestUnstructured(),
			keepAnnotations: false,
			expectedErr:     true,
		},
		{
			name:            "don't keep annotations",
			obj:             NewTestUnstructured().WithMetadata("name", "namespace", "labels", "annotations").Unstructured,
			keepAnnotations: false,
			expectedErr:     false,
			expectedRes:     NewTestUnstructured().WithMetadata("name", "namespace", "labels").Unstructured,
		},
		{
			name:            "keep annotations",
			obj:             NewTestUnstructured().WithMetadata("name", "namespace", "labels", "annotations").Unstructured,
			keepAnnotations: true,
			expectedErr:     false,
			expectedRes:     NewTestUnstructured().WithMetadata("name", "namespace", "labels", "annotations").Unstructured,
		},
		{
			name:            "don't keep extraneous metadata",
			obj:             NewTestUnstructured().WithMetadata("foo").Unstructured,
			keepAnnotations: false,
			expectedErr:     false,
			expectedRes:     NewTestUnstructured().WithMetadata().Unstructured,
		},
		{
			name:            "don't keep status",
			obj:             NewTestUnstructured().WithMetadata().WithStatus().Unstructured,
			keepAnnotations: false,
			expectedErr:     false,
			expectedRes:     NewTestUnstructured().WithMetadata().Unstructured,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			res, err := resetMetadataAndStatus(test.obj, test.keepAnnotations)

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

func (obj *testUnstructured) WithMetadata(fields ...string) *testUnstructured {
	return obj.withMap("metadata", fields...)
}

func (obj *testUnstructured) WithSpec(fields ...string) *testUnstructured {
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
	annotations := make(map[string]interface{})
	for _, field := range fields {
		annotations[field] = "foo"
	}

	obj = obj.WithMetadataField("annotations", annotations)

	return obj
}

func (obj *testUnstructured) WithName(name string) *testUnstructured {
	return obj.WithMetadataField("name", name)
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
