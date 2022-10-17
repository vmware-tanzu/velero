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
	"time"

	"github.com/pkg/errors"

	"github.com/sirupsen/logrus"
	v1 "k8s.io/api/core/v1"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/vmware-tanzu/velero/internal/credentials"
	velerov1api "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	"github.com/vmware-tanzu/velero/pkg/repository/provider"
	"github.com/vmware-tanzu/velero/pkg/uploader"
	"github.com/vmware-tanzu/velero/pkg/util/filesystem"
)

const restoreProgressCheckInterval = 10 * time.Second
const backupProgressCheckInterval = 10 * time.Second

// Provider which is designed for one pod volume to do the backup or restore
type Provider interface {
	// RunBackup which will do backup for one specific volume and return snapshotID, isSnapshotEmpty, error
	// updater is used for updating backup progress which implement by third-party
	RunBackup(
		ctx context.Context,
		path string,
		tags map[string]string,
		parentSnapshot string,
		updater uploader.ProgressUpdater) (string, bool, error)
	// RunRestore which will do restore for one specific volume with given snapshot id and return error
	// updater is used for updating backup progress which implement by third-party
	RunRestore(
		ctx context.Context,
		snapshotID string,
		volumePath string,
		updater uploader.ProgressUpdater) error
	// Close which will close related repository
	Close(ctx context.Context) error
}

// NewUploaderProvider initialize provider with specific uploaderType
func NewUploaderProvider(
	ctx context.Context,
	client client.Client,
	uploaderType string,
	repoIdentifier string,
	bsl *velerov1api.BackupStorageLocation,
	backupRepo *velerov1api.BackupRepository,
	credGetter *credentials.CredentialGetter,
	repoKeySelector *v1.SecretKeySelector,
	log logrus.FieldLogger,
) (Provider, error) {
	if credGetter.FromFile == nil {
		return nil, errors.New("uninitialized FileStore credentail is not supported")
	}
	if uploaderType == uploader.KopiaType {
		// We use the hardcode repositoryType velerov1api.BackupRepositoryTypeKopia for now, because we have only one implementation of unified repo.
		// TODO: post v1.10, replace the hardcode. In future, when we have multiple implementations of Unified Repo (besides Kopia), we will add the
		// repositoryType to BSL, because by then, we are not able to hardcode the repositoryType to BackupRepositoryTypeKopia for Unified Repo.
		if err := provider.NewUnifiedRepoProvider(*credGetter, velerov1api.BackupRepositoryTypeKopia, log).ConnectToRepo(ctx, provider.RepoParam{BackupLocation: bsl, BackupRepo: backupRepo}); err != nil {
			return nil, errors.Wrap(err, "failed to connect repository")
		}
		return NewKopiaUploaderProvider(ctx, credGetter, backupRepo, log)
	} else {
		if err := provider.NewResticRepositoryProvider(credGetter.FromFile, filesystem.NewFileSystem(), log).ConnectToRepo(ctx, provider.RepoParam{BackupLocation: bsl, BackupRepo: backupRepo}); err != nil {
			return nil, errors.Wrap(err, "failed to connect repository")
		}
		return NewResticUploaderProvider(repoIdentifier, bsl, credGetter, repoKeySelector, log)
	}
}
