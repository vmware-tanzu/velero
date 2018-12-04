/*
Copyright 2018 the Heptio Ark contributors.

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

package azure

import (
	"net/http"
	"testing"

	"github.com/Azure/azure-sdk-for-go/storage"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
)

func TestIsNotFoundError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{
			name:     "nil err",
			err:      nil,
			expected: false,
		},
		{
			name:     "non azure error",
			err:      errors.Errorf("foo"),
			expected: false,
		},
		{
			name:     "azure error, wrong code",
			err:      storage.AzureStorageServiceError{StatusCode: http.StatusInternalServerError},
			expected: false,
		},
		{
			name:     "azure error, right code",
			err:      storage.AzureStorageServiceError{StatusCode: http.StatusNotFound},
			expected: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			actual := isNotFoundError(tc.err)
			assert.Equal(t, tc.expected, actual)
		})
	}
}
