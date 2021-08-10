/*
Copyright the Velero contributors.

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
	"k8s.io/apimachinery/pkg/runtime"
	apiregistrationv1 "k8s.io/kube-aggregator/pkg/apis/apiregistration/v1"
	"k8s.io/kube-aggregator/pkg/controllers/autoregister"

	"github.com/vmware-tanzu/velero/pkg/plugin/velero"
)

type APIServiceAction struct {
	logger logrus.FieldLogger
}

// NewAPIServiceAction returns an APIServiceAction which is a RestoreItemAction plugin
// that will skip the restore of any APIServices which are managed by Kubernetes. This
// is determined by looking for the "kube-aggregator.kubernetes.io/automanaged" label on
// the APIService.
func NewAPIServiceAction(logger logrus.FieldLogger) *APIServiceAction {
	return &APIServiceAction{logger: logger}
}

func (a *APIServiceAction) AppliesTo() (velero.ResourceSelector, error) {
	return velero.ResourceSelector{
		IncludedResources: []string{"apiservices"},
	}, nil
}

func (a *APIServiceAction) Execute(input *velero.RestoreItemActionExecuteInput) (*velero.RestoreItemActionExecuteOutput, error) {
	apiService := new(apiregistrationv1.APIService)
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(input.Item.UnstructuredContent(), apiService); err != nil {
		return nil, errors.WithStack(err)
	}

	output := velero.NewRestoreItemActionExecuteOutput(input.Item)

	if _, ok := apiService.Labels[autoregister.AutoRegisterManagedLabel]; ok {
		output = output.WithoutRestore()
	}

	return output, nil
}
