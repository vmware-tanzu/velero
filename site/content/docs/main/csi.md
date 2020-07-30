# Container Storage Interface Snapshot Support in Velero

_This feature is under development. Documentation may not be up-to-date and features may not work as expected._

Integrating Container Storage Interface (CSI) snapshot support into Velero enables Velero to backup and restore CSI-backed volumes using the [Kubernetes CSI Snapshot Beta APIs](https://kubernetes.io/docs/concepts/storage/volume-snapshots/).

By supporting CSI snapshot APIs, Velero can support any volume provider that has a CSI driver, without requiring a Velero-specific plugin to be available.

# Prerequisites

The following are the prerequisites for using Velero to take Container Storage Interface (CSI) snapshots:

 1. The cluster is Kubernetes version 1.17 or greater.
 1. The cluster is running a CSI driver capable of support volume snapshots at the [v1beta1 API level](https://kubernetes.io/blog/2019/12/09/kubernetes-1-17-feature-cis-volume-snapshot-beta/).
 1. When restoring CSI volumesnapshots across clusters, the name of the CSI driver in the destination cluster is the same as that on the source cluster to ensure cross cluster portability of CSI volumesnapshots

# Installing Velero with CSI support

Ensure that the Velero server is running with the `EnableCSI` feature flag. See [Enabling Features][1] for more information.
Also, the Velero [CSI plugin][2] ([Docker Hub][3]) is necessary to integrate with the CSI volume snapshot APIs.

Both of these can be added with the `velero install` command.

```bash
velero install \
--features=EnableCSI \
--plugins=<object storage plugin>,velero/velero-plugin-for-csi:v0.1.0 \
...
```

To include the status of CSI objects associated with a Velero backup or restore in `velero backup describe` or `velero restore describe` output, run `velero client config set features=EnableCSI`.
See [Enabling Features][1] for more information about managing client-side feature flags.

# Implementation Choices

This section documents some of the choices made during implementation of the Velero [CSI plugin][2]:

1. Volumesnapshots created by the plugin will be retained only for the lifetime of the backup even if the `DeletionPolicy` on the volumesnapshotclass is set to `Retain`. To accomplish this, during deletion of the backup the prior to deleting the volumesnapshot, volumesnapshotcontent object will be patched to set its `DeletionPolicy` to `Delete`. Thus deleting volumesnapshot object will result in cascade delete of the volumesnapshotcontent and the snapshot in the storage provider.
1. Volumesnapshotcontent objects created during a velero backup that are dangling, unbound to a volumesnapshot object, will also be discovered, through labels, and deleted on backup deletion.
1. The Velero CSI plugin, to backup CSI backed PVCs, will choose the VolumeSnapshotClass in the cluster that has the same driver name and also has the `velero.io/csi-volumesnapshot-class` label set on it, like
```yaml
velero.io/csi-volumesnapshot-class: "true"
```

# Roadmap

Velero's support level for CSI volume snapshotting will follow upstream Kubernetes support for the feature, and will reach general availability sometime
after volume snapshotting is GA in upstream Kubernetes. Beta support is expected to launch in Velero v1.4.

# How it Works - Overview

Velero's CSI support does not rely on the Velero VolumeSnapshotter plugin interface.

Instead, Velero uses a collection of BackupItemAction plugins that act first against PersistentVolumeClaims.

When this BackupItemAction sees PersistentVolumeClaims pointing to a PersistentVolume backed by a CSI driver, it will choose the VolumeSnapshotClass with the same driver name that has the `velero.io/csi-volumesnapshot-class` label to create a CSI VolumeSnapshot object with the PersistentVolumeClaim as a source.
This VolumeSnapshot object resides in the same namespace as the PersistentVolumeClaim that was used as a source.

From there, the CSI external-snapshotter controller will see the VolumeSnapshot and create a VolumeSnapshotContent object, a cluster-scoped resource that will point to the actual, disk-based snapshot in the storage system.
The external-snapshotter plugin will call the CSI driver's snapshot method, and the driver will call the storage system's APIs to generate the snapshot.
Once an ID is generated and the storage system marks the snapshot as usable for restore, the VolumeSnapshotContent object will be updated with a `status.snapshotHandle` and the `status.readyToUse` field will be set.

Velero will include the generated VolumeSnapshot and VolumeSnapshotContent objects in the backup tarball, as well as upload all VolumeSnapshots and VolumeSnapshotContents objects in a JSON file to the object storage system.
When Velero synchronizes backups into a new cluster, VolumeSnapshotContent objects will be synced into the cluster as well, so that Velero can manage backup expiration appropriately.

The `DeletionPolicy` on the VolumeSnapshotContent will be the same as the `DeletionPolicy` on the VolumeSnapshotClass that was used to create the VolumeSnapshot. Setting a `DeletionPolicy` of `Retain` on the VolumeSnapshotClass will preserve the volume snapshot in the storage system for the lifetime of the Velero backup and will prevent the deletion of the volume snapshot, in the storage system, in the event of a disaster where the namespace with the VolumeSnapshot object may be lost.

When the Velero backup expires, the VolumeSnapshot objects will be deleted and the VolumeSnapshotContent objects will be updated to have a `DeletionPolicy` of `Delete`, in order to free space on the storage system.

For more details on how each plugin works, see the [CSI plugin repo][2]'s documentation.

[1]: customize-installation.md#enable-server-side-features
[2]: https://github.com/vmware-tanzu/velero-plugin-for-csi/
[3]: https://hub.docker.com/repository/docker/velero/velero-plugin-for-csi
