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
	"fmt"
	"strings"

	corev1api "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	velerov1api "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	"github.com/vmware-tanzu/velero/pkg/podvolume/configs"
	repotypes "github.com/vmware-tanzu/velero/pkg/repository/types"
	"github.com/vmware-tanzu/velero/pkg/uploader"
)

const (
	// Deprecated.
	//
	// TODO(2.0): remove
	podAnnotationPrefix = "snapshot.velero.io/"
)

// volumeBackupInfo describes the backup info of a volume backed up by PodVolumeBackups
type volumeBackupInfo struct {
	snapshotID     string
	uploaderType   string
	repositoryType string
}

// GetVolumeBackupsForPod returns a map, of volume name -> snapshot id,
// of the PodVolumeBackups that exist for the provided pod.
func GetVolumeBackupsForPod(podVolumeBackups []*velerov1api.PodVolumeBackup, pod *corev1api.Pod, sourcePodNs string) map[string]string {
	volumeBkInfo := getVolumeBackupInfoForPod(podVolumeBackups, pod, sourcePodNs)
	if volumeBkInfo == nil {
		return nil
	}

	volumes := make(map[string]string)
	for k, v := range volumeBkInfo {
		volumes[k] = v.snapshotID
	}

	return volumes
}

// GetPvbRepositoryType returns the repositoryType according to the PVB information
func GetPvbRepositoryType(pvb *velerov1api.PodVolumeBackup) string {
	return getRepositoryType(pvb.Spec.UploaderType)
}

// GetPvrRepositoryType returns the repositoryType according to the PVR information
func GetPvrRepositoryType(pvr *velerov1api.PodVolumeRestore) string {
	return getRepositoryType(pvr.Spec.UploaderType)
}

// getVolumeBackupInfoForPod returns a map, of volume name -> VolumeBackupInfo,
// of the PodVolumeBackups that exist for the provided pod.
func getVolumeBackupInfoForPod(podVolumeBackups []*velerov1api.PodVolumeBackup, pod *corev1api.Pod, sourcePodNs string) map[string]volumeBackupInfo {
	volumes := make(map[string]volumeBackupInfo)

	for _, pvb := range podVolumeBackups {
		if !isPVBMatchPod(pvb, pod.GetName(), sourcePodNs) {
			continue
		}

		// skip PVBs without a snapshot ID since there's nothing
		// to restore (they could be failed, or for empty volumes).
		if pvb.Status.SnapshotID == "" {
			continue
		}

		// If the volume came from a projected or DownwardAPI source, skip its restore.
		// This allows backups affected by https://github.com/vmware-tanzu/velero/issues/3863
		// or https://github.com/vmware-tanzu/velero/issues/4053 to be restored successfully.
		if volumeHasNonRestorableSource(pvb.Spec.Volume, pod.Spec.Volumes) {
			continue
		}

		volumes[pvb.Spec.Volume] = volumeBackupInfo{
			snapshotID:     pvb.Status.SnapshotID,
			uploaderType:   getUploaderTypeOrDefault(pvb.Spec.UploaderType),
			repositoryType: getRepositoryType(pvb.Spec.UploaderType),
		}
	}

	if len(volumes) > 0 {
		return volumes
	}

	fromAnnntation := getPodSnapshotAnnotations(pod)
	if fromAnnntation == nil {
		return nil
	}

	for k, v := range fromAnnntation {
		volumes[k] = volumeBackupInfo{v, uploader.ResticType, velerov1api.BackupRepositoryTypeRestic}
	}

	return volumes
}

