# Pre-Backup, Post-Backup, Pre-Restore, and Post-Restore Action Plugin Hooks

## Abstract

Velero should provide a way to trigger actions before and after each backup and restore.
**Important**: These proposed plugin hooks are fundamentally different from the existing plugin hooks, BackupItemAction and RestoreItemAction, which are triggered per item during backup and restore, respectively.
The proposed plugin hooks are to be executed only once.

These plugin hooks will be invoked:

- PreBackupAction: plugin actions are executed after the backup object is created and validated but before the backup is being processed, more precisely _before_ function [c.backupper.Backup](https://github.com/vmware-tanzu/velero/blob/74476db9d791fa91bba0147eac8ec189820adb3d/pkg/controller/backup_controller.go#L590). If the PreBackupActions return an err, the backup object is not processed.
- PostBackupAction: plugin actions are executed after the backup finishes processing all items and volumes snapshots are completed, more precisely _after_ function [recordBackupMetrics](https://github.com/vmware-tanzu/velero/blob/74476db9d791fa91bba0147eac8ec189820adb3d/pkg/controller/backup_controller.go#L630). If the PostBackupActions return errors or warnings, these return statuses are counted towards to the backup Status object.
- PreRestoreAction: plugin actions are executed after the restore object is created and validated and backup object is fetched, more precisely in function `runValidatedRestore` _before_ function [c.restore.Restore](https://github.com/vmware-tanzu/velero/blob/7c75cd6cf854064c9a454e53ba22cc5881d3f1f0/pkg/controller/restore_controller.go#L479). If the PreRestoreActions return an err, the restore object is not processed.
- PostRestoreAction: plugin actions are executed after the restore finishes processing all items and volumes snapshots are restored, more precisely in function `runValidatedRestore` _after_ function [c.restore.Restore](https://github.com/vmware-tanzu/velero/blob/7c75cd6cf854064c9a454e53ba22cc5881d3f1f0/pkg/controller/restore_controller.go#L479). If the PostRestoreActions return errors or warnings, these return statuses are counted towards to the restore Status object.

## Background

Increasingly, Velero is employed for workload migrations across different Kubernetes clusters.
Using Velero for migrations requires an atomic operation involving a Velero backup on a source cluster followed by a Velero restore on a destination cluster.

It is common during these migrations to perform many actions inside and outside Kubernetes clusters.
**Attention**: these actions are not necessarily per resource item, but they are actions to be executed _once_ before and/or after the migration itself (remember, migration in this context is Velero Backup + Velero Restore).

One important use case driving this proposal is migrating stateful workloads at scale across different clusters/storage backends.
Today, Velero's Restic integration is the response for such use cases, but there are some limitations:

- Quiesce/unquiesce workloads: Pod hooks are useful for quiescing/unquiescing workloads, but platform engineers often do not have the luxury/visibility/time/knowledge to go through each pod in order to add specific commands to quiesce/unquiesce workloads.
- Orphan PVC/PV pairs: PVCs/PVs that do not have associated running pods are not backed up and consequently, are not migrated.

Aiming to address these two limitations, and separate from this proposal, we would like to write a Velero plugin that takes advantage of the proposed Pre-Backup plugin hook. This plugin will be executed _once_ (not per resource item) prior backup. It will scale down the applications setting `.spec.replicas=0` to all deployments, statefulsets, daemonsets, replicasets, etc. and will start a small-footprint staging pod that will mount all PVC/PV pairs. Similarly, we would like to write another plugin that will utilize the proposed Post-Restore plugin hook. This plugin will unquiesce migrated applications by killing the staging pod and reinstating original `.spec.replicas values` after the Velero restore is completed.

Other examples of plugins that can use the proposed plugin hooks are

- PostBackupAction: trigger a Velero Restore after a successful Velero backup (and complete the migration operation).
- PreRestoreAction: pre-expand the cluster's capacity via Cluster API to avoid starvation of cluster resources before the restore.
- PostRestoreAction: call actions to be performed outside Kubernetes clusters, such as configure a global load balancer (GLB) that enables the new cluster.

This design seeks to provide missing extension points. This proposal's scope is to only add the new plugin hooks, not the plugins themselves.

## Goals

- Provide PreBackupAction, PostBackupAction, PreRestoreAction, and PostRestoreAction APIs for plugins to implement.
- Update Velero backup and restore creation logic to invoke registered PreBackupAction and PreRestoreAction plugins before processing the backup and restore respectively.
- Update Velero backup and restore complete logic to invoke registered PostBackupAction and PostRestoreAction plugins after flagging the objects as completed.

## Non-Goals

- Specific implementations of the PreBackupAction, PostBackupAction, PreRestoreAction and PostRestoreAction API beyond test cases.

## High-Level Design

The PreBackupAction plugin API will resemble the BackupItemAction plugin hook design, but with the fundamental difference that it will receive only as input the Velero `Backup` object created.
It will not receive any resource list items because the backup is not yet running at that stage.
In addition, the `PreBackupAction` interface will only have an `Execute()` method since the plugin will be executed once per Backup creation, not per item.

The Velero backup controller will be modified so that if there are any PreBackupAction plugins registered, they will be executed after backup object is created and validated but they will execute prior to processing the backup items and volume snapshots. More precisely _before_ function [c.backupper.Backup](https://github.com/vmware-tanzu/velero/blob/74476db9d791fa91bba0147eac8ec189820adb3d/pkg/controller/backup_controller.go#L590). If the PreBackupActions return an err, the function `runBackup` returns it and backup object is not processed.

The PostBackupAction plugin API will resemble the BackupItemAction plugin design, but with the fundamental difference that it will receive only as input the Velero `Backup` object without any resource list items.
By this stage, the backup has already been executed, with items backed up and volumes snapshots processed. 
The `PostBackupAction` interface will only have an `Execute()` method since the plugin will be executed only once per Backup, not per item.

The Velero backup controller package will be modified.
If there are any PostBackupAction plugins registered, they will be executed as almost last step of Backup object, more precisely _after_ function [recordBackupMetrics](https://github.com/vmware-tanzu/velero/blob/74476db9d791fa91bba0147eac8ec189820adb3d/pkg/controller/backup_controller.go#L630) but before the backup logs are persisted on the gzip file. We want to capture the logs from the PostBackupActions as part of the backup object. If the PostBackupActions return errors or warnings, these return statuses are counted towards to the backup Status object on `backup.Status.Warnings` and `backup.Status.Errors`.

The PreRestoreAction plugin API will resemble the RestoreItemAction plugin design, but with the fundamental difference that it will receive only as input the Velero `Restore` object created.
It will not receive any resource list items because the restore has not yet been running at that stage.
In addition, the `PreRestoreAction` interface will only have an `Execute()` method since the plugin will be executed only once per Restore creation, not per item.

The Velero restore controller will be modified so that if there are any PreRestoreAction plugins registered, they will be executed after restore object is created and validated and backup object is fetched, more precisely in function `runValidatedRestore` _before_ function [c.restore.Restore](https://github.com/vmware-tanzu/velero/blob/7c75cd6cf854064c9a454e53ba22cc5881d3f1f0/pkg/controller/restore_controller.go#L479). If the PreRestoreActions return an err, the restore object is not processed. If the PreRestoreActions return an err, the function `runValidatedRestore` returns it and restore object is not processed.

The PostRestoreAction plugin API will resemble the RestoreItemAction plugin design, but with the fundamental difference that it will receive only as input the Velero `Restore` object without any resource list items.
At this stage, the restore has already been executed.
The `PostRestoreAction` interface will only have an `Execute()` method since the plugin will be executed only once per Restore, not per item.

The Velero restore controller package will be modified.
If any PostRestoreAction plugins are registered, they will be executed after the restore finishes processing all items and volumes snapshots are restored, more precisely in function `runValidatedRestore` _after_ function [c.restore.Restore](https://github.com/vmware-tanzu/velero/blob/7c75cd6cf854064c9a454e53ba22cc5881d3f1f0/pkg/controller/restore_controller.go#L479). If the PostRestoreActions return errors or warnings, these return statuses are counted towards to the restore Status object, on `restoreWarnings`, `restoreErrors`.


## Detailed Design

### New types

#### PreBackupAction

The `PreBackupAction` interface is as follows:

```go
// PreBackupAction provides a hook into the backup process before it begins.
type PreBackupAction interface {
	// Execute the PreBackupAction plugin providing it access to the Backup that
	// is being executed
    Execute(backup *api.Backup) error
}
```

`PreBackupAction` will be defined in `pkg/plugin/velero/pre_backup_action.go`.

#### PostBackupAction

The `PostBackupAction` interface is as follows:

```go
// PostBackupAction provides a hook into the backup process after it completes.
type PostBackupAction interface {
	// Execute the PostBackupAction plugin providing it access to the Backup that
	// has been completed
    Execute(backup *api.Backup) error
}
```

`PostBackupAction` will be defined in `pkg/plugin/velero/post_backup_action.go`.

#### PreRestoreAction

The `PreRestoreAction` interface is as follows:

```go
// PreRestoreAction provides a hook into the restore process before it begins.
type PreRestoreAction interface {
	// Execute the PreRestoreAction plugin providing it access to the Restore that
	// is being executed
    Execute(restore *api.Restore) error
}
```

`PreRestoreAction` will be defined in `pkg/plugin/velero/pre_restore_action.go`.

#### PostRestoreAction

The `PostRestoreAction` interface is as follows:

```go
// PostRestoreAction provides a hook into the restore process after it completes.
type PostRestoreAction interface {
	// Execute the PostRestoreAction plugin providing it access to the Restore that
	// has been completed
    Execute(restore *api.Restore) error
}
```

`PostRestoreAction` will be defined in `pkg/plugin/velero/post_restore_action.go`.

### Generate Protobuf Definitions and Client/Servers

In `pkg/plugin/proto`, add the following:

1. Protobuf definitions will be necessary for PreBackupAction in `pkg/plugin/proto/PreBackupAction.proto`.

```protobuf
message PreBackupActionExecuteRequest {
    ...
}

service PreBackupAction {
    rpc Execute(PreBackupActionExecuteRequest) returns (Empty)
}
```

Once these are written, then a client and server implementation can be written in `pkg/plugin/framework/pre_backup_action_client.go` and `pkg/plugin/framework/pre_backup_action_server.go`, respectively.

2. Protobuf definitions will be necessary for PostBackupAction in `pkg/plugin/proto/PostBackupAction.proto`.

```protobuf
message PostBackupActionExecuteRequest {
    ...
}

service PostBackupAction {
    rpc Execute(PostBackupActionExecuteRequest) returns (Empty)
}
```

Once these are written, then a client and server implementation can be written in `pkg/plugin/framework/post_backup_action_client.go` and `pkg/plugin/framework/post_backup_action_server.go`, respectively.

3. Protobuf definitions will be necessary for PreRestoreAction in `pkg/plugin/proto/PreRestoreAction.proto`.

```protobuf
message PreRestoreActionExecuteRequest {
    ...
}

service PreRestoreAction {
    rpc Execute(PreRestoreActionExecuteRequest) returns (Empty)
}
```

Once these are written, then a client and server implementation can be written in `pkg/plugin/framework/pre_restore_action_client.go` and `pkg/plugin/framework/pre_restore_action_server.go`, respectively.

4. Protobuf definitions will be necessary for PostRestoreAction in `pkg/plugin/proto/PostRestoreAction.proto`.

```protobuf
message PostRestoreActionExecuteRequest {
    ...
}

service PostRestoreAction {
    rpc Execute(PostRestoreActionExecuteRequest) returns (Empty)
}
```

Once these are written, then a client and server implementation can be written in `pkg/plugin/framework/post_restore_action_client.go` and `pkg/plugin/framework/post_restore_action_server.go`, respectively.

### Restartable Delete Plugins

Similar to the `RestoreItemAction` and `BackupItemAction` plugins, restartable processes will need to be implemented (with the difference that there is no `AppliedTo()` method).

In `pkg/plugin/clientmgmt/`, add

1. `restartable_pre_backup_action.go`, creating the following unexported type:

```go
type restartablePreBackupAction struct {
    key                 kindAndName
    sharedPluginProcess RestartableProcess
}

func newRestartablePreBackupAction(name string, sharedPluginProcess RestartableProcess) *restartablePreBackupAction {
    // ...
}

func (r *restartablePreBackupAction) getPreBackupAction() (velero.PreBackupAction, error) {
    // ...
}

func (r *restartablePreBackupAction) getDelegate() (velero.PreBackupAction, error) {
    // ...
}

// Execute restarts the plugin's process if needed, then delegates the call.
func (r *restartablePreBackupAction) Execute(input *velero.PreBackupActionInput) (error) {
    // ...
}
```

2. `restartable_post_backup_action.go`, creating the following unexported type:

```go
type restartablePostBackupAction struct {
    key                 kindAndName
    sharedPluginProcess RestartableProcess
}

func newRestartablePostBackupAction(name string, sharedPluginProcess RestartableProcess) *restartablePostBackupAction {
    // ...
}

func (r *restartablePostBackupAction) getPostBackupAction() (velero.PostBackupAction, error) {
    // ...
}

func (r *restartablePostBackupAction) getDelegate() (velero.PostBackupAction, error) {
    // ...
}

// Execute restarts the plugin's process if needed, then delegates the call.
func (r *restartablePostBackupAction) Execute(input *velero.PostBackupActionInput) (error) {
    // ...
}
```

3. `restartable_pre_restore_action.go`, creating the following unexported type:

```go
type restartablePreRestoreAction struct {
    key                 kindAndName
    sharedPluginProcess RestartableProcess
}

func newRestartablePreRestoreAction(name string, sharedPluginProcess RestartableProcess) *restartablePreRestoreAction {
    // ...
}

func (r *restartablePreRestoreAction) getPreRestoreAction() (velero.PreRestoreAction, error) {
    // ...
}

func (r *restartablePreRestoreAction) getDelegate() (velero.PreRestoreAction, error) {
    // ...
}

// Execute restarts the plugin's process if needed, then delegates the call.
func (r *restartablePreRestoreAction) Execute(input *velero.PreRestoreActionInput) (error) {
    // ...
}
```

4. `restartable_post_restore_action.go`, creating the following unexported type:

```go
type restartablePostRestoreAction struct {
    key                 kindAndName
    sharedPluginProcess RestartableProcess
}

func newRestartablePostRestoreAction(name string, sharedPluginProcess RestartableProcess) *restartablePostRestoreAction {
    // ...
}

func (r *restartablePostRestoreAction) getPostRestoreAction() (velero.PostRestoreAction, error) {
    // ...
}

func (r *restartablePostRestoreAction) getDelegate() (velero.PostRestoreAction, error) {
    // ...
}

// Execute restarts the plugin's process if needed, then delegates the call.
func (r *restartablePostRestoreAction) Execute(input *velero.PostRestoreActionInput) (error) {
    // ...
}
```

### Plugin Manager Changes

Add the following methods to the `Manager` interface in `pkg/plugin/clientmgmt/manager.go`:

```go
type Manager interface {
    ...
    // Get PreBackupAction returns a PreBackupAction plugin for name.
    GetPreBackupAction(name string) (PreBackupAction, error)

    // Get PreBackupActions returns the all PreBackupAction plugins.
    GetPreBackupActions() ([]PreBackupAction, error)

    // Get PostBackupAction returns a PostBackupAction plugin for name.
    GetPostBackupAction(name string) (PostBackupAction, error)

    // GetPostBackupActions returns the all PostBackupAction plugins.
    GetPostBackupActions() ([]PostBackupAction, error)

    // Get PreRestoreAction returns a PreRestoreAction plugin for name.
    GetPreRestoreAction(name string) (PreRestoreAction, error)

    // Get PreRestoreActions returns the all PreRestoreAction plugins.
    GetPreRestoreActions() ([]PreRestoreAction, error)

    // Get PostRestoreAction returns a PostRestoreAction plugin for name.
    GetPostRestoreAction(name string) (PostRestoreAction, error)

    // GetPostRestoreActions returns the all PostRestoreAction plugins.
    GetPostRestoreActions() ([]PostRestoreAction, error)

}
```

`GetPreBackupAction` and `GetPreBackupActions` will invoke the `restartablePreBackupAction` implementations.
`GetPostBackupAction` and `GetPostBackupActions` will invoke the `restartablePostBackupAction` implementations.
`GetPreRestoreAction` and `GetPreRestoreActions` will invoke the `restartablePreRestoreAction` implementations.
`GetPostRestoreAction` and `GetPostRestoreActions` will invoke the `restartablePostRestoreAction` implementations.

## Alternatives Considered

An alternative to these plugin hooks is to implement all the pre/post backup/restore logic _outside_ Velero.
In this case, one would need to write an external controller that works similar to what [Konveyor Crane](https://github.com/konveyor/mig-controller/blob/master/pkg/controller/migmigration/quiesce.go) does today when quiescing applications.
We find this a viable way, but we think that Velero users can benefit from Velero having greater embedded capabilities, which will allow users to write or load plugins extensions without relying on an external components.

## Security Considerations

The plugins will only be invoked if loaded per a user's discretion.
It is recommended to check security vulnerabilities before execution.

## Compatibility

In terms of backward compatibility, this design should stay compatible with most Velero installations that are upgrading.
If plugins are not present, then the backup/restore process should proceed the same way it worked before their inclusion.

## Implementation

The implementation dependencies are roughly in the order as they are described in the [Detailed Design](#detailed-design) section.

## Open Issues
