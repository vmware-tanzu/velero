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

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"

	v1 "github.com/heptio/velero/pkg/apis/velero/v1"
	"github.com/heptio/velero/pkg/client"
	"github.com/heptio/velero/pkg/discovery"
	"github.com/heptio/velero/pkg/kuberesource"
	"github.com/heptio/ark/pkg/plugin/interface/volumeinterface"
	"github.com/heptio/velero/pkg/podexec"
	"github.com/heptio/velero/pkg/restic"
	"github.com/heptio/velero/pkg/util/collections"
	velerotest "github.com/heptio/velero/pkg/util/test"
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
					velerotest.UnstructuredOrDie(`{"apiVersion":"v1","kind":"Pod","metadata":{"namespace":"myns","name":"myname1"}}`),
					velerotest.UnstructuredOrDie(`{"apiVersion":"v1","kind":"Pod","metadata":{"namespace":"myns","name":"myname2"}}`),
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
					velerotest.UnstructuredOrDie(`{"apiVersion":"v1","kind":"Pod","metadata":{"namespace":"a","name":"myname1"}}`),
					velerotest.UnstructuredOrDie(`{"apiVersion":"v1","kind":"Pod","metadata":{"namespace":"a","name":"myname2"}}`),
				},
				{
					velerotest.UnstructuredOrDie(`{"apiVersion":"v1","kind":"Pod","metadata":{"namespace":"b","name":"myname3"}}`),
					velerotest.UnstructuredOrDie(`{"apiVersion":"v1","kind":"Pod","metadata":{"namespace":"b","name":"myname4"}}`),
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
					velerotest.UnstructuredOrDie(`{"apiVersion":"certificates.k8s.io/v1beta1","kind":"CertificateSigningRequest","metadata":{"name":"myname1"}}`),
					velerotest.UnstructuredOrDie(`{"apiVersion":"certificates.k8s.io/v1beta1","kind":"CertificateSigningRequest","metadata":{"name":"myname2"}}`),
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
					velerotest.UnstructuredOrDie(`{"apiVersion":"certificates.k8s.io/v1beta1","kind":"CertificateSigningRequest","metadata":{"name":"myname1"}}`),
					velerotest.UnstructuredOrDie(`{"apiVersion":"certificates.k8s.io/v1beta1","kind":"CertificateSigningRequest","metadata":{"name":"myname2"}}`),
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
					velerotest.UnstructuredOrDie(`{"apiVersion":"certificates.k8s.io/v1beta1","kind":"CertificateSigningRequest","metadata":{"name":"myname1"}}`),
					velerotest.UnstructuredOrDie(`{"apiVersion":"certificates.k8s.io/v1beta1","kind":"CertificateSigningRequest","metadata":{"name":"myname2"}}`),
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
					velerotest.UnstructuredOrDie(`{"apiVersion":"certificates.k8s.io/v1beta1","kind":"CertificateSigningRequest","metadata":{"name":"myname1"}}`),
					velerotest.UnstructuredOrDie(`{"apiVersion":"certificates.k8s.io/v1beta1","kind":"CertificateSigningRequest","metadata":{"name":"myname2"}}`),
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
				velerotest.UnstructuredOrDie(`{"apiVersion":"v1","kind":"Namespace","metadata":{"name":"ns-1"}}`),
				velerotest.UnstructuredOrDie(`{"apiVersion":"v1","kind":"Namespace","metadata":{"name":"ns-2"}}`),
			},
		},
	}

	for _, test := range tests {
		req := &Request{
			Backup: &v1.Backup{
				Spec: v1.BackupSpec{
					IncludeClusterResources: test.includeClusterResources,
				},
			},
			ResolvedActions: []resolvedAction{
				{
					BackupItemAction:         newFakeAction("pods"),
					resourceIncludesExcludes: collections.NewIncludesExcludes().Includes("pods"),
				},
			},
			ResourceHooks: []resourceHook{
				{name: "myhook"},
			},
			ResourceIncludesExcludes:  test.resources,
			NamespaceIncludesExcludes: test.namespaces,
		}

		dynamicFactory := &velerotest.FakeDynamicFactory{}
		defer dynamicFactory.AssertExpectations(t)

		discoveryHelper := velerotest.NewFakeDiscoveryHelper(true, nil)

		backedUpItems := map[itemKey]struct{}{
			{resource: "foo", namespace: "ns", name: "name"}: {},
		}

		cohabitatingResources := map[string]*cohabitatingResource{
			"deployments":     newCohabitatingResource("deployments", "extensions", "apps"),
			"networkpolicies": newCohabitatingResource("networkpolicies", "extensions", "networking.k8s.io"),
		}

		podCommandExecutor := &velerotest.MockPodCommandExecutor{}
		defer podCommandExecutor.AssertExpectations(t)

		tarWriter := &fakeTarWriter{}

		t.Run(test.name, func(t *testing.T) {
			rb := (&defaultResourceBackupperFactory{}).newResourceBackupper(
				velerotest.NewLogger(),
				req,
				dynamicFactory,
				discoveryHelper,
				backedUpItems,
				cohabitatingResources,
				podCommandExecutor,
				tarWriter,
				nil, // restic backupper
				newPVCSnapshotTracker(),
				nil,
			).(*defaultResourceBackupper)

			itemBackupperFactory := &mockItemBackupperFactory{}
			defer itemBackupperFactory.AssertExpectations(t)
			rb.itemBackupperFactory = itemBackupperFactory

			if !test.expectSkip {
				itemBackupper := &mockItemBackupper{}
				defer itemBackupper.AssertExpectations(t)

				itemBackupperFactory.On("newItemBackupper",
					req,
					backedUpItems,
					podCommandExecutor,
					tarWriter,
					dynamicFactory,
					discoveryHelper,
					mock.Anything,
					mock.Anything,
					mock.Anything,
				).Return(itemBackupper)

				if len(test.listResponses) > 0 {
					for i, namespace := range test.expectedListedNamespaces {
						client := &velerotest.FakeDynamicClient{}
						defer client.AssertExpectations(t)

						dynamicFactory.On("ClientForGroupVersionResource", test.groupVersion, test.apiResource, namespace).Return(client, nil)

						list := &unstructured.UnstructuredList{
							Items: []unstructured.Unstructured{},
						}
						for _, item := range test.listResponses[i] {
							list.Items = append(list.Items, *item)
							itemBackupper.On("backupItem", mock.AnythingOfType("*logrus.Entry"), item, test.groupResource).Return(nil)
						}
						client.On("List", metav1.ListOptions{}).Return(list, nil)
					}
				}

				if len(test.getResponses) > 0 {
					client := &velerotest.FakeDynamicClient{}
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
			req := &Request{
				Backup: &v1.Backup{
					Spec: v1.BackupSpec{
						LabelSelector: &metav1.LabelSelector{
							MatchLabels: map[string]string{
								"foo": "bar",
							},
						},
					},
				},
				NamespaceIncludesExcludes: collections.NewIncludesExcludes().Includes("*"),
				ResourceIncludesExcludes:  collections.NewIncludesExcludes().Includes("*"),
				ResolvedActions: []resolvedAction{
					{
						BackupItemAction:         newFakeAction("pods"),
						resourceIncludesExcludes: collections.NewIncludesExcludes().Includes("pods"),
					},
				},
				ResourceHooks: []resourceHook{
					{name: "myhook"},
				},
			}

			dynamicFactory := &velerotest.FakeDynamicFactory{}
			defer dynamicFactory.AssertExpectations(t)

			discoveryHelper := velerotest.NewFakeDiscoveryHelper(true, nil)

			backedUpItems := map[itemKey]struct{}{
				{resource: "foo", namespace: "ns", name: "name"}: {},
			}

			cohabitatingResources := map[string]*cohabitatingResource{
				"deployments":     newCohabitatingResource("deployments", "extensions", "apps"),
				"networkpolicies": newCohabitatingResource("networkpolicies", "extensions", "networking.k8s.io"),
			}

			podCommandExecutor := &velerotest.MockPodCommandExecutor{}
			defer podCommandExecutor.AssertExpectations(t)

			tarWriter := &fakeTarWriter{}

			rb := (&defaultResourceBackupperFactory{}).newResourceBackupper(
				velerotest.NewLogger(),
				req,
				dynamicFactory,
				discoveryHelper,
				backedUpItems,
				cohabitatingResources,
				podCommandExecutor,
				tarWriter,
				nil, // restic backupper
				newPVCSnapshotTracker(),
				nil,
			).(*defaultResourceBackupper)

			itemBackupperFactory := &mockItemBackupperFactory{}
			defer itemBackupperFactory.AssertExpectations(t)
			rb.itemBackupperFactory = itemBackupperFactory

			itemBackupper := &mockItemBackupper{}
			defer itemBackupper.AssertExpectations(t)

			itemBackupperFactory.On("newItemBackupper",
				req,
				backedUpItems,
				podCommandExecutor,
				tarWriter,
				dynamicFactory,
				discoveryHelper,
				mock.Anything, // restic backupper
				mock.Anything, // pvc snapshot tracker
				nil,
				mock.Anything,
			).Return(itemBackupper)

			client := &velerotest.FakeDynamicClient{}
			defer client.AssertExpectations(t)

			// STEP 1: make sure the initial backup goes through
			dynamicFactory.On("ClientForGroupVersionResource", test.groupVersion1, test.apiResource, "").Return(client, nil)
			client.On("List", metav1.ListOptions{LabelSelector: metav1.FormatLabelSelector(req.Backup.Spec.LabelSelector)}).Return(&unstructured.UnstructuredList{}, nil)

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
	req := &Request{
		Backup:                    &v1.Backup{},
		NamespaceIncludesExcludes: collections.NewIncludesExcludes().Includes("ns-1"),
		ResourceIncludesExcludes:  collections.NewIncludesExcludes().Includes("*"),
	}

	backedUpItems := map[itemKey]struct{}{}

	dynamicFactory := &velerotest.FakeDynamicFactory{}
	defer dynamicFactory.AssertExpectations(t)

	discoveryHelper := velerotest.NewFakeDiscoveryHelper(true, nil)

	cohabitatingResources := map[string]*cohabitatingResource{}

	podCommandExecutor := &velerotest.MockPodCommandExecutor{}
	defer podCommandExecutor.AssertExpectations(t)

	tarWriter := &fakeTarWriter{}

	rb := (&defaultResourceBackupperFactory{}).newResourceBackupper(
		velerotest.NewLogger(),
		req,
		dynamicFactory,
		discoveryHelper,
		backedUpItems,
		cohabitatingResources,
		podCommandExecutor,
		tarWriter,
		nil, // restic backupper
		newPVCSnapshotTracker(),
		nil,
	).(*defaultResourceBackupper)

	itemBackupperFactory := &mockItemBackupperFactory{}
	defer itemBackupperFactory.AssertExpectations(t)
	rb.itemBackupperFactory = itemBackupperFactory

	itemHookHandler := &mockItemHookHandler{}
	defer itemHookHandler.AssertExpectations(t)

	itemBackupper := &defaultItemBackupper{
		backupRequest:   req,
		backedUpItems:   backedUpItems,
		tarWriter:       tarWriter,
		dynamicFactory:  dynamicFactory,
		discoveryHelper: discoveryHelper,
		itemHookHandler: itemHookHandler,
	}

	itemBackupperFactory.On("newItemBackupper",
		req,
		backedUpItems,
		podCommandExecutor,
		tarWriter,
		dynamicFactory,
		discoveryHelper,
		mock.Anything,
		mock.Anything,
		mock.Anything,
	).Return(itemBackupper)

	client := &velerotest.FakeDynamicClient{}
	defer client.AssertExpectations(t)

	coreV1Group := schema.GroupVersion{Group: "", Version: "v1"}
	dynamicFactory.On("ClientForGroupVersionResource", coreV1Group, namespacesResource, "").Return(client, nil)
	ns1 := velerotest.UnstructuredOrDie(`{"apiVersion":"v1","kind":"Namespace","metadata":{"name":"ns-1"}}`)
	client.On("Get", "ns-1", metav1.GetOptions{}).Return(ns1, nil)

	itemHookHandler.On("handleHooks", mock.Anything, schema.GroupResource{Group: "", Resource: "namespaces"}, ns1, req.ResourceHooks, hookPhasePre).Return(nil)
	itemHookHandler.On("handleHooks", mock.Anything, schema.GroupResource{Group: "", Resource: "namespaces"}, ns1, req.ResourceHooks, hookPhasePost).Return(nil)

	err := rb.backupResource(v1Group, namespacesResource)
	require.NoError(t, err)

	require.Len(t, tarWriter.headers, 1)
	assert.Equal(t, "resources/namespaces/cluster/ns-1.json", tarWriter.headers[0].Name)
}

func TestBackupResourceListAllNamespacesExcludesCorrectly(t *testing.T) {
	req := &Request{
		Backup: &v1.Backup{
			Spec: v1.BackupSpec{
				LabelSelector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						"foo": "bar",
					},
				},
			},
		},
		NamespaceIncludesExcludes: collections.NewIncludesExcludes().Excludes("ns-1"),
		ResourceIncludesExcludes:  collections.NewIncludesExcludes().Includes("*"),
	}

	backedUpItems := map[itemKey]struct{}{}

	dynamicFactory := &velerotest.FakeDynamicFactory{}
	defer dynamicFactory.AssertExpectations(t)

	discoveryHelper := velerotest.NewFakeDiscoveryHelper(true, nil)

	cohabitatingResources := map[string]*cohabitatingResource{}

	podCommandExecutor := &velerotest.MockPodCommandExecutor{}
	defer podCommandExecutor.AssertExpectations(t)

	tarWriter := &fakeTarWriter{}

	rb := (&defaultResourceBackupperFactory{}).newResourceBackupper(
		velerotest.NewLogger(),
		req,
		dynamicFactory,
		discoveryHelper,
		backedUpItems,
		cohabitatingResources,
		podCommandExecutor,
		tarWriter,
		nil, // restic backupper
		newPVCSnapshotTracker(),
		nil,
	).(*defaultResourceBackupper)

	itemBackupperFactory := &mockItemBackupperFactory{}
	defer itemBackupperFactory.AssertExpectations(t)
	rb.itemBackupperFactory = itemBackupperFactory

	itemHookHandler := &mockItemHookHandler{}
	defer itemHookHandler.AssertExpectations(t)

	itemBackupper := &mockItemBackupper{}
	defer itemBackupper.AssertExpectations(t)

	itemBackupperFactory.On("newItemBackupper",
		req,
		backedUpItems,
		podCommandExecutor,
		tarWriter,
		dynamicFactory,
		discoveryHelper,
		mock.Anything,
		mock.Anything,
		mock.Anything,
	).Return(itemBackupper)

	client := &velerotest.FakeDynamicClient{}
	defer client.AssertExpectations(t)

	coreV1Group := schema.GroupVersion{Group: "", Version: "v1"}
	dynamicFactory.On("ClientForGroupVersionResource", coreV1Group, namespacesResource, "").Return(client, nil)

	ns1 := velerotest.UnstructuredOrDie(`{"apiVersion":"v1","kind":"Namespace","metadata":{"name":"ns-1"}}`)
	ns2 := velerotest.UnstructuredOrDie(`{"apiVersion":"v1","kind":"Namespace","metadata":{"name":"ns-2"}}`)
	list := &unstructured.UnstructuredList{
		Items: []unstructured.Unstructured{*ns1, *ns2},
	}
	client.On("List", metav1.ListOptions{LabelSelector: metav1.FormatLabelSelector(req.Backup.Spec.LabelSelector)}).Return(list, nil)

	itemBackupper.On("backupItem", mock.AnythingOfType("*logrus.Entry"), ns2, kuberesource.Namespaces).Return(nil)

	err := rb.backupResource(v1Group, namespacesResource)
	require.NoError(t, err)
}

type mockItemBackupperFactory struct {
	mock.Mock
}

func (ibf *mockItemBackupperFactory) newItemBackupper(
	backup *Request,
	backedUpItems map[itemKey]struct{},
	podCommandExecutor podexec.PodCommandExecutor,
	tarWriter tarWriter,
	dynamicFactory client.DynamicFactory,
	discoveryHelper discovery.Helper,
	resticBackupper restic.Backupper,
	resticSnapshotTracker *pvcSnapshotTracker,
	blockStoreGetter volumeinterface.BlockStoreGetter,
) ItemBackupper {
	args := ibf.Called(
		backup,
		backedUpItems,
		podCommandExecutor,
		tarWriter,
		dynamicFactory,
		discoveryHelper,
		resticBackupper,
		resticSnapshotTracker,
		blockStoreGetter,
	)
	return args.Get(0).(ItemBackupper)
}
