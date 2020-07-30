# Object Storage Layout Changes in v0.10

## Overview

Ark v0.10 includes breaking changes to where data is stored in your object storage bucket. You'll need to run a [one-time migration procedure](#upgrading-to-v010)
if you're upgrading from prior versions of Ark.

## Details

Prior to v0.10, Ark stored data in an object storage bucket using the following structure:

```
<your-bucket>/
    backup-1/
        ark-backup.json
        backup-1.tar.gz
        backup-1-logs.gz
        restore-of-backup-1-logs.gz
        restore-of-backup-1-results.gz
    backup-2/
        ark-backup.json
        backup-2.tar.gz
        backup-2-logs.gz
        restore-of-backup-2-logs.gz
        restore-of-backup-2-results.gz
    ...
```

Ark also stored restic data, if applicable, in a separate object storage bucket, structured as:

```
<your-ark-restic-bucket>/[<your-optional-prefix>/]
    namespace-1/
        data/
        index/
        keys/
        snapshots/
        config
    namespace-2/
        data/
        index/
        keys/
        snapshots/
        config
    ...
```

As of v0.10, we've reorganized this layout to provide a cleaner and more extensible directory structure. The new layout looks like:

```
<your-bucket>[/<your-prefix>]/
    backups/
        backup-1/
            ark-backup.json
            backup-1.tar.gz
            backup-1-logs.gz
        backup-2/
            ark-backup.json
            backup-2.tar.gz
            backup-2-logs.gz
        ...
    restores/
        restore-of-backup-1/
            restore-of-backup-1-logs.gz
            restore-of-backup-1-results.gz
        restore-of-backup-2/
            restore-of-backup-2-logs.gz
            restore-of-backup-2-results.gz
        ...
    restic/
        namespace-1/
            data/
            index/
            keys/
            snapshots/
            config
        namespace-2/
            data/
            index/
            keys/
            snapshots/
            config
        ...
    ...
```

## Upgrading to v0.10

Before upgrading to v0.10, you'll need to run a one-time upgrade script to rearrange the contents of your existing Ark bucket(s) to be compatible with
the new layout.

Please note that the following scripts **will not** migrate existing restore logs/results into the new `restores/` subdirectory. This means that they
will not be accessible using `ark restore describe` or `ark restore logs`. They *will* remain in the relevant backup's subdirectory so they are manually
accessible, and will eventually be garbage-collected along with the backup. We've taken this approach in order to keep the migration scripts simple 
and less error-prone.

### rclone-Based Script

This script uses [rclone][1], which you can download and install following the instructions [here][2]. 
Please read through the script carefully before starting and execute it step-by-step.

```bash
ARK_BUCKET=<your-ark-bucket>
ARK_TEMP_MIGRATION_BUCKET=<a-temp-bucket-for-migration>

# 1. This is an interactive step that configures rclone to be 
#    able to access your storage provider. Follow the instructions,
#    and keep track of the "remote name" for the next step:
rclone config

# 2. Store the name of the rclone remote that you just set up
#    in Step #1:
RCLONE_REMOTE_NAME=<your-remote-name>

# 3. Create a temporary bucket to be used as a backup of your 
#    current Ark bucket's contents:
rclone mkdir ${RCLONE_REMOTE_NAME}:${ARK_TEMP_MIGRATION_BUCKET}

# 4. Do a full copy of the contents of your Ark bucket into the 
#    temporary bucket:
rclone copy ${RCLONE_REMOTE_NAME}:${ARK_BUCKET} ${RCLONE_REMOTE_NAME}:${ARK_TEMP_MIGRATION_BUCKET}

# 5. Verify that the temporary bucket contains an exact copy of 
#    your Ark bucket's contents. You should see a short block
#    of output stating "0 differences found":
rclone check ${RCLONE_REMOTE_NAME}:${ARK_BUCKET} ${RCLONE_REMOTE_NAME}:${ARK_TEMP_MIGRATION_BUCKET}

# 6. Delete your Ark bucket's contents (this command does not 
#    delete the bucket itself, only the contents):
rclone delete ${RCLONE_REMOTE_NAME}:${ARK_BUCKET}

# 7. Copy the contents of the temporary bucket into your Ark bucket, 
#    under the 'backups/' directory/prefix:
rclone copy ${RCLONE_REMOTE_NAME}:${ARK_TEMP_MIGRATION_BUCKET} ${RCLONE_REMOTE_NAME}:${ARK_BUCKET}/backups

# 8. Verify that the 'backups/' directory in your Ark bucket now
#    contains an exact copy of the temporary bucket's contents:
rclone check ${RCLONE_REMOTE_NAME}:${ARK_BUCKET}/backups ${RCLONE_REMOTE_NAME}:${ARK_TEMP_MIGRATION_BUCKET}

# 9. OPTIONAL: If you have restic data to migrate:

#    a. Copy the contents of your Ark restic location into your 
#       Ark bucket, under the 'restic/' directory/prefix:
     ARK_RESTIC_LOCATION=<your-ark-restic-bucket[/optional-prefix]>
     rclone copy ${RCLONE_REMOTE_NAME}:${ARK_RESTIC_LOCATION} ${RCLONE_REMOTE_NAME}:${ARK_BUCKET}/restic

#    b. Check that the 'restic/' directory in your Ark bucket now
#       contains an exact copy of your restic location: 
    rclone check ${RCLONE_REMOTE_NAME}:${ARK_BUCKET}/restic ${RCLONE_REMOTE_NAME}:${ARK_RESTIC_LOCATION}
    
#    c. Delete your ResticRepository custom resources to allow Ark
#       to find them in the new location:
    kubectl -n heptio-ark delete resticrepositories --all

# 10. Once you've confirmed that Ark v0.10 works with your revised Ark
#    bucket, you can delete the temporary migration bucket.
```

[1]: https://rclone.org/
[2]: https://rclone.org/downloads/
