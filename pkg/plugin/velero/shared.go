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

// Package velero contains the interfaces necessary to implement
// all of the Velero plugins. Users create their own binary containing
// implementations of the plugin kinds in this package. Multiple
// plugins of any type can be implemented.
package velero

import (
	"time"

	"k8s.io/apimachinery/pkg/runtime/schema"
)

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
	// IncludedResources is a slice of resources to match. Resources may be specified
	// as full names (e.g. "services"), abbreviations (e.g. "svc"), or with the
	// groups they are in (e.g. "ingresses.extensions"). All resources in this slice,
	// except those in ExcludedResources, will be matched. A nil/empty slice matches
	// all resources.
	IncludedResources []string
	// ExcludedResources is a slice of resources to exclude. Resources may be specified
	// as full names (e.g. "services"), abbreviations (e.g. "svc"), or with the
	// groups they are in (e.g. "ingresses.extensions"). All resources in IncludedResources,
	// *except* those in this slice, will be matched.
	ExcludedResources []string
	// LabelSelector is a string representation of a selector to apply
	// when matching resources. See "k8s.io/apimachinery/pkg/labels".Parse()
	// for details on syntax.
	LabelSelector string
}

// Applicable allows actions and plugins to specify which resources they should be invoked for
type Applicable interface {
	// AppliesTo returns information about which resources this Responder should be invoked for.
	AppliesTo() (ResourceSelector, error)
}

// ResourceIdentifier describes a single item by its group, resource, namespace, and name.
type ResourceIdentifier struct {
	schema.GroupResource
	Namespace string
	Name      string
}

func (in *ResourceIdentifier) DeepCopy() *ResourceIdentifier {
	if in == nil {
		return nil
	}
	out := new(ResourceIdentifier)
	in.DeepCopyInto(out)
	return out
}

func (in *ResourceIdentifier) DeepCopyInto(out *ResourceIdentifier) {
	*out = *in
	out.GroupResource = in.GroupResource
}

// OperationProgress describes progress of an asynchronous plugin operation.
type OperationProgress struct {
	// True when the operation has completed, either successfully or with a failure
	Completed bool
	// Set when the operation has failed
	Err string
	// Quantity completed so far and the total quantity associated with the operation
	// in OperationUnits. For data mover and volume snapshotter use cases, this will
	// usually be in bytes. On successful completion, NCompleted and NTotal should be
	// the same
	NCompleted, NTotal int64
	// Units represented by NCompleted and NTotal -- for data mover and item
	// snapshotters, this will usually be bytes.
	OperationUnits string
	// Optional description of operation progress (i.e. "Current phase: Running")
	Description string
	// When the operation was started and when the last update was seen.  Not all
	// systems retain when the upload was begun, return Time 0 (time.Unix(0, 0))
	// if unknown.
	Started, Updated time.Time
}
