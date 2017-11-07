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

package restore

import (
	"encoding/json"
	"io"
	"os"
	"testing"

	"github.com/sirupsen/logrus/hooks/test"
	testlogger "github.com/sirupsen/logrus/hooks/test"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	corev1 "k8s.io/client-go/kubernetes/typed/core/v1"

	api "github.com/heptio/ark/pkg/apis/ark/v1"
	"github.com/heptio/ark/pkg/restore/restorers"
	"github.com/heptio/ark/pkg/util/collections"
	. "github.com/heptio/ark/pkg/util/test"
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

	logger, _ := test.NewNullLogger()

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

			helper := NewFakeDiscoveryHelper(true, nil)
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
		fileSystem           *fakeFileSystem
		baseDir              string
		restore              *api.Restore
		expectedReadDirs     []string
		prioritizedResources []schema.GroupResource
	}{
		{
			name:             "namespacesToRestore having * restores all namespaces",
			fileSystem:       newFakeFileSystem().WithDirectories("bak/resources/nodes/cluster", "bak/resources/secrets/namespaces/a", "bak/resources/secrets/namespaces/b", "bak/resources/secrets/namespaces/c"),
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
			fileSystem:       newFakeFileSystem().WithDirectories("bak/resources/nodes/cluster", "bak/resources/secrets/namespaces/a", "bak/resources/secrets/namespaces/b", "bak/resources/secrets/namespaces/c"),
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
			fileSystem:       newFakeFileSystem().WithDirectories("bak/resources/nodes/cluster", "bak/resources/secrets/namespaces/a", "bak/resources/secrets/namespaces/b", "bak/resources/secrets/namespaces/c"),
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
			fileSystem: newFakeFileSystem().WithDirectories("bak/resources/nodes/cluster", "bak/resources/secrets/namespaces/a", "bak/resources/secrets/namespaces/b", "bak/resources/secrets/namespaces/c"),
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
			log, _ := testlogger.NewNullLogger()

			ctx := &context{
				restore:              test.restore,
				namespaceClient:      &fakeNamespaceClient{},
				fileSystem:           test.fileSystem,
				logger:               log,
				prioritizedResources: test.prioritizedResources,
			}

			warnings, errors := ctx.restoreFromDir(test.baseDir)

			assert.Empty(t, warnings.Ark)
			assert.Empty(t, warnings.Cluster)
			assert.Empty(t, warnings.Namespaces)
			assert.Empty(t, errors.Ark)
			assert.Empty(t, errors.Cluster)
			assert.Empty(t, errors.Namespaces)
			assert.Equal(t, test.expectedReadDirs, test.fileSystem.readDirCalls)
		})
	}
}

func TestRestorePriority(t *testing.T) {
	tests := []struct {
		name                 string
		fileSystem           *fakeFileSystem
		restore              *api.Restore
		baseDir              string
		prioritizedResources []schema.GroupResource
		expectedErrors       api.RestoreResult
		expectedReadDirs     []string
	}{
		{
			name:       "cluster test",
			fileSystem: newFakeFileSystem().WithDirectory("bak/resources/a/cluster").WithDirectory("bak/resources/c/cluster"),
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
			fileSystem: newFakeFileSystem().WithDirectory("bak/resources/a/cluster").WithDirectory("bak/resources/c/cluster"),
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
			fileSystem: newFakeFileSystem().WithDirectory("bak/resources/a/namespaces/ns-1").WithDirectory("bak/resources/c/namespaces/ns-1"),
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
			fileSystem: newFakeFileSystem().
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
			log, _ := testlogger.NewNullLogger()

			ctx := &context{
				restore:              test.restore,
				namespaceClient:      &fakeNamespaceClient{},
				fileSystem:           test.fileSystem,
				prioritizedResources: test.prioritizedResources,
				logger:               log,
			}

			warnings, errors := ctx.restoreFromDir(test.baseDir)

			assert.Empty(t, warnings.Ark)
			assert.Empty(t, warnings.Cluster)
			assert.Empty(t, warnings.Namespaces)
			assert.Equal(t, test.expectedErrors, errors)

			assert.Equal(t, test.expectedReadDirs, test.fileSystem.readDirCalls)
		})
	}
}

