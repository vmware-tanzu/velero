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

package provider

import (
	"errors"
	"testing"

	awscredentials "github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1api "k8s.io/api/core/v1"

	filecredentials "github.com/vmware-tanzu/velero/internal/credentials"
	velerov1api "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	velerotest "github.com/vmware-tanzu/velero/pkg/test"
)

func TestGetStorageCredentials(t *testing.T) {
	testCases := []struct {
		name                string
		backupLocation      velerov1api.BackupStorageLocation
		credFileStore       filecredentials.FileStore
		getAzureCredentials func(map[string]string) (string, string, error)
		getS3Credentials    func(map[string]string) (awscredentials.Value, error)
		getGCPCredentials   func(map[string]string) string
		expected            map[string]string
		expectedErr         string
	}{
		{
			name: "invalid provider",
			backupLocation: velerov1api.BackupStorageLocation{
				Spec: velerov1api.BackupStorageLocationSpec{
					Provider: "invalid-provider",
				},
			},
			expected:    map[string]string{},
			expectedErr: "invalid storage provider",
		},
		{
			name: "credential section exists in BSL, file store fail",
			backupLocation: velerov1api.BackupStorageLocation{
				Spec: velerov1api.BackupStorageLocationSpec{
					Provider:   "aws",
					Credential: &corev1api.SecretKeySelector{},
				},
			},
			credFileStore: velerotest.NewFakeCredentialsFileStore("", errors.New("fake error")),
			expected:      map[string]string{},
			expectedErr:   "error get credential file in bsl: fake error",
		},
		{
			name: "aws, Credential section not exists in BSL",
			backupLocation: velerov1api.BackupStorageLocation{
				Spec: velerov1api.BackupStorageLocationSpec{
					Provider: "velero.io/aws",
					Config: map[string]string{
						"credentialsFile": "credentials-from-config-map",
					},
				},
			},
			getS3Credentials: func(config map[string]string) (awscredentials.Value, error) {
				return awscredentials.Value{
					AccessKeyID: "from: " + config["credentialsFile"],
				}, nil
			},

			expected: map[string]string{
				"accessKeyID":     "from: credentials-from-config-map",
				"providerName":    "",
				"secretAccessKey": "",
				"sessionToken":    "",
			},
		},
		{
			name: "aws, Credential section exists in BSL",
			backupLocation: velerov1api.BackupStorageLocation{
				Spec: velerov1api.BackupStorageLocationSpec{
					Provider: "velero.io/aws",
					Config: map[string]string{
						"credentialsFile": "credentials-from-config-map",
					},
					Credential: &corev1api.SecretKeySelector{},
				},
			},
			credFileStore: velerotest.NewFakeCredentialsFileStore("credentials-from-credential-key", nil),
			getS3Credentials: func(config map[string]string) (awscredentials.Value, error) {
				return awscredentials.Value{
					AccessKeyID: "from: " + config["credentialsFile"],
				}, nil
			},

			expected: map[string]string{
				"accessKeyID":     "from: credentials-from-credential-key",
				"providerName":    "",
				"secretAccessKey": "",
				"sessionToken":    "",
			},
		},
		{
			name: "aws, get credentials fail",
			backupLocation: velerov1api.BackupStorageLocation{
				Spec: velerov1api.BackupStorageLocationSpec{
					Provider: "velero.io/aws",
					Config: map[string]string{
						"credentialsFile": "credentials-from-config-map",
					},
				},
			},
			credFileStore: velerotest.NewFakeCredentialsFileStore("", nil),
			getS3Credentials: func(config map[string]string) (awscredentials.Value, error) {
				return awscredentials.Value{}, errors.New("fake error")
			},
			expected:    map[string]string{},
			expectedErr: "error get s3 credentials: fake error",
		},
		{
			name: "azure, Credential section exists in BSL",
			backupLocation: velerov1api.BackupStorageLocation{
				Spec: velerov1api.BackupStorageLocationSpec{
					Provider: "velero.io/azure",
					Config: map[string]string{
						"credentialsFile": "credentials-from-config-map",
					},
					Credential: &corev1api.SecretKeySelector{},
				},
			},
			credFileStore: velerotest.NewFakeCredentialsFileStore("credentials-from-credential-key", nil),
			getAzureCredentials: func(config map[string]string) (string, string, error) {
				return "storage account from: " + config["credentialsFile"], "", nil
			},

			expected: map[string]string{
				"storageAccount": "storage account from: credentials-from-credential-key",
				"storageKey":     "",
			},
		},
		{
			name: "azure, get azure credentials fails",
			backupLocation: velerov1api.BackupStorageLocation{
				Spec: velerov1api.BackupStorageLocationSpec{
					Provider: "velero.io/azure",
					Config: map[string]string{
						"credentialsFile": "credentials-from-config-map",
					},
				},
			},
			getAzureCredentials: func(config map[string]string) (string, string, error) {
				return "", "", errors.New("fake error")
			},

			expected:    map[string]string{},
			expectedErr: "error get azure credentials: fake error",
		},
		{
			name: "gcp, Credential section not exists in BSL",
			backupLocation: velerov1api.BackupStorageLocation{
				Spec: velerov1api.BackupStorageLocationSpec{
					Provider: "velero.io/gcp",
					Config: map[string]string{
						"credentialsFile": "credentials-from-config-map",
					},
				},
			},
			getGCPCredentials: func(config map[string]string) string {
				return "credentials-from-config-map"
			},

			expected: map[string]string{
				"credFile": "credentials-from-config-map",
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			getAzureCredentials = tc.getAzureCredentials
			getS3Credentials = tc.getS3Credentials
			getGCPCredentials = tc.getGCPCredentials

			actual, err := getStorageCredentials(&tc.backupLocation, tc.credFileStore)

			require.Equal(t, tc.expected, actual)

			if tc.expectedErr == "" {
				assert.NoError(t, err)
			} else {
				assert.EqualError(t, err, tc.expectedErr)
			}
		})
	}
}

