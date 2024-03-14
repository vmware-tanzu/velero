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
	"strconv"

	snapshotv1api "github.com/kubernetes-csi/external-snapshotter/client/v7/apis/volumesnapshot/v1"
	"github.com/sirupsen/logrus"
	corev1api "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kbclient "sigs.k8s.io/controller-runtime/pkg/client"

	velerov1api "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	velerov2alpha1 "github.com/vmware-tanzu/velero/pkg/apis/velero/v2alpha1"
	"github.com/vmware-tanzu/velero/pkg/features"
	"github.com/vmware-tanzu/velero/pkg/itemoperation"
	"github.com/vmware-tanzu/velero/pkg/kuberesource"
	"github.com/vmware-tanzu/velero/pkg/plugin/velero"
	"github.com/vmware-tanzu/velero/pkg/volume"
)

type VolumeBackupMethod string

const (
	NativeSnapshot  VolumeBackupMethod = "NativeSnapshot"
	PodVolumeBackup VolumeBackupMethod = "PodVolumeBackup"
	CSISnapshot     VolumeBackupMethod = "CSISnapshot"
)

const (
	FieldValueIsUnknown string = "unknown"
)

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

	CSISnapshotInfo          *CSISnapshotInfo          `json:"csiSnapshotInfo,omitempty"`
	SnapshotDataMovementInfo *SnapshotDataMovementInfo `json:"snapshotDataMovementInfo,omitempty"`
	NativeSnapshotInfo       *NativeSnapshotInfo       `json:"nativeSnapshotInfo,omitempty"`
	PVBInfo                  *PodVolumeBackupInfo      `json:"pvbInfo,omitempty"`
	PVInfo                   *PVInfo                   `json:"pvInfo,omitempty"`
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

	// The Async Operation's ID.
	OperationID string `json:"operationID"`
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

	// The Async Operation's ID.
	OperationID string `json:"operationID"`
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

// VolumesInformation contains the information needs by generating
// the backup VolumeInfo array.
type VolumesInformation struct {
	// A map contains the backup-included PV detail content. The key is PV name.
	pvMap       map[string]pvcPvInfo
	volumeInfos []*VolumeInfo

	logger                 logrus.FieldLogger
	crClient               kbclient.Client
	volumeSnapshots        []snapshotv1api.VolumeSnapshot
	volumeSnapshotContents []snapshotv1api.VolumeSnapshotContent
	volumeSnapshotClasses  []snapshotv1api.VolumeSnapshotClass
	SkippedPVs             map[string]string
	NativeSnapshots        []*volume.Snapshot
	PodVolumeBackups       []*velerov1api.PodVolumeBackup
	BackupOperations       []*itemoperation.BackupOperation
	BackupName             string
}

type pvcPvInfo struct {
	PVCName      string
	PVCNamespace string
	PV           corev1api.PersistentVolume
}

func (v *VolumesInformation) Init() {
	v.pvMap = make(map[string]pvcPvInfo)
	v.volumeInfos = make([]*VolumeInfo, 0)
}

func (v *VolumesInformation) InsertPVMap(pv corev1api.PersistentVolume, pvcName, pvcNamespace string) {
	if v.pvMap == nil {
		v.Init()
	}

	v.pvMap[pv.Name] = pvcPvInfo{
		PVCName:      pvcName,
		PVCNamespace: pvcNamespace,
		PV:           pv,
	}
}

func (v *VolumesInformation) Result(
	csiVolumeSnapshots []snapshotv1api.VolumeSnapshot,
	csiVolumeSnapshotContents []snapshotv1api.VolumeSnapshotContent,
	csiVolumesnapshotClasses []snapshotv1api.VolumeSnapshotClass,
	crClient kbclient.Client,
	logger logrus.FieldLogger,
) []*VolumeInfo {
	v.logger = logger
	v.crClient = crClient
	v.volumeSnapshots = csiVolumeSnapshots
	v.volumeSnapshotContents = csiVolumeSnapshotContents
	v.volumeSnapshotClasses = csiVolumesnapshotClasses

	v.generateVolumeInfoForSkippedPV()
	v.generateVolumeInfoForVeleroNativeSnapshot()
	v.generateVolumeInfoForCSIVolumeSnapshot()
	v.generateVolumeInfoFromPVB()
	v.generateVolumeInfoFromDataUpload()

	return v.volumeInfos
}

