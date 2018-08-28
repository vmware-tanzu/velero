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

package v1

const (
	// BackupNameLabel is the label key used to identify a backup by name.
	BackupNameLabel = "ark.heptio.com/backup-name"

	// BackupUIDLabel is the label key used to identify a backup by uid.
	BackupUIDLabel = "ark.heptio.com/backup-uid"

	// RestoreNameLabel is the label key used to identify a restore by name.
	RestoreNameLabel = "ark.heptio.com/restore-name"

	// RestoreUIDLabel is the label key used to identify a restore by uid.
	RestoreUIDLabel = "ark.heptio.com/restore-uid"

	// PodUIDLabel is the label key used to identify a pod by uid.
	PodUIDLabel = "ark.heptio.com/pod-uid"

	// PodVolumeOperationTimeoutAnnotation is the annotation key used to apply
	// a backup/restore-specific timeout value for pod volume operations (i.e.
	// restic backups/restores).
	PodVolumeOperationTimeoutAnnotation = "ark.heptio.com/pod-volume-timeout"

	// StorageLocationLabel is the label key used to identify the storage
	// location of a backup.
	StorageLocationLabel = "ark.heptio.com/storage-location"
)
