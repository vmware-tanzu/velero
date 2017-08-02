/*
Copyright 2017 Heptio Inc.

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

package controller

import (
	"context"
	"time"

	"github.com/golang/glog"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/util/wait"

	"github.com/heptio/ark/pkg/cloudprovider"
	arkv1client "github.com/heptio/ark/pkg/generated/clientset/typed/ark/v1"
)

type backupSyncController struct {
	client        arkv1client.BackupsGetter
	backupService cloudprovider.BackupService
	bucket        string
	syncPeriod    time.Duration
}

func NewBackupSyncController(client arkv1client.BackupsGetter, backupService cloudprovider.BackupService, bucket string, syncPeriod time.Duration) Interface {
	if syncPeriod < time.Minute {
		glog.Infof("Backup sync period %v is too short. Setting to 1 minute", syncPeriod)
		syncPeriod = time.Minute
	}
	return &backupSyncController{
		client:        client,
		backupService: backupService,
		bucket:        bucket,
		syncPeriod:    syncPeriod,
	}
}

// Run is a blocking function that continually runs the object storage -> Ark API
// sync process according to the controller's syncPeriod. It will return when it
// receives on the ctx.Done() channel.
func (c *backupSyncController) Run(ctx context.Context, workers int) error {
	glog.Info("Running backup sync controller")
	wait.Until(c.run, c.syncPeriod, ctx.Done())
	return nil
}

func (c *backupSyncController) run() {
	glog.Info("Syncing backups from object storage")
	backups, err := c.backupService.GetAllBackups(c.bucket)
	if err != nil {
		glog.Errorf("error listing backups: %v", err)
		return
	}
	glog.Infof("Found %d backups", len(backups))

	for _, cloudBackup := range backups {
		glog.Infof("Syncing backup %s/%s", cloudBackup.Namespace, cloudBackup.Name)
		cloudBackup.ResourceVersion = ""
		if _, err := c.client.Backups(cloudBackup.Namespace).Create(cloudBackup); err != nil && !errors.IsAlreadyExists(err) {
			glog.Errorf("error syncing backup %s/%s from object storage: %v", cloudBackup.Namespace, cloudBackup.Name, err)
		}
	}
}
