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
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/golang/glog"
	"github.com/robfig/cron"

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
}

func NewScheduleController(
	schedulesClient arkv1client.SchedulesGetter,
	backupsClient arkv1client.BackupsGetter,
	schedulesInformer informers.ScheduleInformer,
	syncPeriod time.Duration,
) *scheduleController {
	if syncPeriod < time.Minute {
		glog.Infof("Schedule sync period %v is too short. Setting to 1 minute", syncPeriod)
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
					glog.V(4).Infof("Schedule %s/%s has phase %s - skipping", schedule.Namespace, schedule.Name, schedule.Status.Phase)
					return
				}

				key, err := cache.MetaNamespaceKeyFunc(schedule)
				if err != nil {
					glog.Errorf("error creating queue key for %#v: %v", schedule, err)
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
		glog.Infof("Waiting for workers to finish their work")

		controller.queue.ShutDown()

		// We have to wait here in the deferred function instead of at the bottom of the function body
		// because we have to shut down the queue in order for the workers to shut down gracefully, and
		// we want to shut down the queue via defer and not at the end of the body.
		wg.Wait()

		glog.Infof("All workers have finished")
	}()

	glog.Info("Starting ScheduleController")
	defer glog.Info("Shutting down ScheduleController")

	glog.Info("Waiting for caches to sync")
	if !cache.WaitForCacheSync(ctx.Done(), controller.schedulesListerSynced) {
		return errors.New("timed out waiting for caches to sync")
	}
	glog.Info("Caches are synced")

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
		glog.Errorf("error listing schedules: %v", err)
		return
	}

	for _, schedule := range schedules {
		if schedule.Status.Phase != api.SchedulePhaseEnabled {
			continue
		}

		key, err := cache.MetaNamespaceKeyFunc(schedule)
		if err != nil {
			glog.Errorf("error creating queue key for %#v: %v", schedule, err)
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

	glog.Errorf("syncHandler error: %v", err)
	// we had an error processing the item so add it back
	// into the queue for re-processing with rate-limiting
	controller.queue.AddRateLimited(key)

	return true
}

func (controller *scheduleController) processSchedule(key string) error {
	glog.V(4).Infof("processSchedule for key %q", key)
	ns, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		glog.V(4).Infof("error splitting key %q: %v", key, err)
		return err
	}

	glog.V(4).Infof("Getting schedule %s", key)
	schedule, err := controller.schedulesLister.Schedules(ns).Get(name)
	if err != nil {
		// schedule no longer exists
		if apierrors.IsNotFound(err) {
			glog.V(4).Infof("schedule %s not found: %v", key, err)
			return nil
		}
		glog.V(4).Infof("error getting schedule %s: %v", key, err)
		return err
	}

	switch schedule.Status.Phase {
	case "", api.SchedulePhaseNew, api.SchedulePhaseEnabled:
		// valid phase for processing
	default:
		return nil
	}

	glog.V(4).Infof("Cloning schedule %s", key)
	// don't modify items in the cache
	schedule, err = cloneSchedule(schedule)
	if err != nil {
		glog.V(4).Infof("error cloning schedule %s: %v", key, err)
		return err
	}

	// validation - even if the item is Enabled, we can't trust it
	// so re-validate
	currentPhase := schedule.Status.Phase

	cronSchedule, errs := parseCronSchedule(schedule)
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
			glog.V(4).Infof("error updating status to %s: %v", schedule.Status.Phase, err)
			return err
		}
		schedule = updatedSchedule
	}

	if schedule.Status.Phase != api.SchedulePhaseEnabled {
		return nil
	}

	// check for the schedule being due to run, and submit a Backup if so
	if err := controller.submitBackupIfDue(schedule, cronSchedule); err != nil {
		glog.V(4).Infof("error processing Schedule %v/%v: err=%v", schedule.Namespace, schedule.Name, err)
		return err
	}

	return nil
}

func cloneSchedule(in interface{}) (*api.Schedule, error) {
	clone, err := scheme.Scheme.DeepCopy(in)
	if err != nil {
		return nil, err
	}

	out, ok := clone.(*api.Schedule)
	if !ok {
		return nil, fmt.Errorf("unexpected type: %T", clone)
	}

	return out, nil
}

func parseCronSchedule(itm *api.Schedule) (cron.Schedule, []string) {
	var validationErrors []string
	var schedule cron.Schedule

	// cron.Parse panics if schedule is empty
	if len(itm.Spec.Schedule) == 0 {
		validationErrors = append(validationErrors, "Schedule must be a non-empty valid Cron expression")
		return nil, validationErrors
	}

	// adding a recover() around cron.Parse because it panics on empty string and is possible
	// that it panics under other scenarios as well.
	func() {
		defer func() {
			if r := recover(); r != nil {
				glog.V(4).Infof("panic parsing schedule %v/%v, cron schedule=%v: %v", itm.Namespace, itm.Name, itm.Spec.Schedule, r)
				validationErrors = append(validationErrors, fmt.Sprintf("invalid schedule: %v", r))
			}
		}()

		if res, err := cron.ParseStandard(itm.Spec.Schedule); err != nil {
			glog.V(4).Infof("error parsing schedule %v/%v, cron schedule=%v: %v", itm.Namespace, itm.Name, itm.Spec.Schedule, err)
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
	now := controller.clock.Now()

	isDue, nextRunTime := getNextRunTime(item, cronSchedule, now)

	if !isDue {
		glog.Infof("Next run time for %v/%v is %v, skipping...", item.Namespace, item.Name, nextRunTime)
		return nil
	}

	// Don't attempt to "catch up" if there are any missed or failed runs - simply
	// trigger a Backup if it's time.
	//
	// It might also make sense in the future to explicitly check for currently-running
	// backups so that we don't overlap runs (for disk snapshots in particular, this can
	// lead to performance issues).

	glog.Infof("Next run time for %v/%v is %v, submitting Backup...", item.Namespace, item.Name, nextRunTime)
	backup := getBackup(item, now)
	if _, err := controller.backupsClient.Backups(backup.Namespace).Create(backup); err != nil {
		glog.V(4).Infof("error creating Backup: %v", err)
		return err
	}

	schedule, err := cloneSchedule(item)
	if err != nil {
		glog.V(4).Infof("error cloning Schedule %v/%v: %v", item.Namespace, item.Name, err)
		return err
	}

	schedule.Status.LastBackup = metav1.NewTime(now)

	if _, err := controller.schedulesClient.Schedules(schedule.Namespace).Update(schedule); err != nil {
		glog.V(4).Infof("error updating LastBackup for Schedule %v/%v: %v", schedule.Namespace, schedule.Name, err)
		return err
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
