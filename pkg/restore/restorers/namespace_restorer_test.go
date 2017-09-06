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

	api "github.com/heptio/ark/pkg/apis/ark/v1"
	testutil "github.com/heptio/ark/pkg/util/test"
)

func TestHandles(t *testing.T) {
	tests := []struct {
		name    string
		obj     runtime.Unstructured
		restore *api.Restore
		expect  bool
	}{
		{
			name:    "restorable NS",
			obj:     NewTestUnstructured().WithName("ns-1").Unstructured,
			restore: testutil.NewDefaultTestRestore().WithIncludedNamespace("ns-1").Restore,
			expect:  true,
		},
		{
			name:    "restorable NS via wildcard",
			obj:     NewTestUnstructured().WithName("ns-1").Unstructured,
			restore: testutil.NewDefaultTestRestore().WithIncludedNamespace("*").Restore,
			expect:  true,
		},
		{
			name:    "non-restorable NS",
			obj:     NewTestUnstructured().WithName("ns-1").Unstructured,
			restore: testutil.NewDefaultTestRestore().WithIncludedNamespace("ns-2").Restore,
			expect:  false,
		},
		{
			name:    "namespace is explicitly excluded",
			obj:     NewTestUnstructured().WithName("ns-1").Unstructured,
			restore: testutil.NewDefaultTestRestore().WithIncludedNamespace("*").WithExcludedNamespace("ns-1").Restore,
			expect:  false,
		},
		{
			name:    "namespace obj doesn't have name",
			obj:     NewTestUnstructured().WithMetadata().Unstructured,
			restore: testutil.NewDefaultTestRestore().WithIncludedNamespace("ns-1").Restore,
			expect:  false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			restorer := NewNamespaceRestorer()
			assert.Equal(t, test.expect, restorer.Handles(test.obj, test.restore))
		})
	}
}

func TestPrepare(t *testing.T) {
	tests := []struct {
		name        string
		obj         runtime.Unstructured
		restore     *api.Restore
		expectedErr bool
		expectedRes runtime.Unstructured
	}{
		{
			name:        "standard non-mapped namespace",
			obj:         NewTestUnstructured().WithStatus().WithName("ns-1").Unstructured,
			restore:     testutil.NewDefaultTestRestore().Restore,
			expectedErr: false,
			expectedRes: NewTestUnstructured().WithName("ns-1").Unstructured,
		},
		{
			name:        "standard mapped namespace",
			obj:         NewTestUnstructured().WithStatus().WithName("ns-1").Unstructured,
			restore:     testutil.NewDefaultTestRestore().WithMappedNamespace("ns-1", "ns-2").Restore,
			expectedErr: false,
			expectedRes: NewTestUnstructured().WithName("ns-2").Unstructured,
		},
		{
			name:        "object without name results in error",
			obj:         NewTestUnstructured().WithMetadata().WithStatus().Unstructured,
			restore:     testutil.NewDefaultTestRestore().Restore,
			expectedErr: true,
		},
		{
			name:        "annotations are kept",
			obj:         NewTestUnstructured().WithName("ns-1").WithAnnotations().Unstructured,
			restore:     testutil.NewDefaultTestRestore().Restore,
			expectedErr: false,
			expectedRes: NewTestUnstructured().WithName("ns-1").WithAnnotations().Unstructured,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			restorer := NewNamespaceRestorer()

			res, _, err := restorer.Prepare(test.obj, test.restore, nil)

			if assert.Equal(t, test.expectedErr, err != nil) {
				assert.Equal(t, test.expectedRes, res)
			}
		})
	}
}