func TestNamespaceRemapping(t *testing.T) {
	var (
		baseDir              = "bak"
		restore              = &api.Restore{Spec: api.RestoreSpec{IncludedNamespaces: []string{"*"}, NamespaceMapping: map[string]string{"ns-1": "ns-2"}}}
		prioritizedResources = []schema.GroupResource{{Resource: "configmaps"}}
		labelSelector        = labels.NewSelector()
		fileSystem           = newFakeFileSystem().WithFile("bak/resources/configmaps/namespaces/ns-1/cm-1.json", newTestConfigMap().WithNamespace("ns-1").ToJSON())
		expectedNS           = "ns-2"
		expectedObjs         = toUnstructured(newTestConfigMap().WithNamespace("ns-2").WithArkLabel("").ConfigMap)
	)

	resourceClient := &FakeDynamicClient{}
	for i := range expectedObjs {
		resourceClient.On("Create", &expectedObjs[i]).Return(&expectedObjs[i], nil)
	}

	dynamicFactory := &FakeDynamicFactory{}
	resource := metav1.APIResource{Name: "configmaps", Namespaced: true}
	gv := schema.GroupVersion{Group: "", Version: "v1"}
	dynamicFactory.On("ClientForGroupVersionResource", gv, resource, expectedNS).Return(resourceClient, nil)

	log, _ := testlogger.NewNullLogger()

	ctx := &context{
		dynamicFactory:       dynamicFactory,
		fileSystem:           fileSystem,
		selector:             labelSelector,
		namespaceClient:      &fakeNamespaceClient{},
		prioritizedResources: prioritizedResources,
		restore:              restore,
		backup:               &api.Backup{},
		logger:               log,
	}

	warnings, errors := ctx.restoreFromDir(baseDir)

	assert.Empty(t, warnings.Ark)
	assert.Empty(t, warnings.Cluster)
	assert.Empty(t, warnings.Namespaces)
	assert.Empty(t, errors.Ark)
	assert.Empty(t, errors.Cluster)
	assert.Empty(t, errors.Namespaces)

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
		fileSystem              *fakeFileSystem
		restorers               map[schema.GroupResource]restorers.ResourceRestorer
		expectedErrors          api.RestoreResult
		expectedObjs            []unstructured.Unstructured
	}{
		{
			name:          "basic normal case",
			namespace:     "ns-1",
			resourcePath:  "configmaps",
			labelSelector: labels.NewSelector(),
			fileSystem: newFakeFileSystem().
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
			fileSystem:   newFakeFileSystem(),
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
			fileSystem:   newFakeFileSystem().WithDirectory("configmaps"),
		},
		{
			name:          "unmarshall failure does not cause immediate return",
			namespace:     "ns-1",
			resourcePath:  "configmaps",
			labelSelector: labels.NewSelector(),
			fileSystem: newFakeFileSystem().
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
			fileSystem:    newFakeFileSystem().WithFile("configmaps/cm-1.json", newTestConfigMap().WithLabels(map[string]string{"foo": "bar"}).ToJSON()),
			expectedObjs:  toUnstructured(newTestConfigMap().WithLabels(map[string]string{"foo": "bar"}).WithArkLabel("my-restore").ConfigMap),
		},
		{
			name:          "non-matching label selector correctly excludes",
			namespace:     "ns-1",
			resourcePath:  "configmaps",
			labelSelector: labels.SelectorFromSet(labels.Set(map[string]string{"foo": "not-bar"})),
			fileSystem:    newFakeFileSystem().WithFile("configmaps/cm-1.json", newTestConfigMap().WithLabels(map[string]string{"foo": "bar"}).ToJSON()),
		},
		{
			name:          "items with controller owner are skipped",
			namespace:     "ns-1",
			resourcePath:  "configmaps",
			labelSelector: labels.NewSelector(),
			fileSystem: newFakeFileSystem().
				WithFile("configmaps/cm-1.json", newTestConfigMap().WithControllerOwner().ToJSON()).
				WithFile("configmaps/cm-2.json", newNamedTestConfigMap("cm-2").ToJSON()),
			expectedObjs: toUnstructured(newNamedTestConfigMap("cm-2").WithArkLabel("my-restore").ConfigMap),
		},
		{
			name:          "namespace is remapped",
			namespace:     "ns-2",
			resourcePath:  "configmaps",
			labelSelector: labels.NewSelector(),
			fileSystem:    newFakeFileSystem().WithFile("configmaps/cm-1.json", newTestConfigMap().WithNamespace("ns-1").ToJSON()),
			expectedObjs:  toUnstructured(newTestConfigMap().WithNamespace("ns-2").WithArkLabel("my-restore").ConfigMap),
		},
		{
			name:          "custom restorer is correctly used",
			namespace:     "ns-1",
			resourcePath:  "configmaps",
			labelSelector: labels.NewSelector(),
			fileSystem:    newFakeFileSystem().WithFile("configmaps/cm-1.json", newTestConfigMap().ToJSON()),
			restorers:     map[schema.GroupResource]restorers.ResourceRestorer{{Resource: "configmaps"}: newFakeCustomRestorer()},
			expectedObjs:  toUnstructured(newTestConfigMap().WithLabels(map[string]string{"fake-restorer": "foo"}).WithArkLabel("my-restore").ConfigMap),
		},
		{
			name:          "custom restorer for different group/resource is not used",
			namespace:     "ns-1",
			resourcePath:  "configmaps",
			labelSelector: labels.NewSelector(),
			fileSystem:    newFakeFileSystem().WithFile("configmaps/cm-1.json", newTestConfigMap().ToJSON()),
			restorers:     map[schema.GroupResource]restorers.ResourceRestorer{{Resource: "foo-resource"}: newFakeCustomRestorer()},
			expectedObjs:  toUnstructured(newTestConfigMap().WithArkLabel("my-restore").ConfigMap),
		},
		{
			name:                    "cluster-scoped resources are skipped when IncludeClusterResources=false",
			namespace:               "",
			resourcePath:            "persistentvolumes",
			labelSelector:           labels.NewSelector(),
			includeClusterResources: falsePtr,
			fileSystem:              newFakeFileSystem().WithFile("persistentvolumes/pv-1.json", newTestPV().ToJSON()),
		},
		{
			name:                    "namespaced resources are not skipped when IncludeClusterResources=false",
			namespace:               "ns-1",
			resourcePath:            "configmaps",
			labelSelector:           labels.NewSelector(),
			includeClusterResources: falsePtr,
			fileSystem:              newFakeFileSystem().WithFile("configmaps/cm-1.json", newTestConfigMap().ToJSON()),
			expectedObjs:            toUnstructured(newTestConfigMap().WithArkLabel("my-restore").ConfigMap),
		},
		{
			name:                    "cluster-scoped resources are not skipped when IncludeClusterResources=true",
			namespace:               "",
			resourcePath:            "persistentvolumes",
			labelSelector:           labels.NewSelector(),
			includeClusterResources: truePtr,
			fileSystem:              newFakeFileSystem().WithFile("persistentvolumes/pv-1.json", newTestPV().ToJSON()),
			expectedObjs:            toUnstructured(newTestPV().WithArkLabel("my-restore").PersistentVolume),
		},
		{
			name:                    "namespaced resources are not skipped when IncludeClusterResources=true",
			namespace:               "ns-1",
			resourcePath:            "configmaps",
			labelSelector:           labels.NewSelector(),
			includeClusterResources: truePtr,
			fileSystem:              newFakeFileSystem().WithFile("configmaps/cm-1.json", newTestConfigMap().ToJSON()),
			expectedObjs:            toUnstructured(newTestConfigMap().WithArkLabel("my-restore").ConfigMap),
		},
		{
			name:                    "cluster-scoped resources are not skipped when IncludeClusterResources=nil",
			namespace:               "",
			resourcePath:            "persistentvolumes",
			labelSelector:           labels.NewSelector(),
			includeClusterResources: nil,
			fileSystem:              newFakeFileSystem().WithFile("persistentvolumes/pv-1.json", newTestPV().ToJSON()),
			expectedObjs:            toUnstructured(newTestPV().WithArkLabel("my-restore").PersistentVolume),
		},
		{
			name:                    "namespaced resources are not skipped when IncludeClusterResources=nil",
			namespace:               "ns-1",
			resourcePath:            "configmaps",
			labelSelector:           labels.NewSelector(),
			includeClusterResources: nil,
			fileSystem:              newFakeFileSystem().WithFile("configmaps/cm-1.json", newTestConfigMap().ToJSON()),
			expectedObjs:            toUnstructured(newTestConfigMap().WithArkLabel("my-restore").ConfigMap),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			resourceClient := &FakeDynamicClient{}
			for i := range test.expectedObjs {
				resourceClient.On("Create", &test.expectedObjs[i]).Return(&test.expectedObjs[i], nil)
			}

			dynamicFactory := &FakeDynamicFactory{}
			resource := metav1.APIResource{Name: "configmaps", Namespaced: true}
			gv := schema.GroupVersion{Group: "", Version: "v1"}
			dynamicFactory.On("ClientForGroupVersionResource", gv, resource, test.namespace).Return(resourceClient, nil)

			pvResource := metav1.APIResource{Name: "persistentvolumes", Namespaced: false}
			dynamicFactory.On("ClientForGroupVersionResource", gv, pvResource, test.namespace).Return(resourceClient, nil)

			log, _ := testlogger.NewNullLogger()

			ctx := &context{
				dynamicFactory: dynamicFactory,
				restorers:      test.restorers,
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
				backup: &api.Backup{},
				logger: log,
			}

			warnings, errors := ctx.restoreResource(test.resourcePath, test.namespace, test.resourcePath)

			assert.Empty(t, warnings.Ark)
			assert.Empty(t, warnings.Cluster)
			assert.Empty(t, warnings.Namespaces)
			assert.Equal(t, test.expectedErrors, errors)
		})
	}
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

		if _, exists := metadata["namespace"]; !exists {
			metadata["namespace"] = ""
		}

		delete(unstructuredObj.Object, "status")

		res = append(res, unstructuredObj)
	}

	return res
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

