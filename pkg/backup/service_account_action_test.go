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
	"fmt"
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	rbacclient "k8s.io/client-go/kubernetes/typed/rbac/v1"

	"github.com/heptio/ark/pkg/kuberesource"
	arktest "github.com/heptio/ark/pkg/util/test"
)

type fakeClusterRoleBindingClient struct {
	clusterRoleBindings []v1.ClusterRoleBinding

	rbacclient.ClusterRoleBindingInterface
}

func (c *fakeClusterRoleBindingClient) List(opts metav1.ListOptions) (*v1.ClusterRoleBindingList, error) {
	return &v1.ClusterRoleBindingList{
		Items: c.clusterRoleBindings,
	}, nil
}

func TestServiceAccountActionAppliesTo(t *testing.T) {
	a, _ := NewServiceAccountAction(arktest.NewLogger(), &fakeClusterRoleBindingClient{})

	actual, err := a.AppliesTo()
	require.NoError(t, err)

	expected := ResourceSelector{
		IncludedResources: []string{"serviceaccounts"},
	}
	assert.Equal(t, expected, actual)
}

func TestServiceAccountActionExecute(t *testing.T) {
	tests := []struct {
		name                    string
		serviceAccount          runtime.Unstructured
		crbs                    []v1.ClusterRoleBinding
		expectedAdditionalItems []ResourceIdentifier
	}{
		{
			name: "no crbs",
			serviceAccount: arktest.UnstructuredOrDie(`
			{
				"apiVersion": "v1",
				"kind": "ServiceAccount",
				"metadata": {
					"namespace": "heptio-ark",
					"name": "ark"
				}
			}
			`),
			crbs: nil,
			expectedAdditionalItems: nil,
		},
		{
			name: "no matching crbs",
			serviceAccount: arktest.UnstructuredOrDie(`
			{
				"apiVersion": "v1",
				"kind": "ServiceAccount",
				"metadata": {
					"namespace": "heptio-ark",
					"name": "ark"
				}
			}
			`),
			crbs: []v1.ClusterRoleBinding{
				{
					Subjects: []v1.Subject{
						{
							Kind:      "non-matching-kind",
							Namespace: "non-matching-ns",
							Name:      "non-matching-name",
						},
						{
							Kind:      "non-matching-kind",
							Namespace: "heptio-ark",
							Name:      "ark",
						},
						{
							Kind:      v1.ServiceAccountKind,
							Namespace: "non-matching-ns",
							Name:      "ark",
						},
						{
							Kind:      v1.ServiceAccountKind,
							Namespace: "heptio-ark",
							Name:      "non-matching-name",
						},
					},
					RoleRef: v1.RoleRef{
						Name: "role",
					},
				},
			},
			expectedAdditionalItems: nil,
		},
		{
			name: "some matching crbs",
			serviceAccount: arktest.UnstructuredOrDie(`
			{
				"apiVersion": "v1",
				"kind": "ServiceAccount",
				"metadata": {
					"namespace": "heptio-ark",
					"name": "ark"
				}
			}
			`),
			crbs: []v1.ClusterRoleBinding{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "crb-1",
					},
					Subjects: []v1.Subject{
						{
							Kind:      "non-matching-kind",
							Namespace: "non-matching-ns",
							Name:      "non-matching-name",
						},
					},
					RoleRef: v1.RoleRef{
						Name: "role-1",
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "crb-2",
					},
					Subjects: []v1.Subject{
						{
							Kind:      "non-matching-kind",
							Namespace: "non-matching-ns",
							Name:      "non-matching-name",
						},
						{
							Kind:      v1.ServiceAccountKind,
							Namespace: "heptio-ark",
							Name:      "ark",
						},
					},
					RoleRef: v1.RoleRef{
						Name: "role-2",
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "crb-3",
					},
					Subjects: []v1.Subject{
						{
							Kind:      v1.ServiceAccountKind,
							Namespace: "heptio-ark",
							Name:      "ark",
						},
					},
					RoleRef: v1.RoleRef{
						Name: "role-3",
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "crb-4",
					},
					Subjects: []v1.Subject{
						{
							Kind:      v1.ServiceAccountKind,
							Namespace: "heptio-ark",
							Name:      "ark",
						},
						{
							Kind:      "non-matching-kind",
							Namespace: "non-matching-ns",
							Name:      "non-matching-name",
						},
					},
					RoleRef: v1.RoleRef{
						Name: "role-4",
					},
				},
			},
			expectedAdditionalItems: []ResourceIdentifier{
				{
					GroupResource: kuberesource.ClusterRoleBindings,
					Name:          "crb-2",
				},
				{
					GroupResource: kuberesource.ClusterRoleBindings,
					Name:          "crb-3",
				},
				{
					GroupResource: kuberesource.ClusterRoleBindings,
					Name:          "crb-4",
				},
				{
					GroupResource: kuberesource.ClusterRoles,
					Name:          "role-2",
				},
				{
					GroupResource: kuberesource.ClusterRoles,
					Name:          "role-3",
				},
				{
					GroupResource: kuberesource.ClusterRoles,
					Name:          "role-4",
				},
			},
		},
	}

	crbClient := &fakeClusterRoleBindingClient{}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			crbClient.clusterRoleBindings = test.crbs

			action, err := NewServiceAccountAction(arktest.NewLogger(), crbClient)
			require.Nil(t, err)

			res, additional, err := action.Execute(test.serviceAccount, nil)

			assert.Equal(t, test.serviceAccount, res)
			assert.Nil(t, err)

			// ensure slices are ordered for valid comparison
			sort.Slice(test.expectedAdditionalItems, func(i, j int) bool {
				return fmt.Sprintf("%s.%s", test.expectedAdditionalItems[i].GroupResource.String(), test.expectedAdditionalItems[i].Name) <
					fmt.Sprintf("%s.%s", test.expectedAdditionalItems[j].GroupResource.String(), test.expectedAdditionalItems[j].Name)
			})

			sort.Slice(additional, func(i, j int) bool {
				return fmt.Sprintf("%s.%s", additional[i].GroupResource.String(), additional[i].Name) <
					fmt.Sprintf("%s.%s", additional[j].GroupResource.String(), additional[j].Name)
			})

			assert.Equal(t, test.expectedAdditionalItems, additional)
		})
	}

}
