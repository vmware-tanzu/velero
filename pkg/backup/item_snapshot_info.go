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

package backup

import (
	"encoding/json"

	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/types"

	velerov1api "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	"github.com/vmware-tanzu/velero/pkg/plugin/velero"
	isv1 "github.com/vmware-tanzu/velero/pkg/plugin/velero/item_snapshotter/v1"
	"github.com/vmware-tanzu/velero/pkg/volume"
)

// ItemSnapshotInfo is used for our in-memory representation of ItemSnapshots and contains pointers to runtime
// structures such as ItemSnapshotter and Backup.  Converted to/from volume.ItemSnapshot for externalization

type ItemSnapshotInfo struct {
	ItemSnapshotter isv1.ItemSnapshotter
	Spec            ItemSnapshotInfoSpec
	Status          ItemSnapshotInfoStatus
}

type ItemSnapshotInfoSpec struct {
	ItemSnapshotter    string
	Backup             *velerov1api.Backup
	BackupName         string
	BackupUID          types.UID
	ResourceIdentifier velero.ResourceIdentifier
	Location           velerov1api.VolumeSnapshotLocation
}

type ItemSnapshotInfoStatus struct {
	ProviderSnapshotID string
	Metadata           map[string]string
	Phase              isv1.SnapshotPhase
	Error              string
}

func (recv ItemSnapshotInfo) GetItemSnapshot() (volume.ItemSnapshot, error) {
	ridStr, err := json.Marshal(recv.Spec.ResourceIdentifier)
	if err != nil {
		return volume.ItemSnapshot{}, errors.WithMessagef(err, "failed to marshal resource identifier %v", recv.Spec.ResourceIdentifier)
	}
	return volume.ItemSnapshot{
		Spec: volume.ItemSnapshotSpec{
			ItemSnapshotter:    recv.Spec.ItemSnapshotter,
			BackupName:         recv.Spec.BackupName,
			BackupUID:          string(recv.Spec.BackupUID),
			Location:           recv.Spec.Location.Name,
			ResourceIdentifier: string(ridStr),
		},
		Status: volume.ItemSnapshotStatus{
			ProviderSnapshotID: recv.Status.ProviderSnapshotID,
			Metadata:           recv.Status.Metadata,
			Phase:              recv.Status.Phase,
			Error:              recv.Status.Error,
		},
	}, nil
}
