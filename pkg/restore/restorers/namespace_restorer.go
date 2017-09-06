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

type namespaceRestorer struct{}

var _ ResourceRestorer = &namespaceRestorer{}

func NewNamespaceRestorer() ResourceRestorer {
	return &namespaceRestorer{}
}

func (nsr *namespaceRestorer) Handles(obj runtime.Unstructured, restore *api.Restore) bool {
	nsName, err := collections.GetString(obj.UnstructuredContent(), "metadata.name")
	if err != nil {
		return false
	}

	return collections.NewIncludesExcludes().
		Includes(restore.Spec.IncludedNamespaces...).
		Excludes(restore.Spec.ExcludedNamespaces...).
		ShouldInclude(nsName)
}

func (nsr *namespaceRestorer) Prepare(obj runtime.Unstructured, restore *api.Restore, backup *api.Backup) (runtime.Unstructured, error, error) {
	updated, err := resetMetadataAndStatus(obj, true)
	if err != nil {
		return nil, nil, err
	}

	metadata, err := collections.GetMap(obj.UnstructuredContent(), "metadata")
	if err != nil {
		return nil, nil, err
	}

	currentName, err := collections.GetString(obj.UnstructuredContent(), "metadata.name")
	if err != nil {
		return nil, nil, err
	}

	if newName, mapped := restore.Spec.NamespaceMapping[currentName]; mapped {
		metadata["name"] = newName
	}

	return updated, nil, nil
}

func (nsr *namespaceRestorer) Wait() bool {
	return false
}

func (nsr *namespaceRestorer) Ready(obj runtime.Unstructured) bool {
	return true
}
