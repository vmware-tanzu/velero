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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/clock"
	"sigs.k8s.io/cluster-api/util/patch"
	ctrl "sigs.k8s.io/controller-runtime"
	kbclient "sigs.k8s.io/controller-runtime/pkg/client"

	velerov1api "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	"github.com/vmware-tanzu/velero/pkg/persistence"
	"github.com/vmware-tanzu/velero/pkg/plugin/clientmgmt"
)

// DownloadRequestReconciler reconciles a DownloadRequest object
type DownloadRequestReconciler struct {
	Scheme *runtime.Scheme
	Client kbclient.Client
	Clock  clock.Clock
	// use variables to refer to these functions so they can be
	// replaced with fakes for testing.
	NewPluginManager  func(logrus.FieldLogger) clientmgmt.Manager
	BackupStoreGetter persistence.ObjectBackupStoreGetter

	Log logrus.FieldLogger
}

// +kubebuilder:rbac:groups=velero.io,resources=downloadrequests,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=velero.io,resources=downloadrequests/status,verbs=get;update;patch
func (r *DownloadRequestReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := r.Log.WithFields(logrus.Fields{
		"controller":      "download-request",
		"downloadRequest": req.NamespacedName,
	})

	// Fetch the DownloadRequest instance.
	log.Debug("Getting DownloadRequest")
	downloadRequest := &velerov1api.DownloadRequest{}
	if err := r.Client.Get(ctx, req.NamespacedName, downloadRequest); err != nil {
		if apierrors.IsNotFound(err) {
			log.Debug("Unable to find DownloadRequest")
			return ctrl.Result{}, nil
		}

		log.WithError(err).Error("Error getting DownloadRequest")
		return ctrl.Result{}, errors.WithStack(err)
	}

	// Initialize the patch helper.
	patchHelper, err := patch.NewHelper(downloadRequest, r.Client)
	if err != nil {
		log.WithError(err).Error("Error getting a patch helper to update this resource")
		return ctrl.Result{}, errors.WithStack(err)
	}

	defer func() {
		// Always attempt to Patch the downloadRequest object and status after each reconciliation.
		if err := patchHelper.Patch(ctx, downloadRequest); err != nil {
			log.WithError(err).Error("Error updating download request")
			return
		}
	}()

	if downloadRequest.Status != (velerov1api.DownloadRequestStatus{}) && downloadRequest.Status.Expiration != nil {
		if downloadRequest.Status.Expiration.Time.Before(r.Clock.Now()) {

			// Delete any request that is expired, regardless of the phase: it is not
			// worth proceeding and trying/retrying to find it.
			log.Debug("DownloadRequest has expired - deleting")
			if err := r.Client.Delete(ctx, downloadRequest); err != nil {
				log.WithError(err).Error("Error deleting an expired download request")
				return ctrl.Result{}, errors.WithStack(err)
			}
			return ctrl.Result{Requeue: false}, nil

		} else if downloadRequest.Status.Phase == velerov1api.DownloadRequestPhaseProcessed {

			// Requeue the request if is not yet expired and has already been processed before,
			// since it might still be in use by the logs streaming and shouldn't
			// be deleted until after its expiration.
			log.Debug("DownloadRequest has not yet expired - requeueing")
			return ctrl.Result{Requeue: true}, nil
		}
	}

	// Process a brand new request.
	backupName := downloadRequest.Spec.Target.Name
	if downloadRequest.Status.Phase == "" || downloadRequest.Status.Phase == velerov1api.DownloadRequestPhaseNew {

		// Update the expiration.
		downloadRequest.Status.Expiration = &metav1.Time{Time: r.Clock.Now().Add(persistence.DownloadURLTTL)}

		if downloadRequest.Spec.Target.Kind == velerov1api.DownloadTargetKindRestoreLog ||
			downloadRequest.Spec.Target.Kind == velerov1api.DownloadTargetKindRestoreResults {
			restore := &velerov1api.Restore{}
			if err := r.Client.Get(ctx, kbclient.ObjectKey{
				Namespace: downloadRequest.Namespace,
				Name:      downloadRequest.Spec.Target.Name,
			}, restore); err != nil {
				return ctrl.Result{}, errors.WithStack(err)
			}
			backupName = restore.Spec.BackupName
		}

		backup := &velerov1api.Backup{}
		if err := r.Client.Get(ctx, kbclient.ObjectKey{
			Namespace: downloadRequest.Namespace,
			Name:      backupName,
		}, backup); err != nil {
			return ctrl.Result{}, errors.WithStack(err)
		}

		location := &velerov1api.BackupStorageLocation{}
		if err := r.Client.Get(ctx, kbclient.ObjectKey{
			Namespace: backup.Namespace,
			Name:      backup.Spec.StorageLocation,
		}, location); err != nil {
			return ctrl.Result{}, errors.WithStack(err)
		}

		pluginManager := r.NewPluginManager(log)
		defer pluginManager.CleanupClients()

		backupStore, err := r.BackupStoreGetter.Get(location, pluginManager, log)
		if err != nil {
			log.WithError(err).Error("Error getting a backup store")
			return ctrl.Result{}, errors.WithStack(err)
		}

		if downloadRequest.Status.DownloadURL, err = backupStore.GetDownloadURL(downloadRequest.Spec.Target); err != nil {
			return ctrl.Result{Requeue: true}, errors.WithStack(err)
		}

		downloadRequest.Status.Phase = velerov1api.DownloadRequestPhaseProcessed

		// Update the expiration again to extend the time we wait (the TTL) to start after successfully processing the URL.
		downloadRequest.Status.Expiration = &metav1.Time{Time: r.Clock.Now().Add(persistence.DownloadURLTTL)}
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
