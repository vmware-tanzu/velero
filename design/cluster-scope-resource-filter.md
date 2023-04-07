# Proposal to add resource filters for backup can distinguish whether resource is cluster-scoped or namespace-scoped.

- [Proposal to add resource filters for backup can distinguish whether resource is cluster-scoped or namespace-scoped.](#proposal-to-add-resource-filters-for-backup-can-distinguish-whether-resource-is-cluster-scoped-or-namespace-scoped)
	- [Abstract](#abstract)
	- [Background](#background)
	- [Goals](#goals)
	- [Non Goals](#non-goals)
	- [High-Level Design](#high-level-design)
		- [Parameters Rules](#parameters-rules)
		- [Using scenarios:](#using-scenarios)
			- [no namespace-scoped resources + some cluster-scoped resources](#no-namespace-scoped-resources--some-cluster-scoped-resources)
			- [no namespace-scoped resources + all cluster-scoped resources](#no-namespace-scoped-resources--all-cluster-scoped-resources)
			- [some namespace-scoped resources + no cluster-scoped resources](#some-namespace-scoped-resources--no-cluster-scoped-resources)
				- [scenario 1](#scenario-1)
				- [scenario 2](#scenario-2)
				- [scenario 3](#scenario-3)
				- [scenario 4](#scenario-4)
			- [some namespace-scoped resources + only related cluster-scoped resources](#some-namespace-scoped-resources--only-related-cluster-scoped-resources)
				- [scenario 1](#scenario-1-1)
				- [scenario 2](#scenario-2-1)
				- [scenario 3](#scenario-3-1)
			- [some namespace-scoped resources + some additional cluster-scoped resources](#some-namespace-scoped-resources--some-additional-cluster-scoped-resources)
				- [scenario 1](#scenario-1-2)
				- [scenario 2](#scenario-2-2)
				- [scenario 3](#scenario-3-2)
				- [scenario 4](#scenario-4-1)
			- [some namespace-scoped resources + all cluster-scoped resources](#some-namespace-scoped-resources--all-cluster-scoped-resources)
				- [scenario 1](#scenario-1-3)
				- [scenario 2](#scenario-2-3)
				- [scenario 3](#scenario-3-3)
			- [all namespace-scoped resources + no cluster-scoped resources](#all-namespace-scoped-resources--no-cluster-scoped-resources)
			- [all namespace-scoped resources + some additional cluster-scoped resources](#all-namespace-scoped-resources--some-additional-cluster-scoped-resources)
			- [all namespace-scoped resources + all cluster-scoped resources](#all-namespace-scoped-resources--all-cluster-scoped-resources)
			- [describe command change](#describe-command-change)
	- [Detailed Design](#detailed-design)
	- [Alternatives Considered](#alternatives-considered)
	- [Security Considerations](#security-considerations)
	- [Compatibility](#compatibility)
	- [Implementation](#implementation)
	- [Open Issues](#open-issues)

## Abstract
The current filter (IncludedResources/ExcludedResources + IncludeClusterResources flag) is not enough for some special cases, e.g. all namespace-scoped resources + some kind of cluster-scoped resource and all namespace-scoped resources + cluster-scoped resource excludes.
Propose to add a new group of resource filtering parameters, which can distinguish cluster-scoped and namespace-scoped resources.

## Background
There are two sets of resource filters for Velero: `IncludedNamespaces/ExcludedNamespaces` and `IncludedResources/ExcludedResources`. 
`IncludedResources` means only including the resource types specified in the parameter. Both cluster-scoped and namespace-scoped resources are handled in this parameter by now.
The k8s resources are separated into cluster-scoped and namespace-scoped.
As a result, it's hard to include all resources in one group and only including specified resource in the other group.

## Goals
- Make Velero can support more complicated namespace-scoped and cluster-scoped resources filtering scenarios in backup.

## Non Goals
- Enrich the resource filtering rules, for example, advanced PV filtering and filtering by resource names.


## High-Level Design
Four new parameters are added into command `velero backup create`: `--include-cluster-scoped-resources`, `--exclude-cluster-scoped-resources`, `--include-namespace-scoped-resources` and `--exclude-namespace-scoped-resources`. 
`--include-cluster-scoped-resources` and `--exclude-cluster-scoped-resources` are used to filter cluster-scoped resources included or excluded in backup per resource type.
`--include-namespace-scoped-resources` and `--exclude-namespace-scoped-resources` are used to filter namespace-scoped resources included or excluded in backup per resource type.
Restore and other code pieces also use resource filtering will be handled in future releases.

### Parameters Rules

* `--include-cluster-scoped-resources`, `--include-namespace-scoped-resources`, `--exclude-cluster-scoped-resources` and `--exclude-namespace-scoped-resources` valid value include `*` and comma separated string. Each element of the CSV string should a k8s resource name. The format should be `resource.group`, such as `storageclasses.storage.k8s.io.`.

* `--include-cluster-scoped-resources`, `--include-namespace-scoped-resources`, `--exclude-cluster-scoped-resources` and `--exclude-namespace-scoped-resources` parameters are mutual exclusive with `--include-cluster-resources`, `--include-resources` and `--exclude-resources` parameters. If both sets of parameters are provisioned, validation failure should be returned.

* `--include-cluster-scoped-resources` and `--exclude-cluster-scoped-resources` should only contain cluster-scoped resource type names. If namespace-scoped resource type names are included, they are ignored.

* If there are conflicts between `--include-cluster-scoped-resources` and `--exclude-cluster-scoped-resources` specified resources type lists, `--exclude-cluster-scoped-resources` parameter has higher priority.

* `--include-namespace-scoped-resources` and `--exclude-namespace-scoped-resources` should only contain namespace-scoped resource type names. If cluster-scoped resource type names are included, they are ignored.

* If there are conflicts between `--include-namespace-scoped-resources` and `--exclude-namespace-scoped-resources` specified resources type lists, `--exclude-namespace-scoped-resources` parameter has higher priority.

* If  `--include-namespace-scoped-resources` is not present, it means all namespace-scoped resources are included per resource type.

* If both `--include-cluster-scoped-resources` and `--exclude-cluster-scoped-resources` are not present, it means no additional cluster-scoped resource is included per resource type, just as the existing `--include-cluster-resources` parameter not setting value. Cluster-scoped resources are related to the namespace-scoped resources, which means those are returned in the namespace-scoped resources' BackupItemAction's result AdditionalItems array, are still included in backup by default. Taking backing up PVC scenario as an example, PVC is namespace-scoped, PV is cluster-scoped. PVC's BIA will include PVC related PV into backup too.

### Using scenarios:
Please notice, if the scenario give the example of using old filtering parameters (`--include-cluster-resources`, `--include-resources` and `--exclude-resources`), that means the old parameters also work for this case. If old parameters example is not given, that means they don't work for this scenario, only new parameters (`--include-cluster-scoped-resources`, `--include-namespace-scoped-resources`, `--exclude-cluster-scoped-resources` and `--exclude-namespace-scoped-resources`) work.

#### no namespace-scoped resources + some cluster-scoped resources
The following command means backup no namespace-scoped resources and some cluster-scoped resources.

``` bash
velero backup create <backup-name>
	--exclude-namespace-scoped-resources=*
	--include-cluster-scoped-resources=storageclass
```

#### no namespace-scoped resources + all cluster-scoped resources
The following command means backup no namespace-scoped resources and all cluster-scoped resources.

``` bash
velero backup create <backup-name>
	--exclude-namespace-scoped-resources=*
	--include-cluster-scoped-resources=*
```

#### some namespace-scoped resources + no cluster-scoped resources
##### scenario 1
The following commands mean backup all resources in namespaces default and kube-system, and no cluster-scoped resources.

Example of new parameters:
``` bash
velero backup create <backup-name>
	--include-namespaces=default,kube-system
	--exclude-cluster-scoped-resources=*
```

Example of old parameters:
``` bash
velero backup create <backup-name>
	--include-namespaces=default,kube-system
	--include-cluster-resources=false
```
##### scenario 2
The following commands mean backup PVC, Deployment, Service, Endpoint, Pod and ReplicaSet resources in all namespaces, and no cluster-scoped resources. Although PVC's related PV should be included, due to no cluster-scoped resources are included, so they are ruled out too.

Example of new parameters:
``` bash
velero backup create <backup-name>
	--include-namespace-scoped-resources=persistentvolumeclaim,deployment,service,endpoint,pod,replicaset
	--exclude-cluster-scope-resources=*
```

Example of old parameters:
``` bash
velero backup create <backup-name>
	--include-resources=persistentvolumeclaim,deployment,service,endpoint,pod,replicaset
	--include-cluster-resources=false
```
##### scenario 3
The following commands mean backup PVC, Deployment, Service, Endpoint, Pod and ReplicaSet resources in namespace default and kube-system, and no cluster-scoped resources. Although PVC's related PV should be included, due to no cluster-scoped resources are included, so they are ruled out too.

Example of new parameters:
``` bash
velero backup create <backup-name>
	--include-namespaces=default,kube-system
	--include-namespace-scoped-resources=persistentvolumeclaim,deployment,service,endpoint,pod,replicaset
	--exclude-cluster-scope-resources=*
```

Example of old parameters:
``` bash
velero backup create <backup-name>
	--include-namespaces=default,kube-system
	--include-resources=persistentvolumeclaim,deployment,service,endpoint,pod,replicaset
	--include-cluster-resources=false
```
##### scenario 4
The following commands mean backup all resources except Ingress type resources in all namespaces, and no cluster-scoped resources.

Example of new parameters:
``` bash
velero backup create <backup-name>
	--exclude-namespace-scoped-resources=ingress
	--exclude-cluster-scoped-resources=*
```

Example of old parameters:
``` bash
velero backup create <backup-name>
	--exclude-resources=ingress
	--include-cluster-resources=false
```

#### some namespace-scoped resources + only related cluster-scoped resources
##### scenario 1
This means backup all resources in namespaces default and kube-system, and related cluster-scoped resources.
``` bash
velero backup create <backup-name>
	--include-namespaces=default,kube-system
```

##### scenario 2
This means backup pods and configmaps in namespaces default and kube-system, and related cluster-scoped resources.
``` bash
velero backup create <backup-name>
	--include-namespaces=default,kube-system
	--include-namespace-scoped-resources=pods,configmaps
```

##### scenario 3
This means backup all resources except Ingress type resources in all namespaces, and related cluster-scoped resources.

Example of new parameters:
``` bash
velero backup create <backup-name>
	--exclude-namespace-scoped-resources=ingress
```

Example of old parameters:
``` bash
velero backup create <backup-name>
	--exclude-resources=ingress
```

#### some namespace-scoped resources + some additional cluster-scoped resources
##### scenario 1
This means backup all resources in namespace in default, kube-system, and related cluster-scoped resources, plus all StorageClass resources.
``` bash
velero backup create <backup-name>
	--include-namespaces=default,kube-system
	--include-cluster-scoped-resources=storageclass
```

##### scenario 2
This means backup PVC, Deployment, Service, Endpoint, Pod and ReplicaSet resources in all namespaces, and related cluster-scoped resources, plus all StorageClass resources, and PVC related PV.
``` bash
velero backup create <backup-name>
	--include-namespace-scoped-resources=persistentvolumeclaim,deployment,service,endpoint,pod,replicaset
	--include-cluster-scoped-resources=storageclass
```

##### scenario 3
This means backup PVC, Deployment, Service, Endpoint, Pod and ReplicaSet resources in default and kube-system namespaces, and related cluster-scoped resources, plus all StorageClass resources, and PVC related PV.
``` bash
velero backup create <backup-name>
	--include-namespace-scoped-resources=persistentvolumeclaim,deployment,service,endpoint,pod,replicaset
	--include-namespaces=default,kube-system
	--include-cluster-scoped-resources=storageclass
```

##### scenario 4
This means backup PVC, Deployment, Service, Endpoint, Pod and ReplicaSet resources in default and kube-system namespaces, and related cluster-scoped resources, plus all cluster-scoped resources except StorageClass type resources.
``` bash
velero backup create <backup-name>
	--include-namespace-scoped-resources=persistentvolumeclaim,deployment,service,endpoint,pod,replicaset
	--include-namespaces=default,kube-system
	--exclude-cluster-scoped-resources=storageclass
```

#### some namespace-scoped resources + all cluster-scoped resources
##### scenario 1
The following commands mean backup all resources in namespace in default, kube-system, and all cluster-scoped resources.

Example of new parameters:
``` bash
velero backup create <backup-name>
	--include-namespaces=default,kube-system
	--include-cluster-scoped-resources=*
```

Example of old parameters:
``` bash
velero backup create <backup-name>
	--include-namespaces=default,kube-system
	--include-cluster-resources=true
```

##### scenario 2
This means backup Deployment, Service, Endpoint, Pod and ReplicaSet resources in all namespaces, and all cluster-scoped resources.
``` bash
velero backup create <backup-name>
	--include-namespace-scoped-resources=deployment,service,endpoint,pod,replicaset
	--include-cluster-scoped-resources=*
```

##### scenario 3
This means backup Deployment, Service, Endpoint, Pod and ReplicaSet resources in default and kube-system namespaces, and all cluster-scoped resources.
``` bash
velero backup create <backup-name>
	--include-namespaces=default,kube-system
	--include-namespace-scoped-resources=deployment,service,endpoint,pod,replicaset
	--include-cluster-scoped-resources=*
```

#### all namespace-scoped resources + no cluster-scoped resources
The following commands all mean backup all namespace-scoped resources and no cluster-scoped resources.

Example of new parameters:
``` bash
velero backup create <backup-name>
	--exclude-cluster-scoped-resources=*
```

Example of old parameters:
``` bash
velero backup create <backup-name>
	--include-cluster-resources=false
```

#### all namespace-scoped resources + some additional cluster-scoped resources
This command means backup all namespace-scoped resources, and related cluster-scoped resources, plus all PersistentVolume resources.
``` bash
velero backup create <backup-name>
	--include-namespaces=*
	--include-cluster-scoped-resources=persistentvolume
```

#### all namespace-scoped resources + all cluster-scoped resources
The following commands have the same meaning: backup all namespace-scoped resources, and all cluster-scoped resources.
``` bash
velero backup create <backup-name>
	--include-cluster-scoped-resources=*
```

``` bash
velero backup create <backup-name>
	--include-cluster-resources=true
```

#### describe command change
In `velero backup describe` command, the four new parameters should be outputted too.
``` bash
 velero backup describe <backup-name>
......

Namespaces:
  Included:  ns2
  Excluded:  <none>

Resources:
  Included cluster-scoped:    StorageClass,PersistentVolume
  Excluded cluster-scoped:    <none>
  Included namespace-scoped:  default
  Excluded namespace-scoped:  <none>
......
```

**Note:** `velero restore` command doesn't support those four new parameter in Velero v1.11, but `velero schedule` supports the four new parameters through backup specification.

## Detailed Design
With adding `IncludedNamespaceScopedResources`, `ExcludedNamespaceScopedResources`, `IncludedClusterScopedResources` and `ExcludedClusterScopedResources`, the `BackupSpec` looks like:
``` go
type BackupSpec struct {
    ......
	// IncludedResources is a slice of resource names to include
	// in the backup. If empty, all resources are included.
	// +optional
	// +nullable
	IncludedResources []string `json:"includedResources,omitempty"`

	// ExcludedResources is a slice of resource names that are not
	// included in the backup.
	// +optional
	// +nullable
	ExcludedResources []string `json:"excludedResources,omitempty"`

	// IncludeClusterResources specifies whether cluster-scoped resources
	// should be included for consideration in the backup.
	// +optional
	// +nullable
	IncludeClusterResources *bool `json:"includeClusterResources,omitempty"`

	// IncludedClusterScopedResources is a slice of cluster-scoped
	// resource type names to include in the backup.
	// If set to "*", all cluster scope resource types are included.
	// The default value is empty, which means only related cluster 
	// scope resources are included.
	// +optional
	// +nullable
	IncludedClusterScopedResources []string `json:"includedClusterScopedResources,omitempty"`

	// ExcludedClusterScopedResources is a slice of cluster-scoped 
	// resource type names to exclude from the backup.
	// If set to "*", all cluster scope resource types are excluded.
	// +optional
	// +nullable
	ExcludedClusterScopedResources []string `json:"excludedClusterScopedResources,omitempty"`

	// IncludedNamespaceScopedResources is a slice of namespace-scoped
	// resource type names to include in the backup.
	// The default value is "*".
	// +optional
	// +nullable
	IncludedNamespaceScopedResources []string `json:"includedNamespaceScopedResources,omitempty"`

	// ExcludedNamespaceScopedResources is a slice of namespace-scoped
	// resource type names to exclude from the backup.
	// If set to "*", all namespace scope resource types are excluded.
	// +optional
	// +nullable
	ExcludedNamespaceScopedResources []string `json:"excludedNamespaceScopedResources,omitempty"`
    ......
}
```

## Alternatives Considered
Proposal from Jibu Data [Issue 5120](https://github.com/vmware-tanzu/velero/issues/5120#issue-1304534563)

## Security Considerations
No security impact.

## Compatibility
The four new parameters cannot be mixed with existing resource filter parameters: `IncludedResources`, `ExcludedResources` and `IncludeClusterResources`.
If the new parameters and old parameters both appears in command line, or are specified in backup spec, the command line and the backup should fail.

## Implementation
This change should be included into Velero v1.11.
New parameters will coexist with `IncludedResources`, `ExcludedResources` and `IncludeClusterResources`.
Plan to deprecate `IncludedResources`, `ExcludedResources` and `IncludeClusterResources` in future releases, but also open to the community's feedback.

## Open Issues
`LabelSelector/OrLabelSelectors` apply to namespace-scoped resources.
It may be reasonable to make them also working on cluster-scoped resources.
An issue is created to trace this topic [resource label selector not work for cluster-scoped resources](https://github.com/vmware-tanzu/velero/issues/5787)
