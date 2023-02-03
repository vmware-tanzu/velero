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

package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/vmware-tanzu/velero/internal/credentials"
	velerov1api "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	"github.com/vmware-tanzu/velero/pkg/repository/provider"
	"github.com/vmware-tanzu/velero/pkg/util/filesystem"
)

// SnapshotIdentifier uniquely identifies a snapshot
// taken by Velero.
type SnapshotIdentifier struct {
	// VolumeNamespace is the namespace of the pod/volume that
	// the snapshot is for.
	VolumeNamespace string

	// BackupStorageLocation is the backup's storage location
	// name.
	BackupStorageLocation string

	// SnapshotID is the short ID of the snapshot.
	SnapshotID string

	// RepositoryType is the type of the repository where the
	// snapshot is stored
	RepositoryType string
}

// Manager manages backup repositories.
type Manager interface {
	// InitRepo initializes a repo with the specified name and identifier.
	InitRepo(repo *velerov1api.BackupRepository) error

	// ConnectToRepo tries to connect to the specified repo, and returns an error if it fails.
	// This is intended to be used to ensure that the repo exists/can be authenticated to.
	ConnectToRepo(repo *velerov1api.BackupRepository) error

	// PrepareRepo tries to connect to the specific repo first, if it fails because of the
	// repo is not initialized, it turns to initialize the repo
	PrepareRepo(repo *velerov1api.BackupRepository) error

	// PruneRepo deletes unused data from a repo.
	PruneRepo(repo *velerov1api.BackupRepository) error

	// UnlockRepo removes stale locks from a repo.
	UnlockRepo(repo *velerov1api.BackupRepository) error

	// Forget removes a snapshot from the list of
	// available snapshots in a repo.
	Forget(context.Context, SnapshotIdentifier) error
	// DefaultMaintenanceFrequency returns the default maintenance frequency from the specific repo
	DefaultMaintenanceFrequency(repo *velerov1api.BackupRepository) (time.Duration, error)
}

type manager struct {
	namespace   string
	providers   map[string]provider.Provider
	client      client.Client
	repoLocker  *RepoLocker
	repoEnsurer *RepositoryEnsurer
	fileSystem  filesystem.Interface
	log         logrus.FieldLogger
}

// NewManager create a new repository manager.
func NewManager(
	namespace string,
	client client.Client,
	repoLocker *RepoLocker,
	repoEnsurer *RepositoryEnsurer,
	credentialFileStore credentials.FileStore,
	credentialSecretStore credentials.SecretStore,
	log logrus.FieldLogger,
) Manager {
	mgr := &manager{
		namespace:   namespace,
		client:      client,
		providers:   map[string]provider.Provider{},
		repoLocker:  repoLocker,
		repoEnsurer: repoEnsurer,
		fileSystem:  filesystem.NewFileSystem(),
		log:         log,
	}

	mgr.providers[velerov1api.BackupRepositoryTypeRestic] = provider.NewResticRepositoryProvider(credentialFileStore, mgr.fileSystem, mgr.log)
	mgr.providers[velerov1api.BackupRepositoryTypeKopia] = provider.NewUnifiedRepoProvider(credentials.CredentialGetter{
		FromFile:   credentialFileStore,
		FromSecret: credentialSecretStore,
	}, velerov1api.BackupRepositoryTypeKopia, mgr.log)

	return mgr
}

func (m *manager) InitRepo(repo *velerov1api.BackupRepository) error {
	m.repoLocker.LockExclusive(repo.Name)
	defer m.repoLocker.UnlockExclusive(repo.Name)

	prd, err := m.getRepositoryProvider(repo)
	if err != nil {
		return errors.WithStack(err)
	}
	param, err := m.assembleRepoParam(repo)
	if err != nil {
		return errors.WithStack(err)
	}
	return prd.InitRepo(context.Background(), param)
}

func (m *manager) ConnectToRepo(repo *velerov1api.BackupRepository) error {
	m.repoLocker.Lock(repo.Name)
	defer m.repoLocker.Unlock(repo.Name)

	prd, err := m.getRepositoryProvider(repo)
	if err != nil {
		return errors.WithStack(err)
	}
	param, err := m.assembleRepoParam(repo)
	if err != nil {
		return errors.WithStack(err)
	}
	return prd.ConnectToRepo(context.Background(), param)
}

