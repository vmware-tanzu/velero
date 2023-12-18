# Velero Uploader Configuration Integration and Extensibility

## Abstract
This design proposal aims to make Velero Uploader configurable by introducing a structured approach for managing Uploader settings. we will define and standardize a data structure to facilitate future additions to Uploader configurations. This enhancement provides a template for extending Uploader-related options. And also includes examples of adding sub-options to the Uploader Configuration.

## Background
Velero is widely used for backing up and restoring Kubernetes clusters. In various scenarios, optimizing the backup process is essential, future needs may arise for adding more configuration options related to the Uploader component especially when dealing with large datasets. Therefore, a standardized configuration template is required.

## Goals
1. **Extensible Uploader Configuration**: Provide an extensible approach to manage Uploader configurations, making it easy to add and modify configuration options related to the Velero uploader.
2. **User-friendliness**: Ensure that the new Uploader configuration options are easy to understand and use for Velero users without introducing excessive complexity.

## Non Goals
1. Expanding to other Velero components: The primary focus of this design is Uploader configuration and does not include extending to other components or modules within Velero. Configuration changes for other components may require separate design and implementation.

## High-Level Design
To achieve extensibility in Velero Uploader configurations, the following key components and changes are proposed:

### UploaderConfig Structure
A new data structure, `UploaderConfig`, will be defined to store Uploader configurations. This structure will include the configuration options related to backup and restore for Uploader:

```go
type UploaderConfig struct {
    // sub-options
}
```

### Integration with Backup CRD
The Velero CLI will support an uploader configuration-related flag, allowing users to set the value when creating backups or restores. This value will be stored in the `UploaderConfig` field within the `Backup` CRD and `Restore` CRD:

```go
type BackupSpec struct {
    UploaderConfig shared.UploaderConfig `json:"uploaderConfig,omitempty"`
}

type RestoreSpec struct {
    UploaderConfig shared.UploaderConfig `json:"uploaderConfig,omitempty"`
}
```

### Configuration Propagated to Different CRDs
The configuration specified in `UploaderConfig` needs to be effective for backup and restore both by file system way and data-mover way. 
Therefore, the `UploaderConfig` field value from the `Backup` CRD should be propagated to `PodVolumeBackup` and `DataUpload` CRDs:

```go
type PodVolumeBackupSpec struct {
    ...
    UploaderConfig shared.UploaderConfig `json:"uploaderConfig,omitempty"`
}

type DataUploadSpec struct {
    ...
    UploaderConfig shared.UploaderConfig `json:"uploaderConfig,omitempty"`
}
```

Also the `UploaderConfig` field value from the `Restore` CRD should be propagated to `PodVolumeRestore` and `DataDownload` CRDs:

```go
type PodVolumeRestoreSpec struct {
    ...
    UploaderConfig shared.UploaderConfig `json:"uploaderConfig,omitempty"`
}

type DataDownloadSpec struct {
    ...
    UploaderConfig shared.UploaderConfig `json:"uploaderConfig,omitempty"`
}
```

## Sub-options in UploaderConfig
Adding fields above in CRDs can accommodate any future additions to Uploader configurations by adding new fields to the `UploaderConfig` structure.

### Parallel Files Upload
This section focuses on enabling the configuration for the number of parallel file uploads during backups.
below are the key steps that should be added to support this new feature.

#### Velero CLI 
The Velero CLI will support a `--parallel-files-upload` flag, allowing users to set the `ParallelFilesUpload` value when creating backups.

#### UploaderConfig 
below the sub-option `ParallelFilesUpload` is added into UploaderConfig:

```go
type UploaderConfig struct {
    ParallelFilesUpload int `json:"parallelFilesUpload,omitempty"`
}
```

#### Kopia Parallel Upload Policy
Velero Uploader can set upload policies when calling Kopia APIs. In the Kopia codebase, the structure for upload policies is defined as follows:

```go
// UploadPolicy describes the policy to apply when uploading snapshots.
type UploadPolicy struct {
    ...
    MaxParallelFileReads *OptionalInt `json:"maxParallelFileReads,omitempty"`
}
```

Velero can set the `MaxParallelFileReads` parameter for Kopia's upload policy as follows:

```go
curPolicy := getDefaultPolicy()
if uploaderCfg.ParallelFilesUpload > 0 {
    curPolicy.UploadPolicy.MaxParallelFileReads = newOptionalInt(uploaderCfg.ParallelFilesUpload)
}
```

#### Restic Parallel Upload Policy
As Restic does not support parallel file upload, the configuration would not take effect, so we should output a warning when the user sets the `ParallelFilesUpload` value by using Restic to do a backup.

```go
if uploaderCfg.ParallelFilesUpload > 0 {
		log.Warnf("ParallelFilesUpload is set to %d, but Restic does not support parallel file uploads. Ignoring", uploaderCfg.ParallelFilesUpload)
	}
```

Roughly, the process is as follows: 
1. Users pass the ParallelFilesUpload parameter and its value through the Velero CLI. This parameter and its value are stored as a sub-option within UploaderConfig and then placed into the Backup CR. 
2. When users perform file system backups, UploaderConfig is passed to the PodVolumeBackup CR. When users use the Data-mover for backups, it is passed to the DataUpload CR. 
3. Each respective controller within the CRs calls the uploader, and the ParallelFilesUpload from UploaderConfig in CRs is passed to the uploader. 
4. When the uploader subsequently calls the Kopia API, it can use the ParallelFilesUpload to set the MaxParallelFileReads parameter, and if the uploader calls the Restic command it would output one warning log for Restic does not support this feature.

## Alternatives Considered
To enhance extensibility further, the option of storing `UploaderConfig` in a Kubernetes ConfigMap can be explored, this approach would allow the addition and modification of configuration options without the need to modify the CRD.