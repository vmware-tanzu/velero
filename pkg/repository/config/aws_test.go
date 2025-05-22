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
			getS3Credentials: func(config map[string]string) (*aws.Credentials, error) {
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
			getS3Credentials: func(config map[string]string) (*aws.Credentials, error) {
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
				t.Errorf("GetS3Credentials() want %v", tt.want.AccessKeyID)
			}
			if !reflect.DeepEqual(got.SecretAccessKey, tt.want.SecretAccessKey) {
				t.Errorf("GetS3Credentials() want %v", tt.want.SecretAccessKey)
			}
		})
	}
}

func TestGetS3CredentialsWithMultipleProfiles(t *testing.T) {
	// Create a credentials file with multiple profiles
	credentialsContent := `[default]
aws_access_key_id = default-access-key
aws_secret_access_key = default-secret-key

[profileone]
aws_access_key_id = profileone-access-key
aws_secret_access_key = profileone-secret-key

[profiletwo]
aws_access_key_id = profiletwo-access-key
aws_secret_access_key = profiletwo-secret-key
`

	// Create a temporary file for the credentials
	tmpFile, err := os.CreateTemp("", "velero-test-aws-multiple-profiles")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	// Write the credentials content to the file
	if _, err := tmpFile.WriteString(credentialsContent); err != nil {
		t.Fatalf("Failed to write to temp file: %v", err)
	}
	if err := tmpFile.Close(); err != nil {
		t.Fatalf("Failed to close temp file: %v", err)
	}

	// Clear environment variables that might interfere with the test
	t.Setenv("AWS_ACCESS_KEY_ID", "")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "")
	t.Setenv("AWS_SHARED_CREDENTIALS_FILE", "")
	t.Setenv("AWS_WEB_IDENTITY_TOKEN_FILE", "")
	t.Setenv("AWS_ROLE_ARN", "")

	testCases := []struct {
		name          string
		config        map[string]string
		expected      *aws.Credentials
		wantErr       bool
		wantErrString string
	}{
		{
			name: "default profile",
			config: map[string]string{
				"credentialsFile": tmpFile.Name(),
				// No profile specified, should use default
			},
			expected: &aws.Credentials{
				AccessKeyID:     "default-access-key",
				SecretAccessKey: "default-secret-key",
			},
			wantErr:       false,
			wantErrString: "",
		},
		{
			name: "profileone",
			config: map[string]string{
				"credentialsFile": tmpFile.Name(),
				"profile":         "profileone",
			},
			expected: &aws.Credentials{
				AccessKeyID:     "profileone-access-key",
				SecretAccessKey: "profileone-secret-key",
			},
			wantErr:       false,
			wantErrString: "",
		},
		{
			name: "profiletwo",
			config: map[string]string{
				"credentialsFile": tmpFile.Name(),
				"profile":         "profiletwo",
			},
			expected: &aws.Credentials{
				AccessKeyID:     "profiletwo-access-key",
				SecretAccessKey: "profiletwo-secret-key",
			},
			wantErr:       false,
			wantErrString: "",
		},
		{
			name: "non-existent profile",
			config: map[string]string{
				"credentialsFile": tmpFile.Name(),
				"profile":         "nonexistentprofile",
			},
			expected:      nil,
			wantErr:       true,
			wantErrString: "failed to get shared config profile",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Call GetS3Credentials with the config
			got, err := GetS3Credentials(tc.config)

			// Check for errors
			if tc.wantErr {
				require.Error(t, err, "GetS3Credentials should return an error for non-existent profile")
				if tc.wantErrString != "" {
					require.Contains(t, err.Error(), tc.wantErrString, "Error should contain the expected error string")
				}
			} else {
				require.NoError(t, err, "GetS3Credentials should not return an error")
				require.NotNil(t, got, "GetS3Credentials should return credentials")

				// Verify the credentials match what we expect
				if got.AccessKeyID != tc.expected.AccessKeyID {
					t.Errorf("AccessKeyID = %v, want %v", got.AccessKeyID, tc.expected.AccessKeyID)
				}
				if got.SecretAccessKey != tc.expected.SecretAccessKey {
					t.Errorf("SecretAccessKey = %v, want %v", got.SecretAccessKey, tc.expected.SecretAccessKey)
				}
			}
		})
	}
}
