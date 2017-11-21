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
	"github.com/sirupsen/logrus"

	"k8s.io/apimachinery/pkg/runtime"

	api "github.com/heptio/ark/pkg/apis/ark/v1"
	"github.com/heptio/ark/pkg/util/collections"
)

type serviceAction struct {
	log logrus.FieldLogger
}

func NewServiceAction(log logrus.FieldLogger) ItemAction {
	return &serviceAction{
		log: log,
	}
}

func (a *serviceAction) AppliesTo() (ResourceSelector, error) {
	return ResourceSelector{
		IncludedResources: []string{"services"},
	}, nil
}

func (a *serviceAction) Execute(obj runtime.Unstructured, restore *api.Restore) (runtime.Unstructured, error, error) {
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
