/*
Copyright 2018 the Velero contributors.

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

package backup

import (
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	rbac "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/sets"

	v1 "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	velerodiscovery "github.com/vmware-tanzu/velero/pkg/discovery"
	"github.com/vmware-tanzu/velero/pkg/kuberesource"
	"github.com/vmware-tanzu/velero/pkg/plugin/velero"
)

// ServiceAccountAction implements ItemAction.
type ServiceAccountAction struct {
	log                 logrus.FieldLogger
	clusterRoleBindings []ClusterRoleBinding
}

// NewServiceAccountAction creates a new ItemAction for service accounts.
func NewServiceAccountAction(logger logrus.FieldLogger, clusterRoleBindingListers map[string]ClusterRoleBindingLister, discoveryHelper velerodiscovery.Helper) (*ServiceAccountAction, error) {
	// Look up the supported RBAC version
	var supportedAPI metav1.GroupVersionForDiscovery
	for _, ag := range discoveryHelper.APIGroups() {
		if ag.Name == rbac.GroupName {
			supportedAPI = ag.PreferredVersion
			break
		}
	}

	crbLister := clusterRoleBindingListers[supportedAPI.Version]

	// This should be safe because the List call will return a 0-item slice
	// if there's no matching API version.
	crbs, err := crbLister.List()
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

// Execute checks for any ClusterRoleBindings that have this service account as a subject, and
// adds the ClusterRoleBinding and associated ClusterRole to the list of additional items to
// be backed up.
func (a *ServiceAccountAction) Execute(item runtime.Unstructured, backup *v1.Backup) (runtime.Unstructured, []velero.ResourceIdentifier, error) {
	a.log.Info("Running ServiceAccountAction")
	defer a.log.Info("Done running ServiceAccountAction")

	objectMeta, err := meta.Accessor(item)
	if err != nil {
		return nil, nil, errors.WithStack(err)
	}

	var (
		namespace = objectMeta.GetNamespace()
		name      = objectMeta.GetName()
		bindings  = sets.NewString()
		roles     = sets.NewString()
	)

	for _, crb := range a.clusterRoleBindings {
		for _, s := range crb.ServiceAccountSubjects(namespace) {
			if s == name {
				a.log.Infof("Adding clusterrole %s and clusterrolebinding %s to additionalItems since serviceaccount %s/%s is a subject",
					crb.RoleRefName(), crb.Name(), namespace, name)

				bindings.Insert(crb.Name())
				roles.Insert(crb.RoleRefName())
				break
			}
		}
	}

	var additionalItems []velero.ResourceIdentifier
	for binding := range bindings {
		additionalItems = append(additionalItems, velero.ResourceIdentifier{
			GroupResource: kuberesource.ClusterRoleBindings,
			Name:          binding,
		})
	}

	for role := range roles {
		additionalItems = append(additionalItems, velero.ResourceIdentifier{
			GroupResource: kuberesource.ClusterRoles,
			Name:          role,
		})
	}

	return item, additionalItems, nil
}
