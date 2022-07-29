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

package config

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGetS3ResticEnvVars(t *testing.T) {
	testCases := []struct {
		name     string
		config   map[string]string
		expected map[string]string
	}{
		{
			name:     "when config is empty, no env vars are returned",
			config:   map[string]string{},
			expected: map[string]string{},
		},
		{
			name: "when config contains profile key, profile env var is set with profile value",
			config: map[string]string{
				"profile": "profile-value",
			},
			expected: map[string]string{
				"AWS_PROFILE": "profile-value",
			},
		},
		{
			name: "when config contains credentials file key, credentials file env var is set with credentials file value",
			config: map[string]string{
				"credentialsFile": "/tmp/credentials/path/to/secret",
			},
			expected: map[string]string{
				"AWS_SHARED_CREDENTIALS_FILE": "/tmp/credentials/path/to/secret",
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actual, err := GetS3ResticEnvVars(tc.config)

			require.NoError(t, err)

			require.Equal(t, tc.expected, actual)
		})
	}
}
