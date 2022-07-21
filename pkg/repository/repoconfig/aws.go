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

package repoconfig

import (
	"errors"
	"os"

	"github.com/aws/aws-sdk-go/aws/credentials"
)

const (
	// AWS specific environment variable
	awsProfileEnvVar         = "AWS_PROFILE"
	awsProfileKey            = "profile"
	awsCredentialsFileEnvVar = "AWS_SHARED_CREDENTIALS_FILE"
)

// GetS3ResticEnvVars gets the environment variables that restic
// relies on (AWS_PROFILE) based on info in the provided object
// storage location config map.
func GetS3ResticEnvVars(config map[string]string) (map[string]string, error) {
	result := make(map[string]string)

	if credentialsFile, ok := config[CredentialsFileKey]; ok {
		result[awsCredentialsFileEnvVar] = credentialsFile
	}

	if profile, ok := config[awsProfileKey]; ok {
		result[awsProfileEnvVar] = profile
	}

	return result, nil
}

// GetS3Credentials gets the S3 credential values according to the information
// of the provided config or the system's environment variables
func GetS3Credentials(config map[string]string) (credentials.Value, error) {
	credentialsFile := config[CredentialsFileKey]
	if credentialsFile == "" {
		credentialsFile = os.Getenv("AWS_SHARED_CREDENTIALS_FILE")
	}

	if credentialsFile == "" {
		return credentials.Value{}, errors.New("missing credential file")
	}

	creds := credentials.NewSharedCredentials(credentialsFile, "")
	credValue, err := creds.Get()
	if err != nil {
		return credValue, err
	}

	return credValue, nil
}
