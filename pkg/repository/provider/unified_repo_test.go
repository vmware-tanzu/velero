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
	"context"
	"errors"
	"testing"

	awscredentials "github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	corev1api "k8s.io/api/core/v1"

	velerocredentials "github.com/vmware-tanzu/velero/internal/credentials"
	credmock "github.com/vmware-tanzu/velero/internal/credentials/mocks"
	velerov1api "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	"github.com/vmware-tanzu/velero/pkg/repository/udmrepo"
	reposervicenmocks "github.com/vmware-tanzu/velero/pkg/repository/udmrepo/mocks"
	velerotest "github.com/vmware-tanzu/velero/pkg/test"
)

func TestGetStorageCredentials(t *testing.T) {
	testCases := []struct {
		name                string
		backupLocation      velerov1api.BackupStorageLocation
		credFileStore       *credmock.FileStore
		credStoreError      error
		credStorePath       string
		getAzureCredentials func(map[string]string) (string, string, error)
		getS3Credentials    func(map[string]string) (awscredentials.Value, error)
		getGCPCredentials   func(map[string]string) string
		expected            map[string]string
		expectedErr         string
	}{
		{
			name:        "invalid credentials file store interface",
			expected:    map[string]string{},
			expectedErr: "invalid credentials interface",
		},
		{
			name: "invalid provider",
			backupLocation: velerov1api.BackupStorageLocation{
				Spec: velerov1api.BackupStorageLocationSpec{
					Provider: "invalid-provider",
				},
			},
			credFileStore: new(credmock.FileStore),
			expected:      map[string]string{},
			expectedErr:   "invalid storage provider",
		},
		{
			name: "credential section exists in BSL, file store fail",
			backupLocation: velerov1api.BackupStorageLocation{
				Spec: velerov1api.BackupStorageLocationSpec{
					Provider:   "aws",
					Credential: &corev1api.SecretKeySelector{},
				},
			},
			credFileStore:  new(credmock.FileStore),
			credStoreError: errors.New("fake error"),
			expected:       map[string]string{},
			expectedErr:    "error get credential file in bsl: fake error",
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
			credFileStore: new(credmock.FileStore),
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
			credFileStore: new(credmock.FileStore),
			credStorePath: "credentials-from-credential-key",
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
			getS3Credentials: func(config map[string]string) (awscredentials.Value, error) {
				return awscredentials.Value{}, errors.New("fake error")
			},
			credFileStore: new(credmock.FileStore),
			expected:      map[string]string{},
			expectedErr:   "error get s3 credentials: fake error",
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
			credFileStore: new(credmock.FileStore),
			credStorePath: "credentials-from-credential-key",
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
			credFileStore: new(credmock.FileStore),
			expected:      map[string]string{},
			expectedErr:   "error get azure credentials: fake error",
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
			credFileStore: new(credmock.FileStore),
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

			var fileStore velerocredentials.FileStore
			if tc.credFileStore != nil {
				tc.credFileStore.On("Path", mock.Anything, mock.Anything).Return(tc.credStorePath, tc.credStoreError)
				fileStore = tc.credFileStore
			}

			actual, err := getStorageCredentials(&tc.backupLocation, fileStore)

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
		repoBackend           string
		getS3BucketRegion     func(string) (string, error)
		getAzureStorageDomain func(map[string]string) (string, error)
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
			name: "aws, ObjectStorage section not exists in BSL, s3Url exist, https",
			backupLocation: velerov1api.BackupStorageLocation{
				Spec: velerov1api.BackupStorageLocationSpec{
					Provider: "velero.io/aws",
					Config: map[string]string{
						"bucket":                "fake-bucket",
						"prefix":                "fake-prefix",
						"region":                "fake-region/",
						"s3Url":                 "https://fake-url/",
						"insecureSkipTLSVerify": "true",
					},
				},
			},
			repoBackend: "fake-repo-type",
			expected: map[string]string{
				"bucket":        "fake-bucket",
				"prefix":        "fake-prefix/fake-repo-type/",
				"region":        "fake-region",
				"fspath":        "",
				"endpoint":      "fake-url",
				"doNotUseTLS":   "false",
				"skipTLSVerify": "true",
			},
		},
		{
			name: "aws, ObjectStorage section not exists in BSL, s3Url exist, invalid",
			backupLocation: velerov1api.BackupStorageLocation{
				Spec: velerov1api.BackupStorageLocationSpec{
					Provider: "velero.io/aws",
					Config: map[string]string{
						"bucket":                "fake-bucket",
						"prefix":                "fake-prefix",
						"region":                "fake-region/",
						"s3Url":                 "https://fake-url/fake-path",
						"insecureSkipTLSVerify": "true",
					},
				},
			},
			repoBackend: "fake-repo-type",
			expected:    map[string]string{},
			expectedErr: "path is not expected in s3Url https://fake-url/fake-path",
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
			repoBackend: "fake-repo-type",
			expected: map[string]string{
				"bucket":        "fake-bucket",
				"prefix":        "fake-prefix/fake-repo-type/",
				"region":        "region from bucket: fake-bucket",
				"fspath":        "",
				"endpoint":      "s3-region from bucket: fake-bucket.amazonaws.com",
				"doNotUseTLS":   "false",
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
			name: "aws, ObjectStorage section exists in BSL, s3Url exist, http",
			backupLocation: velerov1api.BackupStorageLocation{
				Spec: velerov1api.BackupStorageLocationSpec{
					Provider: "velero.io/aws",
					Config: map[string]string{
						"bucket":                "fake-bucket-config",
						"prefix":                "fake-prefix-config",
						"region":                "fake-region",
						"s3Url":                 "http://fake-url/",
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
			repoBackend: "fake-repo-type",
			expected: map[string]string{
				"bucket":        "fake-bucket-object-store",
				"prefix":        "fake-prefix-object-store/fake-repo-type/",
				"region":        "fake-region",
				"fspath":        "",
				"endpoint":      "fake-url",
				"doNotUseTLS":   "true",
				"skipTLSVerify": "false",
			},
		},
		{
			name: "azure, getAzureStorageDomain fail",
			backupLocation: velerov1api.BackupStorageLocation{
				Spec: velerov1api.BackupStorageLocationSpec{
					Provider: "velero.io/azure",
					Config: map[string]string{
						"bucket": "fake-bucket-config",
						"prefix": "fake-prefix-config",
						"region": "fake-region",
						"fspath": "",
					},
					StorageType: velerov1api.StorageType{
						ObjectStorage: &velerov1api.ObjectStorageLocation{
							Bucket: "fake-bucket-object-store",
							Prefix: "fake-prefix-object-store",
						},
					},
				},
			},
			getAzureStorageDomain: func(config map[string]string) (string, error) {
				return "", errors.New("fake error")
			},
			repoBackend: "fake-repo-type",
			expected:    map[string]string{},
			expectedErr: "error to get azure storage domain: fake error",
		},
		{
			name: "azure, ObjectStorage section exists in BSL",
			backupLocation: velerov1api.BackupStorageLocation{
				Spec: velerov1api.BackupStorageLocationSpec{
					Provider: "velero.io/azure",
					Config: map[string]string{
						"bucket": "fake-bucket-config",
						"prefix": "fake-prefix-config",
						"region": "fake-region",
						"fspath": "",
					},
					StorageType: velerov1api.StorageType{
						ObjectStorage: &velerov1api.ObjectStorageLocation{
							Bucket: "fake-bucket-object-store",
							Prefix: "fake-prefix-object-store",
						},
					},
				},
			},
			getAzureStorageDomain: func(config map[string]string) (string, error) {
				return "fake-domain", nil
			},
			repoBackend: "fake-repo-type",
			expected: map[string]string{
				"bucket":        "fake-bucket-object-store",
				"prefix":        "fake-prefix-object-store/fake-repo-type/",
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
						"bucket": "fake-bucket",
						"prefix": "fake-prefix",
						"region": "fake-region",
						"fspath": "",
					},
				},
			},
			repoName:    "//fake-name//",
			repoBackend: "fake-repo-type",
			getAzureStorageDomain: func(config map[string]string) (string, error) {
				return "fake-domain", nil
			},
			expected: map[string]string{
				"bucket":        "fake-bucket",
				"prefix":        "fake-prefix/fake-repo-type/fake-name/",
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
			repoBackend: "fake-repo-type",
			expected: map[string]string{
				"fspath": "fake-path",
				"bucket": "",
				"prefix": "fake-prefix/fake-repo-type/",
				"region": "",
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			getS3BucketRegion = tc.getS3BucketRegion
			getAzureStorageDomain = tc.getAzureStorageDomain

			actual, err := getStorageVariables(&tc.backupLocation, tc.repoBackend, tc.repoName)

			require.Equal(t, tc.expected, actual)

			if tc.expectedErr == "" {
				assert.NoError(t, err)
			} else {
				assert.EqualError(t, err, tc.expectedErr)
			}
		})
	}
}

func TestGetRepoPassword(t *testing.T) {
	testCases := []struct {
		name            string
		getter          *credmock.SecretStore
		credStoreReturn string
		credStoreError  error
		cached          string
		expected        string
		expectedErr     string
	}{
		{
			name:        "invalid secret interface",
			expectedErr: "invalid credentials interface",
		},
		{
			name:           "error from secret interface",
			getter:         new(credmock.SecretStore),
			credStoreError: errors.New("fake error"),
			expectedErr:    "error to get password: fake error",
		},
		{
			name:            "secret with whitespace",
			getter:          new(credmock.SecretStore),
			credStoreReturn: " fake-passwor d  ",
			expected:        "fake-passwor d",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var secretStore velerocredentials.SecretStore
			if tc.getter != nil {
				tc.getter.On("Get", mock.Anything, mock.Anything).Return(tc.credStoreReturn, tc.credStoreError)
				secretStore = tc.getter
			}

			urp := unifiedRepoProvider{
				credentialGetter: velerocredentials.CredentialGetter{
					FromSecret: secretStore,
				},
			}

			password, err := getRepoPassword(urp.credentialGetter.FromSecret)

			require.Equal(t, tc.expected, password)

			if tc.expectedErr == "" {
				assert.NoError(t, err)
			} else {
				assert.EqualError(t, err, tc.expectedErr)
			}
		})
	}
}

func TestGetStoreOptions(t *testing.T) {
	testCases := []struct {
		name        string
		funcTable   localFuncTable
		repoParam   interface{}
		expected    map[string]string
		expectedErr string
	}{
		{
			name:        "wrong param type",
			repoParam:   struct{}{},
			expected:    map[string]string{},
			expectedErr: "invalid parameter, expect provider.RepoParam, actual struct {}",
		},
		{
			name: "get storage variable fail",
			repoParam: RepoParam{
				BackupLocation: &velerov1api.BackupStorageLocation{},
				BackupRepo:     &velerov1api.BackupRepository{},
			},
			funcTable: localFuncTable{
				getStorageVariables: func(*velerov1api.BackupStorageLocation, string, string) (map[string]string, error) {
					return map[string]string{}, errors.New("fake-error-2")
				},
			},
			expected:    map[string]string{},
			expectedErr: "error to get storage variables: fake-error-2",
		},
		{
			name: "get storage credentials fail",
			repoParam: RepoParam{
				BackupLocation: &velerov1api.BackupStorageLocation{},
				BackupRepo:     &velerov1api.BackupRepository{},
			},
			funcTable: localFuncTable{
				getStorageVariables: func(*velerov1api.BackupStorageLocation, string, string) (map[string]string, error) {
					return map[string]string{}, nil
				},
				getStorageCredentials: func(*velerov1api.BackupStorageLocation, velerocredentials.FileStore) (map[string]string, error) {
					return map[string]string{}, errors.New("fake-error-3")
				},
			},
			expected:    map[string]string{},
			expectedErr: "error to get repo credentials: fake-error-3",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			funcTable = tc.funcTable

			urp := unifiedRepoProvider{}

			options, err := urp.GetStoreOptions(tc.repoParam)

			require.Equal(t, tc.expected, options)

			if tc.expectedErr == "" {
				assert.NoError(t, err)
			} else {
				assert.EqualError(t, err, tc.expectedErr)
			}
		})
	}
}

func TestPrepareRepo(t *testing.T) {
	testCases := []struct {
		name            string
		funcTable       localFuncTable
		getter          *credmock.SecretStore
		repoService     *reposervicenmocks.BackupRepoService
		retFuncInit     func(context.Context, udmrepo.RepoOptions, bool) error
		credStoreReturn string
		credStoreError  error
		expectedErr     string
	}{
		{
			name:        "get repo option fail",
			repoService: new(reposervicenmocks.BackupRepoService),
			expectedErr: "error to get repo options: error to get repo password: invalid credentials interface",
		},
		{
			name:           "get repo option fail, get password fail",
			getter:         new(credmock.SecretStore),
			repoService:    new(reposervicenmocks.BackupRepoService),
			credStoreError: errors.New("fake-password-error"),
			expectedErr:    "error to get repo options: error to get repo password: error to get password: fake-password-error",
		},
		{
			name:            "get repo option fail, get store options fail",
			getter:          new(credmock.SecretStore),
			repoService:     new(reposervicenmocks.BackupRepoService),
			credStoreReturn: "fake-password",
			funcTable: localFuncTable{
				getStorageVariables: func(*velerov1api.BackupStorageLocation, string, string) (map[string]string, error) {
					return map[string]string{}, errors.New("fake-store-option-error")
				},
			},
			expectedErr: "error to get repo options: error to get storage variables: fake-store-option-error",
		},
		{
			name:            "already initialized",
			getter:          new(credmock.SecretStore),
			credStoreReturn: "fake-password",
			funcTable: localFuncTable{
				getStorageVariables: func(*velerov1api.BackupStorageLocation, string, string) (map[string]string, error) {
					return map[string]string{}, nil
				},
				getStorageCredentials: func(*velerov1api.BackupStorageLocation, velerocredentials.FileStore) (map[string]string, error) {
					return map[string]string{}, nil
				},
			},
			repoService: new(reposervicenmocks.BackupRepoService),
			retFuncInit: func(ctx context.Context, repoOption udmrepo.RepoOptions, createNew bool) error {
				if !createNew {
					return nil
				} else {
					return errors.New("fake-error")
				}
			},
		},
		{
			name:            "initialize fail",
			getter:          new(credmock.SecretStore),
			credStoreReturn: "fake-password",
			funcTable: localFuncTable{
				getStorageVariables: func(*velerov1api.BackupStorageLocation, string, string) (map[string]string, error) {
					return map[string]string{}, nil
				},
				getStorageCredentials: func(*velerov1api.BackupStorageLocation, velerocredentials.FileStore) (map[string]string, error) {
					return map[string]string{}, nil
				},
			},
			repoService: new(reposervicenmocks.BackupRepoService),
			retFuncInit: func(ctx context.Context, repoOption udmrepo.RepoOptions, createNew bool) error {
				if !createNew {
					return errors.New("fake-error-1")
				} else {
					return errors.New("fake-error-2")
				}
			},
			expectedErr: "error to init backup repo: fake-error-2",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			funcTable = tc.funcTable

			var secretStore velerocredentials.SecretStore
			if tc.getter != nil {
				tc.getter.On("Get", mock.Anything, mock.Anything).Return(tc.credStoreReturn, tc.credStoreError)
				secretStore = tc.getter
			}

			urp := unifiedRepoProvider{
				credentialGetter: velerocredentials.CredentialGetter{
					FromSecret: secretStore,
				},
				repoService: tc.repoService,
				log:         velerotest.NewLogger(),
			}

			tc.repoService.On("Init", mock.Anything, mock.Anything, mock.Anything).Return(tc.retFuncInit)

			err := urp.PrepareRepo(context.Background(), RepoParam{
				BackupLocation: &velerov1api.BackupStorageLocation{},
				BackupRepo:     &velerov1api.BackupRepository{},
			})

			if tc.expectedErr == "" {
				assert.NoError(t, err)
			} else {
				assert.EqualError(t, err, tc.expectedErr)
			}
		})
	}
}

