---
title: "Use Cases"
layout: docs
---

This doc provides sample Ark commands for the following common scenarios:
* [Disaster recovery][0]
* [Cluster migration][1]

## Disaster recovery

*Using Schedules and Restore-Only Mode*

If you periodically back up your cluster's resources, you are able to return to a previous state in case of some unexpected mishap, such as a service outage. Doing so with Heptio Ark looks like the following:

1. After you first run the Ark server on your cluster, set up a daily backup (replacing `<SCHEDULE NAME>` in the command as desired):

    ```
    ark schedule create <SCHEDULE NAME> --schedule "0 7 * * *"
    ```
    This creates a Backup object with the name `<SCHEDULE NAME>-<TIMESTAMP>`.

2. A disaster happens and you need to recreate your resources.

3. Update the [Ark server Config][3], setting `restoreOnlyMode` to `true`. This prevents Backup objects from being created or deleted during your Restore process.

4. Create a restore with your most recent Ark Backup:
    ```
    ark restore create <SCHEDULE NAME>-<TIMESTAMP>
    ```

## Cluster migration

*Using Backups and Restores*

Heptio Ark can help you port your resources from one cluster to another, as long as you point each Ark Config to the same cloud object storage. In this scenario, we are also assuming that your clusters are hosted by the same cloud provider. **Note that Heptio Ark does not support the migration of persistent volumes across cloud providers.**

1. *(Cluster 1)* Assuming you haven't already been checkpointing your data with the Ark `schedule` operation, you need to first back up your entire cluster (replacing `<BACKUP-NAME>` as desired):

   ```
   ark backup create <BACKUP-NAME>
   ```
   The default TTL is 30 days (720 hours); you can use the `--ttl` flag to change this as necessary.

2. *(Cluster 2)* Make sure that the `persistentVolumeProvider` and `backupStorageProvider` fields in the Ark Config match the ones from *Cluster 1*, so that your new Ark server instance is pointing to the same bucket.

3. *(Cluster 2)* Make sure that the Ark Backup object has been created. Ark resources are [synced][2] with the backup files available in cloud storage.

4. *(Cluster 2)* Once you have confirmed that the right Backup (`<BACKUP-NAME>`) is now present, you can restore everything with:
```
ark restore create <BACKUP-NAME>
```

[0]: #disaster-recovery
[1]: #cluster-migration
[2]: concepts.md#cloud-storage-sync
[3]: config-definition.md#main-config-parameters
