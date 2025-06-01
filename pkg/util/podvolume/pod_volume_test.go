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
	"github.com/stretchr/testify/require"
	corev1api "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	velerov1api "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	velerotest "github.com/vmware-tanzu/velero/pkg/test"
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
			actualIncluded, actualOptedOut := GetVolumesByPod(tc.pod, tc.defaultVolumesToFsBackup, tc.backupExcludePVC, []string{})

			sort.Strings(tc.expected.included)
			sort.Strings(actualIncluded)
			assert.Equal(t, tc.expected.included, actualIncluded)

			sort.Strings(tc.expected.optedOut)
			sort.Strings(actualOptedOut)
			assert.Equal(t, tc.expected.optedOut, actualOptedOut)
		})
	}
}

func TestIsPVCDefaultToFSBackup(t *testing.T) {
	objs := []runtime.Object{
		&corev1api.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "pod1",
				Namespace: "default",
			},
			Spec: corev1api.PodSpec{
				Volumes: []corev1api.Volume{
					{
						Name: "csi-vol1",
						VolumeSource: corev1api.VolumeSource{
							PersistentVolumeClaim: &corev1api.PersistentVolumeClaimVolumeSource{
								ClaimName: "csi-pvc1",
							},
						},
					},
				},
			},
		},
		&corev1api.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "pod2",
				Namespace: "default",
				Annotations: map[string]string{
					"backup.velero.io/backup-volumes": "csi-vol1",
				},
			},
			Spec: corev1api.PodSpec{
				Volumes: []corev1api.Volume{
					{
						Name: "csi-vol1",
						VolumeSource: corev1api.VolumeSource{
							PersistentVolumeClaim: &corev1api.PersistentVolumeClaimVolumeSource{
								ClaimName: "csi-pvc1",
							},
						},
					},
				},
			},
		},
		&corev1api.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "pod3",
				Namespace: "default",
			},
			Spec: corev1api.PodSpec{
				Volumes: []corev1api.Volume{
					{
						Name: "csi-vol1",
						VolumeSource: corev1api.VolumeSource{
							EmptyDir: &corev1api.EmptyDirVolumeSource{},
						},
					},
				},
			},
		},
		&corev1api.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "awesome-pod-1",
				Namespace: "awesome-ns",
			},
			Spec: corev1api.PodSpec{
				Volumes: []corev1api.Volume{
					{
						Name: "csi-vol1",
						VolumeSource: corev1api.VolumeSource{
							PersistentVolumeClaim: &corev1api.PersistentVolumeClaimVolumeSource{
								ClaimName: "awesome-csi-pvc1",
							},
						},
					},
				},
			},
		},
		&corev1api.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "awesome-pod-2",
				Namespace: "awesome-ns",
			},
			Spec: corev1api.PodSpec{
				Volumes: []corev1api.Volume{
					{
						Name: "csi-vol1",
						VolumeSource: corev1api.VolumeSource{
							PersistentVolumeClaim: &corev1api.PersistentVolumeClaimVolumeSource{
								ClaimName: "awesome-csi-pvc1",
							},
						},
					},
				},
			},
		},
		&corev1api.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "pod1",
				Namespace: "uploader-ns",
				Annotations: map[string]string{
					"backup.velero.io/backup-volumes": "csi-vol1",
				},
			},
			Spec: corev1api.PodSpec{
				Volumes: []corev1api.Volume{
					{
						Name: "csi-vol1",
						VolumeSource: corev1api.VolumeSource{
							PersistentVolumeClaim: &corev1api.PersistentVolumeClaimVolumeSource{
								ClaimName: "csi-pvc1",
							},
						},
					},
				},
			},
		},
		&corev1api.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "pod2",
				Namespace: "uploader-ns",
				Annotations: map[string]string{
					"backup.velero.io/backup-volumes": "csi-vol1",
				},
			},
			Spec: corev1api.PodSpec{
				Volumes: []corev1api.Volume{
					{
						Name: "csi-vol1",
						VolumeSource: corev1api.VolumeSource{
							PersistentVolumeClaim: &corev1api.PersistentVolumeClaimVolumeSource{
								ClaimName: "csi-pvc1",
							},
						},
					},
				},
			},
		},
	}
	fakeClient := velerotest.NewFakeControllerRuntimeClient(t, objs...)

	testCases := []struct {
		name                     string
		inPVCNamespace           string
		inPVCName                string
		expectedIsFSUploaderUsed bool
		defaultVolumesToFSBackup bool
	}{
		{
			name:                     "2 pods using PVC, 1 pod using uploader",
			inPVCNamespace:           "default",
			inPVCName:                "csi-pvc1",
			expectedIsFSUploaderUsed: true,
			defaultVolumesToFSBackup: false,
		},
		{
			name:                     "2 pods using PVC, 2 pods using uploader",
			inPVCNamespace:           "uploader-ns",
			inPVCName:                "csi-pvc1",
			expectedIsFSUploaderUsed: true,
			defaultVolumesToFSBackup: false,
		},
		{
			name:                     "2 pods using PVC, 0 pods using uploader",
			inPVCNamespace:           "awesome-ns",
			inPVCName:                "awesome-csi-pvc1",
			expectedIsFSUploaderUsed: false,
			defaultVolumesToFSBackup: false,
		},
		{
			name:                     "0 pods using PVC",
			inPVCNamespace:           "default",
			inPVCName:                "does-not-exist",
			expectedIsFSUploaderUsed: false,
			defaultVolumesToFSBackup: false,
		},
		{
			name:                     "2 pods using PVC, using uploader by default",
			inPVCNamespace:           "awesome-ns",
			inPVCName:                "awesome-csi-pvc1",
			expectedIsFSUploaderUsed: true,
			defaultVolumesToFSBackup: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actualIsFSUploaderUsed, _ := IsPVCDefaultToFSBackup(tc.inPVCNamespace, tc.inPVCName, fakeClient, tc.defaultVolumesToFSBackup)
			assert.Equal(t, tc.expectedIsFSUploaderUsed, actualIsFSUploaderUsed)
		})
	}
}

