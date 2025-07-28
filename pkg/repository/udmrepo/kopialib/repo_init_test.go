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

package kopialib

import (
	"testing"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/vmware-tanzu/velero/pkg/repository/udmrepo"
	storagemocks "github.com/vmware-tanzu/velero/pkg/repository/udmrepo/kopialib/backend/mocks"
	velerotest "github.com/vmware-tanzu/velero/pkg/test"
)

type comparableError struct {
	message string
}

func (ce *comparableError) Error() string {
	return ce.message
}

func (ce *comparableError) Is(err error) bool {
	return err.Error() == ce.message
}

func TestCreateBackupRepo(t *testing.T) {
	testCases := []struct {
		name         string
		backendStore *storagemocks.Store
		repoOptions  udmrepo.RepoOptions
		connectErr   error
		setupError   error
		returnStore  *storagemocks.Storage
		storeListErr error
		getBlobErr   error
		listBlobErr  error
		expectedErr  string
	}{
		{
			name:        "invalid config file",
			expectedErr: "invalid config file path",
		},
		{
			name: "storage setup fail, invalid type",
			repoOptions: udmrepo.RepoOptions{
				ConfigFilePath: "fake-file",
			},
			expectedErr: "error to setup backend storage: error to find storage type",
		},
		{
			name: "storage setup fail, backend store steup fail",
			repoOptions: udmrepo.RepoOptions{
				ConfigFilePath: "fake-file",
				StorageType:    udmrepo.StorageTypeAzure,
			},
			backendStore: new(storagemocks.Store),
			setupError:   errors.New("fake-setup-error"),
			expectedErr:  "error to setup backend storage: error to setup storage: fake-setup-error",
		},
		{
			name: "storage connect fail",
			repoOptions: udmrepo.RepoOptions{
				ConfigFilePath: "fake-file",
				StorageType:    udmrepo.StorageTypeAzure,
			},
			backendStore: new(storagemocks.Store),
			connectErr:   errors.New("fake-connect-error"),
			expectedErr:  "error to connect to storage: fake-connect-error",
		},
		{
			name: "create repository error, exist blobs",
			repoOptions: udmrepo.RepoOptions{
				ConfigFilePath: "fake-file",
				StorageType:    udmrepo.StorageTypeAzure,
			},
			backendStore: new(storagemocks.Store),
			returnStore:  new(storagemocks.Storage),
			listBlobErr: &comparableError{
				message: "has data",
			},
			expectedErr: "error to create repo with storage: error to ensure repository storage empty: found existing data in storage location",
		},
		{
			name: "create repository error, error list blobs",
			repoOptions: udmrepo.RepoOptions{
				ConfigFilePath: "fake-file",
				StorageType:    udmrepo.StorageTypeAzure,
			},
			backendStore: new(storagemocks.Store),
			returnStore:  new(storagemocks.Storage),
			listBlobErr:  errors.New("fake-list-blob-error"),
			expectedErr:  "error to create repo with storage: error to ensure repository storage empty: error to list blobs: fake-list-blob-error",
		},
		{
			name: "create repository error, initialize error",
			repoOptions: udmrepo.RepoOptions{
				ConfigFilePath: "fake-file",
				StorageType:    udmrepo.StorageTypeAzure,
			},
			backendStore: new(storagemocks.Store),
			returnStore:  new(storagemocks.Storage),
			getBlobErr:   errors.New("fake-list-blob-error-01"),
			expectedErr:  "error to create repo with storage: error to initialize repository: unexpected error when checking for format blob: fake-list-blob-error-01",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			logger := velerotest.NewLogger()
			backendStores = []kopiaBackendStore{
				{udmrepo.StorageTypeAzure, "fake store", tc.backendStore},
				{udmrepo.StorageTypeFs, "fake store", tc.backendStore},
				{udmrepo.StorageTypeGcs, "fake store", tc.backendStore},
				{udmrepo.StorageTypeS3, "fake store", tc.backendStore},
			}

			if tc.backendStore != nil {
				tc.backendStore.On("Connect", mock.Anything, mock.Anything, mock.Anything).Return(tc.returnStore, tc.connectErr)
				tc.backendStore.On("Setup", mock.Anything, mock.Anything, mock.Anything).Return(tc.setupError)
			}

			if tc.returnStore != nil {
				tc.returnStore.On("ListBlobs", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(tc.listBlobErr)
				tc.returnStore.On("GetBlob", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(tc.getBlobErr)
			}

			err := CreateBackupRepo(t.Context(), tc.repoOptions, logger)

			if tc.expectedErr == "" {
				assert.NoError(t, err)
			} else {
				assert.EqualError(t, err, tc.expectedErr)
			}
		})
	}
}

func TestConnectBackupRepo(t *testing.T) {
	testCases := []struct {
		name         string
		backendStore *storagemocks.Store
		repoOptions  udmrepo.RepoOptions
		connectErr   error
		setupError   error
		returnStore  *storagemocks.Storage
		getBlobErr   error
		expectedErr  string
	}{
		{
			name:        "invalid config file",
			expectedErr: "invalid config file path",
		},
		{
			name: "storage setup fail, invalid type",
			repoOptions: udmrepo.RepoOptions{
				ConfigFilePath: "fake-file",
			},
			expectedErr: "error to setup backend storage: error to find storage type",
		},
		{
			name: "storage setup fail, backend store steup fail",
			repoOptions: udmrepo.RepoOptions{
				ConfigFilePath: "fake-file",
				StorageType:    udmrepo.StorageTypeAzure,
			},
			backendStore: new(storagemocks.Store),
			setupError:   errors.New("fake-setup-error"),
			expectedErr:  "error to setup backend storage: error to setup storage: fake-setup-error",
		},
		{
			name: "storage connect fail",
			repoOptions: udmrepo.RepoOptions{
				ConfigFilePath: "fake-file",
				StorageType:    udmrepo.StorageTypeAzure,
			},
			backendStore: new(storagemocks.Store),
			connectErr:   errors.New("fake-connect-error"),
			expectedErr:  "error to connect to storage: fake-connect-error",
		},
		{
			name: "connect repository error",
			repoOptions: udmrepo.RepoOptions{
				ConfigFilePath: "fake-file",
				StorageType:    udmrepo.StorageTypeAzure,
			},
			backendStore: new(storagemocks.Store),
			returnStore:  new(storagemocks.Storage),
			getBlobErr:   errors.New("fake-get-blob-error"),
			expectedErr:  "error to connect repo with storage: error to connect to repository: unable to read format blob: fake-get-blob-error",
		},
	}

	logger := velerotest.NewLogger()
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			backendStores = []kopiaBackendStore{
				{udmrepo.StorageTypeAzure, "fake store", tc.backendStore},
				{udmrepo.StorageTypeFs, "fake store", tc.backendStore},
				{udmrepo.StorageTypeGcs, "fake store", tc.backendStore},
				{udmrepo.StorageTypeS3, "fake store", tc.backendStore},
			}

			if tc.backendStore != nil {
				tc.backendStore.On("Connect", mock.Anything, mock.Anything, mock.Anything).Return(tc.returnStore, tc.connectErr)
				tc.backendStore.On("Setup", mock.Anything, mock.Anything, mock.Anything).Return(tc.setupError)
			}

			if tc.returnStore != nil {
				tc.returnStore.On("GetBlob", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(tc.getBlobErr)
			}

			err := ConnectBackupRepo(t.Context(), tc.repoOptions, logger)

			if tc.expectedErr == "" {
				assert.NoError(t, err)
			} else {
				assert.EqualError(t, err, tc.expectedErr)
			}
		})
	}
}
