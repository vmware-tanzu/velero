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
	"archive/tar"
	"encoding/json"
	"reflect"
	"testing"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/sets"

	velerov1api "github.com/heptio/velero/pkg/apis/velero/v1"
	"github.com/heptio/velero/pkg/plugin/velero"
	resticmocks "github.com/heptio/velero/pkg/restic/mocks"
	"github.com/heptio/velero/pkg/util/collections"
	velerotest "github.com/heptio/velero/pkg/util/test"
)

func TestBackupItemNoSkips(t *testing.T) {
	tests := []struct {
		name                      string
		item                      string
		namespaceIncludesExcludes *collections.IncludesExcludes
		expectError               bool
		expectExcluded            bool
		expectedTarHeaderName     string
		tarWriteError             bool
		tarHeaderWriteError       bool
		groupResource             string
		snapshottableVolumes      map[string]velerotest.VolumeBackupInfo
		snapshotError             error
		trackedPVCs               sets.String
		expectedTrackedPVCs       sets.String
	}{
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
			name:                      "takePVSnapshot is not invoked for PVs when their claim is tracked in the restic PVC tracker",
			namespaceIncludesExcludes: collections.NewIncludesExcludes().Includes("*"),
			item:                      `{"apiVersion": "v1", "kind": "PersistentVolume", "metadata": {"name": "mypv", "labels": {"failure-domain.beta.kubernetes.io/zone": "us-east-1c"}}, "spec": {"claimRef": {"namespace": "pvc-ns", "name": "pvc"}, "awsElasticBlockStore": {"volumeID": "aws://us-east-1c/vol-abc123"}}}`,
			expectError:               false,
			expectExcluded:            false,
			expectedTarHeaderName:     "resources/persistentvolumes/cluster/mypv.json",
			groupResource:             "persistentvolumes",
			// empty snapshottableVolumes causes a volumeSnapshotter to be created, but no
			// snapshots are expected to be taken.
			snapshottableVolumes: map[string]velerotest.VolumeBackupInfo{},
			trackedPVCs:          sets.NewString(key("pvc-ns", "pvc"), key("another-pvc-ns", "another-pvc")),
		},
		{
			name:                      "pod's restic PVC volume backups (only) are tracked",
			item:                      `{"apiVersion": "v1", "kind": "Pod", "spec": {"volumes": [{"name": "volume-1", "persistentVolumeClaim": {"claimName": "bar"}},{"name": "volume-2", "persistentVolumeClaim": {"claimName": "baz"}},{"name": "volume-1", "emptyDir": {}}]}}`,
			namespaceIncludesExcludes: collections.NewIncludesExcludes().Includes("*"),
			groupResource:             "pods",
			expectError:               false,
			expectExcluded:            false,
			expectedTarHeaderName:     "resources/pods/cluster/.json",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var (
				backup        = new(Request)
				groupResource = schema.ParseGroupResource("resource.group")
				backedUpItems = make(map[itemKey]struct{})
				w             = &fakeTarWriter{}
			)

			backup.Backup = new(velerov1api.Backup)
			backup.NamespaceIncludesExcludes = collections.NewIncludesExcludes()
			backup.ResourceIncludesExcludes = collections.NewIncludesExcludes()
			backup.SnapshotLocations = []*velerov1api.VolumeSnapshotLocation{
				newSnapshotLocation("velero", "default", "default"),
			}

			if test.groupResource != "" {
				groupResource = schema.ParseGroupResource(test.groupResource)
			}

			item, err := velerotest.GetAsMap(test.item)
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

			podCommandExecutor := &velerotest.MockPodCommandExecutor{}
			defer podCommandExecutor.AssertExpectations(t)

			dynamicFactory := &velerotest.FakeDynamicFactory{}
			defer dynamicFactory.AssertExpectations(t)

			discoveryHelper := velerotest.NewFakeDiscoveryHelper(true, nil)

			volumeSnapshotterGetter := make(volumeSnapshotterGetter)

			b := (&defaultItemBackupperFactory{}).newItemBackupper(
				backup,
				backedUpItems,
				podCommandExecutor,
				w,
				dynamicFactory,
				discoveryHelper,
				nil, // restic backupper
				newPVCSnapshotTracker(),
				volumeSnapshotterGetter,
			).(*defaultItemBackupper)

			var volumeSnapshotter *velerotest.FakeVolumeSnapshotter
			if test.snapshottableVolumes != nil {
				volumeSnapshotter = &velerotest.FakeVolumeSnapshotter{
					SnapshottableVolumes: test.snapshottableVolumes,
					VolumeID:             "vol-abc123",
					Error:                test.snapshotError,
				}

				volumeSnapshotterGetter["default"] = volumeSnapshotter
			}

			if test.trackedPVCs != nil {
				b.resticSnapshotTracker.pvcs = test.trackedPVCs
			}

			// make sure the podCommandExecutor was set correctly in the real hook handler
			assert.Equal(t, podCommandExecutor, b.itemHookHandler.(*defaultItemHookHandler).podCommandExecutor)

			itemHookHandler := &mockItemHookHandler{}
			defer itemHookHandler.AssertExpectations(t)
			b.itemHookHandler = itemHookHandler

			obj := &unstructured.Unstructured{Object: item}
			itemHookHandler.On("handleHooks", mock.Anything, groupResource, obj, backup.ResourceHooks, hookPhasePre).Return(nil)
			itemHookHandler.On("handleHooks", mock.Anything, groupResource, obj, backup.ResourceHooks, hookPhasePost).Return(nil)

			err = b.backupItem(velerotest.NewLogger(), obj, groupResource)
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

			actual, err := velerotest.GetAsMap(string(w.data[0]))
			if err != nil {
				t.Fatal(err)
			}
			if e, a := item, actual; !reflect.DeepEqual(e, a) {
				t.Errorf("data: expected %s, got %s", e, a)
			}

			if test.snapshottableVolumes != nil {
				require.Equal(t, len(test.snapshottableVolumes), len(volumeSnapshotter.SnapshotsTaken))
			}

			if len(test.snapshottableVolumes) > 0 {
				require.Len(t, backup.VolumeSnapshots, 1)
				snapshot := backup.VolumeSnapshots[0]

				assert.Equal(t, test.snapshottableVolumes["vol-abc123"].SnapshotID, snapshot.Status.ProviderSnapshotID)
				assert.Equal(t, test.snapshottableVolumes["vol-abc123"].Type, snapshot.Spec.VolumeType)
				assert.Equal(t, test.snapshottableVolumes["vol-abc123"].Iops, snapshot.Spec.VolumeIOPS)
				assert.Equal(t, test.snapshottableVolumes["vol-abc123"].AvailabilityZone, snapshot.Spec.VolumeAZ)
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

func (a *addAnnotationAction) Execute(item runtime.Unstructured, backup *velerov1api.Backup) (runtime.Unstructured, []velero.ResourceIdentifier, error) {
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

func (a *addAnnotationAction) AppliesTo() (velero.ResourceSelector, error) {
	panic("not implemented")
}

func TestResticAnnotationsPersist(t *testing.T) {
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
		req = &Request{
			NamespaceIncludesExcludes: collections.NewIncludesExcludes(),
			ResourceIncludesExcludes:  collections.NewIncludesExcludes(),
			ResolvedActions: []resolvedAction{
				{
					BackupItemAction:          &addAnnotationAction{},
					namespaceIncludesExcludes: collections.NewIncludesExcludes(),
					resourceIncludesExcludes:  collections.NewIncludesExcludes(),
					selector:                  labels.Everything(),
				},
			},
		}
		resticBackupper = &resticmocks.Backupper{}
		b               = (&defaultItemBackupperFactory{}).newItemBackupper(
			req,
			make(map[itemKey]struct{}),
			nil,
			w,
			&velerotest.FakeDynamicFactory{},
			velerotest.NewFakeDiscoveryHelper(true, nil),
			resticBackupper,
			newPVCSnapshotTracker(),
			nil,
		).(*defaultItemBackupper)
	)

	podVolumeBackups := []*velerov1api.PodVolumeBackup{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: "volume-1",
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: "volume-2",
			},
		},
	}

	resticBackupper.
		On("BackupPodVolumes", mock.Anything, mock.Anything, mock.Anything).
		Return(podVolumeBackups, nil)

	// our expected backed-up object is the passed-in object, plus the annotation
	// that the backup item action adds, plus the annotations that the restic
	// backupper adds
	expected := obj.DeepCopy()
	annotations := expected.GetAnnotations()
	if annotations == nil {
		annotations = make(map[string]string)
	}
	annotations["foo"] = "bar"
	expected.SetAnnotations(annotations)

	// method under test
	require.NoError(t, b.backupItem(velerotest.NewLogger(), obj, schema.ParseGroupResource("pods")))

	// get the actual backed-up item
	require.Len(t, w.data, 1)
	actual, err := velerotest.GetAsMap(string(w.data[0]))
	require.NoError(t, err)

	assert.EqualValues(t, expected.Object, actual)
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