func TestGetPodVolumeNameForPVC(t *testing.T) {
	testCases := []struct {
		name               string
		pod                corev1api.Pod
		pvcName            string
		expectError        bool
		expectedVolumeName string
	}{
		{
			name: "should get volume name for pod with multiple PVCs",
			pod: corev1api.Pod{
				Spec: corev1api.PodSpec{
					Volumes: []corev1api.Volume{
						{
							Name: "csi-vol1",
							VolumeSource: corev1api.VolumeSource{
								PersistentVolumeClaim: &corev1api.PersistentVolumeClaimVolumeSource{
									ClaimName: "csi-pvc1",
								},
							},
						},
						{
							Name: "csi-vol2",
							VolumeSource: corev1api.VolumeSource{
								PersistentVolumeClaim: &corev1api.PersistentVolumeClaimVolumeSource{
									ClaimName: "csi-pvc2",
								},
							},
						},
						{
							Name: "csi-vol3",
							VolumeSource: corev1api.VolumeSource{
								PersistentVolumeClaim: &corev1api.PersistentVolumeClaimVolumeSource{
									ClaimName: "csi-pvc3",
								},
							},
						},
					},
				},
			},
			pvcName:            "csi-pvc2",
			expectedVolumeName: "csi-vol2",
			expectError:        false,
		},
		{
			name: "should get volume name from pod using exactly one PVC",
			pod: corev1api.Pod{
				Spec: corev1api.PodSpec{
					Volumes: []corev1api.Volume{
						{
							Name: "csi-vol1",
							VolumeSource: corev1api.VolumeSource{
								PersistentVolumeClaim: &corev1api.PersistentVolumeClaimVolumeSource{
									ClaimName: "csi-pvc1",
								},
							},
						},
					},
				},
			},
			pvcName:            "csi-pvc1",
			expectedVolumeName: "csi-vol1",
			expectError:        false,
		},
		{
			name: "should return error for pod with no PVCs",
			pod: corev1api.Pod{
				Spec: corev1api.PodSpec{},
			},
			pvcName:     "csi-pvc2",
			expectError: true,
		},
		{
			name: "should return error for pod with no matching PVC",
			pod: corev1api.Pod{
				Spec: corev1api.PodSpec{
					Volumes: []corev1api.Volume{
						{
							Name: "csi-vol1",
							VolumeSource: corev1api.VolumeSource{
								PersistentVolumeClaim: &corev1api.PersistentVolumeClaimVolumeSource{
									ClaimName: "csi-pvc1",
								},
							},
						},
					},
				},
			},
			pvcName:     "mismatch-pvc",
			expectError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actualVolumeName, err := getPodVolumeNameForPVC(tc.pod, tc.pvcName)
			if tc.expectError && err == nil {
				assert.Error(t, err, "Want error; Got nil error")
				return
			}
			assert.Equalf(t, tc.expectedVolumeName, actualVolumeName, "unexpected podVolumename returned. Want %s; Got %s", tc.expectedVolumeName, actualVolumeName)
		})
	}
}

