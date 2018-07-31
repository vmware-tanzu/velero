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
	"time"

	"github.com/heptio/ark/pkg/apis/ark/v1"
	api "github.com/heptio/ark/pkg/apis/ark/v1"
	"github.com/heptio/ark/pkg/util/collections"
	arktest "github.com/heptio/ark/pkg/util/test"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	corev1api "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/sets"
)

func TestBackupItemSkips(t *testing.T) {
	tests := []struct {
		testName      string
		namespace     string
		name          string
		namespaces    *collections.IncludesExcludes
		groupResource schema.GroupResource
		resources     *collections.IncludesExcludes
		terminating   bool
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
				{resource: "bar.foo", namespace: "ns", name: "foo"}: {},
			},
		},
		{
			testName:      "terminating resource",
			namespace:     "ns",
			name:          "foo",
			groupResource: schema.GroupResource{Group: "foo", Resource: "bar"},
			namespaces:    collections.NewIncludesExcludes(),
			resources:     collections.NewIncludesExcludes(),
			terminating:   true,
		},
	}

	for _, test := range tests {
		t.Run(test.testName, func(t *testing.T) {

			ib := &defaultItemBackupper{
				namespaces:    test.namespaces,
				resources:     test.resources,
				backedUpItems: test.backedUpItems,
			}

			pod := &corev1api.Pod{
				TypeMeta:   metav1.TypeMeta{APIVersion: "v1", Kind: "Pod"},
				ObjectMeta: metav1.ObjectMeta{Namespace: test.namespace, Name: test.name},
			}

			if test.terminating {
				pod.ObjectMeta.DeletionTimestamp = &metav1.Time{Time: time.Now()}
			}
			unstructuredObj, unmarshalErr := runtime.DefaultUnstructuredConverter.ToUnstructured(pod)
			require.NoError(t, unmarshalErr)
			u := &unstructured.Unstructured{Object: unstructuredObj}
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

	u := arktest.UnstructuredOrDie(`{"apiVersion":"v1","kind":"Foo","metadata":{"name":"bar"}}`)
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
		groupResource                         string
		snapshottableVolumes                  map[string]api.VolumeBackupInfo
		snapshotError                         error
		additionalItemError                   error
		trackedPVCs                           sets.String
		expectedTrackedPVCs                   sets.String
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
				arktest.UnstructuredOrDie(`{"apiVersion":"g1/v1","kind":"r1","metadata":{"namespace":"ns1","name":"n1"}}`),
				arktest.UnstructuredOrDie(`{"apiVersion":"g2/v1","kind":"r1","metadata":{"namespace":"ns2","name":"n2"}}`),
			},
		},
		{
			name: "action invoked - additional items - error",
			namespaceIncludesExcludes: collections.NewIncludesExcludes().Includes("*"),
			item:                  `{"metadata":{"namespace": "myns", "name":"bar"}}`,
			expectError:           true,
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
				arktest.UnstructuredOrDie(`{"apiVersion":"g1/v1","kind":"r1","metadata":{"namespace":"ns1","name":"n1"}}`),
				arktest.UnstructuredOrDie(`{"apiVersion":"g2/v1","kind":"r1","metadata":{"namespace":"ns2","name":"n2"}}`),
			},
			additionalItemError: errors.New("foo"),
		},
		{
			name: "takePVSnapshot is not invoked for PVs when blockStore == nil",
			namespaceIncludesExcludes: collections.NewIncludesExcludes().Includes("*"),
			item:                  `{"apiVersion": "v1", "kind": "PersistentVolume", "metadata": {"name": "mypv", "labels": {"failure-domain.beta.kubernetes.io/zone": "us-east-1c"}}, "spec": {"awsElasticBlockStore": {"volumeID": "aws://us-east-1c/vol-abc123"}}}`,
			expectError:           false,
			expectExcluded:        false,
			expectedTarHeaderName: "resources/persistentvolumes/cluster/mypv.json",
			groupResource:         "persistentvolumes",
		},
		{
			name: "takePVSnapshot is invoked for PVs when blockStore != nil",
			namespaceIncludesExcludes: collections.NewIncludesExcludes().Includes("*"),
			item:                  `{"apiVersion": "v1", "kind": "PersistentVolume", "metadata": {"name": "mypv", "labels": {"failure-domain.beta.kubernetes.io/zone": "us-east-1c"}}, "spec": {"awsElasticBlockStore": {"volumeID": "aws://us-east-1c/vol-abc123"}}}`,
			expectError:           false,
			expectExcluded:        false,
			expectedTarHeaderName: "resources/persistentvolumes/cluster/mypv.json",
			groupResource:         "persistentvolumes",
			snapshottableVolumes: map[string]api.VolumeBackupInfo{
				"vol-abc123": {SnapshotID: "snapshot-1", AvailabilityZone: "us-east-1c"},
			},
		},
		{
			name: "takePVSnapshot is not invoked for PVs when their claim is tracked in the restic PVC tracker",
			namespaceIncludesExcludes: collections.NewIncludesExcludes().Includes("*"),
			item:                  `{"apiVersion": "v1", "kind": "PersistentVolume", "metadata": {"name": "mypv", "labels": {"failure-domain.beta.kubernetes.io/zone": "us-east-1c"}}, "spec": {"claimRef": {"namespace": "pvc-ns", "name": "pvc"}, "awsElasticBlockStore": {"volumeID": "aws://us-east-1c/vol-abc123"}}}`,
			expectError:           false,
			expectExcluded:        false,
			expectedTarHeaderName: "resources/persistentvolumes/cluster/mypv.json",
			groupResource:         "persistentvolumes",
			// empty snapshottableVolumes causes a blockStore to be created, but no
			// snapshots are expected to be taken.
			snapshottableVolumes: map[string]api.VolumeBackupInfo{},
			trackedPVCs:          sets.NewString(key("pvc-ns", "pvc"), key("another-pvc-ns", "another-pvc")),
		},
		{
			name: "takePVSnapshot is invoked for PVs when their claim is not tracked in the restic PVC tracker",
			namespaceIncludesExcludes: collections.NewIncludesExcludes().Includes("*"),
			item:                  `{"apiVersion": "v1", "kind": "PersistentVolume", "metadata": {"name": "mypv", "labels": {"failure-domain.beta.kubernetes.io/zone": "us-east-1c"}}, "spec": {"claimRef": {"namespace": "pvc-ns", "name": "pvc"}, "awsElasticBlockStore": {"volumeID": "aws://us-east-1c/vol-abc123"}}}`,
			expectError:           false,
			expectExcluded:        false,
			expectedTarHeaderName: "resources/persistentvolumes/cluster/mypv.json",
			groupResource:         "persistentvolumes",
			snapshottableVolumes: map[string]api.VolumeBackupInfo{
				"vol-abc123": {SnapshotID: "snapshot-1", AvailabilityZone: "us-east-1c"},
			},
			trackedPVCs: sets.NewString(key("another-pvc-ns", "another-pvc")),
		},
		{
			name: "backup fails when takePVSnapshot fails",
			namespaceIncludesExcludes: collections.NewIncludesExcludes().Includes("*"),
			item:          `{"apiVersion": "v1", "kind": "PersistentVolume", "metadata": {"name": "mypv", "labels": {"failure-domain.beta.kubernetes.io/zone": "us-east-1c"}}, "spec": {"awsElasticBlockStore": {"volumeID": "aws://us-east-1c/vol-abc123"}}}`,
			expectError:   true,
			groupResource: "persistentvolumes",
			snapshottableVolumes: map[string]api.VolumeBackupInfo{
				"vol-abc123": {SnapshotID: "snapshot-1", AvailabilityZone: "us-east-1c"},
			},
			snapshotError: fmt.Errorf("failure"),
		},
		{
			name: "pod's restic PVC volume backups (only) are tracked",
			item: `{"apiVersion": "v1", "kind": "Pod", "spec": {"volumes": [{"name": "volume-1", "persistentVolumeClaim": {"claimName": "bar"}},{"name": "volume-2", "persistentVolumeClaim": {"claimName": "baz"}},{"name": "volume-1", "emptyDir": {}}]}, "metadata":{"namespace":"foo","name":"bar", "annotations": {"backup.ark.heptio.com/backup-volumes": "volume-1,volume-2"}}}`,
			namespaceIncludesExcludes: collections.NewIncludesExcludes().Includes("*"),
			groupResource:             "pods",
			expectError:               false,
			expectExcluded:            false,
			expectedTarHeaderName:     "resources/pods/namespaces/foo/bar.json",
			expectedTrackedPVCs:       sets.NewString(key("foo", "bar"), key("foo", "baz")),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var (
				actions       []resolvedAction
				action        *fakeAction
				backup        = &v1.Backup{}
				groupResource = schema.ParseGroupResource("resource.group")
				backedUpItems = make(map[itemKey]struct{})
				resources     = collections.NewIncludesExcludes()
				w             = &fakeTarWriter{}
			)

			if test.groupResource != "" {
				groupResource = schema.ParseGroupResource(test.groupResource)
			}

			item, err := arktest.GetAsMap(test.item)
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
				actions = []resolvedAction{
					{
						ItemAction:                action,
						namespaceIncludesExcludes: collections.NewIncludesExcludes(),
						resourceIncludesExcludes:  collections.NewIncludesExcludes().Includes(groupResource.String()),
						selector:                  labels.Everything(),
					},
				}
			}

			resourceHooks := []resourceHook{}

			podCommandExecutor := &arktest.MockPodCommandExecutor{}
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
				nil, // snapshot service
				nil, // restic backupper
				newPVCSnapshotTracker(),
			).(*defaultItemBackupper)

			var blockStore *arktest.FakeBlockStore
			if test.snapshottableVolumes != nil {
				blockStore = &arktest.FakeBlockStore{
					SnapshottableVolumes: test.snapshottableVolumes,
					VolumeID:             "vol-abc123",
					Error:                test.snapshotError,
				}
				b.blockStore = blockStore
			}

			if test.trackedPVCs != nil {
				b.resticSnapshotTracker.pvcs = test.trackedPVCs
			}

			// make sure the podCommandExecutor was set correctly in the real hook handler
			assert.Equal(t, podCommandExecutor, b.itemHookHandler.(*defaultItemHookHandler).podCommandExecutor)

			itemHookHandler := &mockItemHookHandler{}
			defer itemHookHandler.AssertExpectations(t)
			b.itemHookHandler = itemHookHandler

			additionalItemBackupper := &mockItemBackupper{}
			defer additionalItemBackupper.AssertExpectations(t)
			b.additionalItemBackupper = additionalItemBackupper

			obj := &unstructured.Unstructured{Object: item}
			itemHookHandler.On("handleHooks", mock.Anything, groupResource, obj, resourceHooks, hookPhasePre).Return(nil)
			itemHookHandler.On("handleHooks", mock.Anything, groupResource, obj, resourceHooks, hookPhasePost).Return(nil)

			for i, item := range test.customActionAdditionalItemIdentifiers {
				if test.additionalItemError != nil && i > 0 {
					break
				}
				itemClient := &arktest.FakeDynamicClient{}
				defer itemClient.AssertExpectations(t)

				dynamicFactory.On("ClientForGroupVersionResource", item.GroupResource.WithVersion("").GroupVersion(), metav1.APIResource{Name: item.Resource}, item.Namespace).Return(itemClient, nil)

				itemClient.On("Get", item.Name, metav1.GetOptions{}).Return(test.customActionAdditionalItems[i], nil)

				additionalItemBackupper.On("backupItem", mock.AnythingOfType("*logrus.Entry"), test.customActionAdditionalItems[i], item.GroupResource).Return(test.additionalItemError)
			}

			err = b.backupItem(arktest.NewLogger(), obj, groupResource)
			gotError := err != nil
			if e, a := test.expectError, gotError; e != a {
				t.Fatalf("error: expected %t, got %t: %v", e, a, err)
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

			// Convert to JSON for comparing number of bytes to the tar header
			itemJSON, err := json.Marshal(&item)
			if err != nil {
				t.Fatal(err)
			}
			require.Equal(t, 1, len(w.headers), "headers")
			assert.Equal(t, test.expectedTarHeaderName, w.headers[0].Name, "header.name")
			assert.Equal(t, int64(len(itemJSON)), w.headers[0].Size, "header.size")
			assert.Equal(t, byte(tar.TypeReg), w.headers[0].Typeflag, "header.typeflag")
			assert.Equal(t, int64(0755), w.headers[0].Mode, "header.mode")
			assert.False(t, w.headers[0].ModTime.IsZero(), "header.modTime set")
			assert.Equal(t, 1, len(w.data), "# of data")

			actual, err := arktest.GetAsMap(string(w.data[0]))
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

				require.Equal(t, 1, len(action.backups), "unexpected custom action backups: %#v", action.backups)
				assert.Equal(t, backup, &(action.backups[0]), "backup")
			}

			if test.snapshottableVolumes != nil {
				require.Equal(t, len(test.snapshottableVolumes), len(blockStore.SnapshotsTaken))

				var expectedBackups []api.VolumeBackupInfo
				for _, vbi := range test.snapshottableVolumes {
					expectedBackups = append(expectedBackups, vbi)
				}

				var actualBackups []api.VolumeBackupInfo
				for _, vbi := range backup.Status.VolumeBackups {
					actualBackups = append(actualBackups, *vbi)
				}

				assert.Equal(t, expectedBackups, actualBackups)
			}

			if test.expectedTrackedPVCs != nil {
				require.Equal(t, len(test.expectedTrackedPVCs), len(b.resticSnapshotTracker.pvcs))

				for key := range test.expectedTrackedPVCs {
					assert.True(t, b.resticSnapshotTracker.pvcs.Has(key))
				}
			}
		})
	}
}

