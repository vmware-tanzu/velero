/*
Copyright 2020 the Velero contributors.

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
	"context"
	"fmt"
	"sort"
	"strconv"

	corev1api "k8s.io/api/core/v1"
	kbclient "sigs.k8s.io/controller-runtime/pkg/client"

	snapshotv1api "github.com/kubernetes-csi/external-snapshotter/client/v4/apis/volumesnapshot/v1"
	"github.com/sirupsen/logrus"

	"github.com/vmware-tanzu/velero/internal/hook"
	"github.com/vmware-tanzu/velero/internal/resourcepolicies"
	velerov1api "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	velerov2alpha1 "github.com/vmware-tanzu/velero/pkg/apis/velero/v2alpha1"
	"github.com/vmware-tanzu/velero/pkg/itemoperation"
	"github.com/vmware-tanzu/velero/pkg/kuberesource"
	"github.com/vmware-tanzu/velero/pkg/plugin/framework"
	"github.com/vmware-tanzu/velero/pkg/plugin/velero"
	"github.com/vmware-tanzu/velero/pkg/util/collections"
	"github.com/vmware-tanzu/velero/pkg/volume"
)

type itemKey struct {
	resource  string
	namespace string
	name      string
}

// Request is a request for a backup, with all references to other objects
// materialized (e.g. backup/snapshot locations, includes/excludes, etc.)
type Request struct {
	*velerov1api.Backup

	StorageLocation           *velerov1api.BackupStorageLocation
	SnapshotLocations         []*velerov1api.VolumeSnapshotLocation
	NamespaceIncludesExcludes *collections.IncludesExcludes
	ResourceIncludesExcludes  collections.IncludesExcludesInterface
	ResourceHooks             []hook.ResourceHook
	ResolvedActions           []framework.BackupItemResolvedActionV2
	VolumeSnapshots           []*volume.Snapshot
	PodVolumeBackups          []*velerov1api.PodVolumeBackup
	BackedUpItems             map[itemKey]struct{}
	itemOperationsList        *[]*itemoperation.BackupOperation
	ResPolicies               *resourcepolicies.Policies
	SkippedPVTracker          *skipPVTracker
	VolumesInformation        VolumesInformation
}

// VolumesInformation contains the information needs by generating
// the backup VolumeInfo array.
type VolumesInformation struct {
	// A map contains the backup-included PV detail content. The key is PV name.
	pvMap map[string]PvcPvInfo
}

func (v *VolumesInformation) InitPVMap() {
	v.pvMap = make(map[string]PvcPvInfo)
}

func (v *VolumesInformation) InsertPVMap(pvName string, pv PvcPvInfo) {
	if v.pvMap == nil {
		v.InitPVMap()
	}
	v.pvMap[pvName] = pv
}

func (v *VolumesInformation) GenerateVolumeInfo(backup *Request, csiVolumeSnapshots []snapshotv1api.VolumeSnapshot,
	csiVolumeSnapshotContents []snapshotv1api.VolumeSnapshotContent, csiVolumesnapshotClasses []snapshotv1api.VolumeSnapshotClass,
	crClient kbclient.Client, logger logrus.FieldLogger) []volume.VolumeInfo {
	volumeInfos := make([]volume.VolumeInfo, 0)

	skippedVolumeInfos := generateVolumeInfoForSkippedPV(*v, backup, logger)
	volumeInfos = append(volumeInfos, skippedVolumeInfos...)

	nativeSnapshotVolumeInfos := generateVolumeInfoForVeleroNativeSnapshot(*v, backup, logger)
	volumeInfos = append(volumeInfos, nativeSnapshotVolumeInfos...)

	csiVolumeInfos := generateVolumeInfoForCSIVolumeSnapshot(*v, backup, csiVolumeSnapshots, csiVolumeSnapshotContents, csiVolumesnapshotClasses, logger)
	volumeInfos = append(volumeInfos, csiVolumeInfos...)

	pvbVolumeInfos := generateVolumeInfoFromPVB(*v, backup, crClient, logger)
	volumeInfos = append(volumeInfos, pvbVolumeInfos...)

	dataUploadVolumeInfos := generateVolumeInfoFromDataUpload(*v, backup, crClient, logger)
	volumeInfos = append(volumeInfos, dataUploadVolumeInfos...)

	return volumeInfos
}

// generateVolumeInfoForSkippedPV generate VolumeInfos for SkippedPV.
func generateVolumeInfoForSkippedPV(info VolumesInformation, backup *Request, logger logrus.FieldLogger) []volume.VolumeInfo {
	tmpVolumeInfos := make([]volume.VolumeInfo, 0)

	for _, skippedPV := range backup.SkippedPVTracker.Summary() {
		if pvcPVInfo := info.retrievePvcPvInfo(skippedPV.Name, "", ""); pvcPVInfo != nil {
			volumeInfo := volume.VolumeInfo{
				PVCName:           pvcPVInfo.PVCName,
				PVCNamespace:      pvcPVInfo.PVCNamespace,
				PVName:            skippedPV.Name,
				SnapshotDataMoved: false,
				Skipped:           true,
				SkippedReason:     skippedPV.SerializeSkipReasons(),
				PVInfo: volume.PVInfo{
					ReclaimPolicy: string(pvcPVInfo.PV.Spec.PersistentVolumeReclaimPolicy),
					Labels:        pvcPVInfo.PV.Labels,
				},
			}
			tmpVolumeInfos = append(tmpVolumeInfos, volumeInfo)
		} else {
			logger.Warnf("Cannot find info for PV %s", skippedPV.Name)
			continue
		}
	}

	return tmpVolumeInfos
}

// generateVolumeInfoForVeleroNativeSnapshot generate VolumeInfos for Velero native snapshot
func generateVolumeInfoForVeleroNativeSnapshot(info VolumesInformation, backup *Request, logger logrus.FieldLogger) []volume.VolumeInfo {
	tmpVolumeInfos := make([]volume.VolumeInfo, 0)

	for _, nativeSnapshot := range backup.VolumeSnapshots {
		var iops int64
		if nativeSnapshot.Spec.VolumeIOPS != nil {
			iops = *nativeSnapshot.Spec.VolumeIOPS
		}

		if pvcPVInfo := info.retrievePvcPvInfo(nativeSnapshot.Spec.PersistentVolumeName, "", ""); pvcPVInfo != nil {
			volumeInfo := volume.VolumeInfo{
				BackupMethod:      volume.NativeSnapshot,
				PVCName:           pvcPVInfo.PVCName,
				PVCNamespace:      pvcPVInfo.PVCNamespace,
				PVName:            pvcPVInfo.PV.Name,
				SnapshotDataMoved: false,
				Skipped:           false,
				NativeSnapshotInfo: volume.NativeSnapshotInfo{
					SnapshotHandle: nativeSnapshot.Status.ProviderSnapshotID,
					VolumeType:     nativeSnapshot.Spec.VolumeType,
					VolumeAZ:       nativeSnapshot.Spec.VolumeAZ,
					IOPS:           strconv.FormatInt(iops, 10),
				},
				PVInfo: volume.PVInfo{
					ReclaimPolicy: string(pvcPVInfo.PV.Spec.PersistentVolumeReclaimPolicy),
					Labels:        pvcPVInfo.PV.Labels,
				},
			}

			tmpVolumeInfos = append(tmpVolumeInfos, volumeInfo)
		} else {
			logger.Warnf("cannot find info for PV %s", nativeSnapshot.Spec.PersistentVolumeName)
			continue
		}
	}

	return tmpVolumeInfos
}

// generateVolumeInfoForCSIVolumeSnapshot generate VolumeInfos for CSI VolumeSnapshot
func generateVolumeInfoForCSIVolumeSnapshot(info VolumesInformation, backup *Request, csiVolumeSnapshots []snapshotv1api.VolumeSnapshot,
	csiVolumeSnapshotContents []snapshotv1api.VolumeSnapshotContent, csiVolumesnapshotClasses []snapshotv1api.VolumeSnapshotClass,
	logger logrus.FieldLogger) []volume.VolumeInfo {
	tmpVolumeInfos := make([]volume.VolumeInfo, 0)

	for _, volumeSnapshot := range csiVolumeSnapshots {
		var volumeSnapshotClass *snapshotv1api.VolumeSnapshotClass
		var volumeSnapshotContent *snapshotv1api.VolumeSnapshotContent

		// This is protective logic. The passed-in VS should be all related
		// to this backup.
		if volumeSnapshot.Labels[velerov1api.BackupNameLabel] != backup.Name {
			continue
		}

		if volumeSnapshot.Spec.VolumeSnapshotClassName == nil {
			logger.Warnf("Cannot find VolumeSnapshotClass for VolumeSnapshot %s/%s", volumeSnapshot.Namespace, volumeSnapshot.Name)
			continue
		}

		if volumeSnapshot.Status == nil || volumeSnapshot.Status.BoundVolumeSnapshotContentName == nil {
			logger.Warnf("Cannot fine VolumeSnapshotContent for VolumeSnapshot %s/%s", volumeSnapshot.Namespace, volumeSnapshot.Name)
			continue
		}

		if volumeSnapshot.Spec.Source.PersistentVolumeClaimName == nil {
			logger.Warnf("VolumeSnapshot %s/%s doesn't have a source PVC", volumeSnapshot.Namespace, volumeSnapshot.Name)
			continue
		}

		for index := range csiVolumesnapshotClasses {
			if *volumeSnapshot.Spec.VolumeSnapshotClassName == csiVolumesnapshotClasses[index].Name {
				volumeSnapshotClass = &csiVolumesnapshotClasses[index]
			}
		}

		for index := range csiVolumeSnapshotContents {
			if *volumeSnapshot.Status.BoundVolumeSnapshotContentName == csiVolumeSnapshotContents[index].Name {
				volumeSnapshotContent = &csiVolumeSnapshotContents[index]
			}
		}

		if volumeSnapshotClass == nil || volumeSnapshotContent == nil {
			logger.Warnf("fail to get VolumeSnapshotContent or VolumeSnapshotClass for VolumeSnapshot: %s/%s",
				volumeSnapshot.Namespace, volumeSnapshot.Name)
			continue
		}

		var operation itemoperation.BackupOperation
		for _, op := range *backup.GetItemOperationsList() {
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
		if pvcPVInfo := info.retrievePvcPvInfo("", *volumeSnapshot.Spec.Source.PersistentVolumeClaimName, volumeSnapshot.Namespace); pvcPVInfo != nil {
			volumeInfo := volume.VolumeInfo{
				BackupMethod:          volume.CSISnapshot,
				PVCName:               pvcPVInfo.PVCName,
				PVCNamespace:          pvcPVInfo.PVCNamespace,
				PVName:                pvcPVInfo.PV.Name,
				Skipped:               false,
				SnapshotDataMoved:     false,
				PreserveLocalSnapshot: true,
				OperationID:           operation.Spec.OperationID,
				StartTimestamp:        &(volumeSnapshot.CreationTimestamp),
				CSISnapshotInfo: volume.CSISnapshotInfo{
					VSCName:        *volumeSnapshot.Status.BoundVolumeSnapshotContentName,
					Size:           size,
					Driver:         volumeSnapshotClass.Driver,
					SnapshotHandle: snapshotHandle,
				},
				PVInfo: volume.PVInfo{
					ReclaimPolicy: string(pvcPVInfo.PV.Spec.PersistentVolumeReclaimPolicy),
					Labels:        pvcPVInfo.PV.Labels,
				},
			}

			tmpVolumeInfos = append(tmpVolumeInfos, volumeInfo)
		} else {
			logger.Warnf("cannot find info for PVC %s/%s", volumeSnapshot.Namespace, volumeSnapshot.Spec.Source.PersistentVolumeClaimName)
			continue
		}
	}

	return tmpVolumeInfos
}

// generateVolumeInfoFromPVB generate VolumeInfo for PVB.
func generateVolumeInfoFromPVB(info VolumesInformation, backup *Request, crClient kbclient.Client, logger logrus.FieldLogger) []volume.VolumeInfo {
	tmpVolumeInfos := make([]volume.VolumeInfo, 0)

	for _, pvb := range backup.PodVolumeBackups {
		volumeInfo := volume.VolumeInfo{
			BackupMethod:      volume.PodVolumeBackup,
			SnapshotDataMoved: false,
			Skipped:           false,
			StartTimestamp:    pvb.Status.StartTimestamp,
			PVBInfo: volume.PodVolumeBackupInfo{
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
		err := crClient.Get(context.TODO(), kbclient.ObjectKey{Namespace: pvb.Spec.Pod.Namespace, Name: pvb.Spec.Pod.Name}, pod)
		if err != nil {
			logger.WithError(err).Warn("Fail to get pod for PodVolumeBackup: ", pvb.Name)
			continue
		}
		for _, volume := range pod.Spec.Volumes {
			if volume.Name == pvb.Spec.Volume && volume.PersistentVolumeClaim != nil {
				pvcName = volume.PersistentVolumeClaim.ClaimName
			}
		}

		if pvcName != "" {
			if pvcPVInfo := info.retrievePvcPvInfo("", pvcName, pod.Namespace); pvcPVInfo != nil {
				volumeInfo.PVCName = pvcPVInfo.PVCName
				volumeInfo.PVCNamespace = pvcPVInfo.PVCNamespace
				volumeInfo.PVName = pvcPVInfo.PV.Name
				volumeInfo.PVInfo = volume.PVInfo{
					ReclaimPolicy: string(pvcPVInfo.PV.Spec.PersistentVolumeReclaimPolicy),
					Labels:        pvcPVInfo.PV.Labels,
				}
			} else {
				logger.Warnf("Cannot find info for PVC %s/%s", pod.Namespace, pvcName)
				continue
			}
		} else {
			logger.Debug("The PVB %s doesn't have a corresponding PVC", pvb.Name)
		}

		tmpVolumeInfos = append(tmpVolumeInfos, volumeInfo)
	}

	return tmpVolumeInfos
}

// generateVolumeInfoFromDataUpload generate VolumeInfo for DataUpload.
func generateVolumeInfoFromDataUpload(info VolumesInformation, backup *Request, crClient kbclient.Client, logger logrus.FieldLogger) []volume.VolumeInfo {
	tmpVolumeInfos := make([]volume.VolumeInfo, 0)
	vsClassList := new(snapshotv1api.VolumeSnapshotClassList)
	if err := crClient.List(context.TODO(), vsClassList); err != nil {
		logger.WithError(err).Errorf("cannot list VolumeSnapshotClass %s", err.Error())
		return tmpVolumeInfos
	}

	for _, operation := range *backup.GetItemOperationsList() {
		if operation.Spec.ResourceIdentifier.GroupResource.String() == kuberesource.PersistentVolumeClaims.String() {
			var duIdentifier velero.ResourceIdentifier

			for _, identifier := range operation.Spec.PostOperationItems {
				if identifier.GroupResource.String() == "datauploads.velero.io" {
					duIdentifier = identifier
				}
			}
			if duIdentifier.Empty() {
				logger.Warnf("cannot find DataUpload for PVC %s/%s backup async operation",
					operation.Spec.ResourceIdentifier.Namespace, operation.Spec.ResourceIdentifier.Name)
				continue
			}

			dataUpload := new(velerov2alpha1.DataUpload)
			err := crClient.Get(
				context.TODO(),
				kbclient.ObjectKey{
					Namespace: duIdentifier.Namespace,
					Name:      duIdentifier.Name},
				dataUpload,
			)
			if err != nil {
				logger.Warnf("fail to get DataUpload for operation %s: %s", operation.Spec.OperationID, err.Error())
				continue
			}

			driverUsedByVSClass := ""
			for index := range vsClassList.Items {
				if vsClassList.Items[index].Name == dataUpload.Spec.CSISnapshot.SnapshotClass {
					driverUsedByVSClass = vsClassList.Items[index].Driver
				}
			}

			if pvcPVInfo := info.retrievePvcPvInfo("", operation.Spec.ResourceIdentifier.Name, operation.Spec.ResourceIdentifier.Namespace); pvcPVInfo != nil {
				volumeInfo := volume.VolumeInfo{
					BackupMethod:      volume.CSISnapshot,
					PVCName:           pvcPVInfo.PVCName,
					PVCNamespace:      pvcPVInfo.PVCNamespace,
					PVName:            pvcPVInfo.PV.Name,
					SnapshotDataMoved: true,
					Skipped:           false,
					OperationID:       operation.Spec.OperationID,
					StartTimestamp:    operation.Status.Created,
					CSISnapshotInfo: volume.CSISnapshotInfo{
						Driver: driverUsedByVSClass,
					},
					SnapshotDataMovementInfo: volume.SnapshotDataMovementInfo{
						DataMover:    dataUpload.Spec.DataMover,
						UploaderType: "kopia",
					},
					PVInfo: volume.PVInfo{
						ReclaimPolicy: string(pvcPVInfo.PV.Spec.PersistentVolumeReclaimPolicy),
						Labels:        pvcPVInfo.PV.Labels,
					},
				}

				tmpVolumeInfos = append(tmpVolumeInfos, volumeInfo)
			} else {
				logger.Warnf("Cannot find info for PVC %s/%s", operation.Spec.ResourceIdentifier.Namespace, operation.Spec.ResourceIdentifier.Name)
				continue
			}
		}
	}

	return tmpVolumeInfos
}

// retrievePvcPvInfo gets the PvcPvInfo from the PVMap.
// support retrieve info by PV's name, or by PVC's name
// and namespace.
func (v *VolumesInformation) retrievePvcPvInfo(pvName, pvcName, pvcNS string) *PvcPvInfo {
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

type PvcPvInfo struct {
	PVCName      string
	PVCNamespace string
	PV           corev1api.PersistentVolume
}

// GetItemOperationsList returns ItemOperationsList, initializing it if necessary
func (r *Request) GetItemOperationsList() *[]*itemoperation.BackupOperation {
	if r.itemOperationsList == nil {
		list := []*itemoperation.BackupOperation{}
		r.itemOperationsList = &list
	}
	return r.itemOperationsList
}

// BackupResourceList returns the list of backed up resources grouped by the API
// Version and Kind
func (r *Request) BackupResourceList() map[string][]string {
	resources := map[string][]string{}
	for i := range r.BackedUpItems {
		entry := i.name
		if i.namespace != "" {
			entry = fmt.Sprintf("%s/%s", i.namespace, i.name)
		}
		resources[i.resource] = append(resources[i.resource], entry)
	}

	// sort namespace/name entries for each GVK
	for _, v := range resources {
		sort.Strings(v)
	}

	return resources
}
