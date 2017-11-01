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

type serviceRestorer struct{}

var _ ResourceRestorer = &serviceRestorer{}

func NewServiceRestorer() ResourceRestorer {
	return &serviceRestorer{}
}

func (sr *serviceRestorer) Handles(obj runtime.Unstructured, restore *api.Restore) bool {
	return true
}

func (sr *serviceRestorer) Prepare(obj runtime.Unstructured, restore *api.Restore, backup *api.Backup) (runtime.Unstructured, error, error) {
	if _, err := resetMetadataAndStatus(obj, true); err != nil {
		return nil, nil, err
	}

	spec, err := collections.GetMap(obj.UnstructuredContent(), "spec")
	if err != nil {
		return nil, nil, err
	}

	// Since clusterIP is an optional key, we can ignore 'not found' errors. Also assuming it was a string already.
	if val, _ := collections.GetString(spec, "clusterIP"); val != "None" {
		delete(spec, "clusterIP")
	}

	ports, err := collections.GetSlice(obj.UnstructuredContent(), "spec.ports")
	if err != nil {
		return nil, nil, err
	}

	for _, port := range ports {
		p := port.(map[string]interface{})
		delete(p, "nodePort")
	}

	return obj, nil, nil
}

func (sr *serviceRestorer) Wait() bool {
	return false
}

func (sr *serviceRestorer) Ready(obj runtime.Unstructured) bool {
	return true
}
