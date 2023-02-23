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
	clocks "k8s.io/utils/clock"
	ctrl "sigs.k8s.io/controller-runtime"
	kbclient "sigs.k8s.io/controller-runtime/pkg/client"

	velerov1api "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	"github.com/vmware-tanzu/velero/pkg/persistence"
	"github.com/vmware-tanzu/velero/pkg/plugin/clientmgmt"
)

// downloadRequestReconciler reconciles a DownloadRequest object
type downloadRequestReconciler struct {
	client kbclient.Client
	clock  clocks.Clock
	// use variables to refer to these functions so they can be
	// replaced with fakes for testing.
	newPluginManager  func(logrus.FieldLogger) clientmgmt.Manager
	backupStoreGetter persistence.ObjectBackupStoreGetter

	log logrus.FieldLogger
}

// NewDownloadRequestReconciler initializes and returns downloadRequestReconciler struct.
func NewDownloadRequestReconciler(
	client kbclient.Client,
	clock clocks.Clock,
	newPluginManager func(logrus.FieldLogger) clientmgmt.Manager,
	backupStoreGetter persistence.ObjectBackupStoreGetter,
	log logrus.FieldLogger,
) *downloadRequestReconciler {
	return &downloadRequestReconciler{
		client:            client,
		clock:             clock,
		newPluginManager:  newPluginManager,
		backupStoreGetter: backupStoreGetter,
		log:               log,
	}
}

// +kubebuilder:rbac:groups=velero.io,resources=downloadrequests,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=velero.io,resources=downloadrequests/status,verbs=get;update;patch
func (r *downloadRequestReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := r.log.WithFields(logrus.Fields{
		"controller":      "download-request",
		"downloadRequest": req.NamespacedName,
	})

	// Fetch the DownloadRequest instance.
	log.Debug("Getting DownloadRequest")
	downloadRequest := &velerov1api.DownloadRequest{}
	if err := r.client.Get(ctx, req.NamespacedName, downloadRequest); err != nil {
		if apierrors.IsNotFound(err) {
			log.Debug("Unable to find DownloadRequest")
			return ctrl.Result{}, nil
		}

		log.WithError(err).Error("Error getting DownloadRequest")
		return ctrl.Result{}, errors.WithStack(err)
	}

	original := downloadRequest.DeepCopy()
	defer func() {
		// Always attempt to Patch the downloadRequest object and status after each reconciliation.
		if err := r.client.Patch(ctx, downloadRequest, kbclient.MergeFrom(original)); err != nil {
			log.WithError(err).Error("Error updating download request")
			return
		}
	}()

	if downloadRequest.Status != (velerov1api.DownloadRequestStatus{}) && downloadRequest.Status.Expiration != nil {
		if downloadRequest.Status.Expiration.Time.Before(r.clock.Now()) {

			// Delete any request that is expired, regardless of the phase: it is not
			// worth proceeding and trying/retrying to find it.
			log.Debug("DownloadRequest has expired - deleting")
			if err := r.client.Delete(ctx, downloadRequest); err != nil {
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
		downloadRequest.Status.Expiration = &metav1.Time{Time: r.clock.Now().Add(persistence.DownloadURLTTL)}

		if downloadRequest.Spec.Target.Kind == velerov1api.DownloadTargetKindRestoreLog ||
			downloadRequest.Spec.Target.Kind == velerov1api.DownloadTargetKindRestoreResults {
			restore := &velerov1api.Restore{}
			if err := r.client.Get(ctx, kbclient.ObjectKey{
				Namespace: downloadRequest.Namespace,
				Name:      downloadRequest.Spec.Target.Name,
			}, restore); err != nil {
				return ctrl.Result{}, errors.WithStack(err)
			}
			backupName = restore.Spec.BackupName
		}

		backup := &velerov1api.Backup{}
		if err := r.client.Get(ctx, kbclient.ObjectKey{
			Namespace: downloadRequest.Namespace,
			Name:      backupName,
		}, backup); err != nil {
			return ctrl.Result{}, errors.WithStack(err)
		}

		location := &velerov1api.BackupStorageLocation{}
		if err := r.client.Get(ctx, kbclient.ObjectKey{
			Namespace: backup.Namespace,
			Name:      backup.Spec.StorageLocation,
		}, location); err != nil {
			return ctrl.Result{}, errors.WithStack(err)
		}

		pluginManager := r.newPluginManager(log)
		defer pluginManager.CleanupClients()

		backupStore, err := r.backupStoreGetter.Get(location, pluginManager, log)
		if err != nil {
			log.WithError(err).Error("Error getting a backup store")
			return ctrl.Result{}, errors.WithStack(err)
		}

		if downloadRequest.Status.DownloadURL, err = backupStore.GetDownloadURL(downloadRequest.Spec.Target); err != nil {
			return ctrl.Result{Requeue: true}, errors.WithStack(err)
		}

		downloadRequest.Status.Phase = velerov1api.DownloadRequestPhaseProcessed

		// Update the expiration again to extend the time we wait (the TTL) to start after successfully processing the URL.
		downloadRequest.Status.Expiration = &metav1.Time{Time: r.clock.Now().Add(persistence.DownloadURLTTL)}
	}

	// Requeue is mostly to handle deleting any expired requests that were not
	// deleted as part of the normal client flow for whatever reason.
	return ctrl.Result{Requeue: true}, nil
}

func (r *downloadRequestReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&velerov1api.DownloadRequest{}).
		Complete(r)
}
