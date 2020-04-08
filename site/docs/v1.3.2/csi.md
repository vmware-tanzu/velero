# Container Storage Interface Snapshot Support in Velero

_This feature is under development. Documentation may not be up-to-date and features may not work as expected._

Velero supports taking Container Storage Interface (CSI) snapshots as a beta feature on clusters that meet the following prerequisites.

 1. The cluster is Kubernetes version 1.17 or greater.
 1. The cluster is running a CSI driver capable of support volume snapshots at the [v1beta1 API level](https://kubernetes.io/blog/2019/12/09/kubernetes-1-17-feature-cis-volume-snapshot-beta/).
 1. The Velero server is running with the `--features EnableCSI` feature flag to enable CSI logic in Velero's core.
 1. The Velero [CSI plugin](https://github.com/vmware-tanzu/velero-plugin-for-csi/) is installed to integrate with the CSI volume snapshot APIs.
 1. When restoring CSI volumesnapshots across clusters, the name of the CSI driver in the destination cluster should be the same as that on the source cluster to ensure cross cluster portability of CSI volumesnapshots

# Roadmap

Velero's support level for CSI volume snapshotting will follow upstream Kubernetes support for it, and will reach general availability sometime
after volume snapshotting is GA in upstream Kubernetes. Beta support is expected to launch in Velero v1.4.
