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

package restore

import (
	"encoding/json"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	corev1api "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/sets"

	"github.com/vmware-tanzu/velero/pkg/plugin/velero"
	"github.com/vmware-tanzu/velero/pkg/util/boolptr"
)

const annotationLastAppliedConfig = "kubectl.kubernetes.io/last-applied-configuration"

type ServiceAction struct {
	log logrus.FieldLogger
}

func NewServiceAction(logger logrus.FieldLogger) *ServiceAction {
	return &ServiceAction{log: logger}
}

func (a *ServiceAction) AppliesTo() (velero.ResourceSelector, error) {
	return velero.ResourceSelector{
		IncludedResources: []string{"services"},
	}, nil
}

func (a *ServiceAction) Execute(input *velero.RestoreItemActionExecuteInput) (*velero.RestoreItemActionExecuteOutput, error) {
	service := new(corev1api.Service)
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(input.Item.UnstructuredContent(), service); err != nil {
		return nil, errors.WithStack(err)
	}

	if service.Spec.ClusterIP != "None" {
		service.Spec.ClusterIP = ""
	}

	/* Do not delete NodePorts if restore triggered with "--preserve-nodeports" flag */
	if boolptr.IsSetToTrue(input.Restore.Spec.PreserveNodePorts) {
		a.log.Info("Restoring Services with original NodePort(s)")
	} else {
		if err := deleteNodePorts(service); err != nil {
			return nil, err
		}
	}

	res, err := runtime.DefaultUnstructuredConverter.ToUnstructured(service)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	return velero.NewRestoreItemActionExecuteOutput(&unstructured.Unstructured{Object: res}), nil
}

func deleteNodePorts(service *corev1api.Service) error {
	if service.Spec.Type == corev1api.ServiceTypeExternalName {
		return nil
	}

	// find any NodePorts whose values were explicitly specified according
	// to the last-applied-config annotation. We'll retain these values, and
	// clear out any other (presumably auto-assigned) NodePort values.
	explicitNodePorts := sets.NewString()
	lastAppliedConfig, ok := service.Annotations[annotationLastAppliedConfig]
	if ok {
		appliedService := new(corev1api.Service)
		if err := json.Unmarshal([]byte(lastAppliedConfig), appliedService); err != nil {
			return errors.WithStack(err)
		}

		for _, port := range appliedService.Spec.Ports {
			if port.NodePort > 0 {
				explicitNodePorts.Insert(port.Name)
			}
		}
	}

	for i, port := range service.Spec.Ports {
		if !explicitNodePorts.Has(port.Name) {
			service.Spec.Ports[i].NodePort = 0
		}
	}

	return nil
}