type addAnnotationAction struct{}

func (a *addAnnotationAction) Execute(item runtime.Unstructured, backup *v1.Backup) (runtime.Unstructured, []ResourceIdentifier, error) {
	// since item actions run out-of-proc, do a deep-copy here to simulate passing data
	// across a process boundary.
	copy := item.(*unstructured.Unstructured).DeepCopy()

	metadata, err := meta.Accessor(copy)
	if err != nil {
		return copy, nil, nil
	}

	annotations := metadata.GetAnnotations()
	if annotations == nil {
		annotations = make(map[string]string)
	}
	annotations["foo"] = "bar"
	metadata.SetAnnotations(annotations)

	return copy, nil, nil
}

func (a *addAnnotationAction) AppliesTo() (ResourceSelector, error) {
	panic("not implemented")
}

func TestItemActionModificationsToItemPersist(t *testing.T) {
	var (
		w   = &fakeTarWriter{}
		obj = &unstructured.Unstructured{
			Object: map[string]interface{}{
				"metadata": map[string]interface{}{
					"namespace": "myns",
					"name":      "bar",
				},
			},
		}
		actions = []resolvedAction{
			{
				ItemAction:                &addAnnotationAction{},
				namespaceIncludesExcludes: collections.NewIncludesExcludes(),
				resourceIncludesExcludes:  collections.NewIncludesExcludes(),
				selector:                  labels.Everything(),
			},
		}
		b = (&defaultItemBackupperFactory{}).newItemBackupper(
			&v1.Backup{},
			collections.NewIncludesExcludes(),
			collections.NewIncludesExcludes(),
			make(map[itemKey]struct{}),
			actions,
			nil,
			w,
			nil,
			&arktest.FakeDynamicFactory{},
			arktest.NewFakeDiscoveryHelper(true, nil),
			nil,
			nil,
			newPVCSnapshotTracker(),
		).(*defaultItemBackupper)
	)

	// our expected backed-up object is the passed-in object plus the annotation
	// that the backup item action adds.
	expected := obj.DeepCopy()
	expected.SetAnnotations(map[string]string{"foo": "bar"})

	// method under test
	require.NoError(t, b.backupItem(arktest.NewLogger(), obj, schema.ParseGroupResource("resource.group")))

	// get the actual backed-up item
	require.Len(t, w.data, 1)
	actual, err := arktest.GetAsMap(string(w.data[0]))
	require.NoError(t, err)

	assert.EqualValues(t, expected.Object, actual)
}

