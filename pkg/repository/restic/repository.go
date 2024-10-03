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

package restic

import (
	"os"
	"time"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"

	"github.com/vmware-tanzu/velero/internal/credentials"
	velerov1api "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	repokey "github.com/vmware-tanzu/velero/pkg/repository/keys"
	"github.com/vmware-tanzu/velero/pkg/restic"
	veleroexec "github.com/vmware-tanzu/velero/pkg/util/exec"
	"github.com/vmware-tanzu/velero/pkg/util/filesystem"
)

func NewRepositoryService(store credentials.FileStore, fs filesystem.Interface, log logrus.FieldLogger) *RepositoryService {
	return &RepositoryService{
		credentialsFileStore: store,
		fileSystem:           fs,
		log:                  log,
	}
}

type RepositoryService struct {
	credentialsFileStore credentials.FileStore
	fileSystem           filesystem.Interface
	log                  logrus.FieldLogger
}

func (r *RepositoryService) InitRepo(bsl *velerov1api.BackupStorageLocation, repo *velerov1api.BackupRepository) error {
	return r.exec(restic.InitCommand(repo.Spec.ResticIdentifier), bsl)
}

func (r *RepositoryService) ConnectToRepo(bsl *velerov1api.BackupStorageLocation, repo *velerov1api.BackupRepository) error {
	snapshotsCmd := restic.SnapshotsCommand(repo.Spec.ResticIdentifier)
	// use the '--latest=1' flag to minimize the amount of data fetched since
	// we're just validating that the repo exists and can be authenticated
	// to.
	// "--last" is replaced by "--latest=1" in restic v0.12.1
	snapshotsCmd.ExtraFlags = append(snapshotsCmd.ExtraFlags, "--latest=1")

	return r.exec(snapshotsCmd, bsl)
}

func (r *RepositoryService) PruneRepo(bsl *velerov1api.BackupStorageLocation, repo *velerov1api.BackupRepository) error {
	return r.exec(restic.PruneCommand(repo.Spec.ResticIdentifier), bsl)
}

func (r *RepositoryService) UnlockRepo(bsl *velerov1api.BackupStorageLocation, repo *velerov1api.BackupRepository) error {
	return r.exec(restic.UnlockCommand(repo.Spec.ResticIdentifier), bsl)
}

func (r *RepositoryService) Forget(bsl *velerov1api.BackupStorageLocation, repo *velerov1api.BackupRepository, snapshotID string) error {
	return r.exec(restic.ForgetCommand(repo.Spec.ResticIdentifier, snapshotID), bsl)
}

func (r *RepositoryService) DefaultMaintenanceFrequency() time.Duration {
	return restic.DefaultMaintenanceFrequency
}

func (r *RepositoryService) exec(cmd *restic.Command, bsl *velerov1api.BackupStorageLocation) error {
	file, err := r.credentialsFileStore.Path(repokey.RepoKeySelector())
	if err != nil {
		return err
	}
	// ignore error since there's nothing we can do and it's a temp file.
	defer os.Remove(file)

	cmd.PasswordFile = file

	// if there's a caCert on the ObjectStorage, write it to disk so that it can be passed to restic
	var caCertFile string
	if bsl.Spec.ObjectStorage != nil && bsl.Spec.ObjectStorage.CACert != nil {
		caCertFile, err = restic.TempCACertFile(bsl.Spec.ObjectStorage.CACert, bsl.Name, r.fileSystem)
		if err != nil {
			return errors.Wrap(err, "error creating temp cacert file")
		}
		// ignore error since there's nothing we can do and it's a temp file.
		defer os.Remove(caCertFile)
	}
	cmd.CACertFile = caCertFile

	env, err := restic.CmdEnv(bsl, r.credentialsFileStore)
	if err != nil {
		return err
	}
	cmd.Env = env

	// #4820: restrieve insecureSkipTLSVerify from BSL configuration for
	// AWS plugin. If nothing is return, that means insecureSkipTLSVerify
	// is not enable for Restic command.
	skipTLSRet := restic.GetInsecureSkipTLSVerifyFromBSL(bsl, r.log)
	if len(skipTLSRet) > 0 {
		cmd.ExtraFlags = append(cmd.ExtraFlags, skipTLSRet)
	}

	stdout, stderr, err := veleroexec.RunCommandWithLog(cmd.Cmd(), r.log)
	r.log.WithFields(logrus.Fields{
		"repository": cmd.RepoName(),
		"command":    cmd.String(),
		"stdout":     stdout,
		"stderr":     stderr,
	}).Debugf("Ran restic command")
	if err != nil {
		return errors.Wrapf(err, "error running command=%s, stdout=%s, stderr=%s", cmd.String(), stdout, stderr)
	}

	return nil
}
