---
title: "Resource filtering"
layout: docs
---

*Filter objects by namespace, type, labels or resource policies.*

This page describes how to filter resource for backup and restore.
User could use the include and exclude flags with the `velero backup` and `velero restore` commands. And user could also use resource policies to handle backup.
By default, Velero includes all objects in a backup or restore when no filtering options are used.

## Includes

Only specific resources are included, all others are excluded.

Wildcard takes precedence when both a wildcard and specific resource are included.

### --include-namespaces

Namespaces to include. Default is `*`, all namespaces.

* Backup a namespace and it's objects.

  ```bash
  velero backup create <backup-name> --include-namespaces <namespace>
  ```

* Restore two namespaces and their objects.

  ```bash
  velero restore create <backup-name> --include-namespaces <namespace1>,<namespace2>
  ```

### --include-resources

Kubernetes resources to include in the backup, formatted as resource.group, such as storageclasses.storage.k8s.io (use `*` for all resources). Cannot work with `--include-cluster-scoped-resources`, `--exclude-cluster-scoped-resources`, `--include-namespace-scoped-resources` and `--exclude-namespace-scoped-resources`.

* Backup all deployments in the cluster.

  ```bash
  velero backup create <backup-name> --include-resources deployments
  ```

* Restore all deployments and configmaps in the cluster.

  ```bash
  velero restore create <backup-name> --include-resources deployments,configmaps
  ```

* Backup the deployments in a namespace.

  ```bash
  velero backup create <backup-name> --include-resources deployments --include-namespaces <namespace>
  ```

### --include-cluster-resources

Includes cluster-scoped resources. Cannot work with `--include-cluster-scoped-resources`, `--exclude-cluster-scoped-resources`, `--include-namespace-scoped-resources` and `--exclude-namespace-scoped-resources`. This option can have three possible values:

* `true`: all cluster-scoped resources are included.

* `false`: no cluster-scoped resources are included.

* `nil` ("auto" or not supplied):

  - Cluster-scoped resources are included when backing up or restoring all namespaces. Default: `true`.

  - Cluster-scoped resources are not included when namespace filtering is used. Default: `false`.

    * Some related cluster-scoped resources may still be backed/restored up if triggered by a custom action (for example, PVC->PV) unless `--include-cluster-resources=false`.

* Backup entire cluster including cluster-scoped resources.

  ```bash
  velero backup create <backup-name>
  ```

* Restore only namespaced resources in the cluster.

  ```bash
  velero restore create <backup-name> --include-cluster-resources=false
  ```

* Backup a namespace and include cluster-scoped resources.

  ```bash
  velero backup create <backup-name> --include-namespaces <namespace> --include-cluster-resources=true
  ```

### --selector

* Include resources matching the label selector.

  ```bash
  velero backup create <backup-name> --selector <key>=<value>
  ```
* Include resources that are not matching the selector
  ```bash
  velero backup create <backup-name> --selector "<key> notin (<value>)"
  ```

