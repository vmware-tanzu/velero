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

package serverstatus

import (
	"context"
	"fmt"
	"time"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"go.etcd.io/etcd/client"
	"k8s.io/apimachinery/pkg/runtime"
	kbclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	"github.com/vmware-tanzu/velero/internal/velero"
	velerov1api "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	"github.com/vmware-tanzu/velero/pkg/builder"
	velerov1client "github.com/vmware-tanzu/velero/pkg/generated/clientset/versioned/typed/velero/v1"
)

type ServerStatusGetter interface {
	GetServerStatus(client velerov1client.ServerStatusRequestsGetter) (*velerov1api.ServerStatusRequest, error)
}

type DefaultServerStatusGetter struct {
	Namespace string
	Timeout   time.Duration
}

func (g *DefaultServerStatusGetter) GetServerStatus(mgr manager.Manager) (*velerov1api.ServerStatusRequest, error) {
	req := builder.ForServerStatusRequest(g.Namespace, "", "0").
		ObjectMeta(
			builder.WithGenerateName("velero-cli-"),
		).Result()

	if err := mgr.GetClient().Create(context.TODO(), req, &kbclient.CreateOptions{}); err != nil {
		return nil, errors.WithStack(err)
	}

	// select stmt waiting for a time out or a response (data on the channel)

	// entryLog.Info("Setting up controller")
	// c, err := controller.New("serverstatusrequest", mgr, controller.Options{
	// 	Reconciler: &velerocontroller.ServerStatusRequestReconciler{client: mgr.GetClient(), log: log.WithName("reconciler")},
	// })
	// if err != nil {
	// 	return nil, errors.WithStack(err)
	// }

	// defer mgr.GetClient().Delete(context.TODO(), req)

	// 1) create a Source (e.g. a KindSource)
	// and start it with whatever event handler you want + a queue, and then just pop things off the queue
	// var c controller.Controller
	// err := c.Watch(
	// 	&source.Kind{Type: &velerov1api.ServerStatusRequest{}},
	// 	handler.Funcs{
	// 		CreateFunc: func(e event.CreateEvent, q workqueue.RateLimitingInterface) {
	// 			q.Add(reconcile.Request{NamespacedName: types.NamespacedName{
	// 				Name:      e.Meta.GetName(),
	// 				Namespace: e.Meta.GetNamespace(),
	// 			}})
	// 		},
	// 		UpdateFunc: func(e event.UpdateEvent, q workqueue.RateLimitingInterface) {
	// 			q.Add(reconcile.Request{NamespacedName: types.NamespacedName{
	// 				Name:      e.MetaNew.GetName(),
	// 				Namespace: e.MetaNew.GetNamespace(),
	// 			}})
	// 		},
	// 		DeleteFunc: func(e event.DeleteEvent, q workqueue.RateLimitingInterface) {
	// 			q.Add(reconcile.Request{NamespacedName: types.NamespacedName{
	// 				Name:      e.Meta.GetName(),
	// 				Namespace: e.Meta.GetNamespace(),
	// 			}})
	// 		},
	// 		GenericFunc: func(e event.GenericEvent, q workqueue.RateLimitingInterface) {
	// 			q.Add(reconcile.Request{NamespacedName: types.NamespacedName{
	// 				Name:      e.Meta.GetName(),
	// 				Namespace: e.Meta.GetNamespace(),
	// 			}})
	// 		},
	// 	},
	// )
	// if err != nil {
	// 	return nil, errors.WithStack(err)
	// }

	// ctrl.NewControllerManagedBy(mgr).
	// 	For(&velerov1api.ServerStatusRequest{}).
	// 	Owns(&velerov1api.ServerStatusRequest{}).
	// 	Watches(&source.Kind{Type: &velerov1api.ServerStatusRequest{}}, &handler.EnqueueRequestForObject{}).Complete(r reconcile.Reconciler)

	// 2) Alternatively: use mgr.GetCache() to get the cache, and then grab and informer and just interface with the informer directly

	// reqInformer, err := mgr.GetCache().GetInformer(context.TODO(), req)
	// if err != nil {
	// 	return nil, errors.WithStack(err)
	// }

	// addFunc := func(obj interface{}) {
	// 	statusReq, ok := obj.(*velerov1api.ServerStatusRequest)
	// 	if !ok {
	// 		fmt.Printf("unexpected type %+v", obj)
	// 		// return nil, errors.Errorf("unexpected type %T", obj)
	// 	}

	// 	// if statusReq.Status.Phase == velerov1api.ServerStatusRequestPhaseProcessed {
	// 	req = statusReq
	// 	// }

	// }

	// updateFunc := func(_, newObj interface{}) {
	// 	statusReq, ok := newObj.(*velerov1api.ServerStatusRequest)
	// 	if !ok {
	// 		fmt.Printf("unexpected type %+v", newObj)
	// 		// return nil, errors.Errorf("unexpected type %T", oldObj)
	// 	}

	// 	fmt.Printf("\n\nUPDATED NEW %+v", statusReq)

	// 	// if statusReq.Status.Phase == velerov1api.ServerStatusRequestPhaseProcessed {
	// 	req = statusReq
	// 	// }
	// }

	// deleteFunc := func(obj interface{}) {
	// 	fmt.Print("obj deleted: %\n", obj)
	// 	// return nil, errors.New("server status request was unexpectedly deleted")
	// }

	// fmt.Printf("ss before: %+v\n\n", req)

	// reqInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
	// 	AddFunc:    addFunc,
	// 	UpdateFunc: updateFunc,
	// 	DeleteFunc: deleteFunc,
	// })

	// reqInformer.AddEventHandlerWithResyncPeriod(handler cache.ResourceEventHandler, resyncPeriod time.Duration)

	fmt.Printf("\n\nss after: %+v", req)

	// defer client.ServerStatusRequests(g.Namespace).Delete(context.TODO(), created.Name, metav1.DeleteOptions{})

	// listOptions := metav1.ListOptions{
	// 	// TODO: once the minimum supported Kubernetes version is v1.9.0, uncomment the following line.
	// 	// See http://issue.k8s.io/51046 for details.
	// 	//FieldSelector:   "metadata.name=" + req.Name
	// 	ResourceVersion: created.ResourceVersion,
	// }

	// s := source.NewKindWithCache(req, mgr.GetCache())
	// err = s.Start(handler.EventHandler, workqueue.RateLimitingInterface, ...predicate.Predicate)
	// if err != nil {
	// 		return nil, errors.WithStack(err)
	// 	}

	// watcher, err := client.ServerStatusRequests(g.Namespace).Watch(context.TODO(), listOptions)
	// if err != nil {
	// 	return nil, errors.WithStack(err)
	// }
	// defer watcher.Stop()

	// expired := time.NewTimer(g.Timeout)
	// defer expired.Stop()

	// Loop:
	// 	for {
	// 		select {
	// 		case <-expired.C:
	// 			return nil, errors.New("timed out waiting for server status request to be processed")
	// 		case e := <-watcher.ResultChan():
	// 			updated, ok := e.Object.(*velerov1api.ServerStatusRequest)
	// 			if !ok {
	// 				return nil, errors.Errorf("unexpected type %T", e.Object)
	// 			}

	// 			// TODO: once the minimum supported Kubernetes version is v1.9.0, remove the following check.
	// 			// See http://issue.k8s.io/51046 for details.
	// 			if updated.Name != created.Name {
	// 				continue
	// 			}

	// 			switch e.Type {
	// 			case watch.Deleted:
	// 				return nil, errors.New("server status request was unexpectedly deleted")
	// 			case watch.Modified:
	// 				if updated.Status.Phase == velerov1api.ServerStatusRequestPhaseProcessed {
	// 					req = updated
	// 					break Loop
	// 				}
	// 			}
	// 		}
	// 	}

	return req, nil

}

type ServerStatusRequest2Reconciler struct {
	Scheme *runtime.Scheme
	Client client.Client
	Ctx    context.Context

	Log logrus.FieldLogger
}

// +kubebuilder:rbac:groups=velero.io,resources=serverstatusrequests,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=velero.io,resources=serverstatusrequests/status,verbs=get;update;patch
func (r *ServerStatusRequest2Reconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
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
			return ctrl.Result{RequeueAfter: statusRequestResyncPeriod}, nil
		}

		// Error reading the object - requeue the request.
		return ctrl.Result{RequeueAfter: statusRequestResyncPeriod}, err
	}

	log = r.Log.WithFields(logrus.Fields{
		"controller":          "serverstatusrequest",
		"serverStatusRequest": req.NamespacedName,
		"phase":               statusRequest.Status.Phase,
	})

	err := velero.Process(statusRequest.DeepCopy(), r.Client, r.PluginRegistry, r.Clock, log)
	if err != nil {
		return ctrl.Result{RequeueAfter: statusRequestResyncPeriod}, err
	}

	return ctrl.Result{RequeueAfter: statusRequestResyncPeriod}, nil
}

func (r *ServerStatusRequestReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&velerov1api.ServerStatusRequest{}).
		Complete(r)
}
