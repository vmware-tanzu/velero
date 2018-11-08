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

package restore

import (
	"encoding/json"
	"testing"
	"time"

	"k8s.io/client-go/kubernetes/scheme"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/watch"
	corev1 "k8s.io/client-go/kubernetes/typed/core/v1"

	api "github.com/heptio/ark/pkg/apis/ark/v1"
	"github.com/heptio/ark/pkg/cloudprovider"
	"github.com/heptio/ark/pkg/kuberesource"
	"github.com/heptio/ark/pkg/util/boolptr"
	"github.com/heptio/ark/pkg/util/collections"
	arktest "github.com/heptio/ark/pkg/util/test"
)

func TestPrioritizeResources(t *testing.T) {
	tests := []struct {
		name         string
		apiResources map[string][]string
		priorities   []string
		includes     []string
		excludes     []string
		expected     []string
	}{
		{
			name: "priorities & ordering are correctly applied",
			apiResources: map[string][]string{
				"v1": {"aaa", "bbb", "configmaps", "ddd", "namespaces", "ooo", "pods", "sss"},
			},
			priorities: []string{"namespaces", "configmaps", "pods"},
			includes:   []string{"*"},
			expected:   []string{"namespaces", "configmaps", "pods", "aaa", "bbb", "ddd", "ooo", "sss"},
		},
		{
			name: "includes are correctly applied",
			apiResources: map[string][]string{
				"v1": {"aaa", "bbb", "configmaps", "ddd", "namespaces", "ooo", "pods", "sss"},
			},
			priorities: []string{"namespaces", "configmaps", "pods"},
			includes:   []string{"namespaces", "aaa", "sss"},
			expected:   []string{"namespaces", "aaa", "sss"},
		},
		{
			name: "excludes are correctly applied",
			apiResources: map[string][]string{
				"v1": {"aaa", "bbb", "configmaps", "ddd", "namespaces", "ooo", "pods", "sss"},
			},
			priorities: []string{"namespaces", "configmaps", "pods"},
			includes:   []string{"*"},
			excludes:   []string{"ooo", "pods"},
			expected:   []string{"namespaces", "configmaps", "aaa", "bbb", "ddd", "sss"},
		},
	}

	logger := arktest.NewLogger()

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var helperResourceList []*metav1.APIResourceList

			for gv, resources := range test.apiResources {
				resourceList := &metav1.APIResourceList{GroupVersion: gv}
				for _, resource := range resources {
					resourceList.APIResources = append(resourceList.APIResources, metav1.APIResource{Name: resource})
				}
				helperResourceList = append(helperResourceList, resourceList)
			}

			helper := arktest.NewFakeDiscoveryHelper(true, nil)
			helper.ResourceList = helperResourceList

			includesExcludes := collections.NewIncludesExcludes().Includes(test.includes...).Excludes(test.excludes...)

			result, err := prioritizeResources(helper, test.priorities, includesExcludes, logger)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			require.Equal(t, len(test.expected), len(result))

			for i := range result {
				if e, a := test.expected[i], result[i].Resource; e != a {
					t.Errorf("index %d, expected %s, got %s", i, e, a)
				}
			}
		})
	}
}

func TestRestoreNamespaceFiltering(t *testing.T) {
	tests := []struct {
		name                 string
		fileSystem           *arktest.FakeFileSystem
		baseDir              string
		restore              *api.Restore
		expectedReadDirs     []string
		prioritizedResources []schema.GroupResource
	}{
		{
			name:             "namespacesToRestore having * restores all namespaces",
			fileSystem:       arktest.NewFakeFileSystem().WithDirectories("bak/resources/nodes/cluster", "bak/resources/secrets/namespaces/a", "bak/resources/secrets/namespaces/b", "bak/resources/secrets/namespaces/c"),
			baseDir:          "bak",
			restore:          &api.Restore{Spec: api.RestoreSpec{IncludedNamespaces: []string{"*"}}},
			expectedReadDirs: []string{"bak/resources", "bak/resources/nodes/cluster", "bak/resources/secrets/namespaces", "bak/resources/secrets/namespaces/a", "bak/resources/secrets/namespaces/b", "bak/resources/secrets/namespaces/c"},
			prioritizedResources: []schema.GroupResource{
				{Resource: "nodes"},
				{Resource: "secrets"},
			},
		},
		{
			name:             "namespacesToRestore properly filters",
			fileSystem:       arktest.NewFakeFileSystem().WithDirectories("bak/resources/nodes/cluster", "bak/resources/secrets/namespaces/a", "bak/resources/secrets/namespaces/b", "bak/resources/secrets/namespaces/c"),
			baseDir:          "bak",
			restore:          &api.Restore{Spec: api.RestoreSpec{IncludedNamespaces: []string{"b", "c"}}},
			expectedReadDirs: []string{"bak/resources", "bak/resources/nodes/cluster", "bak/resources/secrets/namespaces", "bak/resources/secrets/namespaces/b", "bak/resources/secrets/namespaces/c"},
			prioritizedResources: []schema.GroupResource{
				{Resource: "nodes"},
				{Resource: "secrets"},
			},
		},
		{
			name:             "namespacesToRestore properly filters with exclusion filter",
			fileSystem:       arktest.NewFakeFileSystem().WithDirectories("bak/resources/nodes/cluster", "bak/resources/secrets/namespaces/a", "bak/resources/secrets/namespaces/b", "bak/resources/secrets/namespaces/c"),
			baseDir:          "bak",
			restore:          &api.Restore{Spec: api.RestoreSpec{IncludedNamespaces: []string{"*"}, ExcludedNamespaces: []string{"a"}}},
			expectedReadDirs: []string{"bak/resources", "bak/resources/nodes/cluster", "bak/resources/secrets/namespaces", "bak/resources/secrets/namespaces/b", "bak/resources/secrets/namespaces/c"},
			prioritizedResources: []schema.GroupResource{
				{Resource: "nodes"},
				{Resource: "secrets"},
			},
		},
		{
			name:       "namespacesToRestore properly filters with inclusion & exclusion filters",
			fileSystem: arktest.NewFakeFileSystem().WithDirectories("bak/resources/nodes/cluster", "bak/resources/secrets/namespaces/a", "bak/resources/secrets/namespaces/b", "bak/resources/secrets/namespaces/c"),
			baseDir:    "bak",
			restore: &api.Restore{
				Spec: api.RestoreSpec{
					IncludedNamespaces: []string{"a", "b", "c"},
					ExcludedNamespaces: []string{"b"},
				},
			},
			expectedReadDirs: []string{"bak/resources", "bak/resources/nodes/cluster", "bak/resources/secrets/namespaces", "bak/resources/secrets/namespaces/a", "bak/resources/secrets/namespaces/c"},
			prioritizedResources: []schema.GroupResource{
				{Resource: "nodes"},
				{Resource: "secrets"},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			log := arktest.NewLogger()

			ctx := &context{
				restore:              test.restore,
				namespaceClient:      &fakeNamespaceClient{},
				fileSystem:           test.fileSystem,
				log:                  log,
				prioritizedResources: test.prioritizedResources,
			}

			warnings, errors := ctx.restoreFromDir(test.baseDir)

			assert.Empty(t, warnings.Ark)
			assert.Empty(t, warnings.Cluster)
			assert.Empty(t, warnings.Namespaces)
			assert.Empty(t, errors.Ark)
			assert.Empty(t, errors.Cluster)
			assert.Empty(t, errors.Namespaces)
			assert.Equal(t, test.expectedReadDirs, test.fileSystem.ReadDirCalls)
		})
	}
}

