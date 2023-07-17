/*
Copyright The Velero Contributors.

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

package datapath

import (
	"context"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/vmware-tanzu/velero/internal/credentials"
	velerov1api "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	"github.com/vmware-tanzu/velero/pkg/repository"
	repokey "github.com/vmware-tanzu/velero/pkg/repository/keys"
	repoProvider "github.com/vmware-tanzu/velero/pkg/repository/provider"
	"github.com/vmware-tanzu/velero/pkg/uploader"
	"github.com/vmware-tanzu/velero/pkg/uploader/provider"
	"github.com/vmware-tanzu/velero/pkg/util/filesystem"
)

type fileSystemBR struct {
	ctx            context.Context
	cancel         context.CancelFunc
	backupRepo     *velerov1api.BackupRepository
	uploaderProv   provider.Provider
	log            logrus.FieldLogger
	client         client.Client
	backupLocation *velerov1api.BackupStorageLocation
	namespace      string
	initialized    bool
	callbacks      Callbacks
	jobName        string
	requestorType  string
}

func newFileSystemBR(jobName string, requestorType string, client client.Client, namespace string, callbacks Callbacks, log logrus.FieldLogger) AsyncBR {
	fs := &fileSystemBR{
		jobName:       jobName,
		requestorType: requestorType,
		client:        client,
		namespace:     namespace,
		callbacks:     callbacks,
		log:           log,
	}

	return fs
}

func (fs *fileSystemBR) Init(ctx context.Context, bslName string, sourceNamespace string, uploaderType string, repositoryType string,
	repoIdentifier string, repositoryEnsurer *repository.Ensurer, credentialGetter *credentials.CredentialGetter) error {
	var err error
	defer func() {
		if err != nil {
			fs.Close(ctx)
		}
	}()

	fs.ctx, fs.cancel = context.WithCancel(ctx)

	backupLocation := &velerov1api.BackupStorageLocation{}
	if err = fs.client.Get(ctx, client.ObjectKey{
		Namespace: fs.namespace,
		Name:      bslName,
	}, backupLocation); err != nil {
		return errors.Wrapf(err, "error getting backup storage location %s", bslName)
	}

	fs.backupLocation = backupLocation

	fs.backupRepo, err = repositoryEnsurer.EnsureRepo(ctx, fs.namespace, sourceNamespace, bslName, repositoryType)
	if err != nil {
		return errors.Wrapf(err, "error to ensure backup repository %s-%s-%s", bslName, sourceNamespace, repositoryType)
	}

	err = fs.boostRepoConnect(ctx, repositoryType, credentialGetter)
	if err != nil {
		return errors.Wrapf(err, "error to boost backup repository connection %s-%s-%s", bslName, sourceNamespace, repositoryType)
	}

	fs.uploaderProv, err = provider.NewUploaderProvider(ctx, fs.client, uploaderType, fs.requestorType, repoIdentifier,
		fs.backupLocation, fs.backupRepo, credentialGetter, repokey.RepoKeySelector(), fs.log)
	if err != nil {
		return errors.Wrapf(err, "error creating uploader %s", uploaderType)
	}

	fs.initialized = true

	fs.log.WithFields(
		logrus.Fields{
			"jobName":          fs.jobName,
			"bsl":              bslName,
			"source namespace": sourceNamespace,
			"uploader":         uploaderType,
			"repository":       repositoryType,
		}).Info("FileSystemBR is initialized")

	return nil
}

func (fs *fileSystemBR) Close(ctx context.Context) {
	if fs.uploaderProv != nil {
		if err := fs.uploaderProv.Close(ctx); err != nil {
			fs.log.Errorf("failed to close uploader provider with error %v", err)
		}

		fs.uploaderProv = nil
	}

	if fs.cancel != nil {
		fs.cancel()
		fs.cancel = nil
	}

	fs.log.WithField("user", fs.jobName).Info("FileSystemBR is closed")
}

func (fs *fileSystemBR) StartBackup(source AccessPoint, realSource string, parentSnapshot string, forceFull bool, tags map[string]string) error {
	if !fs.initialized {
		return errors.New("file system data path is not initialized")
	}

	go func() {
		snapshotID, emptySnapshot, err := fs.uploaderProv.RunBackup(fs.ctx, source.ByPath, realSource, tags, forceFull, parentSnapshot, fs)

		if err == provider.ErrorCanceled {
			fs.callbacks.OnCancelled(context.Background(), fs.namespace, fs.jobName)
		} else if err != nil {
			fs.callbacks.OnFailed(context.Background(), fs.namespace, fs.jobName, err)
		} else {
			fs.callbacks.OnCompleted(context.Background(), fs.namespace, fs.jobName, Result{Backup: BackupResult{snapshotID, emptySnapshot, source}})
		}
	}()

	return nil
}

func (fs *fileSystemBR) StartRestore(snapshotID string, target AccessPoint) error {
	if !fs.initialized {
		return errors.New("file system data path is not initialized")
	}

	go func() {
		err := fs.uploaderProv.RunRestore(fs.ctx, snapshotID, target.ByPath, fs)

		if err == provider.ErrorCanceled {
			fs.callbacks.OnCancelled(context.Background(), fs.namespace, fs.jobName)
		} else if err != nil {
			fs.callbacks.OnFailed(context.Background(), fs.namespace, fs.jobName, err)
		} else {
			fs.callbacks.OnCompleted(context.Background(), fs.namespace, fs.jobName, Result{Restore: RestoreResult{Target: target}})
		}
	}()

	return nil
}

// UpdateProgress which implement ProgressUpdater interface to update progress status
func (fs *fileSystemBR) UpdateProgress(p *uploader.Progress) {
	if fs.callbacks.OnProgress != nil {
		fs.callbacks.OnProgress(context.Background(), fs.namespace, fs.jobName, &uploader.Progress{TotalBytes: p.TotalBytes, BytesDone: p.BytesDone})
	}
}

func (fs *fileSystemBR) Cancel() {
	fs.cancel()
	fs.log.WithField("user", fs.jobName).Info("FileSystemBR is canceled")
}

func (fs *fileSystemBR) boostRepoConnect(ctx context.Context, repositoryType string, credentialGetter *credentials.CredentialGetter) error {
	if repositoryType == velerov1api.BackupRepositoryTypeKopia {
		if err := repoProvider.NewUnifiedRepoProvider(*credentialGetter, repositoryType, fs.log).BoostRepoConnect(ctx, repoProvider.RepoParam{BackupLocation: fs.backupLocation, BackupRepo: fs.backupRepo}); err != nil {
			return err
		}
	} else {
		if err := repoProvider.NewResticRepositoryProvider(credentialGetter.FromFile, filesystem.NewFileSystem(), fs.log).BoostRepoConnect(ctx, repoProvider.RepoParam{BackupLocation: fs.backupLocation, BackupRepo: fs.backupRepo}); err != nil {
			return err
		}
	}

	return nil
}
