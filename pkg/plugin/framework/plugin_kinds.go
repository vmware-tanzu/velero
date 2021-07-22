/*
Copyright 2018, 2019 the Velero contributors.

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

package framework

// PluginKind is a type alias for a string that describes
// the kind of a Velero-supported plugin.
type PluginKind string

// String returns the string for k.
func (k PluginKind) String() string {
	return string(k)
}

const (
	// PluginKindObjectStore represents an object store plugin.
	PluginKindObjectStore PluginKind = "ObjectStore"

	// PluginKindVolumeSnapshotter represents a volume snapshotter plugin.
	PluginKindVolumeSnapshotter PluginKind = "VolumeSnapshotter"

	// PluginKindBackupItemAction represents a backup item action plugin.
	PluginKindBackupItemAction PluginKind = "BackupItemAction"

	// PluginKindRestoreItemAction represents a restore item action plugin.
	PluginKindRestoreItemAction PluginKind = "RestoreItemAction"

	// PluginKindDeleteItemAction represents a delete item action plugin.
	PluginKindDeleteItemAction PluginKind = "DeleteItemAction"

	// PluginKindPluginLister represents a plugin lister plugin.
	PluginKindPluginLister PluginKind = "PluginLister"
)

const (
	// PluginKindObjectStoreV2 represents an object store plugin version 2.
	PluginKindObjectStoreV2 PluginKind = "ObjectStoreV2"

	// PluginKindVolumeSnapshotterV2 represents a volume snapshotter plugin version 2.
	PluginKindVolumeSnapshotterV2 PluginKind = "VolumeSnapshotterV2"

	// PluginKindBackupItemActionV2 represents a backup item action plugin version 2.
	PluginKindBackupItemActionV2 PluginKind = "BackupItemActionV2"

	// PluginKindRestoreItemActionV2 represents a restore item action plugin version 2.
	PluginKindRestoreItemActionV2 PluginKind = "RestoreItemActionV2"

	// PluginKindDeleteItemActionV2 represents a delete item action plugin version 2.
	PluginKindDeleteItemActionV2 PluginKind = "DeleteItemActionV2"
)

func ObjectStoreKinds() []PluginKind {
	return []PluginKind{
		PluginKindObjectStoreV2,
		PluginKindObjectStore,
	}
}

func VolumeSnapshotterKinds() []PluginKind {
	return []PluginKind{
		PluginKindVolumeSnapshotterV2,
		PluginKindVolumeSnapshotter,
	}
}

func BackupItemActionKinds() []PluginKind {
	return []PluginKind{
		PluginKindBackupItemActionV2,
		PluginKindBackupItemAction,
	}
}

func RestoreItemActionKinds() []PluginKind {
	return []PluginKind{
		PluginKindRestoreItemActionV2,
		PluginKindRestoreItemAction,
	}
}

func DeleteItemActionKinds() []PluginKind {
	return []PluginKind{
		PluginKindDeleteItemActionV2,
		PluginKindDeleteItemAction,
	}
}

// AllPluginKinds contains all the valid plugin kinds that Velero supports, excluding PluginLister because that is not a
// kind that a developer would ever need to implement (it's handled by Velero and the Velero plugin library code).
func AllPluginKinds() map[string]PluginKind {
	allPluginKinds := make(map[string]PluginKind)
	allPluginKinds[PluginKindObjectStore.String()] = PluginKindObjectStore
	allPluginKinds[PluginKindVolumeSnapshotter.String()] = PluginKindVolumeSnapshotter
	allPluginKinds[PluginKindBackupItemAction.String()] = PluginKindBackupItemAction
	allPluginKinds[PluginKindRestoreItemAction.String()] = PluginKindRestoreItemAction
	allPluginKinds[PluginKindDeleteItemAction.String()] = PluginKindDeleteItemAction
	return allPluginKinds
}
