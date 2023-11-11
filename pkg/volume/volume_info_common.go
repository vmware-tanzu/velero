/*
Copyright 2018 the Velero contributors.

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

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

type VolumeBackupMethod string

const (
	NativeSnapshot  VolumeBackupMethod = "NativeSnapshot"
	PodVolumeBackup VolumeBackupMethod = "PodVolumeBackup"
	CSISnapshot     VolumeBackupMethod = "CSISnapshot"
)

type VolumeInfoVersion struct {
	Version string `json:"version"`
}

type VolumeInfos struct {
	VolumeInfos []VolumeInfo `json:"volumeInfos"`
}

type VolumeInfo struct {
	// The PVC's name.
	PVCName string `json:"pvcName,omitempty"`

	// The PVC's namespace
	PVCNamespace string `json:"pvcNamespace,omitempty"`

	// The PV name.
	PVName string `json:"pvName,omitempty"`

	// The way the volume data is backed up. The valid value includes `VeleroNativeSnapshot`, `PodVolumeBackup` and `CSISnapshot`.
	BackupMethod VolumeBackupMethod `json:"backupMethod,omitempty"`

	// Whether the volume's snapshot data is moved to specified storage.
	SnapshotDataMoved bool `json:"snapshotDataMoved"`

	// Whether the local snapshot is preserved after snapshot is moved.
	// The local snapshot may be a result of CSI snapshot backup(no data movement)
	// or a CSI snapshot data movement plus preserve local snapshot.
	PreserveLocalSnapshot bool `json:"preserveLocalSnapshot"`

	// Whether the Volume is skipped in this backup.
	Skipped bool `json:"skipped"`

	// The reason for the volume is skipped in the backup.
	SkippedReason string `json:"skippedReason,omitempty"`

	// Snapshot starts timestamp.
	StartTimestamp *metav1.Time `json:"startTimestamp,omitempty"`

	// The Async Operation's ID.
	OperationID string `json:"operationID,omitempty"`

	CSISnapshotInfo          CSISnapshotInfo          `json:"csiSnapshotInfo,omitempty"`
	SnapshotDataMovementInfo SnapshotDataMovementInfo `json:"snapshotDataMovementInfo,omitempty"`
	NativeSnapshotInfo       NativeSnapshotInfo       `json:"nativeSnapshotInfo,omitempty"`
	PVBInfo                  PodVolumeBackupInfo      `json:"pvbInfo,omitempty"`
	PVInfo                   PVInfo                   `json:"pvInfo,omitempty"`
}

// CSISnapshotInfo is used for displaying the CSI snapshot status
type CSISnapshotInfo struct {
	// It's the storage provider's snapshot ID for CSI.
	SnapshotHandle string `json:"snapshotHandle"`

	// The snapshot corresponding volume size.
	Size int64 `json:"size"`

	// The name of the CSI driver.
	Driver string `json:"driver"`

	// The name of the VolumeSnapshotContent.
	VSCName string `json:"vscName"`
}

// SnapshotDataMovementInfo is used for displaying the snapshot data mover status.
type SnapshotDataMovementInfo struct {
	// The data mover used by the backup. The valid values are `velero` and ``(equals to `velero`).
	DataMover string `json:"dataMover"`

	// The type of the uploader that uploads the snapshot data. The valid values are `kopia` and `restic`.
	UploaderType string `json:"uploaderType"`

	// The name or ID of the snapshot associated object(SAO).
	// SAO is used to support local snapshots for the snapshot data mover,
	// e.g. it could be a VolumeSnapshot for CSI snapshot data movement.
	RetainedSnapshot string `json:"retainedSnapshot"`

	// It's the filesystem repository's snapshot ID.
	SnapshotHandle string `json:"snapshotHandle"`
}

// NativeSnapshotInfo is used for displaying the Velero native snapshot status.
// A Velero Native Snapshot is a cloud storage snapshot taken by the Velero native
// plugins, e.g. velero-plugin-for-aws, velero-plugin-for-gcp, and
// velero-plugin-for-microsoft-azure.
type NativeSnapshotInfo struct {
	// It's the storage provider's snapshot ID for the Velero-native snapshot.
	SnapshotHandle string `json:"snapshotHandle"`

	// The cloud provider snapshot volume type.
	VolumeType string `json:"volumeType"`

	// The cloud provider snapshot volume's availability zones.
	VolumeAZ string `json:"volumeAZ"`

	// The cloud provider snapshot volume's IOPS.
	IOPS string `json:"iops"`
}

// PodVolumeBackupInfo is used for displaying the PodVolumeBackup snapshot status.
type PodVolumeBackupInfo struct {
	// It's the file-system uploader's snapshot ID for PodVolumeBackup.
	SnapshotHandle string `json:"snapshotHandle"`

	// The snapshot corresponding volume size.
	Size int64 `json:"size"`

	// The type of the uploader that uploads the data. The valid values are `kopia` and `restic`.
	UploaderType string `json:"uploaderType"`

	// The PVC's corresponding volume name used by Pod
	// https://github.com/kubernetes/kubernetes/blob/e4b74dd12fa8cb63c174091d5536a10b8ec19d34/pkg/apis/core/types.go#L48
	VolumeName string `json:"volumeName"`

	// The Pod name mounting this PVC.
	PodName string `json:"podName"`

	// The Pod namespace
	PodNamespace string `json:"podNamespace"`

	// The PVB-taken k8s node's name.
	NodeName string `json:"nodeName"`
}

// PVInfo is used to store some PV information modified after creation.
// Those information are lost after PV recreation.
type PVInfo struct {
	// ReclaimPolicy of PV. It could be different from the referenced StorageClass.
	ReclaimPolicy string `json:"reclaimPolicy"`

	// The PV's labels should be kept after recreation.
	Labels map[string]string `json:"labels"`
}
