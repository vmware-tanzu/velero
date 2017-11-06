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
	"archive/tar"
	"encoding/json"
	"fmt"
	"reflect"
	"testing"

	"github.com/heptio/ark/pkg/apis/ark/v1"
	"github.com/heptio/ark/pkg/util/collections"
	arktest "github.com/heptio/ark/pkg/util/test"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func TestBackupItemSkips(t *testing.T) {
	tests := []struct {
		testName      string
		namespace     string
		name          string
		namespaces    *collections.IncludesExcludes
		groupResource schema.GroupResource
		resources     *collections.IncludesExcludes
		backedUpItems map[itemKey]struct{}
	}{
		{
			testName:   "namespace not in includes list",
			namespace:  "ns",
			name:       "foo",
			namespaces: collections.NewIncludesExcludes().Includes("a"),
		},
		{
			testName:   "namespace in excludes list",
			namespace:  "ns",
			name:       "foo",
			namespaces: collections.NewIncludesExcludes().Excludes("ns"),
		},
		{
			testName:      "resource not in includes list",
			namespace:     "ns",
			name:          "foo",
			groupResource: schema.GroupResource{Group: "foo", Resource: "bar"},
			namespaces:    collections.NewIncludesExcludes(),
			resources:     collections.NewIncludesExcludes().Includes("a.b"),
		},
		{
			testName:      "resource in excludes list",
			namespace:     "ns",
			name:          "foo",
			groupResource: schema.GroupResource{Group: "foo", Resource: "bar"},
			namespaces:    collections.NewIncludesExcludes(),
			resources:     collections.NewIncludesExcludes().Excludes("bar.foo"),
		},
		{
			testName:      "resource already backed up",
			namespace:     "ns",
			name:          "foo",
			groupResource: schema.GroupResource{Group: "foo", Resource: "bar"},
			namespaces:    collections.NewIncludesExcludes(),
			resources:     collections.NewIncludesExcludes(),
			backedUpItems: map[itemKey]struct{}{
				{resource: "bar.foo", namespace: "ns", name: "foo"}: struct{}{},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.testName, func(t *testing.T) {
			ib := &defaultItemBackupper{
				namespaces:    test.namespaces,
				resources:     test.resources,
				backedUpItems: test.backedUpItems,
			}

			u := unstructuredOrDie(fmt.Sprintf(`{"apiVersion":"v1","kind":"Pod","metadata":{"namespace":"%s","name":"%s"}}`, test.namespace, test.name))
			err := ib.backupItem(arktest.NewLogger(), u, test.groupResource)
			assert.NoError(t, err)
		})
	}
}

func TestBackupItemSkipsClusterScopedResourceWhenIncludeClusterResourcesFalse(t *testing.T) {
	f := false
	ib := &defaultItemBackupper{
		backup: &v1.Backup{
			Spec: v1.BackupSpec{
				IncludeClusterResources: &f,
			},
		},
		namespaces: collections.NewIncludesExcludes(),
		resources:  collections.NewIncludesExcludes(),
	}

	u := unstructuredOrDie(`{"apiVersion":"v1","kind":"Foo","metadata":{"name":"bar"}}`)
	err := ib.backupItem(arktest.NewLogger(), u, schema.GroupResource{Group: "foo", Resource: "bar"})
	assert.NoError(t, err)
}

func TestBackupItemNoSkips(t *testing.T) {
	tests := []struct {
		name                                  string
		item                                  string
		namespaceIncludesExcludes             *collections.IncludesExcludes
		expectError                           bool
		expectExcluded                        bool
		expectedTarHeaderName                 string
		tarWriteError                         bool
		tarHeaderWriteError                   bool
		customAction                          bool
		expectedActionID                      string
		customActionAdditionalItemIdentifiers []ResourceIdentifier
		customActionAdditionalItems           []runtime.Unstructured
	}{
		{
			name: "explicit namespace include",
			item: `{"metadata":{"namespace":"foo","name":"bar"}}`,
			namespaceIncludesExcludes: collections.NewIncludesExcludes().Includes("foo"),
			expectError:               false,
			expectExcluded:            false,
			expectedTarHeaderName:     "resources/resource.group/namespaces/foo/bar.json",
		},
		{
			name: "* namespace include",
			item: `{"metadata":{"namespace":"foo","name":"bar"}}`,
			namespaceIncludesExcludes: collections.NewIncludesExcludes().Includes("*"),
			expectError:               false,
			expectExcluded:            false,
			expectedTarHeaderName:     "resources/resource.group/namespaces/foo/bar.json",
		},
		{
			name:                  "cluster-scoped",
			item:                  `{"metadata":{"name":"bar"}}`,
			expectError:           false,
			expectExcluded:        false,
			expectedTarHeaderName: "resources/resource.group/cluster/bar.json",
		},
		{
			name:                  "make sure status is deleted",
			item:                  `{"metadata":{"name":"bar"},"spec":{"color":"green"},"status":{"foo":"bar"}}`,
			expectError:           false,
			expectExcluded:        false,
			expectedTarHeaderName: "resources/resource.group/cluster/bar.json",
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
			expectedTarHeaderName: "resources/resource.group/cluster/bar.json",
			customAction:          true,
			expectedActionID:      "bar",
		},
		{
			name: "action invoked - namespaced",
			namespaceIncludesExcludes: collections.NewIncludesExcludes().Includes("*"),
			item:                  `{"metadata":{"namespace": "myns", "name":"bar"}}`,
			expectError:           false,
			expectExcluded:        false,
			expectedTarHeaderName: "resources/resource.group/namespaces/myns/bar.json",
			customAction:          true,
			expectedActionID:      "myns/bar",
		},
		{
			name: "action invoked - additional items",
			namespaceIncludesExcludes: collections.NewIncludesExcludes().Includes("*"),
			item:                  `{"metadata":{"namespace": "myns", "name":"bar"}}`,
			expectError:           false,
			expectExcluded:        false,
			expectedTarHeaderName: "resources/resource.group/namespaces/myns/bar.json",
			customAction:          true,
			expectedActionID:      "myns/bar",
			customActionAdditionalItemIdentifiers: []ResourceIdentifier{
				{
					GroupResource: schema.GroupResource{Group: "g1", Resource: "r1"},
					Namespace:     "ns1",
					Name:          "n1",
				},
				{
					GroupResource: schema.GroupResource{Group: "g2", Resource: "r2"},
					Namespace:     "ns2",
					Name:          "n2",
				},
			},
			customActionAdditionalItems: []runtime.Unstructured{
				unstructuredOrDie(`{"apiVersion":"g1/v1","kind":"r1","metadata":{"namespace":"ns1","name":"n1"}}`),
				unstructuredOrDie(`{"apiVersion":"g2/v1","kind":"r1","metadata":{"namespace":"ns2","name":"n2"}}`),
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var (
				actions       map[schema.GroupResource]Action
				action        *fakeAction
				backup        = &v1.Backup{}
				groupResource = schema.ParseGroupResource("resource.group")
				backedUpItems = make(map[itemKey]struct{})
				resources     = collections.NewIncludesExcludes()
				w             = &fakeTarWriter{}
			)

			item, err := getAsMap(test.item)
			if err != nil {
				t.Fatal(err)
			}

			namespaces := test.namespaceIncludesExcludes
			if namespaces == nil {
				namespaces = collections.NewIncludesExcludes()
			}

			if test.tarHeaderWriteError {
				w.writeHeaderError = errors.New("error")
			}
			if test.tarWriteError {
				w.writeError = errors.New("error")
			}

			if test.customAction {
				action = &fakeAction{
					additionalItems: test.customActionAdditionalItemIdentifiers,
				}
				actions = map[schema.GroupResource]Action{
					groupResource: action,
				}
			}

			resourceHooks := []resourceHook{}

			podCommandExecutor := &mockPodCommandExecutor{}
			defer podCommandExecutor.AssertExpectations(t)

			dynamicFactory := &arktest.FakeDynamicFactory{}
			defer dynamicFactory.AssertExpectations(t)

			discoveryHelper := arktest.NewFakeDiscoveryHelper(true, nil)

			b := (&defaultItemBackupperFactory{}).newItemBackupper(
				backup,
				namespaces,
				resources,
				backedUpItems,
				actions,
				podCommandExecutor,
				w,
				resourceHooks,
				dynamicFactory,
				discoveryHelper,
			).(*defaultItemBackupper)

			// make sure the podCommandExecutor was set correctly in the real hook handler
			assert.Equal(t, podCommandExecutor, b.itemHookHandler.(*defaultItemHookHandler).podCommandExecutor)

			itemHookHandler := &mockItemHookHandler{}
			defer itemHookHandler.AssertExpectations(t)
			b.itemHookHandler = itemHookHandler

			additionalItemBackupper := &mockItemBackupper{}
			defer additionalItemBackupper.AssertExpectations(t)
			b.additionalItemBackupper = additionalItemBackupper

			obj := &unstructured.Unstructured{Object: item}
			itemHookHandler.On("handleHooks", mock.Anything, groupResource, obj, resourceHooks).Return(nil)

			for i, item := range test.customActionAdditionalItemIdentifiers {
				itemClient := &arktest.FakeDynamicClient{}
				defer itemClient.AssertExpectations(t)

				dynamicFactory.On("ClientForGroupVersionResource", item.GroupResource.WithVersion("").GroupVersion(), metav1.APIResource{Name: item.Resource}, item.Namespace).Return(itemClient, nil)

				itemClient.On("Get", item.Name, metav1.GetOptions{}).Return(test.customActionAdditionalItems[i], nil)

				additionalItemBackupper.On("backupItem", mock.AnythingOfType("*logrus.Entry"), test.customActionAdditionalItems[i], item.GroupResource).Return(nil)
			}

			err = b.backupItem(arktest.NewLogger(), obj, groupResource)
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

type mockItemBackupper struct {
	mock.Mock
}

func (ib *mockItemBackupper) backupItem(logger *logrus.Entry, obj runtime.Unstructured, groupResource schema.GroupResource) error {
	args := ib.Called(logger, obj, groupResource)
	return args.Error(0)
}
