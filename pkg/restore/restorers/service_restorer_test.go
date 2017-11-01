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

	"github.com/stretchr/testify/assert"

	"k8s.io/apimachinery/pkg/runtime"
)

func TestServiceRestorerPrepare(t *testing.T) {
	tests := []struct {
		name        string
		obj         runtime.Unstructured
		expectedErr bool
		expectedRes runtime.Unstructured
	}{
		{
			name:        "no spec should error",
			obj:         NewTestUnstructured().WithName("svc-1").Unstructured,
			expectedErr: true,
		},
		{
			name:        "clusterIP (only) should be deleted from spec",
			obj:         NewTestUnstructured().WithName("svc-1").WithSpec("clusterIP", "foo").WithSpecField("ports", []interface{}{}).Unstructured,
			expectedErr: false,
			expectedRes: NewTestUnstructured().WithName("svc-1").WithSpec("foo").WithSpecField("ports", []interface{}{}).Unstructured,
		},
		{
			name:        "headless clusterIP should not be deleted from spec",
			obj:         NewTestUnstructured().WithName("svc-1").WithSpecField("clusterIP", "None").WithSpecField("ports", []interface{}{}).Unstructured,
			expectedErr: false,
			expectedRes: NewTestUnstructured().WithName("svc-1").WithSpecField("clusterIP", "None").WithSpecField("ports", []interface{}{}).Unstructured,
		},
		{
			name: "nodePort (only) should be deleted from all spec.ports",
			obj: NewTestUnstructured().WithName("svc-1").
				WithSpecField("ports", []interface{}{
					map[string]interface{}{"nodePort": ""},
					map[string]interface{}{"nodePort": "", "foo": "bar"},
				}).Unstructured,
			expectedErr: false,
			expectedRes: NewTestUnstructured().WithName("svc-1").
				WithSpecField("ports", []interface{}{
					map[string]interface{}{},
					map[string]interface{}{"foo": "bar"},
				}).Unstructured,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			restorer := NewServiceRestorer()

			res, _, err := restorer.Prepare(test.obj, nil, nil)

			if assert.Equal(t, test.expectedErr, err != nil) {
				assert.Equal(t, test.expectedRes, res)
			}
		})
	}
}