type fakeFileSystem struct {
	fs afero.Fs

	readDirCalls []string
}

func newFakeFileSystem() *fakeFileSystem {
	return &fakeFileSystem{
		fs: afero.NewMemMapFs(),
	}
}

func (fs *fakeFileSystem) WithFile(path string, data []byte) *fakeFileSystem {
	file, _ := fs.fs.Create(path)
	file.Write(data)
	file.Close()

	return fs
}

func (fs *fakeFileSystem) WithDirectory(path string) *fakeFileSystem {
	fs.fs.MkdirAll(path, 0755)
	return fs
}

func (fs *fakeFileSystem) WithDirectories(path ...string) *fakeFileSystem {
	for _, dir := range path {
		fs = fs.WithDirectory(dir)
	}

	return fs
}

func (fs *fakeFileSystem) TempDir(dir, prefix string) (string, error) {
	return afero.TempDir(fs.fs, dir, prefix)
}

func (fs *fakeFileSystem) MkdirAll(path string, perm os.FileMode) error {
	return fs.fs.MkdirAll(path, perm)
}

func (fs *fakeFileSystem) Create(name string) (io.WriteCloser, error) {
	return fs.fs.Create(name)
}

func (fs *fakeFileSystem) RemoveAll(path string) error {
	return fs.fs.RemoveAll(path)
}

