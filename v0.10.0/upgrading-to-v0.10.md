# Upgrading to Ark v0.10

## Overview

Ark v0.10 includes a number of breaking changes. Below, we outline what those changes are, and what steps you should take to ensure
a successful upgrade from prior versions of Ark.

## Breaking Changes

### Switch from Config to BackupStorageLocation and VolumeSnapshotLocation CRDs, and new server flags

Prior to v0.10, Ark used a `Config` CRD to capture information about your backup storage and persistent volume providers, as well
some miscellaneous Ark settings. In v0.10, we've eliminated this CRD and replaced it with:

- A [BackupStorageLocation][1] CRD to capture information about where to store your backups
- A [VolumeSnapshotLocation][2] CRD to capture information about where to store your persistent volume snapshots
- Command-line flags for the `ark server` command (run by your Ark deployment) to capture miscellaneous Ark settings

When upgrading to v0.10, you'll need to transfer the configuration information that you currently have in the `Config` CRD
into the above. We'll cover exactly how to do this below.

For a general overview of this change, see the [Locations documentation][4].

### Reorganization of data in object storage

We've made [changes to the layout of data stored in object storage][3] for simplicity and extensibility. You'll need to
rearrange any pre-v0.10 data as part of the upgrade. We've provided a script to help with this.

## Step-by-Step Upgrade Instructions

1. Ensure you've [downloaded & extracted the latest release][5].

1. Scale down your existing Ark deployment:
    ```bash
    kubectl scale -n heptio-ark deploy/ark --replicas 0
    ```

1. In the Ark directory (i.e. where you extracted the release tarball), re-apply the `00-prereqs.yaml` file to create new CRDs:
    ```bash
    kubectl apply -f config/common/00-prereqs.yaml
    ```

1. Create one or more [BackupStorageLocation][1] resources based on the examples provided in the `config/` directory for your platform, using information from the existing `Config` resource as necessary.

1. If you're using Ark to take PV snapshots, create one or more [VolumeSnapshotLocation][2] resources based on the examples provided in the `config/` directory for your platform, using information from the existing `Config` resource as necessary.

1. Perform the one-time object storage migration detailed [here][3].

1. In your Ark deployment YAML (see the `config/` directory for samples), specify flags to the `ark server` command under the container's `args`:

    a. The names of the `BackupStorageLocation` and `VolumeSnapshotLocation(s)` that should be used by default for backups. If defaults are set here, 
    users won't need to explicitly specify location names when creating backups (though they still can, if they want to store backups/snapshots in
    alternate locations). If no value is specified for `--default-backup-storage-location`, the Ark server looks for a `BackupStorageLocation` 
    named `default` to use.

    Flag | Default Value | Description | Example
    ---- | ------------- | ----------- | -------
    `--default-backup-storage-location` | "default" | name of the backup storage location that should be used by default for backups | aws-us-east-1-bucket
    `--default-volume-snapshot-locations` | [none] | name of the volume snapshot location(s) that should be used by default for PV snapshots, for each PV provider | aws:us-east-1,portworx:local

    **NOTE:** the values of these flags should correspond to the names of a `BackupStorageLocation` and `VolumeSnapshotLocation(s)` custom resources
    in the cluster.

    b. Any non-default Ark server settings:

    Flag | Default Value | Description
    ---- | ------------- | -----------
    `--backup-sync-period` | 1m | how often to ensure all Ark backups in object storage exist as Backup API objects in the cluster
    `--restic-timeout` | 1h | how long backups/restores of pod volumes should be allowed to run before timing out (previously `podVolumeOperationTimeout` in the `Config` resource in pre-v0.10 versions)
    `--restore-only` | false | run in a mode where only restores are allowed; backups, schedules, and garbage-collection are all disabled

1. If you are using any plugins, update the Ark deployment YAML to reference the latest image tag for your plugins. This can be found under the `initContainers` section of your deployment YAML.

1. Apply your updated Ark deployment YAML to your cluster and ensure the pod(s) starts up successfully.

1. If you're using Ark's restic integration, ensure the daemon set pods have been re-created with the latest Ark image (if your daemon set YAML is using the `:latest` tag, you can delete the pods so they're recreated with an updated image).

1. Once you've confirmed all of your settings have been migrated over correctly, delete the Config CRD:
    ```bash
    kubectl delete -n heptio-ark config --all
    kubectl delete crd configs.ark.heptio.com
    ```


[1]: /api-types/backupstoragelocation.md
[2]: /api-types/volumesnapshotlocation.md
[3]: storage-layout-reorg-v0.10.md
[4]: locations.md
[5]: get-started.md#download
