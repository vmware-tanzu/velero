/*
Copyright 2017 the Velero contributors.

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

package velero

import (
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"

	api "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
)

// BackupItemAction is an actor that performs an operation on an individual item being backed up.
type BackupItemAction interface {
	// AppliesTo returns information about which resources this action should be invoked for.
	// A BackupItemAction's Execute function will only be invoked on items that match the returned
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
