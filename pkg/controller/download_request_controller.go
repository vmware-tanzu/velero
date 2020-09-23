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

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/clock"
	"sigs.k8s.io/cluster-api/util/patch"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	kbclient "sigs.k8s.io/controller-runtime/pkg/client"

	velerov1api "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	"github.com/vmware-tanzu/velero/pkg/persistence"
	"github.com/vmware-tanzu/velero/pkg/plugin/clientmgmt"
)

// DownloadRequestReconciler reconciles a DownloadRequest object
type DownloadRequestReconciler struct {
	Scheme *runtime.Scheme
	Client kbclient.Client
	Ctx    context.Context
	Clock  clock.Clock
	// use variables to refer to these functions so they can be
	// replaced with fakes for testing.
	NewPluginManager func(logrus.FieldLogger) clientmgmt.Manager
	NewBackupStore   func(*velerov1api.BackupStorageLocation, persistence.ObjectStoreGetter, logrus.FieldLogger) (persistence.BackupStore, error)

	Log logrus.FieldLogger
}

// +kubebuilder:rbac:groups=velero.io,resources=downloadrequests,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=velero.io,resources=downloadrequests/status,verbs=get;update;patch
func (r *DownloadRequestReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	log := r.Log.WithFields(logrus.Fields{
		"controller":      "download-request",
		"downloadRequest": req.NamespacedName,
	})

	// Fetch the DownloadRequest instance.
	log.Debug("Getting DownloadRequest")
	downloadRequest := &velerov1api.DownloadRequest{}
	if err := r.Client.Get(r.Ctx, req.NamespacedName, downloadRequest); err != nil {
		if apierrors.IsNotFound(err) {
			log.Debug("Unable to find DownloadRequest")
			return ctrl.Result{}, nil
		}

		log.WithError(err).Error("Error getting DownloadRequest")
		return ctrl.Result{}, err
	}

	// Initialize the patch helper.
	patchHelper, err := patch.NewHelper(downloadRequest, r.Client)
	if err != nil {
		log.WithError(err).Error("Error getting a patch helper to update this resource")
		return ctrl.Result{}, err
	}

	defer func() {
		// Always attempt to Patch the downloadRequest object and status after each reconciliation.
		if err := patchHelper.Patch(r.Ctx, downloadRequest); err != nil {
			log.WithError(err).Error("Error updating download request")
			return
		}
	}()

	backupName := downloadRequest.Spec.Target.Name
	switch downloadRequest.Status.Phase {
	case "", velerov1api.DownloadRequestPhaseNew:
		if downloadRequest.Spec.Target.Kind == velerov1api.DownloadTargetKindRestoreLog ||
			downloadRequest.Spec.Target.Kind == velerov1api.DownloadTargetKindRestoreResults {
			restore := &velerov1api.Restore{}
			if err := r.Client.Get(r.Ctx, client.ObjectKey{
				Namespace: downloadRequest.Namespace,
				Name:      downloadRequest.Spec.Target.Name,
			}, restore); err != nil {
				return ctrl.Result{}, errors.Wrap(err, "error getting Restore")
			}
			backupName = restore.Spec.BackupName
		}

		backup := &velerov1api.Backup{}
		if err := r.Client.Get(r.Ctx, client.ObjectKey{
			Namespace: downloadRequest.Namespace,
			Name:      backupName,
		}, backup); err != nil {
			return ctrl.Result{}, errors.WithStack(err)
		}

		location := &velerov1api.BackupStorageLocation{}
		if err := r.Client.Get(r.Ctx, client.ObjectKey{
			Namespace: backup.Namespace,
			Name:      backup.Spec.StorageLocation,
		}, location); err != nil {
			return ctrl.Result{}, errors.WithStack(err)
		}

		pluginManager := r.NewPluginManager(log)
		defer pluginManager.CleanupClients()

		backupStore, err := r.NewBackupStore(location, pluginManager, log)
		if err != nil {
			log.WithError(err).Error("Error getting a backup store")
			return ctrl.Result{}, errors.WithStack(err)
		}

		if downloadRequest.Status.DownloadURL, err = backupStore.GetDownloadURL(downloadRequest.Spec.Target); err != nil {
			return ctrl.Result{Requeue: true}, err
		}
		downloadRequest.Status.Phase = velerov1api.DownloadRequestPhaseProcessed
		downloadRequest.Status.Expiration = &metav1.Time{Time: r.Clock.Now().Add(persistence.DownloadURLTTL)}
	case velerov1api.DownloadRequestPhaseProcessed:
		if downloadRequest.Status.Expiration.Time.After(r.Clock.Now()) {
			log.Debug("DownloadRequest has not expired")
			return ctrl.Result{Requeue: true}, nil
		}
		log.Debug("DownloadRequest has expired - deleting")
		if err := r.Client.Delete(r.Ctx, downloadRequest); err != nil {
			log.WithError(err).Error("Error deleting an expired download request")
			return ctrl.Result{}, errors.WithStack(err)
		}
	}

	// Requeue is mostly to handle deleting any expired requests that were not
	// deleted as part of the normal client flow for whatever reason.
	return ctrl.Result{Requeue: true}, nil
}

func (r *DownloadRequestReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&velerov1api.DownloadRequest{}).
		Complete(r)
}
