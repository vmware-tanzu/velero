# Plugin Versioning

## Abstract
This proposal outlines an approach to support versioning of Velero's plugin APIs to enable changes to those APIs.
It will allow for backwards compatible changes to be made, such as the addition of new plugin methods, but also backwards incompatible changes such as method removal or method signature changes.


## Background
When changes are made to Velero’s plugin APIs, there is no mechanism for the Velero server to communicate the version of the API that is supported, or for plugins to communicate what version they implement.
This means that any modification to a plugin API is a backwards incompatible change as it requires all plugins which implement the API to update and implement the new method.

There are several components involved to use plugins within Velero.
From the perspective of the core Velero codebase, all plugin kinds (e.g. `ObjectStore`, `BackupItemAction`) are defined by a single API interface and all interactions with plugins are managed by a plugin manager which provides an implementation of the plugin API interface for Velero to use.

Velero communicates with plugins via gRPC.
The core Velero project provides a framework (using the [go-plugin project](https://github.com/hashicorp/go-plugin)) for plugin authors to use to implement their plugins which manages the creation of gRPC servers and clients.
Velero plugins import the Velero plugin library in order to use this framework.
When a change is made to a plugin API, it needs to be made to the Go interface used by the Velero codebase, and also to the rpc service definition which is compiled to form part of the framework.
As each plugin kind is defined by a single interface, when a plugin imports the latest version of the Velero framework, it will need to implement the new APIs in order to build and run successfully.
If a plugin does not use the latest version of the framework, and is used with a newer version of Velero that expects the plugin to implement those methods, this will result in a runtime error as the plugin is incompatible.

With this proposal, we aim to break this coupling and introduce plugin API versions.

## Scenarios to Support
The following describes interactions between Velero and its plugins that will be supported with the implementation of this proposal.
For the purposes of this list, we will refer to existing Velero and plugin versions as `v1` and all following versions as version `n`.

Velero client communicating with plugins or plugin client calling other plugins:

- Version `n` client will be able to communicate with Version `n` plugin
- Version `n` client will be able to communicate with all previous versions of the plugin (Version `n-1` back to `v1`)

Velero plugins importing Velero framework:
- `v1` plugin built against Version `n` Velero framework
    - A plugin may choose to only implement a `v1` API, but it must be able to be built using Version `n` of the Velero framework


## Goals

- Allow plugin APIs to change without requiring all plugins to implement the latest changes (even if they upgrade the version of Velero that is imported)
- Allow plugins to choose which plugin versions they support and enable them to support multiple versions
- Support breaking changes in the plugin APIs such as method removal or method signature changes
- Establish a design process for modifying plugin APIs such as method addition and removal and signature changes
- Establish a process for newer Velero clients to use older versions of a plugin API through adaptation

## Non Goals

- Change how plugins are managed or added
- Allow older plugin clients to communicate with new versions of plugins

## High-Level Design

With each change to a plugin API, a new version of the plugin interface and the proto service definition will be created which describes the new plugin API.
The plugin framework will be adapted to allow these new plugin versions to be registered.
Plugins can opt to implement any or all versions of an API, however Velero will always attempt to use the latest version, and the plugin management will be modified to adapt earlier versions of a plugin to be compatible with the latest API where possible.
Under the existing plugin framework, any new plugin version will be treated as a new plugin with a new kind.
The plugin manager (which provides implementations of a plugin to Velero) will include an adapter layer which will manage the different versions and provide the adaptation for versions which do not implement the latest version of the plugin API.
Providing an adaptation layer enables Velero and other plugin clients to use an older version of a plugin if it can be safely adapted.
As the plugins will be able to introduce backwards incompatible changes, it will _not_ be possible for older version of Velero to use plugins which only support the latest versions of the plugin APIs.

Although adding new rpc methods to a service is considered a backwards compatible change within gRPC, due to the way the proto definitions are compiled and included in the framework used by plugins, this will require every plugin to implement the new methods.
Instead, we are opting to treat the addition of a method to an API as one requiring versioning.

The addition of optional fields to existing structs which are used as parameters to or return values of API methods will not be considered as a change requiring versioning.
These kinds of changes do not modify method signatures and have been safely made in the past with no impact on existing plugins.

## Detailed Design

The following areas will need to be adapted to support plugin versioning.

### Plugin Interface Definitions

To provide versioned plugins, any change to a plugin interface (method addition, removal, or signature change) will require a new versioned interface to be created.
Currently, all plugin interface definitions reside in `pkg/plugin/velero` in a file corresponding to their plugin kind.
These files will be rearranged to be grouped by kind and then versioned: `pkg/plugin/velero/<plugin_kind>/<version>/`.

The following are examples of how each change may be treated:

#### Complete Interface Change
If the entire `ObjectStore` interface is being changed such that no previous methods are being included, a file would be added to `pkg/plugin/velero/objectstore/v2/` and would contain the new interface definition:

```
type ObjectStore interface {
	// Only include new methods that the new API version will support
	
	NewMethod()
	// ...
}
```

#### Method Addition
If a method is being added to the `ObjectStore` API, a file would be added to `pkg/plugin/velero/objectstore/v2/` and may contain a new API definition as follows:

```
import "github.com/vmware-tanzu/velero/pkg/plugin/velero/objectstore/v1"

type ObjectStore interface {
    // Import all the methods from the previous version of the API if they are to be included as is
    v1.ObjectStore

    // Provide definitions of any new methods
    NewMethod()
```

#### Method Removal
If a method is being removed from the `ObjectStore` API, a file would be added to `pkg/plugin/velero/objectstore/v2/` and may contain a new API definition as follows:

```
type ObjectStore interface {
	// Methods which are required from the previous API version must be included, for example
	Init(config)
	PutObject(bucket, key, body)
    // ...

    // Methods which are to be removed are not included
```

#### Method Signature modification
If a method signature in the `ObjectStore` API is being modified, a file would be added to `pkg/plugin/velero/objectstore/v2/` and may contain a new API definition as follows:

```
type ObjectStore interface {
	// Methods which are required from the previous API version must be included, for example
	Init(config)
	PutObject(bucket, key, body)
    // ...	

    // Provide new definitions for methods which are being modified
	List(bucket, prefix, newParameter)
	
}
```

### Proto Service Definitions

The proto service definitions of the plugins will also be versioned and arranged by their plugin kind.
Currently, all the proto definitions reside under `pkg/plugin/proto` in a file corresponding to their plugin kind.
These files will be rearranged to be grouped by kind and then versioned: `pkg/plugin/proto/<plugin_kind>/<version>`,
except for the current v1 plugins. Those will remain in their current package/location for backwards compatibility.
This will allow plugin images built with earlier versions of velero to work with the latest velero (for v1 plugins
only). The go_package option will be added to all proto service definitions to allow the proto compilation script
to place the generated go code for each plugin api version in the proper go package directory.

It is not possible to import an existing proto service into a new one, so any methods will need to be duplicated across versions if they are required by the new version.
The message definitions can be shared however, so these could be extracted from the service definition files and placed in a file that can be shared across all versions of the service.

### Plugin Framework

To allow plugins to register which versions of the API they implement, the plugin framework will need to be adapted to accept new versions.
Currently, the plugin manager stores a [`map[string]RestartableProcess`](https://github.com/vmware-tanzu/velero/blob/main/pkg/plugin/clientmgmt/manager.go#L69), where the string key is the binary name for the plugin process (e.g. "velero-plugin-for-aws").
Each `RestartableProcess` contains a [`map[kindAndName]interface{}`](https://github.com/vmware-tanzu/velero/blob/main/pkg/plugin/clientmgmt/restartable_process.go#L60) which represents each of the unique plugin implementations provided by that binary.
[`kindAndName`](https://github.com/vmware-tanzu/velero/blob/main/pkg/plugin/clientmgmt/registry.go#L42) is a struct which combines the plugin kind (`ObjectStore`, `VolumeSnapshotter`) and the plugin name ("velero.io/aws", "velero.io/azure").

Each plugin version registration must be unique (to allow for multiple versions to be implemented within the same plugin binary).
This will be achieved by adding a specific registration method for each version to the Server interface in the plugin framework.
For example, if adding a V2 `RestoreItemAction` plugin, the Server interface would be modified to add the `RegisterRestoreItemActionV2` method.
This would require [adding a new plugin Kind const](https://github.com/vmware-tanzu/velero/blob/main/pkg/plugin/framework/plugin_kinds.go#L28-L46) to represent the new plugin version, e.g. `PluginKindRestoreItemActionV2`.
It also requires the creation of a new implementation of the go-plugin interface ([example](https://github.com/vmware-tanzu/velero/blob/main/pkg/plugin/framework/object_store.go)) to support that version and use the generated gRPC code from the proto definition (including a client and server implementation).
The Server will also need to be adapted to recognize this new plugin Kind and to serve the new implementation.

Existing plugin Kind consts and registration methods will be left unchanged and will correspond to the current version of the plugin APIs (assumed to be v1).

### Plugin Manager

The plugin manager is responsible for managing the lifecycle of plugins.
It provides an interface which is used by Velero to retrieve an instance of a plugin kind with a specific name (e.g. `ObjectStore` with the name "velero.io/aws").
The manager contains a registry of all available plugins which is populated during the main Velero server startup.
When the plugin manager is requested to provide a particular plugin, it checks the registry for that plugin kind and name.
If it is available in the registry, the manager retrieves a `RestartableProcess` for the plugin binary, creating it if it does not already exist.
That `RestartableProcess` is then used by individual restartable implementations of a plugin kind (e.g. `restartableObjectStore`, `restartableVolumeSnapshotter`).

As new plugin versions are added, the plugin manager will be modified to always retrieve the latest version of a plugin kind.
This is to allow the remainder of the Velero codebase to assume that it will always interact with the latest version of a plugin.
If the latest version of a plugin is not available, it will attempt to fall back to previous versions and use an implementation adapted to the latest version if available.
It will be up to the author of new plugin versions to determine whether a previous version of a plugin can be adapted to work with the interface of the new version.

For each plugin kind, a new `Restartable<PluginKind>` struct will be introduced which will contain the plugin Kind and a function, `Get`, which will instantiate a restartable instance of that plugin kind and perform any adaptation required to make it compatible with the latest version.
For example, `RestartableObjectStore` or `RestartableVolumeSnapshotter`.
For each restartable plugin kind, a new function will be introduced which will return a slice of `Restartable<PluginKind>` objects, sorted by version in descending order.

The manager will iterate through the list of `Restartable<PluginKind>`s and will check the registry for the given plugin kind and name.
If the requested version is not found, it will skip and continue to iterate, attempting to fetch previous versions of the plugin kind.
Once the requested version is found, the `Get` function will be called, returning the restartable implementation of the latest version of that plugin Kind.

```
type RestartableObjectStore struct {
	kind framework.PluginKind
	
	// Get returns a restartable ObjectStore for the given name and process, wrapping if necessary
	Get func(name string, restartableProcess RestartableProcess) v2.ObjectStore
}

func (m *manager) restartableObjectStores() []RestartableObjectStore {
	return []RestartableObjectStore{
		{
			kind: framework.PluginKindObjectStoreV2,
			Get: newRestartableObjectStoreV2,
		},
		{
			kind: framework.PluginKindObjectStore,
			Get: func(name string, restartableProcess RestartableProcess) v2.ObjectStore {
				// Adapt the existing restartable v1 plugin to be compatible with the v2 interface
				return newAdaptedV1ObjectStore(newRestartableObjectStore(name, restartableProcess))
			},
		},
	}
}

// GetObjectStore returns a restartableObjectStore for name.
func (m *manager) GetObjectStore(name string) (v2.ObjectStore, error) {
	name = sanitizeName(name)

	for _, restartableObjStore := range m.restartableObjectStores() {
		restartableProcess, err := m.getRestartableProcess(restartableObjStore.kind, name)
		if err != nil {
			// Check if plugin was not found
			if errors.Is(err, &pluginNotFoundError{}) {
				continue
			}
			return nil, err
		}
		return restartableObjStore.Get(name, restartableProcess), nil
	}

	return nil, fmt.Errorf("unable to get valid ObjectStore for %q", name)
}
```

If the previous version is not available, or can not be adapted to the latest version, it should not be included in the `restartableObjectStores` slice.
This will result in an error being returned as is currently the case when a plugin implementation for a particular kind and provider can not be found.

There are situations where it may be beneficial to check at the point where a plugin API call is made whether it implements a specific version of the API.
This is something that can be addressed with future amendments to this design, however it does not seem to be necessary at this time.

#### Plugin Adaptation

When a new plugin API version is being proposed, it will be up to the author and the maintainer team to determine whether older versions of an API can be safely adapted to the latest version.
An adaptation will implement the latest version of the plugin API interface but will use the methods from the version that is being adapted.
In cases where the methods signatures remain the same, the adaptation layer will call through to the same method in the version being adapted.

Examples where an adaptation may be safe:
- A method signature is being changed to add a new parameter but the parameter could be optional (for example, adding a context parameter). The adaptation could call through to the method provided in the previous version but omit the parameter.
- A method signature is being changed to remove a parameter, but it is safe to pass a default value to the previous version. The adaptation could call through to the method provided in the previous version but use a default value for the parameter.
- A new method is being added but does not impact any existing behaviour of Velero (for example, a new method which will allow Velero to [wait for additional items to be ready](https://github.com/vmware-tanzu/velero/blob/main/design/wait-for-additional-items.md)). The adaptation would return a value which allows the existing behaviour to be performed.
- A method is being deleted as it is no longer used. The adaptation would call through to any methods which are still included but would omit the deleted method in the adaptation.

Examples where an adaptation may not be safe:
- A new method is added which is used to provide new critical functionality in Velero. If this functionality can not be replicated using existing plugin methods in previous API versions, this should not be adapted and instead the plugin manager should return an error indicating that the plugin implementation can not be found.

### Restartable Plugin Process

As new versions of plugins are added, new restartable implementations of plugins will also need to be created.
These are currently located within "pkg/plugin/clientmgmt" but will be rearranged to be grouped by kind and version like other plugin files.

## Versioning Considerations

It should be noted that if changes are being made to a plugin's API, it will only be necessary to bump the API version once within a release cycle, regardless of how many changes are made within that cycle.
This is because the changes will only be available to consumers when they upgrade to the next minor version of the Velero library.
New plugin API versions will not be introduced or backported to patch releases.

Once a new minor or major version of Velero has been released however, any further changes will need to follow the process above and use a new API version.

## Alternatives Considered

### Relying on gRPC’s backwards compatibility when adding new methods

One approach for adapting the plugin APIs would have been to rely on the fact that adding methods to gRPC services is a backwards compatible change.
This approach would allow older clients to communicate with newer plugins as the existing interface would still be provided.
This was considered but ruled out as our current framework would require any plugin that recompiles using the latest version of the framework to adapt to the new version.
Also, without specific versioned interfaces, it would require checking plugin implementations at runtime for the specific methods that are supported.

## Compatibility

This design doc aims to allow plugin API changes to be made in a manner that may provide some backwards compatibility.
Older versions of Velero will not be able to make use of new plugin versions however may continue to use previous versions of a plugin API if supported by the plugin.

All compatibility concerns are addressed earlier in the document.

## Implementation

This design document primarily outlines an approach to allow future plugin API changes to be made.
However, there are changes to the existing code base that will be made to allow plugin authors to more easily propose and introduce changes to these APIs.

* Plugin interface definitions (currently in `pkg/plugin/velero`) will be rearranged to be grouped by kind and then versioned: `pkg/plugin/velero/<plugin_kind>/<version>/`.
* Proto definitions (currently in `pkg/plugin/proto`) will be rearranged to be grouped by kind and then versioned: `pkg/plugin/proto/<plugin_kind>/<version>`.
  * This will also require changes to the `make update` build task to correctly find the new proto location and output to the versioned directories.

It is anticipated that changes to the plugin APIs will be made as part of the 1.9 release cycle.
To assist with this work, an additional follow-up task to the ones listed above would be to prepare a V2 version of each of the existing plugins.
These new versions will not yet provide any new API methods but will provide a layout for new additions to be made

## Open Issues
