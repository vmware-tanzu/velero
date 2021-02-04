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

package restic

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

// setAzureEnvironment sets the Azure credentials environment variable to the
// given value and returns a function to restore it to its previous value
func setAzureEnvironment(t *testing.T, value string) func() {
	envVar := "AZURE_CREDENTIALS_FILE"
	var cleanup func()

	if original, exists := os.LookupEnv(envVar); exists {
		cleanup = func() {
			require.NoError(t, os.Setenv(envVar, original), "failed to reset %s environment variable", envVar)
		}
	} else {
		cleanup = func() {
			require.NoError(t, os.Unsetenv(envVar), "failed to reset %s environment variable", envVar)
		}
	}

	require.NoError(t, os.Setenv(envVar, value), "failed to set %s environment variable", envVar)

	return cleanup
}

func TestSelectCredentialsFile(t *testing.T) {
	testCases := []struct {
		name        string
		config      map[string]string
		environment string
		expected    string
	}{
		{
			name:     "when config is empty and environment variable is not set, no file is selected",
			expected: "",
		},
		{
			name: "when config contains credentials file and environment variable is not set, file from config is selected",
			config: map[string]string{
				"credentialsFile": "/tmp/credentials/path/to/secret",
			},
			expected: "/tmp/credentials/path/to/secret",
		},
		{
			name:        "when config is empty and environment variable is set, file from environment is selected",
			environment: "/credentials/file/from/env",
			expected:    "/credentials/file/from/env",
		},
		{
			name: "when config contains credentials file and environment variable is set, file from config is selected",
			config: map[string]string{
				"credentialsFile": "/tmp/credentials/path/to/secret",
			},
			environment: "/credentials/file/from/env",
			expected:    "/tmp/credentials/path/to/secret",
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			cleanup := setAzureEnvironment(t, tc.environment)
			defer cleanup()

			selectedFile := selectCredentialsFile(tc.config)
			require.Equal(t, tc.expected, selectedFile)
		})
	}
}
