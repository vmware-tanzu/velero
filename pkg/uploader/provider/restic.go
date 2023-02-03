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

package provider

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	v1 "k8s.io/api/core/v1"

	"github.com/vmware-tanzu/velero/internal/credentials"
	velerov1api "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	"github.com/vmware-tanzu/velero/pkg/restic"
	"github.com/vmware-tanzu/velero/pkg/uploader"
	"github.com/vmware-tanzu/velero/pkg/util/filesystem"
)

// mainly used to make testing more convenient
var ResticBackupCMDFunc = restic.BackupCommand
var ResticRestoreCMDFunc = restic.RestoreCommand

type resticProvider struct {
	repoIdentifier  string
	credentialsFile string
	caCertFile      string
	cmdEnv          []string
	extraFlags      []string
	bsl             *velerov1api.BackupStorageLocation
	log             logrus.FieldLogger
}

func NewResticUploaderProvider(
	repoIdentifier string,
	bsl *velerov1api.BackupStorageLocation,
	credGetter *credentials.CredentialGetter,
	repoKeySelector *v1.SecretKeySelector,
	log logrus.FieldLogger,
) (Provider, error) {
	provider := resticProvider{
		repoIdentifier: repoIdentifier,
		bsl:            bsl,
		log:            log,
	}

	var err error
	provider.credentialsFile, err = credGetter.FromFile.Path(repoKeySelector)
	if err != nil {
		return nil, errors.Wrap(err, "error creating temp restic credentials file")
	}

	// if there's a caCert on the ObjectStorage, write it to disk so that it can be passed to restic
	if bsl.Spec.ObjectStorage != nil && bsl.Spec.ObjectStorage.CACert != nil {
		provider.caCertFile, err = restic.TempCACertFile(bsl.Spec.ObjectStorage.CACert, bsl.Name, filesystem.NewFileSystem())
		if err != nil {
			return nil, errors.Wrap(err, "error create temp cert file")
		}
	}

	provider.cmdEnv, err = restic.CmdEnv(bsl, credGetter.FromFile)
	if err != nil {
		return nil, errors.Wrap(err, "error generating repository cmnd env")
	}

	// #4820: restrieve insecureSkipTLSVerify from BSL configuration for
	// AWS plugin. If nothing is return, that means insecureSkipTLSVerify
	// is not enable for Restic command.
	skipTLSRet := restic.GetInsecureSkipTLSVerifyFromBSL(bsl, log)
	if len(skipTLSRet) > 0 {
		provider.extraFlags = append(provider.extraFlags, skipTLSRet)
	}

	return &provider, nil
}

func (rp *resticProvider) Close(ctx context.Context) error {
	_, err := os.Stat(rp.credentialsFile)
	if err == nil {
		return os.Remove(rp.credentialsFile)
	} else if !os.IsNotExist(err) {
		return errors.Errorf("failed to get file %s info with error %v", rp.credentialsFile, err)
	}

	_, err = os.Stat(rp.caCertFile)
	if err == nil {
		return os.Remove(rp.caCertFile)
	} else if !os.IsNotExist(err) {
		return errors.Errorf("failed to get file %s info with error %v", rp.caCertFile, err)
	}
	return nil
}

// RunBackup runs a `backup` command and watches the output to provide
// progress updates to the caller and return snapshotID, isEmptySnapshot, error
func (rp *resticProvider) RunBackup(
	ctx context.Context,
	path string,
	tags map[string]string,
	parentSnapshot string,
	updater uploader.ProgressUpdater) (string, bool, error) {
	if updater == nil {
		return "", false, errors.New("Need to initial backup progress updater first")
	}

	log := rp.log.WithFields(logrus.Fields{
		"path":           path,
		"parentSnapshot": parentSnapshot,
	})

	backupCmd := ResticBackupCMDFunc(rp.repoIdentifier, rp.credentialsFile, path, tags)
	backupCmd.Env = rp.cmdEnv
	backupCmd.CACertFile = rp.caCertFile
	if len(rp.extraFlags) != 0 {
		backupCmd.ExtraFlags = append(backupCmd.ExtraFlags, rp.extraFlags...)
	}

	if parentSnapshot != "" {
		backupCmd.ExtraFlags = append(backupCmd.ExtraFlags, fmt.Sprintf("--parent=%s", parentSnapshot))
	}

	summary, stderrBuf, err := restic.RunBackup(backupCmd, log, updater)
	if err != nil {
		if strings.Contains(stderrBuf, "snapshot is empty") {
			log.Debugf("Restic backup got empty dir with %s path", path)
			return "", true, nil
		}
		return "", false, errors.WithStack(fmt.Errorf("error running restic backup command %s with error: %v stderr: %v", backupCmd.String(), err, stderrBuf))
	}
	// GetSnapshotID
	snapshotIdCmd := restic.GetSnapshotCommand(rp.repoIdentifier, rp.credentialsFile, tags)
	snapshotIdCmd.Env = rp.cmdEnv
	snapshotIdCmd.CACertFile = rp.caCertFile
	if len(rp.extraFlags) != 0 {
		snapshotIdCmd.ExtraFlags = append(snapshotIdCmd.ExtraFlags, rp.extraFlags...)
	}
	snapshotID, err := restic.GetSnapshotID(snapshotIdCmd)
	if err != nil {
		return "", false, errors.WithStack(fmt.Errorf("error getting snapshot id with error: %v", err))
	}
	log.Infof("Run command=%s, stdout=%s, stderr=%s", backupCmd.String(), summary, stderrBuf)
	return snapshotID, false, nil
}

// RunRestore runs a `restore` command and monitors the volume size to
// provide progress updates to the caller.
func (rp *resticProvider) RunRestore(
	ctx context.Context,
	snapshotID string,
	volumePath string,
	updater uploader.ProgressUpdater) error {
	if updater == nil {
		return errors.New("Need to initial backup progress updater first")
	}
	log := rp.log.WithFields(logrus.Fields{
		"snapshotID": snapshotID,
		"volumePath": volumePath,
	})

	restoreCmd := ResticRestoreCMDFunc(rp.repoIdentifier, rp.credentialsFile, snapshotID, volumePath)
	restoreCmd.Env = rp.cmdEnv
	restoreCmd.CACertFile = rp.caCertFile
	if len(rp.extraFlags) != 0 {
		restoreCmd.ExtraFlags = append(restoreCmd.ExtraFlags, rp.extraFlags...)
	}
	stdout, stderr, err := restic.RunRestore(restoreCmd, log, updater)

	log.Infof("Run command=%s, stdout=%s, stderr=%s", restoreCmd.Command, stdout, stderr)
	return err
}
