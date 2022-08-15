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
	"bytes"
	"context"
	"fmt"
	"os"
	"time"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	v1 "k8s.io/api/core/v1"

	"github.com/vmware-tanzu/velero/internal/credentials"
	velerov1api "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	"github.com/vmware-tanzu/velero/pkg/restic"
	"github.com/vmware-tanzu/velero/pkg/uploader"
	"github.com/vmware-tanzu/velero/pkg/util/exec"
	"github.com/vmware-tanzu/velero/pkg/util/filesystem"
)

// BackupFunc mainly used to make testing more convenient
var ResticBackupFunc = restic.BackupCommand
var ResticRunCMDFunc = exec.RunCommand
var ResticGetSnapshotSizeFunc = restic.GetSnapshotSize
var ResticGetVolumeSizeFunc = restic.GetVolumeSize
var ResticRestoreFunc = restic.RestoreCommand

type resticProvider struct {
	repoIdentifier  string
	credentialsFile string
	caCertFile      string
	repoEnv         []string
	backupTags      map[string]string
	log             logrus.FieldLogger
	bsl             *velerov1api.BackupStorageLocation
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
		log:            log,
		bsl:            bsl,
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

	provider.repoEnv, err = restic.CmdEnv(bsl, credGetter.FromFile)
	if err != nil {
		return nil, errors.Wrap(err, "error generating repository cmnd env")
	}

	return &provider, nil
}

// Not implement yet
func (rp *resticProvider) Cancel() {
}

func (rp *resticProvider) Close(ctx context.Context) error {
	if err := os.Remove(rp.credentialsFile); err != nil {
		return errors.Wrapf(err, "failed to remove file %s", rp.credentialsFile)
	}
	if err := os.Remove(rp.caCertFile); err != nil {
		return errors.Wrapf(err, "failed to remove file %s", rp.caCertFile)
	}
	return nil
}

// RunBackup runs a `backup` command and watches the output to provide
// progress updates to the caller.
func (rp *resticProvider) RunBackup(
	ctx context.Context,
	path string,
	tags map[string]string,
	parentSnapshot string,
	updater uploader.ProgressUpdater) (string, error) {
	if updater == nil {
		return "", errors.New("Need to initial backup progress updater first")
	}

	log := rp.log.WithFields(logrus.Fields{
		"path":           path,
		"parentSnapshot": parentSnapshot,
	})

	// buffers for copying command stdout/err output into
	stdoutBuf := new(bytes.Buffer)
	stderrBuf := new(bytes.Buffer)

	// create a channel to signal when to end the goroutine scanning for progress
	// updates
	quit := make(chan struct{})

	rp.backupTags = tags
	backupCmd := ResticBackupFunc(rp.repoIdentifier, rp.credentialsFile, path, tags)
	backupCmd.Env = rp.repoEnv
	backupCmd.CACertFile = rp.caCertFile

	if parentSnapshot != "" {
		backupCmd.ExtraFlags = append(backupCmd.ExtraFlags, fmt.Sprintf("--parent=%s", parentSnapshot))
	}

	// #4820: restrieve insecureSkipTLSVerify from BSL configuration for
	// AWS plugin. If nothing is return, that means insecureSkipTLSVerify
	// is not enable for Restic command.
	skipTLSRet := restic.GetInsecureSkipTLSVerifyFromBSL(rp.bsl, rp.log)
	if len(skipTLSRet) > 0 {
		backupCmd.ExtraFlags = append(backupCmd.ExtraFlags, skipTLSRet)
	}

	cmd := backupCmd.Cmd()
	cmd.Stdout = stdoutBuf
	cmd.Stderr = stderrBuf

	err := cmd.Start()
	if err != nil {
		log.Errorf("failed to execute %v with stderr %v error %v", cmd, stderrBuf.String(), err)
		return "", err
	}

	go func() {
		ticker := time.NewTicker(backupProgressCheckInterval)
		for {
			select {
			case <-ticker.C:
				lastLine := restic.GetLastLine(stdoutBuf.Bytes())
				if len(lastLine) > 0 {
					stat, err := restic.DecodeBackupStatusLine(lastLine)
					if err != nil {
						rp.log.WithError(err).Errorf("error getting backup progress")
					}

					// if the line contains a non-empty bytes_done field, we can update the
					// caller with the progress
					if stat.BytesDone != 0 {
						updater.UpdateProgress(&uploader.UploaderProgress{
							TotalBytes: stat.TotalBytesProcessed,
							BytesDone:  stat.TotalBytesProcessed,
						})
					}
				}
			case <-quit:
				ticker.Stop()
				return
			}
		}
	}()

	err = cmd.Wait()
	if err != nil {
		return "", errors.WithStack(fmt.Errorf("failed to wait execute %v with stderr %v error %v", cmd, stderrBuf.String(), err))
	}
	quit <- struct{}{}

	summary, err := restic.GetSummaryLine(stdoutBuf.Bytes())
	if err != nil {
		return "", errors.WithStack(fmt.Errorf("failed to get summary %v with error %v", stderrBuf.String(), err))
	}
	stat, err := restic.DecodeBackupStatusLine(summary)
	if err != nil {
		return "", errors.WithStack(fmt.Errorf("failed to decode summary %v with error %v", string(summary), err))
	}
	if stat.MessageType != "summary" {
		return "", errors.WithStack(fmt.Errorf("error getting backup summary: %s", string(summary)))
	}

	// update progress to 100%
	updater.UpdateProgress(&uploader.UploaderProgress{
		TotalBytes: stat.TotalBytesProcessed,
		BytesDone:  stat.TotalBytesProcessed,
	})

	//GetSnapshtotID
	snapshotIdCmd := restic.GetSnapshotCommand(rp.repoIdentifier, rp.credentialsFile, rp.backupTags)
	snapshotIdCmd.Env = rp.repoEnv
	snapshotIdCmd.CACertFile = rp.caCertFile

	snapshotID, err := restic.GetSnapshotID(snapshotIdCmd)
	if err != nil {
		return "", errors.WithStack(fmt.Errorf("error getting snapshot id with error: %s", err))
	}
	log.Debugf("Ran command=%s, stdout=%s, stderr=%s", cmd.String(), summary, stderrBuf.String())
	return snapshotID, nil
}