func TestRestorePriority(t *testing.T) {
	tests := []struct {
		name                 string
		fileSystem           *arktest.FakeFileSystem
		restore              *api.Restore
		baseDir              string
		prioritizedResources []schema.GroupResource
		expectedErrors       api.RestoreResult
		expectedReadDirs     []string
	}{
		{
			name:       "cluster test",
			fileSystem: arktest.NewFakeFileSystem().WithDirectory("bak/resources/a/cluster").WithDirectory("bak/resources/c/cluster"),
			baseDir:    "bak",
			restore:    &api.Restore{Spec: api.RestoreSpec{IncludedNamespaces: []string{"*"}}},
			prioritizedResources: []schema.GroupResource{
				{Resource: "a"},
				{Resource: "b"},
				{Resource: "c"},
			},
			expectedReadDirs: []string{"bak/resources", "bak/resources/a/cluster", "bak/resources/c/cluster"},
		},
		{
			name:       "resource priorities are applied",
			fileSystem: arktest.NewFakeFileSystem().WithDirectory("bak/resources/a/cluster").WithDirectory("bak/resources/c/cluster"),
			restore:    &api.Restore{Spec: api.RestoreSpec{IncludedNamespaces: []string{"*"}}},
			baseDir:    "bak",
			prioritizedResources: []schema.GroupResource{
				{Resource: "c"},
				{Resource: "b"},
				{Resource: "a"},
			},
			expectedReadDirs: []string{"bak/resources", "bak/resources/c/cluster", "bak/resources/a/cluster"},
		},
		{
			name:       "basic namespace",
			fileSystem: arktest.NewFakeFileSystem().WithDirectory("bak/resources/a/namespaces/ns-1").WithDirectory("bak/resources/c/namespaces/ns-1"),
			restore:    &api.Restore{Spec: api.RestoreSpec{IncludedNamespaces: []string{"*"}}},
			baseDir:    "bak",
			prioritizedResources: []schema.GroupResource{
				{Resource: "a"},
				{Resource: "b"},
				{Resource: "c"},
			},
			expectedReadDirs: []string{"bak/resources", "bak/resources/a/namespaces", "bak/resources/a/namespaces/ns-1", "bak/resources/c/namespaces", "bak/resources/c/namespaces/ns-1"},
		},
		{
			name: "error in a single resource doesn't terminate restore immediately, but is returned",
			fileSystem: arktest.NewFakeFileSystem().
				WithFile("bak/resources/a/namespaces/ns-1/invalid-json.json", []byte("invalid json")).
				WithDirectory("bak/resources/c/namespaces/ns-1"),
			restore: &api.Restore{Spec: api.RestoreSpec{IncludedNamespaces: []string{"*"}}},
			baseDir: "bak",
			prioritizedResources: []schema.GroupResource{
				{Resource: "a"},
				{Resource: "b"},
				{Resource: "c"},
			},
			expectedErrors: api.RestoreResult{
				Namespaces: map[string][]string{
					"ns-1": {"error decoding \"bak/resources/a/namespaces/ns-1/invalid-json.json\": invalid character 'i' looking for beginning of value"},
				},
			},
			expectedReadDirs: []string{"bak/resources", "bak/resources/a/namespaces", "bak/resources/a/namespaces/ns-1", "bak/resources/c/namespaces", "bak/resources/c/namespaces/ns-1"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			log := arktest.NewLogger()

			ctx := &context{
				restore:              test.restore,
				namespaceClient:      &fakeNamespaceClient{},
				fileSystem:           test.fileSystem,
				prioritizedResources: test.prioritizedResources,
				log:                  log,
			}

			warnings, errors := ctx.restoreFromDir(test.baseDir)

			assert.Empty(t, warnings.Ark)
			assert.Empty(t, warnings.Cluster)
			assert.Empty(t, warnings.Namespaces)
			assert.Equal(t, test.expectedErrors, errors)

			assert.Equal(t, test.expectedReadDirs, test.fileSystem.ReadDirCalls)
		})
	}
}

func TestNamespaceRemapping(t *testing.T) {
	var (
		baseDir              = "bak"
		restore              = &api.Restore{Spec: api.RestoreSpec{IncludedNamespaces: []string{"*"}, NamespaceMapping: map[string]string{"ns-1": "ns-2"}}}
		prioritizedResources = []schema.GroupResource{{Resource: "namespaces"}, {Resource: "configmaps"}}
		labelSelector        = labels.NewSelector()
		fileSystem           = arktest.NewFakeFileSystem().
					WithFile("bak/resources/configmaps/namespaces/ns-1/cm-1.json", newTestConfigMap().WithNamespace("ns-1").ToJSON()).
					WithFile("bak/resources/namespaces/cluster/ns-1.json", newTestNamespace("ns-1").ToJSON())
		expectedNS   = "ns-2"
		expectedObjs = toUnstructured(newTestConfigMap().WithNamespace("ns-2").WithArkLabel("").ConfigMap)
	)

	resourceClient := &arktest.FakeDynamicClient{}
	for i := range expectedObjs {
		resourceClient.On("Create", &expectedObjs[i]).Return(&expectedObjs[i], nil)
	}

	dynamicFactory := &arktest.FakeDynamicFactory{}
	resource := metav1.APIResource{Name: "configmaps", Namespaced: true}
	gv := schema.GroupVersion{Group: "", Version: "v1"}
	dynamicFactory.On("ClientForGroupVersionResource", gv, resource, expectedNS).Return(resourceClient, nil)

	namespaceClient := &fakeNamespaceClient{}

	ctx := &context{
		dynamicFactory:       dynamicFactory,
		fileSystem:           fileSystem,
		selector:             labelSelector,
		namespaceClient:      namespaceClient,
		prioritizedResources: prioritizedResources,
		restore:              restore,
		backup:               &api.Backup{},
		log:                  arktest.NewLogger(),
	}

	warnings, errors := ctx.restoreFromDir(baseDir)

	assert.Empty(t, warnings.Ark)
	assert.Empty(t, warnings.Cluster)
	assert.Empty(t, warnings.Namespaces)
	assert.Empty(t, errors.Ark)
	assert.Empty(t, errors.Cluster)
	assert.Empty(t, errors.Namespaces)

	// ensure the remapped NS (only) was created via the namespaceClient
	assert.Equal(t, 1, len(namespaceClient.createdNamespaces))
	assert.Equal(t, "ns-2", namespaceClient.createdNamespaces[0].Name)

	// ensure that we did not try to create namespaces via dynamic client
	dynamicFactory.AssertNotCalled(t, "ClientForGroupVersionResource", gv, metav1.APIResource{Name: "namespaces", Namespaced: true}, "")

	dynamicFactory.AssertExpectations(t)
	resourceClient.AssertExpectations(t)
}

