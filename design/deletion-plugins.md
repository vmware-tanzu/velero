# Deletion Plugins

Status: Alternative Proposal


## Abstract
Velero should introduce a new type of plugin that runs when a backup is deleted.
These plugins will delete any external resources associated with the backup so that they will not be left orphaned.

## Background
With the CSI plugin, Velero developers introduced a pattern of using BackupItemAction and RestoreItemAction plugins tied to PersistentVolumeClaims to create other resources to complete a backup.
In the CSI plugin case, Velero does clean up of these other resources, which are Kubernetes Custom Resources, within the core Velero server.
However, for external plugins that wish to use this same pattern, this is not a practical solution.
Velero's core cannot be extended for all possible Custom Resources, and not external resources that get created are Kubernetes Custom Resources.

Therefore, Velero needs some mechanism that allows plugin authors who have created resources within a BackupItemAction or RestoreItemAction plugin to ensure those resources are deleted, regardless of what system those resources reside in.

## Goals
- Provide a new plugin type in Velero that is invoked when a backup is deleted.

## Non Goals
- Implementations of specific deletion plugins.
- Rollback of deletion plugin execution.


## High-Level Design
Velero will provide a new plugin type that is similar to its existing plugin architecture.
These plugins will be referred to as `DeleteAction` plugins.
`DeleteAction` plugins will receive the `Backup` CustomResource being deleted on execution.

`DeleteAction` plugins cannot prevent deletion of an item.
This is because multiple `DeleteAction` plugins can be registered, and this proposal does not include rollback and undoing of a deletion action.
Thus, if multiple `DeleteAction` plugins have already run but another would request the deletion of a backup stopped, the backup that's retained would be inconsistent.

`DeleteActions` will apply to `Backup`s based on a label on the `Backup` itself.
In order to ensure that `Backup`s don't execute `DeleteAction` plugins that are not relevant to them, `DeleteAction` plugins can register an `AppliesTo` function which will define a label selector on Velero backups.

`DeleteActions` will be run in alphanumerical order by plugin name.
This order is somewhat arbitrary, but will be used to give authors and users a somewhat predictable order of events.

## Detailed Design
The `DeleteAction` plugins will implement the following Go interface, defined in `pkg/plugin/velero/deletion_action.go`:

```go
type DeleteAction struct {

    // AppliesTo will match the DeleteAction plugin against Velero Backups that it should operate against.
    AppliesTo()

    // Execute runs the custom plugin logic and may connect to external services.
    Execute(backup *api.backup) error
}

```

The following methods would be added to the `clientmgmt.Manager` interface in `pkg/pluginclientmgmt/manager.go`:

```
type Manager interface {
    ...

    // GetDeleteActions returns the registered DeleteActions.
    //TODO: do we need to get these by name, or can we get them all?
    GetDeleteActions([]velero.DeleteAction, error)
    ...
```


## Alternatives Considered
TODO

## Security Considerations
TODO

## Compatibility
Backwards compatibility should be straight-forward; if there are no installed `DeleteAction` plugins, then the backup deletion process will proceed as it does today.

## Implementation
TODO

## Open Issues
In order to add a custom label to the backup, the backup must be modifiable inside of the `BackupItemActon` and `RestoreItemAction` plugins, which it currently is not. A work around for now is for the user to apply a label to the backup at creation time, but that is not ideal.
