/*
Copyright The Velero Contributors.

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

package volume

import (
	"context"
	"testing"

	snapshotv1api "github.com/kubernetes-csi/external-snapshotter/client/v4/apis/volumesnapshot/v1"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
	corev1api "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"

	velerov1api "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	velerov2alpha1 "github.com/vmware-tanzu/velero/pkg/apis/velero/v2alpha1"
	"github.com/vmware-tanzu/velero/pkg/builder"
	"github.com/vmware-tanzu/velero/pkg/itemoperation"
	"github.com/vmware-tanzu/velero/pkg/plugin/velero"
	velerotest "github.com/vmware-tanzu/velero/pkg/test"
	"github.com/vmware-tanzu/velero/pkg/util/logging"
	"github.com/vmware-tanzu/velero/pkg/volume"
)

func TestGenerateVolumeInfoForSkippedPV(t *testing.T) {
	tests := []struct {
		name                string
		skippedPVName       string
		pvMap               map[string]pvcPvInfo
		expectedVolumeInfos []*VolumeInfo
	}{
		{
			name:          "Cannot find info for PV",
			skippedPVName: "testPV",
			pvMap: map[string]pvcPvInfo{
				"velero/testPVC": {
					PVCName:      "testPVC",
					PVCNamespace: "velero",
					PV: corev1api.PersistentVolume{
						ObjectMeta: metav1.ObjectMeta{
							Name:   "testPV",
							Labels: map[string]string{"a": "b"},
						},
						Spec: corev1api.PersistentVolumeSpec{
							PersistentVolumeReclaimPolicy: corev1api.PersistentVolumeReclaimDelete,
						},
					},
				},
			},
			expectedVolumeInfos: []*VolumeInfo{},
		},
		{
			name:          "Normal Skipped PV info",
			skippedPVName: "testPV",
			pvMap: map[string]pvcPvInfo{
				"velero/testPVC": {
					PVCName:      "testPVC",
					PVCNamespace: "velero",
					PV: corev1api.PersistentVolume{
						ObjectMeta: metav1.ObjectMeta{
							Name:   "testPV",
							Labels: map[string]string{"a": "b"},
						},
						Spec: corev1api.PersistentVolumeSpec{
							PersistentVolumeReclaimPolicy: corev1api.PersistentVolumeReclaimDelete,
						},
					},
				},
				"testPV": {
					PVCName:      "testPVC",
					PVCNamespace: "velero",
					PV: corev1api.PersistentVolume{
						ObjectMeta: metav1.ObjectMeta{
							Name:   "testPV",
							Labels: map[string]string{"a": "b"},
						},
						Spec: corev1api.PersistentVolumeSpec{
							PersistentVolumeReclaimPolicy: corev1api.PersistentVolumeReclaimDelete,
						},
					},
				},
			},
			expectedVolumeInfos: []*VolumeInfo{
				{
					PVCName:       "testPVC",
					PVCNamespace:  "velero",
					PVName:        "testPV",
					Skipped:       true,
					SkippedReason: "CSI: skipped for PodVolumeBackup",
					PVInfo: &PVInfo{
						ReclaimPolicy: "Delete",
						Labels: map[string]string{
							"a": "b",
						},
					},
				},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			volumesInfo := VolumesInformation{}
			volumesInfo.Init()

			if tc.skippedPVName != "" {
				volumesInfo.SkippedPVs = map[string]string{
					tc.skippedPVName: "CSI: skipped for PodVolumeBackup",
				}
			}

			if tc.pvMap != nil {
				for k, v := range tc.pvMap {
					volumesInfo.pvMap[k] = v
				}
			}
			volumesInfo.logger = logging.DefaultLogger(logrus.DebugLevel, logging.FormatJSON)

			volumesInfo.generateVolumeInfoForSkippedPV()
			require.Equal(t, tc.expectedVolumeInfos, volumesInfo.volumeInfos)
		})
	}
}

func TestGenerateVolumeInfoForVeleroNativeSnapshot(t *testing.T) {
	tests := []struct {
		name                string
		nativeSnapshot      volume.Snapshot
		pvMap               map[string]pvcPvInfo
		expectedVolumeInfos []*VolumeInfo
	}{
		{
			name: "Native snapshot's IPOS pointer is nil",
			nativeSnapshot: volume.Snapshot{
				Spec: volume.SnapshotSpec{
					PersistentVolumeName: "testPV",
					VolumeIOPS:           nil,
				},
			},
			expectedVolumeInfos: []*VolumeInfo{},
		},
		{
			name: "Cannot find info for the PV",
			nativeSnapshot: volume.Snapshot{
				Spec: volume.SnapshotSpec{
					PersistentVolumeName: "testPV",
					VolumeIOPS:           int64Ptr(100),
				},
			},
			expectedVolumeInfos: []*VolumeInfo{},
		},
		{
			name: "Cannot find PV info in pvMap",
			pvMap: map[string]pvcPvInfo{
				"velero/testPVC": {
					PVCName:      "testPVC",
					PVCNamespace: "velero",
					PV: corev1api.PersistentVolume{
						ObjectMeta: metav1.ObjectMeta{
							Name:   "testPV",
							Labels: map[string]string{"a": "b"},
						},
						Spec: corev1api.PersistentVolumeSpec{
							PersistentVolumeReclaimPolicy: corev1api.PersistentVolumeReclaimDelete,
						},
					},
				},
			},
			nativeSnapshot: volume.Snapshot{
				Spec: volume.SnapshotSpec{
					PersistentVolumeName: "testPV",
					VolumeIOPS:           int64Ptr(100),
					VolumeType:           "ssd",
					VolumeAZ:             "us-central1-a",
				},
				Status: volume.SnapshotStatus{
					ProviderSnapshotID: "pvc-b31e3386-4bbb-4937-95d-7934cd62-b0a1-494b-95d7-0687440e8d0c",
				},
			},
			expectedVolumeInfos: []*VolumeInfo{},
		},
		{
			name: "Normal native snapshot",
			pvMap: map[string]pvcPvInfo{
				"testPV": {
					PVCName:      "testPVC",
					PVCNamespace: "velero",
					PV: corev1api.PersistentVolume{
						ObjectMeta: metav1.ObjectMeta{
							Name:   "testPV",
							Labels: map[string]string{"a": "b"},
						},
						Spec: corev1api.PersistentVolumeSpec{
							PersistentVolumeReclaimPolicy: corev1api.PersistentVolumeReclaimDelete,
						},
					},
				},
			},
			nativeSnapshot: volume.Snapshot{
				Spec: volume.SnapshotSpec{
					PersistentVolumeName: "testPV",
					VolumeIOPS:           int64Ptr(100),
					VolumeType:           "ssd",
					VolumeAZ:             "us-central1-a",
				},
				Status: volume.SnapshotStatus{
					ProviderSnapshotID: "pvc-b31e3386-4bbb-4937-95d-7934cd62-b0a1-494b-95d7-0687440e8d0c",
				},
			},
			expectedVolumeInfos: []*VolumeInfo{
				{
					PVCName:      "testPVC",
					PVCNamespace: "velero",
					PVName:       "testPV",
					BackupMethod: NativeSnapshot,
					PVInfo: &PVInfo{
						ReclaimPolicy: "Delete",
						Labels: map[string]string{
							"a": "b",
						},
					},
					NativeSnapshotInfo: &NativeSnapshotInfo{
						SnapshotHandle: "pvc-b31e3386-4bbb-4937-95d-7934cd62-b0a1-494b-95d7-0687440e8d0c",
						VolumeType:     "ssd",
						VolumeAZ:       "us-central1-a",
						IOPS:           "100",
					},
				},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			volumesInfo := VolumesInformation{}
			volumesInfo.Init()
			volumesInfo.NativeSnapshots = append(volumesInfo.NativeSnapshots, &tc.nativeSnapshot)
			if tc.pvMap != nil {
				for k, v := range tc.pvMap {
					volumesInfo.pvMap[k] = v
				}
			}
			volumesInfo.logger = logging.DefaultLogger(logrus.DebugLevel, logging.FormatJSON)

			volumesInfo.generateVolumeInfoForVeleroNativeSnapshot()
			require.Equal(t, tc.expectedVolumeInfos, volumesInfo.volumeInfos)
		})
	}
}

func TestGenerateVolumeInfoForCSIVolumeSnapshot(t *testing.T) {
	resourceQuantity := resource.MustParse("100Gi")
	now := metav1.Now()
	tests := []struct {
		name                  string
		volumeSnapshot        snapshotv1api.VolumeSnapshot
		volumeSnapshotContent snapshotv1api.VolumeSnapshotContent
		volumeSnapshotClass   snapshotv1api.VolumeSnapshotClass
		pvMap                 map[string]pvcPvInfo
		operation             *itemoperation.BackupOperation
		expectedVolumeInfos   []*VolumeInfo
	}{
		{
			name: "VS doesn't have VolumeSnapshotClass name",
			volumeSnapshot: snapshotv1api.VolumeSnapshot{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "testVS",
					Namespace: "velero",
				},
				Spec: snapshotv1api.VolumeSnapshotSpec{},
			},
			expectedVolumeInfos: []*VolumeInfo{},
		},
		{
			name: "VS doesn't have status",
			volumeSnapshot: snapshotv1api.VolumeSnapshot{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "testVS",
					Namespace: "velero",
				},
				Spec: snapshotv1api.VolumeSnapshotSpec{
					VolumeSnapshotClassName: stringPtr("testClass"),
				},
			},
			expectedVolumeInfos: []*VolumeInfo{},
		},
		{
			name: "VS doesn't have PVC",
			volumeSnapshot: snapshotv1api.VolumeSnapshot{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "testVS",
					Namespace: "velero",
				},
				Spec: snapshotv1api.VolumeSnapshotSpec{
					VolumeSnapshotClassName: stringPtr("testClass"),
				},
				Status: &snapshotv1api.VolumeSnapshotStatus{
					BoundVolumeSnapshotContentName: stringPtr("testContent"),
				},
			},
			expectedVolumeInfos: []*VolumeInfo{},
		},
		{
			name: "Cannot find VSC for VS",
			volumeSnapshot: snapshotv1api.VolumeSnapshot{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "testVS",
					Namespace: "velero",
				},
				Spec: snapshotv1api.VolumeSnapshotSpec{
					VolumeSnapshotClassName: stringPtr("testClass"),
					Source: snapshotv1api.VolumeSnapshotSource{
						PersistentVolumeClaimName: stringPtr("testPVC"),
					},
				},
				Status: &snapshotv1api.VolumeSnapshotStatus{
					BoundVolumeSnapshotContentName: stringPtr("testContent"),
				},
			},
			expectedVolumeInfos: []*VolumeInfo{},
		},
		{
			name: "Cannot find VolumeInfo for PVC",
			volumeSnapshot: snapshotv1api.VolumeSnapshot{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "testVS",
					Namespace: "velero",
				},
				Spec: snapshotv1api.VolumeSnapshotSpec{
					VolumeSnapshotClassName: stringPtr("testClass"),
					Source: snapshotv1api.VolumeSnapshotSource{
						PersistentVolumeClaimName: stringPtr("testPVC"),
					},
				},
				Status: &snapshotv1api.VolumeSnapshotStatus{
					BoundVolumeSnapshotContentName: stringPtr("testContent"),
				},
			},
			volumeSnapshotClass:   *builder.ForVolumeSnapshotClass("testClass").Driver("pd.csi.storage.gke.io").Result(),
			volumeSnapshotContent: *builder.ForVolumeSnapshotContent("testContent").Status(&snapshotv1api.VolumeSnapshotContentStatus{SnapshotHandle: stringPtr("testSnapshotHandle")}).Result(),
			expectedVolumeInfos:   []*VolumeInfo{},
		},
		{
			name: "Normal VolumeSnapshot case",
			volumeSnapshot: snapshotv1api.VolumeSnapshot{
				ObjectMeta: metav1.ObjectMeta{
					Name:              "testVS",
					Namespace:         "velero",
					CreationTimestamp: now,
				},
				Spec: snapshotv1api.VolumeSnapshotSpec{
					VolumeSnapshotClassName: stringPtr("testClass"),
					Source: snapshotv1api.VolumeSnapshotSource{
						PersistentVolumeClaimName: stringPtr("testPVC"),
					},
				},
				Status: &snapshotv1api.VolumeSnapshotStatus{
					BoundVolumeSnapshotContentName: stringPtr("testContent"),
					RestoreSize:                    &resourceQuantity,
				},
			},
			volumeSnapshotClass:   *builder.ForVolumeSnapshotClass("testClass").Driver("pd.csi.storage.gke.io").Result(),
			volumeSnapshotContent: *builder.ForVolumeSnapshotContent("testContent").Status(&snapshotv1api.VolumeSnapshotContentStatus{SnapshotHandle: stringPtr("testSnapshotHandle")}).Result(),
			pvMap: map[string]pvcPvInfo{
				"testPV": {
					PVCName:      "testPVC",
					PVCNamespace: "velero",
					PV: corev1api.PersistentVolume{
						ObjectMeta: metav1.ObjectMeta{
							Name:   "testPV",
							Labels: map[string]string{"a": "b"},
						},
						Spec: corev1api.PersistentVolumeSpec{
							PersistentVolumeReclaimPolicy: corev1api.PersistentVolumeReclaimDelete,
						},
					},
				},
			},
			operation: &itemoperation.BackupOperation{
				Spec: itemoperation.BackupOperationSpec{
					OperationID: "testID",
					ResourceIdentifier: velero.ResourceIdentifier{
						GroupResource: schema.GroupResource{
							Group:    "snapshot.storage.k8s.io",
							Resource: "volumesnapshots",
						},
						Namespace: "velero",
						Name:      "testVS",
					},
				},
			},
			expectedVolumeInfos: []*VolumeInfo{
				{
					PVCName:               "testPVC",
					PVCNamespace:          "velero",
					PVName:                "testPV",
					BackupMethod:          CSISnapshot,
					StartTimestamp:        &now,
					PreserveLocalSnapshot: true,
					CSISnapshotInfo: &CSISnapshotInfo{
						Driver:         "pd.csi.storage.gke.io",
						SnapshotHandle: "testSnapshotHandle",
						Size:           107374182400,
						VSCName:        "testContent",
						OperationID:    "testID",
					},
					PVInfo: &PVInfo{
						ReclaimPolicy: "Delete",
						Labels: map[string]string{
							"a": "b",
						},
					},
				},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			volumesInfo := VolumesInformation{}
			volumesInfo.Init()

			if tc.pvMap != nil {
				for k, v := range tc.pvMap {
					volumesInfo.pvMap[k] = v
				}
			}

			if tc.operation != nil {
				volumesInfo.BackupOperations = append(volumesInfo.BackupOperations, tc.operation)
			}

			volumesInfo.volumeSnapshots = []snapshotv1api.VolumeSnapshot{tc.volumeSnapshot}
			volumesInfo.volumeSnapshotContents = []snapshotv1api.VolumeSnapshotContent{tc.volumeSnapshotContent}
			volumesInfo.volumeSnapshotClasses = []snapshotv1api.VolumeSnapshotClass{tc.volumeSnapshotClass}
			volumesInfo.logger = logging.DefaultLogger(logrus.DebugLevel, logging.FormatJSON)

			volumesInfo.generateVolumeInfoForCSIVolumeSnapshot()
			require.Equal(t, tc.expectedVolumeInfos, volumesInfo.volumeInfos)
		})
	}
}

func TestGenerateVolumeInfoFromPVB(t *testing.T) {
	tests := []struct {
		name                string
		pvb                 *velerov1api.PodVolumeBackup
		pod                 *corev1api.Pod
		pvMap               map[string]pvcPvInfo
		expectedVolumeInfos []*VolumeInfo
	}{
		{
			name:                "cannot find PVB's pod, should fail",
			pvb:                 builder.ForPodVolumeBackup("velero", "testPVB").PodName("testPod").PodNamespace("velero").Result(),
			expectedVolumeInfos: []*VolumeInfo{},
		},
		{
			name: "PVB doesn't have a related PVC",
			pvb:  builder.ForPodVolumeBackup("velero", "testPVB").PodName("testPod").PodNamespace("velero").Result(),
			pod: builder.ForPod("velero", "testPod").Containers(&corev1api.Container{
				Name: "test",
				VolumeMounts: []corev1api.VolumeMount{
					{
						Name:      "testVolume",
						MountPath: "/data",
					},
				},
			}).Volumes(
				&corev1api.Volume{
					Name: "",
					VolumeSource: corev1api.VolumeSource{
						HostPath: &corev1api.HostPathVolumeSource{},
					},
				},
			).Result(),
			expectedVolumeInfos: []*VolumeInfo{
				{
					PVCName:      "",
					PVCNamespace: "",
					PVName:       "",
					BackupMethod: PodVolumeBackup,
					PVBInfo: &PodVolumeBackupInfo{
						PodName:      "testPod",
						PodNamespace: "velero",
					},
				},
			},
		},
		{
			name: "Backup doesn't have information for PVC",
			pvb:  builder.ForPodVolumeBackup("velero", "testPVB").PodName("testPod").PodNamespace("velero").Result(),
			pod: builder.ForPod("velero", "testPod").Containers(&corev1api.Container{
				Name: "test",
				VolumeMounts: []corev1api.VolumeMount{
					{
						Name:      "testVolume",
						MountPath: "/data",
					},
				},
			}).Volumes(
				&corev1api.Volume{
					Name: "",
					VolumeSource: corev1api.VolumeSource{
						PersistentVolumeClaim: &corev1api.PersistentVolumeClaimVolumeSource{
							ClaimName: "testPVC",
						},
					},
				},
			).Result(),
			expectedVolumeInfos: []*VolumeInfo{},
		},
		{
			name: "PVB's volume has a PVC",
			pvMap: map[string]pvcPvInfo{
				"testPV": {
					PVCName:      "testPVC",
					PVCNamespace: "velero",
					PV: corev1api.PersistentVolume{
						ObjectMeta: metav1.ObjectMeta{
							Name:   "testPV",
							Labels: map[string]string{"a": "b"},
						},
						Spec: corev1api.PersistentVolumeSpec{
							PersistentVolumeReclaimPolicy: corev1api.PersistentVolumeReclaimDelete,
						},
					},
				},
			},
			pvb: builder.ForPodVolumeBackup("velero", "testPVB").PodName("testPod").PodNamespace("velero").Result(),
			pod: builder.ForPod("velero", "testPod").Containers(&corev1api.Container{
				Name: "test",
				VolumeMounts: []corev1api.VolumeMount{
					{
						Name:      "testVolume",
						MountPath: "/data",
					},
				},
			}).Volumes(
				&corev1api.Volume{
					Name: "",
					VolumeSource: corev1api.VolumeSource{
						PersistentVolumeClaim: &corev1api.PersistentVolumeClaimVolumeSource{
							ClaimName: "testPVC",
						},
					},
				},
			).Result(),
			expectedVolumeInfos: []*VolumeInfo{
				{
					PVCName:      "testPVC",
					PVCNamespace: "velero",
					PVName:       "testPV",
					BackupMethod: PodVolumeBackup,
					PVBInfo: &PodVolumeBackupInfo{
						PodName:      "testPod",
						PodNamespace: "velero",
					},
					PVInfo: &PVInfo{
						ReclaimPolicy: string(corev1api.PersistentVolumeReclaimDelete),
						Labels:        map[string]string{"a": "b"},
					},
				},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			volumesInfo := VolumesInformation{}
			volumesInfo.Init()
			volumesInfo.crClient = velerotest.NewFakeControllerRuntimeClient(t)

			volumesInfo.PodVolumeBackups = append(volumesInfo.PodVolumeBackups, tc.pvb)

			if tc.pvMap != nil {
				for k, v := range tc.pvMap {
					volumesInfo.pvMap[k] = v
				}
			}
			if tc.pod != nil {
				require.NoError(t, volumesInfo.crClient.Create(context.TODO(), tc.pod))
			}
			volumesInfo.logger = logging.DefaultLogger(logrus.DebugLevel, logging.FormatJSON)

			volumesInfo.generateVolumeInfoFromPVB()
			require.Equal(t, tc.expectedVolumeInfos, volumesInfo.volumeInfos)
		})
	}
}

func TestGenerateVolumeInfoFromDataUpload(t *testing.T) {
	now := metav1.Now()
	tests := []struct {
		name                string
		volumeSnapshotClass *snapshotv1api.VolumeSnapshotClass
		dataUpload          *velerov2alpha1.DataUpload
		operation           *itemoperation.BackupOperation
		pvMap               map[string]pvcPvInfo
		expectedVolumeInfos []*VolumeInfo
	}{
		{
			name: "Operation is not for PVC",
			operation: &itemoperation.BackupOperation{
				Spec: itemoperation.BackupOperationSpec{
					ResourceIdentifier: velero.ResourceIdentifier{
						GroupResource: schema.GroupResource{
							Group:    "",
							Resource: "configmaps",
						},
					},
				},
			},
			expectedVolumeInfos: []*VolumeInfo{},
		},
		{
			name: "Operation doesn't have DataUpload PostItemOperation",
			operation: &itemoperation.BackupOperation{
				Spec: itemoperation.BackupOperationSpec{
					ResourceIdentifier: velero.ResourceIdentifier{
						GroupResource: schema.GroupResource{
							Group:    "",
							Resource: "persistentvolumeclaims",
						},
						Namespace: "velero",
						Name:      "testPVC",
					},
					PostOperationItems: []velero.ResourceIdentifier{
						{
							GroupResource: schema.GroupResource{
								Group:    "",
								Resource: "configmaps",
							},
						},
					},
				},
			},
			expectedVolumeInfos: []*VolumeInfo{},
		},
		{
			name: "DataUpload cannot be found for operation",
			operation: &itemoperation.BackupOperation{
				Spec: itemoperation.BackupOperationSpec{
					OperationID: "testOperation",
					ResourceIdentifier: velero.ResourceIdentifier{
						GroupResource: schema.GroupResource{
							Group:    "",
							Resource: "persistentvolumeclaims",
						},
						Namespace: "velero",
						Name:      "testPVC",
					},
					PostOperationItems: []velero.ResourceIdentifier{
						{
							GroupResource: schema.GroupResource{
								Group:    "velero.io",
								Resource: "datauploads",
							},
							Namespace: "velero",
							Name:      "testDU",
						},
					},
				},
			},
			expectedVolumeInfos: []*VolumeInfo{},
		},
		{
			name: "VolumeSnapshotClass cannot be found for operation",
			dataUpload: builder.ForDataUpload("velero", "testDU").DataMover("velero").CSISnapshot(&velerov2alpha1.CSISnapshotSpec{
				VolumeSnapshot: "testVS",
			}).SnapshotID("testSnapshotHandle").Result(),
			operation: &itemoperation.BackupOperation{
				Spec: itemoperation.BackupOperationSpec{
					OperationID: "testOperation",
					ResourceIdentifier: velero.ResourceIdentifier{
						GroupResource: schema.GroupResource{
							Group:    "",
							Resource: "persistentvolumeclaims",
						},
						Namespace: "velero",
						Name:      "testPVC",
					},
					PostOperationItems: []velero.ResourceIdentifier{
						{
							GroupResource: schema.GroupResource{
								Group:    "velero.io",
								Resource: "datauploads",
							},
							Namespace: "velero",
							Name:      "testDU",
						},
					},
				},
			},
			pvMap: map[string]pvcPvInfo{
				"testPV": {
					PVCName:      "testPVC",
					PVCNamespace: "velero",
					PV: corev1api.PersistentVolume{
						ObjectMeta: metav1.ObjectMeta{
							Name:   "testPV",
							Labels: map[string]string{"a": "b"},
						},
						Spec: corev1api.PersistentVolumeSpec{
							PersistentVolumeReclaimPolicy: corev1api.PersistentVolumeReclaimDelete,
						},
					},
				},
			},
			expectedVolumeInfos: []*VolumeInfo{
				{
					PVCName:           "testPVC",
					PVCNamespace:      "velero",
					PVName:            "testPV",
					BackupMethod:      CSISnapshot,
					SnapshotDataMoved: true,
					CSISnapshotInfo: &CSISnapshotInfo{
						SnapshotHandle: FieldValueIsUnknown,
						VSCName:        FieldValueIsUnknown,
						OperationID:    FieldValueIsUnknown,
						Size:           0,
					},
					SnapshotDataMovementInfo: &SnapshotDataMovementInfo{
						DataMover:    "velero",
						UploaderType: "kopia",
						OperationID:  "testOperation",
					},
					PVInfo: &PVInfo{
						ReclaimPolicy: string(corev1api.PersistentVolumeReclaimDelete),
						Labels:        map[string]string{"a": "b"},
					},
				},
			},
		},
		{
			name: "Normal DataUpload case",
			dataUpload: builder.ForDataUpload("velero", "testDU").DataMover("velero").CSISnapshot(&velerov2alpha1.CSISnapshotSpec{
				VolumeSnapshot: "testVS",
				SnapshotClass:  "testClass",
			}).SnapshotID("testSnapshotHandle").Result(),
			volumeSnapshotClass: builder.ForVolumeSnapshotClass("testClass").Driver("pd.csi.storage.gke.io").Result(),
			operation: &itemoperation.BackupOperation{
				Spec: itemoperation.BackupOperationSpec{
					OperationID: "testOperation",
					ResourceIdentifier: velero.ResourceIdentifier{
						GroupResource: schema.GroupResource{
							Group:    "",
							Resource: "persistentvolumeclaims",
						},
						Namespace: "velero",
						Name:      "testPVC",
					},
					PostOperationItems: []velero.ResourceIdentifier{
						{
							GroupResource: schema.GroupResource{
								Group:    "velero.io",
								Resource: "datauploads",
							},
							Namespace: "velero",
							Name:      "testDU",
						},
					},
				},
				Status: itemoperation.OperationStatus{
					Created: &now,
				},
			},
			pvMap: map[string]pvcPvInfo{
				"testPV": {
					PVCName:      "testPVC",
					PVCNamespace: "velero",
					PV: corev1api.PersistentVolume{
						ObjectMeta: metav1.ObjectMeta{
							Name:   "testPV",
							Labels: map[string]string{"a": "b"},
						},
						Spec: corev1api.PersistentVolumeSpec{
							PersistentVolumeReclaimPolicy: corev1api.PersistentVolumeReclaimDelete,
						},
					},
				},
			},
			expectedVolumeInfos: []*VolumeInfo{
				{
					PVCName:           "testPVC",
					PVCNamespace:      "velero",
					PVName:            "testPV",
					BackupMethod:      CSISnapshot,
					SnapshotDataMoved: true,
					StartTimestamp:    &now,
					CSISnapshotInfo: &CSISnapshotInfo{
						VSCName:        FieldValueIsUnknown,
						SnapshotHandle: FieldValueIsUnknown,
						OperationID:    FieldValueIsUnknown,
						Size:           0,
						Driver:         "pd.csi.storage.gke.io",
					},
					SnapshotDataMovementInfo: &SnapshotDataMovementInfo{
						DataMover:    "velero",
						UploaderType: "kopia",
						OperationID:  "testOperation",
					},
					PVInfo: &PVInfo{
						ReclaimPolicy: string(corev1api.PersistentVolumeReclaimDelete),
						Labels:        map[string]string{"a": "b"},
					},
				},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			volumesInfo := VolumesInformation{}
			volumesInfo.Init()

			if tc.operation != nil {
				volumesInfo.BackupOperations = append(volumesInfo.BackupOperations, tc.operation)
			}

			if tc.pvMap != nil {
				for k, v := range tc.pvMap {
					volumesInfo.pvMap[k] = v
				}
			}

			volumesInfo.crClient = velerotest.NewFakeControllerRuntimeClient(t)
			if tc.dataUpload != nil {
				volumesInfo.crClient.Create(context.TODO(), tc.dataUpload)
			}

			if tc.volumeSnapshotClass != nil {
				volumesInfo.crClient.Create(context.TODO(), tc.volumeSnapshotClass)
			}

			volumesInfo.logger = logging.DefaultLogger(logrus.DebugLevel, logging.FormatJSON)

			volumesInfo.generateVolumeInfoFromDataUpload()
			require.Equal(t, tc.expectedVolumeInfos, volumesInfo.volumeInfos)
		})
	}
}

func stringPtr(str string) *string {
	return &str
}

func int64Ptr(val int) *int64 {
	i := int64(val)
	return &i
}
