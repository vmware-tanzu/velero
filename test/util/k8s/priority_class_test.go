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

package k8s

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/client-go/kubernetes/fake"
)

func TestCreatePriorityClassValidation(t *testing.T) {
	tests := []struct {
		name          string
		className     string
		value         int32
		expectedError string
	}{
		{
			name:          "empty name should fail",
			className:     "",
			value:         100,
			expectedError: "priority class name cannot be empty",
		},
		{
			name:          "value too low should fail",
			className:     "test-priority",
			value:         -1000000001,
			expectedError: "priority class value must be between -1000000000 and 1000000000",
		},
		{
			name:          "value too high should fail",
			className:     "test-priority",
			value:         1000000001,
			expectedError: "priority class value must be between -1000000000 and 1000000000",
		},
		{
			name:          "valid minimum value should succeed",
			className:     "test-priority",
			value:         -1000000000,
			expectedError: "",
		},
		{
			name:          "valid maximum value should succeed",
			className:     "test-priority",
			value:         1000000000,
			expectedError: "",
		},
		{
			name:          "valid normal value should succeed",
			className:     "test-priority",
			value:         100,
			expectedError: "",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fakeClient := fake.NewSimpleClientset()
			testClient := TestClient{ClientGo: fakeClient}

			err := CreatePriorityClass(t.Context(), testClient, test.className, test.value)

			if test.expectedError != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), test.expectedError)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
