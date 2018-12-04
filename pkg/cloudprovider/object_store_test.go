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

package cloudprovider

import (
	"testing"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
)

func TestToNotFoundError(t *testing.T) {
	tests := []struct {
		name                string
		err                 error
		expectNotFoundError bool
	}{
		{
			name:                "nil error",
			err:                 nil,
			expectNotFoundError: false,
		},
		{
			name:                "some other error",
			err:                 errors.Errorf("foo"),
			expectNotFoundError: false,
		},
		{
			name:                "naked *NotFoundError",
			err:                 NewNotFoundError("bucket", "key"),
			expectNotFoundError: true,
		},
		{
			name:                "wrapped *NotFoundError",
			err:                 errors.WithMessage(NewNotFoundError("bucket", "key"), "some message"),
			expectNotFoundError: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			actual := ToNotFoundError(tc.err)
			if tc.expectNotFoundError {
				assert.IsType(t, &NotFoundError{}, actual)
			} else {
				assert.Nil(t, actual)
			}
		})
	}
}
