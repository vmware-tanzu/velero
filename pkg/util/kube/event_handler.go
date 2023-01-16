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

package kube

import (
	"k8s.io/client-go/util/workqueue"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

type MapUpdateFunc func(client.Object) []reconcile.Request

// EnqueueRequestsFromMapUpdateFunc is for the same purpose with EnqueueRequestsFromMapFunc.
// Merely, it is more friendly to updating the mapped objects in the MapUpdateFunc, because
// on Update event, MapUpdateFunc is called for only once with the new object, so if MapUpdateFunc
// does some update to the mapped objects, the update is done for once
func EnqueueRequestsFromMapUpdateFunc(fn MapUpdateFunc) handler.EventHandler {
	return &enqueueRequestsFromMapFunc{
		toRequests: fn,
	}
}

var _ handler.EventHandler = &enqueueRequestsFromMapFunc{}

type enqueueRequestsFromMapFunc struct {
	toRequests MapUpdateFunc
}

// Create implements EventHandler.
func (e *enqueueRequestsFromMapFunc) Create(evt event.CreateEvent, q workqueue.RateLimitingInterface) {
	e.mapAndEnqueue(q, evt.Object)
}

// Update implements EventHandler.
func (e *enqueueRequestsFromMapFunc) Update(evt event.UpdateEvent, q workqueue.RateLimitingInterface) {
	e.mapAndEnqueue(q, evt.ObjectNew)
}

// Delete implements EventHandler.
func (e *enqueueRequestsFromMapFunc) Delete(evt event.DeleteEvent, q workqueue.RateLimitingInterface) {
	e.mapAndEnqueue(q, evt.Object)
}

// Generic implements EventHandler.
func (e *enqueueRequestsFromMapFunc) Generic(evt event.GenericEvent, q workqueue.RateLimitingInterface) {
	e.mapAndEnqueue(q, evt.Object)
}

func (e *enqueueRequestsFromMapFunc) mapAndEnqueue(q workqueue.RateLimitingInterface, object client.Object) {
	reqs := map[reconcile.Request]struct{}{}

	for _, req := range e.toRequests(object) {
		_, ok := reqs[req]
		if !ok {
			q.Add(req)
			reqs[req] = struct{}{}
		}
	}
}
