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
	"archive/tar"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"errors"
	"io"
	"io/ioutil"
	"reflect"
	"sort"
	"testing"
	"time"

	testlogger "github.com/sirupsen/logrus/hooks/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/sets"

	"github.com/heptio/ark/pkg/apis/ark/v1"
	"github.com/heptio/ark/pkg/util/collections"
	. "github.com/heptio/ark/pkg/util/test"
)

var (
	trueVal      = true
	falseVal     = false
	truePointer  = &trueVal
	falsePointer = &falseVal
)

type fakeAction struct {
	ids     []string
	backups []*v1.Backup
}

var _ Action = &fakeAction{}

func (a *fakeAction) Execute(ctx *backupContext, item map[string]interface{}, backupper itemBackupper) error {
	metadata, err := collections.GetMap(item, "metadata")
	if err != nil {
		return err
	}

	var id string

	if v, ok := metadata["namespace"]; ok {
		id = v.(string) + "/"
	}

	if v, ok := metadata["name"]; ok {
		id += v.(string)
	}

	a.ids = append(a.ids, id)
	a.backups = append(a.backups, ctx.backup)

	return nil
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
			discoveryHelper := NewFakeDiscoveryHelper(false, resources)

			actual, err := resolveActions(discoveryHelper, test.input)
			gotError := err != nil

			if e, a := test.expectError, gotError; e != a {
				t.Fatalf("error: expected %t, got %t", e, a)
			}
			if test.expectError {
				return
			}

			if e, a := test.expected, actual; !reflect.DeepEqual(e, a) {
				t.Errorf("expected %v, got %v", e, a)
			}
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
			discoveryHelper := NewFakeDiscoveryHelper(false, resources)

			log, _ := testlogger.NewNullLogger()

			ctx := &backupContext{
				logger: log,
			}
			actual := ctx.getResourceIncludesExcludes(discoveryHelper, test.includes, test.excludes)

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

	discoveryHelper := &FakeDiscoveryHelper{
		Mapper: &FakeMapper{
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

	dynamicFactory := &FakeDynamicFactory{}

	legacyGV := schema.GroupVersionResource{Version: "v1"}

	configMapsClientA := &FakeDynamicClient{}
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

	configMapsClientB := &FakeDynamicClient{}
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
	csrClient := &FakeDynamicClient{}
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

	rolesClientA := &FakeDynamicClient{}
	rolesClientA.On("List", metav1.ListOptions{}).Return(roleListA, nil)
	dynamicFactory.On("ClientForGroupVersionResource", rbacGV, rolesResource, "a").Return(rolesClientA, nil)
	rolesClientB := &FakeDynamicClient{}
	rolesClientB.On("List", metav1.ListOptions{}).Return(roleListB, nil)
	dynamicFactory.On("ClientForGroupVersionResource", rbacGV, rolesResource, "b").Return(rolesClientB, nil)

	cmAction := &fakeAction{}
	csrAction := &fakeAction{}

	actions := map[string]Action{
		"cm":  cmAction,
		"csr": csrAction,
	}

	backupper, err := NewKubernetesBackupper(discoveryHelper, dynamicFactory, actions)
	require.NoError(t, err)

	output := new(bytes.Buffer)
	err = backupper.Backup(backup, output, ioutil.Discard)
	require.NoError(t, err)

	expectedFiles := sets.NewString(
		"namespaces/a/configmaps/configMap1.json",
		"namespaces/b/configmaps/configMap2.json",
		"namespaces/a/roles.rbac.authorization.k8s.io/role1.json",
		// CSRs are not expected because they're unrelated cluster-scoped resources
	)

	expectedData := map[string]string{
		"namespaces/a/configmaps/configMap1.json": `
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
		"namespaces/b/configmaps/configMap2.json": `
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
		"namespaces/a/roles.rbac.authorization.k8s.io/role1.json": `
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

	gzipReader, err := gzip.NewReader(output)
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

func TestBackupResource(t *testing.T) {
	tests := []struct {
		name                            string
		resourceIncludesExcludes        *collections.IncludesExcludes
		resourceGroup                   string
		resourceVersion                 string
		resourceGV                      string
		resourceName                    string
		resourceNamespaced              bool
		namespaceIncludesExcludes       *collections.IncludesExcludes
		expectedListedNamespaces        []string
		lists                           []string
		labelSelector                   string
		actions                         map[string]Action
		expectedActionIDs               map[string][]string
		deploymentsBackedUp             bool
		expectedDeploymentsBackedUp     bool
		networkPoliciesBackedUp         bool
		expectedNetworkPoliciesBackedUp bool
		includeClusterResources         *bool
	}{
		{
			name: "should not include resource",
			resourceIncludesExcludes: collections.NewIncludesExcludes().Includes("pods"),
			resourceGV:               "v1",
			resourceName:             "secrets",
			resourceNamespaced:       true,
		},
		{
			name: "should skip deployments.extensions if we've seen deployments.apps",
			resourceIncludesExcludes:    collections.NewIncludesExcludes().Includes("*"),
			resourceGV:                  "extensions/v1beta1",
			resourceName:                "deployments",
			resourceNamespaced:          true,
			deploymentsBackedUp:         true,
			expectedDeploymentsBackedUp: true,
		},
		{
			name: "should skip deployments.apps if we've seen deployments.extensions",
			resourceIncludesExcludes:    collections.NewIncludesExcludes().Includes("*"),
			resourceGV:                  "apps/v1beta1",
			resourceName:                "deployments",
			resourceNamespaced:          true,
			deploymentsBackedUp:         true,
			expectedDeploymentsBackedUp: true,
		},
		{
			name: "should skip networkpolicies.extensions if we've seen networkpolicies.networking.k8s.io",
			resourceIncludesExcludes:        collections.NewIncludesExcludes().Includes("*"),
			resourceGV:                      "extensions/v1beta1",
			resourceName:                    "networkpolicies",
			resourceNamespaced:              true,
			networkPoliciesBackedUp:         true,
			expectedNetworkPoliciesBackedUp: true,
		},
		{
			name: "should skip networkpolicies.networking.k8s.io if we've seen networkpolicies.extensions",
			resourceIncludesExcludes:        collections.NewIncludesExcludes().Includes("*"),
			resourceGV:                      "networking.k8s.io/v1",
			resourceName:                    "networkpolicies",
			resourceNamespaced:              true,
			networkPoliciesBackedUp:         true,
			expectedNetworkPoliciesBackedUp: true,
		},
		{
			name: "list per namespace when not including *",
			resourceIncludesExcludes:  collections.NewIncludesExcludes().Includes("*"),
			resourceGroup:             "apps",
			resourceVersion:           "v1beta1",
			resourceGV:                "apps/v1beta1",
			resourceName:              "deployments",
			resourceNamespaced:        true,
			namespaceIncludesExcludes: collections.NewIncludesExcludes().Includes("a", "b"),
			expectedListedNamespaces:  []string{"a", "b"},
			lists: []string{
				`{
			"apiVersion": "apps/v1beta1",
			"kind": "DeploymentList",
			"items": [
				{
					"metadata": {
						"namespace": "a",
						"name": "1"
					}
				}
			]
		}`,
				`{
			"apiVersion": "apps/v1beta1v1",
			"kind": "DeploymentList",
			"items": [
				{
					"metadata": {
						"namespace": "b",
						"name": "2"
					}
				}
			]
		}`,
			},
			expectedDeploymentsBackedUp: true,
		},
		{
			name: "list all namespaces when including *",
			resourceIncludesExcludes:  collections.NewIncludesExcludes().Includes("*"),
			resourceGroup:             "networking.k8s.io",
			resourceVersion:           "v1",
			resourceGV:                "networking.k8s.io/v1",
			resourceName:              "networkpolicies",
			resourceNamespaced:        true,
			namespaceIncludesExcludes: collections.NewIncludesExcludes().Includes("*"),
			expectedListedNamespaces:  []string{""},
			lists: []string{
				`{
			"apiVersion": "networking.k8s.io/v1",
			"kind": "NetworkPolicyList",
			"items": [
				{
					"metadata": {
						"namespace": "a",
						"name": "1"
					}
				}
			]
		}`,
			},
			expectedNetworkPoliciesBackedUp: true,
		},
		{
			name: "list all namespaces when cluster-scoped, even with namespace includes",
			resourceIncludesExcludes:  collections.NewIncludesExcludes().Includes("*"),
			resourceGroup:             "certificates.k8s.io",
			resourceVersion:           "v1beta1",
			resourceGV:                "certificates.k8s.io/v1beta1",
			resourceName:              "certificatesigningrequests",
			resourceNamespaced:        false,
			namespaceIncludesExcludes: collections.NewIncludesExcludes().Includes("a"),
			expectedListedNamespaces:  []string{""},
			labelSelector:             "a=b",
			lists: []string{
				`{
			"apiVersion": "certifiaces.k8s.io/v1beta1",
			"kind": "CertificateSigningRequestList",
			"items": [
				{
					"metadata": {
						"name": "1",
						"labels": {
							"a": "b"
						}
					}
				}
			]
		}`,
			},
		},
		{
			name: "use a custom action",
			resourceIncludesExcludes:  collections.NewIncludesExcludes().Includes("*"),
			resourceGroup:             "certificates.k8s.io",
			resourceVersion:           "v1beta1",
			resourceGV:                "certificates.k8s.io/v1beta1",
			resourceName:              "certificatesigningrequests",
			resourceNamespaced:        false,
			namespaceIncludesExcludes: collections.NewIncludesExcludes().Includes("a"),
			expectedListedNamespaces:  []string{""},
			labelSelector:             "a=b",
			lists: []string{
				`{
	"apiVersion": "certificates.k8s.io/v1beta1",
	"kind": "CertificateSigningRequestList",
	"items": [
		{
			"metadata": {
				"name": "1",
				"labels": {
					"a": "b"
				}
			}
		}
	]
}`,
			},
			actions: map[string]Action{
				"certificatesigningrequests": &fakeAction{},
				"other": &fakeAction{},
			},
			expectedActionIDs: map[string][]string{
				"certificatesigningrequests": {"1"},
			},
		},
		{
			name: "should include cluster-scoped resource if backing up subset of namespaces and --include-cluster-resources=true",
			resourceIncludesExcludes:  collections.NewIncludesExcludes().Includes("*"),
			resourceGroup:             "foogroup",
			resourceVersion:           "v1",
			resourceGV:                "foogroup/v1",
			resourceName:              "bars",
			resourceNamespaced:        false,
			namespaceIncludesExcludes: collections.NewIncludesExcludes().Includes("ns-1"),
			includeClusterResources:   truePointer,
			lists: []string{
				`{
			"apiVersion": "foogroup/v1",
			"kind": "BarList",
			"items": [
				{
					"metadata": {
						"namespace": "",
						"name": "1"
					}
				}
			]
		}`,
			},
			expectedListedNamespaces: []string{""},
		},
		{
			name: "should not include cluster-scoped resource if backing up subset of namespaces and --include-cluster-resources=false",
			resourceIncludesExcludes:  collections.NewIncludesExcludes().Includes("*"),
			resourceGroup:             "foogroup",
			resourceVersion:           "v1",
			resourceGV:                "foogroup/v1",
			resourceName:              "bars",
			resourceNamespaced:        false,
			namespaceIncludesExcludes: collections.NewIncludesExcludes().Includes("ns-1"),
			includeClusterResources:   falsePointer,
		},
		{
			name: "should not include cluster-scoped resource if backing up subset of namespaces and --include-cluster-resources=<nil>",
			resourceIncludesExcludes:  collections.NewIncludesExcludes().Includes("*"),
			resourceGroup:             "foogroup",
			resourceVersion:           "v1",
			resourceGV:                "foogroup/v1",
			resourceName:              "bars",
			resourceNamespaced:        false,
			namespaceIncludesExcludes: collections.NewIncludesExcludes().Includes("ns-1"),
			includeClusterResources:   nil,
		},
		{
			name: "should include cluster-scoped resources if backing up all namespaces and --include-cluster-resources=true",
			resourceIncludesExcludes:  collections.NewIncludesExcludes().Includes("*"),
			resourceGroup:             "foogroup",
			resourceVersion:           "v1",
			resourceGV:                "foogroup/v1",
			resourceName:              "bars",
			resourceNamespaced:        false,
			namespaceIncludesExcludes: collections.NewIncludesExcludes().Includes("*"),
			includeClusterResources:   truePointer,
			lists: []string{
				`{
			"apiVersion": "foogroup/v1",
			"kind": "BarList",
			"items": [
				{
					"metadata": {
						"namespace": "",
						"name": "1"
					}
				}
			]
		}`,
			},
			expectedListedNamespaces: []string{""},
		},
		{
			name: "should not include cluster-scoped resource if backing up all namespaces and --include-cluster-resources=false",
			resourceIncludesExcludes:  collections.NewIncludesExcludes().Includes("*"),
			resourceGroup:             "foogroup",
			resourceVersion:           "v1",
			resourceGV:                "foogroup/v1",
			resourceName:              "bars",
			resourceNamespaced:        false,
			namespaceIncludesExcludes: collections.NewIncludesExcludes().Includes("*"),
			includeClusterResources:   falsePointer,
		},
		{
			name: "should include cluster-scoped resource if backing up all namespaces and --include-cluster-resources=<nil>",
			resourceIncludesExcludes:  collections.NewIncludesExcludes().Includes("*"),
			resourceGroup:             "foogroup",
			resourceVersion:           "v1",
			resourceGV:                "foogroup/v1",
			resourceName:              "bars",
			resourceNamespaced:        false,
			namespaceIncludesExcludes: collections.NewIncludesExcludes().Includes("*"),
			includeClusterResources:   nil,
			lists: []string{
				`{
			"apiVersion": "foogroup/v1",
			"kind": "BarList",
			"items": [
				{
					"metadata": {
						"namespace": "",
						"name": "1"
					}
				}
			]
		}`,
			},
			expectedListedNamespaces: []string{""},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var labelSelector *metav1.LabelSelector
			if test.labelSelector != "" {
				s, err := metav1.ParseToLabelSelector(test.labelSelector)
				require.NoError(t, err)
				labelSelector = s
			}

			log, _ := testlogger.NewNullLogger()

			ctx := &backupContext{
				backup: &v1.Backup{
					Spec: v1.BackupSpec{
						LabelSelector:           labelSelector,
						IncludeClusterResources: test.includeClusterResources,
					},
				},
				resourceIncludesExcludes:  test.resourceIncludesExcludes,
				namespaceIncludesExcludes: test.namespaceIncludesExcludes,
				deploymentsBackedUp:       test.deploymentsBackedUp,
				networkPoliciesBackedUp:   test.networkPoliciesBackedUp,
				logger:                    log,
			}

			group := &metav1.APIResourceList{
				GroupVersion: test.resourceGV,
			}

			resource := metav1.APIResource{Name: test.resourceName, Namespaced: test.resourceNamespaced}

			itemBackupper := &fakeItemBackupper{}

			var actualActionIDs map[string][]string

			dynamicFactory := &FakeDynamicFactory{}
			gvr := schema.GroupVersionResource{Group: test.resourceGroup, Version: test.resourceVersion}
			gr := schema.GroupResource{Group: test.resourceGroup, Resource: test.resourceName}
			for i, namespace := range test.expectedListedNamespaces {
				obj := toRuntimeObject(t, test.lists[i])

				client := &FakeDynamicClient{}
				client.On("List", metav1.ListOptions{LabelSelector: test.labelSelector}).Return(obj, nil)
				dynamicFactory.On("ClientForGroupVersionResource", gvr, resource, namespace).Return(client, nil)

				action := test.actions[test.resourceName]

				list, err := meta.ExtractList(obj)
				require.NoError(t, err)
				for i := range list {
					item := list[i].(*unstructured.Unstructured)
					itemBackupper.On("backupItem", ctx, item.Object, gr).Return(nil)
					if action != nil {
						a, err := meta.Accessor(item)
						require.NoError(t, err)
						ns := a.GetNamespace()
						name := a.GetName()
						id := ns
						if id != "" {
							id += "/"
						}
						id += name
						if actualActionIDs == nil {
							actualActionIDs = make(map[string][]string)
						}
						actualActionIDs[test.resourceName] = append(actualActionIDs[test.resourceName], id)
					}
				}
			}

			resources := map[schema.GroupVersionResource]schema.GroupVersionResource{
				schema.GroupVersionResource{Resource: "certificatesigningrequests"}: schema.GroupVersionResource{Group: "certificates.k8s.io", Version: "v1beta1", Resource: "certificatesigningrequests"},
				schema.GroupVersionResource{Resource: "other"}:                      schema.GroupVersionResource{Group: "somegroup", Version: "someversion", Resource: "otherthings"},
			}
			discoveryHelper := NewFakeDiscoveryHelper(false, resources)

			kb, err := NewKubernetesBackupper(discoveryHelper, dynamicFactory, test.actions)
			require.NoError(t, err)
			backupper := kb.(*kubernetesBackupper)
			backupper.itemBackupper = itemBackupper

			err = backupper.backupResource(ctx, group, resource)

			assert.Equal(t, test.expectedDeploymentsBackedUp, ctx.deploymentsBackedUp)
			assert.Equal(t, test.expectedNetworkPoliciesBackedUp, ctx.networkPoliciesBackedUp)
			assert.Equal(t, test.expectedActionIDs, actualActionIDs)
		})
	}
}

type fakeItemBackupper struct {
	mock.Mock
}

func (f *fakeItemBackupper) backupItem(ctx *backupContext, obj map[string]interface{}, groupResource schema.GroupResource) error {
	args := f.Called(ctx, obj, groupResource)
	return args.Error(0)
}

type fakeTarWriter struct {
	closeCalled      bool
	headers          []*tar.Header
	data             [][]byte
	writeHeaderError error
	writeError       error
}

func (w *fakeTarWriter) Close() error { return nil }

func (w *fakeTarWriter) Write(data []byte) (int, error) {
	w.data = append(w.data, data)
	return 0, w.writeError
}

func (w *fakeTarWriter) WriteHeader(header *tar.Header) error {
	w.headers = append(w.headers, header)
	return w.writeHeaderError
}

func TestBackupItem(t *testing.T) {
	tests := []struct {
		name                      string
		item                      string
		namespaceIncludesExcludes *collections.IncludesExcludes
		resourceIncludesExcludes  *collections.IncludesExcludes
		includeClusterResources   *bool
		backedUpItems             map[itemKey]struct{}
		expectError               bool
		expectExcluded            bool
		expectedTarHeaderName     string
		tarWriteError             bool
		tarHeaderWriteError       bool
		customAction              bool
		expectedActionID          string
	}{
		{
			name:        "empty map",
			item:        "{}",
			expectError: true,
		},
		{
			name:        "missing name",
			item:        `{"metadata":{}}`,
			expectError: true,
		},
		{
			name: "excluded by namespace",
			item: `{"metadata":{"namespace":"foo","name":"bar"}}`,
			namespaceIncludesExcludes: collections.NewIncludesExcludes().Includes("*").Excludes("foo"),
			expectError:               false,
			expectExcluded:            true,
		},
		{
			name: "explicit namespace include",
			item: `{"metadata":{"namespace":"foo","name":"bar"}}`,
			namespaceIncludesExcludes: collections.NewIncludesExcludes().Includes("foo"),
			expectError:               false,
			expectExcluded:            false,
			expectedTarHeaderName:     "namespaces/foo/resource.group/bar.json",
		},
		{
			name: "* namespace include",
			item: `{"metadata":{"namespace":"foo","name":"bar"}}`,
			namespaceIncludesExcludes: collections.NewIncludesExcludes().Includes("*"),
			expectError:               false,
			expectExcluded:            false,
			expectedTarHeaderName:     "namespaces/foo/resource.group/bar.json",
		},
		{
			name:                  "cluster-scoped",
			item:                  `{"metadata":{"name":"bar"}}`,
			expectError:           false,
			expectExcluded:        false,
			expectedTarHeaderName: "cluster/resource.group/bar.json",
		},
		{
			name:                  "make sure status is deleted",
			item:                  `{"metadata":{"name":"bar"},"spec":{"color":"green"},"status":{"foo":"bar"}}`,
			expectError:           false,
			expectExcluded:        false,
			expectedTarHeaderName: "cluster/resource.group/bar.json",
		},
		{
			name:                "tar header write error",
			item:                `{"metadata":{"name":"bar"},"spec":{"color":"green"},"status":{"foo":"bar"}}`,
			expectError:         true,
			tarHeaderWriteError: true,
		},
		{
			name:          "tar write error",
			item:          `{"metadata":{"name":"bar"},"spec":{"color":"green"},"status":{"foo":"bar"}}`,
			expectError:   true,
			tarWriteError: true,
		},
		{
			name: "action invoked - cluster-scoped",
			namespaceIncludesExcludes: collections.NewIncludesExcludes().Includes("*"),
			item:                  `{"metadata":{"name":"bar"}}`,
			expectError:           false,
			expectExcluded:        false,
			expectedTarHeaderName: "cluster/resource.group/bar.json",
			customAction:          true,
			expectedActionID:      "bar",
		},
		{
			name: "action invoked - namespaced",
			namespaceIncludesExcludes: collections.NewIncludesExcludes().Includes("*"),
			item:                  `{"metadata":{"namespace": "myns", "name":"bar"}}`,
			expectError:           false,
			expectExcluded:        false,
			expectedTarHeaderName: "namespaces/myns/resource.group/bar.json",
			customAction:          true,
			expectedActionID:      "myns/bar",
		},
		{
			name: "cluster-scoped item not backed up when --include-cluster-resources=false",
			item: `{"metadata":{"namespace":"","name":"bar"}}`,
			namespaceIncludesExcludes: collections.NewIncludesExcludes().Includes("*"),
			includeClusterResources:   falsePointer,
			expectError:               false,
			expectExcluded:            true,
		},
		{
			name: "item not backed up when resource includes/excludes excludes it",
			item: `{"metadata":{"namespace":"","name":"bar"}}`,
			namespaceIncludesExcludes: collections.NewIncludesExcludes().Includes("*"),
			resourceIncludesExcludes:  collections.NewIncludesExcludes().Includes("*").Excludes("resource.group"),
			expectError:               false,
			expectExcluded:            true,
		},
		{
			name: "item not backed up when it's already been backed up",
			item: `{"metadata":{"namespace":"","name":"bar"}}`,
			namespaceIncludesExcludes: collections.NewIncludesExcludes().Includes("*"),
			backedUpItems:             map[itemKey]struct{}{itemKey{resource: "resource.group", namespace: "", name: "bar"}: struct{}{}},
			expectError:               false,
			expectExcluded:            true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			item, err := getAsMap(test.item)
			if err != nil {
				t.Fatal(err)
			}

			namespaces := test.namespaceIncludesExcludes
			if namespaces == nil {
				namespaces = collections.NewIncludesExcludes()
			}

			w := &fakeTarWriter{}
			if test.tarHeaderWriteError {
				w.writeHeaderError = errors.New("error")
			}
			if test.tarWriteError {
				w.writeError = errors.New("error")
			}

			var (
				action        *fakeAction
				backup        = &v1.Backup{Spec: v1.BackupSpec{IncludeClusterResources: test.includeClusterResources}}
				groupResource = schema.ParseGroupResource("resource.group")
				log, _        = testlogger.NewNullLogger()
			)

			ctx := &backupContext{
				backup: backup,
				namespaceIncludesExcludes: namespaces,
				w:                        w,
				logger:                   log,
				backedUpItems:            make(map[itemKey]struct{}),
				resourceIncludesExcludes: collections.NewIncludesExcludes().Includes("*"),
			}

			if test.resourceIncludesExcludes != nil {
				ctx.resourceIncludesExcludes = test.resourceIncludesExcludes
			}

			if test.backedUpItems != nil {
				ctx.backedUpItems = test.backedUpItems
			}

			if test.customAction {
				action = &fakeAction{}
				ctx.actions = map[schema.GroupResource]Action{
					groupResource: action,
				}
				backup = ctx.backup
			}

			b := &realItemBackupper{}
			err = b.backupItem(ctx, item, groupResource)
			gotError := err != nil
			if e, a := test.expectError, gotError; e != a {
				t.Fatalf("error: expected %t, got %t", e, a)
			}
			if test.expectError {
				return
			}

			if test.expectExcluded {
				if len(w.headers) > 0 {
					t.Errorf("unexpected header write")
				}
				if len(w.data) > 0 {
					t.Errorf("unexpected data write")
				}
				return
			}

			// we have to delete status as that's what backupItem does,
			// and this ensures that we're verifying the right data
			delete(item, "status")
			itemWithoutStatus, err := json.Marshal(&item)
			if err != nil {
				t.Fatal(err)
			}

			require.Equal(t, 1, len(w.headers), "headers")
			assert.Equal(t, test.expectedTarHeaderName, w.headers[0].Name, "header.name")
			assert.Equal(t, int64(len(itemWithoutStatus)), w.headers[0].Size, "header.size")
			assert.Equal(t, byte(tar.TypeReg), w.headers[0].Typeflag, "header.typeflag")
			assert.Equal(t, int64(0755), w.headers[0].Mode, "header.mode")
			assert.False(t, w.headers[0].ModTime.IsZero(), "header.modTime set")
			assert.Equal(t, 1, len(w.data), "# of data")

			actual, err := getAsMap(string(w.data[0]))
			if err != nil {
				t.Fatal(err)
			}
			if e, a := item, actual; !reflect.DeepEqual(e, a) {
				t.Errorf("data: expected %s, got %s", e, a)
			}

			if test.customAction {
				if len(action.ids) != 1 {
					t.Errorf("unexpected custom action ids: %v", action.ids)
				} else if e, a := test.expectedActionID, action.ids[0]; e != a {
					t.Errorf("action.ids[0]: expected %s, got %s", e, a)
				}

				if len(action.backups) != 1 {
					t.Errorf("unexpected custom action backups: %#v", action.backups)
				} else if e, a := backup, action.backups[0]; e != a {
					t.Errorf("action.backups[0]: expected %#v, got %#v", e, a)
				}
			}
		})
	}
}

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
