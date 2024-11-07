# Handle backup of volumes by resources filters
 
## Abstract
Currently, Velero doesn't have one flexible way to handle volumes.
 
If users want to skip the backup of volumes or only backup some volumes in different namespaces in batch, currently they need to use the opt-in and opt-out approach one by one, or use label-selector but if it has big different labels on each different related pod, which is cumbersome when they have lots of volumes to handle with. it would be convenient if Velero could provide policies to handle the backup of volumes just by `some specific volumes conditions`.
 
## Background
As of Today, Velero has lots of filters to handle (backup or skip backup) resources including resources filters like `IncludedNamespaces, ExcludedNamespaces`, label selectors like `LabelSelector, OrLabelSelectors`, annotation like `backup.velero.io/must-include-additional-items` etc. But it's not enough flexible to handle volumes, we need one generic way to handle volumes.
 
## Goals
- Introducing flexible policies to handle volumes, and do not patch any labels or annotations to the pods or volumes.
 
## Non-Goals
- We only handle volumes for backup and do not support restore.
- Currently, only handles volumes, and does not support other resources.
- Only environment-unrelated and platform-independent general volumes attributes are supported, do not support volumes attributes related to a specific environment.
 
## Use-cases/Scenarios
### Skip backup volumes by some attributes
Users want to skip PV with the requirements:
- option to skip all PV data
- option to skip specified PV type (RBD, NFS)
- option to skip specified PV size
- option to skip specified storage-class
 
## High-Level Design
First, Velero will provide the user with one YAML file template and all supported volume policies will be in.

Second, writing your own configuration file by imitating the YAML template, it could be partial volume policies from the template.

Third, create one configmap from your own configuration file, and the configmap should be in Velero install namespace.

Fourth, create a backup with the command `velero backup create --resource-policies-configmap $policiesConfigmap`, which will reference the current backup to your volume policies. At the same time, Velero will validate all volume policies user imported, the backup will fail if the volume policies are not supported or some items could not be parsed.

Fifth, the current backup CR will record the reference of volume policies configmap.

Sixth, Velero first filters volumes by other current supported filters, at last, it will apply the volume policies to the filtered volumes to get the final matched volume to handle.
 
## Detailed Design
The volume resources policies should contain a list of policies which is the combination of conditions and related `action`, when target volumes meet the conditions, the related `action` will take effection.

Below is the API Design for the user configuration:

### API Design
```go
type VolumeActionType string

const Skip VolumeActionType = "skip"

// Action defined as one action for a specific way of backup
type Action struct {
	// Type defined specific type of action, it could be 'file-system-backup', 'volume-snapshot', or 'skip' currently
	Type VolumeActionType `yaml:"type"`
	// Parameters defined map of parameters when executing a specific action
	// +optional
	// +nullable
	Parameters map[string]interface{} `yaml:"parameters,omitempty"`
}

// VolumePolicy defined policy to conditions to match Volumes and related action to handle matched Volumes
type VolumePolicy struct {
	// Conditions defined list of conditions to match Volumes
	Conditions map[string]interface{} `yaml:"conditions"`
	Action     Action                 `yaml:"action"`
}

// ResourcePolicies currently defined slice of volume policies to handle backup
type ResourcePolicies struct {
  Version        string         `yaml:"version"`
	VolumePolicies []VolumePolicy `yaml:"volumePolicies"`
	// we may support other resource policies in the future, and they could be added separately
  	// OtherResourcePolicies: []OtherResourcePolicy
}
```

The policies YAML config file would look like this:
```yaml
version: v1
volumePolicies:
# it's a list and if the input item matches the first policy, the latters will be ignored
# each policy consists of a list of conditions and an action

# each key in the object is one condition, and one policy will apply to resources that meet ALL conditions
- conditions:
    # capacity condition matches the volumes whose capacity falls into the range
    capacity: "0,100Gi"
    csi:
      driver: ebs.csi.aws.com
      fsType: ext4
    storageClass:
    - gp2
    - ebs-sc
  action:
    type: volume-snapshot
    parameters:
      # optional parameters which are custom-defined parameters when doing an action
      volume-snapshot-timeout: "6h"
- conditions:
    capacity: "0,100Gi"
    storageClass:
    - gp2
    - ebs-sc
  action:
    type: file-system-backup
- conditions:
    nfs:
      server: 192.168.200.90
  action:
    # type of file-system-backup could be defined a second time
    type: file-system-backup
- conditions:
    nfs: {}
  action:
    type: skip
- conditions:
    csi:
      driver: aws.efs.csi.driver
  action:
    type: skip
```
 
