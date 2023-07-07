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

package backup

import (
	"fmt"

	corev1api "k8s.io/api/core/v1"
)

// pvcSnapshotTracker keeps track of persistent volume claims that have been handled
// via pod volume backup.
type pvcSnapshotTracker struct {
	pvcs   map[string]pvcSnapshotStatus
	pvcPod map[string]string
}

type pvcSnapshotStatus int

const (
	pvcSnapshotStatusNotTracked pvcSnapshotStatus = -1
	pvcSnapshotStatusTracked    pvcSnapshotStatus = iota
	pvcSnapshotStatusTaken
	pvcSnapshotStatusOptedout
)

func newPVCSnapshotTracker() *pvcSnapshotTracker {
	return &pvcSnapshotTracker{
		pvcs: make(map[string]pvcSnapshotStatus),
		// key: pvc ns/name, value: pod name
		pvcPod: make(map[string]string),
	}
}

// Track indicates a volume from a pod should be snapshotted by pod volume backup.
func (t *pvcSnapshotTracker) Track(pod *corev1api.Pod, volumeName string) {
	t.recordStatus(pod, volumeName, pvcSnapshotStatusTracked, pvcSnapshotStatusNotTracked)
}

// Take indicates a volume from a pod has been taken by pod volume backup.
func (t *pvcSnapshotTracker) Take(pod *corev1api.Pod, volumeName string) {
	t.recordStatus(pod, volumeName, pvcSnapshotStatusTaken, pvcSnapshotStatusTracked)
}

// Optout indicates a volume from a pod has been opted out by pod's annotation
func (t *pvcSnapshotTracker) Optout(pod *corev1api.Pod, volumeName string) {
	t.recordStatus(pod, volumeName, pvcSnapshotStatusOptedout, pvcSnapshotStatusNotTracked)
}

// OptedoutByPod returns true if the PVC with the specified namespace and name has been opted out by the pod.  The
// second return value is the name of the pod which has the annotation that opted out the volume/pvc
func (t *pvcSnapshotTracker) OptedoutByPod(namespace, name string) (bool, string) {
	status, found := t.pvcs[key(namespace, name)]

	if !found || status != pvcSnapshotStatusOptedout {
		return false, ""
	}
	return true, t.pvcPod[key(namespace, name)]
}

// if the volume is a PVC, record the status and the related pod
func (t *pvcSnapshotTracker) recordStatus(pod *corev1api.Pod, volumeName string, status pvcSnapshotStatus, preReqStatus pvcSnapshotStatus) {
	for _, volume := range pod.Spec.Volumes {
		if volume.Name == volumeName {
			if volume.PersistentVolumeClaim != nil {
				t.pvcPod[key(pod.Namespace, volume.PersistentVolumeClaim.ClaimName)] = pod.Name
				currStatus, ok := t.pvcs[key(pod.Namespace, volume.PersistentVolumeClaim.ClaimName)]
				if !ok {
					currStatus = pvcSnapshotStatusNotTracked
				}
				if currStatus == preReqStatus {
					t.pvcs[key(pod.Namespace, volume.PersistentVolumeClaim.ClaimName)] = status
				}
			}
			break
		}
	}
}

// Has returns true if the PVC with the specified namespace and name has been tracked.
func (t *pvcSnapshotTracker) Has(namespace, name string) bool {
	status, found := t.pvcs[key(namespace, name)]
	return found && (status == pvcSnapshotStatusTracked || status == pvcSnapshotStatusTaken)
}

// TakenForPodVolume returns true and the PVC's name if the pod volume with the specified name uses a
// PVC and that PVC has been taken by pod volume backup.
func (t *pvcSnapshotTracker) TakenForPodVolume(pod *corev1api.Pod, volume string) (bool, string) {
	for _, podVolume := range pod.Spec.Volumes {
		if podVolume.Name != volume {
			continue
		}

		if podVolume.PersistentVolumeClaim == nil {
			return false, ""
		}

		status, found := t.pvcs[key(pod.Namespace, podVolume.PersistentVolumeClaim.ClaimName)]
		if !found {
			return false, ""
		}

		if status != pvcSnapshotStatusTaken {
			return false, ""
		}

		return true, podVolume.PersistentVolumeClaim.ClaimName
	}

	return false, ""
}

func key(namespace, name string) string {
	return fmt.Sprintf("%s/%s", namespace, name)
}
