---
title: "How Ark Works"
layout: docs
---

Each Ark operation -- on-demand backup, scheduled backup, restore -- is a custom resource, defined with a Kubernetes [Custom Resource Definition (CRD)][20] and stored in [etcd][22]. Ark also includes controllers that process the custom resources to perform backups, restores, and all related operations.

You can back up or restore all objects in your cluster, or you can filter objects by type, namespace, and/or label.

Ark is ideal for the disaster recovery use case, as well as for snapshotting your application state, prior to performing system operations on your cluster (e.g. upgrades).

## On-demand backups

The **backup** operation:

1. Uploads a tarball of copied Kubernetes objects into cloud object storage.

1. Calls the cloud provider API to make disk snapshots of persistent volumes, if specified.

You can optionally specify hooks to be executed during the backup. For example, you might
need to tell a database to flush its in-memory buffers to disk before taking a snapshot. [More about hooks][10].

Note that cluster backups are not strictly atomic. If Kubernetes objects are being created or edited at the time of backup, they might not be included in the backup. The odds of capturing inconsistent information are low, but it is possible.

## Scheduled backups

The **schedule** operation allows you to back up your data at recurring intervals. The first backup is performed when the schedule is first created, and subsequent backups happen at the schedule's specified interval. These intervals are specified by a Cron expression.

Scheduled backups are saved with the name `<SCHEDULE NAME>-<TIMESTAMP>`, where `<TIMESTAMP>` is formatted as *YYYYMMDDhhmmss*.

## Restores

The **restore** operation allows you to restore all of the objects and persistent volumes from a previously created backup. You can also restore only a filtered subset of objects and persistent volumes. Ark supports multiple namespace remapping--for example, in a single restore, objects in namespace "abc" can be recreated under namespace "def", and the objects in namespace "123" under "456".

The default name of a restore is `<BACKUP NAME>-<TIMESTAMP>`, where `<TIMESTAMP>` is formatted as *YYYYMMDDhhmmss*. You can also specify a custom name. A restored object also includes a label with key `ark.heptio.com/restore-name` and value `<RESTORE NAME>`.

You can also run the Ark server in restore-only mode, which disables backup, schedule, and garbage collection functionality during disaster recovery.

## Backup workflow

When you run `ark backup create test-backup`:

1. The Ark client makes a call to the Kubernetes API server to create a `Backup` object.

1. The `BackupController` notices the new `Backup` object and performs validation.

1. The `BackupController` begins the backup process. It collects the data to back up by querying the API server for resources.

1. The `BackupController` makes a call to the object storage service -- for example, AWS S3 -- to upload the backup file.

By default, `ark backup create` makes disk snapshots of any persistent volumes. You can adjust the snapshots by specifying additional flags. Run `ark backup create --help` to see available flags. Snapshots can be disabled with the option `--snapshot-volumes=false`.

![19]

## Set a backup to expire

When you create a backup, you can specify a TTL by adding the flag `--ttl <DURATION>`. If Ark sees that an existing backup resource is expired, it removes:

* The backup resource
* The backup file from cloud object storage
* All PersistentVolume snapshots
* All associated Restores

## Object storage sync

Heptio Ark treats object storage as the source of truth. It continuously checks to see that the correct backup resources are always present. If there is a properly formatted backup file in the storage bucket, but no corresponding backup resource in the Kubernetes API, Ark synchronizes the information from object storage to Kubernetes.

This allows restore functionality to work in a cluster migration scenario, where the original backup objects do not exist in the new cluster.

Likewise, if a backup object exists in Kubernetes but not in object storage, it will be deleted from Kubernetes since the backup tarball no longer exists.

[10]: hooks.md
[19]: /img/backup-process.png
[20]: https://kubernetes.io/docs/concepts/api-extension/custom-resources/#customresourcedefinitions
[21]: https://kubernetes.io/docs/concepts/api-extension/custom-resources/#custom-controllers
[22]: https://github.com/coreos/etcd
