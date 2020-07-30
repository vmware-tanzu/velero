# Cluster migration

*Using Backups and Restores*

Heptio Ark can help you port your resources from one cluster to another, as long as you point each Ark instance to the same cloud object storage location. In this scenario, we are also assuming that your clusters are hosted by the same cloud provider. **Note that Heptio Ark does not support the migration of persistent volumes across cloud providers.**

1.  *(Cluster 1)* Assuming you haven't already been checkpointing your data with the Ark `schedule` operation, you need to first back up your entire cluster (replacing `<BACKUP-NAME>` as desired):

    ```
    ark backup create <BACKUP-NAME>
    ```
    The default TTL is 30 days (720 hours); you can use the `--ttl` flag to change this as necessary.

1.  *(Cluster 2)* Add the `--restore-only` flag to the server spec in the Ark deployment YAML.

1.  *(Cluster 2)* Make sure that the `BackupStorageLocation` and `VolumeSnapshotLocation` CRDs match the ones from *Cluster 1*, so that your new Ark server instance points to the same bucket.

1.  *(Cluster 2)* Make sure that the Ark Backup object is created. Ark resources are synchronized with the backup files in cloud storage.

    ```
    ark backup describe <BACKUP-NAME>
    ```

    **Note:** As of version 0.10, the default sync interval is 1 minute, so make sure to wait before checking. You can configure this interval with the `--backup-sync-period` flag to the Ark server.

1.  *(Cluster 2)* Once you have confirmed that the right Backup (`<BACKUP-NAME>`) is now present, you can restore everything with:

    ```
    ark restore create --from-backup <BACKUP-NAME>
    ```

## Verify both clusters

Check that the second cluster is behaving as expected:

1.  *(Cluster 2)* Run:

    ```
    ark restore get
    ```

1.  Then run:

    ```
    ark restore describe <RESTORE-NAME-FROM-GET-COMMAND>
    ```

If you encounter issues, make sure that Ark is running in the same namespace in both clusters.
