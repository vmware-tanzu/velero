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
	"fmt"
	"sync"
	"time"

	"github.com/pkg/errors"
	"github.com/robfig/cron"
	"github.com/sirupsen/logrus"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/clock"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"

	api "github.com/heptio/ark/pkg/apis/ark/v1"
	"github.com/heptio/ark/pkg/generated/clientset/scheme"
	arkv1client "github.com/heptio/ark/pkg/generated/clientset/typed/ark/v1"
	informers "github.com/heptio/ark/pkg/generated/informers/externalversions/ark/v1"
	listers "github.com/heptio/ark/pkg/generated/listers/ark/v1"
	kubeutil "github.com/heptio/ark/pkg/util/kube"
)

type scheduleController struct {
	schedulesClient       arkv1client.SchedulesGetter
	backupsClient         arkv1client.BackupsGetter
	schedulesLister       listers.ScheduleLister
	schedulesListerSynced cache.InformerSynced
	syncHandler           func(scheduleName string) error
	queue                 workqueue.RateLimitingInterface
	syncPeriod            time.Duration
	clock                 clock.Clock
	logger                *logrus.Logger
}

func NewScheduleController(
	schedulesClient arkv1client.SchedulesGetter,
	backupsClient arkv1client.BackupsGetter,
	schedulesInformer informers.ScheduleInformer,
	syncPeriod time.Duration,
	logger *logrus.Logger,
) *scheduleController {
	if syncPeriod < time.Minute {
		logger.WithField("syncPeriod", syncPeriod).Info("Provided schedule sync period is too short. Setting to 1 minute")
		syncPeriod = time.Minute
	}

	c := &scheduleController{
		schedulesClient:       schedulesClient,
		backupsClient:         backupsClient,
		schedulesLister:       schedulesInformer.Lister(),
		schedulesListerSynced: schedulesInformer.Informer().HasSynced,
		queue:      workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "schedule"),
		syncPeriod: syncPeriod,
		clock:      clock.RealClock{},
		logger:     logger,
	}

	c.syncHandler = c.processSchedule

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
			},
		},
	)

	return c
}

// Run is a blocking function that runs the specified number of worker goroutines
// to process items in the work queue. It will return when it receives on the
// ctx.Done() channel.
func (controller *scheduleController) Run(ctx context.Context, numWorkers int) error {
	var wg sync.WaitGroup

	defer func() {
		controller.logger.Info("Waiting for workers to finish their work")

		controller.queue.ShutDown()

		// We have to wait here in the deferred function instead of at the bottom of the function body
		// because we have to shut down the queue in order for the workers to shut down gracefully, and
		// we want to shut down the queue via defer and not at the end of the body.
		wg.Wait()

		controller.logger.Info("All workers have finished")
	}()

	controller.logger.Info("Starting ScheduleController")
	defer controller.logger.Info("Shutting down ScheduleController")

	controller.logger.Info("Waiting for caches to sync")
	if !cache.WaitForCacheSync(ctx.Done(), controller.schedulesListerSynced) {
		return errors.New("timed out waiting for caches to sync")
	}
	controller.logger.Info("Caches are synced")

	wg.Add(numWorkers)
	for i := 0; i < numWorkers; i++ {
		go func() {
			wait.Until(controller.runWorker, time.Second, ctx.Done())
			wg.Done()
		}()
	}

	go wait.Until(controller.enqueueAllEnabledSchedules, controller.syncPeriod, ctx.Done())

	<-ctx.Done()
	return nil
}

func (controller *scheduleController) enqueueAllEnabledSchedules() {
	schedules, err := controller.schedulesLister.Schedules(api.DefaultNamespace).List(labels.NewSelector())
	if err != nil {
		controller.logger.WithError(errors.WithStack(err)).Error("Error listing Schedules")
		return
	}

	for _, schedule := range schedules {
		if schedule.Status.Phase != api.SchedulePhaseEnabled {
			continue
		}

		key, err := cache.MetaNamespaceKeyFunc(schedule)
		if err != nil {
			controller.logger.WithError(errors.WithStack(err)).WithField("schedule", schedule).Error("Error creating queue key, item not added to queue")
			continue
		}
		controller.queue.Add(key)
	}
}

func (controller *scheduleController) runWorker() {
	// continually take items off the queue (waits if it's
	// empty) until we get a shutdown signal from the queue
	for controller.processNextWorkItem() {
	}
}

func (controller *scheduleController) processNextWorkItem() bool {
	key, quit := controller.queue.Get()
	if quit {
		return false
	}
	// always call done on this item, since if it fails we'll add
	// it back with rate-limiting below
	defer controller.queue.Done(key)

	err := controller.syncHandler(key.(string))
	if err == nil {
		// If you had no error, tell the queue to stop tracking history for your key. This will reset
		// things like failure counts for per-item rate limiting.
		controller.queue.Forget(key)
		return true
	}

	controller.logger.WithError(err).WithField("key", key).Error("Error in syncHandler, re-adding item to queue")
	// we had an error processing the item so add it back
	// into the queue for re-processing with rate-limiting
	controller.queue.AddRateLimited(key)

	return true
}

