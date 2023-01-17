# Handle backup of volumes by resources filters
 
## Abstract
Currently, Velero doesn't have one flexible way to filter volumes.
 
If users want to skip backup of volumes or only backup some volumes in different namespaces in batch, currently they need to use the opt-in and opt-out approach one by one, or use label-selector but if it has big different labels on each different related pods, which is cumbersome when they have lots of volumes to handle with. it would be convenient if Velero could provide one way to filter the backup of volumes just by `some specific volumes attributes`.
 
Also, currently, it's not accurate enough if the users want to select a specific volume to do a backup or skip by without patching labels or annotations to the pods. It would be useful if users could accurately select target volume by `one specific resource selector`. Users could accurately select the volume to backup or skip in their own console when using velero for secondary development.
 
## Background
As of Today, Velero has lots of filters to handle (backup or skip backup) resources including resources filters like `IncludedNamespaces, ExcludedNamespaces`, label selectors like `LabelSelector, OrLabelSelectors`, annotation like `backup.velero.io/must-include-additional-items` etc. But it's not enough flexible to handle volumes, we need one generic way to filter volumes.
 
## Goals
- Introducing one flexible way to filter volumes.
 
## Non Goals
- We only handle volumes for backup and do not support restore.
- Currently, only handle volumes, does not support other resources.
- Only environment-unrelated and platform-independent general volumes attributes are supported, do not support volumes attributes related to a specific environment.
 
## Use-cases/Scenarios
### A. Skip backup volumes by some attributes
Users want to skip PV with the requirements:
- option to skip all PV data
- option to skip specified PV type (RBD, NFS)
- option to skip specified PV size
- option to skip specified storage-class
- option to skip folders
 
### B. Accurately select target volume to backup
Some volumes are only used for logging while others volumes are for DBs, and only need to backup DBs data. users need `one specific resource seletor` to accurately select target volumes to backup.
 
## High-Level Design
Add a new flag `pv-backup-policy-configmap` when executing `velero backup create`, which imports the defined resources filters in one YAML file. the YAML file including all defined filter rules for the current backup.
 
When Velero handles volumes backup should respect the filter rules defined in the imported YAML file.
 
## Detailed Design
The resources filters rules should contain both `include` and `exclude` rules.
 
For the rules on `one specific resource selector`, we introduced a `GVRN` way of resources filters, for resources are identified by their resource type and resource name, or GVRN.

Here we call it `GVRN Selector` which exactly matches the resources to be handled.

