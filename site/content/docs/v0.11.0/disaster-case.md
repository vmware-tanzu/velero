---
title: "Disaster recovery"
layout: docs
---

*Using Schedules and Restore-Only Mode*

If you periodically back up your cluster's resources, you are able to return to a previous state in case of some unexpected mishap, such as a service outage. Doing so with Velero looks like the following:

1.  After you first run the Velero server on your cluster, set up a daily backup (replacing `<SCHEDULE NAME>` in the command as desired):

    ```
    velero schedule create <SCHEDULE NAME> --schedule "0 7 * * *"
    ```
    This creates a Backup object with the name `<SCHEDULE NAME>-<TIMESTAMP>`.

1.  A disaster happens and you need to recreate your resources.

1.  Update the Velero server deployment, adding the argument for the `server` command flag `restore-only` set to `true`. This prevents Backup objects from being created or deleted during your Restore process.

1.  Create a restore with your most recent Velero Backup:
    ```
    velero restore create --from-backup <SCHEDULE NAME>-<TIMESTAMP>
    ```



