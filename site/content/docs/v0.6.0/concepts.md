---
title: "Concepts"
layout: docs
---

* [Overview][0]
* [Operation types][1]
    * [1. Backups][2]
    * [2. Schedules][3]
    * [3. Restores][4]
* [API types][9]
* [Expired backup deletion][5]
* [Cloud storage sync][6]

## Overview

Heptio Ark provides customizable degrees of recovery for all Kubernetes API objects (Pods, Deployments, Jobs, Custom Resource Definitions, etc.), as well as for persistent volumes. This recovery can be cluster-wide, or fine-tuned according to object type, namespace, or labels.

Ark is ideal for the disaster recovery use case, as well as for snapshotting your application state, prior to performing system operations on your cluster (e.g. upgrades).

## Operation types

This section gives a quick overview of the Ark operation types.

### 1. Backups
The *backup* operation (1) uploads a tarball of copied Kubernetes resources into cloud object storage and (2) uses the cloud provider API to make disk snapshots of persistent volumes, if specified. [Annotations][8] are cleared for PVs but kept for all other object types.

You can optionally specify hooks that should be executed during the backup. For example, you may
need to tell a database to flush its in-memory buffers to disk prior to taking a snapshot. You can
find more information about hooks [here][11].

Some things to be aware of:
* *Cluster backups are not strictly atomic.* If API objects are being created or edited at the time of backup, they may or not be included in the backup. In practice, backups happen very quickly and so the odds of capturing inconsistent information are low, but still possible.

* *A backup usually takes no more than a few seconds.* The snapshotting process for persistent volumes is asynchronous, so the runtime of the `ark backup` command isn't dependent on disk size.

These ad-hoc backups are saved with the `<BACKUP NAME>` specified during creation.


### 2. Schedules
The *schedule* operation allows you to back up your data at recurring intervals. The first backup is performed when the schedule is first created, and subsequent backups happen at the schedule's specified interval. These intervals are specified by a Cron expression.

A Schedule acts as a wrapper for Backups; when triggered, it creates them behind the scenes.

Scheduled backups are saved with the name `<SCHEDULE NAME>-<TIMESTAMP>`, where `<TIMESTAMP>` is formatted as *YYYYMMDDhhmmss*.

### 3. Restores
The *restore* operation allows you to restore all of the objects and persistent volumes from a previously created Backup. Heptio Ark supports multiple namespace remapping--for example, in a single restore, objects in namespace "abc" can be recreated under namespace "def", and the ones in "123" under "456".

Kubernetes API objects that have been restored can be identified with a label that looks like `ark-restore=<BACKUP NAME>-<TIMESTAMP>`, where `<TIMESTAMP>` is formatted as *YYYYMMDDhhmmss*.

You can also run the Ark server in *restore-only* mode, which disables backup, schedule, and garbage collection functionality during disaster recovery.

## API types

For information about the individual API types Ark uses, please see the [API types reference][10].

## Expired backup deletion

When first creating a backup, you can specify a TTL. If Ark sees that an existing Backup resource has expired, it removes both:
* The Backup resource itself
* The actual backup file from cloud object storage

## Cloud storage sync

Heptio Ark treats object storage as the source of truth. It continuously checks to see that the correct Backup resources are always present. If there is a properly formatted backup file in the storage bucket, but no corresponding Backup resources in the Kubernetes API, Ark synchronizes the information from object storage to Kubernetes.

This allows *restore* functionality to work in a cluster migration scenario, where the original Backup objects do not exist in the new cluster. See the [use case guide][7] for details.

[0]: #overview
[1]: #operation-types
[2]: #1-backups
[3]: #2-schedules
[4]: #3-restores
[5]: #expired-backup-deletion
[6]: #cloud-storage-sync
[7]: use-cases.md#cluster-migration
[8]: https://kubernetes.io/docs/concepts/overview/working-with-objects/annotations/
[9]: #api-types
[10]: api-types/
[11]: hooks.md