func TestForget(t *testing.T) {
	var backupRepo *reposervicenmocks.BackupRepo

	testCases := []struct {
		name            string
		funcTable       localFuncTable
		getter          *credmock.SecretStore
		repoService     *reposervicenmocks.BackupRepoService
		backupRepo      *reposervicenmocks.BackupRepo
		retFuncOpen     []interface{}
		retFuncDelete   interface{}
		credStoreReturn string
		credStoreError  error
		expectedErr     string
	}{
		{
			name:        "get repo option fail",
			expectedErr: "error to get repo options: error to get repo password: invalid credentials interface",
		},
		{
			name:            "repo open fail",
			getter:          new(credmock.SecretStore),
			credStoreReturn: "fake-password",
			funcTable: localFuncTable{
				getStorageVariables: func(*velerov1api.BackupStorageLocation, string, string) (map[string]string, error) {
					return map[string]string{}, nil
				},
				getStorageCredentials: func(*velerov1api.BackupStorageLocation, velerocredentials.FileStore) (map[string]string, error) {
					return map[string]string{}, nil
				},
			},
			repoService: new(reposervicenmocks.BackupRepoService),
			retFuncOpen: []interface{}{
				func(context.Context, udmrepo.RepoOptions) udmrepo.BackupRepo {
					return backupRepo
				},

				func(context.Context, udmrepo.RepoOptions) error {
					return errors.New("fake-error-2")
				},
			},
			expectedErr: "error to open backup repo: fake-error-2",
		},
		{
			name:            "delete fail",
			getter:          new(credmock.SecretStore),
			credStoreReturn: "fake-password",
			funcTable: localFuncTable{
				getStorageVariables: func(*velerov1api.BackupStorageLocation, string, string) (map[string]string, error) {
					return map[string]string{}, nil
				},
				getStorageCredentials: func(*velerov1api.BackupStorageLocation, velerocredentials.FileStore) (map[string]string, error) {
					return map[string]string{}, nil
				},
			},
			repoService: new(reposervicenmocks.BackupRepoService),
			backupRepo:  new(reposervicenmocks.BackupRepo),
			retFuncOpen: []interface{}{
				func(context.Context, udmrepo.RepoOptions) udmrepo.BackupRepo {
					return backupRepo
				},

				func(context.Context, udmrepo.RepoOptions) error {
					return nil
				},
			},
			retFuncDelete: func(context.Context, udmrepo.ID) error {
				return errors.New("fake-error-3")
			},
			expectedErr: "error to delete manifest: fake-error-3",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			funcTable = tc.funcTable

			var secretStore velerocredentials.SecretStore
			if tc.getter != nil {
				tc.getter.On("Get", mock.Anything, mock.Anything).Return(tc.credStoreReturn, tc.credStoreError)
				secretStore = tc.getter
			}

			urp := unifiedRepoProvider{
				credentialGetter: velerocredentials.CredentialGetter{
					FromSecret: secretStore,
				},
				repoService: tc.repoService,
				log:         velerotest.NewLogger(),
			}

			backupRepo = tc.backupRepo

			if tc.repoService != nil {
				tc.repoService.On("Open", mock.Anything, mock.Anything).Return(tc.retFuncOpen[0], tc.retFuncOpen[1])
			}

			if tc.backupRepo != nil {
				backupRepo.On("DeleteManifest", mock.Anything, mock.Anything).Return(tc.retFuncDelete)
				backupRepo.On("Close", mock.Anything).Return(nil)
			}

			err := urp.Forget(context.Background(), "", RepoParam{
				BackupLocation: &velerov1api.BackupStorageLocation{},
				BackupRepo:     &velerov1api.BackupRepository{},
			})

			if tc.expectedErr == "" {
				assert.NoError(t, err)
			} else {
				assert.EqualError(t, err, tc.expectedErr)
			}
		})
	}
}
