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

	"github.com/sirupsen/logrus"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/clock"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/vmware-tanzu/velero/internal/velero"
	velerov1api "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	"github.com/vmware-tanzu/velero/pkg/plugin/clientmgmt"
)

// const statusRequestResyncPeriod = 5 * time.Minute

// ServerStatusRequestReconciler reconciles a ServerStatusRequest object
type ServerStatusRequestReconciler struct {
	Scheme         *runtime.Scheme
	Client         client.Client
	Ctx            context.Context
	PluginRegistry clientmgmt.Registry
	Clock          clock.Clock

	Log logrus.FieldLogger
}

// +kubebuilder:rbac:groups=velero.io,resources=serverstatusrequests,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=velero.io,resources=serverstatusrequests/status,verbs=get;update;patch
func (r *ServerStatusRequestReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	log := r.Log.WithFields(logrus.Fields{
		"controller":          "serverstatusrequest",
		"serverStatusRequest": req.NamespacedName,
	})

	// Fetch the ServerStatusRequest instance.
	log.Debug("Getting ServerStatusRequest")
	statusRequest := &velerov1api.ServerStatusRequest{}
	if err := r.Client.Get(r.Ctx, req.NamespacedName, statusRequest); err != nil {
		if apierrors.IsNotFound(err) {
			// server status request no longer exists
			log.WithError(err).Debug("ServerStatusRequest not found")
			return ctrl.Result{}, nil
		}

		// Error reading the object - requeue the request.
		return ctrl.Result{}, err
	}

	log = r.Log.WithFields(logrus.Fields{
		"controller":          "serverstatusrequest",
		"serverStatusRequest": req.NamespacedName,
		"phase":               statusRequest.Status.Phase,
	})

	err := velero.Process(statusRequest.DeepCopy(), r.Client, r.PluginRegistry, r.Clock, log)
	if err != nil {
		log.WithError(err).Error("Unable to process the request")
		return ctrl.Result{}, err
	}

	// return ctrl.Result{RequeueAfter: statusRequestResyncPeriod}, nil
	return ctrl.Result{}, nil
}

func (r *ServerStatusRequestReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&velerov1api.ServerStatusRequest{}).
		Complete(r)
}
