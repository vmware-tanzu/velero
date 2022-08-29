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
	"strings"

	corev1api "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	velerov1api "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
)

const (
	// PVCNameAnnotation is the key for the annotation added to
	// pod volume backups when they're for a PVC.
	PVCNameAnnotation = "velero.io/pvc-name"

	// Deprecated.
	//
	// TODO(2.0): remove
	podAnnotationPrefix = "snapshot.velero.io/"

	// VolumesToBackupAnnotation is the annotation on a pod whose mounted volumes
	// need to be backed up using restic.
	VolumesToBackupAnnotation = "backup.velero.io/backup-volumes"

	// VolumesToExcludeAnnotation is the annotation on a pod whose mounted volumes
	// should be excluded from restic backup.
	VolumesToExcludeAnnotation = "backup.velero.io/backup-volumes-excludes"

	// InitContainer is the name of the init container added
	// to workload pods to help with restores.
	InitContainer = "restic-wait"
)

// GetVolumeBackupsForPod returns a map, of volume name -> snapshot id,
// of the PodVolumeBackups that exist for the provided pod.
func GetVolumeBackupsForPod(podVolumeBackups []*velerov1api.PodVolumeBackup, pod *corev1api.Pod, sourcePodNs string) map[string]string {
	volumes := make(map[string]string)

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

		volumes[pvb.Spec.Volume] = pvb.Status.SnapshotID
	}

	if len(volumes) > 0 {
		return volumes
	}

	return getPodSnapshotAnnotations(pod)
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
// Deprecated: we will stop using pod annotations to record restic snapshot IDs after they're taken,
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

// GetVolumesToBackup returns a list of volume names to backup for
// the provided pod.
// Deprecated: Use GetPodVolumesUsingRestic instead.
func GetVolumesToBackup(obj metav1.Object) []string {
	annotations := obj.GetAnnotations()
	if annotations == nil {
		return nil
	}

	backupsValue := annotations[VolumesToBackupAnnotation]
	if backupsValue == "" {
		return nil
	}

	return strings.Split(backupsValue, ",")
}

func getVolumesToExclude(obj metav1.Object) []string {
	annotations := obj.GetAnnotations()
	if annotations == nil {
		return nil
	}

	return strings.Split(annotations[VolumesToExcludeAnnotation], ",")
}

func contains(list []string, k string) bool {
	for _, i := range list {
		if i == k {
			return true
		}
	}
	return false
}

// GetPodVolumesUsingRestic returns a list of volume names to backup for the provided pod.
func GetPodVolumesUsingRestic(pod *corev1api.Pod, defaultVolumesToRestic bool) []string {
	if !defaultVolumesToRestic {
		return GetVolumesToBackup(pod)
	}

	volsToExclude := getVolumesToExclude(pod)
	podVolumes := []string{}
	for _, pv := range pod.Spec.Volumes {
		// cannot backup hostpath volumes as they are not mounted into /var/lib/kubelet/pods
		// and therefore not accessible to the restic daemon set.
		if pv.HostPath != nil {
			continue
		}
		// don't backup volumes mounting secrets. Secrets will be backed up separately.
		if pv.Secret != nil {
			continue
		}
		// don't backup volumes mounting config maps. Config maps will be backed up separately.
		if pv.ConfigMap != nil {
			continue
		}
		// don't backup volumes mounted as projected volumes, all data in those come from kube state.
		if pv.Projected != nil {
			continue
		}
		// don't backup DownwardAPI volumes, all data in those come from kube state.
		if pv.DownwardAPI != nil {
			continue
		}
		// don't backup volumes that are included in the exclude list.
		if contains(volsToExclude, pv.Name) {
			continue
		}
		// don't include volumes that mount the default service account token.
		if strings.HasPrefix(pv.Name, "default-token") {
			continue
		}
		podVolumes = append(podVolumes, pv.Name)
	}
	return podVolumes
}
