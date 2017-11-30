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

package backup

import (
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"

	api "github.com/heptio/ark/pkg/apis/ark/v1"
)

// ItemAction is an actor that performs an operation on an individual item being backed up.
type ItemAction interface {
	// AppliesTo returns information about which resources this action should be invoked for.
	// An ItemAction's Execute function will only be invoked on items that match the returned
	// selector. A zero-valued ResourceSelector matches all resources.
	AppliesTo() (ResourceSelector, error)

	// Execute allows the ItemAction to perform arbitrary logic with the item being backed up,
	// including mutating the item itself prior to backup. The item (unmodified or modified)
	// should be returned, along with an optional slice of ResourceIdentifiers specifying
	// additional related items that should be backed up.
	Execute(item runtime.Unstructured, backup *api.Backup) (runtime.Unstructured, []ResourceIdentifier, error)
}

// ResourceIdentifier describes a single item by its group, resource, namespace, and name.
type ResourceIdentifier struct {
	schema.GroupResource
	Namespace string
	Name      string
}

// ResourceSelector is a collection of included/excluded namespaces,
// included/excluded resources, and a label-selector that can be used
// to match a set of items from a cluster.
type ResourceSelector struct {
	// IncludedNamespaces is a slice of namespace names to match. All
	// namespaces in this slice, except those in ExcludedNamespaces,
	// will be matched. A nil/empty slice matches all namespaces.
	IncludedNamespaces []string
	// ExcludedNamespaces is a slice of namespace names to exclude.
	// All namespaces in IncludedNamespaces, *except* those in
	// this slice, will be matched.
	ExcludedNamespaces []string
	// IncludedResources is a slice of resources to match. Resources
	// may be specified as full names (e.g. "services") or abbreviations
	// (e.g. "svc"). All resources in this slice, except those in
	// ExcludedResources, will be matched. A nil/empty slice matches
	// all resources.
	IncludedResources []string
	// ExcludedResources is a slice of resources to exclude.
	// Resources may be specified as full names (e.g. "services") or
	// abbreviations (e.g. "svc"). All resources in IncludedResources,
	// *except* those in this slice, will be matched.
	ExcludedResources []string
	// LabelSelector is a string representation of a selector to apply
	// when matching resources. See "k8s.io/apimachinery/pkg/labels".Parse()
	// for details on syntax.
	LabelSelector string
}
