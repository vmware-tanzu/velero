
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

- Provide clear backup phase transitions (WaitingForPluginOperations → FinalizingCancelled → Failed)

## Non Goals
- Cancelling backups that have already completed or failed

- Implementing cancellation for restore operations (future work)


## High-Level Design
- The solution introduces a new `cancel` boolean field to the backup specification that users can set to `true` to request cancellation

- The `backup_operations_controller` will detect cancellation requests, cancel ongoing async operations, and transition to `BackupPhaseFinalizingCancelled`

- The existing `backup_finalizer_controller` will handle cancelled backups by cleaning up backup data while preserving logs, then transitioning to `BackupPhaseFailed`


- Extend the backup request struct to include:
    - `Cancel` (bool): reflects whether cancellation has been requested.
    - `LastCancelCheck` (timestamp): records the last time cancellation status was checked.

- Cancellation check logic:
    1. If `LastCancelCheck` is zero, set it to the current time and continue as usual.
    2. If more than a configured interval (e.g., 1–3 seconds) has passed since `LastCancelCheck`:
        - Update `LastCancelCheck` to now.
        - Retrieve the latest backup object from the API.
        - Set `request.Cancel` to the current value of `backup.Spec.Cancel`.
    3. Always use the current value of `request.Cancel` to determine if cancellation is requested.

- If backup processing is concurrent (multi-threaded), protect access to `request.Cancel` and `LastCancelCheck` with read/write mutexes to avoid race conditions.
    
     


## Detailed Design

### API Changes
Add a new field to `BackupSpec`:
```go
type BackupSpec struct {
    // ... existing fields ...
    
    // Cancel indicates whether the backup should be cancelled.
    // When set to true, Velero will attempt to cancel all ongoing operations
    // and transition the backup to a cancelled state.
    // +optional
    Cancel bool `json:"cancel,omitempty"`
}
```

Add new backup phase to `BackupPhase`:
```go
const (
    // ... existing phases ...
    BackupPhaseFinalizingCancelled BackupPhase = "FinalizingCancelled"
)
```

### Controller Changes

#### Existing Controllers

**backup_operations_controller**

The backup operations controller will be modified to detect cancellation requests and cancel ongoing async operations:

1. **Cancellation Detection**: In `getBackupItemOperationProgress()`, check `backup.Spec.Cancel` alongside existing timeout logic
2. **Operation Cancellation**: When cancellation is requested, call `bia.Cancel(operationID, backup)` for all in-progress operations
3. **Phase Transition**: When all operations complete (either successfully, failed, or cancelled), transition to appropriate finalizing phase:
   - If `backup.Spec.Cancel == true` → `BackupPhaseFinalizingCancelled`
   - Otherwise → `BackupPhaseFinalizing` or `BackupPhaseFinalizingPartiallyFailed`

```go
// In getBackupItemOperationProgress()
if backup.Spec.Cancel {
    _ = bia.Cancel(operation.Spec.OperationID, backup)
    operation.Status.Phase = itemoperation.OperationPhaseFailed
    operation.Status.Error = "Backup cancelled by user"
    // ... mark as failed and continue
}

// In Reconcile() phase transition logic
if !stillInProgress {
    if backup.Spec.Cancel {
        backup.Status.Phase = velerov1api.BackupPhaseFinalizingCancelled
    } else if backup.Status.Phase == velerov1api.BackupPhaseWaitingForPluginOperations {
        backup.Status.Phase = velerov1api.BackupPhaseFinalizing
    } else {
        backup.Status.Phase = velerov1api.BackupPhaseFinalizingPartiallyFailed
    }
}
```

**backup_finalizer_controller**

The backup finalizer controller will handle cancelled backups differently from normal finalization:

1. **Detect Cancelled Finalization**: Check for `BackupPhaseFinalizingCancelled` phase
2. **Cleanup Backup Data**: Remove backup content, metadata, and snapshots from object storage
3. **Preserve Logs**: Keep backup logs for debugging and inspection
4. **Final Phase**: Transition to `BackupPhaseFailed` with appropriate failure reason

```go
switch backup.Status.Phase {
case velerov1api.BackupPhaseFinalizing:
    // Normal finalization - preserve everything
    backup.Status.Phase = velerov1api.BackupPhaseCompleted
    
case velerov1api.BackupPhaseFinalizingPartiallyFailed:
    // Partial failure finalization - preserve data
    backup.Status.Phase = velerov1api.BackupPhasePartiallyFailed
    
case velerov1api.BackupPhaseFinalizingCancelled:
    // Cancelled finalization - clean up data but preserve logs
    if err := cleanupCancelledBackupData(backupStore, backup.Name); err != nil {
        log.WithError(err).Error("Failed to cleanup cancelled backup data")
    }
    backup.Status.Phase = velerov1api.BackupPhaseFailed
    backup.Status.FailureReason = "Backup cancelled by user"
}
```

**backup_controller**

No changes required for the first implementation. Cancellation during the initial backup phase (before async operations) is out of scope.

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

We focus on async ops for this first pass


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

