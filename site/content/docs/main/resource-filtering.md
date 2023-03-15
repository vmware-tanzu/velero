---
title: "Resource filtering"
layout: docs
---

*Filter objects by namespace, type, or labels.*

This page describes how to use the include and exclude flags with the `velero backup` and `velero restore` commands. By default Velero includes all objects in a backup or restore when no filtering options are used. 

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

Kubernetes resources to include in the backup, formatted as resource.group, such as storageclasses.storage.k8s.io (use `*` for all resources). Cannot work with `--include-cluster-scope-resources`, `--exclude-cluster-scope-resources`, `--include-namespaced-resources` and `--exclude-namespaced-resources`.

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

Includes cluster-scoped resources. Cannot work with `--include-cluster-scope-resources`, `--exclude-cluster-scope-resources`, `--include-namespaced-resources` and `--exclude-namespaced-resources`. This option can have three possible values:

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
  velero backup create <backup-name> --selector <key>!=<value>
  ```

For more information read the [Kubernetes label selector documentation](https://kubernetes.io/docs/concepts/overview/working-with-objects/labels/#label-selectors)

### --include-cluster-scope-resources
Kubernetes cluster-scoped resources to include in the backup, formatted as resource.group, such as `storageclasses.storage.k8s.io`(use '*' for all resources). Cannot work with `--include-resources`, `--exclude-resources` and `--include-cluster-resources`. This parameter only works for backup, not for restore.

* Backup all StorageClasses and ClusterRoles in the cluster.

  ```bash
  velero backup create <backup-name> --include-cluster-scope-resources="storageclasses,clusterroles"
  ```

* Backup all cluster-scoped resources in the cluster.

  ```bash
  velero backup create <backup-name> --include-cluster-scope-resources="*"
  ```


### --include-namespaced-resources
Kubernetes namespace resources to include in the backup, formatted as resource.group, such as `deployments.apps`(use '*' for all resources). Cannot work with `--include-resources`, `--exclude-resources` and `--include-cluster-resources`. This parameter only works for backup, not for restore.

* Backup all Deployments and ConfigMaps in the cluster.

  ```bash
  velero backup create <backup-name> --include-namespaced-resources="deployments.apps,configmaps"
  ```

* Backup all namespace resources in the cluster.

  ```bash
  velero backup create <backup-name> --include-namespaced-resources="*"
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

Kubernetes resources to exclude, formatted as resource.group, such as storageclasses.storage.k8s.io. Cannot work with `--include-cluster-scope-resources`, `--exclude-cluster-scope-resources`, `--include-namespaced-resources` and `--exclude-namespaced-resources`.

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

### --exclude-cluster-scope-resources
Kubernetes cluster-scoped resources to exclude from the backup, formatted as resource.group, such as `storageclasses.storage.k8s.io`(use '*' for all resources). Cannot work with `--include-resources`, `--exclude-resources` and `--include-cluster-resources`. This parameter only works for backup, not for restore.

* Exclude StorageClasses and ClusterRoles from the backup.

  ```bash
  velero backup create <backup-name> --exclude-cluster-scope-resources="storageclasses,clusterroles"
  ```

* Exclude all cluster-scoped resources from the backup.

  ```bash
  velero backup create <backup-name> --exclude-cluster-scope-resources="*"
  ```

### --exclude-namespaced-resources
Kubernetes namespace resources to exclude from the backup, formatted as resource.group, such as `deployments.apps`(use '*' for all resources). Cannot work with `--include-resources`, `--exclude-resources` and `--include-cluster-resources`. This parameter only works for backup, not for restore.

* Exclude all Deployments and ConfigMaps from the backup.

  ```bash
  velero backup create <backup-name> --exclude-namespaced-resources="deployments.apps,configmaps"
  ```

* Exclude all namespace resources from the backup.

  ```bash
  velero backup create <backup-name> --exclude-namespaced-resources="*"
  ```
