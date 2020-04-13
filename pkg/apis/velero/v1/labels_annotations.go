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

package v1

const (
	// BackupNameLabel is the label key used to identify a backup by name.
	BackupNameLabel = "velero.io/backup-name"

	// BackupUIDLabel is the label key used to identify a backup by uid.
	BackupUIDLabel = "velero.io/backup-uid"

	// RestoreNameLabel is the label key used to identify a restore by name.
	RestoreNameLabel = "velero.io/restore-name"

	// ScheduleNameLabel is the label key used to identify a schedule by name.
	ScheduleNameLabel = "velero.io/schedule-name"

	// RestoreUIDLabel is the label key used to identify a restore by uid.
	RestoreUIDLabel = "velero.io/restore-uid"

	// PodUIDLabel is the label key used to identify a pod by uid.
	PodUIDLabel = "velero.io/pod-uid"

	// PVCUIDLabel is the label key used to identify a PVC by uid.
	PVCUIDLabel = "velero.io/pvc-uid"

	// PodVolumeOperationTimeoutAnnotation is the annotation key used to apply
	// a backup/restore-specific timeout value for pod volume operations (i.e.
	// restic backups/restores).
	PodVolumeOperationTimeoutAnnotation = "velero.io/pod-volume-timeout"

	// StorageLocationLabel is the label key used to identify the storage
	// location of a backup.
	StorageLocationLabel = "velero.io/storage-location"

	// ResticVolumeNamespaceLabel is the label key used to identify which
	// namespace a restic repository stores pod volume backups for.
	ResticVolumeNamespaceLabel = "velero.io/volume-namespace"

	// SourceClusterK8sVersionAnnotation is the label key used to identify the k8s
	// git version of the backup , i.e. v1.16.4
	SourceClusterK8sGitVersionAnnotation = "velero.io/source-cluster-k8s-gitversion"

	// SourceClusterK8sMajorVersionAnnotation is the label key used to identify the k8s
	// major version of the backup , i.e. 1
	SourceClusterK8sMajorVersionAnnotation = "velero.io/source-cluster-k8s-major-version"

	// SourceClusterK8sMajorVersionAnnotation is the label key used to identify the k8s
	// minor version of the backup , i.e. 16
	SourceClusterK8sMinorVersionAnnotation = "velero.io/source-cluster-k8s-minor-version"
)
