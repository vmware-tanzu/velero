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
	"strings"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/vmware-tanzu/velero/internal/credentials"
	"github.com/vmware-tanzu/velero/pkg/repository/restic"
	"github.com/vmware-tanzu/velero/pkg/util/filesystem"
)

func NewResticRepositoryProvider(store credentials.FileStore, fs filesystem.Interface, log logrus.FieldLogger) Provider {
	return &resticRepositoryProvider{
		svc: restic.NewRepositoryService(store, fs, log),
	}
}

type resticRepositoryProvider struct {
	svc *restic.RepositoryService
}

func (r *resticRepositoryProvider) InitRepo(ctx context.Context, param RepoParam) error {
	return r.svc.InitRepo(param.BackupLocation, param.BackupRepo)
}

func (r *resticRepositoryProvider) ConnectToRepo(ctx context.Context, param RepoParam) error {
	return r.svc.ConnectToRepo(param.BackupLocation, param.BackupRepo)
}

func (r *resticRepositoryProvider) PrepareRepo(ctx context.Context, param RepoParam) error {
	if err := r.ConnectToRepo(ctx, param); err != nil {
		// If the repository has not yet been initialized, the error message will always include
		// the following string. This is the only scenario where we should try to initialize it.
		// Other errors (e.g. "already locked") should be returned as-is since the repository
		// does already exist, but it can't be connected to.
		if strings.Contains(err.Error(), "Is there a repository at the following location?") {
			return r.InitRepo(ctx, param)
		}

		return err
	}

	return nil
}

func (r *resticRepositoryProvider) BoostRepoConnect(ctx context.Context, param RepoParam) error {
	return nil
}

func (r *resticRepositoryProvider) PruneRepo(ctx context.Context, param RepoParam) error {
	return r.svc.PruneRepo(param.BackupLocation, param.BackupRepo)
}

func (r *resticRepositoryProvider) EnsureUnlockRepo(ctx context.Context, param RepoParam) error {
	return r.svc.UnlockRepo(param.BackupLocation, param.BackupRepo)
}

func (r *resticRepositoryProvider) Forget(ctx context.Context, snapshotID string, param RepoParam) error {
	return r.svc.Forget(param.BackupLocation, param.BackupRepo, snapshotID)
}

func (r *resticRepositoryProvider) DefaultMaintenanceFrequency(ctx context.Context, param RepoParam) time.Duration {
	return r.svc.DefaultMaintenanceFrequency()
}
