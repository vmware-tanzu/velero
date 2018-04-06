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
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"

	api "github.com/heptio/ark/pkg/apis/ark/v1"
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
	// Since clusterIP is an optional key, we can ignore 'not found' errors.
	if val, _ := unstructured.NestedString(obj.UnstructuredContent(), "spec", "clusterIP"); val != "None" {
		unstructured.RemoveNestedField(obj.UnstructuredContent(), "spec", "clusterIP")
	}

	ports, found := unstructured.NestedSlice(obj.UnstructuredContent(), "spec", "ports")
	if !found {
		return nil, nil, errors.New("unable to find spec.ports")
	}

	for _, port := range ports {
		p := port.(map[string]interface{})
		delete(p, "nodePort")
	}

	if !unstructured.SetNestedSlice(obj.UnstructuredContent(), ports, "spec", "ports") {
		return nil, nil, errors.New("unable to set spec.ports")
	}

	return obj, nil, nil
}
