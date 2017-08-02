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

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestPVCRestorerReady(t *testing.T) {
	tests := []struct {
		name     string
		obj      *unstructured.Unstructured
		expected bool
	}{
		{
			name:     "no status returns not ready",
			obj:      NewTestUnstructured().Unstructured,
			expected: false,
		},
		{
			name:     "no status.phase returns not ready",
			obj:      NewTestUnstructured().WithStatus().Unstructured,
			expected: false,
		},
		{
			name:     "empty status.phase returns not ready",
			obj:      NewTestUnstructured().WithStatusField("phase", "").Unstructured,
			expected: false,
		},
		{
			name:     "non-Available status.phase returns not ready",
			obj:      NewTestUnstructured().WithStatusField("phase", "foo").Unstructured,
			expected: false,
		},
		{
			name:     "Bound status.phase returns ready",
			obj:      NewTestUnstructured().WithStatusField("phase", "Bound").Unstructured,
			expected: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			restorer := NewPersistentVolumeClaimRestorer()

			assert.Equal(t, test.expected, restorer.Ready(test.obj))
		})
	}
}
