/*
Copyright the Velero contributors.

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

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/clock"

	velerov1api "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	"github.com/vmware-tanzu/velero/pkg/metrics"
	"github.com/vmware-tanzu/velero/pkg/persistence"
	"github.com/vmware-tanzu/velero/pkg/plugin/clientmgmt"
	kubeutil "github.com/vmware-tanzu/velero/pkg/util/kube"
	"github.com/vmware-tanzu/velero/pkg/util/results"
)

type restoreFinalizerReconciler struct {
	client.Client
	namespace         string
	logger            logrus.FieldLogger
	newPluginManager  func(logger logrus.FieldLogger) clientmgmt.Manager
	backupStoreGetter persistence.ObjectBackupStoreGetter
	metrics           *metrics.ServerMetrics
	clock             clock.WithTickerAndDelayedExecution
}

func NewRestoreFinalizerReconciler(
	logger logrus.FieldLogger,
	namespace string,
	client client.Client,
	newPluginManager func(logrus.FieldLogger) clientmgmt.Manager,
	backupStoreGetter persistence.ObjectBackupStoreGetter,
	metrics *metrics.ServerMetrics,
) *restoreFinalizerReconciler {
	return &restoreFinalizerReconciler{
		Client:            client,
		logger:            logger,
		namespace:         namespace,
		newPluginManager:  newPluginManager,
		backupStoreGetter: backupStoreGetter,
		metrics:           metrics,
		clock:             &clock.RealClock{},
	}
}

func (r *restoreFinalizerReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&velerov1api.Restore{}).
		Complete(r)
}

// +kubebuilder:rbac:groups=velero.io,resources=restores,verbs=get;list;watch;update
// +kubebuilder:rbac:groups=velero.io,resources=restores/status,verbs=get
func (r *restoreFinalizerReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := r.logger.WithField("restore finalizer", req.String())
	log.Debug("restoreFinalizerReconciler getting restore")

	original := &velerov1api.Restore{}
	if err := r.Get(ctx, req.NamespacedName, original); err != nil {
		if apierrors.IsNotFound(err) {
			log.WithError(err).Error("restore not found")
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, errors.Wrapf(err, "error getting restore %s", req.String())
	}
	restore := original.DeepCopy()
	log.Debugf("restore: %s", restore.Name)

	log = r.logger.WithFields(
		logrus.Fields{
			"restore": req.String(),
		},
	)

	switch restore.Status.Phase {
	case velerov1api.RestorePhaseFinalizing, velerov1api.RestorePhaseFinalizingPartiallyFailed:
	default:
		log.Debug("Restore is not awaiting finalization, skipping")
		return ctrl.Result{}, nil
	}

	info, err := fetchBackupInfoInternal(r.Client, r.namespace, restore.Spec.BackupName)
	if err != nil {
		if apierrors.IsNotFound(err) {
			log.WithError(err).Error("not found backup, skip")
			if err2 := r.finishProcessing(velerov1api.RestorePhasePartiallyFailed, restore, original); err2 != nil {
				log.WithError(err2).Error("error updating restore's final status")
				return ctrl.Result{}, errors.Wrap(err2, "error updating restore's final status")
			}
			return ctrl.Result{}, nil
		}
		log.WithError(err).Error("error getting backup info")
		return ctrl.Result{}, errors.Wrap(err, "error getting backup info")
	}

	pluginManager := r.newPluginManager(r.logger)
	defer pluginManager.CleanupClients()
	backupStore, err := r.backupStoreGetter.Get(info.location, pluginManager, r.logger)
	if err != nil {
		log.WithError(err).Error("error getting backup store")
		return ctrl.Result{}, errors.Wrap(err, "error getting backup store")
	}

	finalizerCtx := &finalizerContext{log: log}
	warnings, errs := finalizerCtx.execute()

	warningCnt := len(warnings.Velero) + len(warnings.Cluster)
	for _, w := range warnings.Namespaces {
		warningCnt += len(w)
	}
	errCnt := len(errs.Velero) + len(errs.Cluster)
	for _, e := range errs.Namespaces {
		errCnt += len(e)
	}
	restore.Status.Warnings += warningCnt
	restore.Status.Errors += errCnt

	if !errs.IsEmpty() {
		restore.Status.Phase = velerov1api.RestorePhaseFinalizingPartiallyFailed
	}

	if warningCnt > 0 || errCnt > 0 {
		err := r.updateResults(backupStore, restore, &warnings, &errs)
		if err != nil {
			log.WithError(err).Error("error updating results")
			return ctrl.Result{}, errors.Wrap(err, "error updating results")
		}
	}

	finalPhase := velerov1api.RestorePhaseCompleted
	if restore.Status.Phase == velerov1api.RestorePhaseFinalizingPartiallyFailed {
		finalPhase = velerov1api.RestorePhasePartiallyFailed
	}
	log.Infof("Marking restore %s", finalPhase)

	if err := r.finishProcessing(finalPhase, restore, original); err != nil {
		log.WithError(err).Error("error updating restore's final status")
		return ctrl.Result{}, errors.Wrap(err, "error updating restore's final status")
	}

	return ctrl.Result{}, nil
}

func (r *restoreFinalizerReconciler) updateResults(backupStore persistence.BackupStore, restore *velerov1api.Restore, newWarnings *results.Result, newErrs *results.Result) error {
	originResults, err := backupStore.GetRestoreResults(restore.Name)
	if err != nil {
		return errors.Wrap(err, "error getting restore results")
	}
	warnings := originResults["warnings"]
	errs := originResults["errors"]
	warnings.Merge(newWarnings)
	errs.Merge(newErrs)

	m := map[string]results.Result{
		"warnings": warnings,
		"errors":   errs,
	}
	if err := putResults(restore, m, backupStore); err != nil {
		return errors.Wrap(err, "error putting restore results")
	}

	return nil
}

func (r *restoreFinalizerReconciler) finishProcessing(restorePhase velerov1api.RestorePhase, restore *velerov1api.Restore, original *velerov1api.Restore) error {
	if restorePhase == velerov1api.RestorePhasePartiallyFailed {
		restore.Status.Phase = velerov1api.RestorePhasePartiallyFailed
		r.metrics.RegisterRestorePartialFailure(restore.Spec.ScheduleName)
	} else {
		restore.Status.Phase = velerov1api.RestorePhaseCompleted
		r.metrics.RegisterRestoreSuccess(restore.Spec.ScheduleName)
	}
	restore.Status.CompletionTimestamp = &metav1.Time{Time: r.clock.Now()}

	return kubeutil.PatchResource(original, restore, r.Client)
}

// finalizerContext includes all the dependencies required by finalization tasks and
// a function execute() to orderly implement task logic.
type finalizerContext struct {
	log logrus.FieldLogger
}

func (ctx *finalizerContext) execute() (results.Result, results.Result) { //nolint:unparam //temporarily ignore the lint report: result 0 is always nil (unparam)
	warnings, errs := results.Result{}, results.Result{}

	// implement finalization tasks
	ctx.log.Debug("Starting running execute()")

	return warnings, errs
}
