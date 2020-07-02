---
title: "About Heptio Ark"
layout: docs
---

Heptio Ark provides customizable degrees of recovery for all Kubernetes objects (Pods, Deployments, Jobs, Custom Resource Definitions, etc.), as well as for persistent volumes. This recovery can be cluster-wide, or fine-tuned according to object type, namespace, or labels.

Ark is ideal for the disaster recovery use case, as well as for snapshotting your application state, prior to performing system operations on your cluster (e.g. upgrades).

## Features

Ark provides the following operations:

* On-demand backups
* Scheduled backups
* Restores

Each operation is a custom resource, defined with a Kubernetes [Custom Resource Definition (CRD)][20] and stored in [etcd][22]. An additional custom resource, Config, specifies required information and customized options, such as cloud provider settings. These resources are handled by [custom controllers][21] when their corresponding requests are submitted to the Kubernetes API server.

Each controller watches its custom resource for API requests (Ark operations), performs validations, and handles the logic for interacting with the cloud provider API -- for example, managing object storage and persistent volumes.

### On-demand backups

The **backup** operation:

1. Uploads a tarball of copied Kubernetes objects into cloud object storage.

1. Calls the cloud provider API to make disk snapshots of persistent volumes, if specified.

You can optionally specify hooks to be executed during the backup. For example, you might
need to tell a database to flush its in-memory buffers to disk before taking a snapshot. [More about hooks][10].

Note that cluster backups are not strictly atomic. If Kubernetes objects are being created or edited at the time of backup, they might not be included in the backup. The odds of capturing inconsistent information are low, but it is possible.

### Scheduled backups

The **schedule** operation allows you to back up your data at recurring intervals. The first backup is performed when the schedule is first created, and subsequent backups happen at the schedule's specified interval. These intervals are specified by a Cron expression.

A Schedule acts as a wrapper for Backups; when triggered, it creates them behind the scenes.

Scheduled backups are saved with the name `<SCHEDULE NAME>-<TIMESTAMP>`, where `<TIMESTAMP>` is formatted as *YYYYMMDDhhmmss*.

### Restores

The **restore** operation allows you to restore all of the objects and persistent volumes from a previously created Backup. Heptio Ark supports multiple namespace remapping--for example, in a single restore, objects in namespace "abc" can be recreated under namespace "def", and the ones in "123" under "456".

Kubernetes objects that have been restored can be identified with a label that looks like `ark-restore=<BACKUP NAME>-<TIMESTAMP>`, where `<TIMESTAMP>` is formatted as *YYYYMMDDhhmmss*.

You can also run the Ark server in restore-only mode, which disables backup, schedule, and garbage collection functionality during disaster recovery.

## Backup workflow

Here's what happens when you run `ark backup create test-backup`:

1. The Ark client makes a call to the Kubernetes API server to create a `Backup` object.

1. The `BackupController` notices the new `Backup` object and performs validation.

1. The `BackupController` begins the backup process. It collects the data to back up by querying the API server for resources.

1. The `BackupController` makes a call to the object storage service -- for example, AWS S3 -- to upload the backup file.

By default `ark backup create` makes disk snapshots of any persistent volumes. You can adjust the snapshots by specifying additional flags. See [the CLI help][30] for more information. Snapshots can be disabled with the option `--snapshot-volumes=false`.

![19]

## Set a backup to expire

When you create a backup, you can specify a TTL by adding the flag `--ttl <DURATION>`. If Ark sees that an existing Backup resource is expired, it removes:

* The Backup resource
* The backup file from cloud object storage
* All PersistentVolume snapshots
* All associated Restores

## Object storage sync

Heptio Ark treats object storage as the source of truth. It continuously checks to see that the correct Backup resources are always present. If there is a properly formatted backup file in the storage bucket, but no corresponding Backup resources in the Kubernetes API, Ark synchronizes the information from object storage to Kubernetes.

This allows restore functionality to work in a cluster migration scenario, where the original Backup objects do not exist in the new cluster. See the tutorials for details.

[19]: /img/backup-process.png
[30]: https://github.com/heptio/ark/blob/master/docs/cli-reference/ark_create_backup.md