func TestRestoreResourceForNamespace(t *testing.T) {
	var (
		trueVal  = true
		falseVal = false
		truePtr  = &trueVal
		falsePtr = &falseVal
	)

	tests := []struct {
		name                    string
		namespace               string
		resourcePath            string
		labelSelector           labels.Selector
		includeClusterResources *bool
		fileSystem              *arktest.FakeFileSystem
		actions                 []resolvedAction
		expectedErrors          api.RestoreResult
		expectedObjs            []unstructured.Unstructured
	}{
		{
			name:          "basic normal case",
			namespace:     "ns-1",
			resourcePath:  "configmaps",
			labelSelector: labels.NewSelector(),
			fileSystem: arktest.NewFakeFileSystem().
				WithFile("configmaps/cm-1.json", newNamedTestConfigMap("cm-1").ToJSON()).
				WithFile("configmaps/cm-2.json", newNamedTestConfigMap("cm-2").ToJSON()),
			expectedObjs: toUnstructured(
				newNamedTestConfigMap("cm-1").WithArkLabel("my-restore").ConfigMap,
				newNamedTestConfigMap("cm-2").WithArkLabel("my-restore").ConfigMap,
			),
		},
		{
			name:         "no such directory causes error",
			namespace:    "ns-1",
			resourcePath: "configmaps",
			fileSystem:   arktest.NewFakeFileSystem(),
			expectedErrors: api.RestoreResult{
				Namespaces: map[string][]string{
					"ns-1": {"error reading \"configmaps\" resource directory: open configmaps: file does not exist"},
				},
			},
		},
		{
			name:         "empty directory is no-op",
			namespace:    "ns-1",
			resourcePath: "configmaps",
			fileSystem:   arktest.NewFakeFileSystem().WithDirectory("configmaps"),
		},
		{
			name:          "unmarshall failure does not cause immediate return",
			namespace:     "ns-1",
			resourcePath:  "configmaps",
			labelSelector: labels.NewSelector(),
			fileSystem: arktest.NewFakeFileSystem().
				WithFile("configmaps/cm-1-invalid.json", []byte("this is not valid json")).
				WithFile("configmaps/cm-2.json", newNamedTestConfigMap("cm-2").ToJSON()),
			expectedErrors: api.RestoreResult{
				Namespaces: map[string][]string{
					"ns-1": {"error decoding \"configmaps/cm-1-invalid.json\": invalid character 'h' in literal true (expecting 'r')"},
				},
			},
			expectedObjs: toUnstructured(newNamedTestConfigMap("cm-2").WithArkLabel("my-restore").ConfigMap),
		},
		{
			name:          "matching label selector correctly includes",
			namespace:     "ns-1",
			resourcePath:  "configmaps",
			labelSelector: labels.SelectorFromSet(labels.Set(map[string]string{"foo": "bar"})),
			fileSystem:    arktest.NewFakeFileSystem().WithFile("configmaps/cm-1.json", newTestConfigMap().WithLabels(map[string]string{"foo": "bar"}).ToJSON()),
			expectedObjs:  toUnstructured(newTestConfigMap().WithLabels(map[string]string{"foo": "bar"}).WithArkLabel("my-restore").ConfigMap),
		},
		{
			name:          "non-matching label selector correctly excludes",
			namespace:     "ns-1",
			resourcePath:  "configmaps",
			labelSelector: labels.SelectorFromSet(labels.Set(map[string]string{"foo": "not-bar"})),
			fileSystem:    arktest.NewFakeFileSystem().WithFile("configmaps/cm-1.json", newTestConfigMap().WithLabels(map[string]string{"foo": "bar"}).ToJSON()),
		},
		{
			name:          "namespace is remapped",
			namespace:     "ns-2",
			resourcePath:  "configmaps",
			labelSelector: labels.NewSelector(),
			fileSystem:    arktest.NewFakeFileSystem().WithFile("configmaps/cm-1.json", newTestConfigMap().WithNamespace("ns-1").ToJSON()),
			expectedObjs:  toUnstructured(newTestConfigMap().WithNamespace("ns-2").WithArkLabel("my-restore").ConfigMap),
		},
		{
			name:          "custom restorer is correctly used",
			namespace:     "ns-1",
			resourcePath:  "configmaps",
			labelSelector: labels.NewSelector(),
			fileSystem:    arktest.NewFakeFileSystem().WithFile("configmaps/cm-1.json", newTestConfigMap().ToJSON()),
			actions: []resolvedAction{
				{
					ItemAction:                newFakeAction("configmaps"),
					resourceIncludesExcludes:  collections.NewIncludesExcludes().Includes("configmaps"),
					namespaceIncludesExcludes: collections.NewIncludesExcludes(),
					selector:                  labels.Everything(),
				},
			},
			expectedObjs: toUnstructured(newTestConfigMap().WithLabels(map[string]string{"fake-restorer": "foo"}).WithArkLabel("my-restore").ConfigMap),
		},
		{
			name:          "custom restorer for different group/resource is not used",
			namespace:     "ns-1",
			resourcePath:  "configmaps",
			labelSelector: labels.NewSelector(),
			fileSystem:    arktest.NewFakeFileSystem().WithFile("configmaps/cm-1.json", newTestConfigMap().ToJSON()),
			actions: []resolvedAction{
				{
					ItemAction:                newFakeAction("foo-resource"),
					resourceIncludesExcludes:  collections.NewIncludesExcludes().Includes("foo-resource"),
					namespaceIncludesExcludes: collections.NewIncludesExcludes(),
					selector:                  labels.Everything(),
				},
			},
			expectedObjs: toUnstructured(newTestConfigMap().WithArkLabel("my-restore").ConfigMap),
		},
		{
			name:                    "cluster-scoped resources are skipped when IncludeClusterResources=false",
			namespace:               "",
			resourcePath:            "persistentvolumes",
			labelSelector:           labels.NewSelector(),
			includeClusterResources: falsePtr,
			fileSystem:              arktest.NewFakeFileSystem().WithFile("persistentvolumes/pv-1.json", newTestPV().ToJSON()),
		},
		{
			name:                    "namespaced resources are not skipped when IncludeClusterResources=false",
			namespace:               "ns-1",
			resourcePath:            "configmaps",
			labelSelector:           labels.NewSelector(),
			includeClusterResources: falsePtr,
			fileSystem:              arktest.NewFakeFileSystem().WithFile("configmaps/cm-1.json", newTestConfigMap().ToJSON()),
			expectedObjs:            toUnstructured(newTestConfigMap().WithArkLabel("my-restore").ConfigMap),
		},
		{
			name:                    "cluster-scoped resources are not skipped when IncludeClusterResources=true",
			namespace:               "",
			resourcePath:            "persistentvolumes",
			labelSelector:           labels.NewSelector(),
			includeClusterResources: truePtr,
			fileSystem:              arktest.NewFakeFileSystem().WithFile("persistentvolumes/pv-1.json", newTestPV().ToJSON()),
			expectedObjs:            toUnstructured(newTestPV().WithArkLabel("my-restore").PersistentVolume),
		},
		{
			name:                    "namespaced resources are not skipped when IncludeClusterResources=true",
			namespace:               "ns-1",
			resourcePath:            "configmaps",
			labelSelector:           labels.NewSelector(),
			includeClusterResources: truePtr,
			fileSystem:              arktest.NewFakeFileSystem().WithFile("configmaps/cm-1.json", newTestConfigMap().ToJSON()),
			expectedObjs:            toUnstructured(newTestConfigMap().WithArkLabel("my-restore").ConfigMap),
		},
		{
			name:                    "cluster-scoped resources are not skipped when IncludeClusterResources=nil",
			namespace:               "",
			resourcePath:            "persistentvolumes",
			labelSelector:           labels.NewSelector(),
			includeClusterResources: nil,
			fileSystem:              arktest.NewFakeFileSystem().WithFile("persistentvolumes/pv-1.json", newTestPV().ToJSON()),
			expectedObjs:            toUnstructured(newTestPV().WithArkLabel("my-restore").PersistentVolume),
		},
		{
			name:                    "namespaced resources are not skipped when IncludeClusterResources=nil",
			namespace:               "ns-1",
			resourcePath:            "configmaps",
			labelSelector:           labels.NewSelector(),
			includeClusterResources: nil,
			fileSystem:              arktest.NewFakeFileSystem().WithFile("configmaps/cm-1.json", newTestConfigMap().ToJSON()),
			expectedObjs:            toUnstructured(newTestConfigMap().WithArkLabel("my-restore").ConfigMap),
		},
		{
			name:                    "serviceaccounts are restored",
			namespace:               "ns-1",
			resourcePath:            "serviceaccounts",
			labelSelector:           labels.NewSelector(),
			includeClusterResources: nil,
			fileSystem:              arktest.NewFakeFileSystem().WithFile("serviceaccounts/sa-1.json", newTestServiceAccount().ToJSON()),
			expectedObjs:            toUnstructured(newTestServiceAccount().WithArkLabel("my-restore").ServiceAccount),
		},
		{
			name:                    "non-mirror pods are restored",
			namespace:               "ns-1",
			resourcePath:            "pods",
			labelSelector:           labels.NewSelector(),
			includeClusterResources: nil,
			fileSystem: arktest.NewFakeFileSystem().
				WithFile(
					"pods/pod.json",
					NewTestUnstructured().
						WithAPIVersion("v1").
						WithKind("Pod").
						WithNamespace("ns-1").
						WithName("pod1").
						ToJSON(),
				),
			expectedObjs: []unstructured.Unstructured{
				*(NewTestUnstructured().
					WithAPIVersion("v1").
					WithKind("Pod").
					WithNamespace("ns-1").
					WithName("pod1").
					WithArkLabel("my-restore").
					Unstructured),
			},
		},
		{
			name:                    "mirror pods are not restored",
			namespace:               "ns-1",
			resourcePath:            "pods",
			labelSelector:           labels.NewSelector(),
			includeClusterResources: nil,
			fileSystem: arktest.NewFakeFileSystem().
				WithFile(
					"pods/pod.json",
					NewTestUnstructured().
						WithAPIVersion("v1").
						WithKind("Pod").
						WithNamespace("ns-1").
						WithName("pod1").
						WithAnnotations(v1.MirrorPodAnnotationKey).
						ToJSON(),
				),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			resourceClient := &arktest.FakeDynamicClient{}
			for i := range test.expectedObjs {
				resourceClient.On("Create", &test.expectedObjs[i]).Return(&test.expectedObjs[i], nil)
			}

			dynamicFactory := &arktest.FakeDynamicFactory{}
			gv := schema.GroupVersion{Group: "", Version: "v1"}

			configMapResource := metav1.APIResource{Name: "configmaps", Namespaced: true}
			dynamicFactory.On("ClientForGroupVersionResource", gv, configMapResource, test.namespace).Return(resourceClient, nil)

			pvResource := metav1.APIResource{Name: "persistentvolumes", Namespaced: false}
			dynamicFactory.On("ClientForGroupVersionResource", gv, pvResource, test.namespace).Return(resourceClient, nil)
			resourceClient.On("Watch", metav1.ListOptions{}).Return(&fakeWatch{}, nil)

			var empty *unstructured.Unstructured
			resourceClient.On("Get", newTestPV().PersistentVolume.Name, metav1.GetOptions{}).Return(empty, nil)

			saResource := metav1.APIResource{Name: "serviceaccounts", Namespaced: true}
			dynamicFactory.On("ClientForGroupVersionResource", gv, saResource, test.namespace).Return(resourceClient, nil)

			podResource := metav1.APIResource{Name: "pods", Namespaced: true}
			dynamicFactory.On("ClientForGroupVersionResource", gv, podResource, test.namespace).Return(resourceClient, nil)

			ctx := &context{
				dynamicFactory: dynamicFactory,
				actions:        test.actions,
				fileSystem:     test.fileSystem,
				selector:       test.labelSelector,
				restore: &api.Restore{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: api.DefaultNamespace,
						Name:      "my-restore",
					},
					Spec: api.RestoreSpec{
						IncludeClusterResources: test.includeClusterResources,
					},
				},
				backup:     &api.Backup{},
				log:        arktest.NewLogger(),
				pvRestorer: &pvRestorer{},
			}

			warnings, errors := ctx.restoreResource(test.resourcePath, test.namespace, test.resourcePath)

			assert.Empty(t, warnings.Ark)
			assert.Empty(t, warnings.Cluster)
			assert.Empty(t, warnings.Namespaces)
			assert.Equal(t, test.expectedErrors, errors)
		})
	}
}