### Filter rules
#### VolumePolicies
The whole resource policies consist of groups of volume policies.

For one specific volume policy which is a combination of one action and serval conditions. which means one action and serval conditions are the smallest unit of volume policy.

Volume policies are a list and if the target volumes match the first policy, the latter will be ignored, which would reduce the complexity of matching volumes especially when there are multiple complex volumes policies.

#### Action
`Action` defined one action for a specific way of backup:
  - if choosing `Kopia` or `Restic`, the action value would be `file-system-backup`.
  - if choosing volume snapshot, the action value would be `volume-snapshot`.
  - if choosing skip backup of volume, the action value would be `skip`, and it will skip backup of volume no matter is `file-system-backup` or `volume-snapshot`.

The policies could be extended for later other ways of backup, which means it may have some other `Action` value that will be assigned in the future.

Both `file-system-backup` `volume-snapshot`, and `skip` could be partially or fully configured in the YAML file. And configuration could take effect only for the related action.

#### Conditions
The conditions are serials of volume attributes, the matched Volumes should meet all the volume attributes in one conditions configuration.

##### Supported conditions
In Velero 1.11, we want to support the volume attributes listed below:
- capacity: matching volumes have the capacity that falls within this `capacity` range.
- storageClass: matching volumes those with specified `storageClass`, such as `gp2`, `ebs-sc` in eks.
- matching volumes that used specified volume sources.
##### Parameters
Parameters are optional for one specific action. For example, it could be `csi-snapshot-timeout: 6h` for CSI snapshot.

#### Special rule definitions:
- One single condition in `Conditions` with a specific key and empty value, which means the value matches any value. For example, if the `conditions.nfs` is `{}`, it means if `NFS` is used as `persistentVolumeSource` in Persistent Volume will be skipped no matter what the NFS server or NFS Path is.
 
- The size of each single filter value should limit to 256 bytes in case of an unfriendly long variable assignment.
 
- For capacity for PV or size for Volume, the value should include the lower value and upper value concatenated by commas. And it has several combinations below:
  - "0,5Gi" or "0Gi,5Gi" which means capacity or size matches from 0 to 5Gi, including value 0 and value 5Gi
  - ",5Gi" which is equal to "0,5Gi"
  - "5Gi," which means capacity or size matches larger than 5Gi, including value 5Gi
  - "5Gi" which is not supported and will be failed in validating configuration.
 
### Configmap Reference
Currently, resources policies are defined in `BackupSpec` struct, it will be more and more bloated with adding more and more filters which makes the size of `Backup` CR bigger and bigger, so we want to store the resources policies in configmap, and `Backup` CRD reference to current configmap.

the `configmap` user created would be like this:
```yaml
apiVersion: v1
data:
 policies.yaml:
  ----
  version: v1
  volumePolicies:
  - conditions:
      capacity: "0,100Gi"
      csi:
        driver: ebs.csi.aws.com
        fsType: ext4
      storageClass:
      - gp2
      - ebs-sc
    action:
      type: volume-snapshot
      parameters:
        volume-snapshot-timeout: "6h"
kind: ConfigMap
metadata:
 creationTimestamp: "2023-01-16T14:08:12Z"
 name: backup01
 namespace: velero
 resourceVersion: "17891025"
 uid: b73e7f76-fc9e-4e72-8e2e-79db717fe9f1
```

A new variable `resourcePolices` would be added into `BackupSpec`, it's value is assigned with the current resources policy configmap
```yaml
apiVersion: velero.io/v1
kind: Backup
metadata:
  name: backup-1
  spec:
    resourcePolices:
      refType: Configmap
      ref: backup01
    ...
```
The configmap only stores those assigned values, not the whole resources policies.
 
The name of the configmap is `$BackupName`, and it's in Velero install namespace.
 
#### Resource policies configmap related
The life cycle of resource policies configmap is managed by the user instead of Velero, which could make it more flexible and easy to maintain.
- The resource policies configmap will remain in the cluster until the user deletes it.
- Unlike backup, the resource policies configmap will not sync to the new cluster. So if the user wants to use one resource policies that do not sync to the new cluster, the backup will fail with resource policies not found.
- One resource policies configmap could be used by multiple backups.
- If the backup referenced resource policies configmap is been deleted, it won't affect the already existing backups, but if the user wants to reference the deleted configmap to create one new backup, it will fail with resource policies not found.

