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
package csi

import (
	"fmt"

	snapshotv1beta1api "github.com/kubernetes-csi/external-snapshotter/v2/pkg/apis/volumesnapshot/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	velerov1api "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	"github.com/vmware-tanzu/velero/pkg/label"
)

// NewListOptions returns a ListOptions for matching CSI objects based on the Velero backup name and label.
func NewListOptions(name string) metav1.ListOptions {
	return metav1.ListOptions{
		LabelSelector: fmt.Sprintf("%s=%s", velerov1api.BackupNameLabel, label.GetValidName(name)),
	}
}

// SumSnapshotSize sums up the total restore size (in bytes) of a given slice of VolumeSnapshotContent objects.
func SumSnapshotSize(vscs []*snapshotv1beta1api.VolumeSnapshotContent) int64 {
	var total int64
	for _, vsc := range vscs {
		if vsc.Status != nil && vsc.Status.RestoreSize != nil {
			total += *vsc.Status.RestoreSize
		}
	}
	return total
}
