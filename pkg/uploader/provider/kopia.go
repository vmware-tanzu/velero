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
	"time"

	"github.com/kopia/kopia/snapshot/snapshotfs"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"

	"github.com/vmware-tanzu/velero/pkg/uploader/kopia"

	"github.com/vmware-tanzu/velero/internal/credentials"
	velerov1api "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	repokeys "github.com/vmware-tanzu/velero/pkg/repository/keys"
	"github.com/vmware-tanzu/velero/pkg/repository/udmrepo"
	"github.com/vmware-tanzu/velero/pkg/repository/udmrepo/service"
)

//BackupFunc mainly used to make testing more convenient
var BackupFunc = kopia.Backup
var RestoreFunc = kopia.Restore

//kopiaProvider recorded info related with kopiaProvider
//action which means provider handle backup or restore
type kopiaProvider struct {
	bkRepo        udmrepo.BackupRepo
	credGetter    *credentials.CredentialGetter
	uploader      *snapshotfs.Uploader
	restoreCancel chan struct{}
	log           logrus.FieldLogger
}

//NewKopiaUploaderProvider initialized with open or create a repository
func NewKopiaUploaderProvider(
	ctx context.Context,
	credGetter *credentials.CredentialGetter,
	bsl *velerov1api.BackupStorageLocation,
	log logrus.FieldLogger,
) (Provider, error) {
	kp := &kopiaProvider{
		log:        log,
		credGetter: credGetter,
	}
	//repoUID which is used to generate kopia repository config with unique directory path
	repoUID := string(bsl.GetUID())
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

//CheckContext check context status periodically
//check if context is timeout or cancel
func (kp *kopiaProvider) CheckContext(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			if kp.uploader != nil {
				kp.uploader.Cancel()
				kp.log.Infof("Backup is been canceled")
			}
			if kp.restoreCancel != nil {
				close(kp.restoreCancel)
				kp.log.Infof("Restore is been canceled")
			}
			return
		default:
			time.Sleep(time.Second * 10)
		}
	}
}

func (kp *kopiaProvider) Close(ctx context.Context) {
	kp.bkRepo.Close(ctx)
}

//RunBackup which will backup specific path and update backup progress in pvb status
func (kp *kopiaProvider) RunBackup(
	ctx context.Context,
	path string,
	tags map[string]string,
	parentSnapshot string,
	updater velerov1api.ProgressUpdater) (string, error) {
	if updater == nil {
		return "", errors.New("Need to inital backup progress updater first")
	}

	log := kp.log.WithFields(logrus.Fields{
		"path":           path,
		"parentSnapshot": parentSnapshot,
	})
	repoWriter := kopia.NewShimRepo(kp.bkRepo)
	kp.uploader = snapshotfs.NewUploader(repoWriter)
	prorgess := new(kopia.KopiaProgress)
	prorgess.InitThrottle(backupProgressCheckInterval)
	prorgess.Updater = updater
	kp.uploader.Progress = prorgess

	log.Info("Starting backup")
	go kp.CheckContext(ctx)

	snapshotInfo, err := BackupFunc(ctx, kp.uploader, repoWriter, path, parentSnapshot, log)

	if err != nil {
		return "", errors.Wrapf(err, "Failed to run kopia backup")
	} else if snapshotInfo == nil {
		return "", fmt.Errorf("failed to get kopia backup snapshot info for path %v", path)
	}

	updater.UpdateProgress(
		&velerov1api.UploaderProgress{
			TotalBytes: snapshotInfo.Size,
			BytesDone:  snapshotInfo.Size,
		},
	)

	log.Debugf("Kopia backup finished, snapshot ID %s, backup size %d", snapshotInfo.ID, snapshotInfo.Size)
	return snapshotInfo.ID, nil
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

//RunRestore which will restore specific path and update restore progress in pvr status
func (kp *kopiaProvider) RunRestore(
	ctx context.Context,
	snapshotID string,
	volumePath string,
	updater velerov1api.ProgressUpdater) error {
	log := kp.log.WithFields(logrus.Fields{
		"snapshotID": snapshotID,
		"volumePath": volumePath,
	})
	repoWriter := kopia.NewShimRepo(kp.bkRepo)
	prorgess := new(kopia.KopiaProgress)
	prorgess.InitThrottle(restoreProgressCheckInterval)
	prorgess.Updater = updater
	kp.restoreCancel = make(chan struct{})
	defer func() {
		if kp.restoreCancel != nil {
			close(kp.restoreCancel)
		}
	}()

	log.Info("Starting restore")
	go kp.CheckContext(ctx)
	size, fileCount, err := RestoreFunc(ctx, repoWriter, prorgess, snapshotID, volumePath, log, kp.restoreCancel)

	if err != nil {
		return errors.Wrapf(err, "Failed to run kopia restore")
	}

	updater.UpdateProgress(&velerov1api.UploaderProgress{
		TotalBytes: size,
		BytesDone:  size,
	})

	output := fmt.Sprintf("Kopia restore finished, restore size %d, file count %d", size, fileCount)

	log.Info(output)

	return nil
}