#### Versioning
We want to introduce the version field in the YAML data to contain break changes. Therefore, we won't follow a semver paradigm, for example in v1.11 the data look like this:
```yaml
version: v1
volumePolicies:
  ....
```
Hypothetically, in v1.12 we add new fields like clusterResourcePolicies, the version will remain as v1 b/c this change is backward compatible:
```yaml
version: v1
volumePolicies:
  ....
clusterResourcePolicies:
  ....
```
Suppose in v1.13, we have to introduce a break change, at this time we will bump up the version:
```yaml
version: v2
# This is just an example, we should try to avoid break change
volume-policies: 
  ....
```
We only support one version in Velero, so it won't be recognized if backup using a former version of YAML data.

#### Multiple versions supporting
To manage the effort for maintenance, we will only support one version of the data in Velero. Suppose that there is one break change for the YAML data in Velero v1.13, we should bump up the config version to v2, and v2 is only supported in v1.13. For the existing data with version: v1, it should migrate them when the Velero startup, this won't hurt the existing backup schedule CR as it only references the configmap. To make the migration easier, the configmap for such resource filter policies should be labeled manually before Velero startup like this, Velero will migrate the labeled configmap.

We only support migrating from the previous version to the current version in case of complexity in data format conversion, which users could regenerate configmap in the new YAML data version, and it is easier to do version control.

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  labels:
# This label can be optional but if this is not set, the backup will fail after the breaking change and the user will need to update the data manually 
    velero.io/resource-filter-policies: "true"
  name: example
  namespace: velero
data:
  .....
```
### Display of resources policies
As the resource policies configmap is referenced by backup CR, the policies in configmap are not so intuitive, so we need to integrate policies in configmap to the output of the command `velero backup describe`, and make it more readable.
 
## Compatibility
Currently, we have these resources filters:
- IncludedNamespaces
- ExcludedNamespaces
- IncludedResources
- ExcludedResources
- LabelSelector
- OrLabelSelectors
- IncludeClusterResources
- UseVolumeSnapshots
- velero.io/exclude-from-backup=true
- backup.velero.io/backup-volumes-excludes
- backup.velero.io/backup-volumes
- backup.velero.io/must-include-additional-items
 
So it should be careful with the combination of volumes resources policies and the above resources filters.
- When volume resource policies conflict with the above resource filters, we should respect the above resource filters. For example, if the user used the opt-out approach to `backup.velero.io/backup-volumes-excludes` annotation on the pod and also defined include volume in volumes resources filters configuration, we should respect the opt-out approach to skip backup of the volume.
- If volume resource policies conflict with themselves, the first matched policy will be respect.

## Implementation
This implementation should be included in Velero v1.11.0

Currently, in Velero v1.11.0 we only support `Action`
 `skip`, and support `file-system-backup` and `volume-snapshot` for the later version. And `Parameters` in `Action` is also not supported in v1.11.0, we will support in a later version.

In Velero 1.11, we supported Conditions and format listed below:
 - capacity
    ```yaml
    capacity: "10Gi,100Gi" // match volume has the size between 10Gi and 100Gi
    ```
 - storageClass
    ```yaml
    storageClass: // match volume has the storage class gp2 or ebs-sc
     - gp2
     - ebs-sc
    ```
- volume sources (currently only support below format and attributes)
  1. Specify the volume source name, the name could be `nfs`, `rbd`, `iscsi`, `csi` etc.
     ```yaml
     nfs : {} // match any volume has nfs volume source
     
     csi : {} // match any volume has csi volume source
     ```

  2. Specify details for the related volume source (currently we only support csi driver filter and nfs server or path filter)
     ```yaml
     csi: // match volume has nfs volume source and using `aws.efs.csi.driver`
       driver: aws.efs.csi.driver 
     
     nfs: // match volume has nfs volume source and using below server and path
       server: 192.168.200.90
       path: /mnt/nfs
     ```
The conditions also could be extended in later versions, such as we could further supporting filtering other volume source detail not only NFS and CSI.

## Alternatives Considered
### Configmap VS CRD
Here we support the user define the YAML config file and storing the resources policies into configmap, also we could define one resource's policies CRD and store policies imported from the user-defined config file in the related CR.

But CRD is more like one kind of resource with status, Kubernetes API Server handles the lifecycle of a CR and handles it in different statuses. Compared to CRD, Configmap is more focused to store data.

## Open Issues
Should we support more than one version of filter policies configmap?