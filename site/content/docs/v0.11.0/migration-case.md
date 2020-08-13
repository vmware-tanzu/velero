---
title: "Cluster migration"
layout: docs
---

*Using Backups and Restores*

Velero can help you port your resources from one cluster to another, as long as you point each Velero instance to the same cloud object storage location. In this scenario, we are also assuming that your clusters are hosted by the same cloud provider. **Note that Velero does not support the migration of persistent volumes across cloud providers.**

1.  *(Cluster 1)* Assuming you haven't already been checkpointing your data with the Velero `schedule` operation, you need to first back up your entire cluster (replacing `<BACKUP-NAME>` as desired):

    ```
    velero backup create <BACKUP-NAME>
    ```
    The default TTL is 30 days (720 hours); you can use the `--ttl` flag to change this as necessary.

1.  *(Cluster 2)* Add the `--restore-only` flag to the server spec in the Velero deployment YAML.

1.  *(Cluster 2)* Make sure that the `BackupStorageLocation` and `VolumeSnapshotLocation` CRDs match the ones from *Cluster 1*, so that your new Velero server instance points to the same bucket.

1.  *(Cluster 2)* Make sure that the Velero Backup object is created. Velero resources are synchronized with the backup files in cloud storage.

    ```
    velero backup describe <BACKUP-NAME>
    ```

    **Note:** As of version 0.10, the default sync interval is 1 minute, so make sure to wait before checking. You can configure this interval with the `--backup-sync-period` flag to the Velero server.

1.  *(Cluster 2)* Once you have confirmed that the right Backup (`<BACKUP-NAME>`) is now present, you can restore everything with:

    ```
    velero restore create --from-backup <BACKUP-NAME>
    ```

## Verify both clusters

Check that the second cluster is behaving as expected:

1.  *(Cluster 2)* Run:

    ```
    velero restore get
    ```

1.  Then run:

    ```
    velero restore describe <RESTORE-NAME-FROM-GET-COMMAND>
    ```

If you encounter issues, make sure that Velero is running in the same namespace in both clusters.
