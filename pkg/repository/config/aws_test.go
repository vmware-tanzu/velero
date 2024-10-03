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
	"reflect"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
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

func TestGetS3CredentialsCorrectlyUseProfile(t *testing.T) {
	type args struct {
		config             map[string]string
		secretFileContents string
	}
	tests := []struct {
		name    string
		args    args
		want    *aws.Credentials
		wantErr bool
	}{
		{
			name: "Test GetS3Credentials use profile correctly",
			args: args{
				config: map[string]string{
					"profile": "some-profile",
				},
				secretFileContents: `[default]
	aws_access_key_id = default-access-key-id
	aws_secret_access_key = default-secret-access-key
	[profile some-profile]
	aws_access_key_id = some-profile-access-key-id
	aws_secret_access_key = some-profile-secret-access-key
	`,
			},
			want: &aws.Credentials{
				AccessKeyID:     "some-profile-access-key-id",
				SecretAccessKey: "some-profile-secret-access-key",
			},
		},
		{
			name: "Test GetS3Credentials default to default profile",
			args: args{
				config: map[string]string{},
				secretFileContents: `[default]
	aws_access_key_id = default-access-key-id
	aws_secret_access_key = default-secret-access-key
	[profile some-profile]
	aws_access_key_id = some-profile-access-key-id
	aws_secret_access_key = some-profile-secret-access-key
	`,
			},
			want: &aws.Credentials{
				AccessKeyID:     "default-access-key-id",
				SecretAccessKey: "default-secret-access-key",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpFile, err := os.CreateTemp("", "velero-test-aws-credentials")
			defer os.Remove(tmpFile.Name())
			if err != nil {
				t.Errorf("GetS3Credentials() error = %v", err)
				return
			}
			// write the contents of the secret file to the temp file
			_, err = tmpFile.WriteString(tt.args.secretFileContents)
			if err != nil {
				t.Errorf("GetS3Credentials() error = %v", err)
				return
			}
			tt.args.config["credentialsFile"] = tmpFile.Name()
			got, err := GetS3Credentials(tt.args.config)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetS3Credentials() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got.AccessKeyID, tt.want.AccessKeyID) {
				t.Errorf("GetS3Credentials() got = %v, want %v", got.AccessKeyID, tt.want.AccessKeyID)
			}
			if !reflect.DeepEqual(got.SecretAccessKey, tt.want.SecretAccessKey) {
				t.Errorf("GetS3Credentials() got = %v, want %v", got.SecretAccessKey, tt.want.SecretAccessKey)
			}
		})
	}
}