func TestRestoringExistingServiceAccount(t *testing.T) {
	fromCluster := newTestServiceAccount()
	fromClusterUnstructured, err := runtime.DefaultUnstructuredConverter.ToUnstructured(fromCluster.ServiceAccount)
	require.NoError(t, err)

	different := newTestServiceAccount().WithImagePullSecret("image-secret").WithSecret("secret")
	differentUnstructured, err := runtime.DefaultUnstructuredConverter.ToUnstructured(different.ServiceAccount)
	require.NoError(t, err)

	tests := []struct {
		name          string
		expectedPatch []byte
		fromBackup    *unstructured.Unstructured
	}{
		{
			name:       "fromCluster and fromBackup are exactly the same",
			fromBackup: &unstructured.Unstructured{Object: fromClusterUnstructured},
		},
		{
			name:          "fromCluster and fromBackup are different",
			fromBackup:    &unstructured.Unstructured{Object: differentUnstructured},
			expectedPatch: []byte(`{"imagePullSecrets":[{"name":"image-secret"}],"secrets":[{"name":"secret"}]}`),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			resourceClient := &arktest.FakeDynamicClient{}
			defer resourceClient.AssertExpectations(t)
			name := fromCluster.GetName()

			// restoreResource will add the restore label to object provided to create, so we need to make a copy to provide to our expected call
			m := make(map[string]interface{})
			for k, v := range test.fromBackup.Object {
				m[k] = v
			}
			fromBackupWithLabel := &unstructured.Unstructured{Object: m}
			l := map[string]string{api.RestoreLabelKey: "my-restore"}
			fromBackupWithLabel.SetLabels(l)
			// resetMetadataAndStatus will strip the creationTimestamp before calling Create
			fromBackupWithLabel.SetCreationTimestamp(metav1.Time{Time: time.Time{}})

			resourceClient.On("Create", fromBackupWithLabel).Return(new(unstructured.Unstructured), k8serrors.NewAlreadyExists(kuberesource.ServiceAccounts, name))
			resourceClient.On("Get", name, metav1.GetOptions{}).Return(&unstructured.Unstructured{Object: fromClusterUnstructured}, nil)

			if len(test.expectedPatch) > 0 {
				resourceClient.On("Patch", name, test.expectedPatch).Return(test.fromBackup, nil)
			}

			dynamicFactory := &arktest.FakeDynamicFactory{}
			gv := schema.GroupVersion{Group: "", Version: "v1"}

			resource := metav1.APIResource{Name: "serviceaccounts", Namespaced: true}
			dynamicFactory.On("ClientForGroupVersionResource", gv, resource, "ns-1").Return(resourceClient, nil)
			fromBackupJSON, err := json.Marshal(test.fromBackup)
			require.NoError(t, err)
			ctx := &context{
				dynamicFactory: dynamicFactory,
				actions:        []resolvedAction{},
				fileSystem: arktest.NewFakeFileSystem().
					WithFile("foo/resources/serviceaccounts/namespaces/ns-1/sa-1.json", fromBackupJSON),
				selector: labels.NewSelector(),
				restore: &api.Restore{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: api.DefaultNamespace,
						Name:      "my-restore",
					},
					Spec: api.RestoreSpec{
						IncludeClusterResources: nil,
					},
				},
				backup: &api.Backup{},
				log:    arktest.NewLogger(),
			}
			warnings, errors := ctx.restoreResource("serviceaccounts", "ns-1", "foo/resources/serviceaccounts/namespaces/ns-1/")

			assert.Empty(t, warnings.Ark)
			assert.Empty(t, warnings.Cluster)
			assert.Empty(t, warnings.Namespaces)
			assert.Equal(t, api.RestoreResult{}, errors)
		})
	}
}