// generateVolumeInfoForSkippedPV generate VolumeInfos for SkippedPV.
func (v *VolumesInformation) generateVolumeInfoForSkippedPV() {
	tmpVolumeInfos := make([]*VolumeInfo, 0)

	for pvName, skippedReason := range v.SkippedPVs {
		if pvcPVInfo := v.retrievePvcPvInfo(pvName, "", ""); pvcPVInfo != nil {
			volumeInfo := &VolumeInfo{
				PVCName:           pvcPVInfo.PVCName,
				PVCNamespace:      pvcPVInfo.PVCNamespace,
				PVName:            pvName,
				SnapshotDataMoved: false,
				Skipped:           true,
				SkippedReason:     skippedReason,
				PVInfo: &PVInfo{
					ReclaimPolicy: string(pvcPVInfo.PV.Spec.PersistentVolumeReclaimPolicy),
					Labels:        pvcPVInfo.PV.Labels,
				},
			}
			tmpVolumeInfos = append(tmpVolumeInfos, volumeInfo)
		} else {
			v.logger.Warnf("Cannot find info for PV %s", pvName)
			continue
		}
	}

	v.volumeInfos = append(v.volumeInfos, tmpVolumeInfos...)
}

// generateVolumeInfoForVeleroNativeSnapshot generate VolumeInfos for Velero native snapshot
func (v *VolumesInformation) generateVolumeInfoForVeleroNativeSnapshot() {
	tmpVolumeInfos := make([]*VolumeInfo, 0)

	for _, nativeSnapshot := range v.NativeSnapshots {
		var iops int64
		if nativeSnapshot.Spec.VolumeIOPS != nil {
			iops = *nativeSnapshot.Spec.VolumeIOPS
		}

		if pvcPVInfo := v.retrievePvcPvInfo(nativeSnapshot.Spec.PersistentVolumeName, "", ""); pvcPVInfo != nil {
			volumeInfo := &VolumeInfo{
				BackupMethod:      NativeSnapshot,
				PVCName:           pvcPVInfo.PVCName,
				PVCNamespace:      pvcPVInfo.PVCNamespace,
				PVName:            pvcPVInfo.PV.Name,
				SnapshotDataMoved: false,
				Skipped:           false,
				NativeSnapshotInfo: &NativeSnapshotInfo{
					SnapshotHandle: nativeSnapshot.Status.ProviderSnapshotID,
					VolumeType:     nativeSnapshot.Spec.VolumeType,
					VolumeAZ:       nativeSnapshot.Spec.VolumeAZ,
					IOPS:           strconv.FormatInt(iops, 10),
				},
				PVInfo: &PVInfo{
					ReclaimPolicy: string(pvcPVInfo.PV.Spec.PersistentVolumeReclaimPolicy),
					Labels:        pvcPVInfo.PV.Labels,
				},
			}

			tmpVolumeInfos = append(tmpVolumeInfos, volumeInfo)
		} else {
			v.logger.Warnf("cannot find info for PV %s", nativeSnapshot.Spec.PersistentVolumeName)
			continue
		}
	}

	v.volumeInfos = append(v.volumeInfos, tmpVolumeInfos...)
}

