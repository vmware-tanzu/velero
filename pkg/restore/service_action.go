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
	"encoding/json"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"

	corev1api "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"

	api "github.com/heptio/ark/pkg/apis/ark/v1"
	"github.com/heptio/ark/pkg/util/collections"
)

const annotationLastAppliedConfig = "kubectl.kubernetes.io/last-applied-configuration"

type serviceAction struct {
	log logrus.FieldLogger
}

func NewServiceAction(logger logrus.FieldLogger) ItemAction {
	return &serviceAction{log: logger}
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

	preservedPorts, err := getPreservedPorts(obj)
	if err != nil {
		return nil, nil, err
	}

	ports, err := collections.GetSlice(obj.UnstructuredContent(), "spec.ports")
	if err != nil {
		return nil, nil, err
	}

	for _, port := range ports {
		p := port.(map[string]interface{})
		var name string
		if nameVal, ok := p["name"]; ok {
			name = nameVal.(string)
		}
		if preservedPorts[name] {
			continue
		}
		delete(p, "nodePort")
	}

	return obj, nil, nil
}

func getPreservedPorts(obj runtime.Unstructured) (map[string]bool, error) {
	preservedPorts := map[string]bool{}
	metadata, err := meta.Accessor(obj)
	if err != nil {
		return nil, errors.WithStack(err)
	}
	if lac, ok := metadata.GetAnnotations()[annotationLastAppliedConfig]; ok {
		var svc corev1api.Service
		if err := json.Unmarshal([]byte(lac), &svc); err != nil {
			return nil, errors.WithStack(err)
		}
		for _, port := range svc.Spec.Ports {
			if port.NodePort > 0 {
				preservedPorts[port.Name] = true
			}
		}
	}
	return preservedPorts, nil
}
