# Proposal to add resource filters for backup can distinguish whether resource is cluster scope or namespace scope.

- [Proposal to add resource filters for backup can distinguish whether resource is cluster scope or namespace scope.](#proposal-to-add-resource-filters-for-backup-can-distinguish-whether-resource-is-cluster-scope-or-namespace-scope)
	- [Abstract](#abstract)
	- [Background](#background)
	- [Goals](#goals)
	- [Non Goals](#non-goals)
	- [High-Level Design](#high-level-design)
		- [Parameters Rules](#parameters-rules)
		- [Using scenarios:](#using-scenarios)
			- [no namespaced resources + no cluster resources](#no-namespaced-resources--no-cluster-resources)
			- [no namespaced resources + some cluster resources](#no-namespaced-resources--some-cluster-resources)
			- [no namespaced resources + all cluster resources](#no-namespaced-resources--all-cluster-resources)
			- [some namespaced resources + no cluster resources](#some-namespaced-resources--no-cluster-resources)
			- [some namespaced resources + only related cluster resources](#some-namespaced-resources--only-related-cluster-resources)
			- [some namespaced resources + some additional cluster resources](#some-namespaced-resources--some-additional-cluster-resources)
			- [some namespaced resources + all cluster resources](#some-namespaced-resources--all-cluster-resources)
			- [all namespaced resources + no cluster resources](#all-namespaced-resources--no-cluster-resources)
			- [all namespaced resources + some additional cluster resources](#all-namespaced-resources--some-additional-cluster-resources)
			- [all namespaced resources + all cluster resources](#all-namespaced-resources--all-cluster-resources)
			- [describe command change](#describe-command-change)
	- [Detailed Design](#detailed-design)
	- [Alternatives Considered](#alternatives-considered)
	- [Security Considerations](#security-considerations)
	- [Compatibility](#compatibility)
	- [Implementation](#implementation)
	- [Open Issues](#open-issues)

## Abstract
The current filter (IncludedResources/ExcludedResources + IncludeClusterResources flag) is not enough for some special cases, e.g. all namespaced resources + some kind of cluster resource and all namespaced resources + cluster resource excludes.
Propose to add a new group of resource filtering parameters, which can distinguish cluster and namespaced resources.

## Background
There are two sets of resource filters for Velero: `IncludedNamespaces/ExcludedNamespaces` and `IncludedResources/ExcludedResources`. 
`IncludedResources` means only including the resource types specified in the parameter. Both cluster and namespaced resources are handled in this parameter by now.
The k8s resources are separated into cluster scope and namespaced scope.
As a result, it's hard to include all resources in one group and only including specified resource in the other group.

## Goals
- Make Velero can support more complicated namespaced and cluster resources filtering scenarios in backup.

## Non Goals
- Enrich the resource filtering rules, for example, advanced PV filtering and filtering by resource names.


## High-Level Design
Four new parameters are added into command `velero backup create`: `--include-cluster-scope-resources`, `--exclude-cluster-scope-resources`, `--include-namespaced-resources` and `--exclude-namespaced-resources`. 
`--include-cluster-scope-resources` and `--exclude-cluster-scope-resources` are used to filter cluster scope resources included or excluded in backup per resource type.
`--include-namespaced-resources` and `--exclude-namespaced-resources` are used to filter namespace scope resources included or excluded in backup per resource type.
Restore and other code pieces also use resource filtering will be handled in future releases.

### Parameters Rules

* `--include-cluster-scope-resources`, `--include-namespaced-resources`, `--exclude-cluster-scope-resources` and `--exclude-namespaced-resources` valid value include `*` and comma separated string. Each element of the CSV string should a k8s resource name. The format should be `resource.group`, such as `storageclasses.storage.k8s.io.`.

* `--include-cluster-scope-resources`, `--include-namespaced-resources`, `--exclude-cluster-scope-resources` and `--exclude-namespaced-resources` parameters are mutual exclusive with `--include-cluster-resources`, `--include-resources` and `--exclude-resources` parameters. If both sets of parameters are provisioned, validation failure should be returned.

* `--include-cluster-scope-resources` and `--exclude-cluster-scope-resources` should only contain cluster scope resource type names. If namespace scope resource type names are included, they are ignored.

* If there are conflicts between `--include-cluster-scope-resources` and `--exclude-cluster-scope-resources` specified resources type lists, `--exclude-cluster-scope-resources` parameter has higher priority.

* `--include-namespaced-resources` and `--exclude-namespaced-resources` should only contain namespace scope resource type names. If cluster scope resource type names are included, they are ignored.

* If there are conflicts between `--include-namespaced-resources` and `--exclude-namespaced-resources` specified resources type lists, `--exclude-namespaced-resources` parameter has higher priority.

* If  `--include-namespaced-resources` is not present, it means all namespace scope resources are included per resource type.

* If both `--include-cluster-scope-resources` and `--exclude-cluster-scope-resources` are not present, it means no additional cluster resource is included per resource type, just as the existing `--include-cluster-resources` parameter not setting value. Cluster resources are related to the namespace scope resources, which means those are returned in the namespace resources' BackupItemAction's result AdditionalItems array, are still included in backup by default. Taking backing up PVC scenario as an example, PVC is namespaced, PV is in cluster scope. PVC's BIA will include PVC related PV into backup too.

* If the backup contains no resource, validation failure should be returned.

### Using scenarios:
Please notice, if the scenario give the example of using old filtering parameters (`--include-cluster-resources`, `--include-resources` and `--exclude-resources`), that means the old parameters also work for this case. If old parameters example is not given, that means they don't work for this scenario, only new parameters (`--include-cluster-scope-resources`, `--include-namespaced-resources`, `--exclude-cluster-scope-resources` and `--exclude-namespaced-resources`) work.

#### no namespaced resources + no cluster resources
This is not allowed. Backup or restore cannot contain no resource.

#### no namespaced resources + some cluster resources
The following command means backup no namespaced resources and some cluster resources.

``` bash
velero backup create <backup-name>
	--exclude-namespaced-resources=*
	--include-cluster-scope-resources=storageclass
```

#### no namespaced resources + all cluster resources
The following command means backup no namespaced resources and all cluster resources.

``` bash
velero backup create <backup-name>
	--exclude-namespaced-resources=*
	--include-cluster-scope-resources=*
```

#### some namespaced resources + no cluster resources
The following commands mean backup all resources in namespaces default and kube-system, and no cluster resources.

Example of new parameters:
``` bash
velero backup create <backup-name>
	--include-namespaces=default,kube-system
	--exclude-cluster-scope-resources=*
```

Example of old parameters:
``` bash
velero backup create <backup-name>
	--include-namespaces=default,kube-system
	--include-cluster-resources=false
```

The following commands mean backup PVC, Deployment, Service, Endpoint, Pod and ReplicaSet resources in all namespaces, and no cluster resources. Although PVC's related PV should be included, due to no cluster resources are included, so they are ruled out too.

Example of new parameters:
``` bash
velero backup create <backup-name>
	--include-namespaced-resources=persistentvolumeclaim,deployment,service,endpoint,pod,replicaset
	--exclude-cluster-scope-resources=*
```

Example of old parameters:
``` bash
velero backup create <backup-name>
	--include-resources=persistentvolumeclaim,deployment,service,endpoint,pod,replicaset
	--include-cluster-resources=false
```

The following commands mean backup PVC, Deployment, Service, Endpoint, Pod and ReplicaSet resources in namespace default and kube-system, and no cluster resources. Although PVC's related PV should be included, due to no cluster resources are included, so they are ruled out too.

Example of new parameters:
``` bash
velero backup create <backup-name>
	--include-namespaces=default,kube-system
	--include-namespaced-resources=persistentvolumeclaim,deployment,service,endpoint,pod,replicaset
	--exclude-cluster-scope-resources=*
```

Example of old parameters:
``` bash
velero backup create <backup-name>
	--include-namespaces=default,kube-system
	--include-resources=persistentvolumeclaim,deployment,service,endpoint,pod,replicaset
	--include-cluster-resources=false
```

The following commands mean backup all resources except Ingress type resources in all namespaces, and no cluster resources.

Example of new parameters:
``` bash
velero backup create <backup-name>
	--exclude-namespaced-resources=ingress
	--exclude-cluster-scope-resources=*
```

Example of old parameters:
``` bash
velero backup create <backup-name>
	--exclude-resources=ingress
	--include-cluster-resources=false
```

#### some namespaced resources + only related cluster resources
This means backup all resources in namespaces default and kube-system, and related cluster resources.
``` bash
velero backup create <backup-name>
	--include-namespaces=default,kube-system
```

The following commands mean backup PVC, Deployment, Service, Endpoint, Pod and ReplicaSet resources in all namespaces, and related cluster resources (PVC's related PV).

Example of new parameters:
``` bash
velero backup create <backup-name>
	--include-namespaced-resources=persistentvolumeclaim,deployment,service,endpoint,pod,replicaset
```

Example of old parameters:
``` bash
velero backup create <backup-name>
	--include-resources=persistentvolumeclaim,deployment,service,endpoint,pod,replicaset
```

The following commands mean backup PVC, Deployment, Service, Endpoint, Pod and ReplicaSet resources in namespaces default and kube-system, and related cluster resources. PVC related PV is included too.

Example of new parameters:
``` bash
velero backup create <backup-name>
	--include-namespaces=default,kube-system
	--include-namespaced-resources=persistentvolumeclaim,deployment,service,endpoint,pod,replicaset
```

Example of old parameters:
``` bash
velero backup create <backup-name>
	--include-namespaces=default,kube-system
	--include-resources=persistentvolumeclaim,deployment,service,endpoint,pod,replicaset
```

This means backup all resources except Ingress type resources in all namespaces, and related cluster resources.

Example of new parameters:
``` bash
velero backup create <backup-name>
	--exclude-namespaced-resources=ingress
```

Example of old parameters:
``` bash
velero backup create <backup-name>
	--exclude-resources=ingress
```

#### some namespaced resources + some additional cluster resources
This means backup all resources in namespace in default, kube-system, and related cluster resources, plus all StorageClass cluster resources.
``` bash
velero backup create <backup-name>
	--include-namespaces=default,kube-system
	--include-cluster-scope-resources=storageclass
```

This means backup PVC, Deployment, Service, Endpoint, Pod and ReplicaSet resources in all namespaces, and related cluster resources, plus all StorageClass cluster resources, and PVC related PV.
``` bash
velero backup create <backup-name>
	--include-namespaced-resources=persistentvolumeclaim,deployment,service,endpoint,pod,replicaset
	--include-cluster-scope-resources=storageclass
```

This means backup PVC, Deployment, Service, Endpoint, Pod and ReplicaSet resources in default and kube-system namespaces, and related cluster resources, plus all StorageClass cluster resources, and PVC related PV.
``` bash
velero backup create <backup-name>
	--include-namespaced-resources=persistentvolumeclaim,deployment,service,endpoint,pod,replicaset
	--include-namespaces=default,kube-system
	--include-cluster-scope-resources=storageclass
```

This means backup PVC, Deployment, Service, Endpoint, Pod and ReplicaSet resources in default and kube-system namespaces, and related cluster resources, plus all cluster scope resources except StorageClass type resources.
``` bash
velero backup create <backup-name>
	--include-namespaced-resources=persistentvolumeclaim,deployment,service,endpoint,pod,replicaset
	--include-namespaces=default,kube-system
	--exclude-cluster-scope-resources=storageclass
```

#### some namespaced resources + all cluster resources
The following commands mean backup all resources in namespace in default, kube-system, and all cluster resources.

Example of new parameters:
``` bash
velero backup create <backup-name>
	--include-namespaces=default,kube-system
	--include-cluster-scope-resources=*
```

Example of old parameters:
``` bash
velero backup create <backup-name>
	--include-namespaces=default,kube-system
	--include-cluster-resources=true
```

This means backup Deployment, Service, Endpoint, Pod and ReplicaSet resources in all namespaces, and all cluster resources.
``` bash
velero backup create <backup-name>
	--include-namespaced-resources=deployment,service,endpoint,pod,replicaset
	--include-cluster-scope-resources=*
```

This means backup Deployment, Service, Endpoint, Pod and ReplicaSet resources in default and kube-system namespaces, and all cluster resources.
``` bash
velero backup create <backup-name>
	--include-namespaces=default,kube-system
	--include-namespaced-resources=deployment,service,endpoint,pod,replicaset
	--include-cluster-scope-resources=*
```

#### all namespaced resources + no cluster resources
The following commands all mean backup all namespace scope resources and no cluster resources.

Example of new parameters:
``` bash
velero backup create <backup-name>
	--exclude-cluster-scope-resources=*
```

Example of old parameters:
``` bash
velero backup create <backup-name>
	--include-cluster-resources=false
```

#### all namespaced resources + some additional cluster resources
This command means backup all namespace scope resources, and related cluster resources, plus all PersistentVolume resources.
``` bash
velero backup create <backup-name>
	--include-namespaces=*
	--include-cluster-scope-resources=persistentvolume
```

#### all namespaced resources + all cluster resources
The following commands have the same meaning: backup all namespace scope resources, and all cluster scope resources.
``` bash
velero backup create <backup-name>
	--include-cluster-scope-resources=*
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
  Included:               <none>
  Excluded:               <none>
  Included-cluster-scope: StorageClass,PersistentVolume
  Excluded-cluster-scope: <none>
  Included-namespaced:    default
  Excluded-namespaced:    <none>
  Cluster-scoped:  auto

......
```

**Note:** `velero restore` command doesn't support those four new parameter in Velero v1.11, but `velero schedule` supports the four new parameters through backup specification.

## Detailed Design
With adding `IncludedNamespacedResources`, `ExcludedNamespacedResources`, `IncludedClusterScopeResources` and `ExcludedClusterScopeResources`, the `BackupSpec` looks like:
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

	// IncludedClusterScopeResources is a slice of cluster scope 
	// resource type names to include in the backup.
	// If set to "*", all cluster scope resource types are included.
	// The default value is empty, which means only related cluster 
	// scope resources are included.
	// +optional
	// +nullable
	IncludedClusterScopeResources []string `json:"includedClusterScopeResources,omitempty"`

	// ExcludedClusterScopeResources is a slice of cluster scope 
	// resource type names to exclude from the backup.
	// If set to "*", all cluster scope resource types are excluded.
	// +optional
	// +nullable
	ExcludedClusterScopeResources []string `json:"excludedClusterScopeResources,omitempty"`

	// IncludedNamespacedResources is a slice of namespace scope
	// resource type names to include in the backup.
	// The default value is "*".
	// +optional
	// +nullable
	IncludedNamespacedResources []string `json:"includedNamespacedResources,omitempty"`

	// ExcludedNamespacedResources is a slice of namespace scope
	// resource type names to exclude from the backup.
	// If set to "*", all namespace scope resource types are excluded.
	// +optional
	// +nullable
	ExcludedNamespacedResources []string `json:"excludedNamespacedResources,omitempty"`
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
`LabelSelector/OrLabelSelectors` apply to namespaced scope resources.
It may be reasonable to make them also working on cluster scope resources.
An issue is created to trace this topic [resource label selector not work for cluster scope resources](https://github.com/vmware-tanzu/velero/issues/5787)
