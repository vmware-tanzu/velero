---
title: "BackupPVC Configuration for Data Movement Backup"
layout: docs
---

`BackupPVC`  is an intermediate PVC to access data from during the data movement backup operation.

In some scenarios users may need to configure some advanced options of the backupPVC so that the data movement backup
operation could perform better. Specifically:
- For some storage providers, when creating a read-only volume from a snapshot, it is very fast; whereas, if a writable volume
  is created from the snapshot, they need to clone the entire disk data, which is time consuming. If the `backupPVC`'s `accessModes` is
  set as `ReadOnlyMany`, the volume driver is able to tell the storage to create a read-only volume, which may dramatically shorten the
  snapshot expose time. On the other hand,  `ReadOnlyMany` is not supported by all volumes. Therefore, users should be allowed to configure
  the `accessModes` for the `backupPVC`.
- Some storage providers create one or more replicas when creating a volume, the number of replicas is defined in the storage class.
  However, it doesn't make any sense to keep replicas when an intermediate volume used by the backup. Therefore, users should be allowed
  to configure another storage class specifically used by the `backupPVC`.

Velero introduces a new section in the node agent configuration configMap (the name of this configMap is passed using `--node-agent-config` velero server argument)
called `backupPVC`, through which you can specify the following
configurations:

- `storageClass`: This specifies the storage class to be used for the backupPVC. If this value does not exist or is empty then by 
default the source PVC's storage class will be used.

- `readOnly`: This is a boolean value. If set to `true` then `ReadOnlyMany` will be the only value set to the backupPVC's access modes. Otherwise 
`ReadWriteOnce` value will be used.

A sample of `backupPVC` config as part of the configMap would look like:
```json
{
    "backupPVC": {
        "storage-class-1": {
            "storageClass": "backupPVC-storage-class",
            "readOnly": true
        },
        "storage-class-2": {
            "storageClass": "backupPVC-storage-class"
        },
        "storage-class-3": {
            "readOnly": true
        }        
    }
}
```

**Note:** 
- Users should make sure that the storage class specified in `backupPVC` config should exist in the cluster and can be used by the
`backupPVC`, otherwise the corresponding DataUpload CR will stay in `Accepted` phase until timeout (data movement prepare timeout value is 30m by default).
- If the users are setting `readOnly` value as `true` in the `backupPVC` config then they must also make sure that the storage class that is being used for 
`backupPVC` should support creation of `ReadOnlyMany` PVC from a snapshot, otherwise the corresponding DataUpload CR will stay in `Accepted` phase until
timeout (data movement prepare timeout value is 30m by default).
- If any of the above problems occur, then the DataUpload CR is `canceled` after timeout, and the backupPod and backupPVC will be deleted, and the backup
will be marked as `PartiallyFailed`.
