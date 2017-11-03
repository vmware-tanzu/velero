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
	"github.com/stretchr/testify/assert"
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
		getResponses             []*unstructured.Unstructured
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
			namespaces:               collections.NewIncludesExcludes().Includes("ns-1", "ns-2"),
			resources:                collections.NewIncludesExcludes(),
			includeClusterResources:  nil,
			expectedListedNamespaces: []string{"ns-1", "ns-2"},
			apiGroup:                 v1Group,
			apiResource:              namespacesResource,
			groupVersion:             schema.GroupVersion{Group: "", Version: "v1"},
			groupResource:            schema.GroupResource{Group: "", Resource: "namespaces"},
			expectSkip:               false,
			getResponses: []*unstructured.Unstructured{
				unstructuredOrDie(`{"apiVersion":"v1","kind":"Namespace","metadata":{"name":"ns-1"}}`),
				unstructuredOrDie(`{"apiVersion":"v1","kind":"Namespace","metadata":{"name":"ns-2"}}`),
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

				if len(test.listResponses) > 0 {
					for i, namespace := range test.expectedListedNamespaces {
						client := &arktest.FakeDynamicClient{}
						defer client.AssertExpectations(t)

						dynamicFactory.On("ClientForGroupVersionResource", test.groupVersion, test.apiResource, namespace).Return(client, nil)

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

				if len(test.getResponses) > 0 {
					client := &arktest.FakeDynamicClient{}
					defer client.AssertExpectations(t)

					dynamicFactory.On("ClientForGroupVersionResource", test.groupVersion, test.apiResource, "").Return(client, nil)

					for i, namespace := range test.expectedListedNamespaces {
						item := test.getResponses[i]
						client.On("Get", namespace, metav1.GetOptions{}).Return(item, nil)
						itemBackupper.On("backupItem", mock.AnythingOfType("*logrus.Entry"), item, test.groupResource).Return(nil)
					}
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

func TestBackupResourceOnlyIncludesSpecifiedNamespaces(t *testing.T) {
	backup := &v1.Backup{}

	namespaces := collections.NewIncludesExcludes().Includes("ns-1")
	resources := collections.NewIncludesExcludes().Includes("*")

	labelSelector := "foo=bar"
	backedUpItems := map[itemKey]struct{}{}

	dynamicFactory := &arktest.FakeDynamicFactory{}
	defer dynamicFactory.AssertExpectations(t)

	discoveryHelper := arktest.NewFakeDiscoveryHelper(true, nil)

	cohabitatingResources := map[string]*cohabitatingResource{}

	actions := map[schema.GroupResource]Action{}

	resourceHooks := []resourceHook{}

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

	itemHookHandler := &mockItemHookHandler{}
	defer itemHookHandler.AssertExpectations(t)

	itemBackupper := &defaultItemBackupper{
		backup:          backup,
		namespaces:      namespaces,
		resources:       resources,
		backedUpItems:   backedUpItems,
		actions:         actions,
		tarWriter:       tarWriter,
		resourceHooks:   resourceHooks,
		dynamicFactory:  dynamicFactory,
		discoveryHelper: discoveryHelper,
		itemHookHandler: itemHookHandler,
	}

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

	coreV1Group := schema.GroupVersion{Group: "", Version: "v1"}
	dynamicFactory.On("ClientForGroupVersionResource", coreV1Group, namespacesResource, "").Return(client, nil)
	ns1 := unstructuredOrDie(`{"apiVersion":"v1","kind":"Namespace","metadata":{"name":"ns-1"}}`)
	client.On("Get", "ns-1", metav1.GetOptions{}).Return(ns1, nil)

	itemHookHandler.On("handleHooks", mock.Anything, schema.GroupResource{Group: "", Resource: "namespaces"}, ns1, resourceHooks).Return(nil)

	err := rb.backupResource(v1Group, namespacesResource)
	require.NoError(t, err)

	require.Len(t, tarWriter.headers, 1)
	assert.Equal(t, "resources/namespaces/cluster/ns-1.json", tarWriter.headers[0].Name)
}

func TestBackupResourceListAllNamespacesExcludesCorrectly(t *testing.T) {
	backup := &v1.Backup{}

	namespaces := collections.NewIncludesExcludes().Excludes("ns-1")
	resources := collections.NewIncludesExcludes().Includes("*")

	labelSelector := "foo=bar"
	backedUpItems := map[itemKey]struct{}{}

	dynamicFactory := &arktest.FakeDynamicFactory{}
	defer dynamicFactory.AssertExpectations(t)

	discoveryHelper := arktest.NewFakeDiscoveryHelper(true, nil)

	cohabitatingResources := map[string]*cohabitatingResource{}

	actions := map[schema.GroupResource]Action{}

	resourceHooks := []resourceHook{}

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

	itemHookHandler := &mockItemHookHandler{}
	defer itemHookHandler.AssertExpectations(t)

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

	coreV1Group := schema.GroupVersion{Group: "", Version: "v1"}
	dynamicFactory.On("ClientForGroupVersionResource", coreV1Group, namespacesResource, "").Return(client, nil)

	ns1 := unstructuredOrDie(`{"apiVersion":"v1","kind":"Namespace","metadata":{"name":"ns-1"}}`)
	ns2 := unstructuredOrDie(`{"apiVersion":"v1","kind":"Namespace","metadata":{"name":"ns-2"}}`)
	list := &unstructured.UnstructuredList{
		Items: []unstructured.Unstructured{*ns1, *ns2},
	}
	client.On("List", metav1.ListOptions{LabelSelector: labelSelector}).Return(list, nil)

	itemBackupper.On("backupItem", mock.AnythingOfType("*logrus.Entry"), ns2, namespacesGroupResource).Return(nil)

	err := rb.backupResource(v1Group, namespacesResource)
	require.NoError(t, err)
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
