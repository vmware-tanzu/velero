/*
Copyright 2022 the Velero contributors.

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
	corev1api "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/vmware-tanzu/velero/pkg/plugin/velero"
)

// NamespaceAction handle namespace mapping
type NamespaceAction struct {
	logger logrus.FieldLogger
}

func NewNamespaceAction(logger logrus.FieldLogger) *NamespaceAction {
	return &NamespaceAction{logger: logger}
}

func (a *NamespaceAction) AppliesTo() (velero.ResourceSelector, error) {
	return velero.ResourceSelector{
		IncludedResources: []string{"namespaces"},
	}, nil
}

func (a *NamespaceAction) Execute(input *velero.RestoreItemActionExecuteInput) (*velero.RestoreItemActionExecuteOutput, error) {
	namespaceMapping := input.Restore.Spec.NamespaceMapping
	if len(namespaceMapping) == 0 {
		return velero.NewRestoreItemActionExecuteOutput(&unstructured.Unstructured{Object: input.Item.UnstructuredContent()}), nil
	}

	namespace := new(corev1api.Namespace)
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(input.Item.UnstructuredContent(), namespace); err != nil {
		return nil, errors.WithStack(err)
	}

	if newNamespace, ok := namespaceMapping[namespace.Name]; ok {
		namespace.SetName(newNamespace)
	}

	res, err := runtime.DefaultUnstructuredConverter.ToUnstructured(namespace)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	return velero.NewRestoreItemActionExecuteOutput(&unstructured.Unstructured{Object: res}), nil
}