func TestRestoringPVsWithoutSnapshots(t *testing.T) {
	pv := `apiVersion: v1
kind: PersistentVolume
metadata:
  annotations:
    EXPORT_block: "\nEXPORT\n{\n\tExport_Id = 1;\n\tPath = /export/pvc-6a74b5af-78a5-11e8-a0d8-e2ad1e9734ce;\n\tPseudo
      = /export/pvc-6a74b5af-78a5-11e8-a0d8-e2ad1e9734ce;\n\tAccess_Type = RW;\n\tSquash
      = no_root_squash;\n\tSecType = sys;\n\tFilesystem_id = 1.1;\n\tFSAL {\n\t\tName
      = VFS;\n\t}\n}\n"
    Export_Id: "1"
    Project_Id: "0"
    Project_block: ""
    Provisioner_Id: 5fdf4025-78a5-11e8-9ece-0242ac110004
    kubernetes.io/createdby: nfs-dynamic-provisioner
    pv.kubernetes.io/provisioned-by: example.com/nfs
    volume.beta.kubernetes.io/mount-options: vers=4.1
  creationTimestamp: 2018-06-25T18:27:35Z
  finalizers:
  - kubernetes.io/pv-protection
  name: pvc-6a74b5af-78a5-11e8-a0d8-e2ad1e9734ce
  resourceVersion: "2576"
  selfLink: /api/v1/persistentvolumes/pvc-6a74b5af-78a5-11e8-a0d8-e2ad1e9734ce
  uid: 6ecd24e4-78a5-11e8-a0d8-e2ad1e9734ce
spec:
  accessModes:
  - ReadWriteMany
  capacity:
    storage: 1Mi
  claimRef:
    apiVersion: v1
    kind: PersistentVolumeClaim
    name: nfs
    namespace: default
    resourceVersion: "2565"
    uid: 6a74b5af-78a5-11e8-a0d8-e2ad1e9734ce
  nfs:
    path: /export/pvc-6a74b5af-78a5-11e8-a0d8-e2ad1e9734ce
    server: 10.103.235.254
  storageClassName: example-nfs
status:
  phase: Bound`

	pvc := `apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  annotations:
    control-plane.alpha.kubernetes.io/leader: '{"holderIdentity":"5fdf5572-78a5-11e8-9ece-0242ac110004","leaseDurationSeconds":15,"acquireTime":"2018-06-25T18:27:35Z","renewTime":"2018-06-25T18:27:37Z","leaderTransitions":0}'
    kubectl.kubernetes.io/last-applied-configuration: |
      {"apiVersion":"v1","kind":"PersistentVolumeClaim","metadata":{"annotations":{},"name":"nfs","namespace":"default"},"spec":{"accessModes":["ReadWriteMany"],"resources":{"requests":{"storage":"1Mi"}},"storageClassName":"example-nfs"}}
    pv.kubernetes.io/bind-completed: "yes"
    pv.kubernetes.io/bound-by-controller: "yes"
    volume.beta.kubernetes.io/storage-provisioner: example.com/nfs
  creationTimestamp: 2018-06-25T18:27:28Z
  finalizers:
  - kubernetes.io/pvc-protection
  name: nfs
  namespace: default
  resourceVersion: "2578"
  selfLink: /api/v1/namespaces/default/persistentvolumeclaims/nfs
  uid: 6a74b5af-78a5-11e8-a0d8-e2ad1e9734ce
spec:
  accessModes:
  - ReadWriteMany
  resources:
    requests:
      storage: 1Mi
  storageClassName: example-nfs
  volumeName: pvc-6a74b5af-78a5-11e8-a0d8-e2ad1e9734ce
status:
  accessModes:
  - ReadWriteMany
  capacity:
    storage: 1Mi
  phase: Bound`

	tests := []struct {
		name                          string
		haveSnapshot                  bool
		reclaimPolicy                 string
		expectPVCVolumeName           bool
		expectedPVCAnnotationsMissing sets.String
		expectPVCreation              bool
		expectPVFound                 bool
	}{
		{
			name:                "have snapshot, reclaim policy delete, no existing PV found",
			haveSnapshot:        true,
			reclaimPolicy:       "Delete",
			expectPVCVolumeName: true,
			expectPVCreation:    true,
		},
		{
			name:                "have snapshot, reclaim policy retain, no existing PV found",
			haveSnapshot:        true,
			reclaimPolicy:       "Retain",
			expectPVCVolumeName: true,
			expectPVCreation:    true,
		},
		{
			name:                "have snapshot, reclaim policy retain, existing PV found",
			haveSnapshot:        true,
			reclaimPolicy:       "Retain",
			expectPVCVolumeName: true,
			expectPVCreation:    false,
			expectPVFound:       true,
		},
		{
			name:                          "no snapshot, reclaim policy delete, no existing PV",
			haveSnapshot:                  false,
			reclaimPolicy:                 "Delete",
			expectPVCVolumeName:           false,
			expectedPVCAnnotationsMissing: sets.NewString("pv.kubernetes.io/bind-completed", "pv.kubernetes.io/bound-by-controller"),
		},
		{
			name:                "no snapshot, reclaim policy retain, existing PV found",
			haveSnapshot:        false,
			reclaimPolicy:       "Retain",
			expectPVCVolumeName: true,
			expectPVCreation:    false,
			expectPVFound:       true,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			dynamicFactory := &arktest.FakeDynamicFactory{}
			gv := schema.GroupVersion{Group: "", Version: "v1"}

			pvClient := &arktest.FakeDynamicClient{}
			defer pvClient.AssertExpectations(t)

			pvResource := metav1.APIResource{Name: "persistentvolumes", Namespaced: false}
			dynamicFactory.On("ClientForGroupVersionResource", gv, pvResource, "").Return(pvClient, nil)

			pvcClient := &arktest.FakeDynamicClient{}
			defer pvcClient.AssertExpectations(t)

			pvcResource := metav1.APIResource{Name: "persistentvolumeclaims", Namespaced: true}
			dynamicFactory.On("ClientForGroupVersionResource", gv, pvcResource, "default").Return(pvcClient, nil)

			obj, _, err := scheme.Codecs.UniversalDecoder(v1.SchemeGroupVersion).Decode([]byte(pv), nil, nil)
			require.NoError(t, err)
			pvObj, ok := obj.(*v1.PersistentVolume)
			require.True(t, ok)
			pvObj.Spec.PersistentVolumeReclaimPolicy = v1.PersistentVolumeReclaimPolicy(test.reclaimPolicy)
			pvBytes, err := json.Marshal(pvObj)
			require.NoError(t, err)

			obj, _, err = scheme.Codecs.UniversalDecoder(v1.SchemeGroupVersion).Decode([]byte(pvc), nil, nil)
			require.NoError(t, err)
			pvcObj, ok := obj.(*v1.PersistentVolumeClaim)
			require.True(t, ok)
			pvcBytes, err := json.Marshal(pvcObj)
			require.NoError(t, err)

			backup := &api.Backup{}
			if test.haveSnapshot {
				backup.Status.VolumeBackups = map[string]*api.VolumeBackupInfo{
					"pvc-6a74b5af-78a5-11e8-a0d8-e2ad1e9734ce": {
						SnapshotID: "snap",
					},
				}
			}

			pvRestorer := new(mockPVRestorer)
			defer pvRestorer.AssertExpectations(t)

			ctx := &context{
				dynamicFactory: dynamicFactory,
				actions:        []resolvedAction{},
				fileSystem: arktest.NewFakeFileSystem().
					WithFile("foo/resources/persistentvolumes/cluster/pv.json", pvBytes).
					WithFile("foo/resources/persistentvolumeclaims/default/pvc.json", pvcBytes),
				selector: labels.NewSelector(),
				prioritizedResources: []schema.GroupResource{
					kuberesource.PersistentVolumes,
					kuberesource.PersistentVolumeClaims,
				},
				restore: &api.Restore{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: api.DefaultNamespace,
						Name:      "my-restore",
					},
				},
				backup:         backup,
				log:            arktest.NewLogger(),
				pvsToProvision: sets.NewString(),
				pvRestorer:     pvRestorer,
			}

			pvWatch := new(mockWatch)
			defer pvWatch.AssertExpectations(t)

			unstructuredPVMap, err := runtime.DefaultUnstructuredConverter.ToUnstructured(pvObj)
			require.NoError(t, err)
			unstructuredPV := &unstructured.Unstructured{Object: unstructuredPVMap}

			if test.expectPVFound {
				pvClient.On("Get", unstructuredPV.GetName(), metav1.GetOptions{}).Return(unstructuredPV, nil)
				pvClient.On("Create", mock.Anything).Return(unstructuredPV, k8serrors.NewAlreadyExists(kuberesource.PersistentVolumes, unstructuredPV.GetName()))
			}
			// Only set up the client expectation if the test has the proper prerequisites
			if test.haveSnapshot || test.reclaimPolicy != "Delete" {
				pvClient.On("Get", unstructuredPV.GetName(), metav1.GetOptions{}).Return(&unstructured.Unstructured{}, k8serrors.NewNotFound(schema.GroupResource{Resource: "persistentvolumes"}, unstructuredPV.GetName()))
			}

			pvToRestore := unstructuredPV.DeepCopy()
			restoredPV := unstructuredPV.DeepCopy()

			if test.expectPVCreation {
				// just to ensure we have the data flowing correctly
				restoredPV.Object["foo"] = "bar"
				pvRestorer.On("executePVAction", pvToRestore).Return(restoredPV, nil)
			}

			resetMetadataAndStatus(unstructuredPV)
			addLabel(unstructuredPV, api.RestoreLabelKey, ctx.restore.Name)
			unstructuredPV.Object["foo"] = "bar"

			if test.expectPVCreation {
				createdPV := unstructuredPV.DeepCopy()
				pvClient.On("Create", unstructuredPV).Return(createdPV, nil)

				pvClient.On("Watch", metav1.ListOptions{}).Return(pvWatch, nil)
				pvWatchChan := make(chan watch.Event, 1)
				readyPV := restoredPV.DeepCopy()
				readyStatus, err := collections.GetMap(readyPV.Object, "status")
				require.NoError(t, err)
				readyStatus["phase"] = string(v1.VolumeAvailable)
				pvWatchChan <- watch.Event{
					Type:   watch.Modified,
					Object: readyPV,
				}
				pvWatch.On("ResultChan").Return(pvWatchChan)
			}

			// Restore PV
			warnings, errors := ctx.restoreResource("persistentvolumes", "", "foo/resources/persistentvolumes/cluster/")

			assert.Empty(t, warnings.Ark)
			assert.Empty(t, warnings.Namespaces)
			assert.Equal(t, api.RestoreResult{}, errors)

			if test.expectPVFound {
				assert.Equal(t, 1, len(warnings.Cluster))
			} else {
				assert.Empty(t, warnings.Cluster)
			}

			// Prep PVC restore
			// Handle expectations
			if !test.expectPVCVolumeName {
				pvcObj.Spec.VolumeName = ""
			}
			for _, key := range test.expectedPVCAnnotationsMissing.List() {
				delete(pvcObj.Annotations, key)
			}

			unstructuredPVCMap, err := runtime.DefaultUnstructuredConverter.ToUnstructured(pvcObj)
			require.NoError(t, err)
			unstructuredPVC := &unstructured.Unstructured{Object: unstructuredPVCMap}

			resetMetadataAndStatus(unstructuredPVC)
			addLabel(unstructuredPVC, api.RestoreLabelKey, ctx.restore.Name)

			createdPVC := unstructuredPVC.DeepCopy()
			// just to ensure we have the data flowing correctly
			createdPVC.Object["foo"] = "bar"

			pvcClient.On("Create", unstructuredPVC).Return(createdPVC, nil)

			// Restore PVC
			warnings, errors = ctx.restoreResource("persistentvolumeclaims", "default", "foo/resources/persistentvolumeclaims/default/")

			assert.Empty(t, warnings.Ark)
			assert.Empty(t, warnings.Cluster)
			assert.Empty(t, warnings.Namespaces)
			assert.Equal(t, api.RestoreResult{}, errors)

			ctx.resourceWaitGroup.Wait()
		})
	}
}