// generateVolumeInfoForCSIVolumeSnapshot generate VolumeInfos for CSI VolumeSnapshot
func (v *VolumesInformation) generateVolumeInfoForCSIVolumeSnapshot() {
	tmpVolumeInfos := make([]*VolumeInfo, 0)

	for _, volumeSnapshot := range v.volumeSnapshots {
		var volumeSnapshotClass *snapshotv1api.VolumeSnapshotClass
		var volumeSnapshotContent *snapshotv1api.VolumeSnapshotContent

		// This is protective logic. The passed-in VS should be all related
		// to this backup.
		if volumeSnapshot.Labels[velerov1api.BackupNameLabel] != v.BackupName {
			continue
		}

		if volumeSnapshot.Spec.VolumeSnapshotClassName == nil {
			v.logger.Warnf("Cannot find VolumeSnapshotClass for VolumeSnapshot %s/%s", volumeSnapshot.Namespace, volumeSnapshot.Name)
			continue
		}

		if volumeSnapshot.Status == nil || volumeSnapshot.Status.BoundVolumeSnapshotContentName == nil {
			v.logger.Warnf("Cannot fine VolumeSnapshotContent for VolumeSnapshot %s/%s", volumeSnapshot.Namespace, volumeSnapshot.Name)
			continue
		}

		if volumeSnapshot.Spec.Source.PersistentVolumeClaimName == nil {
			v.logger.Warnf("VolumeSnapshot %s/%s doesn't have a source PVC", volumeSnapshot.Namespace, volumeSnapshot.Name)
			continue
		}

		for index := range v.volumeSnapshotClasses {
			if *volumeSnapshot.Spec.VolumeSnapshotClassName == v.volumeSnapshotClasses[index].Name {
				volumeSnapshotClass = &v.volumeSnapshotClasses[index]
			}
		}

		for index := range v.volumeSnapshotContents {
			if *volumeSnapshot.Status.BoundVolumeSnapshotContentName == v.volumeSnapshotContents[index].Name {
				volumeSnapshotContent = &v.volumeSnapshotContents[index]
			}
		}

		if volumeSnapshotClass == nil || volumeSnapshotContent == nil {
			v.logger.Warnf("fail to get VolumeSnapshotContent or VolumeSnapshotClass for VolumeSnapshot: %s/%s",
				volumeSnapshot.Namespace, volumeSnapshot.Name)
			continue
		}

		var operation itemoperation.BackupOperation
		for _, op := range v.BackupOperations {
			if op.Spec.ResourceIdentifier.GroupResource.String() == kuberesource.VolumeSnapshots.String() &&
				op.Spec.ResourceIdentifier.Name == volumeSnapshot.Name &&
				op.Spec.ResourceIdentifier.Namespace == volumeSnapshot.Namespace {
				operation = *op
			}
		}

		var size int64
		if volumeSnapshot.Status.RestoreSize != nil {
			size = volumeSnapshot.Status.RestoreSize.Value()
		}
		snapshotHandle := ""
		if volumeSnapshotContent.Status.SnapshotHandle != nil {
			snapshotHandle = *volumeSnapshotContent.Status.SnapshotHandle
		}
		if pvcPVInfo := v.retrievePvcPvInfo("", *volumeSnapshot.Spec.Source.PersistentVolumeClaimName, volumeSnapshot.Namespace); pvcPVInfo != nil {
			volumeInfo := &VolumeInfo{
				BackupMethod:          CSISnapshot,
				PVCName:               pvcPVInfo.PVCName,
				PVCNamespace:          pvcPVInfo.PVCNamespace,
				PVName:                pvcPVInfo.PV.Name,
				Skipped:               false,
				SnapshotDataMoved:     false,
				PreserveLocalSnapshot: true,
				StartTimestamp:        &(volumeSnapshot.CreationTimestamp),
				CSISnapshotInfo: &CSISnapshotInfo{
					VSCName:        *volumeSnapshot.Status.BoundVolumeSnapshotContentName,
					Size:           size,
					Driver:         volumeSnapshotClass.Driver,
					SnapshotHandle: snapshotHandle,
					OperationID:    operation.Spec.OperationID,
				},
				PVInfo: &PVInfo{
					ReclaimPolicy: string(pvcPVInfo.PV.Spec.PersistentVolumeReclaimPolicy),
					Labels:        pvcPVInfo.PV.Labels,
				},
			}

			tmpVolumeInfos = append(tmpVolumeInfos, volumeInfo)
		} else {
			v.logger.Warnf("cannot find info for PVC %s/%s", volumeSnapshot.Namespace, volumeSnapshot.Spec.Source.PersistentVolumeClaimName)
			continue
		}
	}

	v.volumeInfos = append(v.volumeInfos, tmpVolumeInfos...)
}

