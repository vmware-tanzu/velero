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

package actions

import (
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	networkapi "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/runtime"

	v1 "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	"github.com/vmware-tanzu/velero/pkg/kuberesource"
	"github.com/vmware-tanzu/velero/pkg/plugin/velero"
)

// IngressAction implements ItemAction.
type IngressAction struct {
	log logrus.FieldLogger
}

// NewIngressAction creates a new ItemAction for pods.
func NewIngressAction(logger logrus.FieldLogger) *IngressAction {
	return &IngressAction{log: logger}
}

// AppliesTo returns a ResourceSelector that applies only to pods.
func (a *IngressAction) AppliesTo() (velero.ResourceSelector, error) {
	return velero.ResourceSelector{
		IncludedResources: []string{"ingresses"},
	}, nil
}

// Execute scans the ingress's spec.ingressClassName for ingressClass and returns a
// ResourceIdentifier list containing references to the ingressClass used by
// the ingress. This ensures that when a ingress is backed up, related ingressClass is backed up too.
func (a *IngressAction) Execute(item runtime.Unstructured, backup *v1.Backup) (runtime.Unstructured, []velero.ResourceIdentifier, error) {
	a.log.Info("Executing ingressAction")
	defer a.log.Info("Done executing ingressAction")

	ing := new(networkapi.Ingress)
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(item.UnstructuredContent(), ing); err != nil {
		return nil, nil, errors.WithStack(err)
	}

	var additionalItems []velero.ResourceIdentifier
	if ing.Spec.IngressClassName != nil && *ing.Spec.IngressClassName != "" {
		a.log.Infof("Adding IngressClass %s to additionalItems", ing.Spec.IngressClassName)
		additionalItems = append(additionalItems, velero.ResourceIdentifier{
			GroupResource: kuberesource.IngressClasses,
			Name:          *ing.Spec.IngressClassName,
		})
	}

	return item, additionalItems, nil
}