type mockPVRestorer struct {
	mock.Mock
}

func (r *mockPVRestorer) executePVAction(obj *unstructured.Unstructured) (*unstructured.Unstructured, error) {
	args := r.Called(obj)
	return args.Get(0).(*unstructured.Unstructured), args.Error(1)
}

type mockWatch struct {
	mock.Mock
}

func (w *mockWatch) Stop() {
	w.Called()
}

func (w *mockWatch) ResultChan() <-chan watch.Event {
	args := w.Called()
	return args.Get(0).(chan watch.Event)
}

type fakeWatch struct{}

func (w *fakeWatch) Stop() {}

func (w *fakeWatch) ResultChan() <-chan watch.Event {
	return make(chan watch.Event)
}

func TestHasControllerOwner(t *testing.T) {
	tests := []struct {
		name        string
		object      map[string]interface{}
		expectOwner bool
	}{
		{
			name:   "missing metadata",
			object: map[string]interface{}{},
		},
		{
			name: "missing ownerReferences",
			object: map[string]interface{}{
				"metadata": map[string]interface{}{},
			},
			expectOwner: false,
		},
		{
			name: "have ownerReferences, no controller fields",
			object: map[string]interface{}{
				"metadata": map[string]interface{}{
					"ownerReferences": []interface{}{
						map[string]interface{}{"foo": "bar"},
					},
				},
			},
			expectOwner: false,
		},
		{
			name: "have ownerReferences, controller=false",
			object: map[string]interface{}{
				"metadata": map[string]interface{}{
					"ownerReferences": []interface{}{
						map[string]interface{}{"controller": false},
					},
				},
			},
			expectOwner: false,
		},
		{
			name: "have ownerReferences, controller=true",
			object: map[string]interface{}{
				"metadata": map[string]interface{}{
					"ownerReferences": []interface{}{
						map[string]interface{}{"controller": false},
						map[string]interface{}{"controller": false},
						map[string]interface{}{"controller": true},
					},
				},
			},
			expectOwner: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			u := &unstructured.Unstructured{Object: test.object}
			hasOwner := hasControllerOwner(u.GetOwnerReferences())
			assert.Equal(t, test.expectOwner, hasOwner)
		})
	}
}

func TestResetMetadataAndStatus(t *testing.T) {
	tests := []struct {
		name        string
		obj         *unstructured.Unstructured
		expectedErr bool
		expectedRes *unstructured.Unstructured
	}{
		{
			name:        "no metadata causes error",
			obj:         NewTestUnstructured().Unstructured,
			expectedErr: true,
		},
		{
			name:        "keep name, namespace, labels, annotations only",
			obj:         NewTestUnstructured().WithMetadata("name", "blah", "namespace", "labels", "annotations", "foo").Unstructured,
			expectedErr: false,
			expectedRes: NewTestUnstructured().WithMetadata("name", "namespace", "labels", "annotations").Unstructured,
		},
		{
			name:        "don't keep status",
			obj:         NewTestUnstructured().WithMetadata().WithStatus().Unstructured,
			expectedErr: false,
			expectedRes: NewTestUnstructured().WithMetadata().Unstructured,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			res, err := resetMetadataAndStatus(test.obj)

			if assert.Equal(t, test.expectedErr, err != nil) {
				assert.Equal(t, test.expectedRes, res)
			}
		})
	}
}

func TestIsCompleted(t *testing.T) {
	tests := []struct {
		name          string
		expected      bool
		content       string
		groupResource schema.GroupResource
		expectedErr   bool
	}{
		{
			name:          "Failed pods are complete",
			expected:      true,
			content:       `{"apiVersion":"v1","kind":"Pod","metadata":{"namespace":"ns","name":"pod1"}, "status": {"phase": "Failed"}}`,
			groupResource: schema.GroupResource{Group: "", Resource: "pods"},
		},
		{
			name:          "Succeeded pods are complete",
			expected:      true,
			content:       `{"apiVersion":"v1","kind":"Pod","metadata":{"namespace":"ns","name":"pod1"}, "status": {"phase": "Succeeded"}}`,
			groupResource: schema.GroupResource{Group: "", Resource: "pods"},
		},
		{
			name:          "Pending pods aren't complete",
			expected:      false,
			content:       `{"apiVersion":"v1","kind":"Pod","metadata":{"namespace":"ns","name":"pod1"}, "status": {"phase": "Pending"}}`,
			groupResource: schema.GroupResource{Group: "", Resource: "pods"},
		},
		{
			name:          "Running pods aren't complete",
			expected:      false,
			content:       `{"apiVersion":"v1","kind":"Pod","metadata":{"namespace":"ns","name":"pod1"}, "status": {"phase": "Running"}}`,
			groupResource: schema.GroupResource{Group: "", Resource: "pods"},
		},
		{
			name:          "Jobs without a completion time aren't complete",
			expected:      false,
			content:       `{"apiVersion":"v1","kind":"Pod","metadata":{"namespace":"ns","name":"pod1"}}`,
			groupResource: schema.GroupResource{Group: "batch", Resource: "jobs"},
		},
		{
			name:          "Jobs with a completion time are completed",
			expected:      true,
			content:       `{"apiVersion":"v1","kind":"Pod","metadata":{"namespace":"ns","name":"pod1"}, "status": {"completionTime": "bar"}}`,
			groupResource: schema.GroupResource{Group: "batch", Resource: "jobs"},
		},
		{
			name:          "Jobs with an empty completion time are not completed",
			expected:      false,
			content:       `{"apiVersion":"v1","kind":"Pod","metadata":{"namespace":"ns","name":"pod1"}, "status": {"completionTime": ""}}`,
			groupResource: schema.GroupResource{Group: "batch", Resource: "jobs"},
		},
		{
			name:          "Something not a pod or a job may actually be complete, but we're not concerned with that",
			expected:      false,
			content:       `{"apiVersion": "v1", "kind": "Namespace", "metadata": {"name": "ns"}, "status": {"completionTime": "bar", "phase":"Completed"}}`,
			groupResource: schema.GroupResource{Group: "", Resource: "namespaces"},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			u := arktest.UnstructuredOrDie(test.content)
			backup, err := isCompleted(u, test.groupResource)

			if assert.Equal(t, test.expectedErr, err != nil) {
				assert.Equal(t, test.expected, backup)
			}
		})
	}
}

