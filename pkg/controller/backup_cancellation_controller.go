/*
Copyright 2020 the Velero contributors.

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

	"github.com/sirupsen/logrus"
	velerov1api "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	"github.com/vmware-tanzu/velero/pkg/constant"
	"github.com/vmware-tanzu/velero/pkg/itemoperationmap"
	"github.com/vmware-tanzu/velero/pkg/metrics"
	"github.com/vmware-tanzu/velero/pkg/persistence"
	"github.com/vmware-tanzu/velero/pkg/plugin/clientmgmt"
	kubeutil "github.com/vmware-tanzu/velero/pkg/util/kube"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	clocks "k8s.io/utils/clock"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type backupCancellationReconciler struct {
	client.Client
	logger            logrus.FieldLogger
	clock             clocks.WithTickerAndDelayedExecution
	itemOperationsMap *itemoperationmap.BackupItemOperationsMap
	newPluginManager  func(logger logrus.FieldLogger) clientmgmt.Manager
	backupStoreGetter persistence.ObjectBackupStoreGetter
	metrics           *metrics.ServerMetrics
	backupTracker     BackupTracker
}

func NewBackupCancellationReconciler(
	logger logrus.FieldLogger,
	client client.Client,
	newPluginManager func(logrus.FieldLogger) clientmgmt.Manager,
	backupStoreGetter persistence.ObjectBackupStoreGetter,
	metrics *metrics.ServerMetrics,
	itemOperationsMap *itemoperationmap.BackupItemOperationsMap,
	backupTracker BackupTracker,
) *backupCancellationReconciler {
	return &backupCancellationReconciler{
		Client:            client,
		logger:            logger,
		clock:             clocks.RealClock{},
		itemOperationsMap: itemOperationsMap,
		newPluginManager:  newPluginManager,
		backupStoreGetter: backupStoreGetter,
		metrics:           metrics,
		backupTracker:     backupTracker,
	}
}

func (r *backupCancellationReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&velerov1api.Backup{}).
		Named(constant.ControllerBackupCancellation).
		Complete(r)
}

func (r *backupCancellationReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := r.logger.WithFields(logrus.Fields{
		"controller":    constant.ControllerBackupCancellation,
		"backuprequest": req.String(),
	})

	original := &velerov1api.Backup{}
	err := r.Client.Get(ctx, req.NamespacedName, original)
	if err != nil {
		if apierrors.IsNotFound(err) {
			log.Debug("backup not found")
			return ctrl.Result{}, nil
		}
		log.WithError(err).Error("error getting backup")
		return ctrl.Result{}, err
	}

	switch original.Status.Phase {
	case "", velerov1api.BackupPhaseCancelling:
		// only process cancelling backups
	default:
		r.logger.WithFields(logrus.Fields{
			"backup": kubeutil.NamespaceAndName(original),
			"phase":  original.Status.Phase,
		}).Debug("Backup is not handled")
		return ctrl.Result{}, nil
	}

	log.Info("Reconciling backup cancellation")

	log.Debug("Getting backup")

	backup := original.DeepCopy()
	log.Debugf("backup: %s", backup.Name)

	log = r.logger.WithFields(
		logrus.Fields{
			"backup": req.String(),
		},
	)

	switch original.Status.Phase {
	case velerov1api.BackupPhaseCancelling:
		// only process cancelling backups
	default:
		r.logger.WithFields(logrus.Fields{
			"backup": kubeutil.NamespaceAndName(original),
			"phase":  original.Status.Phase,
		}).Debug("Backup is not handled")
		return ctrl.Result{}, nil
	}

	// Controller successfully catches a cancelling backup
	// Retrieve all async ops for the backup
	// Lets try to make the funcs in backup_operations_controller usable here

	// After line 113 in backup_cancellation_controller.go:

	// Perform cancellation cleanup
	// TODO: Add cleanup logic here (subset of deletion):
	// - Delete partial backup data from object storage?
	// - Clean up in-progress volume snapshots?
	// - Remove temporary files?

	//todo: remove backup from backup tracker
	r.backupTracker.Delete(backup.Namespace, backup.Name)

	// Then set to cancelled
	backup.Status.Phase = velerov1api.BackupPhaseCancelled

	if err := r.Client.Patch(ctx, backup, client.MergeFrom(original)); err != nil {
		log.WithError(err).Errorf("error updating backup phase to %v", backup.Status.Phase)
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}