func TestTakePVSnapshot(t *testing.T) {
	iops := int64(1000)

	tests := []struct {
		name                   string
		snapshotEnabled        bool
		pv                     string
		ttl                    time.Duration
		expectError            bool
		expectedVolumeID       string
		expectedSnapshotsTaken int
		existingVolumeBackups  map[string]*v1.VolumeBackupInfo
		volumeInfo             map[string]v1.VolumeBackupInfo
	}{
		{
			name:            "snapshot disabled",
			pv:              `{"apiVersion": "v1", "kind": "PersistentVolume", "metadata": {"name": "mypv"}}`,
			snapshotEnabled: false,
		},
		{
			name:            "unsupported PV source type",
			snapshotEnabled: true,
			pv:              `{"apiVersion": "v1", "kind": "PersistentVolume", "metadata": {"name": "mypv"}, "spec": {"unsupportedPVSource": {}}}`,
			expectError:     false,
		},
		{
			name:                   "without iops",
			snapshotEnabled:        true,
			pv:                     `{"apiVersion": "v1", "kind": "PersistentVolume", "metadata": {"name": "mypv", "labels": {"failure-domain.beta.kubernetes.io/zone": "us-east-1c"}}, "spec": {"awsElasticBlockStore": {"volumeID": "aws://us-east-1c/vol-abc123"}}}`,
			expectError:            false,
			expectedSnapshotsTaken: 1,
			expectedVolumeID:       "vol-abc123",
			ttl:                    5 * time.Minute,
			volumeInfo: map[string]v1.VolumeBackupInfo{
				"vol-abc123": {Type: "gp", SnapshotID: "snap-1", AvailabilityZone: "us-east-1c"},
			},
		},
		{
			name:                   "with iops",
			snapshotEnabled:        true,
			pv:                     `{"apiVersion": "v1", "kind": "PersistentVolume", "metadata": {"name": "mypv", "labels": {"failure-domain.beta.kubernetes.io/zone": "us-east-1c"}}, "spec": {"awsElasticBlockStore": {"volumeID": "aws://us-east-1c/vol-abc123"}}}`,
			expectError:            false,
			expectedSnapshotsTaken: 1,
			expectedVolumeID:       "vol-abc123",
			ttl:                    5 * time.Minute,
			volumeInfo: map[string]v1.VolumeBackupInfo{
				"vol-abc123": {Type: "io1", Iops: &iops, SnapshotID: "snap-1", AvailabilityZone: "us-east-1c"},
			},
		},
		{
			name:                   "preexisting volume backup info in backup status",
			snapshotEnabled:        true,
			pv:                     `{"apiVersion": "v1", "kind": "PersistentVolume", "metadata": {"name": "mypv"}, "spec": {"gcePersistentDisk": {"pdName": "pd-abc123"}}}`,
			expectError:            false,
			expectedSnapshotsTaken: 1,
			expectedVolumeID:       "pd-abc123",
			ttl:                    5 * time.Minute,
			existingVolumeBackups: map[string]*v1.VolumeBackupInfo{
				"anotherpv": {SnapshotID: "anothersnap"},
			},
			volumeInfo: map[string]v1.VolumeBackupInfo{
				"pd-abc123": {Type: "gp", SnapshotID: "snap-1"},
			},
		},
		{
			name:             "create snapshot error",
			snapshotEnabled:  true,
			pv:               `{"apiVersion": "v1", "kind": "PersistentVolume", "metadata": {"name": "mypv"}, "spec": {"gcePersistentDisk": {"pdName": "pd-abc123"}}}`,
			expectedVolumeID: "pd-abc123",
			expectError:      true,
		},
		{
			name:                   "PV with label metadata but no failureDomainZone",
			snapshotEnabled:        true,
			pv:                     `{"apiVersion": "v1", "kind": "PersistentVolume", "metadata": {"name": "mypv", "labels": {"failure-domain.beta.kubernetes.io/region": "us-east-1"}}, "spec": {"awsElasticBlockStore": {"volumeID": "aws://us-east-1c/vol-abc123"}}}`,
			expectError:            false,
			expectedSnapshotsTaken: 1,
			expectedVolumeID:       "vol-abc123",
			ttl:                    5 * time.Minute,
			volumeInfo: map[string]v1.VolumeBackupInfo{
				"vol-abc123": {Type: "gp", SnapshotID: "snap-1"},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			backup := &v1.Backup{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: v1.DefaultNamespace,
					Name:      "mybackup",
				},
				Spec: v1.BackupSpec{
					SnapshotVolumes: &test.snapshotEnabled,
					TTL:             metav1.Duration{Duration: test.ttl},
				},
				Status: v1.BackupStatus{
					VolumeBackups: test.existingVolumeBackups,
				},
			}

			blockStore := &arktest.FakeBlockStore{
				SnapshottableVolumes: test.volumeInfo,
				VolumeID:             test.expectedVolumeID,
			}

			ib := &defaultItemBackupper{blockStore: blockStore}

			pv, err := arktest.GetAsMap(test.pv)
			if err != nil {
				t.Fatal(err)
			}

			// method under test
			err = ib.takePVSnapshot(&unstructured.Unstructured{Object: pv}, backup, arktest.NewLogger())

			gotErr := err != nil

			if e, a := test.expectError, gotErr; e != a {
				t.Errorf("error: expected %v, got %v", e, a)
			}
			if test.expectError {
				return
			}

			if !test.snapshotEnabled {
				// don't need to check anything else if snapshots are disabled
				return
			}

			expectedVolumeBackups := test.existingVolumeBackups
			if expectedVolumeBackups == nil {
				expectedVolumeBackups = make(map[string]*v1.VolumeBackupInfo)
			}

			// we should have one snapshot taken exactly
			require.Equal(t, test.expectedSnapshotsTaken, blockStore.SnapshotsTaken.Len())

			if test.expectedSnapshotsTaken > 0 {
				// the snapshotID should be the one in the entry in blockStore.SnapshottableVolumes
				// for the volume we ran the test for
				snapshotID, _ := blockStore.SnapshotsTaken.PopAny()

				expectedVolumeBackups["mypv"] = &v1.VolumeBackupInfo{
					SnapshotID:       snapshotID,
					Type:             test.volumeInfo[test.expectedVolumeID].Type,
					Iops:             test.volumeInfo[test.expectedVolumeID].Iops,
					AvailabilityZone: test.volumeInfo[test.expectedVolumeID].AvailabilityZone,
				}

				if e, a := expectedVolumeBackups, backup.Status.VolumeBackups; !reflect.DeepEqual(e, a) {
					t.Errorf("backup.status.VolumeBackups: expected %v, got %v", e, a)
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

func (ib *mockItemBackupper) backupItem(logger logrus.FieldLogger, obj runtime.Unstructured, groupResource schema.GroupResource) error {
	args := ib.Called(logger, obj, groupResource)
	return args.Error(0)
}