func TestExecutePVAction(t *testing.T) {
	iops := int64(1000)

	tests := []struct {
		name              string
		obj               *unstructured.Unstructured
		restore           *api.Restore
		backup            *api.Backup
		volumeMap         map[api.VolumeBackupInfo]string
		noSnapshotService bool
		expectedErr       bool
		expectedRes       *unstructured.Unstructured
		volumeID          string
		expectSetVolumeID bool
	}{
		{
			name:        "no name should error",
			obj:         NewTestUnstructured().WithMetadata().Unstructured,
			restore:     arktest.NewDefaultTestRestore().Restore,
			expectedErr: true,
		},
		{
			name:        "no spec should error",
			obj:         NewTestUnstructured().WithName("pv-1").Unstructured,
			restore:     arktest.NewDefaultTestRestore().Restore,
			expectedErr: true,
		},
		{
			name:        "ensure spec.claimRef, spec.storageClassName are deleted",
			obj:         NewTestUnstructured().WithName("pv-1").WithAnnotations("a", "b").WithSpec("claimRef", "storageClassName", "someOtherField").Unstructured,
			restore:     arktest.NewDefaultTestRestore().WithRestorePVs(false).Restore,
			backup:      &api.Backup{},
			expectedRes: NewTestUnstructured().WithAnnotations("a", "b").WithName("pv-1").WithSpec("someOtherField").Unstructured,
		},
		{
			name:        "if backup.spec.snapshotVolumes is false, ignore restore.spec.restorePVs and return early",
			obj:         NewTestUnstructured().WithName("pv-1").WithAnnotations("a", "b").WithSpec("claimRef", "storageClassName", "someOtherField").Unstructured,
			restore:     arktest.NewDefaultTestRestore().WithRestorePVs(true).Restore,
			backup:      &api.Backup{Spec: api.BackupSpec{SnapshotVolumes: boolptr.False()}},
			expectedRes: NewTestUnstructured().WithName("pv-1").WithAnnotations("a", "b").WithSpec("someOtherField").Unstructured,
		},
		{
			name:        "not restoring, return early",
			obj:         NewTestUnstructured().WithName("pv-1").WithSpec().Unstructured,
			restore:     arktest.NewDefaultTestRestore().WithRestorePVs(false).Restore,
			backup:      &api.Backup{Status: api.BackupStatus{VolumeBackups: map[string]*api.VolumeBackupInfo{"pv-1": {SnapshotID: "snap-1"}}}},
			expectedErr: false,
			expectedRes: NewTestUnstructured().WithName("pv-1").WithSpec().Unstructured,
		},
		{
			name:        "restoring, return without error if there is no PV->BackupInfo map",
			obj:         NewTestUnstructured().WithName("pv-1").WithSpec("xyz").Unstructured,
			restore:     arktest.NewDefaultTestRestore().WithRestorePVs(true).Restore,
			backup:      &api.Backup{Status: api.BackupStatus{}},
			expectedErr: false,
			expectedRes: NewTestUnstructured().WithName("pv-1").WithSpec("xyz").Unstructured,
		},
		{
			name:        "restoring, return early if there is PV->BackupInfo map but no entry for this PV",
			obj:         NewTestUnstructured().WithName("pv-1").WithSpec("xyz").Unstructured,
			restore:     arktest.NewDefaultTestRestore().WithRestorePVs(true).Restore,
			backup:      &api.Backup{Status: api.BackupStatus{VolumeBackups: map[string]*api.VolumeBackupInfo{"another-pv": {}}}},
			expectedErr: false,
			expectedRes: NewTestUnstructured().WithName("pv-1").WithSpec("xyz").Unstructured,
		},
		{
			name:              "volume type and IOPS are correctly passed to CreateVolume",
			obj:               NewTestUnstructured().WithName("pv-1").WithSpec("xyz").Unstructured,
			restore:           arktest.NewDefaultTestRestore().WithRestorePVs(true).Restore,
			backup:            &api.Backup{Status: api.BackupStatus{VolumeBackups: map[string]*api.VolumeBackupInfo{"pv-1": {SnapshotID: "snap-1", Type: "gp", Iops: &iops}}}},
			volumeMap:         map[api.VolumeBackupInfo]string{{SnapshotID: "snap-1", Type: "gp", Iops: &iops}: "volume-1"},
			volumeID:          "volume-1",
			expectedErr:       false,
			expectSetVolumeID: true,
			expectedRes:       NewTestUnstructured().WithName("pv-1").WithSpec("xyz").Unstructured,
		},
		{
			name:              "restoring, snapshotService=nil, backup has at least 1 snapshot -> error",
			obj:               NewTestUnstructured().WithName("pv-1").WithSpecField("awsElasticBlockStore", make(map[string]interface{})).Unstructured,
			restore:           arktest.NewDefaultTestRestore().Restore,
			backup:            &api.Backup{Status: api.BackupStatus{VolumeBackups: map[string]*api.VolumeBackupInfo{"pv-1": {SnapshotID: "snap-1"}}}},
			volumeMap:         map[api.VolumeBackupInfo]string{{SnapshotID: "snap-1"}: "volume-1"},
			volumeID:          "volume-1",
			noSnapshotService: true,
			expectedErr:       true,
			expectedRes:       NewTestUnstructured().WithName("pv-1").WithSpecField("awsElasticBlockStore", make(map[string]interface{})).Unstructured,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var (
				snapshotService     cloudprovider.SnapshotService
				fakeSnapshotService *arktest.FakeSnapshotService
			)
			if !test.noSnapshotService {
				fakeSnapshotService = &arktest.FakeSnapshotService{
					RestorableVolumes: test.volumeMap,
					VolumeID:          test.volumeID,
				}
				snapshotService = fakeSnapshotService
			}

			r := &pvRestorer{
				logger:          arktest.NewLogger(),
				restorePVs:      test.restore.Spec.RestorePVs,
				snapshotService: snapshotService,
			}
			if test.backup != nil {
				r.snapshotVolumes = test.backup.Spec.SnapshotVolumes
				r.volumeBackups = test.backup.Status.VolumeBackups
			}

			res, err := r.executePVAction(test.obj)

			if test.expectedErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)

			if test.expectSetVolumeID {
				assert.Equal(t, test.volumeID, fakeSnapshotService.VolumeIDSet)
			} else {
				assert.Equal(t, "", fakeSnapshotService.VolumeIDSet)
			}
			assert.Equal(t, test.expectedRes, res)
		})
	}
}

func TestIsPVReady(t *testing.T) {
	tests := []struct {
		name     string
		obj      *unstructured.Unstructured
		expected bool
	}{
		{
			name:     "no status returns not ready",
			obj:      NewTestUnstructured().Unstructured,
			expected: false,
		},
		{
			name:     "no status.phase returns not ready",
			obj:      NewTestUnstructured().WithStatus().Unstructured,
			expected: false,
		},
		{
			name:     "empty status.phase returns not ready",
			obj:      NewTestUnstructured().WithStatusField("phase", "").Unstructured,
			expected: false,
		},
		{
			name:     "non-Available status.phase returns not ready",
			obj:      NewTestUnstructured().WithStatusField("phase", "foo").Unstructured,
			expected: false,
		},
		{
			name:     "Available status.phase returns ready",
			obj:      NewTestUnstructured().WithStatusField("phase", "Available").Unstructured,
			expected: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.expected, isPVReady(test.obj))
		})
	}
}

type testUnstructured struct {
	*unstructured.Unstructured
}

func NewTestUnstructured() *testUnstructured {
	obj := &testUnstructured{
		Unstructured: &unstructured.Unstructured{
			Object: make(map[string]interface{}),
		},
	}

	return obj
}

func (obj *testUnstructured) WithAPIVersion(v string) *testUnstructured {
	obj.Object["apiVersion"] = v
	return obj
}

func (obj *testUnstructured) WithKind(k string) *testUnstructured {
	obj.Object["kind"] = k
	return obj
}

func (obj *testUnstructured) WithMetadata(fields ...string) *testUnstructured {
	return obj.withMap("metadata", fields...)
}

func (obj *testUnstructured) WithSpec(fields ...string) *testUnstructured {
	return obj.withMap("spec", fields...)
}

func (obj *testUnstructured) WithStatus(fields ...string) *testUnstructured {
	return obj.withMap("status", fields...)
}

