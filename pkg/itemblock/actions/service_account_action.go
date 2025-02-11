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
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"

	v1 "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	velerodiscovery "github.com/vmware-tanzu/velero/pkg/discovery"
	"github.com/vmware-tanzu/velero/pkg/plugin/velero"
	"github.com/vmware-tanzu/velero/pkg/util/actionhelpers"
)

// ServiceAccountAction implements ItemBlockAction.
type ServiceAccountAction struct {
	log                 logrus.FieldLogger
	clusterRoleBindings []actionhelpers.ClusterRoleBinding
}

// NewServiceAccountAction creates a new ItemBlockAction for service accounts.
func NewServiceAccountAction(logger logrus.FieldLogger, clusterRoleBindingListers map[string]actionhelpers.ClusterRoleBindingLister, discoveryHelper velerodiscovery.Helper) (*ServiceAccountAction, error) {
	crbs, err := actionhelpers.ClusterRoleBindingsForAction(clusterRoleBindingListers, discoveryHelper)
	if err != nil {
		return nil, err
	}

	return &ServiceAccountAction{
		log:                 logger,
		clusterRoleBindings: crbs,
	}, nil
}

// AppliesTo returns a ResourceSelector that applies only to service accounts.
func (a *ServiceAccountAction) AppliesTo() (velero.ResourceSelector, error) {
	return velero.ResourceSelector{
		IncludedResources: []string{"serviceaccounts"},
	}, nil
}

// GetRelatedItems checks for any ClusterRoleBindings that have this service account as a subject, and
// returns the ClusterRoleBinding and associated ClusterRole.
func (a *ServiceAccountAction) GetRelatedItems(item runtime.Unstructured, _ *v1.Backup) ([]velero.ResourceIdentifier, error) {
	a.log.Info("Running ServiceAccount ItemBlockAction")
	defer a.log.Info("Done running ServiceAccount ItemBlockAction")

	objectMeta, err := meta.Accessor(item)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	return actionhelpers.RelatedItemsForServiceAccount(objectMeta, a.clusterRoleBindings, a.log), nil
}

func (a *ServiceAccountAction) Name() string {
	return "ServiceAccountItemBlockAction"
}
