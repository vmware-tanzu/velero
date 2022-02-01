# Add support for `ExistingResourcePolicy` to restore API
## Abstract
Velero currently does not support any restore policy on kubernetes resources that are already present in-cluster. Velero skips over the restore of the resource if it already exists in the namespace/cluster irrespective of whether the resource present in the restore is the same or different from the one present on the cluster. It is desired that Velero gives the option to the user to decide whether or not the resource in backup should overwrite the one present in the cluster.

## Background
As of Today, Velero will skip over the restoration of resources that already exist in the cluster. The current workflow followed by Velero is (Using a `service` that is backed up for example):
- Velero tries to attempt restore of the `service` 
- Fetches the `service` from the cluster 
- If the `service` exists then:
    - Checks whether the `service` instance in the cluster is equal to the `service`  instance present in backup
        - If not equal then skips the restore of the `service` and adds a restore warning (except for [ServiceAccount objects](https://github.com/vmware-tanzu/velero/blob/574baeb3c920f97b47985ec3957debdc70bcd5f8/pkg/restore/restore.go#L1246))
        - If equal then skips the restore of the `service` and mentions that the restore of resource `service` is skipped in logs

It is desired to add the functionality to specify whether or not to overwrite the instance of resource `service` in cluster with the one present in backup during the restore process.

Related issue: https://github.com/vmware-tanzu/velero/issues/4066

## Goals
- Add support for `ExistingResourcePolicy` to restore API for Kubernetes resources.

## Non Goals
- Change existing restore workflow for `ServiceAccount` objects
- Add support for `ExistingResourcePolicy` as `recreate` for Kubernetes resources. (Future scope feature)

## Unrelated Proposals (Completely different functionalities than the one proposed in the design)
- Add support for `ExistingResourcePolicy` to restore API for Non-Kubernetes resources.
- Add support for `ExistingResourcePolicy` to restore API for `PersistentVolume` data.

### Use-cases/Scenarios

### A. Production Cluster - Backup Cluster:
Let's say you have a Backup Cluster which is identical to the Production Cluster. After some operations/usage/time the Production Cluster had changed itself, there might be new deployments, some secrets might have been updated. Now, this means that the Backup cluster will no longer be identical to the Production Cluster. In order to keep the Backup Cluster up to date/identical to the Production Cluster with respect to Kubernetes resources except PV data we would like to use Velero for scheduling new backups which would in turn help us update the Backup Cluster via Velero restore.

Reference: https://github.com/vmware-tanzu/velero/issues/4066#issuecomment-954320686

### B. Help identify resource delta:
Here delta resources mean the resources restored by a previous backup, but they are no longer in the latest backup. Let's follow a sequence of steps to understand this scenario:
- Consider there are 2 clusters, Cluster A, which has 3 resources - P1, P2 and P3.
- Create a Backup1 from Cluster A which has P1, P2 and P3.
- Perform restore on a new Cluster B using Backup1.
- Now, Lets say in Cluster A resource P1 gets deleted and resource P2 gets updated.
- Create a new Backup2 with the new state of Cluster A, keep in mind Backup1 has P1, P2 and P3 while Backup2 has P2' and P3.
- So the Delta here is (|Cluster B - Backup2|), Delete P1 and Update P2.
- During Restore time we would want the Restore to help us identify this resource delta.

Reference: https://github.com/vmware-tanzu/velero/pull/4613#issuecomment-1027260446

## High-Level Design
### Approach 1: Add a new spec field `existingResourcePolicy` to the Restore API
In this approach we do *not* change existing velero behavior. If the resource to restore in cluster is equal to the one backed up then do nothing following current Velero behavior. For resources that already exist in the cluster that are not equal to the resource in the backup (other than Service Accounts). We add a new optional spec field `existingResourcePolicy` which can have the following values:
1. `none`: This is the existing behavior, if Velero encounters a resource that already exists in the cluster, we simply
skip restoration.
2. `update`: This option would provide the following behavior.
   - Unchanged resources: Velero would update the backup/restore labels on the unchanged resources, if labels patch fails Velero adds a restore error.
   - Changed resources: Velero will first try to patch the changed resource, Now if the patch:
       - succeeds: Then the in-cluster resource gets updated with the labels as well as the resource diff
       - fails: Velero adds a restore warning and tries to just update the backup/restore labels on the resource, if the labels patch also fails then we add restore error.
3. `recreate`:  If resource already exists, then Velero will delete it and recreate the resource.

*Note:* The `recreate` option is a non-goal for this enhancement proposal, but it is considered as a future scope.
Another thing to highlight is that Velero will not be deleting any resources in any of the policy options proposed in 
this design but Velero will patch the resources in `update` policy option.

Example:
A. The following Restore will execute the `existingResourcePolicy` restore type `none` for the `services` and `deployments` present in the `velero-protection` namespace.

```
Kind: Restore

…

includeNamespaces: velero-protection
includeResources:
  - services
  - deployments
existingResourcePolicy: none

```

B. The following Restore will execute the `existingResourcePolicy` restore type `update` for the `secrets` and `daemonsets` present in the `gdpr-application` namespace.
```
Kind: Restore

…
includeNamespaces: gdpr-application
includeResources:
  - secrets
  - daemonsets
existingResourcePolicy: update
```

### Approach 2: Add a new spec field `existingResourcePolicyConfig` to the Restore API
In this approach we give user the ability to specify which resources are to be included for a particular kind of force update behaviour, essentially a more granular approach where in the user is able to specify a resource:behaviour mapping. It would look like:
`existingResourcePolicyConfig`:
- `patch:` 
    - `includedResources:` [ ]string
- `recreate:`
    - `includedResources:` [ ]string

*Note:* 
- There is no `none` behaviour in this approach as that would conform to the current/default Velero restore behaviour.
- The `recreate` option is a non-goal for this enhancement proposal, but it is considered as a future scope.


Example:
A. The following Restore will execute the restore type `patch` and apply the `existingResourcePolicyConfig` for `secrets` and `daemonsets` present in the `inventory-app` namespace.
```
Kind: Restore
…
includeNamespaces: inventory-app
existingResourcePolicyConfig:
  patch:
    includedResources
     - secrets
     - daemonsets

```


### Approach 3: Combination of Approach 1 and Approach 2

Now, this approach is somewhat a combination of the aforementioned approaches. Here we propose addition of two spec fields to the Restore API - `existingResourceDefaultPolicy` and `existingResourcePolicyOverrides`. As the names suggest ,the idea being that `existingResourceDefaultPolicy` would describe the default velero behaviour for this restore and `existingResourcePolicyOverrides` would override the default policy explicitly for some resources.

Example:
A. The following Restore will execute the restore type `patch` as the `existingResourceDefaultPolicy` but will override the default policy for `secrets` using the `existingResourcePolicyOverrides` spec as `none`.
```
Kind: Restore
…
includeNamespaces: inventory-app
existingResourceDefaultPolicy: patch
existingResourcePolicyOverrides:
  none:
    includedResources
     - secrets

```

## Detailed Design
### Approach 1: Add a new spec field `existingResourcePolicy` to the Restore API
The `existingResourcePolicy` spec field will be an `PolicyType` type field. 

Restore API:
```
type RestoreSpec struct {
.
.
.
// ExistingResourcePolicy specifies the restore behaviour for the kubernetes resource to be restored
// +optional
ExistingResourcePolicy PolicyType

}
```
PolicyType:
```
type PolicyType string
const PolicyTypeNone PolicyType  = "none"
const PolicyTypePatch PolicyType = "update"
```

### Approach 2: Add a new spec field `existingResourcePolicyConfig` to the Restore API
The `existingResourcePolicyConfig` will be a spec of type `PolicyConfiguration` which gets added to the Restore API.

Restore API:
```
type RestoreSpec struct {
.
.
.
// ExistingResourcePolicyConfig specifies the restore behaviour for a particular/list of kubernetes resource(s) to be restored
// +optional
ExistingResourcePolicyConfig []PolicyConfiguration

}
```

PolicyConfiguration:
```
type PolicyConfiguration struct {

PolicyTypeMapping map[PolicyType]ResourceList

}
```

PolicyType:
```
type PolicyType string
const PolicyTypePatch PolicyType = "patch"
const PolicyTypeRecreate PolicyType = "recreate"
```

ResourceList:
```
type ResourceList struct {
IncludedResources []string 
}
```

### Approach 3: Combination of Approach 1 and Approach 2

Restore API:
```
type RestoreSpec struct {
.
.
.
// ExistingResourceDefaultPolicy specifies the default restore behaviour for the kubernetes resource to be restored
// +optional
existingResourceDefaultPolicy PolicyType

// ExistingResourcePolicyOverrides specifies the restore behaviour for a particular/list of kubernetes resource(s) to be restored
// +optional
existingResourcePolicyOverrides []PolicyConfiguration

}
```

PolicyType:
```
type PolicyType string
const PolicyTypeNone PolicyType  = "none"
const PolicyTypePatch PolicyType = "patch"
const PolicyTypeRecreate PolicyType = "recreate"
```
PolicyConfiguration:
```
type PolicyConfiguration struct {

PolicyTypeMapping map[PolicyType]ResourceList

}
```
ResourceList:
```
type ResourceList struct {
IncludedResources []string 
}
```

The restore workflow changes will be done [here](https://github.com/vmware-tanzu/velero/blob/b40bbda2d62af2f35d1406b9af4d387d4b396839/pkg/restore/restore.go#L1245)

### CLI changes for Approach 1
We would introduce a new CLI flag called `existing-resource-policy` of string type. This flag would be used to accept the 
policy from the user. The velero restore command would look somewhat like this:
```
velero create restore <restore_name> --existing-resource-policy=update 
```

Help message `Restore Policy to be used during the restore workflow, can be - none, update`

The CLI changes will go at `pkg/cmd/cli/restore/create.go`

We would also add a validation which checks for invalid policy values provided to this flag.

Restore describer will also be updated to reflect the policy `pkg/cmd/util/output/restore_describer.go`

### Implementation Decision
We have decided to go ahead with the implementation of Approach 1 as:
- It is easier to implement 
- It is also easier to scale and leaves room for improvement and the door open to expanding to approach 3
- It also provides an option to preserve the existing velero restore workflow