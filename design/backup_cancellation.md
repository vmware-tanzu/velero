
# Backup Cancellation Design

## Abstract
- This proposal introduces user-initiated backup cancellation functionality to Velero, allowing users to abort running backups through a new `cancel` field in the backup specification
    > backup.Spec.Cancel

- The design addresses GitHub issues [#9189](https://github.com/vmware-tanzu/velero/issues/9189
) and [#2098](https://github.com/vmware-tanzu/velero/issues/2098)
    - It is currently not possible to delete an in-progress backup: the deletion controller blocks it
    - Cancellation flow would allow this to happen

## Background
- Currently, Velero lacks the ability to cancel running backups, leading to several critical issues

- When users accidentally submit broad backup jobs (e.g., forgot to narrow resource selectors), the system becomes blocked and scheduled jobs accumulate

- Additionally, the backup deletion controller prevents running backups from being deleted


## Goals
- Enable users to cancel running backups through a `cancel` field in the backup specification

- Cleanly cancel all associated async operations (BackupItemAction operations, DataUploads)

- Delete backup data on 
    - object storage, 
    - csi native snapshots, 
    - backup tarball etc

    while keeping backup logs and backup associated data for inspection

- Provide clear backup phase transitions (InProgress → Cancelling → Cancelled)

## Non Goals
- Cancelling backups that have already completed or failed

- Implementing cancellation for restore operations (future work)


## High-Level Design
- The solution introduces a new `cancel` boolean field to the backup specification that users can set to `true` to request cancellation

- Existing controllers `(backup_controller, backup_operations_controller, backup_finalizer_controller)` will check for this field, attempt to cancel async ops and then transition to the `Cancelling` phase

- A new dedicated backup cancellation controller will watch for backups in the `Cancelling` phase, trying to cleanup backup data


## Detailed Design

### API Changes
Add a new field to `BackupSpec`:
```go
type BackupSpec struct {
    // ... existing fields ...
    
    // Cancel indicates whether the backup should be cancelled.
    // When set to true, Velero will attempt to cancel all ongoing operations
    // and transition the backup to Cancelled phase.
    // +optional
    Cancel *bool `json:"cancel,omitempty"`
}
```

Add new backup phases to `BackupPhase`:
```go
const (
    // ... existing phases ...
    BackupPhaseCancelling BackupPhase = "Cancelling" 
    BackupPhaseCancelled  BackupPhase = "Cancelled"
)
```

### Controller Changes

#### Existing Controllers
`backup_controller`

`backup_operations_controller`

`backup_finalizer_controller`


#### New Backup Cancellation Controller

```

The controller will:
1. Watch for backups in `BackupPhaseCancelling`
2. Attempt to delete backup data
3. Set phase to `BackupPhaseCancelled`

### Cancellation Flow

#### BackupItemAction Operations
For operations with BackupItemAction v2 implementations (e.g., CSI PVC actions):
1. Controller calls `bia.Cancel(operationID, backup)`
2. CSI PVC action finds associated DataUpload and sets `du.Spec.Cancel = true`
3. Node-agent DataUpload controller handles actual cancellation
4. Operation marked as `OperationPhaseCanceled`

#### PodVolumeBackup Operations
BackupWithResolvers is atomic
If cancellation happens before the call, nothing happens, ItemBlocks or PodVolumeBackups
If after, PostHooks ensure that PodVolumeBackups are completed, so there is no cancellation here


## Alternatives Considered


### Alternative 1: Deletion-Based Cancellation
Using backup deletion as the cancellation mechanism instead of a cancel field.
This was rejected because it doesn't allow users to preserve the backup object for inspection after cancellation, and deletion has different semantic meaning.

### Alternative 2: Timeout-Based Automatic Cancellation
Automatically cancelling backups after a configurable timeout.
This was considered out of scope for the initial implementation as it addresses a different use case than user-initiated cancellation.

## Security Considerations
The cancel field requires the same RBAC permissions as updating other backup specification fields.
No additional security considerations are introduced as the cancellation mechanism reuses existing operation cancellation pathways that are already secured.

## Compatibility
The new `cancel` field is optional and defaults to nil/false, ensuring backward compatibility with existing backup specifications.
Existing backups will continue to work without modification.
The new backup phases (`Cancelling`, `Cancelled`) are additive and don't affect existing phase transitions.


**Future Work**:

- Replacing logic that blocks deletion of in-progress backups with the cancellation flow, followed by usual deletion for a terminal phase (Cancelled)

