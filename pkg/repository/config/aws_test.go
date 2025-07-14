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
		name             string
		config           map[string]string
		expected         map[string]string
		getS3Credentials func(config map[string]string) (*aws.Credentials, error)
	}{
		{
			name:     "when config is empty, no env vars are returned",
			config:   map[string]string{},
			expected: map[string]string{},
			getS3Credentials: func(map[string]string) (*aws.Credentials, error) {
				return nil, nil
			},
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
			getS3Credentials: func(map[string]string) (*aws.Credentials, error) {
				return nil, nil
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Mock GetS3Credentials
			if tc.getS3Credentials != nil {
				getS3CredentialsFunc = tc.getS3Credentials
			} else {
				getS3CredentialsFunc = GetS3Credentials
			}

			actual, err := GetS3ResticEnvVars(tc.config)

			require.NoError(t, err)

			// Avoid direct comparison of expected and actual to prevent exposing secrets.
			// This may occur if the test doesn't set getS3Credentials func correctly.
			if !reflect.DeepEqual(tc.expected, actual) {
				t.Errorf("Expected and actual results do not match for test case %q", tc.name)
				for key, value := range actual {
					if expVal, err := tc.expected[key]; !err || expVal != value {
						if actualVal, ok := actual[key]; !ok {
							t.Errorf("Key %q is missing in actual result", key)
						} else if expVal != actualVal {
							t.Errorf("Key %q: expected value %q", key, expVal)
						}
					}
				}
			}
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
			// Ensure env variables do not set AWS config entries
			t.Setenv("AWS_ACCESS_KEY_ID", "")
			t.Setenv("AWS_SECRET_ACCESS_KEY", "")
			t.Setenv("AWS_SHARED_CREDENTIALS_FILE", "")

			tmpFile, err := os.CreateTemp(t.TempDir(), "velero-test-aws-credentials")
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
				t.Errorf("GetS3Credentials() want %v", tt.want.AccessKeyID)
			}
			if !reflect.DeepEqual(got.SecretAccessKey, tt.want.SecretAccessKey) {
				t.Errorf("GetS3Credentials() want %v", tt.want.SecretAccessKey)
			}
		})
	}
}
