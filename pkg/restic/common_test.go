/*
Copyright 2018 the Heptio Ark contributors.

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

package restic

import (
	"sort"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/stretchr/testify/assert"

	corev1api "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	corev1listers "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"

	arkv1api "github.com/heptio/ark/pkg/apis/ark/v1"
	"github.com/heptio/ark/pkg/generated/clientset/versioned/fake"
	informers "github.com/heptio/ark/pkg/generated/informers/externalversions"
	arktest "github.com/heptio/ark/pkg/util/test"
)

func TestPodHasSnapshotAnnotation(t *testing.T) {
	tests := []struct {
		name        string
		annotations map[string]string
		expected    bool
	}{
		{
			name:        "nil annotations",
			annotations: nil,
			expected:    false,
		},
		{
			name:        "empty annotations",
			annotations: make(map[string]string),
			expected:    false,
		},
		{
			name:        "non-empty map, no snapshot annotation",
			annotations: map[string]string{"foo": "bar"},
			expected:    false,
		},
		{
			name:        "has snapshot annotation only, no suffix",
			annotations: map[string]string{podAnnotationPrefix: "bar"},
			expected:    true,
		},
		{
			name:        "has snapshot annotation only, with suffix",
			annotations: map[string]string{podAnnotationPrefix + "foo": "bar"},
			expected:    true,
		},
		{
			name:        "has snapshot annotation, with suffix",
			annotations: map[string]string{"foo": "bar", podAnnotationPrefix + "foo": "bar"},
			expected:    true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			pod := &corev1api.Pod{}
			pod.Annotations = test.annotations
			assert.Equal(t, test.expected, PodHasSnapshotAnnotation(pod))
		})
	}
}

func TestGetPodSnapshotAnnotations(t *testing.T) {
	tests := []struct {
		name        string
		annotations map[string]string
		expected    map[string]string
	}{
		{
			name:        "nil annotations",
			annotations: nil,
			expected:    nil,
		},
		{
			name:        "empty annotations",
			annotations: make(map[string]string),
			expected:    nil,
		},
		{
			name:        "non-empty map, no snapshot annotation",
			annotations: map[string]string{"foo": "bar"},
			expected:    nil,
		},
		{
			name:        "has snapshot annotation only, no suffix",
			annotations: map[string]string{podAnnotationPrefix: "bar"},
			expected:    map[string]string{"": "bar"},
		},
		{
			name:        "has snapshot annotation only, with suffix",
			annotations: map[string]string{podAnnotationPrefix + "foo": "bar"},
			expected:    map[string]string{"foo": "bar"},
		},
		{
			name:        "has snapshot annotation, with suffix",
			annotations: map[string]string{"x": "y", podAnnotationPrefix + "foo": "bar", podAnnotationPrefix + "abc": "123"},
			expected:    map[string]string{"foo": "bar", "abc": "123"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			pod := &corev1api.Pod{}
			pod.Annotations = test.annotations
			assert.Equal(t, test.expected, GetPodSnapshotAnnotations(pod))
		})
	}
}

func TestSetPodSnapshotAnnotation(t *testing.T) {
	tests := []struct {
		name        string
		annotations map[string]string
		volumeName  string
		snapshotID  string
		expected    map[string]string
	}{
		{
			name:        "set snapshot annotation on pod with no annotations",
			annotations: nil,
			volumeName:  "foo",
			snapshotID:  "bar",
			expected:    map[string]string{podAnnotationPrefix + "foo": "bar"},
		},
		{
			name:        "set snapshot annotation on pod with existing annotations",
			annotations: map[string]string{"existing": "annotation"},
			volumeName:  "foo",
			snapshotID:  "bar",
			expected:    map[string]string{"existing": "annotation", podAnnotationPrefix + "foo": "bar"},
		},
		{
			name:        "snapshot annotation is overwritten if already exists",
			annotations: map[string]string{podAnnotationPrefix + "foo": "existing"},
			volumeName:  "foo",
			snapshotID:  "bar",
			expected:    map[string]string{podAnnotationPrefix + "foo": "bar"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			pod := &corev1api.Pod{}
			pod.Annotations = test.annotations

			SetPodSnapshotAnnotation(pod, test.volumeName, test.snapshotID)
			assert.Equal(t, test.expected, pod.Annotations)
		})
	}
}

func TestGetVolumesToBackup(t *testing.T) {
	tests := []struct {
		name        string
		annotations map[string]string
		expected    []string
	}{
		{
			name:        "nil annotations",
			annotations: nil,
			expected:    nil,
		},
		{
			name:        "no volumes to backup",
			annotations: map[string]string{"foo": "bar"},
			expected:    nil,
		},
		{
			name:        "one volume to backup",
			annotations: map[string]string{"foo": "bar", volumesToBackupAnnotation: "volume-1"},
			expected:    []string{"volume-1"},
		},
		{
			name:        "multiple volumes to backup",
			annotations: map[string]string{"foo": "bar", volumesToBackupAnnotation: "volume-1,volume-2,volume-3"},
			expected:    []string{"volume-1", "volume-2", "volume-3"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			pod := &corev1api.Pod{}
			pod.Annotations = test.annotations

			res := GetVolumesToBackup(pod)

			// sort to ensure good compare of slices
			sort.Strings(test.expected)
			sort.Strings(res)

			assert.Equal(t, test.expected, res)
		})
	}
}

func TestGetSnapshotsInBackup(t *testing.T) {
	tests := []struct {
		name             string
		podVolumeBackups []arkv1api.PodVolumeBackup
		expected         []SnapshotIdentifier
	}{
		{
			name:             "no pod volume backups",
			podVolumeBackups: nil,
			expected:         nil,
		},
		{
			name: "no pod volume backups with matching label",
			podVolumeBackups: []arkv1api.PodVolumeBackup{
				{
					ObjectMeta: metav1.ObjectMeta{Name: "foo", Labels: map[string]string{arkv1api.BackupNameLabel: "non-matching-backup-1"}},
					Spec: arkv1api.PodVolumeBackupSpec{
						Pod: corev1api.ObjectReference{Name: "pod-1", Namespace: "ns-1"},
					},
					Status: arkv1api.PodVolumeBackupStatus{SnapshotID: "snap-1"},
				},
				{
					ObjectMeta: metav1.ObjectMeta{Name: "bar", Labels: map[string]string{arkv1api.BackupNameLabel: "non-matching-backup-2"}},
					Spec: arkv1api.PodVolumeBackupSpec{
						Pod: corev1api.ObjectReference{Name: "pod-2", Namespace: "ns-2"},
					},
					Status: arkv1api.PodVolumeBackupStatus{SnapshotID: "snap-2"},
				},
			},
			expected: nil,
		},
		{
			name: "some pod volume backups with matching label",
			podVolumeBackups: []arkv1api.PodVolumeBackup{
				{
					ObjectMeta: metav1.ObjectMeta{Name: "foo", Labels: map[string]string{arkv1api.BackupNameLabel: "non-matching-backup-1"}},
					Spec: arkv1api.PodVolumeBackupSpec{
						Pod: corev1api.ObjectReference{Name: "pod-1", Namespace: "ns-1"},
					},
					Status: arkv1api.PodVolumeBackupStatus{SnapshotID: "snap-1"},
				},
				{
					ObjectMeta: metav1.ObjectMeta{Name: "bar", Labels: map[string]string{arkv1api.BackupNameLabel: "non-matching-backup-2"}},
					Spec: arkv1api.PodVolumeBackupSpec{
						Pod: corev1api.ObjectReference{Name: "pod-2", Namespace: "ns-2"},
					},
					Status: arkv1api.PodVolumeBackupStatus{SnapshotID: "snap-2"},
				},
				{
					ObjectMeta: metav1.ObjectMeta{Name: "completed-pvb", Labels: map[string]string{arkv1api.BackupNameLabel: "backup-1"}},
					Spec: arkv1api.PodVolumeBackupSpec{
						Pod: corev1api.ObjectReference{Name: "pod-1", Namespace: "ns-1"},
					},
					Status: arkv1api.PodVolumeBackupStatus{SnapshotID: "snap-3"},
				},
				{
					ObjectMeta: metav1.ObjectMeta{Name: "completed-pvb-2", Labels: map[string]string{arkv1api.BackupNameLabel: "backup-1"}},
					Spec: arkv1api.PodVolumeBackupSpec{
						Pod: corev1api.ObjectReference{Name: "pod-1", Namespace: "ns-1"},
					},
					Status: arkv1api.PodVolumeBackupStatus{SnapshotID: "snap-4"},
				},
				{
					ObjectMeta: metav1.ObjectMeta{Name: "incomplete-or-failed-pvb", Labels: map[string]string{arkv1api.BackupNameLabel: "backup-1"}},
					Spec: arkv1api.PodVolumeBackupSpec{
						Pod: corev1api.ObjectReference{Name: "pod-1", Namespace: "ns-2"},
					},
					Status: arkv1api.PodVolumeBackupStatus{SnapshotID: ""},
				},
			},
			expected: []SnapshotIdentifier{
				{
					Repo:       "ns-1",
					SnapshotID: "snap-3",
				},
				{
					Repo:       "ns-1",
					SnapshotID: "snap-4",
				},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var (
				client          = fake.NewSimpleClientset()
				sharedInformers = informers.NewSharedInformerFactory(client, 0)
				pvbInformer     = sharedInformers.Ark().V1().PodVolumeBackups()
				arkBackup       = &arkv1api.Backup{}
			)

			arkBackup.Name = "backup-1"

			for _, pvb := range test.podVolumeBackups {
				require.NoError(t, pvbInformer.Informer().GetStore().Add(pvb.DeepCopy()))
			}

			res, err := GetSnapshotsInBackup(arkBackup, pvbInformer.Lister())
			assert.NoError(t, err)

			// sort to ensure good compare of slices
			less := func(snapshots []SnapshotIdentifier) func(i, j int) bool {
				return func(i, j int) bool {
					return snapshots[i].Repo < snapshots[j].Repo &&
						snapshots[i].SnapshotID < snapshots[j].SnapshotID
				}

			}
			sort.Slice(test.expected, less(test.expected))
			sort.Slice(res, less(res))

			assert.Equal(t, test.expected, res)
		})
	}
}

func TestTempCredentialsFile(t *testing.T) {
	var (
		secretInformer = cache.NewSharedIndexInformer(nil, new(corev1api.Secret), 0, cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc})
		secretLister   = corev1listers.NewSecretLister(secretInformer.GetIndexer())
		fs             = arktest.NewFakeFileSystem()
		secret         = &corev1api.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "heptio-ark",
				Name:      CredentialsSecretName,
			},
			Data: map[string][]byte{
				CredentialsKey: []byte("passw0rd"),
			},
		}
	)

	// secret not in lister: expect an error
	fileName, err := TempCredentialsFile(secretLister, "heptio-ark", "default", fs)
	assert.Error(t, err)

	// now add secret to lister
	require.NoError(t, secretInformer.GetStore().Add(secret))

	// secret in lister: expect temp file to be created with password
	fileName, err = TempCredentialsFile(secretLister, "heptio-ark", "default", fs)
	require.NoError(t, err)

	contents, err := fs.ReadFile(fileName)
	require.NoError(t, err)

	assert.Equal(t, "passw0rd", string(contents))
}
