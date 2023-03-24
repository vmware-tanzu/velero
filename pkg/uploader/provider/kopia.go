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
	"strings"

	"github.com/kopia/kopia/snapshot/snapshotfs"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"

	"github.com/vmware-tanzu/velero/pkg/uploader"
	"github.com/vmware-tanzu/velero/pkg/uploader/kopia"

	"github.com/vmware-tanzu/velero/internal/credentials"
	velerov1api "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	repokeys "github.com/vmware-tanzu/velero/pkg/repository/keys"
	"github.com/vmware-tanzu/velero/pkg/repository/udmrepo"
	"github.com/vmware-tanzu/velero/pkg/repository/udmrepo/service"
)

// BackupFunc mainly used to make testing more convenient
var BackupFunc = kopia.Backup
var RestoreFunc = kopia.Restore

// kopiaProvider recorded info related with kopiaProvider
type kopiaProvider struct {
	bkRepo     udmrepo.BackupRepo
	credGetter *credentials.CredentialGetter
	log        logrus.FieldLogger
}

// NewKopiaUploaderProvider initialized with open or create a repository
func NewKopiaUploaderProvider(
	ctx context.Context,
	credGetter *credentials.CredentialGetter,
	backupRepo *velerov1api.BackupRepository,
	log logrus.FieldLogger,
) (Provider, error) {
	kp := &kopiaProvider{
		log:        log,
		credGetter: credGetter,
	}
	//repoUID which is used to generate kopia repository config with unique directory path
	repoUID := string(backupRepo.GetUID())
	repoOpt, err := udmrepo.NewRepoOptions(
		udmrepo.WithPassword(kp, ""),
		udmrepo.WithConfigFile("", repoUID),
		udmrepo.WithDescription("Initial kopia uploader provider"),
	)
	if err != nil {
		return nil, errors.Wrapf(err, "error to get repo options")
	}

	repoSvc := service.Create(log)
	log.WithField("repoUID", repoUID).Info("Opening backup repo")

	kp.bkRepo, err = repoSvc.Open(ctx, *repoOpt)
	if err != nil {
		return nil, errors.Wrapf(err, "Failed to find kopia repository")
	}
	return kp, nil
}

// CheckContext check context status check if context is timeout or cancel and backup restore once finished it will quit and return
func (kp *kopiaProvider) CheckContext(ctx context.Context, finishChan chan struct{}, restoreChan chan struct{}, uploader *snapshotfs.Uploader) {
	select {
	case <-finishChan:
		kp.log.Infof("Action finished")
		return
	case <-ctx.Done():
		if uploader != nil {
			uploader.Cancel()
			kp.log.Infof("Backup is been canceled")
		}
		if restoreChan != nil {
			close(restoreChan)
			kp.log.Infof("Restore is been canceled")
		}
		return
	}
}

func (kp *kopiaProvider) Close(ctx context.Context) error {
	return kp.bkRepo.Close(ctx)
}

// RunBackup which will backup specific path and update backup progress
// return snapshotID, isEmptySnapshot, error
func (kp *kopiaProvider) RunBackup(
	ctx context.Context,
	path string,
	tags map[string]string,
	parentSnapshot string,
	updater uploader.ProgressUpdater) (string, bool, error) {
	if updater == nil {
		return "", false, errors.New("Need to initial backup progress updater first")
	}

	log := kp.log.WithFields(logrus.Fields{
		"path":           path,
		"parentSnapshot": parentSnapshot,
	})
	repoWriter := kopia.NewShimRepo(kp.bkRepo)
	kpUploader := snapshotfs.NewUploader(repoWriter)
	progress := new(kopia.KopiaProgress)
	progress.InitThrottle(backupProgressCheckInterval)
	progress.Updater = updater
	progress.Log = log
	kpUploader.Progress = progress
	quit := make(chan struct{})
	log.Info("Starting backup")
	go kp.CheckContext(ctx, quit, nil, kpUploader)

	defer func() {
		close(quit)
	}()

	snapshotInfo, isSnapshotEmpty, err := BackupFunc(ctx, kpUploader, repoWriter, path, parentSnapshot, log)
	if err != nil {
		return "", false, errors.Wrapf(err, "Failed to run kopia backup")
	} else if isSnapshotEmpty {
		log.Debugf("Kopia backup got empty dir with path %s", path)
		return "", true, nil
	} else if snapshotInfo == nil {
		return "", false, fmt.Errorf("failed to get kopia backup snapshot info for path %v", path)
	}

	// which ensure that the statistic data of TotalBytes equal to BytesDone when finished
	updater.UpdateProgress(
		&uploader.UploaderProgress{
			TotalBytes: snapshotInfo.Size,
			BytesDone:  snapshotInfo.Size,
		},
	)

	log.Debugf("Kopia backup finished, snapshot ID %s, backup size %d", snapshotInfo.ID, snapshotInfo.Size)
	return snapshotInfo.ID, false, nil
}

func (kp *kopiaProvider) GetPassword(param interface{}) (string, error) {
	if kp.credGetter.FromSecret == nil {
		return "", errors.New("invalid credentials interface")
	}
	rawPass, err := kp.credGetter.FromSecret.Get(repokeys.RepoKeySelector())
	if err != nil {
		return "", errors.Wrap(err, "error to get password")
	}

	return strings.TrimSpace(rawPass), nil
}

// RunRestore which will restore specific path and update restore progress
func (kp *kopiaProvider) RunRestore(
	ctx context.Context,
	snapshotID string,
	volumePath string,
	updater uploader.ProgressUpdater) error {
	log := kp.log.WithFields(logrus.Fields{
		"snapshotID": snapshotID,
		"volumePath": volumePath,
	})
	repoWriter := kopia.NewShimRepo(kp.bkRepo)
	prorgess := new(kopia.KopiaProgress)
	prorgess.InitThrottle(restoreProgressCheckInterval)
	prorgess.Updater = updater
	restoreCancel := make(chan struct{})
	quit := make(chan struct{})

	log.Info("Starting restore")
	go kp.CheckContext(ctx, quit, restoreCancel, nil)

	defer func() {
		if restoreCancel != nil {
			close(restoreCancel)
		}
		close(quit)
	}()

	size, fileCount, err := RestoreFunc(ctx, repoWriter, prorgess, snapshotID, volumePath, log, restoreCancel)

	if err != nil {
		return errors.Wrapf(err, "Failed to run kopia restore")
	}

	// which ensure that the statistic data of TotalBytes equal to BytesDone when finished
	updater.UpdateProgress(&uploader.UploaderProgress{
		TotalBytes: size,
		BytesDone:  size,
	})

	output := fmt.Sprintf("Kopia restore finished, restore size %d, file count %d", size, fileCount)

	log.Info(output)

	return nil
}
