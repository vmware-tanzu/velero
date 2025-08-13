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

	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	velerov1api "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
)

// backupQueueReconciler reconciles a Backup object
type backupQueueReconciler struct {
	client.Client
	Scheme            *runtime.Scheme
	logger            logrus.FieldLogger
	concurrentBackups int
}

// NewBackupQueueReconciler returns a new backupQueueReconciler
func NewBackupQueueReconciler(
	client client.Client,
	scheme *runtime.Scheme,
	logger logrus.FieldLogger,
	concurrentBackups int,
) *backupQueueReconciler {
	return &backupQueueReconciler{
		Client:            client,
		Scheme:            scheme,
		logger:            logger,
		concurrentBackups: concurrentBackups,
	}
}

// SetupWithManager adds the reconciler to the manager
func (r *backupQueueReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&velerov1api.Backup{}).
		Complete(r)
}

// Reconcile reconciles a Backup object
func (r *backupQueueReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := r.logger.WithField("backup", req.NamespacedName.String())

	log.Debug("Getting backup")
	backup := &velerov1api.Backup{}
	if err := r.Get(ctx, req.NamespacedName, backup); err != nil {
		log.WithError(err).Error("unable to get backup")
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	return ctrl.Result{}, nil
}
