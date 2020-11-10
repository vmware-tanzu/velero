# Progress reporting for backups and restores handled by volume snapshotters

Users face difficulty in knowing the progress of backup/restore operations of volume snapshotters. This is very similar to the issues faced by users to know progress for restic backup/restore, like, estimation of operation, operation in-progress/hung etc.

Each plugin might be providing a way to know the progress, but, it need not uniform across the plugins.

Even though plugins provide the way to know the progress of backup operation, this information won't be available to user during restore time on the destination cluster.

So, apart from the issues like progress, status of operation, volume snapshotters have unique problems like
- not being uniform across plugins
- not knowing the backup information during restore operation
- need to be optional as few plugins may not have a way to provide the progress information

This document proposes an approach for plugins to follow to provide backup/restore progress, which can be used by users to know the progress.

## Goals

- Provide uniform way of visibility into backup/restore operations performed by volume snapshotters

## Non Goals
- Plugin implementation for this approach

## Background

(Omitted, see introduction)

## High-Level Design

### Progress of backup operation handled by volume snapshotter

Progress will be updated by volume snapshotter in VolumePluginBackup CR which is specific to that backup operation.

### Progress of restore operation handled by volume snapshotter

Progress will be updated by volume snapshotter in VolumePluginRestore CR which is specific to that restore operation.

## Detailed Design

### Approach 1

Existing `Snapshot` Go struct from `volume` package have most of the details related to backup operation performed by volumesnapshotters.
This struct also gets backed up to backup location. But, this struct doesn't get synced on other clusters at regular intervals.
It is currently synced only during restore operation, and velero CLI shows few of its contents.

At a high level, in this approach, this struct will be converted to a CR by adding new fields (related to Progress tracking) to it, and gets rid of `volume.Snapshot` struct.

Instead of backing up of Go struct, proposal is: to backup CRs to backup location, and sync them into other cluster by backupSyncController running in that cluster.

#### VolumePluginBackup CR

There is one addition to volume.SnapshotSpec, i.e., ProviderName to convert it to CR's spec. Below is the updated VolumePluginBackup CR's Spec:

```
type VolumePluginBackupSpec struct {
	// BackupName is the name of the Velero backup this snapshot
	// is associated with.
	BackupName string `json:"backupName"`

	// BackupUID is the UID of the Velero backup this snapshot
	// is associated with.
	BackupUID string `json:"backupUID"`

	// Location is the name of the VolumeSnapshotLocation where this snapshot is stored.
	Location string `json:"location"`

	// PersistentVolumeName is the Kubernetes name for the volume.
	PersistentVolumeName string `json:"persistentVolumeName"`

	// ProviderVolumeID is the provider's ID for the volume.
	ProviderVolumeID string `json:"providerVolumeID"`

	// Provider is the Provider field given in VolumeSnapshotLocation
	Provider string `json:"provider"`

	// VolumeType is the type of the disk/volume in the cloud provider
	// API.
	VolumeType string `json:"volumeType"`

	// VolumeAZ is the where the volume is provisioned
	// in the cloud provider.
	VolumeAZ string `json:"volumeAZ,omitempty"`

	// VolumeIOPS is the optional value of provisioned IOPS for the
	// disk/volume in the cloud provider API.
	VolumeIOPS *int64 `json:"volumeIOPS,omitempty"`
}
```

Few fields (except first two) are added to volume.SnapshotStatus to convert it to CR's status. Below is the updated VolumePluginBackup CR's status:
```
type VolumePluginBackupStatus struct {
	// ProviderSnapshotID is the ID of the snapshot taken in the cloud
	// provider API of this volume.
	ProviderSnapshotID string `json:"providerSnapshotID,omitempty"`

	// Phase is the current state of the VolumeSnapshot.
	Phase SnapshotPhase `json:"phase,omitempty"`

	// PluginSpecific are a map of key-value pairs that plugin want to provide
	// to user to identify plugin properties related to this backup
	// +optional
	PluginSpecific map[string]string `json:"pluginSpecific,omitempty"`

	// Message is a message about the volume plugin's backup's status.
	// +optional
	Message string `json:"message,omitempty"`

	// StartTimestamp records the time a backup was started.
	// Separate from CreationTimestamp, since that value changes
	// on restores.
	// The server's time is used for StartTimestamps
	// +optional
	// +nullable
	StartTimestamp *metav1.Time `json:"startTimestamp,omitempty"`

	// CompletionTimestamp records the time a backup was completed.
	// Completion time is recorded even on failed backups.
	// Completion time is recorded before uploading the backup object.
	// The server's time is used for CompletionTimestamps
	// +optional
	// +nullable
	CompletionTimestamp *metav1.Time `json:"completionTimestamp,omitempty"`

	// Progress holds the total number of bytes of the volume and the current
	// number of backed up bytes. This can be used to display progress information
	// about the backup operation.
	// +optional
	Progress VolumeOperationProgress `json:"progress,omitempty"`
}

type VolumeOperationProgress struct {
	TotalBytes int64
	BytesDone int64
}

type VolumePluginBackup struct {
	metav1.TypeMeta `json:",inline"`

	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// +optional
	Spec VolumePluginBackupSpec `json:"spec,omitempty"`

	// +optional
	Status VolumePluginBackupStatus `json:"status,omitempty"`
}
```

