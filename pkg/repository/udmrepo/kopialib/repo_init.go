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
	"context"
	"strings"

	"github.com/kopia/kopia/repo"
	"github.com/kopia/kopia/repo/blob"
	"github.com/pkg/errors"

	"github.com/vmware-tanzu/velero/pkg/repository/udmrepo"
	"github.com/vmware-tanzu/velero/pkg/repository/udmrepo/kopialib/backend"
)

type kopiaBackendStore struct {
	name        string
	description string
	store       backend.Store
}

// backendStores lists the supported backend storages at present
var backendStores []kopiaBackendStore = []kopiaBackendStore{
	{udmrepo.StorageTypeAzure, "an Azure blob storage", &backend.AzureBackend{}},
	{udmrepo.StorageTypeFs, "a filesystem", &backend.FsBackend{}},
	{udmrepo.StorageTypeGcs, "a Google Cloud Storage bucket", &backend.GCSBackend{}},
	{udmrepo.StorageTypeS3, "an S3 bucket", &backend.S3Backend{}},
}

// CreateBackupRepo creates a Kopia repository and then connect to it.
// The storage must be empty, otherwise, it will fail
func CreateBackupRepo(ctx context.Context, repoOption udmrepo.RepoOptions) error {
	if repoOption.ConfigFilePath == "" {
		return errors.New("invalid config file path")
	}

	backendStore, err := setupBackendStore(ctx, repoOption.StorageType, repoOption.StorageOptions)
	if err != nil {
		return errors.Wrap(err, "error to setup backend storage")
	}

	st, err := backendStore.store.Connect(ctx, true)
	if err != nil {
		return errors.Wrap(err, "error to connect to storage")
	}

	err = createWithStorage(ctx, st, repoOption)
	if err != nil {
		return errors.Wrap(err, "error to create repo with storage")
	}

	err = connectWithStorage(ctx, st, repoOption)
	if err != nil {
		return errors.Wrap(err, "error to connect repo with storage")
	}

	return nil
}

// ConnectBackupRepo connects to an existing Kopia repository.
// If the repository doesn't exist, it will fail
func ConnectBackupRepo(ctx context.Context, repoOption udmrepo.RepoOptions) error {
	if repoOption.ConfigFilePath == "" {
		return errors.New("invalid config file path")
	}

	backendStore, err := setupBackendStore(ctx, repoOption.StorageType, repoOption.StorageOptions)
	if err != nil {
		return errors.Wrap(err, "error to setup backend storage")
	}

	st, err := backendStore.store.Connect(ctx, false)
	if err != nil {
		return errors.Wrap(err, "error to connect to storage")
	}

	err = connectWithStorage(ctx, st, repoOption)
	if err != nil {
		return errors.Wrap(err, "error to connect repo with storage")
	}

	return nil
}

func findBackendStore(storage string) *kopiaBackendStore {
	for _, options := range backendStores {
		if strings.EqualFold(options.name, storage) {
			return &options
		}
	}

	return nil
}

func setupBackendStore(ctx context.Context, storageType string, storageOptions map[string]string) (*kopiaBackendStore, error) {
	backendStore := findBackendStore(storageType)
	if backendStore == nil {
		return nil, errors.New("error to find storage type")
	}

	err := backendStore.store.Setup(ctx, storageOptions)
	if err != nil {
		return nil, errors.Wrap(err, "error to setup storage")
	}

	return backendStore, nil
}

func createWithStorage(ctx context.Context, st blob.Storage, repoOption udmrepo.RepoOptions) error {
	err := ensureEmpty(ctx, st)
	if err != nil {
		return errors.Wrap(err, "error to ensure repository storage empty")
	}

	options := backend.SetupNewRepositoryOptions(ctx, repoOption.GeneralOptions)

	if err := repo.Initialize(ctx, st, &options, repoOption.RepoPassword); err != nil {
		return errors.Wrap(err, "error to initialize repository")
	}

	return nil
}

func connectWithStorage(ctx context.Context, st blob.Storage, repoOption udmrepo.RepoOptions) error {
	options := backend.SetupConnectOptions(ctx, repoOption)
	if err := repo.Connect(ctx, repoOption.ConfigFilePath, st, repoOption.RepoPassword, &options); err != nil {
		return errors.Wrap(err, "error to connect to repository")
	}

	return nil
}

func ensureEmpty(ctx context.Context, s blob.Storage) error {
	hasDataError := errors.Errorf("has data")

	err := s.ListBlobs(ctx, "", func(cb blob.Metadata) error {
		return hasDataError
	})

	if errors.Is(err, hasDataError) {
		return errors.New("found existing data in storage location")
	}

	return errors.Wrap(err, "error to list blobs")
}