func (m *manager) PrepareRepo(repo *velerov1api.BackupRepository) error {
	m.repoLocker.Lock(repo.Name)
	defer m.repoLocker.Unlock(repo.Name)

	prd, err := m.getRepositoryProvider(repo)
	if err != nil {
		return errors.WithStack(err)
	}
	param, err := m.assembleRepoParam(repo)
	if err != nil {
		return errors.WithStack(err)
	}
	return prd.PrepareRepo(context.Background(), param)
}

func (m *manager) PruneRepo(repo *velerov1api.BackupRepository) error {
	m.repoLocker.LockExclusive(repo.Name)
	defer m.repoLocker.UnlockExclusive(repo.Name)

	prd, err := m.getRepositoryProvider(repo)
	if err != nil {
		return errors.WithStack(err)
	}
	param, err := m.assembleRepoParam(repo)
	if err != nil {
		return errors.WithStack(err)
	}

	if err := prd.BoostRepoConnect(context.Background(), param); err != nil {
		return errors.WithStack(err)
	}

	return prd.PruneRepo(context.Background(), param)
}

func (m *manager) UnlockRepo(repo *velerov1api.BackupRepository) error {
	m.repoLocker.Lock(repo.Name)
	defer m.repoLocker.Unlock(repo.Name)

	prd, err := m.getRepositoryProvider(repo)
	if err != nil {
		return errors.WithStack(err)
	}
	param, err := m.assembleRepoParam(repo)
	if err != nil {
		return errors.WithStack(err)
	}
	return prd.EnsureUnlockRepo(context.Background(), param)
}

func (m *manager) Forget(ctx context.Context, snapshot SnapshotIdentifier) error {
	repo, err := m.repoEnsurer.EnsureRepo(ctx, m.namespace, snapshot.VolumeNamespace, snapshot.BackupStorageLocation, snapshot.RepositoryType)
	if err != nil {
		return err
	}

	m.repoLocker.LockExclusive(repo.Name)
	defer m.repoLocker.UnlockExclusive(repo.Name)

	prd, err := m.getRepositoryProvider(repo)
	if err != nil {
		return errors.WithStack(err)
	}
	param, err := m.assembleRepoParam(repo)
	if err != nil {
		return errors.WithStack(err)
	}

	if err := prd.BoostRepoConnect(context.Background(), param); err != nil {
		return errors.WithStack(err)
	}

	return prd.Forget(context.Background(), snapshot.SnapshotID, param)
}

func (m *manager) DefaultMaintenanceFrequency(repo *velerov1api.BackupRepository) (time.Duration, error) {
	prd, err := m.getRepositoryProvider(repo)
	if err != nil {
		return 0, errors.WithStack(err)
	}

	param, err := m.assembleRepoParam(repo)
	if err != nil {
		return 0, errors.WithStack(err)
	}

	return prd.DefaultMaintenanceFrequency(context.Background(), param), nil
}

func (m *manager) getRepositoryProvider(repo *velerov1api.BackupRepository) (provider.Provider, error) {
	switch repo.Spec.RepositoryType {
	case "", velerov1api.BackupRepositoryTypeRestic:
		return m.providers[velerov1api.BackupRepositoryTypeRestic], nil
	case velerov1api.BackupRepositoryTypeKopia:
		return m.providers[velerov1api.BackupRepositoryTypeKopia], nil
	default:
		return nil, fmt.Errorf("failed to get provider for repository %s", repo.Spec.RepositoryType)
	}
}

func (m *manager) assembleRepoParam(repo *velerov1api.BackupRepository) (provider.RepoParam, error) {
	bsl := &velerov1api.BackupStorageLocation{}
	if err := m.client.Get(context.Background(), client.ObjectKey{m.namespace, repo.Spec.BackupStorageLocation}, bsl); err != nil {
		return provider.RepoParam{}, errors.WithStack(err)
	}
	return provider.RepoParam{
		BackupLocation: bsl,
		BackupRepo:     repo,
	}, nil
}
