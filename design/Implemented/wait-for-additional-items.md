# Wait for AdditionalItems to be ready on Restore

When a velero `RestoreItemAction` plugin returns a list of resources
via `AdditionalItems`, velero restores these resources before
restoring the current resource. There is a race condition here, as it
is possible that after running the restore on these returned items,
the current item's restore might execute before the additional items
are available. Depending on the nature of the dependency between the
current item and the additional items, this could cause the restore of
the current item to fail.

## Goals

- Enable Velero to ensure that Additional items returned by a restore
  plugin's `Execute` func are ready before restoring the current item
- Augment the RestoreItemAction plugin interface to allow the plugins
  to determine when an additional item is ready, since doing so
  requires knowledge specific to the resource type.

## Background

Because Velero does not wait after restoring additional items to
restore the current item, in some cases the current item restore will
fail if the additional items are not yet ready. Velero (and the
`RestoreItemAction` plugins) need to implement this "wait until ready"
functionality.

## High-Level Design

After each RestoreItemAction execute call (and following restore of
any returned additional items) , we need to wait for these returned
Additional Items to be ready before restoring the current item. In
order to do this, we also need to extend the `RestoreItemActionExecuteOutput`
struct to allow the plugin which returned additional items to
determine whether they are ready.

## Detailed Design

### `restoreItem` Changes

When each `RestoreItemAction` `Execute()` call returns, the
`RestoreItemActionExecuteOutput` struct contains a slice of
`AdditionalItems` which must be restored before this item can be
restored. After restoring these items, Velero needs to be able to wait
for them to be ready before moving on to the next item. Right after
looping over the additional items at
https://github.com/vmware-tanzu/velero/blob/main/pkg/restore/restore.go#L960-L991

we still have a reference to the additional items (`GroupResource` and
namespaced name), as well as a reference to the `RestoreItemAction`
plugin which required it.

At this point, if the `RestoreItemActionExecuteOutput`
`WaitForAdditionalItems` field is set to `true` we need to call a func
similar to `crdAvailable` which we will call `itemsAvailable`
https://github.com/vmware-tanzu/velero/blob/main/pkg/restore/restore.go#L623
This func should also be defined within restore.go

Instead of the one minute CRD timeout, we'll use a timeout specific to
waiting for additional items. There will be a new field added to
serverConfig, `additionalItemsReadyTimeout`, with a
`defaultAdditionalItemsReadyTimeout` const set to 10 minutes. In
addition, each plugin will be able to define an override for the
global server-level value, which will be added as another optional
field in the `RestoreItemActionExecuteOutput` struct. Instead of the
`IsUnstructuredCRDReady` call, we'll call `AreAdditionalItemsReady` on
the plugin, passing in the same `AdditionalItems` slice as an argument
(with items which failed to restore filtered out). If this func
returns an error, then `itemsAvailable` will propagate the error, and
`restoreItem` will handle it the same way it handles an error return
on restoring an additional item. If the timeout is reached without
ready returning true, velero will continue on to attempt restore of
the current item.

### `RestoreItemAction` plugin interface changes

In order to implement the `AreAdditionalItemsReady` plugin func, a new
function will be added to the `RestoreItemAction` interface.
```
type RestoreItemAction interface {
        // AppliesTo returns information about which resources this action should be invoked for.
        // A RestoreItemAction's Execute function will only be invoked on items that match the returned
        // selector. A zero-valued ResourceSelector matches all resources.
        AppliesTo() (ResourceSelector, error)

        // Execute allows the ItemAction to perform arbitrary logic with the item being restored,
        // including mutating the item itself prior to restore. The item (unmodified or modified)
        // should be returned, along with an optional slice of ResourceIdentifiers specifying additional
        // related items that should be restored, a warning (which will be logged but will not prevent
        // the item from being restored) or error (which will be logged and will prevent the item
        // from being restored) if applicable.
        Execute(input *RestoreItemActionExecuteInput) (*RestoreItemActionExecuteOutput, error)

	// AreAdditionalItemsReady allows the ItemAction to communicate whether the passed-in
	// slice of AdditionalItems (previously returned by Execute())
	// are ready. Returns true if all items are ready, and false
	// otherwise. The second return value is an error string if an
	// error occurred.
	AreAdditionalItemsReady(restore *api.Restore, AdditionalItems []ResourceIdentifier) (bool, string)
}
```

