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
1. Deleting with **`velero restore delete`**.
This command will delete the custom resource representing it, along with its individual log and results files. But, it will not delete any objects that were created by it from your cluster.
2. Deleting with **`kubectl -n velero delete restore`**.
This command will delete the custom resource representing the restore, but will not delete log/results files from object storage, or any objects that were created during the restore in your cluster.

## Restore command-line options
To see all commands for restores, run : `velero restore --help`
To see all options associated with a specific command, provide the --help flag to that command. For example,  **`velero restore create --help`** shows all options associated with the **create** command.

To list all options of restore, use **`velero restore --help`**

```Usage:
  velero restore [command]

Available Commands:
  create      Create a restore
  delete      Delete restores
  describe    Describe restores
  get         Get restores
  logs        Get restore logs
```

## What happens to NodePorts when restoring Services

**Auto assigned** NodePorts **deleted** by default and Services get new **auto assigned** nodePorts after restore.

**Explicitly specified** NodePorts auto detected using **`last-applied-config`** annotation and **preserved** after restore.  NodePorts can be explicitly specified as .spec.ports[*].nodePort field on Service definition.

#### Always Preserve NodePorts

It is not always possible to set nodePorts explicitly on some big clusters because of operation complexity. Official Kubernetes documents states that preventing port collisions is responsibility of the user when explicitly specifying nodePorts:

```
If you want a specific port number, you can specify a value in the `nodePort` field. The control plane will either allocate you that port or report that the API transaction failed. This means that you need to take care of possible port collisions yourself. You also have to use a valid port number, one that's inside the range configured for NodePort use.

https://kubernetes.io/docs/concepts/services-networking/service/#nodeport
```

The clusters which are not explicitly specifying nodePorts still may need to restore original NodePorts in case of disaster. Auto assigned nodePorts most probably defined on Load Balancers which located front side of cluster.  Changing all these nodePorts on Load Balancers is another operation complexity after disaster if nodePorts are changed.

Velero has a flag to let user deciding the preservation of nodePorts. **`velero restore create`** sub command has  **`--preserve-nodeports`** flag to **preserve** Service nodePorts **always** regardless of nodePorts **explicitly specified** or **not**. This flag used for preserving the original nodePorts from backup and can be used as **`--preserve-nodeports`** or **`--preserve-nodeports=true`**

If this flag given and/or set to true, Velero does not remove the nodePorts when restoring Service and tries to use the nodePorts which written on backup.

Trying to preserve nodePorts may cause **port conflicts** when restoring on situations below:

- If the nodePort from the backup already allocated on the target cluster then Velero prints error log as shown below and continue to restore operation.

  ```
  time="2020-11-23T12:58:31+03:00" level=info msg="Executing item action for services" logSource="pkg/restore/restore.go:1002" restore=velero/test-with-3-svc-20201123125825

  time="2020-11-23T12:58:31+03:00" level=info msg="Restoring Services with original NodePort(s)" cmd=_output/bin/linux/amd64/velero logSource="pkg/restore/service_action.go:61" pluginName=velero restore=velero/test-with-3-svc-20201123125825

  time="2020-11-23T12:58:31+03:00" level=info msg="Attempting to restore Service: hello-service" logSource="pkg/restore/restore.go:1107" restore=velero/test-with-3-svc-20201123125825

  time="2020-11-23T12:58:31+03:00" level=error msg="error restoring hello-service: Service \"hello-service\" is invalid: spec.ports[0].nodePort: Invalid value: 31536: provided port is already allocated" logSource="pkg/restore/restore.go:1170" restore=velero/test-with-3-svc-20201123125825
  ```



- If the nodePort from the backup is not in the nodePort range of target cluster then Velero prints error log as below and continue to restore operation. Kubernetes default nodePort range is 30000-32767 but on the example cluster nodePort range is 20000-22767 and tried to restore Service with nodePort 31536

  ```
  time="2020-11-23T13:09:17+03:00" level=info msg="Executing item action for services" logSource="pkg/restore/restore.go:1002" restore=velero/test-with-3-svc-20201123130915

  time="2020-11-23T13:09:17+03:00" level=info msg="Restoring Services with original NodePort(s)" cmd=_output/bin/linux/amd64/velero logSource="pkg/restore/service_action.go:61" pluginName=velero restore=velero/test-with-3-svc-20201123130915

  time="2020-11-23T13:09:17+03:00" level=info msg="Attempting to restore Service: hello-service" logSource="pkg/restore/restore.go:1107" restore=velero/test-with-3-svc-20201123130915

  time="2020-11-23T13:09:17+03:00" level=error msg="error restoring hello-service: Service \"hello-service\" is invalid: spec.ports[0].nodePort: Invalid value: 31536: provided port is not in the valid range. The range of valid ports is 20000-22767" logSource="pkg/restore/restore.go:1170" restore=velero/test-with-3-svc-20201123130915
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
    # config for a plugin (i.e. the built-in restore item action plugin)
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

## Changing PVC selected-node

Velero can update the selected-node annotation of persistent volume claim during restores, if selected-node doesn't exist in the cluster then it will remove the selected-node annotation from PersistentVolumeClaim. To configure a node mapping, create a config map in the Velero namespace like the following:

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  # any name can be used; Velero uses the labels (below)
  # to identify it rather than the name
  name: change-pvc-node-selector-config
  # must be in the velero namespace
  namespace: velero
  # the below labels should be used verbatim in your
  # ConfigMap.
  labels:
    # this value-less label identifies the ConfigMap as
    # config for a plugin (i.e. the built-in restore item action plugin)
    velero.io/plugin-config: ""
    # this label identifies the name and kind of plugin
    # that this ConfigMap is for.
    velero.io/change-pvc-node-selector: RestoreItemAction
data:
  # add 1+ key-value pairs here, where the key is the old
  # node name and the value is the new node name.
  <old-node-name>: <new-node-name>
```
