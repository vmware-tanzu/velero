# Container Storage Interface Snapshot Support in Velero

_This feature is under development. Documentation may not be up-to-date and features may not work as expected._

# Prerequisites

The following are the prerequisites for using Velero to take Container Storage Interface (CSI) snapshots:

 1. The cluster is Kubernetes version 1.17 or greater.
 1. The cluster is running a CSI driver capable of support volume snapshots at the [v1beta1 API level](https://kubernetes.io/blog/2019/12/09/kubernetes-1-17-feature-cis-volume-snapshot-beta/).
 1. The Velero server is running with the `--features EnableCSI` feature flag to enable CSI logic in Velero's core. See [Enabling Features][1] for more information.
 1. The Velero [CSI plugin](https://github.com/vmware-tanzu/velero-plugin-for-csi/) is installed to integrate with the CSI volume snapshot APIs.
 1. When restoring CSI volumesnapshots across clusters, the name of the CSI driver in the destination cluster is the same as that on the source cluster to ensure cross cluster portability of CSI volumesnapshots

# Implementation Choices

This section documents some of the choices made during implementation of the Velero [CSI plugin](https://github.com/vmware-tanzu/velero-plugin-for-csi/):

1. Volumesnapshots created by the plugin will be retained only for the lifetime of the backup even if the `DeletionPolicy` on the volumesnapshotclass is set to `Retain`. To accomplish this, during deletion of the backup the prior to deleting the volumesnapshot, volumesnapshotcontent object will be patched to set its `DeletionPolicy` to `Delete`. Thus deleting volumesnapshot object will result in cascade delete of the volumesnapshotcontent and the snapshot in the storage provider.
1. Volumesnapshotcontent objects created during a velero backup that are dangling, unbound to a volumesnapshot object, will also be discovered, through labels, and deleted on backup deletion.

# Known Limitations

1. The Velero [CSI plugin](https://github.com/vmware-tanzu/velero-plugin-for-csi/), to backup CSI backed PVCs, will choose the first VolumeSnapshotClass in the cluster that has the same driver name. _[Issue #17](https://github.com/vmware-tanzu/velero-plugin-for-csi/issues/17)_

# Roadmap

Velero's support level for CSI volume snapshotting will follow upstream Kubernetes support for the feature, and will reach general availability sometime
after volume snapshotting is GA in upstream Kubernetes. Beta support is expected to launch in Velero v1.4.

# Enabling CSI Support

Pass the `--features=EnableCSI` flag to `velero install` - that's it!

To include the status of CSI objects associated with a Velero backup or restore, run `velero client config set features=EnableCSI`.

# How it Works - Overview

Velero's CSI support does not rely on the Velero VolumeSnapshotter plugin interface.

Instead, Velero uses a collection of BackupItemAction plugins that act first against PersistentVolumeClaims.

When this BackupItemAction sees PersistentVolumeClaims pointing to a PersistentVolume backed by a CSI driver, it will create a CSI VolumeSnapshot object with the PersistentVolumeClaim as a source.
This VolumeSnapshot object resides in the same namespace as the PersistentVolumeClaim that was used as a source.

From there, the CSI external-snapshotter controller will see the VolumeSnapshot and create a VolumeSnapshotContent object, a cluster-scoped resource that will point to the actual, disk-based snapshot in the storage system.
The external-snapshotter plugin will call the CSI driver's snapshot method, and the driver will call the storage system's APIs to generate the snapshot.
Once an ID is generated and the storage system marks the snapshot as usable for restore, the VolumeSnapshotContent object will be updated with a `status.snapshotHandle` and the `status.readyToUse` field will be set.

Velero will include the generated VolumeSnapshot and VolumeSnapshotContent objects in the backup tarball, as well as upload all VolumeSnapshots and VolumeSnapshotContents objects in a JSON file to the object storage system.
When Velero synchronizes backups into a new cluster, VolumeSnapshotContent objects will be synced into the cluster as well, so that Velero can manage backup expiration appropriately.

[1]: customize-installation.md#enable-server-side-features
