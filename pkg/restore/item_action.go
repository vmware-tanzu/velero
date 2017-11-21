/*
Copyright 2017 the Heptio Ark contributors.

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

package restore

import (
	"k8s.io/apimachinery/pkg/runtime"

	api "github.com/heptio/ark/pkg/apis/ark/v1"
)

// ItemAction is an actor that performs an operation on an individual item being restored.
type ItemAction interface {
	// AppliesTo returns information about which resources this action should be invoked for.
	AppliesTo() (ResourceSelector, error)

	// Execute allows the ItemAction to perform arbitrary logic with the item being restored.
	Execute(obj runtime.Unstructured, restore *api.Restore) (res runtime.Unstructured, warning error, err error)
}

// ResourceSelector is a collection of included/excluded namespaces,
// included/excluded resources, and a label-selector that can be used
// to match a set of items from a cluster.
type ResourceSelector struct {
	IncludedNamespaces []string
	ExcludedNamespaces []string
	IncludedResources  []string
	ExcludedResources  []string
	LabelSelector      string
}
