/*
Copyright 2018 the Heptio Ark contributors.

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
	rbacclient "k8s.io/client-go/kubernetes/typed/rbac/v1"

	"github.com/heptio/ark/pkg/apis/ark/v1"
	"github.com/heptio/ark/pkg/kuberesource"
)

// serviceAccountAction implements ItemAction.
type serviceAccountAction struct {
	log                 logrus.FieldLogger
	clusterRoleBindings []rbac.ClusterRoleBinding
}

// NewServiceAccountAction creates a new ItemAction for service accounts.
func NewServiceAccountAction(log logrus.FieldLogger, client rbacclient.ClusterRoleBindingInterface) (ItemAction, error) {
	clusterRoleBindings, err := client.List(metav1.ListOptions{})
	if err != nil {
		return nil, errors.WithStack(err)
	}

	return &serviceAccountAction{
		log:                 log,
		clusterRoleBindings: clusterRoleBindings.Items,
	}, nil
}

// AppliesTo returns a ResourceSelector that applies only to service accounts.
func (a *serviceAccountAction) AppliesTo() (ResourceSelector, error) {
	return ResourceSelector{
		IncludedResources: []string{"serviceaccounts"},
	}, nil
}

// Execute checks for any ClusterRoleBindings that have this service account as a subject, and
// adds the ClusterRoleBinding and associated ClusterRole to the list of additional items to
// be backed up.
func (a *serviceAccountAction) Execute(item runtime.Unstructured, backup *v1.Backup) (runtime.Unstructured, []ResourceIdentifier, error) {
	a.log.Info("Running serviceAccountAction")
	defer a.log.Info("Done running serviceAccountAction")

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

	for _, clusterRoleBinding := range a.clusterRoleBindings {
		for _, subj := range clusterRoleBinding.Subjects {
			if subj.Kind == rbac.ServiceAccountKind && subj.Namespace == namespace && subj.Name == name {
				a.log.Infof("Adding clusterrole %s and clusterrolebinding %s to additionalItems since serviceaccount %s/%s is a subject",
					clusterRoleBinding.RoleRef.Name, clusterRoleBinding.Name, namespace, name)

				bindings.Insert(clusterRoleBinding.Name)
				roles.Insert(clusterRoleBinding.RoleRef.Name)
				break
			}
		}
	}

	var additionalItems []ResourceIdentifier
	for binding := range bindings {
		additionalItems = append(additionalItems, ResourceIdentifier{
			GroupResource: kuberesource.ClusterRoleBindings,
			Name:          binding,
		})
	}

	for role := range roles {
		additionalItems = append(additionalItems, ResourceIdentifier{
			GroupResource: kuberesource.ClusterRoles,
			Name:          role,
		})
	}

	return item, additionalItems, nil
}
