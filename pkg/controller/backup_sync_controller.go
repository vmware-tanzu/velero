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

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"

	kuberrs "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/util/wait"

	"github.com/heptio/ark/pkg/cloudprovider"
	arkv1client "github.com/heptio/ark/pkg/generated/clientset/versioned/typed/ark/v1"
	"github.com/heptio/ark/pkg/util/kube"
)

type backupSyncController struct {
	client        arkv1client.BackupsGetter
	backupService cloudprovider.BackupService
	bucket        string
	syncPeriod    time.Duration
	logger        *logrus.Logger
}

func NewBackupSyncController(
	client arkv1client.BackupsGetter,
	backupService cloudprovider.BackupService,
	bucket string,
	syncPeriod time.Duration,
	logger *logrus.Logger,
) Interface {
	if syncPeriod < time.Minute {
		logger.Infof("Provided backup sync period %v is too short. Setting to 1 minute", syncPeriod)
		syncPeriod = time.Minute
	}
	return &backupSyncController{
		client:        client,
		backupService: backupService,
		bucket:        bucket,
		syncPeriod:    syncPeriod,
		logger:        logger,
	}
}

// Run is a blocking function that continually runs the object storage -> Ark API
// sync process according to the controller's syncPeriod. It will return when it
// receives on the ctx.Done() channel.
func (c *backupSyncController) Run(ctx context.Context, workers int) error {
	c.logger.Info("Running backup sync controller")
	wait.Until(c.run, c.syncPeriod, ctx.Done())
	return nil
}

func (c *backupSyncController) run() {
	c.logger.Info("Syncing backups from object storage")
	backups, err := c.backupService.GetAllBackups(c.bucket)
	if err != nil {
		c.logger.WithError(err).Error("error listing backups")
		return
	}
	c.logger.WithField("backupCount", len(backups)).Info("Got backups from object storage")

	for _, cloudBackup := range backups {
		logContext := c.logger.WithField("backup", kube.NamespaceAndName(cloudBackup))
		logContext.Info("Syncing backup")

		cloudBackup.ResourceVersion = ""
		if _, err := c.client.Backups(cloudBackup.Namespace).Create(cloudBackup); err != nil && !kuberrs.IsAlreadyExists(err) {
			logContext.WithError(errors.WithStack(err)).Error("Error syncing backup from object storage")
		}
	}
}
