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
  velero restore create <restore-name> --include-namespaces <namespace1>,<namespace2> --from-backup <backup-name>
  ```

### --include-resources

Kubernetes resources to include in the backup, formatted as resource.group, such as storageclasses.storage.k8s.io (use `*` for all resources).

* Backup all deployments in the cluster.

  ```bash
  velero backup create <backup-name> --include-resources deployments
  ```

* Restore all deployments and configmaps in the cluster.

  ```bash
  velero restore create <restore-name> --include-resources deployments,configmaps --from-backup <backup-name>
  ```

* Backup the deployments in a namespace.

  ```bash
  velero backup create <backup-name> --include-resources deployments --include-namespaces <namespace>
  ```

### --include-cluster-resources

Includes cluster-scoped resources. This option can have three possible values:

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
  velero restore create <restore-name> --include-cluster-resources=false --from-backup <backup-name>
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
  velero restore create <restore-name> --exclude-namespaces <namespace1>,<namespace2> --from-backup <backup-name>
  ```

### --exclude-resources

Kubernetes resources to exclude, formatted as resource.group, such as storageclasses.storage.k8s.io.

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
