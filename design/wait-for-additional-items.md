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

At this point, if the `RestoreItemActionExecuteOutput` includes a
non-nil `AdditionalItemsReadyFunc` we need to call a func similar to
`crdAvailable` which we will call `itemsAvailable`
https://github.com/vmware-tanzu/velero/blob/main/pkg/restore/restore.go#L623
This func should also be defined within restore.go

Instead of the one minute CRD timeout, we'll use a timeout specific to
waiting for additional items. There will be a new field added to
serverConfig, `additionalItemsReadyTimeout`, with a
`defaultAdditionalItemsReadyTimeout` const set to 10 minutes. In addition,
each plugin will be able to define an override for the global
server-level value, which will be added as another optional field in
the `RestoreItemActionExecuteOutput` struct. Instead of the
`IsUnstructuredCRDReady` call, we'll call the returned
`AdditionalItemsReadyFunc` passing in the same `AdditionalItems` slice
as an argument (with items which failed to restore filtered out). If
this func returns an error, then `itemsAvailable` will
propagate the error, and `restoreItem` will handle it the same way it
handles an error return on restoring an additional item. If the
timeout is reached without ready returning true, velero will continue
on to attempt restore of the current item.

### `RestoreItemActionExecuteOutput` changes

Two new fields will be added to `RestoreItemActionExecuteOutput`, both
optional. `AdditionalItemsReadyTimeout`, if specified, will override
`serverConfig.additionalItemsReadyTimeout`. If the new func field
`AdditionalItemsReadyFunc` is non-nil, then `restoreItem` will call
`itemsAvailable` which will invoke the plugin func
`AdditionalItemsReadyFunc` and wait until the func returns true or the
timeout is reached. If `AdditionalItemsReadyFunc` is nil (the default
case), then current velero behavior will be followed. Existing plugins
which do not need to signal to wait for `AdditionalItems` won't need
to change their `Execute()` functions.

In addition, a new func, `WithItemsWait(readyFunc *func)` will
be added to `RestoreItemActionExecuteOutput` similar to
`WithoutRestore()` which will set `AdditionalItemsReadyFunc` to
`readyfunc`. This will allow a plugin to include waiting for
AdditionalItems like this:
```
func AreItemsReady (restore *api.Restore, additionalItems []ResourceIdentifier) (bool, error) {
	...
	return true, nil
}
func (p *RestorePlugin) Execute(input *velero.RestoreItemActionExecuteInput) (*velero.RestoreItemActionExecuteOutput, error) {
	...
	return velero.NewRestoreItemActionExecuteOutput(input.Item).WithItemsWait(AreItemsReady), nil
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

	// AdditionalItemsReadyFunc is a func which returns true if
	// the additionalItems passed into the func are
	// ready/available. A nil value for this func means that
	// velero will not wait for the items to be ready before
	// attempting to restore the current item.
	AdditionalItemsReadyFunc func(restore *api.Restore, []ResourceIdentifier) (bool, error)

	// AdditionalItemsReadyTimeout will override serverConfig.additionalItemsReadyTimeout
	// if specified. This value specifies how long velero will wait
	// for additional items to be ready before moving on.
	AdditionalItemsReadyTimeout *time.Duration
}

// WithoutRestore returns SkipRestore for RestoreItemActionExecuteOutput
func (r *RestoreItemActionExecuteOutput) WithItemsWait(
	readyFunc func(*api.Restore, []ResourceIdentifier)
) *RestoreItemActionExecuteOutput {
	r.AdditionalItemsReadyFunc = readyFunc
	return r
}

```

### Earlier iteration (no longer the current implementation plan)

What follows is the first iteration of the design. Everything from
here is superseded by the content above. The options below require
either breaking backwards compatibility or dealing with runtime
casting and optional interfaces. Adding the func pointer to
`RestoreItemActionExecuteOutput` resolves the problem without
requiring either.

#### `RestoreItemActionExecuteOutput` changes

A new boolean field will be added to
`RestoreItemActionExecuteOutput`. If `WaitForAdditionalItems` is true,
then `restoreItem` will call `itemsAvailable` which will invoke the
plugin func `AreAdditionalItemsReady` and wait until the func returns
true or the timeout is reached. If `WaitForAdditionalItems` is false
(the default case), then current velero behavior will be
followed. Existing plugins which do not need to signal to wait for
`AdditionalItems` won't need to change their `Execute()` functions.

In addition, a new func, `WithItemsWait()` will be added to
`RestoreItemActionExecuteOutput` similar to `WithoutRestore()` which
will set the `WaitForAdditionalItems` bool to `true`.

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
}
```

#### `RestoreItemAction` plugin interface changes

In order to implement the `AreAdditionalItemsReady` plugin func, there
are two different approaches we could take.

The first would be to simply add another entry to the
`RestoreItemAction` interface:
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
	// are ready. Returns true if all items are ready, and false otherwise
	AreAdditionalItemsReady(restore *api.Restore, AdditionalItems []ResourceIdentifier) (bool, error)
}
```

The downside of this approach is that it is not backwards compatible,
and every `RestoreItemAction` plugin will have to implement the new
func, simply to return `true` in most cases, since the plugin will
either never return `AdditionalItems` from Execute or not have any
special readiness requirements.

The alternative to this would be to define an additional interface for
the optional func, leaving the `RestoreItemAction` interface alone.
```
type RestoreItemActionReadyCheck interface {
	// AreAdditionalItemsReady allows the ItemAction to communicate whether the passed-in 
	// slice of AdditionalItems (previously returned by Execute())
	// are ready. Returns true if all items are ready, and false otherwise
	AreAdditionalItemsReady(restore *api.Restore, AdditionalItems []ResourceIdentifier) (bool, error)
}

```
In this case, existing plugins which do not need this functionality
can remain as-is, while plugins which want to make use of this
functionality will just need to implement the optional func. With the
optional interface approach, `itemsAvailable` will only wait if the
plugin can be type-asserted to the new interface:
```
	if actionWithReadyCheck, ok := action.(RestoreItemActionReadyCheck); ok {
		// wait for ready/timeout
	} else {
		return true, nil
	}
```