func (controller *scheduleController) processSchedule(key string) error {
	logContext := controller.logger.WithField("key", key)

	logContext.Debug("Running processSchedule")
	ns, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		return errors.Wrap(err, "error splitting queue key")
	}

	logContext.Debug("Getting Schedule")
	schedule, err := controller.schedulesLister.Schedules(ns).Get(name)
	if err != nil {
		// schedule no longer exists
		if apierrors.IsNotFound(err) {
			logContext.WithError(err).Debug("Schedule not found")
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

	logContext.Debug("Cloning schedule")
	// don't modify items in the cache
	schedule, err = cloneSchedule(schedule)
	if err != nil {
		return err
	}

	// validation - even if the item is Enabled, we can't trust it
	// so re-validate
	currentPhase := schedule.Status.Phase

	cronSchedule, errs := parseCronSchedule(schedule, controller.logger)
	if len(errs) > 0 {
		schedule.Status.Phase = api.SchedulePhaseFailedValidation
		schedule.Status.ValidationErrors = errs
	} else {
		schedule.Status.Phase = api.SchedulePhaseEnabled
	}

	// update status if it's changed
	if currentPhase != schedule.Status.Phase {
		updatedSchedule, err := controller.schedulesClient.Schedules(ns).Update(schedule)
		if err != nil {
			return errors.Wrapf(err, "error updating Schedule phase to %s", schedule.Status.Phase)
		}
		schedule = updatedSchedule
	}

	if schedule.Status.Phase != api.SchedulePhaseEnabled {
		return nil
	}

	// check for the schedule being due to run, and submit a Backup if so
	if err := controller.submitBackupIfDue(schedule, cronSchedule); err != nil {
		return err
	}

	return nil
}

func cloneSchedule(in interface{}) (*api.Schedule, error) {
	clone, err := scheme.Scheme.DeepCopy(in)
	if err != nil {
		return nil, errors.Wrap(err, "error deep-copying Schedule")
	}

	out, ok := clone.(*api.Schedule)
	if !ok {
		return nil, errors.Errorf("unexpected type: %T", clone)
	}

	return out, nil
}

func parseCronSchedule(itm *api.Schedule, logger *logrus.Logger) (cron.Schedule, []string) {
	var validationErrors []string
	var schedule cron.Schedule

	// cron.Parse panics if schedule is empty
	if len(itm.Spec.Schedule) == 0 {
		validationErrors = append(validationErrors, "Schedule must be a non-empty valid Cron expression")
		return nil, validationErrors
	}

	logContext := logger.WithField("schedule", kubeutil.NamespaceAndName(itm))

	// adding a recover() around cron.Parse because it panics on empty string and is possible
	// that it panics under other scenarios as well.
	func() {
		defer func() {
			if r := recover(); r != nil {
				logContext.WithFields(logrus.Fields{
					"schedule": itm.Spec.Schedule,
					"recover":  r,
				}).Debug("Panic parsing schedule")
				validationErrors = append(validationErrors, fmt.Sprintf("invalid schedule: %v", r))
			}
		}()

		if res, err := cron.ParseStandard(itm.Spec.Schedule); err != nil {
			logContext.WithError(errors.WithStack(err)).WithField("schedule", itm.Spec.Schedule).Debug("Error parsing schedule")
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

func (controller *scheduleController) submitBackupIfDue(item *api.Schedule, cronSchedule cron.Schedule) error {
	var (
		now                = controller.clock.Now()
		isDue, nextRunTime = getNextRunTime(item, cronSchedule, now)
		logContext         = controller.logger.WithField("schedule", kubeutil.NamespaceAndName(item))
	)

	if !isDue {
		logContext.WithField("nextRunTime", nextRunTime).Info("Schedule is not due, skipping")
		return nil
	}

	// Don't attempt to "catch up" if there are any missed or failed runs - simply
	// trigger a Backup if it's time.
	//
	// It might also make sense in the future to explicitly check for currently-running
	// backups so that we don't overlap runs (for disk snapshots in particular, this can
	// lead to performance issues).
	logContext.WithField("nextRunTime", nextRunTime).Info("Schedule is due, submitting Backup")
	backup := getBackup(item, now)
	if _, err := controller.backupsClient.Backups(backup.Namespace).Create(backup); err != nil {
		return errors.Wrap(err, "error creating Backup")
	}

	schedule, err := cloneSchedule(item)
	if err != nil {
		return err
	}

	schedule.Status.LastBackup = metav1.NewTime(now)

	if _, err := controller.schedulesClient.Schedules(schedule.Namespace).Update(schedule); err != nil {
		return errors.Wrapf(err, "error updating Schedule's LastBackup time to %v", schedule.Status.LastBackup)
	}

	return nil
}

func getNextRunTime(schedule *api.Schedule, cronSchedule cron.Schedule, asOf time.Time) (bool, time.Time) {
	// get the latest run time (if the schedule hasn't run yet, this will be the zero value which will trigger
	// an immediate backup)
	lastBackupTime := schedule.Status.LastBackup.Time

	nextRunTime := cronSchedule.Next(lastBackupTime)

	return asOf.After(nextRunTime), nextRunTime
}

func getBackup(item *api.Schedule, timestamp time.Time) *api.Backup {
	backup := &api.Backup{
		Spec: item.Spec.Template,
		ObjectMeta: metav1.ObjectMeta{
			Namespace: item.Namespace,
			Name:      fmt.Sprintf("%s-%s", item.Name, timestamp.Format("20060102150405")),
			Labels: map[string]string{
				"ark-schedule": item.Name,
			},
		},
	}

	return backup
}
