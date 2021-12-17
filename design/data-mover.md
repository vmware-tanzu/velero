# Velero Data Mover

## Abstract
Velero currently does not handle data movement of snapshotted data.  We would like to have the ability to move data that has been snapshotted in a storage system or other [Astrolabe Protected Entity](https://github.com/vmware-tanzu/astrolabe) provider into the backup storage location specified.

## Background
Velero has two options for backing up persistent volumes:
Using snapshotting APIs provided by the storage provider (for example, using the VolumeSnapshotter)
Filesystem level backup using restic

Snapshotting capabilities provided by storage providers generally provide crash consistency of volumes, however they are not portable and are typically locked to the particular cloud provider and region in which the snapshot was taken.
Although restic provides portability of data, it is a filesystem level backup and is not crash-consistent by default.
It requires the quiescing of data prior to backup which impacts the availability of an application.

Velero also has beta support for taking snapshots using CSI.
Without data movement though, there is no guarantee that the snapshots are durable as it is dependent on the storage provider.

Extraction of data from snapshots is something that is already available in the [Velero Plugin for vSphere](https://github.com/vmware-tanzu/velero-plugin-for-vsphere).
This plugin makes use of the Astrolabe APIs to implement a vSphere FCD Protected Entity.
This Protected Entity interface provides data readers and writers for the disks.
Once a snapshot is created, the data reader is used to read the data from the disk and it is written to an S3 compatible object store.

The Astrolabe API can be implemented for resources other than volumes, for example, databases.
This enables each resource to define the best way to back up that entity and any subresources that it depends on.
For example, when backing up a Postgres database, it may be preferable to use pg_dump rather than quiescing and snapshotting the volumes.
This can be achieved using Astrolabe and as a result will provide data readers for the dump file.
This data reader can be used to write the dump contents to an arbitrary backup location, rather than being confined to the underlying storage system.

By allowing Velero to interact with Astrolabe Protected Entities, we can utilize the data readers for these entities to read the data and write to other locations.

## Goals
- Enable the data from snapshots of resources to be moved to multiple storage locations including object stores.
- Allow the location for the data store to be specified at backup time and allow selection of backup location at restore time.
- Allow all or a subset of Protected Entities to be copied into the backup location.
- Allow the data mover functionality to be replaced by third parties.
- Include performance considerations:
  - Move data in a scale-out fashion
  - Allow data movement to be throttled
  - Support incremental backups

## Non-goals
This design does not consider removing existing VolumeSnapshot or restic capabilities. The VolumeSnapshot plugin is deprecated in favour of the ItemSnapshot plugin and restic will likely be replaced with Kopia, however support for these will not be removed as part of this work.

## High-level Design
Adding data movement to Velero is a significant feature.
Due to this, the design is being broken into a number of phases where each phase provides benefit and functionality. 
The data movement will be part of the Velero codebase, but later it may be moved to Astrolabe to allow Protected Entities to call it or use it as a destination.

### Phase 1
The movement of data will be triggered and controlled by Velero as part of the backup and restore process.
Once the snapshotting phase of the backup is complete, Velero will iterate over ItemSnapshots (resource snapshotted using an ItemSnapshotter plugin), and retrieve the Astrolabe Protected Entities for these resources.
As a Protected Entity may have child snapshots, the entire tree should be traversed and flattened into a list.
This list can be iterated over (in any order as the snapshot is complete), and the Protect Entities can be passed to the Data Mover.
The Data Mover will use the data streams provided by the Protected Entity to read the data and write it to the backup repository.

### Phase 2
File system level backup will be integrated into the Data Mover, with or without snapshots. Restic will still be supported but Kopia will become the preferred backup repository provider.

### Phase 3
Investigate and consider making the Data Mover a concept in Astrolabe rather than part of Velero so that Protected Entities can call it or use it as a destination.

## Detailed Design


## Alternative Considered

## Security considerations

## Compatibility

## Implementation

## Open Issues
