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
	"context"

	"k8s.io/client-go/util/workqueue"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// EnqueueRequestsFromMapUpdateFunc has the same purpose with handler.EnqueueRequestsFromMapFunc.
// It's simpler on Update event because mapAndEnqueue is called once with the new object. EnqueueRequestsFromMapFunc is called twice with the old and new object.
func EnqueueRequestsFromMapUpdateFunc(fn handler.MapFunc) handler.EventHandler {
	return TypedEnqueueRequestsFromMapUpdateFunc(fn)
}

func TypedEnqueueRequestsFromMapUpdateFunc[object any, request comparable](fn handler.TypedMapFunc[object, request]) handler.TypedEventHandler[object, request] {
	return &enqueueRequestsFromMapFunc[object, request]{
		toRequests: fn,
	}
}

var _ handler.EventHandler = &enqueueRequestsFromMapFunc[client.Object, reconcile.Request]{}

type enqueueRequestsFromMapFunc[object any, request comparable] struct {
	toRequests handler.TypedMapFunc[object, request]
}

// Create implements EventHandler.
func (e *enqueueRequestsFromMapFunc[object, request]) Create(ctx context.Context, evt event.TypedCreateEvent[object], q workqueue.TypedRateLimitingInterface[request]) {
	e.mapAndEnqueue(ctx, q, evt.Object)
}

// Update implements EventHandler.
func (e *enqueueRequestsFromMapFunc[object, request]) Update(ctx context.Context, evt event.TypedUpdateEvent[object], q workqueue.TypedRateLimitingInterface[request]) {
	e.mapAndEnqueue(ctx, q, evt.ObjectNew)
}

// Delete implements EventHandler.
func (e *enqueueRequestsFromMapFunc[object, request]) Delete(ctx context.Context, evt event.TypedDeleteEvent[object], q workqueue.TypedRateLimitingInterface[request]) {
	e.mapAndEnqueue(ctx, q, evt.Object)
}

// Generic implements EventHandler.
func (e *enqueueRequestsFromMapFunc[object, request]) Generic(ctx context.Context, evt event.TypedGenericEvent[object], q workqueue.TypedRateLimitingInterface[request]) {
	e.mapAndEnqueue(ctx, q, evt.Object)
}

func (e *enqueueRequestsFromMapFunc[object, request]) mapAndEnqueue(ctx context.Context, q workqueue.TypedRateLimitingInterface[request], obj object) {
	reqs := map[request]struct{}{}

	for _, req := range e.toRequests(ctx, obj) {
		_, ok := reqs[req]
		if !ok {
			q.Add(req)
			reqs[req] = struct{}{}
		}
	}
}
