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
Two new data structures, `UploaderConfigForBackup` and `UploaderConfigForRestore`, will be defined to store Uploader configurations. These structures will include the configuration options related to backup and restore for Uploader:

```go
type UploaderConfigForBackup struct {
}

type UploaderConfigForRestore struct {
}
```

### Integration with Backup & Restore CRD
The Velero CLI will support an uploader configuration-related flag, allowing users to set the value when creating backups or restores. This value will be stored in the `UploaderConfig` field within the `Backup` CRD and `Restore` CRD:

```go
type BackupSpec struct {
	// UploaderConfig specifies the configuration for the uploader.
	// +optional
	// +nullable
	UploaderConfig *UploaderConfigForBackup `json:"uploaderConfig,omitempty"`
}

type RestoreSpec struct {
	// UploaderConfig specifies the configuration for the restore.
	// +optional
	// +nullable
	UploaderConfig *UploaderConfigForRestore `json:"uploaderConfig,omitempty"`
}
```

### Configuration Propagated to Different CRDs
The configuration specified in `UploaderConfig` needs to be effective for backup and restore both by file system way and data-mover way. 
Therefore, the `UploaderConfig` field value from the `Backup` CRD should be propagated to `PodVolumeBackup` and `DataUpload` CRDs.

We aim for the configurations in PodVolumeBackup to originate not only from UploaderConfig in Backup but also potentially from other sources such as the server or configmap. Simultaneously, to align with the configurations in DataUpload's `DataMoverConfig map[string]string`, we have defined an `UploaderSettings map[string]string` here to record the configurations in PodVolumeBackup.

```go
type PodVolumeBackupSpec struct {
	// UploaderSettings are a map of key-value pairs that should be applied to the
	// uploader configuration.
	// +optional
	// +nullable
	UploaderSettings map[string]string `json:"uploaderSettings,omitempty"`
}
```

`UploaderConfig` will be stored in DataUpload's `DataMoverConfig map[string]string` field.

Also the `UploaderConfig` field value from the `Restore` CRD should be propagated to `PodVolumeRestore` and `DataDownload` CRDs:

```go
type PodVolumeRestoreSpec struct {
	// UploaderSettings are a map of key-value pairs that should be applied to the
	// uploader configuration.
	// +optional
	// +nullable
	UploaderSettings map[string]string `json:"uploaderSettings,omitempty"`
}
```
Also `UploaderConfig` will be stored in DataUpload's `DataMoverConfig map[string]string` field.

### Store and Get Configuration
We need to store and retrieve configurations in the PodVolumeBackup and DataUpload structs. This involves type conversion based on the configuration type, storing it in a map[string]string, or performing type conversion from this map for retrieval.

PodVolumeRestore and DataDownload are also similar.

## Sub-options in UploaderConfig
Adding fields above in CRDs can accommodate any future additions to Uploader configurations by adding new fields to the `UploaderConfigForBackup` or `UploaderConfigForRestore` structures.

### Parallel Files Upload
This section focuses on enabling the configuration for the number of parallel file uploads during backups.
below are the key steps that should be added to support this new feature.

#### Velero CLI
The Velero CLI will support a `--parallel-files-upload` flag, allowing users to set the `ParallelFilesUpload` value when creating backups.

#### UploaderConfig
below the sub-option `ParallelFilesUpload` is added into UploaderConfig:

```go
// UploaderConfigForBackup defines the configuration for the uploader when doing backup.
type UploaderConfigForBackup struct {
	// ParallelFilesUpload is the number of files parallel uploads to perform when using the uploader.
	// +optional
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
if parallelUpload > 0 {
	curPolicy.UploadPolicy.MaxParallelFileReads = newOptionalInt(parallelUpload)
}
```

#### Restic Parallel Upload Policy
As Restic does not support parallel file upload, the configuration would not take effect, so we should output a warning when the user sets the `ParallelFilesUpload` value by using Restic to do a backup.

```go
if parallelFilesUpload > 0 {
	log.Warnf("ParallelFilesUpload is set to %d, but Restic does not support parallel file uploads. Ignoring", parallelFilesUpload)
	}
```

Roughly, the process is as follows: 
1. Users pass the ParallelFilesUpload parameter and its value through the Velero CLI. This parameter and its value are stored as a sub-option within UploaderConfig and then placed into the Backup CR. 
2. When users perform file system backups, UploaderConfig is passed to the PodVolumeBackup CR. When users use the Data-mover for backups, it is passed to the DataUpload CR. 
3. The configuration will be stored in map[string]string type of field in CR.
3. Each respective controller within the CRs calls the uploader, and the ParallelFilesUpload from map in CRs is passed to the uploader.
4. When the uploader subsequently calls the Kopia API, it can use the ParallelFilesUpload to set the MaxParallelFileReads parameter, and if the uploader calls the Restic command it would output one warning log for Restic does not support this feature.

