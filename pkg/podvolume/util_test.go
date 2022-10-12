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

package podvolume

import (
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
	corev1api "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	velerov1api "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	"github.com/vmware-tanzu/velero/pkg/builder"
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
		{
			name: "volumes from PVBs that correspond to a pod volume from a DownwardAPI source are not returned",
			podVolumeBackups: []*velerov1api.PodVolumeBackup{
				builder.ForPodVolumeBackup("velero", "pvb-1").PodName("TestPod").PodNamespace("TestNS").SnapshotID("snapshot1").Volume("pvb-non-downwardapi").Result(),
				builder.ForPodVolumeBackup("velero", "pvb-1").PodName("TestPod").PodNamespace("TestNS").SnapshotID("snapshot2").Volume("pvb-downwardapi").Result(),
			},
			podVolumes: []corev1api.Volume{
				{
					Name: "pvb-non-downwardapi",
					VolumeSource: corev1api.VolumeSource{
						PersistentVolumeClaim: &corev1api.PersistentVolumeClaimVolumeSource{},
					},
				},
				{
					Name: "pvb-downwardapi",
					VolumeSource: corev1api.VolumeSource{
						DownwardAPI: &corev1api.DownwardAPIVolumeSource{},
					},
				},
			},
			podName:     "TestPod",
			sourcePodNs: "TestNS",
			expected:    map[string]string{"pvb-non-downwardapi": "snapshot1"},
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

func TestVolumeHasNonRestorableSource(t *testing.T) {
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
					Name: "restorable",
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
				{
					Name: "downwardapi",
					VolumeSource: corev1api.VolumeSource{
						DownwardAPI: &corev1api.DownwardAPIVolumeSource{},
					},
				},
			},
			expected: false,
		},
		{
			name:       "volume name in list of volumes but not projected or DownwardAPI",
			volumeName: "restorable",
			podVolumes: []corev1api.Volume{
				{
					Name: "restorable",
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
				{
					Name: "downwardapi",
					VolumeSource: corev1api.VolumeSource{
						DownwardAPI: &corev1api.DownwardAPIVolumeSource{},
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
					Name: "restorable",
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
				{
					Name: "downwardapi",
					VolumeSource: corev1api.VolumeSource{
						DownwardAPI: &corev1api.DownwardAPIVolumeSource{},
					},
				},
			},
			expected: true,
		},
		{
			name:       "volume name in list of volumes and is a DownwardAPI volume",
			volumeName: "downwardapi",
			podVolumes: []corev1api.Volume{
				{
					Name: "restorable",
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
				{
					Name: "downwardapi",
					VolumeSource: corev1api.VolumeSource{
						DownwardAPI: &corev1api.DownwardAPIVolumeSource{},
					},
				},
			},
			expected: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actual := volumeHasNonRestorableSource(tc.volumeName, tc.podVolumes)
			assert.Equal(t, tc.expected, actual)
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

func TestGetVolumesByPod(t *testing.T) {
	testCases := []struct {
		name                     string
		pod                      *corev1api.Pod
		expected                 []string
		defaultVolumesToFsBackup bool
	}{
		{
			name:                     "should get PVs from VolumesToBackupAnnotation when defaultVolumesToFsBackup is false",
			defaultVolumesToFsBackup: false,
			pod: &corev1api.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						VolumesToBackupAnnotation: "pvbPV1,pvbPV2,pvbPV3",
					},
				},
			},
			expected: []string{"pvbPV1", "pvbPV2", "pvbPV3"},
		},
		{
			name:                     "should get all pod volumes when defaultVolumesToFsBackup is true and no PVs are excluded",
			defaultVolumesToFsBackup: true,
			pod: &corev1api.Pod{
				Spec: corev1api.PodSpec{
					Volumes: []corev1api.Volume{
						// PVB Volumes
						{Name: "pvbPV1"}, {Name: "pvbPV2"}, {Name: "pvbPV3"},
					},
				},
			},
			expected: []string{"pvbPV1", "pvbPV2", "pvbPV3"},
		},
		{
			name:                     "should get all pod volumes except ones excluded when defaultVolumesToFsBackup is true",
			defaultVolumesToFsBackup: true,
			pod: &corev1api.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						VolumesToExcludeAnnotation: "nonPvbPV1,nonPvbPV2,nonPvbPV3",
					},
				},
				Spec: corev1api.PodSpec{
					Volumes: []corev1api.Volume{
						// PVB Volumes
						{Name: "pvbPV1"}, {Name: "pvbPV2"}, {Name: "pvbPV3"},
						/// Excluded from PVB through annotation
						{Name: "nonPvbPV1"}, {Name: "nonPvbPV2"}, {Name: "nonPvbPV3"},
					},
				},
			},
			expected: []string{"pvbPV1", "pvbPV2", "pvbPV3"},
		},
		{
			name:                     "should exclude default service account token from pod volume backup",
			defaultVolumesToFsBackup: true,
			pod: &corev1api.Pod{
				Spec: corev1api.PodSpec{
					Volumes: []corev1api.Volume{
						// PVB Volumes
						{Name: "pvbPV1"}, {Name: "pvbPV2"}, {Name: "pvbPV3"},
						/// Excluded from PVB because colume mounting default service account token
						{Name: "default-token-5xq45"},
					},
				},
			},
			expected: []string{"pvbPV1", "pvbPV2", "pvbPV3"},
		},
		{
			name:                     "should exclude host path volumes from pod volume backups",
			defaultVolumesToFsBackup: true,
			pod: &corev1api.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						VolumesToExcludeAnnotation: "nonPvbPV1,nonPvbPV2,nonPvbPV3",
					},
				},
				Spec: corev1api.PodSpec{
					Volumes: []corev1api.Volume{
						// PVB Volumes
						{Name: "pvbPV1"}, {Name: "pvbPV2"}, {Name: "pvbPV3"},
						/// Excluded from pod volume backup through annotation
						{Name: "nonPvbPV1"}, {Name: "nonPvbPV2"}, {Name: "nonPvbPV3"},
						// Excluded from pod volume backup because hostpath
						{Name: "hostPath1", VolumeSource: corev1api.VolumeSource{HostPath: &corev1api.HostPathVolumeSource{Path: "/hostpathVol"}}},
					},
				},
			},
			expected: []string{"pvbPV1", "pvbPV2", "pvbPV3"},
		},
		{
			name:                     "should exclude volumes mounting secrets",
			defaultVolumesToFsBackup: true,
			pod: &corev1api.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						VolumesToExcludeAnnotation: "nonPvbPV1,nonPvbPV2,nonPvbPV3",
					},
				},
				Spec: corev1api.PodSpec{
					Volumes: []corev1api.Volume{
						// PVB Volumes
						{Name: "pvbPV1"}, {Name: "pvbPV2"}, {Name: "pvbPV3"},
						/// Excluded from pod volume backup through annotation
						{Name: "nonPvbPV1"}, {Name: "nonPvbPV2"}, {Name: "nonPvbPV3"},
						// Excluded from pod volume backup because hostpath
						{Name: "superSecret", VolumeSource: corev1api.VolumeSource{Secret: &corev1api.SecretVolumeSource{SecretName: "super-secret"}}},
					},
				},
			},
			expected: []string{"pvbPV1", "pvbPV2", "pvbPV3"},
		},
		{
			name:                     "should exclude volumes mounting config maps",
			defaultVolumesToFsBackup: true,
			pod: &corev1api.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						VolumesToExcludeAnnotation: "nonPvbPV1,nonPvbPV2,nonPvbPV3",
					},
				},
				Spec: corev1api.PodSpec{
					Volumes: []corev1api.Volume{
						// PVB Volumes
						{Name: "pvbPV1"}, {Name: "pvbPV2"}, {Name: "pvbPV3"},
						/// Excluded from pod volume backup through annotation
						{Name: "nonPvbPV1"}, {Name: "nonPvbPV2"}, {Name: "nonPvbPV3"},
						// Excluded from pod volume backup because hostpath
						{Name: "appCOnfig", VolumeSource: corev1api.VolumeSource{ConfigMap: &corev1api.ConfigMapVolumeSource{LocalObjectReference: corev1api.LocalObjectReference{Name: "app-config"}}}},
					},
				},
			},
			expected: []string{"pvbPV1", "pvbPV2", "pvbPV3"},
		},
		{
			name:                     "should exclude projected volumes",
			defaultVolumesToFsBackup: true,
			pod: &corev1api.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						VolumesToExcludeAnnotation: "nonPvbPV1,nonPvbPV2,nonPvbPV3",
					},
				},
				Spec: corev1api.PodSpec{
					Volumes: []corev1api.Volume{
						{Name: "pvbPV1"}, {Name: "pvbPV2"}, {Name: "pvbPV3"},
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
			expected: []string{"pvbPV1", "pvbPV2", "pvbPV3"},
		},
		{
			name:                     "should exclude DownwardAPI volumes",
			defaultVolumesToFsBackup: true,
			pod: &corev1api.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						VolumesToExcludeAnnotation: "nonPvbPV1,nonPvbPV2,nonPvbPV3",
					},
				},
				Spec: corev1api.PodSpec{
					Volumes: []corev1api.Volume{
						{Name: "pvbPV1"}, {Name: "pvbPV2"}, {Name: "pvbPV3"},
						{
							Name: "downwardAPI",
							VolumeSource: corev1api.VolumeSource{
								DownwardAPI: &corev1api.DownwardAPIVolumeSource{
									Items: []corev1api.DownwardAPIVolumeFile{
										{
											Path: "labels",
											FieldRef: &corev1api.ObjectFieldSelector{
												APIVersion: "v1",
												FieldPath:  "metadata.labels",
											},
										},
									},
								},
							},
						},
					},
				},
			},
			expected: []string{"pvbPV1", "pvbPV2", "pvbPV3"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actual := GetVolumesByPod(tc.pod, tc.defaultVolumesToFsBackup)

			sort.Strings(tc.expected)
			sort.Strings(actual)
			assert.Equal(t, tc.expected, actual)
		})
	}
}
