# Container Storage Interface Snapshot Support in Velero

_This feature is under development. Documentation may not be up-to-date and features may not work as expected._

# Prerequisites

The following are the prerequisites for using Velero to take Container Storage Interface (CSI) snapshots:

 1. The cluster is Kubernetes version 1.17 or greater.
 1. The cluster is running a CSI driver capable of support volume snapshots at the [v1beta1 API level](https://kubernetes.io/blog/2019/12/09/kubernetes-1-17-feature-cis-volume-snapshot-beta/).
 1. The Velero server is running with the `--features EnableCSI` feature flag to enable CSI logic in Velero's core.
 1. The Velero [CSI plugin](https://github.com/vmware-tanzu/velero-plugin-for-csi/) is installed to integrate with the CSI volume snapshot APIs.
 1. When restoring CSI volumesnapshots across clusters, the name of the CSI driver in the destination cluster is the same as that on the source cluster to ensure cross cluster portability of CSI volumesnapshots

# Implementation Choices

This section documents some of the choices made during implementation of the Velero [CSI plugin](https://github.com/vmware-tanzu/velero-plugin-for-csi/):

1. Volumesnapshots created by the plugin will be retained only for the lifetime of the backup even if the `DeletionPolicy` on the volumesnapshotclass is set to `Retain`. To accomplish this, during deletion of the backup the prior to deleting the volumesnapshot, volumesnapshotcontent object will be patched to set its `DeletionPolicy` to `Delete`. Thus deleting volumesnapshot object will result in cascade delete of the volumesnapshotcontent and the snapshot in the storage provider.
1. Volumesnapshotcontent objects created during a velero backup that are dangling, unbound to a volumesnapshot object, will also be discovered, through labels, and deleted on backup deletion.

# Known Limitations

1. The Velero [CSI plugin](https://github.com/vmware-tanzu/velero-plugin-for-csi/), to backup CSI backed PVCs, will choose the first VolumeSnapshotClass in the cluster that has the same driver name. _[Issue #17](https://github.com/vmware-tanzu/velero-plugin-for-csi/issues/17)_

# Roadmap

Velero's support level for CSI volume snapshotting will follow upstream Kubernetes support for it, and will reach general availability sometime
after volume snapshotting is GA in upstream Kubernetes. Beta support is expected to launch in Velero v1.4.