### Sparse Option For Kopia & Restic Restore
In many system files, numerous zero bytes or empty blocks persist, occupying physical storage space. Sparse restore employs a more intelligent approach, including appropriately handling empty blocks, thereby achieving the correct system state. This write sparse files mechanism aims to enhance restore efficiency while maintaining restoration accuracy.
Below are the key steps that should be added to support this new feature.
#### Velero CLI
The Velero CLI will support a `--write-sparse-files` flag, allowing users to set the `WriteSparseFiles` value when creating restores with Restic or Kopia uploader.
#### UploaderConfig
below the sub-option `WriteSparseFiles` is added into UploaderConfig:
```go
// UploaderConfigForRestore defines the configuration for the restore.
type UploaderConfigForRestore struct {
	// WriteSparseFiles is a flag to indicate whether write files sparsely or not.
	// +optional
	// +nullable
	WriteSparseFiles *bool `json:"writeSparseFiles,omitempty"`
}
```

### Enable Sparse in Restic
For Restic, it could be enabled by pass the flag `--sparse` in creating restore:
```bash
restic restore create --sparse $snapshotID
```
### Enable Sparse in Kopia
For Kopia, it could be enabled this feature by the `WriteSparseFiles` field in the [FilesystemOutput](https://pkg.go.dev/github.com/kopia/kopia@v0.13.0/snapshot/restore#FilesystemOutput).

```go
fsOutput := &restore.FilesystemOutput{
		WriteSparseFiles:       uploaderutil.GetWriteSparseFiles(uploaderCfg),
	}
```
Roughly, the process is as follows:
1. Users pass the WriteSparseFiles parameter and its value through the Velero CLI. This parameter and its value are stored as a sub-option within UploaderConfig and then placed into the Restore CR.
2. When users perform file system restores, UploaderConfig is passed to the PodVolumeRestore CR. When users use the Data-mover for restores, it is passed to the DataDownload CR.
3. The configuration will be stored in map[string]string type of field in CR.
4. Each respective controller within the CRs calls the uploader, and the WriteSparseFiles from map in CRs is passed to the uploader.
5. When the uploader subsequently calls the Kopia API, it can use the WriteSparseFiles to set the WriteSparseFiles parameter, and if the uploader calls the Restic command it would append `--sparse` flag within the restore command.

### Parallel Restore
Setting the parallelism of restore operations can improve the efficiency and speed of the restore process, especially when dealing with large amounts of data.

### Velero CLI
The Velero CLI will support a --parallel-files-download flag, allowing users to set the parallelism value when creating restores. when no value specified, the value of it would be the number of CPUs for the node that the node agent pod is running.
```bash
	velero restore create --parallel-files-download $num
```

### UploaderConfig
below the sub-option parallel is added into UploaderConfig:

```go
	type UploaderConfigForRestore struct {
		// ParallelFilesDownload is the number of parallel for restore.
		// +optional
		ParallelFilesDownload int `json:"parallelFilesDownload,omitempty"`
	}
```

#### Kopia Parallel Restore Policy

Velero Uploader can set restore policies when calling Kopia APIs. In the Kopia codebase, the structure for restore policies is defined as follows:

```go
	// first get concurrrency from uploader config
	restoreConcurrency, _ := uploaderutil.GetRestoreConcurrency(uploaderCfg)
	// set restore concurrency into restore options
	restoreOpt := restore.Options{
		Parallel:               restoreConcurrency,
	}
	// do restore with restore option
	restore.Entry(..., restoreOpt)
```

#### Restic Parallel Restore Policy

Configurable parallel restore is not supported by restic, so we would return one error if the option is configured.
```go
	restoreConcurrency, err := uploaderutil.GetRestoreConcurrency(uploaderCfg)
	if err != nil {
		return extraFlags, errors.Wrap(err, "failed to get uploader config")
	}
	
	if restoreConcurrency > 0 {
		return extraFlags, errors.New("restic does not support parallel restore")
	}
```

## Alternatives Considered
To enhance extensibility further, the option of storing `UploaderConfig` in a Kubernetes ConfigMap can be explored, this approach would allow the addition and modification of configuration options without the need to modify the CRD.