// generateVolumeInfoFromPVB generate VolumeInfo for PVB.
func (v *VolumesInformation) generateVolumeInfoFromPVB() {
	tmpVolumeInfos := make([]*VolumeInfo, 0)

	for _, pvb := range v.PodVolumeBackups {
		volumeInfo := &VolumeInfo{
			BackupMethod:      PodVolumeBackup,
			SnapshotDataMoved: false,
			Skipped:           false,
			StartTimestamp:    pvb.Status.StartTimestamp,
			PVBInfo: &PodVolumeBackupInfo{
				SnapshotHandle: pvb.Status.SnapshotID,
				Size:           pvb.Status.Progress.TotalBytes,
				UploaderType:   pvb.Spec.UploaderType,
				VolumeName:     pvb.Spec.Volume,
				PodName:        pvb.Spec.Pod.Name,
				PodNamespace:   pvb.Spec.Pod.Namespace,
				NodeName:       pvb.Spec.Node,
			},
		}

		pod := new(corev1api.Pod)
		pvcName := ""
		err := v.crClient.Get(context.TODO(), kbclient.ObjectKey{Namespace: pvb.Spec.Pod.Namespace, Name: pvb.Spec.Pod.Name}, pod)
		if err != nil {
			v.logger.WithError(err).Warn("Fail to get pod for PodVolumeBackup: ", pvb.Name)
			continue
		}
		for _, volume := range pod.Spec.Volumes {
			if volume.Name == pvb.Spec.Volume && volume.PersistentVolumeClaim != nil {
				pvcName = volume.PersistentVolumeClaim.ClaimName
			}
		}

		if pvcName != "" {
			if pvcPVInfo := v.retrievePvcPvInfo("", pvcName, pod.Namespace); pvcPVInfo != nil {
				volumeInfo.PVCName = pvcPVInfo.PVCName
				volumeInfo.PVCNamespace = pvcPVInfo.PVCNamespace
				volumeInfo.PVName = pvcPVInfo.PV.Name
				volumeInfo.PVInfo = &PVInfo{
					ReclaimPolicy: string(pvcPVInfo.PV.Spec.PersistentVolumeReclaimPolicy),
					Labels:        pvcPVInfo.PV.Labels,
				}
			} else {
				v.logger.Warnf("Cannot find info for PVC %s/%s", pod.Namespace, pvcName)
				continue
			}
		} else {
			v.logger.Debug("The PVB %s doesn't have a corresponding PVC", pvb.Name)
		}

		tmpVolumeInfos = append(tmpVolumeInfos, volumeInfo)
	}

	v.volumeInfos = append(v.volumeInfos, tmpVolumeInfos...)
}

