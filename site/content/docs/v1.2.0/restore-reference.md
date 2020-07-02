---
title: "Restore Reference"
layout: docs
---

## Restoring Into a Different Namespace

Velero can restore resources into a different namespace than the one they were backed up from. To do this, use the `--namespace-mappings` flag:

```bash
velero restore create RESTORE_NAME \
  --from-backup BACKUP_NAME \
  --namespace-mappings old-ns-1:new-ns-1,old-ns-2:new-ns-2
```
## What happens when user removes restore objects
A **restore** object represents the restore operation. There are two types of deletion for restore objects:
### 1. Deleting with **`velero restore delete`**
This command will delete the custom resource representing it, along with its individual log and results files. But, it will not delete any objects that were created by it from your cluster.
### 2. Deleting with **`kubectl -n velero delete restore`**
This command will delete the custom resource representing the restore, but will not delete log/results files from object storage, or any objects that were created during the restore in your cluster.

## Restore command-line options
To see all commands for restores, run : `velero restore --help`
To see all options associated with a specific command, provide the --help flag to that command. For example,  **`velero restore create --help`** shows all options associated with the **create** command.

### To List all options of restore use : **`velero restore --help`**

```Usage:
  velero restore [command]

Available Commands:
  create      Create a restore
  delete      Delete restores
  describe    Describe restores
  get         Get restores
  logs        Get restore logs
```

## Changing PV/PVC Storage Classes

Velero can change the storage class of persistent volumes and persistent volume claims during restores. To configure a storage class mapping, create a config map in the Velero namespace like the following:

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  # any name can be used; Velero uses the labels (below)
  # to identify it rather than the name
  name: change-storage-class-config
  # must be in the velero namespace
  namespace: velero
  # the below labels should be used verbatim in your
  # ConfigMap.
  labels:
    # this value-less label identifies the ConfigMap as
    # config for a plugin (i.e. the built-in change storage
    # class restore item action plugin)
    velero.io/plugin-config: ""
    # this label identifies the name and kind of plugin
    # that this ConfigMap is for.
    velero.io/change-storage-class: RestoreItemAction
data:
  # add 1+ key-value pairs here, where the key is the old
  # storage class name and the value is the new storage
  # class name.
  <old-storage-class>: <new-storage-class>
```
