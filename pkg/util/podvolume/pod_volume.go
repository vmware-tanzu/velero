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
	"context"
	"strings"

	"github.com/pkg/errors"
	corev1api "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	crclient "sigs.k8s.io/controller-runtime/pkg/client"

	velerov1api "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	"github.com/vmware-tanzu/velero/pkg/util"
)

// GetVolumesByPod returns a list of volume names to backup for the provided pod.
func GetVolumesByPod(pod *corev1api.Pod, defaultVolumesToFsBackup, backupExcludePVC bool, volsToProcessByLegacyApproach []string) ([]string, []string) {
	// tracks the volumes that have been explicitly opted out of backup via the annotation in the pod
	optedOutVolumes := make([]string, 0)

	if !defaultVolumesToFsBackup {
		return GetVolumesToBackup(pod), optedOutVolumes
	}

	volsToExclude := GetVolumesToExclude(pod)
	podVolumes := []string{}
	// Identify volume to process
	// For normal case all the pod volume will be processed
	// For case when volsToProcessByLegacyApproach is non-empty then only those volume will be processed
	volsToProcess := GetVolumesToProcess(pod.Spec.Volumes, volsToProcessByLegacyApproach)
	for _, pv := range volsToProcess {
		// cannot backup hostpath volumes as they are not mounted into /var/lib/kubelet/pods
		// and therefore not accessible to the node agent daemon set.
		if pv.HostPath != nil {
			continue
		}
		// don't backup volumes mounting secrets. Secrets will be backed up separately.
		if pv.Secret != nil {
			continue
		}
		// don't backup volumes mounting ConfigMaps. ConfigMaps will be backed up separately.
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
		if pv.PersistentVolumeClaim != nil && backupExcludePVC {
			continue
		}
		// don't backup volumes that are included in the exclude list.
		if util.Contains(volsToExclude, pv.Name) {
			optedOutVolumes = append(optedOutVolumes, pv.Name)
			continue
		}
		// don't include volumes that mount the default service account token.
		if strings.HasPrefix(pv.Name, "default-token") {
			continue
		}
		podVolumes = append(podVolumes, pv.Name)
	}
	return podVolumes, optedOutVolumes
}

// GetVolumesToBackup returns a list of volume names to backup for
// the provided pod.
// Deprecated: Use GetVolumesByPod instead.
func GetVolumesToBackup(obj metav1.Object) []string {
	annotations := obj.GetAnnotations()
	if annotations == nil {
		return nil
	}

	backupsValue := annotations[velerov1api.VolumesToBackupAnnotation]
	if backupsValue == "" {
		return nil
	}

	return strings.Split(backupsValue, ",")
}

func GetVolumesToExclude(obj metav1.Object) []string {
	annotations := obj.GetAnnotations()
	if annotations == nil {
		return nil
	}

	return strings.Split(annotations[velerov1api.VolumesToExcludeAnnotation], ",")
}

func IsPVCDefaultToFSBackup(pvcNamespace, pvcName string, crClient crclient.Client, defaultVolumesToFsBackup bool) (bool, error) {
	pods, err := GetPodsUsingPVC(pvcNamespace, pvcName, crClient)
	if err != nil {
		return false, errors.WithStack(err)
	}

	for index := range pods {
		vols, _ := GetVolumesByPod(&pods[index], defaultVolumesToFsBackup, false, []string{})
		if len(vols) > 0 {
			volName, err := getPodVolumeNameForPVC(pods[index], pvcName)
			if err != nil {
				return false, err
			}
			if util.Contains(vols, volName) {
				return true, nil
			}
		}
	}

	return false, nil
}

func getPodVolumeNameForPVC(pod corev1api.Pod, pvcName string) (string, error) {
	for _, v := range pod.Spec.Volumes {
		if v.PersistentVolumeClaim != nil && v.PersistentVolumeClaim.ClaimName == pvcName {
			return v.Name, nil
		}
	}
	return "", errors.Errorf("Pod %s/%s does not use PVC %s/%s", pod.Namespace, pod.Name, pod.Namespace, pvcName)
}

func GetPodsUsingPVC(
	pvcNamespace, pvcName string,
	crClient crclient.Client,
) ([]corev1api.Pod, error) {
	podsUsingPVC := []corev1api.Pod{}
	podList := new(corev1api.PodList)
	if err := crClient.List(
		context.TODO(),
		podList,
		&crclient.ListOptions{Namespace: pvcNamespace},
	); err != nil {
		return nil, err
	}

	for _, p := range podList.Items {
		for _, v := range p.Spec.Volumes {
			if v.PersistentVolumeClaim != nil && v.PersistentVolumeClaim.ClaimName == pvcName {
				podsUsingPVC = append(podsUsingPVC, p)
			}
		}
	}

	return podsUsingPVC, nil
}

func GetVolumesToProcess(volumes []corev1api.Volume, volsToProcessByLegacyApproach []string) []corev1api.Volume {
	volsToProcess := make([]corev1api.Volume, 0)

	// return empty list when no volumes associated with pod
	if len(volumes) == 0 {
		return volsToProcess
	}

	// legacy approach as a fallback option case
	if len(volsToProcessByLegacyApproach) > 0 {
		for _, vol := range volumes {
			// don't process volumes that are already matched for supported action in volume policy approach
			if !util.Contains(volsToProcessByLegacyApproach, vol.Name) {
				continue
			}

			// add volume that is not processed in volume policy approach
			volsToProcess = append(volsToProcess, vol)
		}

		return volsToProcess
	}
	// normal case return the list as in
	return volumes
}
