# Design for BackupItemAction v2 API

## Abstract
This design includes the changes to the BackupItemAction (BIA) api design as required by the [Item Action Progress Monitoring](general-progress-monitoring.md) feature.
The BIA v2 interface will have two new methods, and the Execute() return signature will be modified.
If there are any additional BIA API changes that are needed in the same Velero release cycle as this change, those can be added here as well.

## Background
This API change is needed to facilitate long-running plugin actions that may not be complete when the Execute() method returns.
It is an optional feature, so plugins which don't need this feature can simply return an empty operation ID and the new methods can be no-ops.
This will allow long-running plugin actions to continue in the background while Velero moves on to the next plugin, the next item, etc.

## Goals
- Allow for BIA Execute() to optionally initiate a long-running operation and report on operation status.

## Non Goals
- Allowing velero control over when the long-running operation begins.


## High-Level Design
As per the [Plugin Versioning](plugin-versioning.md) design, a new BIAv2 plugin `.proto` file will be created to define the GRPC interface.
v2 go files will also be created in `plugin/clientmgmt/backupitemaction` and `plugin/framework/backupitemaction`, and a new PluginKind will be created.
The velero Backup process will be modified to reference v2 plugins instead of v1 plugins.
An adapter will be created so that any existing BIA v1 plugin can be executed as a v2 plugin when executing a backup.

## Detailed Design

### proto changes (compiled into golang by protoc)

The v2 BackupItemAction.proto will be like the current v1 version with the following changes:
ExecuteResponse gets a new field:
```
message ExecuteResponse {
    bytes item = 1;
    repeated generated.ResourceIdentifier additionalItems = 2;
    string operationID = 3;
    repeated generated.ResourceIdentifier itemsToUpdate = 4;
}
```
The BackupItemAction service gets two new rpc methods:
```
service BackupItemAction {
    rpc AppliesTo(BackupItemActionAppliesToRequest) returns (BackupItemActionAppliesToResponse);
    rpc Execute(ExecuteRequest) returns (ExecuteResponse);
    rpc Progress(BackupItemActionProgressRequest) returns (BackupItemActionProgressResponse);
    rpc Cancel(BackupItemActionCancelRequest) returns (google.protobuf.Empty);
}
```
To support these new rpc methods, we define new request/response message types:
```
message BackupItemActionProgressRequest {
    string plugin = 1;
    string operationID = 2;
    bytes backup = 3;
}

message BackupItemActionProgressResponse {
    generated.OperationProgress progress = 1;
}

message BackupItemActionCancelRequest {
    string plugin = 1;
    string operationID = 2;
    bytes backup = 3;
}

```
One new shared message type will be added, as this will also be needed for v2 RestoreItemAction and VolmeSnapshotter:
```
message OperationProgress {
    bool completed = 1;
    string err = 2;
    int64 nCompleted = 3;
    int64 nTotal = 4;
    string operationUnits = 5;
    string description = 6;
    google.protobuf.Timestamp started = 7;
    google.protobuf.Timestamp updated = 8;
}
```

In addition to the two new rpc methods added to the BackupItemAction interface, there is also a new `Name()` method. This one is only actually used internally by Velero to get the name that the plugin was registered with, but it still must be defined in a plugin which implements BackupItemActionV2 in order to implement the interface. It doesn't really matter what it returns, though, as this particular method is not delegated to the plugin via RPC calls. The new (and modified) interface methods for `BackupItemAction` are as follows:
```
type BackupItemAction interface {
...
	Name() string
...
	Execute(item runtime.Unstructured, backup *api.Backup) (runtime.Unstructured, []velero.ResourceIdentifier, string, []velero.ResourceIdentifier, error)
	Progress(operationID string, backup *api.Backup) (velero.OperationProgress, error)
	Cancel(operationID string, backup *api.Backup) error
...
}
```

A new PluginKind, `BackupItemActionV2`, will be created, and the backup process will be modified to use this plugin kind.
See [Plugin Versioning](plugin-versioning.md) for more details on implementation plans, including v1 adapters, etc.


## Compatibility
The included v1 adapter will allow any existing BackupItemAction plugin to work as expected, with an empty operation ID returned from Execute() and no-op Progress() and Cancel() methods.

## Implementation
This will be implemented during the Velero 1.11 development cycle.