// RunRestore runs a `restore` command and monitors the volume size to
// provide progress updates to the caller.
func (rp *resticProvider) RunRestore(
	ctx context.Context,
	snapshotID string,
	volumePath string,
	updater uploader.ProgressUpdater) error {
	log := rp.log.WithFields(logrus.Fields{
		"snapshotID": snapshotID,
		"volumePath": volumePath,
	})

	restoreCmd := ResticRestoreFunc(rp.repoIdentifier, rp.credentialsFile, snapshotID, volumePath)
	restoreCmd.Env = rp.repoEnv
	restoreCmd.CACertFile = rp.caCertFile

	// #4820: restrieve insecureSkipTLSVerify from BSL configuration for
	// AWS plugin. If nothing is return, that means insecureSkipTLSVerify
	// is not enable for Restic command.
	skipTLSRet := restic.GetInsecureSkipTLSVerifyFromBSL(rp.bsl, log)
	if len(skipTLSRet) > 0 {
		restoreCmd.ExtraFlags = append(restoreCmd.ExtraFlags, skipTLSRet)
	}
	snapshotSize, err := ResticGetSnapshotSizeFunc(restoreCmd.RepoIdentifier, restoreCmd.PasswordFile, restoreCmd.CACertFile, restoreCmd.Args[0], restoreCmd.Env, skipTLSRet)
	if err != nil {
		return errors.Wrap(err, "error getting snapshot size")
	}

	updater.UpdateProgress(&uploader.UploaderProgress{
		TotalBytes: snapshotSize,
	})

	// create a channel to signal when to end the goroutine scanning for progress
	// updates
	quit := make(chan struct{})

	go func() {
		ticker := time.NewTicker(restoreProgressCheckInterval)
		for {
			select {
			case <-ticker.C:
				volumeSize, err := ResticGetVolumeSizeFunc(restoreCmd.Dir)
				if err != nil {
					log.WithError(err).Errorf("error getting volume size for restore dir %v", restoreCmd.Dir)
					return
				}
				if volumeSize != 0 {
					updater.UpdateProgress(&uploader.UploaderProgress{
						TotalBytes: snapshotSize,
						BytesDone:  volumeSize,
					})
				}
			case <-quit:
				ticker.Stop()
				return
			}
		}
	}()

	stdout, stderr, err := ResticRunCMDFunc(restoreCmd.Cmd())
	quit <- struct{}{}
	if err != nil {
		return errors.Wrap(err, fmt.Sprintf("failed to execute restore command %v with stderr %v", restoreCmd.Cmd().String(), stderr))
	}
	// update progress to 100%
	updater.UpdateProgress(&uploader.UploaderProgress{
		TotalBytes: snapshotSize,
		BytesDone:  snapshotSize,
	})
	log.Debugf("Ran command=%s, stdout=%s, stderr=%s", restoreCmd.Command, stdout, stderr)
	return err
}