func (fs *fakeFileSystem) ReadDir(dirname string) ([]os.FileInfo, error) {
	fs.readDirCalls = append(fs.readDirCalls, dirname)
	return afero.ReadDir(fs.fs, dirname)
}

func (fs *fakeFileSystem) ReadFile(filename string) ([]byte, error) {
	return afero.ReadFile(fs.fs, filename)
}

func (fs *fakeFileSystem) DirExists(path string) (bool, error) {
	return afero.DirExists(fs.fs, path)
}

type fakeCustomRestorer struct {
	restorers.ResourceRestorer
}

func newFakeCustomRestorer() *fakeCustomRestorer {
	return &fakeCustomRestorer{
		ResourceRestorer: restorers.NewBasicRestorer(true),
	}
}

func (r *fakeCustomRestorer) Prepare(obj runtime.Unstructured, restore *api.Restore, backup *api.Backup) (runtime.Unstructured, error, error) {
	metadata, err := collections.GetMap(obj.UnstructuredContent(), "metadata")
	if err != nil {
		return nil, nil, err
	}

	if _, found := metadata["labels"]; !found {
		metadata["labels"] = make(map[string]interface{})
	}

	metadata["labels"].(map[string]interface{})["fake-restorer"] = "foo"

	// want the baseline functionality too
	return r.ResourceRestorer.Prepare(obj, restore, backup)
}

type fakeNamespaceClient struct {
	corev1.NamespaceInterface
}

func (nsc *fakeNamespaceClient) Create(ns *v1.Namespace) (*v1.Namespace, error) {
	return ns, nil
}
