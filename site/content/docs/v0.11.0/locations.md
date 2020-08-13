---
title: "Backup Storage Locations and Volume Snapshot Locations"
layout: docs
---

Velero v0.10 introduces a new way of configuring where Velero backups and their associated persistent volume snapshots are stored.

## Motivations

In Velero versions prior to v0.10, the configuration for where to store backups & volume snapshots is specified in a `Config` custom resource. The `backupStorageProvider` section captures the place where all Velero backups should be stored. This is defined by a **provider** (e.g. `aws`, `azure`, `gcp`, `minio`, etc.), a **bucket**, and possibly some additional provider-specific settings (e.g. `region`). Similarly, the `persistentVolumeProvider` section captures the place where all persistent volume snapshots taken as part of Velero backups should be stored, and is defined by a **provider** and additional provider-specific settings (e.g. `region`).

There are a number of use cases that this basic design does not support, such as:

- Take snapshots of more than one kind of persistent volume in a single Velero backup (e.g. in a cluster with both EBS volumes and Portworx volumes)
- Have some Velero backups go to a bucket in an eastern USA region, and others go to a bucket in a western USA region
- For volume providers that support it (e.g. Portworx), have some snapshots be stored locally on the cluster and have others be stored in the cloud

Additionally, as we look ahead to backup replication, a major feature on our roadmap, we know that we'll need Velero to be able to support multiple possible storage locations.

## Overview

In Velero v0.10 we got rid of the `Config` custom resource, and replaced it with two new custom resources, `BackupStorageLocation` and `VolumeSnapshotLocation`. The new resources directly replace the legacy `backupStorageProvider` and `persistentVolumeProvider` sections of the `Config` resource, respectively. 

Now, the user can pre-define more than one possible `BackupStorageLocation` and more than one `VolumeSnapshotLocation`, and can select *at backup creation time* the location in which the backup and associated snapshots should be stored. 

A `BackupStorageLocation` is defined as a bucket, a prefix within that bucket under which all Velero data should be stored, and a set of additional provider-specific fields (e.g. AWS region, Azure storage account, etc.) The [API documentation][1] captures the configurable parameters for each in-tree provider.

A `VolumeSnapshotLocation` is defined entirely by provider-specific fields (e.g. AWS region, Azure resource group, Portworx snapshot type, etc.) The [API documentation][2] captures the configurable parameters for each in-tree provider.

Additionally, since multiple `VolumeSnapshotLocations` can be created, the user can now configure locations for more than one volume provider, and if the cluster has volumes from multiple providers (e.g. AWS EBS and Portworx), all of them can be snapshotted in a single Velero backup.

## Limitations / Caveats

- Volume snapshots are still limited by where your provider allows you to create snapshots. For example, AWS and Azure do not allow you to create a volume snapshot in a different region than where the volume is. If you try to take an Velero backup using a volume snapshot location with a different region than where your cluster's volumes are, the backup will fail.

- Each Velero backup has one `BackupStorageLocation`, and one `VolumeSnapshotLocation` per volume provider. It is not possible (yet) to send a single Velero backup to multiple backup storage locations simultaneously, or a single volume snapshot to multiple locations simultaneously. However, you can always set up multiple scheduled backups that differ only in the storage locations used if redundancy of backups across locations is important.

- Cross-provider snapshots are not supported. If you have a cluster with more than one type of volume (e.g. EBS and Portworx), but you only have a `VolumeSnapshotLocation` configured for EBS, then Velero will **only** snapshot the EBS volumes.

- Restic data is now stored under a prefix/subdirectory of the main Velero bucket, and will go into the bucket corresponding to the `BackupStorageLocation` selected by the user at backup creation time.

## Examples

Let's look at some examples of how we can use this new mechanism to address each of our previously unsupported use cases:

#### Take snapshots of more than one kind of persistent volume in a single Velero backup (e.g. in a cluster with both EBS volumes and Portworx volumes)

During server configuration:

```shell
velero snapshot-location create ebs-us-east-1 \
    --provider aws \
    --config region=us-east-1

velero snapshot-location create portworx-cloud \
    --provider portworx \
    --config type=cloud
```

During backup creation:

```shell
velero backup create full-cluster-backup \
    --volume-snapshot-locations ebs-us-east-1,portworx-cloud
```

Alternately, since in this example there's only one possible volume snapshot location configured for each of our two providers (`ebs-us-east-1` for `aws`, and `portworx-cloud` for `portworx`), Velero doesn't require them to be explicitly specified when creating the backup:

```shell
velero backup create full-cluster-backup
```

#### Have some Velero backups go to a bucket in an eastern USA region, and others go to a bucket in a western USA region

During server configuration:

```shell
velero backup-location create default \
    --provider aws \
    --bucket velero-backups \
    --config region=us-east-1

velero backup-location create s3-alt-region \
    --provider aws \
    --bucket velero-backups-alt \
    --config region=us-west-1
```

During backup creation:
```shell
# The Velero server will automatically store backups in the backup storage location named "default" if
# one is not specified when creating the backup. You can alter which backup storage location is used
# by default by setting the --default-backup-storage-location flag on the `velero server` command (run
# by the Velero deployment) to the name of a different backup storage location.
velero backup create full-cluster-backup
```
Or:
```shell
velero backup create full-cluster-alternate-location-backup \
    --storage-location s3-alt-region
```

#### For volume providers that support it (e.g. Portworx), have some snapshots be stored locally on the cluster and have others be stored in the cloud

During server configuration:

```shell
velero snapshot-location create portworx-local \
    --provider portworx \
    --config type=local

velero snapshot-location create portworx-cloud \
    --provider portworx \
    --config type=cloud
```

During backup creation:

```shell
# Note that since in this example we have two possible volume snapshot locations for the Portworx 
# provider, we need to explicitly specify which one to use when creating a backup. Alternately,
# you can set the --default-volume-snapshot-locations flag on the `velero server` command (run by
# the Velero deployment) to specify which location should be used for each provider by default, in 
# which case you don't need to specify it when creating a backup.
velero backup create local-snapshot-backup \
    --volume-snapshot-locations portworx-local
```

Or:

```shell
velero backup create cloud-snapshot-backup \
    --volume-snapshot-locations portworx-cloud
```

#### One location is still easy

If you don't have a use case for more than one location, it's still just as easy to use Velero. Let's assume you're running on AWS, in the `us-west-1` region:

During server configuration:

```shell
velero backup-location create default \
    --provider aws \
    --bucket velero-backups \
    --config region=us-west-1

velero snapshot-location create ebs-us-west-1 \
    --provider aws \
    --config region=us-west-1
```

During backup creation:
```shell
# Velero's will automatically use your configured backup storage location and volume snapshot location. 
# Nothing new needs to be specified when creating a backup.
velero backup create full-cluster-backup
```

## Additional Use Cases

1. If you're using Azure's AKS, you may want to store your volume snapshots outside of the "infrastructure" resource group that is automatically created when you create your AKS cluster. This is now possible using a `VolumeSnapshotLocation`, by specifying a `resourceGroup` under the `config` section of the snapshot location. See the [Azure volume snapshot location documentation][3] for details.

1. If you're using Azure, you may want to store your Velero backups across multiple storage accounts and/or resource groups. This is now possible using a `BackupStorageLocation`, by specifying a `storageAccount` and/or `resourceGroup`, respectively, under the `config` section of the backup location. See the [Azure backup storage location documentation][4] for details.



[1]: api-types/backupstoragelocation.md
[2]: api-types/volumesnapshotlocation.md
[3]: api-types/volumesnapshotlocation.md#azure
[4]: api-types/backupstoragelocation.md#azure
