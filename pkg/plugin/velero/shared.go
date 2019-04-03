/*
Copyright 2019 the Velero contributors.

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

// Package velero contains the interfaces necessary to implement
// all of the Velero plugins. Users create their own binary containing
// implementations of the plugin kinds in this package. Multiple
// plugins of any type can be implemented.
package velero

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
