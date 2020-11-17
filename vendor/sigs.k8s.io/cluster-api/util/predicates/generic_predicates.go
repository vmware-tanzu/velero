/*
Copyright 2020 The Kubernetes Authors.

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

package predicates

import (
	"strings"

	"github.com/go-logr/logr"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/cluster-api/util/annotations"

	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

// All returns a predicate that returns true only if all given predicates return true
func All(logger logr.Logger, predicates ...predicate.Funcs) predicate.Funcs {
	log := logger.WithValues("predicateAggregation", "All")
	return predicate.Funcs{
		UpdateFunc: func(e event.UpdateEvent) bool {
			for _, p := range predicates {
				if !p.UpdateFunc(e) {
					log.V(4).Info("One of the provided predicates returned false, blocking further processing")
					return false
				}
			}
			log.V(4).Info("All provided predicates returned true, allowing further processing")
			return true
		},
		CreateFunc: func(e event.CreateEvent) bool {
			for _, p := range predicates {
				if !p.CreateFunc(e) {
					log.V(4).Info("One of the provided predicates returned false, blocking further processing")
					return false
				}
			}
			log.V(4).Info("All provided predicates returned true, allowing further processing")
			return true
		},
		DeleteFunc: func(e event.DeleteEvent) bool {
			for _, p := range predicates {
				if !p.DeleteFunc(e) {
					log.V(4).Info("One of the provided predicates returned false, blocking further processing")
					return false
				}
			}
			log.V(4).Info("All provided predicates returned true, allowing further processing")
			return true
		},
		GenericFunc: func(e event.GenericEvent) bool {
			for _, p := range predicates {
				if !p.GenericFunc(e) {
					log.V(4).Info("One of the provided predicates returned false, blocking further processing")
					return false
				}
			}
			log.V(4).Info("All provided predicates returned true, allowing further processing")
			return true
		},
	}
}

// Any returns a predicate that returns true only if any given predicate returns true
func Any(logger logr.Logger, predicates ...predicate.Funcs) predicate.Funcs {
	log := logger.WithValues("predicateAggregation", "Any")
	return predicate.Funcs{
		UpdateFunc: func(e event.UpdateEvent) bool {
			for _, p := range predicates {
				if p.UpdateFunc(e) {
					log.V(4).Info("One of the provided predicates returned true, allowing further processing")
					return true
				}
			}
			log.V(4).Info("All of the provided predicates returned false, blocking further processing")
			return false
		},
		CreateFunc: func(e event.CreateEvent) bool {
			for _, p := range predicates {
				if p.CreateFunc(e) {
					log.V(4).Info("One of the provided predicates returned true, allowing further processing")
					return true
				}
			}
			log.V(4).Info("All of the provided predicates returned false, blocking further processing")
			return false
		},
		DeleteFunc: func(e event.DeleteEvent) bool {
			for _, p := range predicates {
				if p.DeleteFunc(e) {
					log.V(4).Info("One of the provided predicates returned true, allowing further processing")
					return true
				}
			}
			log.V(4).Info("All of the provided predicates returned false, blocking further processing")
			return false
		},
		GenericFunc: func(e event.GenericEvent) bool {
			for _, p := range predicates {
				if p.GenericFunc(e) {
					log.V(4).Info("One of the provided predicates returned true, allowing further processing")
					return true
				}
			}
			log.V(4).Info("All of the provided predicates returned false, blocking further processing")
			return false
		},
	}
}

// ResourceNotPaused returns a Predicate that returns true only if the provided resource does not contain the
// paused annotation.
// This implements a common requirement for all cluster-api and provider controllers skip reconciliation when the paused
// annotation is present for a resource.
// Example use:
//	func (r *MyReconciler) SetupWithManager(mgr ctrl.Manager, options controller.Options) error {
//		controller, err := ctrl.NewControllerManagedBy(mgr).
//			For(&v1.MyType{}).
//			WithOptions(options).
//			WithEventFilter(util.ResourceNotPaused(r.Log)).
//			Build(r)
//		return err
//	}
func ResourceNotPaused(logger logr.Logger) predicate.Funcs {
	return predicate.Funcs{
		UpdateFunc: func(e event.UpdateEvent) bool {
			return processIfNotPaused(logger.WithValues("predicate", "updateEvent"), e.ObjectNew, e.MetaNew)
		},
		CreateFunc: func(e event.CreateEvent) bool {
			return processIfNotPaused(logger.WithValues("predicate", "createEvent"), e.Object, e.Meta)
		},
		DeleteFunc: func(e event.DeleteEvent) bool {
			return processIfNotPaused(logger.WithValues("predicate", "deleteEvent"), e.Object, e.Meta)
		},
		GenericFunc: func(e event.GenericEvent) bool {
			return processIfNotPaused(logger.WithValues("predicate", "genericEvent"), e.Object, e.Meta)
		},
	}
}

func processIfNotPaused(logger logr.Logger, obj runtime.Object, meta v1.Object) bool {
	kind := strings.ToLower(obj.GetObjectKind().GroupVersionKind().Kind)
	log := logger.WithValues("namespace", meta.GetNamespace(), kind, meta.GetName())
	if annotations.HasPausedAnnotation(meta) {
		log.V(4).Info("Resource is paused, will not attempt to map resource")
		return false
	}
	log.V(4).Info("Resource is not paused, will attempt to map resource")
	return true
}
