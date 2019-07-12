# CSI Snapshot Support

Status: Draft

One to two sentences that describes the goal of this proposal.
The reader should be able to tell by the title, and the opening paragraph, if this document is relevant to them.

The Container Storage Interface (CSI) has [introduced an alpha snapshot API in Kubernetes v1.12][1].
This document suggests an approach for integrating support for this snapshot API within Velero, augmenting its existing capabilities.

## Goals

- Enable Velero to create CSI snapshot objects

## Non Goals

- Replacing Velero's existing [VolumeSnapshotter][7] API
- Replacing Velero's Restic support

## Background

Velero has had support for performing persistent volume snapshots since it's inception.
However, support has been limited to a handful of providers.
The plugin API introduced in Velero v0.7 enabled the community to expand the number of supported providers.
In the meantime, the Kubernetes sig-storage advanced the CSI spec to allow for a generic storage interface, opening up the possibility of moving storage code out of the core Kubernetes code base.
The CSI working group has also developed a generic snapshotting API that any CSI driver developer may implement, giving users the ability to snapshot volumes from a standard interface.

By supporting the CSI snapshot API, Velero can extend it's support to any CSI driver, without requiring a Velero-specific plugin be written, easing the development burden on providers while also reaching more end users.

## High-Level Design

One to two paragraphs that describe the high level changes that will be made to implement this proposal.

In order to support CSI's snapshot API, Velero must interact with the [`VolumeSnapshot`][2] and [`VolumeSnapshotContent`][3] CRDs.
These act as requests to the CSI driver to perform a snapshot on the underlying provider's volume.
This can largely be accomplished with Velero `BackupItemAction` and `RestoreItemAction` plugins that operate on these CRDs.

Additionally, changes to the Velero server and client code are necessary to track `VolumeSnapshots` that are associated with a given backup, similarly to how Velero tracks its own [`volume.Snapshot`][4] type.
Tracking these is important for allowing users to see what is in their backup, and provides parity for the existing `volume.Snapshot` and [`PodVolumeBackup`][5] types.
This is also done to retain the object store as Velero's source of truth, without having to query the Kubernetes API server for associated `VolumeSnapshot`s.

`velero backup describe --details` will use the stored VolumeSnapshots to list CSI snapshots included in the backup to the user.

## Detailed Design

### Resource plugins

A set of [prototype][6] plugins was developed that informed this design.

The plugins will be as follows:

* A `BackupItemAction` for `PersistentVolumeClaim`s, named `velero.io/csi-pvc` that will create a `VolumeSnapshot.snapshot.storage.k8s.io` object from the PVC.
The CSI controllers will create a `VolumeSnapshotContent.snapshot.storage.k8s.io` object associated with the `VolumeSnapshot`.
This plugin should label both the created `VolumeSnapshot` and `VolumeSnapshotContent` objects with [`v1.BackupNameLabel`][10] for ease of lookup later.
`velero.io/volume-snapshot-name` will be applied as a label to the PVC so that the `VolumeSnapshot` can be found easily for restore.
`VolumeSnapshot`, `VolumeSnapshotContent`, and `VolumeSnapshotClass` objects would be returned as additional items to be backed up.
The `VolumeSnapshotContent.Spec.VolumeSnapshotSource.SnapshotHandle` field is the link to the underlying platform's snapshot, and must be preserved for restoration.

* A `RestoreItemAction` for `VolumeSnapshotContent` objects, named `velero.io/csi-vsc`.
The metadata (excluding labels), `PersistentVolumeClaim.UUID`, and `VolumeSnapshotRef.UUID` fields will be cleared.
The reference fields are cleared because the associated objects will get new UUIDs in the cluster.
This also maps to the "import" case of [the snapshot API][1].

* A `RestoreItemAction` for `VolumeSnapshot` objects, named `velero.io/csi-vs`.
Metadata (excluding labels) and `Source` fields on the object will be cleared.
The `VolumeSnapshot.Spec.SnapshotContentName` is the link back to the `VolumeSnapshotContent` object, and thus the actual snapshot.
The `Source` field indicates that a new CSI snapshot operation should be performed, which isn't relevant on restore.
This follows the "import" case of [the snapshot API][1].

* A `RestoreItemAction` for `PersistentVolumeClaim`s named `velero.io/csi-pvc`.
Metadata (excluding labels) will be cleared, and the `velero.io/volume-snapshot-name` label will be used to find the relevant `VolumeSnapshot`.
A reference to the `VolumeSnapshot` will be added to the `PersistentVolumeClaim.DataSource` field.

No special logic is required to restore `VolumeSnapshotClass` objects.

These plugins should be provided with Velero, as there will also be some changes to core Velero code to enable association of a `Backup` to the included `VolumeSnapshot`s.

