/*
Copyright 2018 the Velero contributors.

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
	"time"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/clock"
	"sigs.k8s.io/cluster-api/util/patch"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"

	"github.com/vmware-tanzu/velero/internal/velero"
	velerov1api "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	"github.com/vmware-tanzu/velero/pkg/buildinfo"
	"github.com/vmware-tanzu/velero/pkg/plugin/framework"
)

const (
	ttl                       = time.Minute
	statusRequestResyncPeriod = 5 * time.Minute
)

type PluginLister interface {
	// List returns all PluginIdentifiers for kind.
	List(kind framework.PluginKind) []framework.PluginIdentifier
}

// ServerStatusRequestReconciler reconciles a ServerStatusRequest object
type ServerStatusRequestReconciler struct {
	Scheme         *runtime.Scheme
	Client         client.Client
	Ctx            context.Context
	PluginRegistry PluginLister
	Clock          clock.Clock

	Log logrus.FieldLogger
}

// +kubebuilder:rbac:groups=velero.io,resources=serverstatusrequests,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=velero.io,resources=serverstatusrequests/status,verbs=get;update;patch
func (r *ServerStatusRequestReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := r.Log.WithFields(logrus.Fields{
		"controller":          ServerStatusRequest,
		"serverStatusRequest": req.NamespacedName,
	})

	// Fetch the ServerStatusRequest instance.
	log.Debug("Getting ServerStatusRequest")
	statusRequest := &velerov1api.ServerStatusRequest{}
	if err := r.Client.Get(r.Ctx, req.NamespacedName, statusRequest); err != nil {
		if apierrors.IsNotFound(err) {
			log.WithError(err).Error("ServerStatusRequest not found")
			return ctrl.Result{}, nil
		}

		log.WithError(err).Error("Error getting ServerStatusRequest")
		// Error reading the object - requeue the request.
		return ctrl.Result{}, err
	}

	log = r.Log.WithFields(logrus.Fields{
		"controller":          ServerStatusRequest,
		"serverStatusRequest": req.NamespacedName,
		"phase":               statusRequest.Status.Phase,
	})

	switch statusRequest.Status.Phase {
	case "", velerov1api.ServerStatusRequestPhaseNew:
		log.Info("Processing new ServerStatusRequest")

		// Initialize the patch helper.
		patchHelper, err := patch.NewHelper(statusRequest, r.Client)
		if err != nil {
			log.WithError(err).Error("Error getting a patch helper to update this resource")
			return ctrl.Result{}, err
		}

		statusRequest.Status.ServerVersion = buildinfo.Version
		statusRequest.Status.Phase = velerov1api.ServerStatusRequestPhaseProcessed
		statusRequest.Status.ProcessedTimestamp = &metav1.Time{Time: r.Clock.Now()}
		statusRequest.Status.Plugins = velero.GetInstalledPluginInfo(r.PluginRegistry)

		if err := patchHelper.Patch(r.Ctx, statusRequest); err != nil {
			log.WithError(err).Error("Error updating ServerStatusRequest status")
			return ctrl.Result{RequeueAfter: statusRequestResyncPeriod}, err
		}
	case velerov1api.ServerStatusRequestPhaseProcessed:
		log.Debug("Checking whether ServerStatusRequest has expired")
		expiration := statusRequest.Status.ProcessedTimestamp.Add(ttl)
		if expiration.After(r.Clock.Now()) {
			log.Debug("ServerStatusRequest has not expired")
			return ctrl.Result{RequeueAfter: statusRequestResyncPeriod}, nil
		}

		log.Debug("ServerStatusRequest has expired, deleting it")
		if err := r.Client.Delete(r.Ctx, statusRequest); err != nil {
			log.WithError(err).Error("Unable to delete the request")
			return ctrl.Result{}, nil
		}
	default:
		return ctrl.Result{}, errors.New("unexpected ServerStatusRequest phase")
	}

	// Requeue is mostly to handle deleting any expired status requests that were not
	// deleted as part of the normal client flow for whatever reason.
	return ctrl.Result{RequeueAfter: statusRequestResyncPeriod}, nil
}

func (r *ServerStatusRequestReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&velerov1api.ServerStatusRequest{}).
		WithOptions(controller.Options{
			MaxConcurrentReconciles: 10,
		}).
		Complete(r)
}
