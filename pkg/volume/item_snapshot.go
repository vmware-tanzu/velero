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

package volume

import isv1 "github.com/vmware-tanzu/velero/pkg/plugin/velero/item_snapshotter/v1"

// ItemSnapshot stores information about an item snapshot (includes volumes and other Astrolabe objects) taken as
// part of a Velero backup.
type ItemSnapshot struct {
	Spec ItemSnapshotSpec `json:"spec"`

	Status ItemSnapshotStatus `json:"status"`
}

type ItemSnapshotSpec struct {
	// ItemSnapshotter is the name of the ItemSnapshotter plugin that took the snapshot
	ItemSnapshotter string `json:"itemSnapshotter"`

	// BackupName is the name of the Velero backup this snapshot
	// is associated with.
	BackupName string `json:"backupName"`

	// BackupUID is the UID of the Velero backup this snapshot
	// is associated with.
	BackupUID string `json:"backupUID"`

	// Location is the name of the location where this snapshot is stored.
	Location string `json:"location"`

	// Kubernetes resource identifier for the item
	ResourceIdentifier string "json:resourceIdentifier"
}

type ItemSnapshotStatus struct {
	// ProviderSnapshotID is the ID of the snapshot taken by the ItemSnapshotter
	ProviderSnapshotID string `json:"providerSnapshotID,omitempty"`

	// Metadata is the metadata returned with the snapshot to be returned to the ItemSnapshotter at restore time
	Metadata map[string]string `json:"metadata,omitempty"`

	// Phase is the current state of the ItemSnapshot.
	Phase isv1.SnapshotPhase `json:"phase,omitempty"`
}