// GetSnapshotIdentifier returns the snapshots represented by SnapshotIdentifier for the given PVBs
func GetSnapshotIdentifier(podVolumeBackups *velerov1api.PodVolumeBackupList) map[string][]repotypes.SnapshotIdentifier {
	res := map[string][]repotypes.SnapshotIdentifier{}
	for _, item := range podVolumeBackups.Items {
		if item.Status.SnapshotID == "" {
			continue
		}

		if res[item.Spec.Pod.Namespace] == nil {
			res[item.Spec.Pod.Namespace] = []repotypes.SnapshotIdentifier{}
		}

		snapshots := res[item.Spec.Pod.Namespace]

		snapshots = append(snapshots, repotypes.SnapshotIdentifier{
			VolumeNamespace:       item.Spec.Pod.Namespace,
			BackupStorageLocation: item.Spec.BackupStorageLocation,
			SnapshotID:            item.Status.SnapshotID,
			RepositoryType:        getRepositoryType(item.Spec.UploaderType),
			UploaderType:          item.Spec.UploaderType,
			Source:                item.Status.Path,
			RepoIdentifier:        item.Spec.RepoIdentifier,
		})

		res[item.Spec.Pod.Namespace] = snapshots
	}

	return res
}

func GetRealSource(pvb *velerov1api.PodVolumeBackup) string {
	pvcName := ""
	if pvb.Annotations != nil {
		pvcName = pvb.Annotations[configs.PVCNameAnnotation]
	}

	if pvcName != "" {
		return fmt.Sprintf("%s/%s/%s", pvb.Spec.Pod.Namespace, pvb.Spec.Pod.Name, pvcName)
	}
	return fmt.Sprintf("%s/%s/%s", pvb.Spec.Pod.Namespace, pvb.Spec.Pod.Name, pvb.Spec.Volume)
}

func getUploaderTypeOrDefault(uploaderType string) string {
	if uploaderType != "" {
		return uploaderType
	}
	return uploader.ResticType
}

// getRepositoryType returns the hardcode repositoryType for different backup methods - Restic or Kopia,uploaderType
// indicates the method.
// For Restic backup method, it is always hardcode to BackupRepositoryTypeRestic, never changed.
// For Kopia backup method, this means we hardcode repositoryType as BackupRepositoryTypeKopia for Unified Repo,
// at present (Kopia backup method is using Unified Repo). However, it doesn't mean we could deduce repositoryType
// from uploaderType for Unified Repo.
// TODO: post v1.10, refactor this function for Kopia backup method. In future, when we have multiple implementations of
// Unified Repo (besides Kopia), we will add the repositoryType to BSL, because by then, we are not able to hardcode
// the repositoryType to BackupRepositoryTypeKopia for Unified Repo.
func getRepositoryType(uploaderType string) string {
	switch uploaderType {
	case "", uploader.ResticType:
		return velerov1api.BackupRepositoryTypeRestic
	case uploader.KopiaType:
		return velerov1api.BackupRepositoryTypeKopia
	default:
		return ""
	}
}

func isPVBMatchPod(pvb *velerov1api.PodVolumeBackup, podName string, namespace string) bool {
	return podName == pvb.Spec.Pod.Name && namespace == pvb.Spec.Pod.Namespace
}

// volumeHasNonRestorableSource checks if the given volume exists in the list of podVolumes
// and returns true if the volume's source is not restorable. This is true for volumes with
// a Projected or DownwardAPI source.
func volumeHasNonRestorableSource(volumeName string, podVolumes []corev1api.Volume) bool {
	var volume corev1api.Volume
	for _, v := range podVolumes {
		if v.Name == volumeName {
			volume = v
			break
		}
	}
	return volume.Projected != nil || volume.DownwardAPI != nil
}

// getPodSnapshotAnnotations returns a map, of volume name -> snapshot id,
// of all snapshots for this pod.
// TODO(2.0) to remove
// Deprecated: we will stop using pod annotations to record pod volume snapshot IDs after they're taken,
// therefore we won't need to check if these annotations exist.
func getPodSnapshotAnnotations(obj metav1.Object) map[string]string {
	var res map[string]string

	insertSafe := func(k, v string) {
		if res == nil {
			res = make(map[string]string)
		}
		res[k] = v
	}

	for k, v := range obj.GetAnnotations() {
		if strings.HasPrefix(k, podAnnotationPrefix) {
			insertSafe(k[len(podAnnotationPrefix):], v)
		}
	}

	return res
}
