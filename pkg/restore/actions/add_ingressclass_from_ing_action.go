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

package actions

import (
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	networkapi "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/vmware-tanzu/velero/pkg/kuberesource"
	"github.com/vmware-tanzu/velero/pkg/plugin/velero"
)

type AddIngressClassFromIngAction struct {
	logger logrus.FieldLogger
}

func NewAddIngressClassFromIngAction(logger logrus.FieldLogger) *AddIngressClassFromIngAction {
	return &AddIngressClassFromIngAction{logger: logger}
}

func (a *AddIngressClassFromIngAction) AppliesTo() (velero.ResourceSelector, error) {
	return velero.ResourceSelector{
		IncludedResources: []string{"ingresses"},
	}, nil
}

func (a *AddIngressClassFromIngAction) Execute(input *velero.RestoreItemActionExecuteInput) (*velero.RestoreItemActionExecuteOutput, error) {
	a.logger.Info("Executing AddIngressClassFromIngAction")

	var ing networkapi.Ingress
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(input.Item.UnstructuredContent(), &ing); err != nil {
		return nil, errors.Wrap(err, "unable to convert unstructured item to ingress")
	}

	var additionalItems []velero.ResourceIdentifier

	if ing.Spec.IngressClassName != nil && *ing.Spec.IngressClassName != "" {
		a.logger.Infof("Adding IngressClass %s as an additional item to restore", *ing.Spec.IngressClassName)
		additionalItems = append(additionalItems, velero.ResourceIdentifier{
			GroupResource: kuberesource.IngressClasses,
			Name:          *ing.Spec.IngressClassName,
		})
	}
	return &velero.RestoreItemActionExecuteOutput{
		UpdatedItem:     input.Item,
		AdditionalItems: additionalItems,
	}, nil
}
