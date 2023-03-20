# Design for RestoreItemAction v2 API

## Abstract
This design includes the changes to the RestoreItemAction (RIA) api design as required by the [Item Action Progress Monitoring](general-progress-monitoring.md) feature.
It also includes changes as required by the [Wait For Additional Items](wait-for-additional-items.md) feature.
The BIA v2 interface will have three new methods, and the RestoreItemActionExecuteOutput() struct in the return from Execute() will have three optional fields added.
If there are any additional RIA API changes that are needed in the same Velero release cycle as this change, those can be added here as well.

## Background
This API change is needed to facilitate long-running plugin actions that may not be complete when the Execute() method returns.
It is an optional feature, so plugins which don't need this feature can simply return an empty operation ID and the new methods can be no-ops.
This will allow long-running plugin actions to continue in the background while Velero moves on to the next plugin, the next item, etc.
The other change allows Velero to wait until newly-restored AdditionalItems returned by a RIA plugin are ready before moving on to restoring the current item.

## Goals
- Allow for RIA Execute() to optionally initiate a long-running operation and report on operation status.
- Allow for RIA to allow Velero to call back into the plugin to wait until AdditionalItems are ready before continuing with restore.

## Non Goals
- Allowing velero control over when the long-running operation begins.


## High-Level Design
As per the [Plugin Versioning](plugin-versioning.md) design, a new RIAv2 plugin `.proto` file will be created to define the GRPC interface.
v2 go files will also be created in `plugin/clientmgmt/restoreitemaction` and `plugin/framework/restoreitemaction`, and a new PluginKind will be created.
Changes to RestoreItemActionExecuteOutput will be made to the existing struct.
Since the new fields are optional elements of the struct, the new enlarged struct will work with both v1 and v2 plugins.
The velero Restore process will be modified to reference v2 plugins instead of v1 plugins.
An adapter will be created so that any existing RIA v1 plugin can be executed as a v2 plugin when executing a restore.

## Detailed Design

### proto changes (compiled into golang by protoc)

The v2 RestoreItemAction.proto will be like the current v1 version with the following changes:
RestoreItemActionExecuteOutput gets three new fields (defined in the current (v1) RestoreItemAction.proto file:
```
message RestoreItemActionExecuteResponse {
    bytes item = 1;
    repeated ResourceIdentifier additionalItems = 2;
    bool skipRestore = 3;
    string operationID = 4;
    bool waitForAdditionalItems = 5;
    google.protobuf.Duration additionalItemsReadyTimeout = 6;
}

```
The RestoreItemAction service gets three new rpc methods:
```
service RestoreItemAction {
    rpc AppliesTo(RestoreItemActionAppliesToRequest) returns (RestoreItemActionAppliesToResponse);
    rpc Execute(RestoreItemActionExecuteRequest) returns (RestoreItemActionExecuteResponse);
    rpc Progress(RestoreItemActionProgressRequest) returns (RestoreItemActionProgressResponse);
    rpc Cancel(RestoreItemActionCancelRequest) returns (google.protobuf.Empty);
    rpc AreAdditionalItemsReady(RestoreItemActionItemsReadyRequest) returns (RestoreItemActionItemsReadyResponse);
}

```
To support these new rpc methods, we define new request/response message types:
```
message RestoreItemActionProgressRequest {
    string plugin = 1;
    string operationID = 2;
    bytes restore = 3;
}

message RestoreItemActionProgressResponse {
    generated.OperationProgress progress = 1;
}

message RestoreItemActionCancelRequest {
    string plugin = 1;
    string operationID = 2;
    bytes restore = 3;
}

message RestoreItemActionItemsReadyRequest {
    string plugin = 1;
    bytes restore = 2;
    repeated ResourceIdentifier additionalItems = 3;
}
message RestoreItemActionItemsReadyResponse {
    bool ready = 1;
}

```
One new shared message type will be needed, as defined in the v2 BackupItemAction design:
```
message OperationProgress {
    bool completed = 1;
    string err = 2;
    int64 completed = 3;
    int64 total = 4;
    string operationUnits = 5;
    string description = 6;
    google.protobuf.Timestamp started = 7;
    google.protobuf.Timestamp updated = 8;
}
```

In addition to the three new rpc methods added to the RestoreItemAction interface, there is also a new `Name()` method. This one is only actually used internally by Velero to get the name that the plugin was registered with, but it still must be defined in a plugin which implements RestoreItemActionV2 in order to implement the interface. It doesn't really matter what it returns, though, as this particular method is not delegated to the plugin via RPC calls. The new (and modified) interface methods for `RestoreItemAction` are as follows:
```
type BackupItemAction interface {
...
	Name() string
...
	Progress(operationID string, restore *api.Restore) (velero.OperationProgress, error)
	Cancel(operationID string, backup *api.Restore) error
	AreAdditionalItemsReady(AdditionalItems []velero.ResourceIdentifier, restore *api.Restore) (bool, error)
...
}
type RestoreItemActionExecuteOutput struct {
	UpdatedItem            runtime.Unstructured
	AdditionalItems        []ResourceIdentifier
	SkipRestore            bool
	OperationID            string
	WaitForAdditionalItems bool
}

```

A new PluginKind, `RestoreItemActionV2`, will be created, and the restore process will be modified to use this plugin kind.
See [Plugin Versioning](plugin-versioning.md) for more details on implementation plans, including v1 adapters, etc.


## Compatibility
The included v1 adapter will allow any existing RestoreItemAction plugin to work as expected, with no-op AreAdditionalItemsReady(), Progress(), and Cancel() methods.

## Implementation
This will be implemented during the Velero 1.11 development cycle.
