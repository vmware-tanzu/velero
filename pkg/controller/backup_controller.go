/*
Copyright 2017, 2019 the Velero contributors.

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
	"time"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/tools/cache"

	velerov1api "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	"github.com/vmware-tanzu/velero/pkg/backup"
	velerov1client "github.com/vmware-tanzu/velero/pkg/generated/clientset/versioned/typed/velero/v1"
	velerov1informers "github.com/vmware-tanzu/velero/pkg/generated/informers/externalversions/velero/v1"
	velerov1listers "github.com/vmware-tanzu/velero/pkg/generated/listers/velero/v1"
	"github.com/vmware-tanzu/velero/pkg/metrics"
	kubeutil "github.com/vmware-tanzu/velero/pkg/util/kube"
)

type backupController struct {
	*genericController

	backupProcessor *backup.Processor
	lister          velerov1listers.BackupLister
	client          velerov1client.BackupsGetter
	backupTracker   backup.Tracker
	metrics         *metrics.ServerMetrics
}

func NewBackupController(
	backupProcessor *backup.Processor,
	backupInformer velerov1informers.BackupInformer,
	logger logrus.FieldLogger,
	metrics *metrics.ServerMetrics,
) Interface {
	c := &backupController{
		genericController: newGenericController("backup", logger),
		backupProcessor:   backupProcessor,
		lister:            backupInformer.Lister(),
		metrics:           metrics,
	}

	c.syncHandler = c.processBackup
	c.resyncFunc = c.resync
	c.resyncPeriod = time.Minute

	backupInformer.Informer().AddEventHandler(
		cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {
				backup := obj.(*velerov1api.Backup)

				switch backup.Status.Phase {
				case "", velerov1api.BackupPhaseNew:
					// only process new backups
				default:
					c.logger.WithFields(logrus.Fields{
						"backup": kubeutil.NamespaceAndName(backup),
						"phase":  backup.Status.Phase,
					}).Debug("Backup is not new, skipping")
					return
				}

				key, err := cache.MetaNamespaceKeyFunc(backup)
				if err != nil {
					c.logger.WithError(err).WithField("backup", backup).Error("Error creating queue key, item not added to queue")
					return
				}
				c.queue.Add(key)
			},
		},
	)

	return c
}

func (c *backupController) processBackup(key string) error {
	log := c.logger.WithField("key", key)

	log.Debug("Running processBackup")
	ns, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		log.WithError(err).Errorf("error splitting key")
		return nil
	}

	log.Debug("Getting backup")
	original, err := c.lister.Backups(ns).Get(name)
	if apierrors.IsNotFound(err) {
		log.Debugf("Backup %s not found", name)
		return nil
	}
	if err != nil {
		return errors.Wrap(err, "error getting backup")
	}

	return c.backupProcessor.Process(original.DeepCopy())
}

func (c *backupController) resync() {
	// recompute backup_total metric
	backups, err := c.lister.List(labels.Everything())
	if err != nil {
		c.logger.Error(err, "Error computing backup_total metric")
	} else {
		c.metrics.SetBackupTotal(int64(len(backups)))
	}

	// recompute backup_last_successful_timestamp metric for each
	// schedule (including the empty schedule, i.e. ad-hoc backups)
	for schedule, timestamp := range getLastSuccessBySchedule(backups) {
		c.metrics.SetBackupLastSuccessfulTimestamp(schedule, timestamp)
	}
}

// getLastSuccessBySchedule finds the most recent completed backup for each schedule
// and returns a map of schedule name -> completion time of the most recent completed
// backup. This map includes an entry for ad-hoc/non-scheduled backups, where the key
// is the empty string.
func getLastSuccessBySchedule(backups []*velerov1api.Backup) map[string]time.Time {
	lastSuccessBySchedule := map[string]time.Time{}
	for _, backup := range backups {
		if backup.Status.Phase != velerov1api.BackupPhaseCompleted {
			continue
		}
		if backup.Status.CompletionTimestamp == nil {
			continue
		}

		schedule := backup.Labels[velerov1api.ScheduleNameLabel]
		timestamp := backup.Status.CompletionTimestamp.Time

		if timestamp.After(lastSuccessBySchedule[schedule]) {
			lastSuccessBySchedule[schedule] = timestamp
		}
	}

	return lastSuccessBySchedule
}
