# Proposal to add include exclude policy to resource policy

This enhancement will allow the user to set include and exclude filters for resources in a resource policy configmap, so that
these filters are reusable and the user will not need to set them each time they create a backup.

## Background
As mentioned in issue [#8610](https://github.com/vmware-tanzu/velero/issues/8610).  When there's a long list of resources 
to include or exclude in a backup, it can be cumbersome to set them each time a backup is created.  There's a requirement to
set these filters in a separate data structure so that they can be reused in multiple backups.

## High-Level Design
We may extend the data structure of resource policy to add `includeExcludePolicy`, which include the include and exclude filters 
in the BackupSpec.  When the user creates a backup which references the resource policy config `velero backup create --resource-policies-configmap <configmap-name>`,
the filters in "includeExcludePolicy" will take effect to filter the resources when velero collects the resources to backup.

## Detailed Design

### Data Structure
The map `includeExcludePolicy` contains four fields `includedClusterScopedResources`, `excludedClusterScopedResources`, 
`includedNamespaceScopedResources`,`excludedNamespaceScopedResources`.  These filters work exactly as the filters defined BackupSpec with
the same names.  An example of the policy looks like:
```yaml
#omitted other irrelevant fields like 'version', 'volumePolicies'
includeExcludePolicy:
  includedClusterScopedResources:
    - "cr"
    - "crd"
    - "pv"
  excludedClusterScopedResources:
    - "volumegroupsnapshotclass"
    - "ingressclass"
  includedNamespaceScopedResources:
    - "pod"
    - "service"
    - "deployment"
    - "pvc"
  excludedNamespaceScopedResources:
    - "configmap"
```
These filters are in the form of scoped include/exclude filters, which by design will not work with the "old" resource filters.
Therefore, when a Backup references a resource policy configmap which has `includeExcludePolicy`, and at the same time it has 
the "old" resource filters, i.e. `includedResources`, `excludedResources`, `includeClusterResources` set in the BackupSpec, the
Backup will fail with a validation error.

### Priorities 
A user may set the include/exclude filters in Backupspec and also in the resource policy configmap.  In this case, the filters 
in both the Backupspec and the resource policy configmap will take effect.  When there's a conflict, the filters in the Backupspec 
will take precedence.  For example, if resource X is in the list of `includedNamespaceScopedResources` filter in the Backupspec, but 
it's also in the list of `excludedClusterScopedResources` in the resource policy configmap, then resource X will be included in the backup.
In this way, users can set the filters in the resource policy configmap to cover most of their use cases, and then override them 
in the Backupspec when needed.

### Implementation
In addition to the data structure change, we will need to implement the following changes:
1. A new function `CombineWithPolicy` will be added to the struct `ScopeIncludesExcludes`, which will combine the include/exclude filters
in the resource policy configmap with the include/exclude filters in the Backupspec:  
```go
func (ie *ScopeIncludesExcludes) CombineWithPolicy(policy resourcepolicies.IncludeExcludePolicy) {
	mapFunc := scopeResourceMapFunc(ie.helper)
	for _, item := range policy.ExcludedNamespaceScopedResources {
		resolvedItem := mapFunc(item, true)
		if resolvedItem == "" {
			continue
		}
		if !ie.ShouldInclude(resolvedItem) && !ie.ShouldExclude(resolvedItem) {
			// The existing includeExcludes in the struct has higher priority, therefore, we should only add the item to the filter
			// when the struct does not include this item and this item is not yet in the excludes filter.
			ie.namespaceScopedResourceFilter.excludes.Insert(resolvedItem)
		}
		
	}
.....
```
This function will be called in the `kubernetesBackupper.BackupWithResolvers` function, to make sure the combined `ScopeIncludesExcludes` 
filter will be assigned to the `ResourceIncludesExcludes` filter of the Backup request.

2. Extra validation code will be added to the function `prepareBackupRequest` of `BackupReconciler` to check if there are "old"
Resource filters in the BackupSpec when the Backup references a resource policy configmap which has `includeExcludePolicy`.

## Alternatives Considered
We may put `includeExcludePolicy` in a separate configmap, but it will require adding extra field to BackupSpec to reference the configmap,
which is not necessary.
