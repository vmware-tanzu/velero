/*
Copyright 2017 Heptio Inc.

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
	"compress/gzip"
	"encoding/json"
	"io"
	"reflect"
	"sort"
	"testing"
	"time"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/heptio/ark/pkg/apis/ark/v1"
	"github.com/heptio/ark/pkg/client"
	"github.com/heptio/ark/pkg/discovery"
	"github.com/heptio/ark/pkg/util/collections"
	kubeutil "github.com/heptio/ark/pkg/util/kube"
	arktest "github.com/heptio/ark/pkg/util/test"
)

var (
	trueVal      = true
	falseVal     = false
	truePointer  = &trueVal
	falsePointer = &falseVal
)

type fakeAction struct {
	ids             []string
	backups         []*v1.Backup
	additionalItems []ResourceIdentifier
}

var _ Action = &fakeAction{}

func (a *fakeAction) Execute(log *logrus.Entry, item runtime.Unstructured, backup *v1.Backup) ([]ResourceIdentifier, error) {
	metadata, err := meta.Accessor(item)
	if err != nil {
		return a.additionalItems, err
	}
	a.ids = append(a.ids, kubeutil.NamespaceAndName(metadata))
	a.backups = append(a.backups, backup)

	return a.additionalItems, nil
}

func TestResolveActions(t *testing.T) {
	tests := []struct {
		name                string
		input               map[string]Action
		expected            map[schema.GroupResource]Action
		resourcesWithErrors []string
		expectError         bool
	}{
		{
			name:     "empty input",
			input:    map[string]Action{},
			expected: map[schema.GroupResource]Action{},
		},
		{
			name:        "mapper error",
			input:       map[string]Action{"badresource": &fakeAction{}},
			expected:    map[schema.GroupResource]Action{},
			expectError: true,
		},
		{
			name:  "resolved",
			input: map[string]Action{"foo": &fakeAction{}, "bar": &fakeAction{}},
			expected: map[schema.GroupResource]Action{
				schema.GroupResource{Group: "somegroup", Resource: "foodies"}:      &fakeAction{},
				schema.GroupResource{Group: "anothergroup", Resource: "barnacles"}: &fakeAction{},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			resources := map[schema.GroupVersionResource]schema.GroupVersionResource{
				schema.GroupVersionResource{Resource: "foo"}: schema.GroupVersionResource{Group: "somegroup", Resource: "foodies"},
				schema.GroupVersionResource{Resource: "fie"}: schema.GroupVersionResource{Group: "somegroup", Resource: "fields"},
				schema.GroupVersionResource{Resource: "bar"}: schema.GroupVersionResource{Group: "anothergroup", Resource: "barnacles"},
				schema.GroupVersionResource{Resource: "baz"}: schema.GroupVersionResource{Group: "anothergroup", Resource: "bazaars"},
			}
			discoveryHelper := arktest.NewFakeDiscoveryHelper(false, resources)

			actual, err := resolveActions(discoveryHelper, test.input)
			gotError := err != nil

			if e, a := test.expectError, gotError; e != a {
				t.Fatalf("error: expected %t, got %t", e, a)
			}
			if test.expectError {
				return
			}

			assert.Equal(t, test.expected, actual)
		})
	}
}

func TestGetResourceIncludesExcludes(t *testing.T) {
	tests := []struct {
		name                string
		includes            []string
		excludes            []string
		resourcesWithErrors []string
		expectedIncludes    []string
		expectedExcludes    []string
	}{
		{
			name:             "no input",
			expectedIncludes: []string{},
			expectedExcludes: []string{},
		},
		{
			name:             "wildcard includes",
			includes:         []string{"*", "asdf"},
			excludes:         []string{},
			expectedIncludes: []string{"*"},
			expectedExcludes: []string{},
		},
		{
			name:             "wildcard excludes aren't allowed or resolved",
			includes:         []string{},
			excludes:         []string{"*"},
			expectedIncludes: []string{},
			expectedExcludes: []string{},
		},
		{
			name:             "resolution works",
			includes:         []string{"foo", "fie"},
			excludes:         []string{"bar", "baz"},
			expectedIncludes: []string{"foodies.somegroup", "fields.somegroup"},
			expectedExcludes: []string{"barnacles.anothergroup", "bazaars.anothergroup"},
		},
		{
			name:             "some unresolvable",
			includes:         []string{"foo", "fie", "bad1"},
			excludes:         []string{"bar", "baz", "bad2"},
			expectedIncludes: []string{"foodies.somegroup", "fields.somegroup"},
			expectedExcludes: []string{"barnacles.anothergroup", "bazaars.anothergroup"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			resources := map[schema.GroupVersionResource]schema.GroupVersionResource{
				schema.GroupVersionResource{Resource: "foo"}: schema.GroupVersionResource{Group: "somegroup", Resource: "foodies"},
				schema.GroupVersionResource{Resource: "fie"}: schema.GroupVersionResource{Group: "somegroup", Resource: "fields"},
				schema.GroupVersionResource{Resource: "bar"}: schema.GroupVersionResource{Group: "anothergroup", Resource: "barnacles"},
				schema.GroupVersionResource{Resource: "baz"}: schema.GroupVersionResource{Group: "anothergroup", Resource: "bazaars"},
			}
			discoveryHelper := arktest.NewFakeDiscoveryHelper(false, resources)

			actual := getResourceIncludesExcludes(discoveryHelper, test.includes, test.excludes)

			sort.Strings(test.expectedIncludes)
			actualIncludes := actual.GetIncludes()
			sort.Strings(actualIncludes)
			if e, a := test.expectedIncludes, actualIncludes; !reflect.DeepEqual(e, a) {
				t.Errorf("includes: expected %v, got %v", e, a)
			}

			sort.Strings(test.expectedExcludes)
			actualExcludes := actual.GetExcludes()
			sort.Strings(actualExcludes)
			if e, a := test.expectedExcludes, actualExcludes; !reflect.DeepEqual(e, a) {
				t.Errorf("excludes: expected %v, got %v", e, a)
				t.Errorf("excludes: expected %v, got %v", len(e), len(a))
			}
		})
	}
}

func TestGetNamespaceIncludesExcludes(t *testing.T) {
	backup := &v1.Backup{
		Spec: v1.BackupSpec{
			IncludedResources:  []string{"foo", "bar"},
			ExcludedResources:  []string{"fie", "baz"},
			IncludedNamespaces: []string{"a", "b", "c"},
			ExcludedNamespaces: []string{"d", "e", "f"},
			TTL:                metav1.Duration{Duration: 1 * time.Hour},
		},
	}

	ns := getNamespaceIncludesExcludes(backup)

	actualIncludes := ns.GetIncludes()
	sort.Strings(actualIncludes)
	if e, a := backup.Spec.IncludedNamespaces, actualIncludes; !reflect.DeepEqual(e, a) {
		t.Errorf("includes: expected %v, got %v", e, a)
	}

	actualExcludes := ns.GetExcludes()
	sort.Strings(actualExcludes)
	if e, a := backup.Spec.ExcludedNamespaces, actualExcludes; !reflect.DeepEqual(e, a) {
		t.Errorf("excludes: expected %v, got %v", e, a)
	}
}

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
		name                  string
		backup                *v1.Backup
		actions               map[string]Action
		expectedNamespaces    *collections.IncludesExcludes
		expectedResources     *collections.IncludesExcludes
		expectedLabelSelector string
		expectedHooks         []resourceHook
		backupGroupErrors     map[*metav1.APIResourceList]error
		expectedError         error
	}{
		{
			name: "happy path, no actions, no label selector, no hooks, no errors",
			backup: &v1.Backup{
				Spec: v1.BackupSpec{
					// cm - shortcut in legacy api group
					// csr - shortcut in certificates.k8s.io api group
					// roles - fully qualified in rbac.authorization.k8s.io api group
					IncludedResources:  []string{"cm", "csr", "roles"},
					IncludedNamespaces: []string{"a", "b"},
					ExcludedNamespaces: []string{"c", "d"},
				},
			},
			actions:            map[string]Action{},
			expectedNamespaces: collections.NewIncludesExcludes().Includes("a", "b").Excludes("c", "d"),
			expectedResources:  collections.NewIncludesExcludes().Includes("configmaps", "certificatesigningrequests.certificates.k8s.io", "roles.rbac.authorization.k8s.io"),
			expectedHooks:      []resourceHook{},
			backupGroupErrors: map[*metav1.APIResourceList]error{
				v1Group:           nil,
				certificatesGroup: nil,
				rbacGroup:         nil,
			},
		},
		{
			name: "label selector",
			backup: &v1.Backup{
				Spec: v1.BackupSpec{
					LabelSelector: &metav1.LabelSelector{
						MatchLabels: map[string]string{"a": "b"},
					},
				},
			},
			actions:               map[string]Action{},
			expectedNamespaces:    collections.NewIncludesExcludes(),
			expectedResources:     collections.NewIncludesExcludes(),
			expectedHooks:         []resourceHook{},
			expectedLabelSelector: "a=b",
			backupGroupErrors: map[*metav1.APIResourceList]error{
				v1Group:           nil,
				certificatesGroup: nil,
				rbacGroup:         nil,
			},
		},
		{
			name:               "backupGroup errors",
			backup:             &v1.Backup{},
			actions:            map[string]Action{},
			expectedNamespaces: collections.NewIncludesExcludes(),
			expectedResources:  collections.NewIncludesExcludes(),
			expectedHooks:      []resourceHook{},
			backupGroupErrors: map[*metav1.APIResourceList]error{
				v1Group:           errors.New("v1 error"),
				certificatesGroup: nil,
				rbacGroup:         errors.New("rbac error"),
			},
			expectedError: errors.New("[v1 error, rbac error]"),
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
								Hooks: []v1.BackupResourceHook{
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
			actions:            map[string]Action{},
			expectedNamespaces: collections.NewIncludesExcludes(),
			expectedResources:  collections.NewIncludesExcludes(),
			expectedHooks: []resourceHook{
				{
					name:          "hook1",
					namespaces:    collections.NewIncludesExcludes().Includes("a").Excludes("b"),
					resources:     collections.NewIncludesExcludes().Includes("configmaps").Excludes("roles.rbac.authorization.k8s.io"),
					labelSelector: parseLabelSelectorOrDie("1=2"),
					hooks: []v1.BackupResourceHook{
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
			discoveryHelper := &arktest.FakeDiscoveryHelper{
				Mapper: &arktest.FakeMapper{
					Resources: map[schema.GroupVersionResource]schema.GroupVersionResource{
						schema.GroupVersionResource{Resource: "cm"}:    schema.GroupVersionResource{Group: "", Version: "v1", Resource: "configmaps"},
						schema.GroupVersionResource{Resource: "csr"}:   schema.GroupVersionResource{Group: "certificates.k8s.io", Version: "v1beta1", Resource: "certificatesigningrequests"},
						schema.GroupVersionResource{Resource: "roles"}: schema.GroupVersionResource{Group: "rbac.authorization.k8s.io", Version: "v1beta1", Resource: "roles"},
					},
				},
				ResourceList: []*metav1.APIResourceList{
					v1Group,
					certificatesGroup,
					rbacGroup,
				},
			}

			dynamicFactory := &arktest.FakeDynamicFactory{}

			podCommandExecutor := &mockPodCommandExecutor{}
			defer podCommandExecutor.AssertExpectations(t)

			b, err := NewKubernetesBackupper(
				discoveryHelper,
				dynamicFactory,
				test.actions,
				podCommandExecutor,
			)
			require.NoError(t, err)
			kb := b.(*kubernetesBackupper)

			groupBackupperFactory := &mockGroupBackupperFactory{}
			defer groupBackupperFactory.AssertExpectations(t)
			kb.groupBackupperFactory = groupBackupperFactory

			groupBackupper := &mockGroupBackupper{}
			defer groupBackupper.AssertExpectations(t)

			cohabitatingResources := map[string]*cohabitatingResource{
				"deployments":     newCohabitatingResource("deployments", "extensions", "apps"),
				"networkpolicies": newCohabitatingResource("networkpolicies", "extensions", "networking.k8s.io"),
			}

			groupBackupperFactory.On("newGroupBackupper",
				mock.Anything, // log
				test.backup,
				test.expectedNamespaces,
				test.expectedResources,
				test.expectedLabelSelector,
				dynamicFactory,
				discoveryHelper,
				map[itemKey]struct{}{}, // backedUpItems
				cohabitatingResources,
				kb.actions,
				kb.podCommandExecutor,
				mock.Anything, // tarWriter
				test.expectedHooks,
			).Return(groupBackupper)

			for group, err := range test.backupGroupErrors {
				groupBackupper.On("backupGroup", group).Return(err)
			}

			var backupFile, logFile bytes.Buffer

			err = b.Backup(test.backup, &backupFile, &logFile)
			defer func() {
				// print log if anything failed
				if t.Failed() {
					gzr, err := gzip.NewReader(&logFile)
					require.NoError(t, err)
					t.Log("Backup log contents:")
					var buf bytes.Buffer
					_, err = io.Copy(&buf, gzr)
					require.NoError(t, err)
					require.NoError(t, gzr.Close())
					t.Log(buf.String())
				}
			}()

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
	log *logrus.Entry,
	backup *v1.Backup,
	namespaces, resources *collections.IncludesExcludes,
	labelSelector string,
	dynamicFactory client.DynamicFactory,
	discoveryHelper discovery.Helper,
	backedUpItems map[itemKey]struct{},
	cohabitatingResources map[string]*cohabitatingResource,
	actions map[schema.GroupResource]Action,
	podCommandExecutor podCommandExecutor,
	tarWriter tarWriter,
	resourceHooks []resourceHook,
) groupBackupper {
	args := f.Called(
		log,
		backup,
		namespaces,
		resources,
		labelSelector,
		dynamicFactory,
		discoveryHelper,
		backedUpItems,
		cohabitatingResources,
		actions,
		podCommandExecutor,
		tarWriter,
		resourceHooks,
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

/*
func TestBackupMethod(t *testing.T) {
	// TODO ensure LabelSelector is passed through to the List() calls
	backup := &v1.Backup{
		Spec: v1.BackupSpec{
			// cm - shortcut in legacy api group, namespaced
			// csr - shortcut in certificates.k8s.io api group, cluster-scoped
			// roles - fully qualified in rbac.authorization.k8s.io api group, namespaced
			IncludedResources:  []string{"cm", "csr", "roles"},
			IncludedNamespaces: []string{"a", "b"},
			ExcludedNamespaces: []string{"c", "d"},
		},
	}

	configMapsResource := metav1.APIResource{
		Name:         "configmaps",
		SingularName: "configmap",
		Namespaced:   true,
		Kind:         "ConfigMap",
		Verbs:        metav1.Verbs([]string{"create", "update", "get", "list", "watch", "delete"}),
		ShortNames:   []string{"cm"},
		Categories:   []string{"all"},
	}

	podsResource := metav1.APIResource{
		Name:         "pods",
		SingularName: "pod",
		Namespaced:   true,
		Kind:         "Pod",
		Verbs:        metav1.Verbs([]string{"create", "update", "get", "list", "watch", "delete"}),
		ShortNames:   []string{"po"},
		Categories:   []string{"all"},
	}

	rolesResource := metav1.APIResource{
		Name:         "roles",
		SingularName: "role",
		Namespaced:   true,
		Kind:         "Role",
		Verbs:        metav1.Verbs([]string{"create", "update", "get", "list", "watch", "delete"}),
	}

	certificateSigningRequestsResource := metav1.APIResource{
		Name:         "certificatesigningrequests",
		SingularName: "certificatesigningrequest",
		Namespaced:   false,
		Kind:         "CertificateSigningRequest",
		Verbs:        metav1.Verbs([]string{"create", "update", "get", "list", "watch", "delete"}),
		ShortNames:   []string{"csr"},
	}

	discoveryHelper := &arktest.FakeDiscoveryHelper{
		Mapper: &arktest.FakeMapper{
			Resources: map[schema.GroupVersionResource]schema.GroupVersionResource{
				schema.GroupVersionResource{Resource: "cm"}:    schema.GroupVersionResource{Group: "", Version: "v1", Resource: "configmaps"},
				schema.GroupVersionResource{Resource: "csr"}:   schema.GroupVersionResource{Group: "certificates.k8s.io", Version: "v1beta1", Resource: "certificatesigningrequests"},
				schema.GroupVersionResource{Resource: "roles"}: schema.GroupVersionResource{Group: "rbac.authorization.k8s.io", Version: "v1beta1", Resource: "roles"},
			},
		},
		ResourceList: []*metav1.APIResourceList{
			{
				GroupVersion: "v1",
				APIResources: []metav1.APIResource{configMapsResource, podsResource},
			},
			{
				GroupVersion: "certificates.k8s.io/v1beta1",
				APIResources: []metav1.APIResource{certificateSigningRequestsResource},
			},
			{
				GroupVersion: "rbac.authorization.k8s.io/v1beta1",
				APIResources: []metav1.APIResource{rolesResource},
			},
		},
	}

	dynamicFactory := &arktest.FakeDynamicFactory{}

	legacyGV := schema.GroupVersionResource{Version: "v1"}

	configMapsClientA := &arktest.FakeDynamicClient{}
	configMapsA := toRuntimeObject(t, `{
		"apiVersion": "v1",
		"kind": "ConfigMapList",
		"items": [
		  {
		  	"metadata": {
		  		"namespace":"a",
					"name":"configMap1"
				},
				"data": {
					"a": "b"
				}
			}
		]
	}`)
	configMapsClientA.On("List", metav1.ListOptions{}).Return(configMapsA, nil)
	dynamicFactory.On("ClientForGroupVersionResource", legacyGV, configMapsResource, "a").Return(configMapsClientA, nil)

	configMapsClientB := &arktest.FakeDynamicClient{}
	configMapsB := toRuntimeObject(t, `{
		"apiVersion": "v1",
		"kind": "ConfigMapList",
		"items": [
		  {
		  	"metadata": {
		  		"namespace":"b",
					"name":"configMap2"
				},
				"data": {
					"c": "d"
				}
			}
		]
	}`)
	configMapsClientB.On("List", metav1.ListOptions{}).Return(configMapsB, nil)
	dynamicFactory.On("ClientForGroupVersionResource", legacyGV, configMapsResource, "b").Return(configMapsClientB, nil)

	certificatesGV := schema.GroupVersionResource{Group: "certificates.k8s.io", Version: "v1beta1"}

	csrList := toRuntimeObject(t, `{
		"apiVersion": "certificates.k8s.io/v1beta1",
		"kind": "CertificateSigningRequestList",
		"items": [
			{
				"metadata": {
					"name": "csr1"
				},
				"spec": {
					"request": "some request",
					"username": "bob",
					"uid": "12345",
					"groups": [
						"group1",
						"group2"
					]
				},
				"status": {
					"certificate": "some cert"
				}
			}
		]
	}`)
	csrClient := &arktest.FakeDynamicClient{}
	csrClient.On("List", metav1.ListOptions{}).Return(csrList, nil)
	dynamicFactory.On("ClientForGroupVersionResource", certificatesGV, certificateSigningRequestsResource, "").Return(csrClient, nil)

	roleListA := toRuntimeObject(t, `{
		"apiVersion": "rbac.authorization.k8s.io/v1beta1",
		"kind": "RoleList",
		"items": [
			{
				"metadata": {
					"namespace": "a",
					"name": "role1"
				},
				"rules": [
					{
						"verbs": ["get","list"],
						"apiGroups": ["apps","extensions"],
						"resources": ["deployments"]
					}
				]
			}
		]
	}`)

	roleListB := toRuntimeObject(t, `{
		"apiVersion": "rbac.authorization.k8s.io/v1beta1",
		"kind": "RoleList",
		"items": []
	}`)

	rbacGV := schema.GroupVersionResource{Group: "rbac.authorization.k8s.io", Version: "v1beta1"}

	rolesClientA := &arktest.FakeDynamicClient{}
	rolesClientA.On("List", metav1.ListOptions{}).Return(roleListA, nil)
	dynamicFactory.On("ClientForGroupVersionResource", rbacGV, rolesResource, "a").Return(rolesClientA, nil)
	rolesClientB := &arktest.FakeDynamicClient{}
	rolesClientB.On("List", metav1.ListOptions{}).Return(roleListB, nil)
	dynamicFactory.On("ClientForGroupVersionResource", rbacGV, rolesResource, "b").Return(rolesClientB, nil)

	cmAction := &fakeAction{}
	csrAction := &fakeAction{}

	actions := map[string]Action{
		"cm":  cmAction,
		"csr": csrAction,
	}

	podCommandExecutor := &arktest.PodCommandExecutor{}
	defer podCommandExecutor.AssertExpectations(t)

	backupper, err := NewKubernetesBackupper(discoveryHelper, dynamicFactory, actions, podCommandExecutor)
	require.NoError(t, err)

	var output, log bytes.Buffer
	err = backupper.Backup(backup, &output, &log)
	defer func() {
		// print log if anything failed
		if t.Failed() {
			gzr, err := gzip.NewReader(&log)
			require.NoError(t, err)
			t.Log("Backup log contents:")
			var buf bytes.Buffer
			_, err = io.Copy(&buf, gzr)
			require.NoError(t, err)
			require.NoError(t, gzr.Close())
			t.Log(buf.String())
		}
	}()
	require.NoError(t, err)

	expectedFiles := sets.NewString(
		"resources/configmaps/namespaces/a/configMap1.json",
		"resources/configmaps/namespaces/b/configMap2.json",
		"resources/roles.rbac.authorization.k8s.io/namespaces/a/role1.json",
		// CSRs are not expected because they're unrelated cluster-scoped resources
	)

	expectedData := map[string]string{
		"resources/configmaps/namespaces/a/configMap1.json": `
			{
				"apiVersion": "v1",
				"kind": "ConfigMap",
				"metadata": {
					"namespace":"a",
					"name":"configMap1"
				},
				"data": {
					"a": "b"
				}
			}`,
		"resources/configmaps/namespaces/b/configMap2.json": `
			{
				"apiVersion": "v1",
				"kind": "ConfigMap",
				"metadata": {
					"namespace":"b",
					"name":"configMap2"
				},
				"data": {
					"c": "d"
				}
			}
		`,
		"resources/roles.rbac.authorization.k8s.io/namespaces/a/role1.json": `
			{
				"apiVersion": "rbac.authorization.k8s.io/v1beta1",
				"kind": "Role",
				"metadata": {
					"namespace":"a",
					"name": "role1"
				},
				"rules": [
					{
						"verbs": ["get","list"],
						"apiGroups": ["apps","extensions"],
						"resources": ["deployments"]
					}
				]
			}
		`,
		// CSRs are not expected because they're unrelated cluster-scoped resources
	}

	seenFiles := sets.NewString()

	gzipReader, err := gzip.NewReader(&output)
	require.NoError(t, err)
	defer gzipReader.Close()

	tarReader := tar.NewReader(gzipReader)
	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		require.NoError(t, err)

		switch header.Typeflag {
		case tar.TypeReg:
			seenFiles.Insert(header.Name)
			expected, err := getAsMap(expectedData[header.Name])
			if !assert.NoError(t, err, "%q: %v", header.Name, err) {
				continue
			}

			buf := new(bytes.Buffer)
			n, err := io.Copy(buf, tarReader)
			if !assert.NoError(t, err) {
				continue
			}

			if !assert.Equal(t, header.Size, n) {
				continue
			}

			actual, err := getAsMap(string(buf.Bytes()))
			if !assert.NoError(t, err) {
				continue
			}
			assert.Equal(t, expected, actual)
		default:
			t.Errorf("unexpected header: %#v", header)
		}
	}

	if !expectedFiles.Equal(seenFiles) {
		t.Errorf("did not get expected files. expected-seen: %v. seen-expected: %v", expectedFiles.Difference(seenFiles), seenFiles.Difference(expectedFiles))
	}

	expectedCMActionIDs := []string{"a/configMap1", "b/configMap2"}

	assert.Equal(t, expectedCMActionIDs, cmAction.ids)
	// CSRs are not expected because they're unrelated cluster-scoped resources
	assert.Nil(t, csrAction.ids)
}
*/

func getAsMap(j string) (map[string]interface{}, error) {
	m := make(map[string]interface{})
	err := json.Unmarshal([]byte(j), &m)
	return m, err
}

func toRuntimeObject(t *testing.T, data string) runtime.Object {
	o, _, err := unstructured.UnstructuredJSONScheme.Decode([]byte(data), nil, nil)
	require.NoError(t, err)
	return o
}

func unstructuredOrDie(data string) *unstructured.Unstructured {
	o, _, err := unstructured.UnstructuredJSONScheme.Decode([]byte(data), nil, nil)
	if err != nil {
		panic(err)
	}
	return o.(*unstructured.Unstructured)
}
