/*
Copyright 2018 the Heptio Ark contributors.

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
	"k8s.io/apimachinery/pkg/util/sets"
)

// pvcSnapshotTracker keeps track of persistent volume claims that have been snapshotted
// with restic.
type pvcSnapshotTracker struct {
	pvcs sets.String
}

func newPVCSnapshotTracker() *pvcSnapshotTracker {
	return &pvcSnapshotTracker{
		pvcs: sets.NewString(),
	}
}

// Track takes a pod and a list of volumes from that pod that were snapshotted, and
// tracks each snapshotted volume that's a PVC.
func (t *pvcSnapshotTracker) Track(pod *corev1api.Pod, snapshottedVolumes []string) {
	for _, volumeName := range snapshottedVolumes {
		// if the volume is a PVC, track it
		for _, volume := range pod.Spec.Volumes {
			if volume.Name == volumeName {
				if volume.PersistentVolumeClaim != nil {
					t.pvcs.Insert(key(pod.Namespace, volume.PersistentVolumeClaim.ClaimName))
				}
				break
			}
		}
	}
}

// Has returns true if the PVC with the specified namespace and name has been tracked.
func (t *pvcSnapshotTracker) Has(namespace, name string) bool {
	return t.pvcs.Has(key(namespace, name))
}

func key(namespace, name string) string {
	return fmt.Sprintf("%s/%s", namespace, name)
}
