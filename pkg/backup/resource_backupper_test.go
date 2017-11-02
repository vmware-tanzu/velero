/*
Copyright 2017 the Heptio Ark contributors.

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
	"testing"

	"github.com/heptio/ark/pkg/apis/ark/v1"
	"github.com/heptio/ark/pkg/client"
	"github.com/heptio/ark/pkg/discovery"
	"github.com/heptio/ark/pkg/util/collections"
	arktest "github.com/heptio/ark/pkg/util/test"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func TestBackupResource(t *testing.T) {
	var (
		trueVal      = true
		falseVal     = false
		truePointer  = &trueVal
		falsePointer = &falseVal
	)

	tests := []struct {
		name                     string
		namespaces               *collections.IncludesExcludes
		resources                *collections.IncludesExcludes
		expectSkip               bool
		expectedListedNamespaces []string
		apiGroup                 *metav1.APIResourceList
		apiResource              metav1.APIResource
		groupVersion             schema.GroupVersion
		groupResource            schema.GroupResource
		listResponses            [][]*unstructured.Unstructured
		includeClusterResources  *bool
	}{
		{
			name:        "resource not included",
			apiGroup:    v1Group,
			apiResource: podsResource,
			resources:   collections.NewIncludesExcludes().Excludes("pods"),
			expectSkip:  true,
		},
		{
			name:                     "list all namespaces",
			namespaces:               collections.NewIncludesExcludes(),
			resources:                collections.NewIncludesExcludes(),
			expectedListedNamespaces: []string{""},
			apiGroup:                 v1Group,
			apiResource:              podsResource,
			groupVersion:             schema.GroupVersion{Group: "", Version: "v1"},
			groupResource:            schema.GroupResource{Group: "", Resource: "pods"},
			listResponses: [][]*unstructured.Unstructured{
				{
					unstructuredOrDie(`{"apiVersion":"v1","kind":"Pod","metadata":{"namespace":"myns","name":"myname1"}}`),
					unstructuredOrDie(`{"apiVersion":"v1","kind":"Pod","metadata":{"namespace":"myns","name":"myname2"}}`),
				},
			},
		},
		{
			name:                     "list selected namespaces",
			namespaces:               collections.NewIncludesExcludes().Includes("a", "b"),
			resources:                collections.NewIncludesExcludes(),
			expectedListedNamespaces: []string{"a", "b"},
			apiGroup:                 v1Group,
			apiResource:              podsResource,
			groupVersion:             schema.GroupVersion{Group: "", Version: "v1"},
			groupResource:            schema.GroupResource{Group: "", Resource: "pods"},
			listResponses: [][]*unstructured.Unstructured{
				{
					unstructuredOrDie(`{"apiVersion":"v1","kind":"Pod","metadata":{"namespace":"a","name":"myname1"}}`),
					unstructuredOrDie(`{"apiVersion":"v1","kind":"Pod","metadata":{"namespace":"a","name":"myname2"}}`),
				},
				{
					unstructuredOrDie(`{"apiVersion":"v1","kind":"Pod","metadata":{"namespace":"b","name":"myname3"}}`),
					unstructuredOrDie(`{"apiVersion":"v1","kind":"Pod","metadata":{"namespace":"b","name":"myname4"}}`),
				},
			},
		},
		{
			name:                     "list all namespaces - cluster scoped",
			namespaces:               collections.NewIncludesExcludes(),
			resources:                collections.NewIncludesExcludes(),
			expectedListedNamespaces: []string{""},
			apiGroup:                 certificatesGroup,
			apiResource:              certificateSigningRequestsResource,
			groupVersion:             schema.GroupVersion{Group: "certificates.k8s.io", Version: "v1beta1"},
			groupResource:            schema.GroupResource{Group: "certificates.k8s.io", Resource: "certificatesigningrequests"},
			listResponses: [][]*unstructured.Unstructured{
				{
					unstructuredOrDie(`{"apiVersion":"certificates.k8s.io/v1beta1","kind":"CertificateSigningRequest","metadata":{"name":"myname1"}}`),
					unstructuredOrDie(`{"apiVersion":"certificates.k8s.io/v1beta1","kind":"CertificateSigningRequest","metadata":{"name":"myname2"}}`),
				},
			},
		},
		{
			name:                     "should include cluster-scoped resource if backing up subset of namespaces and --include-cluster-resources=true",
			namespaces:               collections.NewIncludesExcludes().Includes("ns-1"),
			resources:                collections.NewIncludesExcludes(),
			includeClusterResources:  truePointer,
			expectedListedNamespaces: []string{""},
			apiGroup:                 certificatesGroup,
			apiResource:              certificateSigningRequestsResource,
			groupVersion:             schema.GroupVersion{Group: "certificates.k8s.io", Version: "v1beta1"},
			groupResource:            schema.GroupResource{Group: "certificates.k8s.io", Resource: "certificatesigningrequests"},
			listResponses: [][]*unstructured.Unstructured{
				{
					unstructuredOrDie(`{"apiVersion":"certificates.k8s.io/v1beta1","kind":"CertificateSigningRequest","metadata":{"name":"myname1"}}`),
					unstructuredOrDie(`{"apiVersion":"certificates.k8s.io/v1beta1","kind":"CertificateSigningRequest","metadata":{"name":"myname2"}}`),
				},
			},
		},
		{
			name:                    "should not include cluster-scoped resource if backing up subset of namespaces and --include-cluster-resources=false",
			namespaces:              collections.NewIncludesExcludes().Includes("ns-1"),
			resources:               collections.NewIncludesExcludes(),
			includeClusterResources: falsePointer,
			apiGroup:                certificatesGroup,
			apiResource:             certificateSigningRequestsResource,
			groupVersion:            schema.GroupVersion{Group: "certificates.k8s.io", Version: "v1beta1"},
			groupResource:           schema.GroupResource{Group: "certificates.k8s.io", Resource: "certificatesigningrequests"},
			expectSkip:              true,
		},
		{
			name:                    "should not include cluster-scoped resource if backing up subset of namespaces and --include-cluster-resources=nil",
			namespaces:              collections.NewIncludesExcludes().Includes("ns-1"),
			resources:               collections.NewIncludesExcludes(),
			includeClusterResources: nil,
			apiGroup:                certificatesGroup,
			apiResource:             certificateSigningRequestsResource,
			groupVersion:            schema.GroupVersion{Group: "certificates.k8s.io", Version: "v1beta1"},
			groupResource:           schema.GroupResource{Group: "certificates.k8s.io", Resource: "certificatesigningrequests"},
			expectSkip:              true,
		},
		{
			name:                     "should include cluster-scoped resource if backing up all namespaces and --include-cluster-resources=true",
			namespaces:               collections.NewIncludesExcludes(),
			resources:                collections.NewIncludesExcludes(),
			includeClusterResources:  truePointer,
			expectedListedNamespaces: []string{""},
			apiGroup:                 certificatesGroup,
			apiResource:              certificateSigningRequestsResource,
			groupVersion:             schema.GroupVersion{Group: "certificates.k8s.io", Version: "v1beta1"},
			groupResource:            schema.GroupResource{Group: "certificates.k8s.io", Resource: "certificatesigningrequests"},
			listResponses: [][]*unstructured.Unstructured{
				{
					unstructuredOrDie(`{"apiVersion":"certificates.k8s.io/v1beta1","kind":"CertificateSigningRequest","metadata":{"name":"myname1"}}`),
					unstructuredOrDie(`{"apiVersion":"certificates.k8s.io/v1beta1","kind":"CertificateSigningRequest","metadata":{"name":"myname2"}}`),
				},
			},
		},
		{
			name:                    "should not include cluster-scoped resource if backing up all namespaces and --include-cluster-resources=false",
			namespaces:              collections.NewIncludesExcludes(),
			resources:               collections.NewIncludesExcludes(),
			includeClusterResources: falsePointer,
			apiGroup:                certificatesGroup,
			apiResource:             certificateSigningRequestsResource,
			groupVersion:            schema.GroupVersion{Group: "certificates.k8s.io", Version: "v1beta1"},
			groupResource:           schema.GroupResource{Group: "certificates.k8s.io", Resource: "certificatesigningrequests"},
			expectSkip:              true,
		},
		{
			name:                     "should include cluster-scoped resource if backing up all namespaces and --include-cluster-resources=nil",
			namespaces:               collections.NewIncludesExcludes(),
			resources:                collections.NewIncludesExcludes(),
			includeClusterResources:  nil,
			expectedListedNamespaces: []string{""},
			apiGroup:                 certificatesGroup,
			apiResource:              certificateSigningRequestsResource,
			groupVersion:             schema.GroupVersion{Group: "certificates.k8s.io", Version: "v1beta1"},
			groupResource:            schema.GroupResource{Group: "certificates.k8s.io", Resource: "certificatesigningrequests"},
			listResponses: [][]*unstructured.Unstructured{
				{
					unstructuredOrDie(`{"apiVersion":"certificates.k8s.io/v1beta1","kind":"CertificateSigningRequest","metadata":{"name":"myname1"}}`),
					unstructuredOrDie(`{"apiVersion":"certificates.k8s.io/v1beta1","kind":"CertificateSigningRequest","metadata":{"name":"myname2"}}`),
				},
			},
		},
		{
			name:                     "should include specified namespaces if backing up subset of namespaces and --include-cluster-resources=nil",
			namespaces:               collections.NewIncludesExcludes().Includes("ns-1"),
			resources:                collections.NewIncludesExcludes(),
			includeClusterResources:  nil,
			expectedListedNamespaces: []string{"ns-1"},
			apiGroup:                 v1Group,
			apiResource:              namespacesResource,
			groupVersion:             schema.GroupVersion{Group: "", Version: "v1"},
			groupResource:            schema.GroupResource{Group: "", Resource: "namespaces"},
			expectSkip:               false,
			listResponses: [][]*unstructured.Unstructured{
				{
					unstructuredOrDie(`{"apiVersion":"v1","kind":"Namespace","metadata":{"name":"ns-1"}}`),
					unstructuredOrDie(`{"apiVersion":"v1","kind":"Namespace","metadata":{"name":"ns-2"}}`),
				},
			},
		},
	}

	for _, test := range tests {
		backup := &v1.Backup{
			Spec: v1.BackupSpec{
				IncludeClusterResources: test.includeClusterResources,
			},
		}

		labelSelector := "foo=bar"

		dynamicFactory := &arktest.FakeDynamicFactory{}
		defer dynamicFactory.AssertExpectations(t)

		discoveryHelper := arktest.NewFakeDiscoveryHelper(true, nil)

		backedUpItems := map[itemKey]struct{}{
			{resource: "foo", namespace: "ns", name: "name"}: {},
		}

		cohabitatingResources := map[string]*cohabitatingResource{
			"deployments":     newCohabitatingResource("deployments", "extensions", "apps"),
			"networkpolicies": newCohabitatingResource("networkpolicies", "extensions", "networking.k8s.io"),
		}

		actions := map[schema.GroupResource]Action{
			{Group: "", Resource: "pods"}: &fakeAction{},
		}

		resourceHooks := []resourceHook{
			{name: "myhook"},
		}

		podCommandExecutor := &mockPodCommandExecutor{}
		defer podCommandExecutor.AssertExpectations(t)

		tarWriter := &fakeTarWriter{}

		t.Run(test.name, func(t *testing.T) {
			rb := (&defaultResourceBackupperFactory{}).newResourceBackupper(
				arktest.NewLogger(),
				backup,
				test.namespaces,
				test.resources,
				labelSelector,
				dynamicFactory,
				discoveryHelper,
				backedUpItems,
				cohabitatingResources,
				actions,
				podCommandExecutor,
				tarWriter,
				resourceHooks,
			).(*defaultResourceBackupper)

			itemBackupperFactory := &mockItemBackupperFactory{}
			defer itemBackupperFactory.AssertExpectations(t)
			rb.itemBackupperFactory = itemBackupperFactory

			if !test.expectSkip {
				itemBackupper := &mockItemBackupper{}
				defer itemBackupper.AssertExpectations(t)

				itemBackupperFactory.On("newItemBackupper",
					backup,
					test.namespaces,
					test.resources,
					backedUpItems,
					actions,
					podCommandExecutor,
					tarWriter,
					resourceHooks,
					dynamicFactory,
					discoveryHelper,
				).Return(itemBackupper)

				for i, namespace := range test.expectedListedNamespaces {
					client := &arktest.FakeDynamicClient{}
					defer client.AssertExpectations(t)

					if test.groupResource.Resource == "namespaces" {
						dynamicFactory.On("ClientForGroupVersionResource", test.groupVersion, test.apiResource, "").Return(client, nil)
					} else {
						dynamicFactory.On("ClientForGroupVersionResource", test.groupVersion, test.apiResource, namespace).Return(client, nil)
					}

					list := &unstructured.UnstructuredList{
						Items: []unstructured.Unstructured{},
					}
					for _, item := range test.listResponses[i] {
						list.Items = append(list.Items, *item)
						itemBackupper.On("backupItem", mock.AnythingOfType("*logrus.Entry"), item, test.groupResource).Return(nil)
					}
					client.On("List", metav1.ListOptions{LabelSelector: labelSelector}).Return(list, nil)

				}
			}
			err := rb.backupResource(test.apiGroup, test.apiResource)
			require.NoError(t, err)
		})
	}
}

func TestBackupResourceCohabitation(t *testing.T) {
	tests := []struct {
		name          string
		apiResource   metav1.APIResource
		apiGroup1     *metav1.APIResourceList
		groupVersion1 schema.GroupVersion
		apiGroup2     *metav1.APIResourceList
		groupVersion2 schema.GroupVersion
	}{
		{
			name:          "deployments - extensions first",
			apiResource:   deploymentsResource,
			apiGroup1:     extensionsGroup,
			groupVersion1: extensionsGroupVersion,
			apiGroup2:     appsGroup,
			groupVersion2: appsGroupVersion,
		},
		{
			name:          "deployments - apps first",
			apiResource:   deploymentsResource,
			apiGroup1:     appsGroup,
			groupVersion1: appsGroupVersion,
			apiGroup2:     extensionsGroup,
			groupVersion2: extensionsGroupVersion,
		},
		{
			name:          "networkpolicies - extensions first",
			apiResource:   networkPoliciesResource,
			apiGroup1:     extensionsGroup,
			groupVersion1: extensionsGroupVersion,
			apiGroup2:     networkingGroup,
			groupVersion2: networkingGroupVersion,
		},
		{
			name:          "networkpolicies - networking first",
			apiResource:   networkPoliciesResource,
			apiGroup1:     networkingGroup,
			groupVersion1: networkingGroupVersion,
			apiGroup2:     extensionsGroup,
			groupVersion2: extensionsGroupVersion,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			backup := &v1.Backup{}

			namespaces := collections.NewIncludesExcludes().Includes("*")
			resources := collections.NewIncludesExcludes().Includes("*")

			labelSelector := "foo=bar"

			dynamicFactory := &arktest.FakeDynamicFactory{}
			defer dynamicFactory.AssertExpectations(t)

			discoveryHelper := arktest.NewFakeDiscoveryHelper(true, nil)

			backedUpItems := map[itemKey]struct{}{
				{resource: "foo", namespace: "ns", name: "name"}: {},
			}

			cohabitatingResources := map[string]*cohabitatingResource{
				"deployments":     newCohabitatingResource("deployments", "extensions", "apps"),
				"networkpolicies": newCohabitatingResource("networkpolicies", "extensions", "networking.k8s.io"),
			}

			actions := map[schema.GroupResource]Action{
				{Group: "", Resource: "pods"}: &fakeAction{},
			}

			resourceHooks := []resourceHook{
				{name: "myhook"},
			}

			podCommandExecutor := &mockPodCommandExecutor{}
			defer podCommandExecutor.AssertExpectations(t)

			tarWriter := &fakeTarWriter{}

			rb := (&defaultResourceBackupperFactory{}).newResourceBackupper(
				arktest.NewLogger(),
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
			).(*defaultResourceBackupper)

			itemBackupperFactory := &mockItemBackupperFactory{}
			defer itemBackupperFactory.AssertExpectations(t)
			rb.itemBackupperFactory = itemBackupperFactory

			itemBackupper := &mockItemBackupper{}
			defer itemBackupper.AssertExpectations(t)

			itemBackupperFactory.On("newItemBackupper",
				backup,
				namespaces,
				resources,
				backedUpItems,
				actions,
				podCommandExecutor,
				tarWriter,
				resourceHooks,
				dynamicFactory,
				discoveryHelper,
			).Return(itemBackupper)

			client := &arktest.FakeDynamicClient{}
			defer client.AssertExpectations(t)

			// STEP 1: make sure the initial backup goes through
			dynamicFactory.On("ClientForGroupVersionResource", test.groupVersion1, test.apiResource, "").Return(client, nil)
			client.On("List", metav1.ListOptions{LabelSelector: labelSelector}).Return(&unstructured.UnstructuredList{}, nil)

			// STEP 2: do the backup
			err := rb.backupResource(test.apiGroup1, test.apiResource)
			require.NoError(t, err)

			// STEP 3: try to back up the cohabitating resource
			err = rb.backupResource(test.apiGroup2, test.apiResource)
			require.NoError(t, err)
		})
	}
}

type mockItemBackupperFactory struct {
	mock.Mock
}

func (ibf *mockItemBackupperFactory) newItemBackupper(
	backup *v1.Backup,
	namespaces, resources *collections.IncludesExcludes,
	backedUpItems map[itemKey]struct{},
	actions map[schema.GroupResource]Action,
	podCommandExecutor podCommandExecutor,
	tarWriter tarWriter,
	resourceHooks []resourceHook,
	dynamicFactory client.DynamicFactory,
	discoveryHelper discovery.Helper,
) ItemBackupper {
	args := ibf.Called(
		backup,
		namespaces,
		resources,
		backedUpItems,
		actions,
		podCommandExecutor,
		tarWriter,
		resourceHooks,
		dynamicFactory,
		discoveryHelper,
	)
	return args.Get(0).(ItemBackupper)
}

/*
func TestBackupResource2(t *testing.T) {
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
						LabelSelector: labelSelector,
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

			itemBackupper := &mockItemBackupper{}

			var actualActionIDs map[string][]string

			dynamicFactory := &arktest.FakeDynamicFactory{}
			gvr := schema.GroupVersionResource{Group: test.resourceGroup, Version: test.resourceVersion}
			gr := schema.GroupResource{Group: test.resourceGroup, Resource: test.resourceName}
			for i, namespace := range test.expectedListedNamespaces {
				obj := toRuntimeObject(t, test.lists[i])

				client := &arktest.FakeDynamicClient{}
				client.On("List", metav1.ListOptions{LabelSelector: test.labelSelector}).Return(obj, nil)
				dynamicFactory.On("ClientForGroupVersionResource", gvr, resource, namespace).Return(client, nil)

				action := test.actions[test.resourceName]

				list, err := meta.ExtractList(obj)
				require.NoError(t, err)
				for i := range list {
					item := list[i].(*unstructured.Unstructured)
					itemBackupper.On("backupItem", ctx, item, gr).Return(nil)
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
			discoveryHelper := arktest.NewFakeDiscoveryHelper(false, resources)

			podCommandExecutor := &arktest.PodCommandExecutor{}
			defer podCommandExecutor.AssertExpectations(t)

			kb, err := NewKubernetesBackupper(discoveryHelper, dynamicFactory, test.actions, podCommandExecutor)
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
*/
