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
	"reflect"

	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

// SpecChangePredicate implements a default update predicate function on Spec change
// As Velero doesn't enable subresource in CRDs, we cannot use the object's metadata.generation field to check the spec change
// More details about the generation field refer to https://github.com/kubernetes-sigs/controller-runtime/blob/v0.12.2/pkg/predicate/predicate.go#L156
type SpecChangePredicate struct {
	predicate.Funcs
}

func (SpecChangePredicate) Update(e event.UpdateEvent) bool {
	if e.ObjectOld == nil {
		return false
	}
	if e.ObjectNew == nil {
		return false
	}
	oldSpec := reflect.ValueOf(e.ObjectOld).Elem().FieldByName("Spec")
	// contains no field named "Spec", return false directly
	if oldSpec.IsZero() {
		return false
	}
	newSpec := reflect.ValueOf(e.ObjectNew).Elem().FieldByName("Spec")
	return !reflect.DeepEqual(oldSpec.Interface(), newSpec.Interface())
}
