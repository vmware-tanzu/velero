
# Backup Cancellation Design

## Abstract
This proposal introduces user-initiated backup cancellation functionality to Velero, allowing users to abort running backups through a new `cancel` field in the backup specification.
The design addresses GitHub issues [#9189](https://github.com/vmware-tanzu/velero/issues/9189
) and [#2098](https://github.com/vmware-tanzu/velero/issues/2098) by providing a mechanism to cleanly cancel async operations and prevent resource leaks when backups need to be terminated.

## Background
Currently, Velero lacks the ability to cancel running backups, leading to several critical issues.
When users accidentally submit broad backup jobs (e.g., forgot to narrow resource selectors), the system becomes blocked and scheduled jobs accumulate.
Additionally, the backup deletion controller prevents running backups from being deleted.


## Goals
- Enable users to cancel running backups through a `cancel` field in the backup specification
- Cleanly cancel all associated async operations (BackupItemAction operations, DataUploads, PodVolumeBackups)
- Provide clear backup phase transitions (InProgress → Cancelling → Cancelled)

## Non Goals
- Cancelling backups that have already completed or failed
- Rolling back partially completed backup operations
- Implementing cancellation for restore operations (future work)


## High-Level Design
The solution introduces a new `cancel` boolean field to the backup specification that users can set to `true` to request cancellation.
Existing controllers (backup_controller, backup_operations_controller, backup_finalizer_controller) will check for this field and transition the backup to a `Cancelling` phase before returning early from their reconcile loops.

A new dedicated backup cancellation controller will watch for backups in the `Cancelling` phase and coordinate the actual cancellation work.
This controller will call `Cancel()` methods on all in-progress BackupItemAction operations (which automatically handles DataUpload cancellation), directly cancel PodVolumeBackups by setting their cancel flags, and finally transition the backup to `Cancelled` phase.
The design uses a 5-second ticker to prevent API overload and ensures clean separation between cancellation detection and execution.

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
Modify `backup_controller.go`, `backup_operations_controller.go`, and `backup_finalizer_controller.go` to check for cancellation:
```go
// Early in each Reconcile method
if backup.Spec.Cancel != nil && *backup.Spec.Cancel {
    if backup.Status.Phase != BackupPhaseCancelling && backup.Status.Phase != BackupPhaseCancelled {
        backup.Status.Phase = BackupPhaseCancelling
        // Update backup and return
        return ctrl.Result{}, c.Client.Patch(ctx, backup, client.MergeFrom(original))
    }
    return ctrl.Result{}, nil // Skip processing for cancelling/cancelled backups
}
```
In addition, the `backup_operations_controller.go` will have a periodic check around backup progress updates, rather than running every time progress is updated to reduce API load.

#### New Backup Cancellation Controller
Create `backup_cancellation_controller.go`:
```go
type backupCancellationReconciler struct {
    client.Client
    logger                logrus.FieldLogger
    itemOperationsMap     *itemoperationmap.BackupItemOperationsMap
    newPluginManager      func(logger logrus.FieldLogger) clientmgmt.Manager
    backupStoreGetter     persistence.ObjectBackupStoreGetter
}
```

The controller will:
1. Watch for backups in `BackupPhaseCancelling`
2. Get operations from `itemOperationsMap.GetOperationsForBackup()`
3. Call `bia.Cancel(operationID, backup)` on all in-progress BackupItemAction operations
4. Find and cancel PodVolumeBackups by setting `pvb.Spec.Cancel = true`
5. Wait for all cancellations to complete
6. Set backup phase to `BackupPhaseCancelled`
7. Update backup metadata in object storage

### Cancellation Flow

#### BackupItemAction Operations
For operations with BackupItemAction v2 implementations (e.g., CSI PVC actions):
1. Controller calls `bia.Cancel(operationID, backup)`
2. CSI PVC action finds associated DataUpload and sets `du.Spec.Cancel = true`
3. Node-agent DataUpload controller handles actual cancellation
4. Operation marked as `OperationPhaseCanceled`

#### PodVolumeBackup Operations
For PodVolumeBackups (which lack BackupItemAction implementations):
1. Controller directly finds PVBs by backup UID label
2. Sets `pvb.Spec.Cancel = true` on in-progress PVBs
3. Node-agent PodVolumeBackup controller handles actual cancellation


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

## Implementation
Implementation will be done incrementally in the following phases:

**Phase 1**: API changes and basic cancellation detection
- Add `cancel` field to BackupSpec
- Add new backup phases
- Update existing controllers to detect cancellation and transition to `Cancelling` phase

**Phase 2**: Cancellation controller implementation
- Implement backup cancellation controller
- Add BackupItemAction operation cancellation
- Add PodVolumeBackup direct cancellation

**Phase 3**: Testing and refinement
- Comprehensive end-to-end testing
- Testing if slowdowns occur due to the frequency of checking `backup.Cancel` spec field
- Documentation and user guide updates

**Future Work**:

- Replacing logic that blocks deletion of in-progress backups with the cancellation flow, followed by usual deletion for a terminal phase (Cancelled)

