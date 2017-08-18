/*
Copyright 2017 Heptio Inc.

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

package restorers

import (
	"k8s.io/apimachinery/pkg/runtime"

	api "github.com/heptio/ark/pkg/apis/ark/v1"
	"github.com/heptio/ark/pkg/util/collections"
)

// ResourceRestorer exposes the operations necessary to prepare Kubernetes resources
// for restore and confirm their readiness following restoration via Ark.
type ResourceRestorer interface {
	// Handles returns true if the Restorer should restore this object.
	Handles(obj runtime.Unstructured, restore *api.Restore) bool

	// Prepare gets an item ready to be restored.
	Prepare(obj runtime.Unstructured, restore *api.Restore, backup *api.Backup) (res runtime.Unstructured, warning error, err error)

	// Wait returns true if restoration should wait for all of this restorer's resources to be ready before moving on to the next restorer.
	Wait() bool

	// Ready returns true if the given item is considered ready by the system. Only used if Wait() returns true.
	Ready(obj runtime.Unstructured) bool
}

func resetMetadataAndStatus(obj runtime.Unstructured, keepAnnotations bool) (runtime.Unstructured, error) {
	metadata, err := collections.GetMap(obj.UnstructuredContent(), "metadata")
	if err != nil {
		return nil, err
	}

	for k := range metadata {
		if k != "name" && k != "namespace" && k != "labels" && (!keepAnnotations || k != "annotations") {
			delete(metadata, k)
		}
	}

	delete(obj.UnstructuredContent(), "status")

	return obj, nil
}

var _ ResourceRestorer = &basicRestorer{}

type basicRestorer struct {
	saveAnnotations bool
}

func (br *basicRestorer) Handles(obj runtime.Unstructured, restore *api.Restore) bool {
	return true
}

func (br *basicRestorer) Prepare(obj runtime.Unstructured, restore *api.Restore, backup *api.Backup) (runtime.Unstructured, error, error) {
	obj, err := resetMetadataAndStatus(obj, br.saveAnnotations)

	return obj, err, nil
}

func (br *basicRestorer) Wait() bool {
	return false
}

func (br *basicRestorer) Ready(obj runtime.Unstructured) bool {
	return true
}

func NewBasicRestorer(saveAnnotations bool) ResourceRestorer {
	return &basicRestorer{saveAnnotations: saveAnnotations}
}