For more information read the [Kubernetes label selector documentation](https://kubernetes.io/docs/concepts/overview/working-with-objects/labels/#label-selectors)

### --include-cluster-scoped-resources
Kubernetes cluster-scoped resources to include in the backup, formatted as resource.group, such as `storageclasses.storage.k8s.io`(use '*' for all resources). Cannot work with `--include-resources`, `--exclude-resources` and `--include-cluster-resources`. This parameter only works for backup, not for restore.

* Backup all StorageClasses and ClusterRoles in the cluster.

  ```bash
  velero backup create <backup-name> --include-cluster-scoped-resources="storageclasses,clusterroles"
  ```

* Backup all cluster-scoped resources in the cluster.

  ```bash
  velero backup create <backup-name> --include-cluster-scoped-resources="*"
  ```


### --include-namespace-scoped-resources
Kubernetes namespace resources to include in the backup, formatted as resource.group, such as `deployments.apps`(use '*' for all resources). Cannot work with `--include-resources`, `--exclude-resources` and `--include-cluster-resources`. This parameter only works for backup, not for restore.

* Backup all Deployments and ConfigMaps in the cluster.

  ```bash
  velero backup create <backup-name> --include-namespace-scoped-resources="deployments.apps,configmaps"
  ```

* Backup all namespace resources in the cluster.

  ```bash
  velero backup create <backup-name> --include-namespace-scoped-resources="*"
  ```

## Excludes

Exclude specific resources from the backup.

Wildcard excludes are ignored.

### --exclude-namespaces

Namespaces to exclude.

* Exclude kube-system from the cluster backup.

  ```bash
  velero backup create <backup-name> --exclude-namespaces kube-system
  ```

* Exclude two namespaces during a restore.

  ```bash
  velero restore create <backup-name> --exclude-namespaces <namespace1>,<namespace2>
  ```

### --exclude-resources

Kubernetes resources to exclude, formatted as resource.group, such as storageclasses.storage.k8s.io. Cannot work with `--include-cluster-scoped-resources`, `--exclude-cluster-scoped-resources`, `--include-namespace-scoped-resources` and `--exclude-namespace-scoped-resources`.

* Exclude secrets from the backup.

  ```bash
  velero backup create <backup-name> --exclude-resources secrets
  ```

* Exclude secrets and rolebindings.

  ```bash
  velero backup create <backup-name> --exclude-resources secrets,rolebindings
  ```

### velero.io/exclude-from-backup=true

* Resources with the label `velero.io/exclude-from-backup=true` are not included in backup, even if it contains a matching selector label.

### --exclude-cluster-scoped-resources
Kubernetes cluster-scoped resources to exclude from the backup, formatted as resource.group, such as `storageclasses.storage.k8s.io`(use '*' for all resources). Cannot work with `--include-resources`, `--exclude-resources` and `--include-cluster-resources`. This parameter only works for backup, not for restore.

* Exclude StorageClasses and ClusterRoles from the backup.

  ```bash
  velero backup create <backup-name> --exclude-cluster-scoped-resources="storageclasses,clusterroles"
  ```

* Exclude all cluster-scoped resources from the backup.

  ```bash
  velero backup create <backup-name> --exclude-cluster-scoped-resources="*"
  ```

### --exclude-namespace-scoped-resources
Kubernetes namespace resources to exclude from the backup, formatted as resource.group, such as `deployments.apps`(use '*' for all resources). Cannot work with `--include-resources`, `--exclude-resources` and `--include-cluster-resources`. This parameter only works for backup, not for restore.

* Exclude all Deployments and ConfigMaps from the backup.

  ```bash
  velero backup create <backup-name> --exclude-namespace-scoped-resources="deployments.apps,configmaps"
  ```

* Exclude all namespace resources from the backup.

  ```bash
  velero backup create <backup-name> --exclude-namespace-scoped-resources="*"
  ```

## Resource policies
Velero provides resource policies to filter resources to do backup or restore. currently, it only supports skip backup volume by resource policies.

**Creating resource policies**

Below is the two-step of using resource policies to skip backup of volume:
1. Creating resource policies configmap

   Users need to create one configmap in Velero install namespace from a YAML file that defined resource policies. The creating command would be like the below:
   ```bash
   kubectl create cm <configmap-name> --from-file <yaml-file> -n velero
   ```
2. Creating a backup reference to the defined resource policies

   Users create a backup with the flag `--resource-policies-configmap`, which will reference the current backup to the defined resource policies. The creating command would be like the below:
   ```bash
   velero backup create --resource-policies-configmap <configmap-name>
   ```
   This flag could also be combined with the other include and exclude filters above

**YAML template**

Velero only support volume resource policies currently, other kinds of resource policies could be extended in the future. The policies YAML config file would look like this:
- Yaml template:
    ```yaml
    # currently only supports v1 version
    version: v1
    volumePolicies:
    # each policy consists of a list of conditions and an action
    # we could have lots of policies, but if the resource matched the first policy, the latters will be ignored
    # each key in the object is one condition, and one policy will apply to resources that meet ALL conditions
    # NOTE: capacity or storageClass is suited for [Persistent Volumes](https://kubernetes.io/docs/concepts/storage/persistent-volumes), and pod [Volume](https://kubernetes.io/docs/concepts/storage/volumes) not support it.
    - conditions:
        # capacity condition matches the volumes whose capacity falls into the range
        capacity: "10,100Gi"
        # pv matches specific csi driver
        csi:
          driver: aws.ebs.csi.driver
        # pv matches one of the storage class list
        storageClass:
        - gp2
        - standard
      action:
        type: skip
    - conditions:
        capacity: "0,100Gi"
        # nfs volume source with specific server and path (nfs could be empty or only config server or path)
        nfs:
          server: 192.168.200.90
          path: /mnt/data
      action:
        type: skip
    - conditions:
        nfs:
          server: 192.168.200.90
      action:
        type: skip
    - conditions:
        # nfs could be empty which matches any nfs volume source
        nfs: {}
      action:
        type: skip
    - conditions:
        # csi could be empty which matches any csi volume source
        csi: {}
      action:
        type: skip
    ```

**Supported conditions**

Currently, Velero supports the volume attributes listed below:
- capacity: matching volumes have the capacity that falls within this `capacity` range. The capacity value should include the lower value and upper value concatenated by commas, the unit of each value in capacity could be `Ti`, `Gi`, `Mi`, `Ki` etc, which is a standard storage unit in Kubernetes. And it has several combinations below:
  - "0,5Gi" or "0Gi,5Gi" which means capacity or size matches from 0 to 5Gi, including value 0 and value 5Gi
  - ",5Gi" which is equal to "0,5Gi"
  - "5Gi," which means capacity or size matches larger than 5Gi, including value 5Gi
  - "5Gi" which is not supported and will be failed in validating the configuration
- storageClass: matching volumes those with specified `storageClass`, such as `gp2`, `ebs-sc` in eks
- volume sources: matching volumes that used specified volume sources. Currently we support nfs or csi backend volume source

Velero supported conditions and format listed below:
- capacity
  ```yaml
  # match volume has the size between 10Gi and 100Gi
  capacity: "10Gi,100Gi"
  ```
- storageClass
  ```yaml
  # match volume has the storage class gp2 or ebs-sc
  storageClass:
    - gp2
    - ebs-sc
  ```
- volume sources (currently only support below format and attributes)
1. Specify the volume source name, the name could be `nfs`, `rbd`, `iscsi`, `csi` etc, but Velero only support `nfs` and `csi` currently.
    ```yaml
    # match any volume has nfs volume source
    nfs : {}
    # match any volume has csi volume source
    csi : {}
    ```

2. Specify details for the related volume source (currently we only support csi driver filter and nfs server or path filter)
    ```yaml
    # match volume has csi volume source and using `aws.efs.csi.driver`
    csi:
      driver: aws.efs.csi.driver 
    # match volume has nfs volume source and using below server and path
    nfs:
      server: 192.168.200.90
      path: /mnt/nfs
    ```
    For volume provisioned by [Persistent Volumes](https://kubernetes.io/docs/concepts/storage/persistent-volumes) support all above attributes, but for pod [Volume](https://kubernetes.io/docs/concepts/storage/volumes) only support filtered by volume source.

**Resource policies rules**
- Velero already has lots of include or exclude filters. the resource policies are the final filters after others include or exclude filters in one backup processing workflow. So if use a defined similar filter like the opt-in approach to backup one pod volume but skip backup of the same pod volume in resource policies, as resource policies are the final filters that are applied, the volume will not be backed up.
- If volume resource policies conflict with themselves the first matched policy will be respected when many policies are defined.