# Progress reporting for backups and restores handled by plugin volume snapshotters

Users face difficulty in knowing the progress of backup/restore operations of volume snapshotters. This is very similar to the issues faced by users to know progress for restic backup/restore, like, estimation of operation, operation in-progress/hung etc.

Each plugin might be providing a way to know the progress, but, it need not uniform across the plugins.

Even though plugins provide the way to know the progress of backup operation, this information won't be available to user during restore time on the destination cluster.

So, apart from the issues like progress, status of operation, volume snapshotters have unique problems like
- not being uniform across plugins
- not knowing the backup information during retore operation
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

### Progress of restic operation handled by volume snapshotter

Progress will be updated by volume snapshotter in VolumePluginRestore CR which is specific to that restore operation.

## Detailed Design

### VolumePluginBackup CR

```
// VolumePluginBackupSpec is the specification for a VolumePluginBackup CR.
type VolumePluginBackupSpec struct {
	// Volume is the PV name to be backed up.
	Volume string `json:"volume"`

	// Backup name
	Backup string `json:"backup"`

	// Plugin name
	Plugin string `json:"plugin"`

	// PluginSpecific are a map of key-value pairs that plugin want to provide
	// to user to identify plugin properties related to this backup
	// +optional
	PluginSpecific map[string]string `json:"pluginSpecific,omitempty"`
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

For every backup operation of volume, Volume snapshotter creates VolumePluginBackup CR in Velero namespace with the name which will be same as return value from CreateSnapshot.
It keep updating the progress of operation along with other details like Volume name, Backup Name, SnapshotID etc as mentioned in the CR.

After return from `CreateSnapshot` in `takePVSnapshot`, if VolumePluginBackup CR of snapshotID exists, Velero adds this CR to backupRequest.
During persistBackup call, this CR also will be backed up to backup location.

In backupSyncController, it checks for any VolumePluginBackup CRs that need to be synced from backup location, and syncs them to cluster if needed.

### VolumePluginRestore CR

```
// VolumePluginRestoreSpec is the specification for a VolumePluginRestore CR.
type VolumePluginRestoreSpec struct {
	// SnapshotID is the identifier for the snapshot of the volume.
	// This will be used to relate with output in 'velero describe backup'
	SnapshotID string `json:"snapshotID"`

	// Backup name
	Backup string `json:"backup"`

	// Plugin name
	Plugin string `json:"plugin"`

	// PluginSpecific are a map of key-value pairs that plugin want to provide
	// to user to identify plugin properties related to this restore
	// +optional
	PluginSpecific map[string]string `json:"pluginSpecific,omitempty"`
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

For every restore operation of volume, Volume snapshotter creates VolumePluginRestore CR in Velero namespace.
There is no restriction on CR name as there is no need for Velero to persist this to backup location.

Plugin keep updating the progress of operation along with other details like Volume name, Backup Name, SnapshotID etc as mentioned in the CR.

##Limitations:

Non K8s native plugins will not be able to implement this as they can not create the CRs.

## Open Questions

- Do we need constraint on the name of VolumePluginBackup CR?

- Don't we need constraint on the name of VolumePluginRestore CR?

## Alternatives Considered

### Add another method to VolumeSnapshotter interface 
Above proposed approach have limitation that plugin need to be K8s native in order to create, update CRs.
Instead, a new method for 'Progress' will be added to interface. Velero server regularly polls this 'Progress' method and updates VolumePluginBackup CR on behalf of plugin.

But, this involves good amount of changes and needs a way for backward compability.

As volume plugins are mostly K8s native, its fine to go ahead with current limiation.

### Update Backup CR
Instead of creating new CRs, plugins can directly update the status of Backup CR. But, this deviates from current approach of having seperate CRs like PodVolumeBackup/PodVolumeRestore to know operations progress.

## Security Considerations

N/A
