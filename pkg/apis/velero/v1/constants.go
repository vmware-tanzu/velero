/*
Copyright 2017 the Velero contributors.

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
	// DefaultNamespace is the Kubernetes namespace that is used by default for
	// the Velero server and API objects.
	DefaultNamespace = "velero"

	// ResourcesDir is a top-level directory expected in backups which contains sub-directories
	// for each resource type in the backup.
	ResourcesDir = "resources"

	// MetadataDir is a top-level directory expected in backups which contains
	// files that store metadata about the backup, such as the backup version.
	MetadataDir = "metadata"

	// ClusterScopedDir is the name of the directory containing cluster-scoped
	// resources within a Velero backup.
	ClusterScopedDir = "cluster"

	// NamespaceScopedDir is the name of the directory containing namespace-scoped
	// resource within a Velero backup.
	NamespaceScopedDir = "namespaces"

	// CSIFeatureFlag is the feature flag string that defines whether or not CSI features are being used.
	CSIFeatureFlag = "EnableCSI"

	// PreferredVersionDir is the suffix name of the directory containing the preferred version of the API group
	// resource within a Velero backup.
	PreferredVersionDir = "-preferredversion"

	// APIGroupVersionsFeatureFlag is the feature flag string that defines whether or not to handle multiple API Group Versions
	APIGroupVersionsFeatureFlag = "EnableAPIGroupVersions"

	BackupRepositoryTypeRestic string = "restic"
	BackupRepositoryTypeKopia  string = "kopia"

	VeleroResourceUsageDataUploadResult = "DataUpload"

	// Labels

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
	// pod volume backups/restores).
	PodVolumeOperationTimeoutAnnotation = "velero.io/pod-volume-timeout"

	// StorageLocationLabel is the label key used to identify the storage
	// location of a backup.
	StorageLocationLabel = "velero.io/storage-location"

	// VolumeNamespaceLabel is the label key used to identify which
	// namespace a repository stores backups for.
	VolumeNamespaceLabel = "velero.io/volume-namespace"

	// RepositoryTypeLabel is the label key used to identify the type of a repository
	RepositoryTypeLabel = "velero.io/repository-type"

	// DataUploadLabel is the label key used to identify the dataupload for snapshot backup pod
	DataUploadLabel = "velero.io/data-upload"

	// DataUploadSnapshotInfoLabel is used to identify the configmap that contains the snapshot info of a data upload
	// normally the value of the label should the "true" or "false"
	DataUploadSnapshotInfoLabel = "velero.io/data-upload-snapshot-info"

	// DataDownloadLabel is the label key used to identify the datadownload for snapshot restore pod
	DataDownloadLabel = "velero.io/data-download"

	// SourceClusterK8sVersionAnnotation is the label key used to identify the k8s
	// git version of the backup , i.e. v1.16.4
	SourceClusterK8sGitVersionAnnotation = "velero.io/source-cluster-k8s-gitversion"

	// SourceClusterK8sMajorVersionAnnotation is the label key used to identify the k8s
	// major version of the backup , i.e. 1
	SourceClusterK8sMajorVersionAnnotation = "velero.io/source-cluster-k8s-major-version"

	// SourceClusterK8sMajorVersionAnnotation is the label key used to identify the k8s
	// minor version of the backup , i.e. 16
	SourceClusterK8sMinorVersionAnnotation = "velero.io/source-cluster-k8s-minor-version"

	// ResourceTimeoutAnnotation is the annotation key used to carry the global resource
	// timeout value for backup to plugins.
	ResourceTimeoutAnnotation = "velero.io/resource-timeout"

	// AsyncOperationIDLabel is the label key used to identify the async operation ID
	AsyncOperationIDLabel = "velero.io/async-operation-id"

	// PVCNameLabel is the label key used to identify the PVC's namespace and name.
	// The format is <namespace>/<name>.
	PVCNamespaceNameLabel = "velero.io/pvc-namespace-name"

	// ResourceUsageLabel is the label key to explain the Velero resource usage.
	ResourceUsageLabel = "velero.io/resource-usage"

	// VolumesToBackupAnnotation is the annotation on a pod whose mounted volumes
	// need to be backed up using pod volume backup.
	VolumesToBackupAnnotation = "backup.velero.io/backup-volumes"

	// VolumesToExcludeAnnotation is the annotation on a pod whose mounted volumes
	// should be excluded from pod volume backup.
	VolumesToExcludeAnnotation = "backup.velero.io/backup-volumes-excludes"
)
