# Expose list of backed up resources in backup details

Status: Draft

To increase the visibility of what a backup might contain, this document proposes storing metadata about backed up resources in object storage and adding a new section to the detailed backup description output to list them.

## Goals

- Include a list of backed up resources as metadata in the bucket
- Enable users to get a view of what resources are included in a backup using the Velero CLI

## Non Goals

- Expose the full manifests of the backed up resources

## Background

As reported in #396, the information reported in a `velero backup describe <name> --details` command is fairly limited, and does not easily describe what resources a backup contains.
In order to see what a backup might contain, a user would have to download the backup tarball and extract it.
This makes it difficult to keep track of different backups in a cluster.

## High-Level Design

After performing a backup, a new file will be created that contains the list of the resources that have been included in the backup.
This file will be persisted in object storage alongside the backup contents and existing metadata.

A section will be added to the output of `velero backup describe <name> --details` command to view this metadata.

## Detailed Design

### Metadata file

This metadata will be in JSON (or YAML) format so that it can be easily inspected from the bucket outside of Velero tooling, and will contain the API resource and group, namespaces and names of the resources:

```
deployments.apps:
- default/database
- default/wordpress
services:
- default/database
- default/wordpress
secrets:
- default/database-root-password
- default/database-user-password
configmaps:
- default/database
```

The filename for this metadata will be `<backup name>-resource-list.json`.
The top-level key is the string form of the `schema.GroupResource` type that we currently keep track of in the backup controller code path.

### Changes in Backup controller

The Backupper currently initialises a map to track the `backedUpItems` (https://github.com/heptio/velero/blob/1594bdc8d0132f548e18ffcc1db8c4cd2b042726/pkg/backup/backup.go#L269), this is passed down through GroupBackupper, ResourceBackupper and ItemBackupper. Moving the map initialisation to the BackupController will provide access to the resulting list there after a successful backup.

The `backedUpItems` map is currently a flat structure and will need to be converted to the nested structure above, grouped by `schema.GroupResource`.

After converting to the right format, it can be passed to the `persistBackup` function to persist the file in object storage.

### Changes to DownloadRequest CRD and processing

A new `DownloadTargetKind` "BackupResourceList" will be added to the DownloadRequest CR.

The `GetDownloadURL` function in the `persistence` package will be updated to handle this new DownloadTargetKind to enable the Velero client to fetch the metadata from the bucket.

### Changes to `velero backup describe <name> --details`

This command will need to be updated to fetch the metadata from the bucket using the `Stream` method used in other commands.
The file will be read in memory and displayed in the output of the command.
Depending on the format the metadata is stored in, it may need processing to print in a more human-readable format.
If we choose to store the metadata in YAML, it can likely be directly printed out.

If the metadata file does not exist, this is an older backup and we cannot display the list of resources that were backed up.

## Open Questions

- Do we want to show the group version in the metadata (i.e. do we want users to see `apps/v1/Deployment` instead of `deployments.apps`)? If so, we wouldn't be able to use the existing list of backedUpItems list, or we'd need to change it to track schema.GroupVersionKinds instead.

## Alternatives Considered

### Fetch backup contents archive and walkthrough to list contents

Instead of recording new metadata about what resources have been backed up, we could simply download the backup contents archive and walkthrough it to list the contents everytime `velero backup describe <name> --details` is run.

The advantage of this approach is that we don't need to change any backup procedures as we already have this content, and we will also be able to list resources for older backups.
Additionally, if we wanted to expose more information about the backed up resources, we can do so without having to update what we store in the metadata.

The disadvantages are:
- downloading the whole backup archive will be larger than just downloading a smaller file with metadata
- doesn't solve the usecase of wanting to see the list of backed up resources in the bucket outside of Velero tooling

## Security Considerations