For the attributes on `some specific volumes attributes`, we basically follow the defined data struct [PersistentVolumeSpec](https://github.com/kubernetes/kubernetes/blob/v1.26.0/pkg/apis/core/types.go#L304), and only handle partial common fields of it currently.
 
Here we call it `Volumes Attributes Selector`, which matches volumes with the same attributes defined.
 
### filter fields format
The filter YAML config file would look like this:
```
---
resources:
  - include:
      groupResource: "/persistentvolumes"
      namespacedNames: "/nginx-logs"
  - include:
      groupResource: "/persistentvolumes"
      namespacedNames: "/minio"
  - exclude:
      groupResource: apps/deployments
      namespacedNames: velero/velero
  - exclude: 
      groupResource: apps/demonset
      namespacedNames: velero/node-agent
storage:
  pv:
    - include:
        storageClassName: gp2, ebs-sc
        volumeMode: block, filesystem
        capacity: OGi,5Gi
        persistentVolumeSource:
          nfs:
            readOnly: true
          csi:
            driver: aws.efs.csi.driver
    - include:
        storageClassName: io1
        persistentVolumeSource:
          csi:
            driver: aws.efs.csi.driver
    - exclude:
        storageClassName: efs-dynamic-sc
        capacity: 5Gi,
        persistentVolumeSource:
          nfs: {}
          csi:
            driver: aws.ebs.csi.driver
    - exclude:
        storageClassName: fsx-lustre
        volumeMode: block, filesystem
        persistentVolumeSource:
          csi:
            driver: csi.aws.amazon.com
```
 
### Filter rules
The whole filter file consists of two parts: resources and storage.
 
Both `Kopia, Restic` and `Volume snapshot` share one YAML configuration file.
 
#### resources
In the resources part, we defined `GVRN Selector` to filter resources. In a filter, an empty or omitted group, version, resource type, or resource name matches any value. `GVRN selector` could match Persistent Volume and other Kubernetes resources.

Taking select PV as an example, if users want to backup PV with name nginx-logs, the `groupResource` could be "/persistentvolumes" in which the group should be empty, the `namespacedNames` could be "/nginx-logs" in which the namespace should be empty.

#### storage
In the storage part, we defined `Volumes Attributes Selector` to filter resources.
 
The storage part defined rules including `pv` and `volume`, which correspond to `Kopia, Restic` and `Volume snapshot`.
 
A filter in storage with a specific key and empty value, which means the value matches any value. For example, if the `storage.pv.exclude.persistentVolumeSource.nfs` is `{}` it means if `NFS` is used as `persistentVolumeSource` in Persistent Volume will be skipped no matter what the NFS server or NFS Path is,  
 
A filter may have multiple values, all the values are concatenated by commas. For example, the `storage.pv.include.storageClassName` is `gp2, ebs-sc` which means Persistent Volume with gp2 or ebs-sc storage class both will be back up.
 
The size of each single filter value should limit to 256 bytes in case of an unfriendly long variable assignment.
 
If user defined pv filter rules but used Kopia or Restic to do a backup, the backup will fail in validating the resource filter configuration. Same as the situation if using defined volume filter rules but using CSI or plugins to take volume snapshots.
 
For capacity in `pv` or size in `volume`, the value should include the lower value and upper value concatenated by commas. And it has several combinations below:
- "0,5Gi" or "0Gi,5Gi" which means capacity or size matches from 0 to 5Gi, including value 0 and value 5Gi
- ",5Gi" which is equal to "0,5Gi"
- "5Gi," which means capacity or size matches larger than 5Gi, including value 5Gi
- "5Gi" which is not supported and will be failed in validating configuration.
 
### Filter Reference
Currently, resources filters are defined in `BackupSpec` struct, it will be more and more bloated with adding more and more filters which makes the size of `Backup` CR bigger and bigger, so we want to store the resources rules in configmap, and `Backup` CRD reference to current configmap.

the `configmap` would be like this:
```
apiVersion: v1
data:
 filter.YAML: |-
   {
       "resources": {
           "include": {
              ...
           },
           "exclude": {
              ...
           }
       },
       "storage": {
           "pv": {
               "include": {
                  ...
               },
               "exclude": {
                  ...
               }
           },
           "volume": {
               "include": {
                  ...
               },
               "exclude": {
                  ...
               }
           }
       }
   }
kind: ConfigMap
metadata:
 creationTimestamp: "2023-01-16T14:08:12Z"
 name: backup01
 namespace: velero
 resourceVersion: "17891025"
 uid: b73e7f76-fc9e-4e72-8e2e-79db717fe9f1
```

a new variable `filterConfigmap` would be added into `BackupSpec`, it's value is assigned with current resources filters configmap
```
apiVersion: velero.io/v1
kind: Backup
metadata:
  name: backup-1
  spec:
    resourcesFilter:
      refType: Configmap
      ref: backup01
    ...
```

The configmap basically equivalent to generated by command `kubectl create cm backup01 --from-file filter.yaml`
 
The configmap only stores those filters assigned value not the whole resources filters.
 
The name of the configmap is `$BackupName`, and it's in Velero install namespace.
 
#### Life-cycle of resource filter configmap
- Resource filter configmap only been generated once Velero backup command with flag `pv-backup-policy-configmap` to import configuration.
- If the referenced backup has been deleted, the relevant configmap should be removed.
 
### Display of volume resource filter
As the resource filter configmap is referenced by backup CR, the rules in configmap are not so intuitive, so we need to integrate rules in configmap to the output of the command `velero backup describe`, and make it more readable.
 
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
 
So it should be careful with the combination of volumes resources filter rules and the above resources filters.
- When volumes resources filter rules conflict with the above resources filters, we should respect the above resources filters. For example, if the user used the opt-out approach to `backup.velero.io/backup-volumes-excludes` annotation on the pod and also defined include volume in volumes resources filters configuration, we should respect the opt-out approach to skip backup of the volume.
- The filtered resources would be the intersection of the result with the above resources filters and volumes resources filters. For example, if user defined `IncludedNamespaces=nginx-example` and also included PV with `storageClassName=gp2`, which results in backing up the volume in nginx-example.
 
## Implementation
This implementation should be included in Velero v1.11.0