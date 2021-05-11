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
	"strconv"

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
		appliedServiceUnstructured := new(map[string]interface{})
		if err := json.Unmarshal([]byte(lastAppliedConfig), appliedServiceUnstructured); err != nil {
			return errors.WithStack(err)
		}

		ports, bool, err := unstructured.NestedSlice(*appliedServiceUnstructured, "spec", "ports")

		if err != nil {
			return errors.WithStack(err)
		}

		if bool {
			for _, port := range ports {
				p, ok := port.(map[string]interface{})
				if !ok {
					continue
				}
				nodePort, nodePortBool, err := unstructured.NestedFieldNoCopy(p, "nodePort")
				if err != nil {
					return errors.WithStack(err)
				}
				if nodePortBool {
					nodePortInt := 0
					switch nodePort.(type) {
					case int32:
						nodePortInt = int(nodePort.(int32))
					case float64:
						nodePortInt = int(nodePort.(float64))
					case string:
						nodePortInt, err = strconv.Atoi(nodePort.(string))
						if err != nil {
							return errors.WithStack(err)
						}
					}
					if nodePortInt > 0 {
						portName, ok := p["name"]
						if !ok {
							// unnamed port
							explicitNodePorts.Insert("")
						} else {
							explicitNodePorts.Insert(portName.(string))
						}

					}
				}
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
