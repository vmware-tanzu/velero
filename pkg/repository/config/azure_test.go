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
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
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

func TestGetStorageDomainFromCloudName(t *testing.T) {
	testCases := []struct {
		name        string
		cloudName   string
		expected    string
		expectedErr string
	}{
		{
			name:        "get azure env fail",
			cloudName:   "fake-cloud",
			expectedErr: "unable to parse azure env from cloud name fake-cloud: autorest/azure: There is no cloud environment matching the name \"FAKE-CLOUD\"",
		},
		{
			name:      "cloud name is empty",
			cloudName: "",
			expected:  "blob.core.windows.net",
		},
		{
			name:      "azure public cloud",
			cloudName: "AzurePublicCloud",
			expected:  "blob.core.windows.net",
		},
		{

			name:      "azure China cloud",
			cloudName: "AzureChinaCloud",
			expected:  "blob.core.chinacloudapi.cn",
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			domain, err := getStorageDomainFromCloudName(tc.cloudName)

			require.Equal(t, tc.expected, domain)

			if tc.expectedErr == "" {
				assert.NoError(t, err)
			} else {
				assert.EqualError(t, err, tc.expectedErr)
				assert.Empty(t, domain)
			}
		})
	}
}
