/*
Copyright the Velero contributors.

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
	"os"
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1api "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	velerov1api "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	"github.com/vmware-tanzu/velero/pkg/builder"
	"github.com/vmware-tanzu/velero/pkg/generated/clientset/versioned/fake"
	informers "github.com/vmware-tanzu/velero/pkg/generated/informers/externalversions"
	velerotest "github.com/vmware-tanzu/velero/pkg/test"
)

func TestGetVolumeBackupsForPod(t *testing.T) {
	tests := []struct {
		name             string
		podVolumeBackups []*velerov1api.PodVolumeBackup
		podVolumes       []corev1api.Volume
		podAnnotations   map[string]string
		podName          string
		sourcePodNs      string
		expected         map[string]string
	}{
		{
			name:           "nil annotations results in no volume backups returned",
			podAnnotations: nil,
			expected:       nil,
		},
		{
			name:           "empty annotations results in no volume backups returned",
			podAnnotations: make(map[string]string),
			expected:       nil,
		},
		{
			name:           "pod annotations with no snapshot annotation prefix results in no volume backups returned",
			podAnnotations: map[string]string{"foo": "bar"},
			expected:       nil,
		},
		{
			name:           "pod annotation with only snapshot annotation prefix, results in volume backup with empty volume key",
			podAnnotations: map[string]string{podAnnotationPrefix: "snapshotID"},
			expected:       map[string]string{"": "snapshotID"},
		},
		{
			name:           "pod annotation with snapshot annotation prefix results in volume backup with volume name and snapshot ID",
			podAnnotations: map[string]string{podAnnotationPrefix + "volume": "snapshotID"},
			expected:       map[string]string{"volume": "snapshotID"},
		},
		{
			name:           "only pod annotations with snapshot annotation prefix are considered",
			podAnnotations: map[string]string{"x": "y", podAnnotationPrefix + "volume1": "snapshot1", podAnnotationPrefix + "volume2": "snapshot2"},
			expected:       map[string]string{"volume1": "snapshot1", "volume2": "snapshot2"},
		},
		{
			name: "pod annotations are not considered if PVBs are provided",
			podVolumeBackups: []*velerov1api.PodVolumeBackup{
				builder.ForPodVolumeBackup("velero", "pvb-1").PodName("TestPod").PodNamespace("TestNS").SnapshotID("snapshot1").Volume("pvbtest1-foo").Result(),
				builder.ForPodVolumeBackup("velero", "pvb-2").PodName("TestPod").PodNamespace("TestNS").SnapshotID("snapshot2").Volume("pvbtest2-abc").Result(),
			},
			podName:        "TestPod",
			sourcePodNs:    "TestNS",
			podAnnotations: map[string]string{"x": "y", podAnnotationPrefix + "foo": "bar", podAnnotationPrefix + "abc": "123"},
			expected:       map[string]string{"pvbtest1-foo": "snapshot1", "pvbtest2-abc": "snapshot2"},
		},
		{
			name: "volume backups are returned even if no pod annotations are present",
			podVolumeBackups: []*velerov1api.PodVolumeBackup{
				builder.ForPodVolumeBackup("velero", "pvb-1").PodName("TestPod").PodNamespace("TestNS").SnapshotID("snapshot1").Volume("pvbtest1-foo").Result(),
				builder.ForPodVolumeBackup("velero", "pvb-2").PodName("TestPod").PodNamespace("TestNS").SnapshotID("snapshot2").Volume("pvbtest2-abc").Result(),
			},
			podName:     "TestPod",
			sourcePodNs: "TestNS",
			expected:    map[string]string{"pvbtest1-foo": "snapshot1", "pvbtest2-abc": "snapshot2"},
		},
		{
			name: "only volumes from PVBs with snapshot IDs are returned",
			podVolumeBackups: []*velerov1api.PodVolumeBackup{
				builder.ForPodVolumeBackup("velero", "pvb-1").PodName("TestPod").PodNamespace("TestNS").SnapshotID("snapshot1").Volume("pvbtest1-foo").Result(),
				builder.ForPodVolumeBackup("velero", "pvb-2").PodName("TestPod").PodNamespace("TestNS").SnapshotID("snapshot2").Volume("pvbtest2-abc").Result(),
				builder.ForPodVolumeBackup("velero", "pvb-3").PodName("TestPod").PodNamespace("TestNS").Volume("pvbtest3-foo").Result(),
				builder.ForPodVolumeBackup("velero", "pvb-4").PodName("TestPod").PodNamespace("TestNS").Volume("pvbtest4-abc").Result(),
			},
			podName:     "TestPod",
			sourcePodNs: "TestNS",
			expected:    map[string]string{"pvbtest1-foo": "snapshot1", "pvbtest2-abc": "snapshot2"},
		},
		{
			name: "only volumes from PVBs for the given pod are returned",
			podVolumeBackups: []*velerov1api.PodVolumeBackup{
				builder.ForPodVolumeBackup("velero", "pvb-1").PodName("TestPod").PodNamespace("TestNS").SnapshotID("snapshot1").Volume("pvbtest1-foo").Result(),
				builder.ForPodVolumeBackup("velero", "pvb-2").PodName("TestPod").PodNamespace("TestNS").SnapshotID("snapshot2").Volume("pvbtest2-abc").Result(),
				builder.ForPodVolumeBackup("velero", "pvb-3").PodName("TestAnotherPod").SnapshotID("snapshot3").Volume("pvbtest3-xyz").Result(),
			},
			podName:     "TestPod",
			sourcePodNs: "TestNS",
			expected:    map[string]string{"pvbtest1-foo": "snapshot1", "pvbtest2-abc": "snapshot2"},
		},
		{
			name: "only volumes from PVBs which match the pod name and source pod namespace are returned",
			podVolumeBackups: []*velerov1api.PodVolumeBackup{
				builder.ForPodVolumeBackup("velero", "pvb-1").PodName("TestPod").PodNamespace("TestNS").SnapshotID("snapshot1").Volume("pvbtest1-foo").Result(),
				builder.ForPodVolumeBackup("velero", "pvb-2").PodName("TestAnotherPod").PodNamespace("TestNS").SnapshotID("snapshot2").Volume("pvbtest2-abc").Result(),
				builder.ForPodVolumeBackup("velero", "pvb-3").PodName("TestPod").PodNamespace("TestAnotherNS").SnapshotID("snapshot3").Volume("pvbtest3-xyz").Result(),
			},
			podName:     "TestPod",
			sourcePodNs: "TestNS",
			expected:    map[string]string{"pvbtest1-foo": "snapshot1"},
		},
		{
			name: "volumes from PVBs that correspond to a pod volume from a projected source are not returned",
			podVolumeBackups: []*velerov1api.PodVolumeBackup{
				builder.ForPodVolumeBackup("velero", "pvb-1").PodName("TestPod").PodNamespace("TestNS").SnapshotID("snapshot1").Volume("pvb-non-projected").Result(),
				builder.ForPodVolumeBackup("velero", "pvb-1").PodName("TestPod").PodNamespace("TestNS").SnapshotID("snapshot2").Volume("pvb-projected").Result(),
			},
			podVolumes: []corev1api.Volume{
				{
					Name: "pvb-non-projected",
					VolumeSource: corev1api.VolumeSource{
						PersistentVolumeClaim: &corev1api.PersistentVolumeClaimVolumeSource{},
					},
				},
				{
					Name: "pvb-projected",
					VolumeSource: corev1api.VolumeSource{
						Projected: &corev1api.ProjectedVolumeSource{},
					},
				},
			},
			podName:     "TestPod",
			sourcePodNs: "TestNS",
			expected:    map[string]string{"pvb-non-projected": "snapshot1"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			pod := &corev1api.Pod{}
			pod.Annotations = test.podAnnotations
			pod.Name = test.podName
			pod.Spec.Volumes = test.podVolumes

			res := GetVolumeBackupsForPod(test.podVolumeBackups, pod, test.sourcePodNs)
			assert.Equal(t, test.expected, res)
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
			annotations: map[string]string{"foo": "bar", VolumesToBackupAnnotation: "volume-1"},
			expected:    []string{"volume-1"},
		},
		{
			name:        "multiple volumes to backup",
			annotations: map[string]string{"foo": "bar", VolumesToBackupAnnotation: "volume-1,volume-2,volume-3"},
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
		name                  string
		podVolumeBackups      []velerov1api.PodVolumeBackup
		expected              []SnapshotIdentifier
		longBackupNameEnabled bool
	}{
		{
			name:             "no pod volume backups",
			podVolumeBackups: nil,
			expected:         nil,
		},
		{
			name: "no pod volume backups with matching label",
			podVolumeBackups: []velerov1api.PodVolumeBackup{
				{
					ObjectMeta: metav1.ObjectMeta{Name: "foo", Labels: map[string]string{velerov1api.BackupNameLabel: "non-matching-backup-1"}},
					Spec: velerov1api.PodVolumeBackupSpec{
						Pod: corev1api.ObjectReference{Name: "pod-1", Namespace: "ns-1"},
					},
					Status: velerov1api.PodVolumeBackupStatus{SnapshotID: "snap-1"},
				},
				{
					ObjectMeta: metav1.ObjectMeta{Name: "bar", Labels: map[string]string{velerov1api.BackupNameLabel: "non-matching-backup-2"}},
					Spec: velerov1api.PodVolumeBackupSpec{
						Pod: corev1api.ObjectReference{Name: "pod-2", Namespace: "ns-2"},
					},
					Status: velerov1api.PodVolumeBackupStatus{SnapshotID: "snap-2"},
				},
			},
			expected: nil,
		},
		{
			name: "some pod volume backups with matching label",
			podVolumeBackups: []velerov1api.PodVolumeBackup{
				{
					ObjectMeta: metav1.ObjectMeta{Name: "foo", Labels: map[string]string{velerov1api.BackupNameLabel: "non-matching-backup-1"}},
					Spec: velerov1api.PodVolumeBackupSpec{
						Pod: corev1api.ObjectReference{Name: "pod-1", Namespace: "ns-1"},
					},
					Status: velerov1api.PodVolumeBackupStatus{SnapshotID: "snap-1"},
				},
				{
					ObjectMeta: metav1.ObjectMeta{Name: "bar", Labels: map[string]string{velerov1api.BackupNameLabel: "non-matching-backup-2"}},
					Spec: velerov1api.PodVolumeBackupSpec{
						Pod: corev1api.ObjectReference{Name: "pod-2", Namespace: "ns-2"},
					},
					Status: velerov1api.PodVolumeBackupStatus{SnapshotID: "snap-2"},
				},
				{
					ObjectMeta: metav1.ObjectMeta{Name: "completed-pvb", Labels: map[string]string{velerov1api.BackupNameLabel: "backup-1"}},
					Spec: velerov1api.PodVolumeBackupSpec{
						Pod: corev1api.ObjectReference{Name: "pod-1", Namespace: "ns-1"},
					},
					Status: velerov1api.PodVolumeBackupStatus{SnapshotID: "snap-3"},
				},
				{
					ObjectMeta: metav1.ObjectMeta{Name: "completed-pvb-2", Labels: map[string]string{velerov1api.BackupNameLabel: "backup-1"}},
					Spec: velerov1api.PodVolumeBackupSpec{
						Pod: corev1api.ObjectReference{Name: "pod-1", Namespace: "ns-1"},
					},
					Status: velerov1api.PodVolumeBackupStatus{SnapshotID: "snap-4"},
				},
				{
					ObjectMeta: metav1.ObjectMeta{Name: "incomplete-or-failed-pvb", Labels: map[string]string{velerov1api.BackupNameLabel: "backup-1"}},
					Spec: velerov1api.PodVolumeBackupSpec{
						Pod: corev1api.ObjectReference{Name: "pod-1", Namespace: "ns-2"},
					},
					Status: velerov1api.PodVolumeBackupStatus{SnapshotID: ""},
				},
			},
			expected: []SnapshotIdentifier{
				{
					VolumeNamespace: "ns-1",
					SnapshotID:      "snap-3",
				},
				{
					VolumeNamespace: "ns-1",
					SnapshotID:      "snap-4",
				},
			},
		},
		{
			name:                  "some pod volume backups with matching label and backup name greater than 63 chars",
			longBackupNameEnabled: true,
			podVolumeBackups: []velerov1api.PodVolumeBackup{
				{
					ObjectMeta: metav1.ObjectMeta{Name: "foo", Labels: map[string]string{velerov1api.BackupNameLabel: "non-matching-backup-1"}},
					Spec: velerov1api.PodVolumeBackupSpec{
						Pod: corev1api.ObjectReference{Name: "pod-1", Namespace: "ns-1"},
					},
					Status: velerov1api.PodVolumeBackupStatus{SnapshotID: "snap-1"},
				},
				{
					ObjectMeta: metav1.ObjectMeta{Name: "bar", Labels: map[string]string{velerov1api.BackupNameLabel: "non-matching-backup-2"}},
					Spec: velerov1api.PodVolumeBackupSpec{
						Pod: corev1api.ObjectReference{Name: "pod-2", Namespace: "ns-2"},
					},
					Status: velerov1api.PodVolumeBackupStatus{SnapshotID: "snap-2"},
				},
				{
					ObjectMeta: metav1.ObjectMeta{Name: "completed-pvb", Labels: map[string]string{velerov1api.BackupNameLabel: "the-really-long-backup-name-that-is-much-more-than-63-cha6ca4bc"}},
					Spec: velerov1api.PodVolumeBackupSpec{
						Pod: corev1api.ObjectReference{Name: "pod-1", Namespace: "ns-1"},
					},
					Status: velerov1api.PodVolumeBackupStatus{SnapshotID: "snap-3"},
				},
				{
					ObjectMeta: metav1.ObjectMeta{Name: "completed-pvb-2", Labels: map[string]string{velerov1api.BackupNameLabel: "backup-1"}},
					Spec: velerov1api.PodVolumeBackupSpec{
						Pod: corev1api.ObjectReference{Name: "pod-1", Namespace: "ns-1"},
					},
					Status: velerov1api.PodVolumeBackupStatus{SnapshotID: "snap-4"},
				},
				{
					ObjectMeta: metav1.ObjectMeta{Name: "incomplete-or-failed-pvb", Labels: map[string]string{velerov1api.BackupNameLabel: "backup-1"}},
					Spec: velerov1api.PodVolumeBackupSpec{
						Pod: corev1api.ObjectReference{Name: "pod-1", Namespace: "ns-2"},
					},
					Status: velerov1api.PodVolumeBackupStatus{SnapshotID: ""},
				},
			},
			expected: []SnapshotIdentifier{
				{
					VolumeNamespace: "ns-1",
					SnapshotID:      "snap-3",
				},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var (
				client          = fake.NewSimpleClientset()
				sharedInformers = informers.NewSharedInformerFactory(client, 0)
				pvbInformer     = sharedInformers.Velero().V1().PodVolumeBackups()
				veleroBackup    = &velerov1api.Backup{}
			)

			veleroBackup.Name = "backup-1"

			if test.longBackupNameEnabled {
				veleroBackup.Name = "the-really-long-backup-name-that-is-much-more-than-63-characters"
			}

			for _, pvb := range test.podVolumeBackups {
				require.NoError(t, pvbInformer.Informer().GetStore().Add(pvb.DeepCopy()))
			}

			res, err := GetSnapshotsInBackup(veleroBackup, pvbInformer.Lister())
			assert.NoError(t, err)

			// sort to ensure good compare of slices
			less := func(snapshots []SnapshotIdentifier) func(i, j int) bool {
				return func(i, j int) bool {
					if snapshots[i].VolumeNamespace == snapshots[j].VolumeNamespace {
						return snapshots[i].SnapshotID < snapshots[j].SnapshotID
					}
					return snapshots[i].VolumeNamespace < snapshots[j].VolumeNamespace
				}

			}

			sort.Slice(test.expected, less(test.expected))
			sort.Slice(res, less(res))

			assert.Equal(t, test.expected, res)
		})
	}
}

func TestTempCACertFile(t *testing.T) {
	var (
		fs         = velerotest.NewFakeFileSystem()
		caCertData = []byte("cacert")
	)

	fileName, err := TempCACertFile(caCertData, "default", fs)
	require.NoError(t, err)

	contents, err := fs.ReadFile(fileName)
	require.NoError(t, err)

	assert.Equal(t, string(caCertData), string(contents))

	os.Remove(fileName)
}

func TestGetPodVolumesUsingRestic(t *testing.T) {
	testCases := []struct {
		name                   string
		pod                    *corev1api.Pod
		expected               []string
		defaultVolumesToRestic bool
	}{
		{
			name:                   "should get PVs from VolumesToBackupAnnotation when defaultVolumesToRestic is false",
			defaultVolumesToRestic: false,
			pod: &corev1api.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						VolumesToBackupAnnotation: "resticPV1,resticPV2,resticPV3",
					},
				},
			},
			expected: []string{"resticPV1", "resticPV2", "resticPV3"},
		},
		{
			name:                   "should get all pod volumes when defaultVolumesToRestic is true and no PVs are excluded",
			defaultVolumesToRestic: true,
			pod: &corev1api.Pod{
				Spec: corev1api.PodSpec{
					Volumes: []corev1api.Volume{
						// Restic Volumes
						{Name: "resticPV1"}, {Name: "resticPV2"}, {Name: "resticPV3"},
					},
				},
			},
			expected: []string{"resticPV1", "resticPV2", "resticPV3"},
		},
		{
			name:                   "should get all pod volumes except ones excluded when defaultVolumesToRestic is true",
			defaultVolumesToRestic: true,
			pod: &corev1api.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						VolumesToExcludeAnnotation: "nonResticPV1,nonResticPV2,nonResticPV3",
					},
				},
				Spec: corev1api.PodSpec{
					Volumes: []corev1api.Volume{
						// Restic Volumes
						{Name: "resticPV1"}, {Name: "resticPV2"}, {Name: "resticPV3"},
						/// Excluded from restic through annotation
						{Name: "nonResticPV1"}, {Name: "nonResticPV2"}, {Name: "nonResticPV3"},
					},
				},
			},
			expected: []string{"resticPV1", "resticPV2", "resticPV3"},
		},
		{
			name:                   "should exclude default service account token from restic backup",
			defaultVolumesToRestic: true,
			pod: &corev1api.Pod{
				Spec: corev1api.PodSpec{
					Volumes: []corev1api.Volume{
						// Restic Volumes
						{Name: "resticPV1"}, {Name: "resticPV2"}, {Name: "resticPV3"},
						/// Excluded from restic because colume mounting default service account token
						{Name: "default-token-5xq45"},
					},
				},
			},
			expected: []string{"resticPV1", "resticPV2", "resticPV3"},
		},
		{
			name:                   "should exclude host path volumes from restic backups",
			defaultVolumesToRestic: true,
			pod: &corev1api.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						VolumesToExcludeAnnotation: "nonResticPV1,nonResticPV2,nonResticPV3",
					},
				},
				Spec: corev1api.PodSpec{
					Volumes: []corev1api.Volume{
						// Restic Volumes
						{Name: "resticPV1"}, {Name: "resticPV2"}, {Name: "resticPV3"},
						/// Excluded from restic through annotation
						{Name: "nonResticPV1"}, {Name: "nonResticPV2"}, {Name: "nonResticPV3"},
						// Excluded from restic because hostpath
						{Name: "hostPath1", VolumeSource: corev1api.VolumeSource{HostPath: &corev1api.HostPathVolumeSource{Path: "/hostpathVol"}}},
					},
				},
			},
			expected: []string{"resticPV1", "resticPV2", "resticPV3"},
		},
		{
			name:                   "should exclude volumes mounting secrets",
			defaultVolumesToRestic: true,
			pod: &corev1api.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						VolumesToExcludeAnnotation: "nonResticPV1,nonResticPV2,nonResticPV3",
					},
				},
				Spec: corev1api.PodSpec{
					Volumes: []corev1api.Volume{
						// Restic Volumes
						{Name: "resticPV1"}, {Name: "resticPV2"}, {Name: "resticPV3"},
						/// Excluded from restic through annotation
						{Name: "nonResticPV1"}, {Name: "nonResticPV2"}, {Name: "nonResticPV3"},
						// Excluded from restic because hostpath
						{Name: "superSecret", VolumeSource: corev1api.VolumeSource{Secret: &corev1api.SecretVolumeSource{SecretName: "super-secret"}}},
					},
				},
			},
			expected: []string{"resticPV1", "resticPV2", "resticPV3"},
		},
		{
			name:                   "should exclude volumes mounting config maps",
			defaultVolumesToRestic: true,
			pod: &corev1api.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						VolumesToExcludeAnnotation: "nonResticPV1,nonResticPV2,nonResticPV3",
					},
				},
				Spec: corev1api.PodSpec{
					Volumes: []corev1api.Volume{
						// Restic Volumes
						{Name: "resticPV1"}, {Name: "resticPV2"}, {Name: "resticPV3"},
						/// Excluded from restic through annotation
						{Name: "nonResticPV1"}, {Name: "nonResticPV2"}, {Name: "nonResticPV3"},
						// Excluded from restic because hostpath
						{Name: "appCOnfig", VolumeSource: corev1api.VolumeSource{ConfigMap: &corev1api.ConfigMapVolumeSource{LocalObjectReference: corev1api.LocalObjectReference{Name: "app-config"}}}},
					},
				},
			},
			expected: []string{"resticPV1", "resticPV2", "resticPV3"},
		},
		{
			name:                   "should exclude projected volumes",
			defaultVolumesToRestic: true,
			pod: &corev1api.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						VolumesToExcludeAnnotation: "nonResticPV1,nonResticPV2,nonResticPV3",
					},
				},
				Spec: corev1api.PodSpec{
					Volumes: []corev1api.Volume{
						{Name: "resticPV1"}, {Name: "resticPV2"}, {Name: "resticPV3"},
						{
							Name: "projected",
							VolumeSource: corev1api.VolumeSource{
								Projected: &corev1api.ProjectedVolumeSource{
									Sources: []corev1api.VolumeProjection{{
										Secret: &corev1api.SecretProjection{
											LocalObjectReference: corev1api.LocalObjectReference{},
											Items:                nil,
											Optional:             nil,
										},
										DownwardAPI:         nil,
										ConfigMap:           nil,
										ServiceAccountToken: nil,
									}},
									DefaultMode: nil,
								},
							},
						},
					},
				},
			},
			expected: []string{"resticPV1", "resticPV2", "resticPV3"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actual := GetPodVolumesUsingRestic(tc.pod, tc.defaultVolumesToRestic)

			sort.Strings(tc.expected)
			sort.Strings(actual)
			assert.Equal(t, tc.expected, actual)
		})
	}
}

func TestIsPVBMatchPod(t *testing.T) {
	testCases := []struct {
		name        string
		pvb         velerov1api.PodVolumeBackup
		podName     string
		sourcePodNs string
		expected    bool
	}{
		{
			name: "should match PVB and pod",
			pvb: velerov1api.PodVolumeBackup{
				Spec: velerov1api.PodVolumeBackupSpec{
					Pod: corev1api.ObjectReference{
						Name:      "matching-pod",
						Namespace: "matching-namespace",
					},
				},
			},
			podName:     "matching-pod",
			sourcePodNs: "matching-namespace",
			expected:    true,
		},
		{
			name: "should not match PVB and pod, pod name mismatch",
			pvb: velerov1api.PodVolumeBackup{
				Spec: velerov1api.PodVolumeBackupSpec{
					Pod: corev1api.ObjectReference{
						Name:      "matching-pod",
						Namespace: "matching-namespace",
					},
				},
			},
			podName:     "not-matching-pod",
			sourcePodNs: "matching-namespace",
			expected:    false,
		},
		{
			name: "should not match PVB and pod, pod namespace mismatch",
			pvb: velerov1api.PodVolumeBackup{
				Spec: velerov1api.PodVolumeBackupSpec{
					Pod: corev1api.ObjectReference{
						Name:      "matching-pod",
						Namespace: "matching-namespace",
					},
				},
			},
			podName:     "matching-pod",
			sourcePodNs: "not-matching-namespace",
			expected:    false,
		},
		{
			name: "should not match PVB and pod, pod name and namespace mismatch",
			pvb: velerov1api.PodVolumeBackup{
				Spec: velerov1api.PodVolumeBackupSpec{
					Pod: corev1api.ObjectReference{
						Name:      "matching-pod",
						Namespace: "matching-namespace",
					},
				},
			},
			podName:     "not-matching-pod",
			sourcePodNs: "not-matching-namespace",
			expected:    false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actual := isPVBMatchPod(&tc.pvb, tc.podName, tc.sourcePodNs)
			assert.Equal(t, tc.expected, actual)
		})

	}
}

func TestVolumeIsProjected(t *testing.T) {
	testCases := []struct {
		name       string
		volumeName string
		podVolumes []corev1api.Volume
		expected   bool
	}{
		{
			name:       "volume name not in list of volumes",
			volumeName: "missing-volume",
			podVolumes: []corev1api.Volume{
				{
					Name: "non-projected",
					VolumeSource: corev1api.VolumeSource{
						PersistentVolumeClaim: &corev1api.PersistentVolumeClaimVolumeSource{},
					},
				},
				{
					Name: "projected",
					VolumeSource: corev1api.VolumeSource{
						Projected: &corev1api.ProjectedVolumeSource{},
					},
				},
			},
			expected: false,
		},
		{
			name:       "volume name in list of volumes but not projected",
			volumeName: "non-projected",
			podVolumes: []corev1api.Volume{
				{
					Name: "non-projected",
					VolumeSource: corev1api.VolumeSource{
						PersistentVolumeClaim: &corev1api.PersistentVolumeClaimVolumeSource{},
					},
				},
				{
					Name: "projected",
					VolumeSource: corev1api.VolumeSource{
						Projected: &corev1api.ProjectedVolumeSource{},
					},
				},
			},
			expected: false,
		},
		{
			name:       "volume name in list of volumes and projected",
			volumeName: "projected",
			podVolumes: []corev1api.Volume{
				{
					Name: "non-projected",
					VolumeSource: corev1api.VolumeSource{
						PersistentVolumeClaim: &corev1api.PersistentVolumeClaimVolumeSource{},
					},
				},
				{
					Name: "projected",
					VolumeSource: corev1api.VolumeSource{
						Projected: &corev1api.ProjectedVolumeSource{},
					},
				},
			},
			expected: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actual := volumeIsProjected(tc.volumeName, tc.podVolumes)
			assert.Equal(t, tc.expected, actual)
		})

	}
}
