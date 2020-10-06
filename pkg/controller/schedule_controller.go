/*
Copyright 2017 the Velero contributors.

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
	"encoding/json"
	"fmt"
	"time"

	jsonpatch "github.com/evanphx/json-patch"
	"github.com/pkg/errors"
	"github.com/robfig/cron"
	"github.com/sirupsen/logrus"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/clock"
	"k8s.io/client-go/tools/cache"

	api "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	"github.com/vmware-tanzu/velero/pkg/builder"
	velerov1client "github.com/vmware-tanzu/velero/pkg/generated/clientset/versioned/typed/velero/v1"
	velerov1informers "github.com/vmware-tanzu/velero/pkg/generated/informers/externalversions/velero/v1"
	velerov1listers "github.com/vmware-tanzu/velero/pkg/generated/listers/velero/v1"
	"github.com/vmware-tanzu/velero/pkg/metrics"
	kubeutil "github.com/vmware-tanzu/velero/pkg/util/kube"
)

const (
	scheduleSyncPeriod = time.Minute
)

type scheduleController struct {
	*genericController

	namespace       string
	schedulesClient velerov1client.SchedulesGetter
	backupsClient   velerov1client.BackupsGetter
	schedulesLister velerov1listers.ScheduleLister
	clock           clock.Clock
	metrics         *metrics.ServerMetrics
}

func NewScheduleController(
	namespace string,
	schedulesClient velerov1client.SchedulesGetter,
	backupsClient velerov1client.BackupsGetter,
	schedulesInformer velerov1informers.ScheduleInformer,
	logger logrus.FieldLogger,
	metrics *metrics.ServerMetrics,
) *scheduleController {
	c := &scheduleController{
		genericController: newGenericController(Schedule, logger),
		namespace:         namespace,
		schedulesClient:   schedulesClient,
		backupsClient:     backupsClient,
		schedulesLister:   schedulesInformer.Lister(),
		clock:             clock.RealClock{},
		metrics:           metrics,
	}

	c.syncHandler = c.processSchedule
	c.resyncFunc = c.enqueueAllEnabledSchedules
	c.resyncPeriod = scheduleSyncPeriod

	schedulesInformer.Informer().AddEventHandler(
		cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {
				schedule := obj.(*api.Schedule)

				switch schedule.Status.Phase {
				case "", api.SchedulePhaseNew, api.SchedulePhaseEnabled:
					// add to work queue
				default:
					c.logger.WithFields(logrus.Fields{
						"schedule": kubeutil.NamespaceAndName(schedule),
						"phase":    schedule.Status.Phase,
					}).Debug("Schedule is not new, skipping")
					return
				}

				key, err := cache.MetaNamespaceKeyFunc(schedule)
				if err != nil {
					c.logger.WithError(errors.WithStack(err)).WithField("schedule", schedule).Error("Error creating queue key, item not added to queue")
					return
				}
				c.queue.Add(key)
				scheduleName := schedule.GetName()
				c.logger.Info("Creating schedule ", scheduleName)
				//Init Prometheus metrics to 0 to have them flowing up
				metrics.InitSchedule(scheduleName)
			},
		},
	)

	return c
}

func (c *scheduleController) enqueueAllEnabledSchedules() {
	schedules, err := c.schedulesLister.Schedules(c.namespace).List(labels.NewSelector())
	if err != nil {
		c.logger.WithError(errors.WithStack(err)).Error("Error listing Schedules")
		return
	}

	for _, schedule := range schedules {
		if schedule.Status.Phase != api.SchedulePhaseEnabled {
			continue
		}

		key, err := cache.MetaNamespaceKeyFunc(schedule)
		if err != nil {
			c.logger.WithError(errors.WithStack(err)).WithField("schedule", schedule).Error("Error creating queue key, item not added to queue")
			continue
		}
		c.queue.Add(key)
	}
}

func (c *scheduleController) processSchedule(key string) error {
	log := c.logger.WithField("key", key)

	log.Debug("Running processSchedule")
	ns, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		return errors.Wrap(err, "error splitting queue key")
	}

	log.Debug("Getting Schedule")
	schedule, err := c.schedulesLister.Schedules(ns).Get(name)
	if err != nil {
		// schedule no longer exists
		if apierrors.IsNotFound(err) {
			log.WithError(err).Debug("Schedule not found")
			return nil
		}
		return errors.Wrap(err, "error getting Schedule")
	}

	switch schedule.Status.Phase {
	case "", api.SchedulePhaseNew, api.SchedulePhaseEnabled:
		// valid phase for processing
	default:
		return nil
	}

	log.Debug("Cloning schedule")
	// store ref to original for creating patch
	original := schedule
	// don't modify items in the cache
	schedule = schedule.DeepCopy()

	// validation - even if the item is Enabled, we can't trust it
	// so re-validate
	currentPhase := schedule.Status.Phase

	cronSchedule, errs := parseCronSchedule(schedule, c.logger)
	if len(errs) > 0 {
		schedule.Status.Phase = api.SchedulePhaseFailedValidation
		schedule.Status.ValidationErrors = errs
	} else {
		schedule.Status.Phase = api.SchedulePhaseEnabled
	}

	// update status if it's changed
	if currentPhase != schedule.Status.Phase {
		updatedSchedule, err := patchSchedule(original, schedule, c.schedulesClient)
		if err != nil {
			return errors.Wrapf(err, "error updating Schedule phase to %s", schedule.Status.Phase)
		}
		schedule = updatedSchedule
	}

	if schedule.Status.Phase != api.SchedulePhaseEnabled {
		return nil
	}

	// check for the schedule being due to run, and submit a Backup if so
	if err := c.submitBackupIfDue(schedule, cronSchedule); err != nil {
		return err
	}

	return nil
}

func parseCronSchedule(itm *api.Schedule, logger logrus.FieldLogger) (cron.Schedule, []string) {
	var validationErrors []string
	var schedule cron.Schedule

	// cron.Parse panics if schedule is empty
	if len(itm.Spec.Schedule) == 0 {
		validationErrors = append(validationErrors, "Schedule must be a non-empty valid Cron expression")
		return nil, validationErrors
	}

	log := logger.WithField("schedule", kubeutil.NamespaceAndName(itm))

	// adding a recover() around cron.Parse because it panics on empty string and is possible
	// that it panics under other scenarios as well.
	func() {
		defer func() {
			if r := recover(); r != nil {
				log.WithFields(logrus.Fields{
					"schedule": itm.Spec.Schedule,
					"recover":  r,
				}).Debug("Panic parsing schedule")
				validationErrors = append(validationErrors, fmt.Sprintf("invalid schedule: %v", r))
			}
		}()

		if res, err := cron.ParseStandard(itm.Spec.Schedule); err != nil {
			log.WithError(errors.WithStack(err)).WithField("schedule", itm.Spec.Schedule).Debug("Error parsing schedule")
			validationErrors = append(validationErrors, fmt.Sprintf("invalid schedule: %v", err))
		} else {
			schedule = res
		}
	}()

	if len(validationErrors) > 0 {
		return nil, validationErrors
	}

	return schedule, nil
}

func (c *scheduleController) submitBackupIfDue(item *api.Schedule, cronSchedule cron.Schedule) error {
	var (
		now                = c.clock.Now()
		isDue, nextRunTime = getNextRunTime(item, cronSchedule, now)
		log                = c.logger.WithField("schedule", kubeutil.NamespaceAndName(item))
	)

	if !isDue {
		log.WithField("nextRunTime", nextRunTime).Debug("Schedule is not due, skipping")
		return nil
	}

	// Don't attempt to "catch up" if there are any missed or failed runs - simply
	// trigger a Backup if it's time.
	//
	// It might also make sense in the future to explicitly check for currently-running
	// backups so that we don't overlap runs (for disk snapshots in particular, this can
	// lead to performance issues).
	log.WithField("nextRunTime", nextRunTime).Info("Schedule is due, submitting Backup")
	backup := getBackup(item, now)
	if _, err := c.backupsClient.Backups(backup.Namespace).Create(context.TODO(), backup, metav1.CreateOptions{}); err != nil {
		return errors.Wrap(err, "error creating Backup")
	}

	original := item
	schedule := item.DeepCopy()

	schedule.Status.LastBackup = &metav1.Time{Time: now}

	if _, err := patchSchedule(original, schedule, c.schedulesClient); err != nil {
		return errors.Wrapf(err, "error updating Schedule's LastBackup time to %v", schedule.Status.LastBackup)
	}

	return nil
}

func getNextRunTime(schedule *api.Schedule, cronSchedule cron.Schedule, asOf time.Time) (bool, time.Time) {
	// get the latest run time (if the schedule hasn't run yet, this will be the zero value which will trigger
	// an immediate backup)
	var lastBackupTime time.Time
	if schedule.Status.LastBackup != nil {
		lastBackupTime = schedule.Status.LastBackup.Time
	}

	nextRunTime := cronSchedule.Next(lastBackupTime)

	return asOf.After(nextRunTime), nextRunTime
}

func getBackup(item *api.Schedule, timestamp time.Time) *api.Backup {
	name := item.TimestampedName(timestamp)
	backup := builder.
		ForBackup(item.Namespace, name).
		FromSchedule(item).
		Result()

	return backup
}

func patchSchedule(original, updated *api.Schedule, client velerov1client.SchedulesGetter) (*api.Schedule, error) {
	origBytes, err := json.Marshal(original)
	if err != nil {
		return nil, errors.Wrap(err, "error marshalling original schedule")
	}

	updatedBytes, err := json.Marshal(updated)
	if err != nil {
		return nil, errors.Wrap(err, "error marshalling updated schedule")
	}

	patchBytes, err := jsonpatch.CreateMergePatch(origBytes, updatedBytes)
	if err != nil {
		return nil, errors.Wrap(err, "error creating json merge patch for schedule")
	}

	res, err := client.Schedules(original.Namespace).Patch(context.TODO(), original.Name, types.MergePatchType, patchBytes, metav1.PatchOptions{})
	if err != nil {
		return nil, errors.Wrap(err, "error patching schedule")
	}

	return res, nil
}
