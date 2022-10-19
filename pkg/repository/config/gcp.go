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

//nolint:gosec
package config

import "os"

const (
	// GCP specific environment variable
	gcpCredentialsFileEnvVar = "GOOGLE_APPLICATION_CREDENTIALS"
)

// GetGCPResticEnvVars gets the environment variables that restic relies
// on based on info in the provided object storage location config map.
func GetGCPResticEnvVars(config map[string]string) (map[string]string, error) {
	result := make(map[string]string)

	if credentialsFile, ok := config[CredentialsFileKey]; ok {
		result[gcpCredentialsFileEnvVar] = credentialsFile
	}

	return result, nil
}

// GetGCPCredentials gets the credential file required by a GCP bucket connection,
// if the provided config doean't have the value, get it from system's environment variables
func GetGCPCredentials(config map[string]string) string {
	if credentialsFile, ok := config[CredentialsFileKey]; ok {
		return credentialsFile
	} else {
		return os.Getenv("GOOGLE_APPLICATION_CREDENTIALS")
	}
}
