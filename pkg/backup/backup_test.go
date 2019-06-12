/*
Copyright 2017, 2019 the Velero contributors.

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
	"bytes"
	"testing"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"

	v1 "github.com/heptio/velero/pkg/apis/velero/v1"
	"github.com/heptio/velero/pkg/client"
	"github.com/heptio/velero/pkg/discovery"
	"github.com/heptio/velero/pkg/plugin/velero"
	"github.com/heptio/velero/pkg/podexec"
	"github.com/heptio/velero/pkg/restic"
	"github.com/heptio/velero/pkg/util/collections"
	"github.com/heptio/velero/pkg/util/logging"
	velerotest "github.com/heptio/velero/pkg/util/test"
)

var (
	v1Group = &metav1.APIResourceList{
		GroupVersion: "v1",
		APIResources: []metav1.APIResource{configMapsResource, podsResource, namespacesResource},
	}

	configMapsResource = metav1.APIResource{
		Name:         "configmaps",
		SingularName: "configmap",
		Namespaced:   true,
		Kind:         "ConfigMap",
		Verbs:        metav1.Verbs([]string{"create", "update", "get", "list", "watch", "delete"}),
		ShortNames:   []string{"cm"},
		Categories:   []string{"all"},
	}

	podsResource = metav1.APIResource{
		Name:         "pods",
		SingularName: "pod",
		Namespaced:   true,
		Kind:         "Pod",
		Verbs:        metav1.Verbs([]string{"create", "update", "get", "list", "watch", "delete"}),
		ShortNames:   []string{"po"},
		Categories:   []string{"all"},
	}

	rbacGroup = &metav1.APIResourceList{
		GroupVersion: "rbac.authorization.k8s.io/v1beta1",
		APIResources: []metav1.APIResource{rolesResource},
	}

	rolesResource = metav1.APIResource{
		Name:         "roles",
		SingularName: "role",
		Namespaced:   true,
		Kind:         "Role",
		Verbs:        metav1.Verbs([]string{"create", "update", "get", "list", "watch", "delete"}),
	}

	namespacesResource = metav1.APIResource{
		Name:         "namespaces",
		SingularName: "namespace",
		Namespaced:   false,
		Kind:         "Namespace",
		Verbs:        metav1.Verbs([]string{"create", "update", "get", "list", "watch", "delete"}),
	}

	certificatesGroup = &metav1.APIResourceList{
		GroupVersion: "certificates.k8s.io/v1beta1",
		APIResources: []metav1.APIResource{certificateSigningRequestsResource},
	}

	certificateSigningRequestsResource = metav1.APIResource{
		Name:         "certificatesigningrequests",
		SingularName: "certificatesigningrequest",
		Namespaced:   false,
		Kind:         "CertificateSigningRequest",
		Verbs:        metav1.Verbs([]string{"create", "update", "get", "list", "watch", "delete"}),
		ShortNames:   []string{"csr"},
	}

	extensionsGroup = &metav1.APIResourceList{
		GroupVersion: "extensions/v1beta1",
		APIResources: []metav1.APIResource{deploymentsResource, networkPoliciesResource},
	}

	extensionsGroupVersion = schema.GroupVersion{
		Group:   "extensions",
		Version: "v1beta1",
	}

	appsGroup = &metav1.APIResourceList{
		GroupVersion: "apps/v1beta1",
		APIResources: []metav1.APIResource{deploymentsResource},
	}

	appsGroupVersion = schema.GroupVersion{
		Group:   "apps",
		Version: "v1beta1",
	}

	deploymentsResource = metav1.APIResource{
		Name:         "deployments",
		SingularName: "deployment",
		Namespaced:   true,
		Kind:         "Deployment",
		Verbs:        metav1.Verbs([]string{"create", "update", "get", "list", "watch", "delete"}),
		ShortNames:   []string{"deploy"},
		Categories:   []string{"all"},
	}

	networkingGroup = &metav1.APIResourceList{
		GroupVersion: "networking.k8s.io/v1",
		APIResources: []metav1.APIResource{networkPoliciesResource},
	}

	networkingGroupVersion = schema.GroupVersion{
		Group:   "networking.k8s.io",
		Version: "v1",
	}

	networkPoliciesResource = metav1.APIResource{
		Name:         "networkpolicies",
		SingularName: "networkpolicy",
		Namespaced:   true,
		Kind:         "Deployment",
		Verbs:        metav1.Verbs([]string{"create", "update", "get", "list", "watch", "delete"}),
	}
)

func parseLabelSelectorOrDie(s string) labels.Selector {
	ret, err := labels.Parse(s)
	if err != nil {
		panic(err)
	}
	return ret
}

func TestBackup(t *testing.T) {
	tests := []struct {
		name               string
		backup             *v1.Backup
		actions            []velero.BackupItemAction
		expectedNamespaces *collections.IncludesExcludes
		expectedResources  *collections.IncludesExcludes
		expectedHooks      []resourceHook
		backupGroupErrors  map[*metav1.APIResourceList]error
		expectedError      error
	}{
		{
			name: "error resolving hooks returns an error",
			backup: &v1.Backup{
				Spec: v1.BackupSpec{
					// cm - shortcut in legacy api group
					// csr - shortcut in certificates.k8s.io api group
					// roles - fully qualified in rbac.authorization.k8s.io api group
					IncludedResources:  []string{"cm", "csr", "roles"},
					IncludedNamespaces: []string{"a", "b"},
					ExcludedNamespaces: []string{"c", "d"},
					Hooks: v1.BackupHooks{
						Resources: []v1.BackupResourceHookSpec{
							{
								Name: "hook-with-invalid-label-selector",
								LabelSelector: &metav1.LabelSelector{
									MatchExpressions: []metav1.LabelSelectorRequirement{
										{
											Key:      "foo",
											Operator: metav1.LabelSelectorOperator("nonexistent-operator"),
											Values:   []string{"bar"},
										},
									},
								},
							},
						},
					},
				},
			},
			expectedNamespaces: collections.NewIncludesExcludes().Includes("a", "b").Excludes("c", "d"),
			expectedResources:  collections.NewIncludesExcludes().Includes("configmaps", "certificatesigningrequests.certificates.k8s.io", "roles.rbac.authorization.k8s.io"),
			expectedHooks:      []resourceHook{},
			expectedError:      errors.New("\"nonexistent-operator\" is not a valid pod selector operator"),
		},
		{
			name:               "backupGroup errors",
			backup:             &v1.Backup{},
			expectedNamespaces: collections.NewIncludesExcludes(),
			expectedResources:  collections.NewIncludesExcludes(),
			expectedHooks:      []resourceHook{},
			backupGroupErrors: map[*metav1.APIResourceList]error{
				v1Group:           errors.New("v1 error"),
				certificatesGroup: nil,
				rbacGroup:         errors.New("rbac error"),
			},
			expectedError: nil,
		},
		{
			name: "hooks",
			backup: &v1.Backup{
				Spec: v1.BackupSpec{
					Hooks: v1.BackupHooks{
						Resources: []v1.BackupResourceHookSpec{
							{
								Name:               "hook1",
								IncludedNamespaces: []string{"a"},
								ExcludedNamespaces: []string{"b"},
								IncludedResources:  []string{"cm"},
								ExcludedResources:  []string{"roles"},
								LabelSelector: &metav1.LabelSelector{
									MatchLabels: map[string]string{"1": "2"},
								},
								PreHooks: []v1.BackupResourceHook{
									{
										Exec: &v1.ExecHook{
											Command: []string{"ls", "/tmp"},
										},
									},
								},
							},
						},
					},
				},
			},
			expectedNamespaces: collections.NewIncludesExcludes(),
			expectedResources:  collections.NewIncludesExcludes(),
			expectedHooks: []resourceHook{
				{
					name:          "hook1",
					namespaces:    collections.NewIncludesExcludes().Includes("a").Excludes("b"),
					resources:     collections.NewIncludesExcludes().Includes("configmaps").Excludes("roles.rbac.authorization.k8s.io"),
					labelSelector: parseLabelSelectorOrDie("1=2"),
					pre: []v1.BackupResourceHook{
						{
							Exec: &v1.ExecHook{
								Command: []string{"ls", "/tmp"},
							},
						},
					},
				},
			},
			backupGroupErrors: map[*metav1.APIResourceList]error{
				v1Group:           nil,
				certificatesGroup: nil,
				rbacGroup:         nil,
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			req := &Request{
				Backup: test.backup,
			}

			discoveryHelper := &velerotest.FakeDiscoveryHelper{
				Mapper: &velerotest.FakeMapper{
					Resources: map[schema.GroupVersionResource]schema.GroupVersionResource{
						{Resource: "cm"}:    {Group: "", Version: "v1", Resource: "configmaps"},
						{Resource: "csr"}:   {Group: "certificates.k8s.io", Version: "v1beta1", Resource: "certificatesigningrequests"},
						{Resource: "roles"}: {Group: "rbac.authorization.k8s.io", Version: "v1beta1", Resource: "roles"},
					},
				},
				ResourceList: []*metav1.APIResourceList{
					v1Group,
					certificatesGroup,
					rbacGroup,
				},
			}

			dynamicFactory := new(velerotest.FakeDynamicFactory)

			podCommandExecutor := &velerotest.MockPodCommandExecutor{}
			defer podCommandExecutor.AssertExpectations(t)

			groupBackupperFactory := &mockGroupBackupperFactory{}
			defer groupBackupperFactory.AssertExpectations(t)

			groupBackupper := &mockGroupBackupper{}
			defer groupBackupper.AssertExpectations(t)

			groupBackupperFactory.On("newGroupBackupper",
				mock.Anything, // log
				req,
				dynamicFactory,
				discoveryHelper,
				map[itemKey]struct{}{}, // backedUpItems
				cohabitatingResources(),
				podCommandExecutor,
				mock.Anything, // tarWriter
				mock.Anything, // restic backupper
				mock.Anything, // pvc snapshot tracker
				mock.Anything, // volume snapshotter getter
			).Maybe().Return(groupBackupper)

			for group, err := range test.backupGroupErrors {
				groupBackupper.On("backupGroup", group).Return(err)
			}

			kb := &kubernetesBackupper{
				discoveryHelper:       discoveryHelper,
				dynamicFactory:        dynamicFactory,
				podCommandExecutor:    podCommandExecutor,
				groupBackupperFactory: groupBackupperFactory,
			}

			err := kb.Backup(logging.DefaultLogger(logrus.DebugLevel), req, new(bytes.Buffer), test.actions, nil)

			assert.Equal(t, test.expectedNamespaces, req.NamespaceIncludesExcludes)
			assert.Equal(t, test.expectedResources, req.ResourceIncludesExcludes)
			assert.Equal(t, test.expectedHooks, req.ResourceHooks)

			if test.expectedError != nil {
				assert.EqualError(t, err, test.expectedError.Error())
				return
			}
			assert.NoError(t, err)

		})
	}
}

type mockGroupBackupperFactory struct {
	mock.Mock
}

func (f *mockGroupBackupperFactory) newGroupBackupper(
	log logrus.FieldLogger,
	backup *Request,
	dynamicFactory client.DynamicFactory,
	discoveryHelper discovery.Helper,
	backedUpItems map[itemKey]struct{},
	cohabitatingResources map[string]*cohabitatingResource,
	podCommandExecutor podexec.PodCommandExecutor,
	tarWriter tarWriter,
	resticBackupper restic.Backupper,
	resticSnapshotTracker *pvcSnapshotTracker,
	volumeSnapshotterGetter VolumeSnapshotterGetter,
) groupBackupper {
	args := f.Called(
		log,
		backup,
		dynamicFactory,
		discoveryHelper,
		backedUpItems,
		cohabitatingResources,
		podCommandExecutor,
		tarWriter,
		resticBackupper,
		resticSnapshotTracker,
		volumeSnapshotterGetter,
	)
	return args.Get(0).(groupBackupper)
}

type mockGroupBackupper struct {
	mock.Mock
}

func (gb *mockGroupBackupper) backupGroup(group *metav1.APIResourceList) error {
	args := gb.Called(group)
	return args.Error(0)
}

func toRuntimeObject(t *testing.T, data string) runtime.Object {
	o, _, err := unstructured.UnstructuredJSONScheme.Decode([]byte(data), nil, nil)
	require.NoError(t, err)
	return o
}

func TestGetResourceHook(t *testing.T) {
	tests := []struct {
		name     string
		hookSpec v1.BackupResourceHookSpec
		expected resourceHook
	}{
		{
			name: "Full test",
			hookSpec: v1.BackupResourceHookSpec{
				Name:               "spec1",
				IncludedNamespaces: []string{"ns1", "ns2"},
				ExcludedNamespaces: []string{"ns3", "ns4"},
				IncludedResources:  []string{"foo", "fie"},
				ExcludedResources:  []string{"bar", "baz"},
				PreHooks: []v1.BackupResourceHook{
					{
						Exec: &v1.ExecHook{
							Container: "a",
							Command:   []string{"b"},
						},
					},
				},
				PostHooks: []v1.BackupResourceHook{
					{
						Exec: &v1.ExecHook{
							Container: "c",
							Command:   []string{"d"},
						},
					},
				},
			},
			expected: resourceHook{
				name:       "spec1",
				namespaces: collections.NewIncludesExcludes().Includes("ns1", "ns2").Excludes("ns3", "ns4"),
				resources:  collections.NewIncludesExcludes().Includes("foodies.somegroup", "fields.somegroup").Excludes("barnacles.anothergroup", "bazaars.anothergroup"),
				pre: []v1.BackupResourceHook{
					{
						Exec: &v1.ExecHook{
							Container: "a",
							Command:   []string{"b"},
						},
					},
				},
				post: []v1.BackupResourceHook{
					{
						Exec: &v1.ExecHook{
							Container: "c",
							Command:   []string{"d"},
						},
					},
				},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			resources := map[schema.GroupVersionResource]schema.GroupVersionResource{
				{Resource: "foo"}: {Group: "somegroup", Resource: "foodies"},
				{Resource: "fie"}: {Group: "somegroup", Resource: "fields"},
				{Resource: "bar"}: {Group: "anothergroup", Resource: "barnacles"},
				{Resource: "baz"}: {Group: "anothergroup", Resource: "bazaars"},
			}
			discoveryHelper := velerotest.NewFakeDiscoveryHelper(false, resources)

			actual, err := getResourceHook(test.hookSpec, discoveryHelper)
			require.NoError(t, err)
			assert.Equal(t, test.expected, actual)
		})
	}
}