For every backup operation of volume, Velero creates VolumePluginBackup CR before calling volumesnapshotter's CreateSnapshot API.

In order to know the CR created for the particular backup of a volume, Velero adds following labels to CR:
- `velero.io/backup-name` with value as Backup Name, and,
- `velero.io/pv-name` with value as volume that is undergoing backup

Backup name being unique won't cause issues like duplicates in identifying the CR.
Labels will be set with the value returned from `GetValidName` function. (https://github.com/vmware-tanzu/velero/blob/main/pkg/label/label.go#L35).

If Plugin supports showing progress of the operation it is performing, it does following:
- finds the VolumePluginBackup CR related to this backup operation by using `tags` passed in CreateSnapshot call
- updates the CR with the progress regularly.

After return from `CreateSnapshot` in `takePVSnapshot`, currently Velero adds `volume.Snapshot` to `backupRequest`. Instead of this, CR will be added to `backupRequest`.
During persistBackup call, this CR also will be backed up to backup location.

In backupSyncController, it checks for any VolumePluginBackup CRs that need to be synced from backup location, and syncs them to cluster if needed.

VolumePluginBackup will be useful as long as backed up data is available at backup location. When the Backup is deleted either by manually or due to expiry, VolumePluginBackup also can be deleted.

`processRequest` of `backupDeletionController` will perform deletion of VolumePluginBackup before volumesnapshotter's DeleteSnapshot is called.

#### Backward compatibility:

Currently `volume.Snapshot` is backed up as `<backupname>-volumesnapshots.json.gz` file in the backup location.

As the VolumePluginBackup CR is backed up instead of `volume.Snapshot`, to provide backward compatibility, CR will be backed as the same file i.e., `<backupname>-volumesnapshots.json.gz` file in the backup location.

For backward compatibility on restore side, consider below possible cases wrt Velero version on restore side and format of json.gz file at object location:

- older version of Velero, older json.gz file (backupname-volumesnapshots.json.gz)

- older version of Velero, newer json.gz file

- newer version of Velero, older json.gz file

- newer version of Velero, newer json.gz file

First and last should be fine.

For second case, decode in `GetBackupVolumeSnapshots` on the restore side should fill only required fields of older version and should work.

For third case, after decode, metadata.name will be empty. `GetBackupVolumeSnapshots` decodes older json.gz into the CR which goes fine.
It will be modified to return []VolumePluginBackupSpec, and the changes are done accordingly in its caller.

If decode fails in second case during implementation, this CR need to be backed up to different file. And, for backward compatibility, newer code should check for old file existence, and follow older code if exists. If it doesn't exists, check for newer file and follow the newer code.

`backupSyncController` on restore clusters gets the `<backupname>-volumesnapshots.json.gz` object from backup location and decodes it to in-memory VolumePluginBackup CR. If its `metadata.name` is populated, controller creates CR. Otherwise, it will not create the CR on the cluster. It can be even considered to create CR on the cluster.

#### VolumePluginRestore CR

```
// VolumePluginRestoreSpec is the specification for a VolumePluginRestore CR.
type VolumePluginRestoreSpec struct {
	// SnapshotID is the identifier for the snapshot of the volume.
	// This will be used to relate with output in 'velero describe backup'
	SnapshotID string `json:"snapshotID"`

	// BackupName is the name of the Velero backup from which PV will be
	// created.
	BackupName string `json:"backupName"`

	// Provider is the Provider field given in VolumeSnapshotLocation
	Provider string `json:"provider"`

	// VolumeType is the type of the disk/volume in the cloud provider
	// API.
	VolumeType string `json:"volumeType"`

	// VolumeAZ is the where the volume is provisioned
	// in the cloud provider.
	VolumeAZ string `json:"volumeAZ,omitempty"`
}

// VolumePluginRestoreStatus is the current status of a VolumePluginRestore CR.
type VolumePluginRestoreStatus struct {
	// Phase is the current state of the VolumePluginRestore.
	Phase string `json:"phase"`

	// VolumeID is the PV name to which restore done
	VolumeID string `json:"volumeID"`

	// Message is a message about the volume plugin's restore's status.
	// +optional
	Message string `json:"message,omitempty"`

	// StartTimestamp records the time a restore was started.
	// Separate from CreationTimestamp, since that value changes
	// on restores.
	// The server's time is used for StartTimestamps
	// +optional
	// +nullable
	StartTimestamp *metav1.Time `json:"startTimestamp,omitempty"`

	// CompletionTimestamp records the time a restore was completed.
	// Completion time is recorded even on failed restores.
	// The server's time is used for CompletionTimestamps
	// +optional
	// +nullable
	CompletionTimestamp *metav1.Time `json:"completionTimestamp,omitempty"`

	// Progress holds the total number of bytes of the snapshot and the current
	// number of restored bytes. This can be used to display progress information
	// about the restore operation.
	// +optional
	Progress VolumeOperationProgress `json:"progress,omitempty"`

	// PluginSpecific are a map of key-value pairs that plugin want to provide
	// to user to identify plugin properties related to this restore
	// +optional
	PluginSpecific map[string]string `json:"pluginSpecific,omitempty"`
}

type VolumePluginRestore struct {
	metav1.TypeMeta `json:",inline"`

	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// +optional
	Spec VolumePluginRestoreSpec `json:"spec,omitempty"`

	// +optional
	Status VolumePluginRestoreStatus `json:"status,omitempty"`
}
```

For every restore operation, Velero creates VolumePluginRestore CR before calling volumesnapshotter's CreateVolumeFromSnapshot API.

In order to know the CR created for the particular restore of a volume, Velero adds following labels to CR:
- `velero.io/backup-name` with value as Backup Name, and,
- `velero.io/snapshot-id` with value as snapshot id that need to be restored
- `velero.io/provider` with value as `Provider` in `VolumeSnapshotLocation`

Labels will be set with the value returned from `GetValidName` function. (https://github.com/vmware-tanzu/velero/blob/main/pkg/label/label.go#L35).

Plugin will be able to identify CR by using snapshotID that it received as parameter of CreateVolumeFromSnapshot API, and plugin's Provider name.
It updates the progress of restore operation regularly if plugin supports feature of showing progress.

Velero deletes VolumePluginRestore CR when it handles deletion of Restore CR.

### Approach 2

This approach is different to approach 1 only with respect to Backup.

#### VolumePluginBackup CR

```
// VolumePluginBackupSpec is the specification for a VolumePluginBackup CR.
type VolumePluginBackupSpec struct {
	// Volume is the PV name to be backed up.
	Volume string `json:"volume"`

	// Backup name
	Backup string `json:"backup"`

	// Provider is the Provider field given in VolumeSnapshotLocation
	Provider string `json:"provider"`
}

// VolumePluginBackupStatus is the current status of a VolumePluginBackup CR.
type VolumePluginBackupStatus struct {
	// Phase is the current state of the VolumePluginBackup.
	Phase string `json:"phase"`

	// SnapshotID is the identifier for the snapshot of the volume.
	// This will be used to relate with output in 'velero describe backup'
	SnapshotID string `json:"snapshotID"`

	// Message is a message about the volume plugin's backup's status.
	// +optional
	Message string `json:"message,omitempty"`

	// StartTimestamp records the time a backup was started.
	// Separate from CreationTimestamp, since that value changes
	// on restores.
	// The server's time is used for StartTimestamps
	// +optional
	// +nullable
	StartTimestamp *metav1.Time `json:"startTimestamp,omitempty"`

	// CompletionTimestamp records the time a backup was completed.
	// Completion time is recorded even on failed backups.
	// Completion time is recorded before uploading the backup object.
	// The server's time is used for CompletionTimestamps
	// +optional
	// +nullable
	CompletionTimestamp *metav1.Time `json:"completionTimestamp,omitempty"`

	// PluginSpecific are a map of key-value pairs that plugin want to provide
	// to user to identify plugin properties related to this backup
	// +optional
	PluginSpecific map[string]string `json:"pluginSpecific,omitempty"`

	// Progress holds the total number of bytes of the volume and the current
	// number of backed up bytes. This can be used to display progress information
	// about the backup operation.
	// +optional
	Progress VolumeOperationProgress `json:"progress,omitempty"`
}

type VolumeOperationProgress struct {
	TotalBytes int64
	BytesDone int64
}

type VolumePluginBackup struct {
	metav1.TypeMeta `json:",inline"`

	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// +optional
	Spec VolumePluginBackupSpec `json:"spec,omitempty"`

	// +optional
	Status VolumePluginBackupStatus `json:"status,omitempty"`
}
```

For every backup operation of volume, volume snapshotter creates VolumePluginBackup CR in Velero namespace.
It keep updating the progress of operation along with other details like Volume name, Backup Name, SnapshotID etc as mentioned in the CR.

In order to know the CR created for the particular backup of a volume, volume snapshotters adds following labels to CR:
- `velero.io/backup-name` with value as Backup Name, and,
- `velero.io/volume-name` with value as volume that is undergoing backup

Backup name being unique won't cause issues like duplicates in identifying the CR.

Plugin need to sanitize the value that can be set for above labels. Label need to be set with the value returned from `GetValidName` function. (https://github.com/vmware-tanzu/velero/blob/main/pkg/label/label.go#L35).

Though no restrictions are required on the name of CR, as a general practice, volume snapshotter can name this CR with the value same as return value of CreateSnapshot.

After return from `CreateSnapshot` in `takePVSnapshot`, if VolumePluginBackup CR exists for particular backup of the volume, velero adds this CR to `backupRequest`.
During persistBackup call, this CR also will be backed up to backup location.

In backupSyncController, it checks for any VolumePluginBackup CRs that need to be synced from backup location, and syncs them to cluster if needed.

`processRequest` of `backupDeletionController` will perform deletion of VolumePluginBackup before volumesnapshotter's DeleteSnapshot is called.

Another alternative is:
Deletion of `VolumePluginBackup` CR can be delegated to plugin. Plugin can perform deletion of VolumePluginBackup using the `snapshotID` passed in volumesnapshotter's DeleteSnapshot request.

### 'core' Velero client/server required changes

- Creation of the VolumePluginBackup/VolumePluginRestore CRDs at installation time
- Persistence of VolumePluginBackup CRs towards the end of the back up operation
- As part of backup synchronization, VolumePluginBackup CRs related to the backup will be synced.
- Deletion of VolumePluginBackup when volumeshapshotter's DeleteSnapshot is called
- Deletion of VolumePluginRestore as part of handling deletion of Restore CR
- In case of approach 1,
  - converting `volume.Snapshot` struct as CR and its related changes
  - creation of VolumePlugin(Backup|Restore) CRs before calling volumesnapshotter's API
  - `GetBackupVolumeSnapshots` and its callers related changes for change in return type from []volume.Snapshot to []VolumePluginBackupSpec.

### Velero CLI required changes

In 'velero describe' CLI, required CRs will be fetched from API server and its contents like backupName, PVName (if changed due to label size limitation), size of PV snapshot will be shown in the output.

### API Upgrade
When CRs gets upgraded, velero can support older API versions also (till they get deprecated) to identify the CRs that need to be persisted to backup location.
However, it can provide preference over latest supported API.

If new fields are added without changing API version, it won't cause any problem as these resources are intended to provide information, and, there is no reconciliation on these resources.

### Compatibility of latest plugin with older version of Velero
Plugin that supports this CR should handle the situation gracefully when CRDs are not installed. It can handle the errors occurred during creation/updation of the CRs.

## Limitations:

Non K8s native plugins will not be able to implement this as they can not create the CRs.

## Open Questions

## Alternatives Considered

### Add another method to VolumeSnapshotter interface 
Above proposed approach have limitation that plugin need to be K8s native in order to create, update CRs.
Instead, a new method for 'Progress' will be added to interface. Velero server regularly polls this 'Progress' method and updates VolumePluginBackup CR on behalf of plugin.

But, this involves good amount of changes and needs a way for backward compatibility.

As volume plugins are mostly K8s native, its fine to go ahead with current limiation.

### Update Backup CR
Instead of creating new CRs, plugins can directly update the status of Backup CR. But, this deviates from current approach of having separate CRs like PodVolumeBackup/PodVolumeRestore to know operations progress.

### Restricting on name rather than using labels
Instead of using labels to identify the CR related to particular backup on a volume, restrictions can be placed on the name of VolumePluginBackup CR to be same as the value returned from CreateSnapshot.
But, this can cause issue when volume snapshotter just crashed without returning snapshot id to velero.

### Backing up VolumePluginBackup CR to different object
If CR is backed up to different object other than `#backup-volumesnapshots.json.gz` in backup location, restore controller need to follow 'fall-back model'.
It first need to check for new kind of object, and, if it doesn't exists, follow the old model. To avoid 'fall-back' model which prone to errors, VolumePluginBackup CR is backed to same location as that of `volume.Snapshot` location.

## Security Considerations

Currently everything runs under the same `velero` service account so all plugins have broad access, which would include being able to modify CRs created by another plugin.