// generateVolumeInfoFromDataUpload generate VolumeInfo for DataUpload.
func (v *VolumesInformation) generateVolumeInfoFromDataUpload() {
	if !features.IsEnabled(velerov1api.CSIFeatureFlag) {
		v.logger.Debug("Skip generating VolumeInfo when the CSI feature is disabled.")
		return
	}

	tmpVolumeInfos := make([]*VolumeInfo, 0)
	vsClassList := new(snapshotv1api.VolumeSnapshotClassList)
	if err := v.crClient.List(context.TODO(), vsClassList); err != nil {
		v.logger.WithError(err).Errorf("cannot list VolumeSnapshotClass %s", err.Error())
		return
	}

	for _, operation := range v.BackupOperations {
		if operation.Spec.ResourceIdentifier.GroupResource.String() == kuberesource.PersistentVolumeClaims.String() {
			var duIdentifier velero.ResourceIdentifier

			for _, identifier := range operation.Spec.PostOperationItems {
				if identifier.GroupResource.String() == "datauploads.velero.io" {
					duIdentifier = identifier
				}
			}
			if duIdentifier.Empty() {
				v.logger.Warnf("cannot find DataUpload for PVC %s/%s backup async operation",
					operation.Spec.ResourceIdentifier.Namespace, operation.Spec.ResourceIdentifier.Name)
				continue
			}

			dataUpload := new(velerov2alpha1.DataUpload)
			err := v.crClient.Get(
				context.TODO(),
				kbclient.ObjectKey{
					Namespace: duIdentifier.Namespace,
					Name:      duIdentifier.Name},
				dataUpload,
			)
			if err != nil {
				v.logger.Warnf("fail to get DataUpload for operation %s: %s", operation.Spec.OperationID, err.Error())
				continue
			}

			driverUsedByVSClass := ""
			for index := range vsClassList.Items {
				if vsClassList.Items[index].Name == dataUpload.Spec.CSISnapshot.SnapshotClass {
					driverUsedByVSClass = vsClassList.Items[index].Driver
				}
			}

			if pvcPVInfo := v.retrievePvcPvInfo("", operation.Spec.ResourceIdentifier.Name, operation.Spec.ResourceIdentifier.Namespace); pvcPVInfo != nil {
				dataMover := "velero"
				if dataUpload.Spec.DataMover != "" {
					dataMover = dataUpload.Spec.DataMover
				}

				volumeInfo := &VolumeInfo{
					BackupMethod:      CSISnapshot,
					PVCName:           pvcPVInfo.PVCName,
					PVCNamespace:      pvcPVInfo.PVCNamespace,
					PVName:            pvcPVInfo.PV.Name,
					SnapshotDataMoved: true,
					Skipped:           false,
					StartTimestamp:    operation.Status.Created,
					CSISnapshotInfo: &CSISnapshotInfo{
						SnapshotHandle: FieldValueIsUnknown,
						VSCName:        FieldValueIsUnknown,
						OperationID:    FieldValueIsUnknown,
						Driver:         driverUsedByVSClass,
					},
					SnapshotDataMovementInfo: &SnapshotDataMovementInfo{
						DataMover:    dataMover,
						UploaderType: "kopia",
						OperationID:  operation.Spec.OperationID,
					},
					PVInfo: &PVInfo{
						ReclaimPolicy: string(pvcPVInfo.PV.Spec.PersistentVolumeReclaimPolicy),
						Labels:        pvcPVInfo.PV.Labels,
					},
				}

				tmpVolumeInfos = append(tmpVolumeInfos, volumeInfo)
			} else {
				v.logger.Warnf("Cannot find info for PVC %s/%s", operation.Spec.ResourceIdentifier.Namespace, operation.Spec.ResourceIdentifier.Name)
				continue
			}
		}
	}

	v.volumeInfos = append(v.volumeInfos, tmpVolumeInfos...)
}

// retrievePvcPvInfo gets the PvcPvInfo from the PVMap.
// support retrieve info by PV's name, or by PVC's name
// and namespace.
func (v *VolumesInformation) retrievePvcPvInfo(pvName, pvcName, pvcNS string) *pvcPvInfo {
	if pvName != "" {
		if info, ok := v.pvMap[pvName]; ok {
			return &info
		}
		return nil
	}

	if pvcNS == "" || pvcName == "" {
		return nil
	}

	for _, info := range v.pvMap {
		if pvcNS == info.PVCNamespace && pvcName == info.PVCName {
			return &info
		}
	}

	return nil
}