### `RestoreItemActionExecuteOutput` changes

Two new fields will be added to `RestoreItemActionExecuteOutput`, both
optional. `AdditionalItemsReadyTimeout`, if non-zero, will override
`serverConfig.additionalItemsReadyTimeout`. If
`WaitForAdditionalItems` is true, then `restoreItem` will call
`itemsAvailable` which will invoke the plugin func
`AreAdditionalItemsReady` and wait until the func returns true or the
timeout is reached. If `WaitForAdditionalItems` is false (the default
case), then current velero behavior will be followed. Existing plugins
which do not need to signal to wait for `AdditionalItems` won't need
to change their `Execute()` functions.

In addition, a new func, `WithItemsWait()` will
be added to `RestoreItemActionExecuteOutput` similar to
`WithoutRestore()` which will set `WaitForAdditionalItems` to
true. This will allow a plugin to include waiting for
AdditionalItems like this:
```
func AreAdditionalItemsReady (restore *api.Restore, additionalItems []ResourceIdentifier) (bool, string) {
	...
	return true, ""
}
func (p *RestorePlugin) Execute(input *velero.RestoreItemActionExecuteInput) (*velero.RestoreItemActionExecuteOutput, error) {
	...
	return velero.NewRestoreItemActionExecuteOutput(input.Item).WithItemsWait(), nil
}

```


```
// RestoreItemActionExecuteOutput contains the output variables for the ItemAction's Execution function.
type RestoreItemActionExecuteOutput struct {
	// UpdatedItem is the item being restored mutated by ItemAction.
	UpdatedItem runtime.Unstructured

	// AdditionalItems is a list of additional related items that should
	// be restored.
	AdditionalItems []ResourceIdentifier

	// SkipRestore tells velero to stop executing further actions
	// on this item, and skip the restore step. When this field's
	// value is true, AdditionalItems will be ignored.
	SkipRestore bool

        // WaitForAdditionalItems determines whether velero will wait
	// until AreAdditionalItemsReady returns true before restoring
	// this item. If this field's value is true, then after restoring
	// the returned AdditionalItems, velero will not restore this item
	// until AreAdditionalItemsReady returns true or the timeout is
        // reached. Otherwise, AreAdditionalItemsReady is not called.
        WaitForAdditionalItems bool

	// AdditionalItemsReadyTimeout will override serverConfig.additionalItemsReadyTimeout
	// if specified. This value specifies how long velero will wait
	// for additional items to be ready before moving on.
	AdditionalItemsReadyTimeout time.Duration
}

// WithItemsWait returns RestoreItemActionExecuteOutput with WaitForAdditionalItems set to true.
func (r *RestoreItemActionExecuteOutput) WithItemsWait()
) *RestoreItemActionExecuteOutput {
	r.WaitForAdditionalItems = true
	return r
}

```

## New design iteration (Feb 2021)

In starting the implementation based on the originally approved
design, I've run into an unexpected snag. When adding the wait func
pointer to the `RestoreItemActionExecuteOutput` struct, I had
forgotten about the protocol buffer message format that's used for
passing args to the plugin methods. Funcs are predefined RPC calls
with autogenerated go code, so we can't just pass a regular golang
func pointer in the struct. I've modified the above design to instead
use an explicit `AreAdditionalItemsReady` func. Since this will break
backwards compatibility with current `RestoreItemAction` plugins,
implementation of this feature should wait until Velero plugin
versioning, as described in
https://github.com/vmware-tanzu/velero/issues/3285 is
implemented. With plugin versioning in place, existing (non-versioned
or 1.0-versioned) `RestoreItemAction` plugins which do not define
`AreAdditionalItemsReady` would be able to coexist with a
to-be-implemented `RestoreItemAction` plugin version 2.0 (or 1.1,
etc.) which defines this new interface method. Without plugin
versioning, implementing this feature would break all existing plugins
until they define `AreAdditionalItemsReady`.

Also note that when moving to the new plugin version, the vast
majority of plugins will probably not need to wait for additional
items. All they will need to do to react to this plugin interface
change would be to define the following in the plugin:

```
func AreAdditionalItemsReady (restore *api.Restore, additionalItems []ResourceIdentifier) (bool, string) {
	return true, ""
}
```

As long as they never set `WaitForAdditionalItems` to true, this
function won't be called anyway, but if it is called, there will be no
waiting, since it will always return true.