func (obj *testUnstructured) WithMetadataField(field string, value interface{}) *testUnstructured {
	return obj.withMapEntry("metadata", field, value)
}

func (obj *testUnstructured) WithSpecField(field string, value interface{}) *testUnstructured {
	return obj.withMapEntry("spec", field, value)
}

func (obj *testUnstructured) WithStatusField(field string, value interface{}) *testUnstructured {
	return obj.withMapEntry("status", field, value)
}

func (obj *testUnstructured) WithAnnotations(fields ...string) *testUnstructured {
	vals := map[string]string{}
	for _, field := range fields {
		vals[field] = "foo"
	}

	return obj.WithAnnotationValues(vals)
}

func (obj *testUnstructured) WithAnnotationValues(fieldVals map[string]string) *testUnstructured {
	annotations := make(map[string]interface{})
	for field, val := range fieldVals {
		annotations[field] = val
	}

	obj = obj.WithMetadataField("annotations", annotations)

	return obj
}

func (obj *testUnstructured) WithNamespace(ns string) *testUnstructured {
	return obj.WithMetadataField("namespace", ns)
}

func (obj *testUnstructured) WithName(name string) *testUnstructured {
	return obj.WithMetadataField("name", name)
}

func (obj *testUnstructured) WithArkLabel(restoreName string) *testUnstructured {
	ls := obj.GetLabels()
	if ls == nil {
		ls = make(map[string]string)
	}
	ls[api.RestoreLabelKey] = restoreName
	obj.SetLabels(ls)

	return obj
}

func (obj *testUnstructured) ToJSON() []byte {
	bytes, err := json.Marshal(obj.Object)
	if err != nil {
		panic(err)
	}
	return bytes
}

func (obj *testUnstructured) withMap(name string, fields ...string) *testUnstructured {
	m := make(map[string]interface{})
	obj.Object[name] = m

	for _, field := range fields {
		m[field] = "foo"
	}

	return obj
}

func (obj *testUnstructured) withMapEntry(mapName, field string, value interface{}) *testUnstructured {
	var m map[string]interface{}

	if res, ok := obj.Unstructured.Object[mapName]; !ok {
		m = make(map[string]interface{})
		obj.Unstructured.Object[mapName] = m
	} else {
		m = res.(map[string]interface{})
	}

	m[field] = value

	return obj
}

func toUnstructured(objs ...runtime.Object) []unstructured.Unstructured {
	res := make([]unstructured.Unstructured, 0, len(objs))

	for _, obj := range objs {
		jsonObj, err := json.Marshal(obj)
		if err != nil {
			panic(err)
		}

		var unstructuredObj unstructured.Unstructured

		if err := json.Unmarshal(jsonObj, &unstructuredObj); err != nil {
			panic(err)
		}

		metadata := unstructuredObj.Object["metadata"].(map[string]interface{})

		delete(metadata, "creationTimestamp")

		delete(unstructuredObj.Object, "status")

		res = append(res, unstructuredObj)
	}

	return res
}

type testServiceAccount struct {
	*v1.ServiceAccount
}

func newTestServiceAccount() *testServiceAccount {
	return &testServiceAccount{
		ServiceAccount: &v1.ServiceAccount{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "v1",
				Kind:       "ServiceAccount",
			},
			ObjectMeta: metav1.ObjectMeta{
				Namespace:         "ns-1",
				Name:              "test-sa",
				CreationTimestamp: metav1.Time{Time: time.Now()},
			},
		},
	}
}

func (sa *testServiceAccount) WithArkLabel(restoreName string) *testServiceAccount {
	if sa.Labels == nil {
		sa.Labels = make(map[string]string)
	}
	sa.Labels[api.RestoreLabelKey] = restoreName
	return sa
}

func (sa *testServiceAccount) WithImagePullSecret(name string) *testServiceAccount {
	secret := v1.LocalObjectReference{Name: name}
	sa.ImagePullSecrets = append(sa.ImagePullSecrets, secret)
	return sa
}

func (sa *testServiceAccount) WithSecret(name string) *testServiceAccount {
	secret := v1.ObjectReference{Name: name}
	sa.Secrets = append(sa.Secrets, secret)
	return sa
}

func (sa *testServiceAccount) ToJSON() []byte {
	bytes, _ := json.Marshal(sa.ServiceAccount)
	return bytes
}

type testPersistentVolume struct {
	*v1.PersistentVolume
}

func newTestPV() *testPersistentVolume {
	return &testPersistentVolume{
		PersistentVolume: &v1.PersistentVolume{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "v1",
				Kind:       "PersistentVolume",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name: "test-pv",
			},
			Status: v1.PersistentVolumeStatus{},
		},
	}
}

func (pv *testPersistentVolume) WithArkLabel(restoreName string) *testPersistentVolume {
	if pv.Labels == nil {
		pv.Labels = make(map[string]string)
	}
	pv.Labels[api.RestoreLabelKey] = restoreName
	return pv
}

func (pv *testPersistentVolume) ToJSON() []byte {
	bytes, _ := json.Marshal(pv.PersistentVolume)
	return bytes
}

type testNamespace struct {
	*v1.Namespace
}

func newTestNamespace(name string) *testNamespace {
	return &testNamespace{
		Namespace: &v1.Namespace{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "v1",
				Kind:       "Namespace",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name: name,
			},
		},
	}
}

func (ns *testNamespace) ToJSON() []byte {
	bytes, _ := json.Marshal(ns.Namespace)
	return bytes
}

type testConfigMap struct {
	*v1.ConfigMap
}

func newTestConfigMap() *testConfigMap {
	return newNamedTestConfigMap("cm-1")
}

func newNamedTestConfigMap(name string) *testConfigMap {
	return &testConfigMap{
		ConfigMap: &v1.ConfigMap{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "v1",
				Kind:       "ConfigMap",
			},
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "ns-1",
				Name:      name,
			},
			Data: map[string]string{
				"foo": "bar",
			},
		},
	}
}

func (cm *testConfigMap) WithArkLabel(restoreName string) *testConfigMap {
	if cm.Labels == nil {
		cm.Labels = make(map[string]string)
	}

	cm.Labels[api.RestoreLabelKey] = restoreName

	return cm
}

func (cm *testConfigMap) WithNamespace(name string) *testConfigMap {
	cm.Namespace = name
	return cm
}

func (cm *testConfigMap) WithLabels(labels map[string]string) *testConfigMap {
	cm.Labels = labels
	return cm
}

func (cm *testConfigMap) WithControllerOwner() *testConfigMap {
	t := true
	ownerRef := metav1.OwnerReference{
		Controller: &t,
	}
	cm.ConfigMap.OwnerReferences = append(cm.ConfigMap.OwnerReferences, ownerRef)
	return cm
}

func (cm *testConfigMap) ToJSON() []byte {
	bytes, _ := json.Marshal(cm.ConfigMap)
	return bytes
}

type fakeAction struct {
	resource string
}

func newFakeAction(resource string) *fakeAction {
	return &fakeAction{resource}
}

func (r *fakeAction) AppliesTo() (ResourceSelector, error) {
	return ResourceSelector{
		IncludedResources: []string{r.resource},
	}, nil
}

func (r *fakeAction) Execute(obj runtime.Unstructured, restore *api.Restore) (runtime.Unstructured, error, error) {
	metadata, err := collections.GetMap(obj.UnstructuredContent(), "metadata")
	if err != nil {
		return nil, nil, err
	}

	if _, found := metadata["labels"]; !found {
		metadata["labels"] = make(map[string]interface{})
	}

	metadata["labels"].(map[string]interface{})["fake-restorer"] = "foo"

	unstructuredObj, ok := obj.(*unstructured.Unstructured)
	if !ok {
		return nil, nil, errors.New("Unexpected type")
	}

	// want the baseline functionality too
	res, err := resetMetadataAndStatus(unstructuredObj)
	if err != nil {
		return nil, nil, err
	}

	return res, nil, nil
}

type fakeNamespaceClient struct {
	createdNamespaces []*v1.Namespace

	corev1.NamespaceInterface
}

func (nsc *fakeNamespaceClient) Create(ns *v1.Namespace) (*v1.Namespace, error) {
	nsc.createdNamespaces = append(nsc.createdNamespaces, ns)
	return ns, nil
}
