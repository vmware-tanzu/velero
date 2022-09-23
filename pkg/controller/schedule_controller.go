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
	"fmt"
	"time"

	"github.com/pkg/errors"
	"github.com/robfig/cron"
	"github.com/sirupsen/logrus"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/clock"
	ctrl "sigs.k8s.io/controller-runtime"
	bld "sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"

	velerov1 "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	"github.com/vmware-tanzu/velero/pkg/builder"
	"github.com/vmware-tanzu/velero/pkg/metrics"
	"github.com/vmware-tanzu/velero/pkg/util/kube"
	kubeutil "github.com/vmware-tanzu/velero/pkg/util/kube"
)

const (
	scheduleSyncPeriod = time.Minute
)

type scheduleReconciler struct {
	client.Client
	namespace string
	logger    logrus.FieldLogger
	clock     clock.Clock
	metrics   *metrics.ServerMetrics
}

func NewScheduleReconciler(
	namespace string,
	logger logrus.FieldLogger,
	client client.Client,
	metrics *metrics.ServerMetrics,
) *scheduleReconciler {
	return &scheduleReconciler{
		Client:    client,
		namespace: namespace,
		logger:    logger,
		clock:     clock.RealClock{},
		metrics:   metrics,
	}
}

func (c *scheduleReconciler) SetupWithManager(mgr ctrl.Manager) error {
	s := kube.NewPeriodicalEnqueueSource(c.logger, mgr.GetClient(), &velerov1.ScheduleList{}, scheduleSyncPeriod, kube.PeriodicalEnqueueSourceOption{})
	return ctrl.NewControllerManagedBy(mgr).
		// global predicate, works for both For and Watch
		WithEventFilter(kube.NewAllEventPredicate(func(obj client.Object) bool {
			schedule := obj.(*velerov1.Schedule)
			if pause := schedule.Spec.Paused; pause {
				c.logger.Infof("schedule %s is paused, skip", schedule.Name)
				return false
			}
			return true
		})).
		For(&velerov1.Schedule{}, bld.WithPredicates(kube.SpecChangePredicate{})).
		Watches(s, nil).
		Complete(c)
}

// +kubebuilder:rbac:groups=velero.io,resources=schedules,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=velero.io,resources=schedules/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=velero.io,resources=backups,verbs=create

func (c *scheduleReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := c.logger.WithField("schedule", req.String())

	log.Debug("Getting schedule")
	schedule := &velerov1.Schedule{}
	if err := c.Get(ctx, req.NamespacedName, schedule); err != nil {
		if apierrors.IsNotFound(err) {
			log.WithError(err).Error("schedule not found")
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, errors.Wrapf(err, "error getting schedule %s", req.String())
	}

	c.metrics.InitSchedule(schedule.Name)

	original := schedule.DeepCopy()

	// validation - even if the item is Enabled, we can't trust it
	// so re-validate
	currentPhase := schedule.Status.Phase

	cronSchedule, errs := parseCronSchedule(schedule, c.logger)
	if len(errs) > 0 {
		schedule.Status.Phase = velerov1.SchedulePhaseFailedValidation
		schedule.Status.ValidationErrors = errs
	} else {
		schedule.Status.Phase = velerov1.SchedulePhaseEnabled
	}

	// update status if it's changed
	if currentPhase != schedule.Status.Phase {
		if err := c.Patch(ctx, schedule, client.MergeFrom(original)); err != nil {
			return ctrl.Result{}, errors.Wrapf(err, "error updating phase of schedule %s to %s", req.String(), schedule.Status.Phase)
		}
	}

	if schedule.Status.Phase != velerov1.SchedulePhaseEnabled {
		log.Debugf("the schedule's phase is %s, isn't %s, skip", schedule.Status.Phase, velerov1.SchedulePhaseEnabled)
		return ctrl.Result{}, nil
	}

	// check for the schedule being due to run, and submit a Backup if so.
	// As the schedule must be validated before checking whether it's due, we cannot put the checking log in Predicate
	if err := c.submitBackupIfDue(ctx, schedule, cronSchedule); err != nil {
		return ctrl.Result{}, errors.Wrapf(err, "error running submitBackupIfDue for schedule %s", req.String())
	}

	return ctrl.Result{}, nil
}

func parseCronSchedule(itm *velerov1.Schedule, logger logrus.FieldLogger) (cron.Schedule, []string) {
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

func (c *scheduleReconciler) submitBackupIfDue(ctx context.Context, item *velerov1.Schedule, cronSchedule cron.Schedule) error {
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
	if err := c.Create(ctx, backup); err != nil {
		return errors.Wrap(err, "error creating Backup")
	}

	original := item.DeepCopy()
	item.Status.LastBackup = &metav1.Time{Time: now}

	if err := c.Patch(ctx, item, client.MergeFrom(original)); err != nil {
		return errors.Wrapf(err, "error updating Schedule's LastBackup time to %v", item.Status.LastBackup)
	}

	return nil
}

func getNextRunTime(schedule *velerov1.Schedule, cronSchedule cron.Schedule, asOf time.Time) (bool, time.Time) {
	var lastBackupTime time.Time
	if schedule.Status.LastBackup != nil {
		lastBackupTime = schedule.Status.LastBackup.Time
	} else {
		lastBackupTime = schedule.CreationTimestamp.Time
	}

	nextRunTime := cronSchedule.Next(lastBackupTime)

	return asOf.After(nextRunTime), nextRunTime
}

func getBackup(item *velerov1.Schedule, timestamp time.Time) *velerov1.Backup {
	name := item.TimestampedName(timestamp)
	backup := builder.
		ForBackup(item.Namespace, name).
		FromSchedule(item).
		Result()

	return backup
}
