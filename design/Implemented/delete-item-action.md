# Delete Item Action Plugins

## Abstract

Velero should provide a way to delete items created during a backup, with a model and interface similar to that of BackupItemAction and RestoreItemAction plugins.
These plugins would be invoked when a backup is deleted, and would receive items from within the backup tarball.

## Background

As part of Container Storage Interface (CSI) snapshot support, Velero added a new pattern for backing up and restoring snapshots via BackupItemAction and RestoreItemAction plugins.
When others have tried to use this pattern, however, they encountered issues with deleting the resources made in their own ItemAction plugins, as Velero does not expose any sort of extension at backup deletion time.
These plugins largely seek to delete resources that exist outside of Kubernetes.
This design seeks to provide the missing extension point.

## Goals

- Provide a DeleteItemAction API for plugins to implement
- Update Velero backup deletion logic to invoke registered DeleteItemAction plugins.

## Non Goals

- Specific implementations of the DeleteItemAction API beyond test cases.
- Rollback of DeleteItemAction execution.

## High-Level Design

The DeleteItemAction plugin API will closely resemble the RestoreItemAction plugin design, in that plugins will receive the Velero `Backup` Go struct that is being deleted and a matching Kubernetes resource extracted from the backup tarball.

The Velero backup deletion process will be modified so that if there are any DeleteItemAction plugins registered, the backup tarball will be downloaded and extracted, similar to how restore logic works now.
Then, each item in the backup tarball will be iterated over to see if a DeleteItemAction plugin matches for it.
If a DeleteItemAction plugin matches, the `Backup` and relevant item will be passed to the DeleteItemAction.

The DeleteItemAction plugins will be run _first_ in the backup deletion process, before deleting snapshots from storage or `Restore`s from the Kubernetes API server.

DeleteItemAction plugins *cannot* rollback their actions.
This is because there is currently no way to recover other deleted components of a backup, such as volume/restic snapshots or other DeleteItemAction resources.

DeleteItemAction plugins will be run in alphanumeric order based on their registered names.

## Detailed Design

### New types

The `DeleteItemAction` interface is as follows:

```go
// DeleteItemAction is an actor that performs an action based on an item in a backup that is being deleted.
type DeleteItemAction interface {
	// AppliesTo returns information about which resources this action should be invoked for.
	// A DeleteItemAction's Execute function will only be invoked on items that match the returned
	// selector. A zero-valued ResourceSelector matches all resources.
    AppliesTo() (ResourceSelector, error)

	// Execute allows the ItemAction to perform arbitrary logic with the item being deleted.
    Execute(DeleteItemActionInput) error
}
```

The `DeleteItemActionInput` type is defined as follows:

```go
type DeleteItemActionInput struct {
	// Item is the item taken from the pristine backed up version of resource.
	Item runtime.Unstructured
	// Backup is the representation of the backup resource processed by Velero.
	Backup *api.Backup
}
```

Both `DeleteItemAction` and `DeleteItemActionInput` will be defined in `pkg/plugin/velero/delete_item_action.go`.

### Generate protobuf definitions and client/servers

In `pkg/plugin/proto`, add `DeleteItemAction.proto`.

Protobuf definitions will be necessary for:

```protobuf
message DeleteItemActionExecuteRequest {
	...
}

message DeleteItemActionExecuteResponse {
	...
}

message DeleteItemActionAppliesToRequest {
	...
}

message DeleteItemActionAppliesToResponse {
	...
}

service DeleteItemAction {
	rpc AppliesTo(DeleteItemActionAppliesToRequest) returns (DeleteItemActionAppliesToResponse)
	rpc Execute(DeleteItemActionExecuteRequest) returns (DeleteItemActionExecuteResponse)
}
```

Once these are written, then a client and server implementation can be written in `pkg/plugin/framework/delete_item_action_client.go` and `pkg/plugin/framework/delete_item_action_server.go`, respectively.
These should be largely the same as the client and server implementations for `RestoreItemAction` and `BackupItemAction` plugins.

### Restartable delete plugins

Similar to `RestoreItemAction` and `BackupItemAction` plugins, restartable processes will need to be implemented.

In `pkg/plugin/clientmgmt`, add `restartable_delete_item_action.go`, creating the following unexported type:

```go
type restartableDeleteItemAction struct {
	key kindAndName
	sharedPluginProcess RestartableProcess
	config map[string]string
}

// newRestartableDeleteItemAction returns a new restartableDeleteItemAction.
func newRestartableDeleteItemAction(name string, sharedPluginProcess RestartableProcess) *restartableDeleteItemAction {
	// ...
}

// getDeleteItemAction returns the delete item action for this restartableDeleteItemAction. It does *not* restart the
// plugin process.
func (r *restartableDeleteItemAction) getDeleteItemAction() (velero.DeleteItemAction, error) {
	// ...
}

// getDelegate restarts the plugin process (if needed) and returns the delete item action for this restartableDeleteItemAction.
func (r *restartableDeleteItemAction) getDelegate() (velero.DeleteItemAction, error) {
	// ...
}

// AppliesTo restarts the plugin's process if needed, then delegates the call.
func (r *restartableDeleteItemAction) AppliesTo() (velero.ResourceSelector, error) {
	// ...
}

// Execute restarts the plugin's process if needed, then delegates the call.
func (r *restartableDeleteItemAction) Execute(input *velero.DeleteItemActionInput) (error) {
	// ...
}
```

This file will be very similar in structure to 

### Plugin manager changes

Add the following methods to `pkg/plugin/clientmgmt/manager.go`'s `Manager` interface:

```go
type Manager interface {
	...
	// Get DeleteItemAction returns a DeleteItemAction plugin for name.
	GetDeleteItemAction(name string) (DeleteItemAction, error)

	// GetDeteteItemActions returns the all DeleteItemAction plugins.
	GetDeleteItemActions() ([]DeleteItemAction, error)
}
```

The unexported `manager` type should implement both the `GetDeleteItemAction` and `GetDeleteItemActions`.

Both of these methods should have the same exception for `velero.io/`-prefixed plugins that all other types do.

`GetDeleteItemAction` and `GetDeleteItemActions` will invoke the `restartableDeleteItemAction` implementations.


### Deletion controller modifications

`pkg/controller/backup_deletion_controller.go` will be updated to have plugin management invoked.

In `processRequest`, before deleting snapshots, get any registered `DeleteItemAction` plugins.
If there are none, proceed as normal.
If there are one or more, download the backup tarball from backup storage, untar it to temporary storage, and iterate through the items, matching them to the applicable plugins.

## Alternatives Considered

Another proposal for higher level `DeleteItemActions` was initially included, which would require implementors to individually download the backup tarball themselves.
While this may be useful long term, it is not a good fit for the current goals as each plugin would be re-implementing a lot of boilerplate.
See the deletion-plugins.md file for this alternative proposal in more detail.

The `VolumeSnapshotter` interface is not generic enough to meet the requirements here, as it is specifically for taking snapshots of block devices.

## Security Considerations

By their nature, `DeleteItemAction` plugins will be deleting data, which would normally be a security concern.
However, these will only be invoked in two situations: either when a `BackupDeleteRequest` is sent via a user with the `velero` CLI or some other management system, or when a Velero `Backup` expires by going over its TTL.
Because of this, the data deletion is not a concern.

## Compatibility

In terms of backwards compatibility, this design should stay compatible with most Velero installations that are upgrading.
If not DeleteItemAction plugins are present, then the backup deletion process should proceed the same way it worked prior to their inclusion.

## Implementation

The implementation dependencies are, roughly, in the order as they are described in the [Detailed Design](#detailed-design) section.

## Open Issues