### Velero server changes

[`persistBackup`][8] will be extended to query for all `VolumeSnapshot`s associated with the backup, and persist the list to JSON.

[`BackupStore.PutBackup`][9] will receive an additional argument, `volumeSnapshots io.Reader`, that contains the JSON representation of `VolumeSnapshots`.
This will be written to a file named `csi-snapshots.json.gz`.

[`defaultRestorePriorities`][11] should be rewritten to the following to accomodate proper association between the CSI objects and PVCs. `CustomResourceDefinition`s are moved up because they're necessary for creating the CSI CRDs. The CSI CRDs are created before `PersistentVolume`s and `PersistentVolumeClaim`s so that they may be used as data sources.

```go
var defaultRestorePriorities = []string{
    "namespaces",
    "storageclasses",
    "customresourcedefinitions",
    "volumesnapshotclass.snapshot.storage.k8s.io",
    "volumesnapshots.snapshot.storage.k8s.io",
    "volumesnapshotcontents.snapshot.storage.k8s.io",
    "persistentvolumes",
    "persistentvolumeclaims",
    "secrets",
    "configmaps",
    "serviceaccounts",
    "limitranges",
    "pods",
    "replicaset",
}
```

## Velero client changes

[`DescribeBackupStatus`][13] will be extended to download the `csi-snapshots.json.gz` file for processing.

A new `describeCSIVolumeSnapshots` function should be added to the [output][12] package that knows how to render the included `VolumeSnapshot` names referenced in the `csi-snapshots.json.gz` file.

### CSI snapshot enablement

Some mechanism for enabling CSI snapshots, either at a global or individual volume level, is necessary.

Some options are:

    * providing a command line switch for the server that will enable CSI snapshotting for all volumes.
    * using annotations on pods to indicate which volumes to use CSI snapshotting on, akin to current restic support
    * a ConfigMap defining which StorageClasses are CSI-enabled, allowing plugins to query based on StorageClass and use CSI if applicable


## Notes on usage

In order for underlying, provider-level snapshots to be retained similarly to Velero's current functionality, the `VolumeSnapshotContent.DeletionPolicy` field must be set to `Retain`.

This is most easily accomplished by setting the `VolumeSnapshotClass.DeletionPolicy` field to `Retain`, which will be inherited by all `VolumeSnapshotContent` objects associated with the `VolumeSnapshotClass`.

It is not currently possible to define a deletion policy on a `VolumeSnapshot` that gets passed to a `VolumeSnapshotContent` object on an individual basis.

## Alternatives Considered

* Implementing similar logic in a Velero VolumeSnapshotter plugin was considered.
However, this is inappropriate given CSI's data model, which requires a PVC/PV's StorageClass.
Given the arguments to the VolumeSnapshotter interface, the plugin would have to instantiate its own client and do queries against the Kubernetes API server to get the necessary information.
This is unnecessary given the fact that the `BackupItemAction` and `RestoreItemAction` APIs can act directly on the appropriate objects.

* Implement CSI logic directly in Velero core code.
The plugins could be packaged separately, but that doesn't necessarily make sense with server and client changes being made to accomodate CSI snapshot lookup.

## Security Considerations

If this proposal has an impact to the security of the product, its users, or data stored or transmitted via the product, they must be addressed here.

[1]: https://kubernetes.io/blog/2018/10/09/introducing-volume-snapshot-alpha-for-kubernetes/
[2]: https://github.com/kubernetes-csi/external-snapshotter/blob/master/pkg/apis/volumesnapshot/v1alpha1/types.go#L41
[3]: https://github.com/kubernetes-csi/external-snapshotter/blob/master/pkg/apis/volumesnapshot/v1alpha1/types.go#L161
[4]: https://github.com/heptio/velero/blob/master/pkg/volume/snapshot.go#L21
[5]: https://github.com/heptio/velero/blob/master/pkg/apis/velero/v1/pod_volume_backup.go#L88
[6]: https://github.com/heptio/velero-csi-plugin/
[7]: https://github.com/heptio/velero/blob/master/pkg/plugin/velero/volume_snapshotter.go#L26
[8]: https://github.com/heptio/velero/blob/master/pkg/controller/backup_controller.go#L560
[9]: https://github.com/heptio/velero/blob/master/pkg/persistence/object_store.go#L46
[10]: https://github.com/heptio/velero/blob/master/pkg/apis/velero/v1/labels_annotations.go#L21
[11]: https://github.com/heptio/velero/blob/master/pkg/cmd/server/server.go#L471
[12]: https://github.com/heptio/velero/blob/master/pkg/cmd/util/output/backup_describer.go
[13]: https://github.com/heptio/velero/blob/master/pkg/cmd/util/output/backup_describer.go#L214
