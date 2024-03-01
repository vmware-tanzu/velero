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
)

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
			annotations: map[string]string{"foo": "bar", velerov1api.VolumesToBackupAnnotation: "volume-1"},
			expected:    []string{"volume-1"},
		},
		{
			name:        "multiple volumes to backup",
			annotations: map[string]string{"foo": "bar", velerov1api.VolumesToBackupAnnotation: "volume-1,volume-2,volume-3"},
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
		name     string
		pod      *corev1api.Pod
		expected struct {
			included []string
			optedOut []string
		}
		defaultVolumesToFsBackup bool
		backupExcludePVC         bool
	}{
		{
			name:                     "should get PVs from VolumesToBackupAnnotation when defaultVolumesToFsBackup is false",
			defaultVolumesToFsBackup: false,
			pod: &corev1api.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						velerov1api.VolumesToBackupAnnotation: "pvbPV1,pvbPV2,pvbPV3",
					},
				},
			},
			expected: struct {
				included []string
				optedOut []string
			}{
				included: []string{"pvbPV1", "pvbPV2", "pvbPV3"},
				optedOut: []string{},
			},
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
			expected: struct {
				included []string
				optedOut []string
			}{
				included: []string{"pvbPV1", "pvbPV2", "pvbPV3"},
				optedOut: []string{},
			},
		},
		{
			name:                     "should get all pod volumes except ones excluded when defaultVolumesToFsBackup is true",
			defaultVolumesToFsBackup: true,
			pod: &corev1api.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						velerov1api.VolumesToExcludeAnnotation: "nonPvbPV1,nonPvbPV2,nonPvbPV3",
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
			expected: struct {
				included []string
				optedOut []string
			}{
				included: []string{"pvbPV1", "pvbPV2", "pvbPV3"},
				optedOut: []string{"nonPvbPV1", "nonPvbPV2", "nonPvbPV3"},
			},
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
			expected: struct {
				included []string
				optedOut []string
			}{
				included: []string{"pvbPV1", "pvbPV2", "pvbPV3"},
				optedOut: []string{},
			},
		},
		{
			name:                     "should exclude host path volumes from pod volume backups",
			defaultVolumesToFsBackup: true,
			pod: &corev1api.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						velerov1api.VolumesToExcludeAnnotation: "nonPvbPV1,nonPvbPV2,nonPvbPV3",
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
			expected: struct {
				included []string
				optedOut []string
			}{
				included: []string{"pvbPV1", "pvbPV2", "pvbPV3"},
				optedOut: []string{"nonPvbPV1", "nonPvbPV2", "nonPvbPV3"},
			},
		},
		{
			name:                     "should exclude volumes mounting secrets",
			defaultVolumesToFsBackup: true,
			pod: &corev1api.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						velerov1api.VolumesToExcludeAnnotation: "nonPvbPV1,nonPvbPV2,nonPvbPV3",
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
			expected: struct {
				included []string
				optedOut []string
			}{
				included: []string{"pvbPV1", "pvbPV2", "pvbPV3"},
				optedOut: []string{"nonPvbPV1", "nonPvbPV2", "nonPvbPV3"},
			},
		},
		{
			name:                     "should exclude volumes mounting ConfigMaps",
			defaultVolumesToFsBackup: true,
			pod: &corev1api.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						velerov1api.VolumesToExcludeAnnotation: "nonPvbPV1,nonPvbPV2,nonPvbPV3",
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
			expected: struct {
				included []string
				optedOut []string
			}{
				included: []string{"pvbPV1", "pvbPV2", "pvbPV3"},
				optedOut: []string{"nonPvbPV1", "nonPvbPV2", "nonPvbPV3"},
			},
		},
		{
			name:                     "should exclude projected volumes",
			defaultVolumesToFsBackup: true,
			pod: &corev1api.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						velerov1api.VolumesToExcludeAnnotation: "nonPvbPV1,nonPvbPV2,nonPvbPV3",
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
			expected: struct {
				included []string
				optedOut []string
			}{
				included: []string{"pvbPV1", "pvbPV2", "pvbPV3"},
				optedOut: []string{},
			},
		},
		{
			name:                     "should exclude DownwardAPI volumes",
			defaultVolumesToFsBackup: true,
			pod: &corev1api.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						velerov1api.VolumesToExcludeAnnotation: "nonPvbPV1,nonPvbPV2,nonPvbPV3",
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
			expected: struct {
				included []string
				optedOut []string
			}{
				included: []string{"pvbPV1", "pvbPV2", "pvbPV3"},
				optedOut: []string{},
			},
		},
		{
			name:                     "should exclude PVC volume when backup excludes PVC resource",
			defaultVolumesToFsBackup: true,
			backupExcludePVC:         true,
			pod: &corev1api.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						velerov1api.VolumesToExcludeAnnotation: "nonPvbPV1,nonPvbPV2,nonPvbPV3",
					},
				},
				Spec: corev1api.PodSpec{
					Volumes: []corev1api.Volume{
						{Name: "pvbPV1"}, {Name: "pvbPV2"}, {Name: "pvbPV3"},
						{
							Name: "downwardAPI",
							VolumeSource: corev1api.VolumeSource{
								PersistentVolumeClaim: &corev1api.PersistentVolumeClaimVolumeSource{
									ClaimName: "testPVC",
								},
							},
						},
					},
				},
			},
			expected: struct {
				included []string
				optedOut []string
			}{
				included: []string{"pvbPV1", "pvbPV2", "pvbPV3"},
				optedOut: []string{},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actualIncluded, actualOptedOut := GetVolumesByPod(tc.pod, tc.defaultVolumesToFsBackup, tc.backupExcludePVC)

			sort.Strings(tc.expected.included)
			sort.Strings(actualIncluded)
			assert.Equal(t, tc.expected.included, actualIncluded)

			sort.Strings(tc.expected.optedOut)
			sort.Strings(actualOptedOut)
			assert.Equal(t, tc.expected.optedOut, actualOptedOut)
		})
	}
}