func TestGetStorageVariables(t *testing.T) {
	testCases := []struct {
		name                  string
		backupLocation        velerov1api.BackupStorageLocation
		repoName              string
		getS3BucketRegion     func(string) (string, error)
		getAzureStorageDomain func(map[string]string) string
		expected              map[string]string
		expectedErr           string
	}{
		{
			name: "invalid provider",
			backupLocation: velerov1api.BackupStorageLocation{
				Spec: velerov1api.BackupStorageLocationSpec{
					Provider: "invalid-provider",
				},
			},
			expected:    map[string]string{},
			expectedErr: "invalid storage provider",
		},
		{
			name: "aws, ObjectStorage section not exists in BSL, s3Url exist",
			backupLocation: velerov1api.BackupStorageLocation{
				Spec: velerov1api.BackupStorageLocationSpec{
					Provider: "velero.io/aws",
					Config: map[string]string{
						"bucket":                "fake-bucket",
						"prefix":                "fake-prefix",
						"region":                "fake-region/",
						"s3Url":                 "fake-url",
						"insecureSkipTLSVerify": "true",
					},
				},
			},
			expected: map[string]string{
				"bucket":        "fake-bucket",
				"prefix":        "fake-prefix/unified-repo/",
				"region":        "fake-region",
				"fspath":        "",
				"endpoint":      "fake-url",
				"skipTLSVerify": "true",
			},
		},
		{
			name: "aws, ObjectStorage section not exists in BSL, s3Url not exist",
			backupLocation: velerov1api.BackupStorageLocation{
				Spec: velerov1api.BackupStorageLocationSpec{
					Provider: "velero.io/aws",
					Config: map[string]string{
						"bucket":                "fake-bucket",
						"prefix":                "fake-prefix",
						"insecureSkipTLSVerify": "false",
					},
				},
			},
			getS3BucketRegion: func(bucket string) (string, error) {
				return "region from bucket: " + bucket, nil
			},
			expected: map[string]string{
				"bucket":        "fake-bucket",
				"prefix":        "fake-prefix/unified-repo/",
				"region":        "region from bucket: fake-bucket",
				"fspath":        "",
				"endpoint":      "s3-region from bucket: fake-bucket.amazonaws.com",
				"skipTLSVerify": "false",
			},
		},
		{
			name: "aws, ObjectStorage section not exists in BSL, s3Url not exist, get region fail",
			backupLocation: velerov1api.BackupStorageLocation{
				Spec: velerov1api.BackupStorageLocationSpec{
					Provider: "velero.io/aws",
					Config:   map[string]string{},
				},
			},
			getS3BucketRegion: func(bucket string) (string, error) {
				return "", errors.New("fake error")
			},
			expected:    map[string]string{},
			expectedErr: "error get s3 bucket region: fake error",
		},
		{
			name: "aws, ObjectStorage section exists in BSL, s3Url exist",
			backupLocation: velerov1api.BackupStorageLocation{
				Spec: velerov1api.BackupStorageLocationSpec{
					Provider: "velero.io/aws",
					Config: map[string]string{
						"bucket":                "fake-bucket-config",
						"prefix":                "fake-prefix-config",
						"region":                "fake-region",
						"s3Url":                 "fake-url",
						"insecureSkipTLSVerify": "false",
					},
					StorageType: velerov1api.StorageType{
						ObjectStorage: &velerov1api.ObjectStorageLocation{
							Bucket: "fake-bucket-object-store",
							Prefix: "fake-prefix-object-store",
						},
					},
				},
			},
			getS3BucketRegion: func(bucket string) (string, error) {
				return "region from bucket: " + bucket, nil
			},
			expected: map[string]string{
				"bucket":        "fake-bucket-object-store",
				"prefix":        "fake-prefix-object-store/unified-repo/",
				"region":        "fake-region",
				"fspath":        "",
				"endpoint":      "fake-url",
				"skipTLSVerify": "false",
			},
		},
		{
			name: "azure, ObjectStorage section exists in BSL",
			backupLocation: velerov1api.BackupStorageLocation{
				Spec: velerov1api.BackupStorageLocationSpec{
					Provider: "velero.io/azure",
					Config: map[string]string{
						"bucket":        "fake-bucket-config",
						"prefix":        "fake-prefix-config",
						"region":        "fake-region",
						"fspath":        "",
						"storageDomain": "fake-domain",
					},
					StorageType: velerov1api.StorageType{
						ObjectStorage: &velerov1api.ObjectStorageLocation{
							Bucket: "fake-bucket-object-store",
							Prefix: "fake-prefix-object-store",
						},
					},
				},
			},
			getAzureStorageDomain: func(config map[string]string) string {
				return config["storageDomain"]
			},
			expected: map[string]string{
				"bucket":        "fake-bucket-object-store",
				"prefix":        "fake-prefix-object-store/unified-repo/",
				"region":        "fake-region",
				"fspath":        "",
				"storageDomain": "fake-domain",
			},
		},
		{
			name: "azure, ObjectStorage section not exists in BSL, repo name exists",
			backupLocation: velerov1api.BackupStorageLocation{
				Spec: velerov1api.BackupStorageLocationSpec{
					Provider: "velero.io/azure",
					Config: map[string]string{
						"bucket":        "fake-bucket",
						"prefix":        "fake-prefix",
						"region":        "fake-region",
						"fspath":        "",
						"storageDomain": "fake-domain",
					},
				},
			},
			repoName: "//fake-name//",
			getAzureStorageDomain: func(config map[string]string) string {
				return config["storageDomain"]
			},
			expected: map[string]string{
				"bucket":        "fake-bucket",
				"prefix":        "fake-prefix/unified-repo/fake-name/",
				"region":        "fake-region",
				"fspath":        "",
				"storageDomain": "fake-domain",
			},
		},
		{
			name: "fs",
			backupLocation: velerov1api.BackupStorageLocation{
				Spec: velerov1api.BackupStorageLocationSpec{
					Provider: "velero.io/fs",
					Config: map[string]string{
						"fspath": "fake-path",
						"prefix": "fake-prefix",
					},
				},
			},
			expected: map[string]string{
				"fspath": "fake-path",
				"bucket": "",
				"prefix": "fake-prefix/unified-repo/",
				"region": "",
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			getS3BucketRegion = tc.getS3BucketRegion
			getAzureStorageDomain = tc.getAzureStorageDomain

			actual, err := getStorageVariables(&tc.backupLocation, tc.repoName)

			require.Equal(t, tc.expected, actual)

			if tc.expectedErr == "" {
				assert.NoError(t, err)
			} else {
				assert.EqualError(t, err, tc.expectedErr)
			}
		})
	}
}