func TestGetPodsUsingPVC(t *testing.T) {
	objs := []runtime.Object{
		&corev1api.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "pod1",
				Namespace: "default",
			},
			Spec: corev1api.PodSpec{
				Volumes: []corev1api.Volume{
					{
						Name: "csi-vol1",
						VolumeSource: corev1api.VolumeSource{
							PersistentVolumeClaim: &corev1api.PersistentVolumeClaimVolumeSource{
								ClaimName: "csi-pvc1",
							},
						},
					},
				},
			},
		},
		&corev1api.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "pod2",
				Namespace: "default",
			},
			Spec: corev1api.PodSpec{
				Volumes: []corev1api.Volume{
					{
						Name: "csi-vol1",
						VolumeSource: corev1api.VolumeSource{
							PersistentVolumeClaim: &corev1api.PersistentVolumeClaimVolumeSource{
								ClaimName: "csi-pvc1",
							},
						},
					},
				},
			},
		},
		&corev1api.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "pod3",
				Namespace: "default",
			},
			Spec: corev1api.PodSpec{
				Volumes: []corev1api.Volume{
					{
						Name: "csi-vol1",
						VolumeSource: corev1api.VolumeSource{
							EmptyDir: &corev1api.EmptyDirVolumeSource{},
						},
					},
				},
			},
		},
		&corev1api.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "pod1",
				Namespace: "awesome-ns",
			},
			Spec: corev1api.PodSpec{
				Volumes: []corev1api.Volume{
					{
						Name: "csi-vol1",
						VolumeSource: corev1api.VolumeSource{
							PersistentVolumeClaim: &corev1api.PersistentVolumeClaimVolumeSource{
								ClaimName: "csi-pvc1",
							},
						},
					},
				},
			},
		},
	}
	fakeClient := velerotest.NewFakeControllerRuntimeClient(t, objs...)

	testCases := []struct {
		name             string
		pvcNamespace     string
		pvcName          string
		expectedPodCount int
	}{
		{
			name:             "should find exactly 2 pods using the PVC",
			pvcNamespace:     "default",
			pvcName:          "csi-pvc1",
			expectedPodCount: 2,
		},
		{
			name:             "should find exactly 1 pod using the PVC",
			pvcNamespace:     "awesome-ns",
			pvcName:          "csi-pvc1",
			expectedPodCount: 1,
		},
		{
			name:             "should find 0 pods using the PVC",
			pvcNamespace:     "default",
			pvcName:          "unused-pvc",
			expectedPodCount: 0,
		},
		{
			name:             "should find 0 pods in non-existent namespace",
			pvcNamespace:     "does-not-exist",
			pvcName:          "csi-pvc1",
			expectedPodCount: 0,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actualPods, err := GetPodsUsingPVC(tc.pvcNamespace, tc.pvcName, fakeClient)
			require.NoErrorf(t, err, "Want error=nil; Got error=%v", err)
			assert.Lenf(t, actualPods, tc.expectedPodCount, "unexpected number of pods in result; Want: %d; Got: %d", tc.expectedPodCount, len(actualPods))
		})
	}
}

func TestGetVolumesToProcess(t *testing.T) {
	testCases := []struct {
		name                          string
		volumes                       []corev1api.Volume
		volsToProcessByLegacyApproach []string
		expectedVolumes               []corev1api.Volume
	}{
		{
			name: "pod has 2 volumes empty volsToProcessByLegacyApproach list return 2 volumes",
			volumes: []corev1api.Volume{
				{
					Name: "sample-volume-1",
					VolumeSource: corev1api.VolumeSource{
						PersistentVolumeClaim: &corev1api.PersistentVolumeClaimVolumeSource{
							ClaimName: "sample-pvc-1",
						},
					},
				},
				{
					Name: "sample-volume-2",
					VolumeSource: corev1api.VolumeSource{
						PersistentVolumeClaim: &corev1api.PersistentVolumeClaimVolumeSource{
							ClaimName: "sample-pvc-2",
						},
					},
				},
			},
			volsToProcessByLegacyApproach: []string{},
			expectedVolumes: []corev1api.Volume{
				{
					Name: "sample-volume-1",
					VolumeSource: corev1api.VolumeSource{
						PersistentVolumeClaim: &corev1api.PersistentVolumeClaimVolumeSource{
							ClaimName: "sample-pvc-1",
						},
					},
				},
				{
					Name: "sample-volume-2",
					VolumeSource: corev1api.VolumeSource{
						PersistentVolumeClaim: &corev1api.PersistentVolumeClaimVolumeSource{
							ClaimName: "sample-pvc-2",
						},
					},
				},
			},
		},
		{
			name: "pod has 2 volumes non-empty volsToProcessByLegacyApproach list returns 1 volumes",
			volumes: []corev1api.Volume{
				{
					Name: "sample-volume-1",
					VolumeSource: corev1api.VolumeSource{
						PersistentVolumeClaim: &corev1api.PersistentVolumeClaimVolumeSource{
							ClaimName: "sample-pvc-1",
						},
					},
				},
				{
					Name: "sample-volume-2",
					VolumeSource: corev1api.VolumeSource{
						PersistentVolumeClaim: &corev1api.PersistentVolumeClaimVolumeSource{
							ClaimName: "sample-pvc-2",
						},
					},
				},
			},
			volsToProcessByLegacyApproach: []string{"sample-volume-2"},
			expectedVolumes: []corev1api.Volume{
				{
					Name: "sample-volume-2",
					VolumeSource: corev1api.VolumeSource{
						PersistentVolumeClaim: &corev1api.PersistentVolumeClaimVolumeSource{
							ClaimName: "sample-pvc-2",
						},
					},
				},
			},
		},
		{
			name:                          "empty case, return empty list",
			volumes:                       []corev1api.Volume{},
			volsToProcessByLegacyApproach: []string{},
			expectedVolumes:               []corev1api.Volume{},
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actualVolumes := GetVolumesToProcess(tc.volumes, tc.volsToProcessByLegacyApproach)
			assert.Equal(t, tc.expectedVolumes, actualVolumes, "Want Volumes List %v; Got Volumes List %v", tc.expectedVolumes, actualVolumes)
		})
	}
}
