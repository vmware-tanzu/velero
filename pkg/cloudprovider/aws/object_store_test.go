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

package aws

import (
	"testing"

	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
)

func TestIsValidSignatureVersion(t *testing.T) {
	assert.True(t, isValidSignatureVersion("1"))
	assert.True(t, isValidSignatureVersion("4"))
	assert.False(t, isValidSignatureVersion("3"))
}

func TestIsNoSuchKeyError(t *testing.T) {
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
			name:     "non aws error",
			err:      errors.Errorf("foo"),
			expected: false,
		},
		{
			name:     "aws error, wrong code",
			err:      awserr.New("some-code", "some-message", nil),
			expected: false,
		},
		{
			name:     "aws error, right code",
			err:      awserr.New(s3.ErrCodeNoSuchKey, "some-message", nil),
			expected: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			actual := isNoSuchKeyError(tc.err)
			assert.Equal(t, tc.expected, actual)
		})
	}
}
