# CSI Snapshot Support

Status: Draft

The Container Storage Interface (CSI) [introduced an alpha snapshot API in Kubernetes v1.12][1].
Current plans indicate it will reach beta support in Kubernetes v1.16.
This proposal documents an approach for integrating support for this snapshot API within Velero, augmenting its existing capabilities.

## Goals

- Enable Velero to backup and restore CSI-backed volumes using the Kubernetes CSI CustomResourceDefinition API

## Non Goals

- Replacing Velero's existing [VolumeSnapshotter][7] API
- Replacing Velero's Restic support

## Background

Velero has had support for performing persistent volume snapshots since its inception.
However, support has been limited to a handful of providers.
The plugin API introduced in Velero v0.7 enabled the community to expand the number of supported providers.
In the meantime, the Kubernetes sig-storage advanced the CSI spec to allow for a generic storage interface, opening up the possibility of moving storage code out of the core Kubernetes code base.
The CSI working group has also developed a generic snapshotting API that any CSI driver developer may implement, giving users the ability to snapshot volumes from a standard interface.

By supporting the CSI snapshot API, Velero can extend its support to any CSI driver, without requiring a Velero-specific plugin be written, easing the development burden on providers while also reaching more end users.

## High-Level Design

In order to support CSI's snapshot API, Velero must interact with the [`VolumeSnapshot`][2] and [`VolumeSnapshotContent`][3] CRDs.
These act as requests to the CSI driver to perform a snapshot on the underlying provider's volume.
This can largely be accomplished with Velero `BackupItemAction` and `RestoreItemAction` plugins that operate on these CRDs.

Additionally, changes to the Velero server and client code are necessary to track `VolumeSnapshot`s that are associated with a given backup, similarly to how Velero tracks its own [`volume.Snapshot`][4] type.
Tracking these is important for allowing users to see what is in their backup, and provides parity for the existing `volume.Snapshot` and [`PodVolumeBackup`][5] types.
This is also done to retain the object store as Velero's source of truth, without having to query the Kubernetes API server for associated `VolumeSnapshot`s.

`velero backup describe --details` will use the stored VolumeSnapshots to list CSI snapshots included in the backup to the user.

## Detailed Design

### Resource Plugins

A set of [prototype][6] plugins was developed that informed this design.

The plugins will be as follows:


#### A `BackupItemAction` for `PersistentVolumeClaim`s, named `velero.io/csi-pvc` 

This plugin will act directly on PVCs, since an implementation of Velero's VolumeSnapshotter does not have enough information about the StorageClass to properly create the `VolumeSnapshot` objects.

The associated PV will be queried and checked for the presence of `PersistentVolume.Spec.PersistentVolumeSource.CSI`. (See the "Snapshot Mechanism Selection" section below).
If this field is `nil`, then the plugin will return early without taking action.

Create a `VolumeSnapshot.snapshot.storage.k8s.io` object from the PVC.
Label the `VolumeSnapshot` object with the [`velero.io/backup-name`][10] label for ease of lookup later.
Also set an ownerRef on the `VolumeSnapshot` so that cascading deletion of the Velero `Backup` will delete associated `VolumeSnapshots`.

The CSI controllers will create a `VolumeSnapshotContent.snapshot.storage.k8s.io` object associated with the `VolumeSnapshot`.

Associated `VolumeSnapshotContent` objects will be retrieved and updated with the [`velero.io/backup-name`][10] label for ease of lookup later.
`velero.io/volume-snapshot-name` will be applied as a label to the PVC so that the `VolumeSnapshot` can be found easily for restore.

`VolumeSnapshot`, `VolumeSnapshotContent`, and `VolumeSnapshotClass` objects would be returned as additional items to be backed up. GitHub issue [1566][18] represents this work.

The `VolumeSnapshotContent.Spec.VolumeSnapshotSource.SnapshotHandle` field is the link to the underlying platform's snapshot, and must be preserved for restoration.

The plugin will _not_ wait for the `VolumeSnapshot.Status.IsReady` field to be `true` before returning.
This maintains current Velero behavior, though may not be desirable for all storage providers.

#### A `RestoreItemAction` for `VolumeSnapshotContent` objects, named `velero.io/csi-vsc`

On restore, `VolumeSnapshotContent` objects are cleaned so that they may be properly associated with IDs assigned by the target cluster.

Only `VolumeSnapshotContent` objects with the `velero.io/backup-name` label will be processed; if the label is missing, the plugin will return the unmodified object.

The metadata (excluding labels), `PersistentVolumeClaim.UUID`, and `VolumeSnapshotRef.UUID` fields will be cleared.
The reference fields are cleared because the associated objects will get new UUIDs in the cluster.
This also maps to the "import" case of [the snapshot API][1].

#### A `RestoreItemAction` for `VolumeSnapshot` objects, named `velero.io/csi-vs`

`VolumeSnapshot` objects must be prepared for importing into the target cluster by removing IDs and metadata associated with their origin cluster.

Only `VolumeSnapshot` objects with the `velero.io/backup-name` label will be processed; if the label is missing, the plugin will return the unmodified object.

Metadata (excluding labels) and `Source` fields on the object will be cleared.
The `VolumeSnapshot.Spec.SnapshotContentName` is the link back to the `VolumeSnapshotContent` object, and thus the actual snapshot.
The `Source` field indicates that a new CSI snapshot operation should be performed, which isn't relevant on restore.
This follows the "import" case of [the snapshot API][1].

The `Backup` associated with the `VolumeSnapshot` will be queried, and set as an ownerRef on the `VolumeSnapshot` so that deletion can cascade.

#### A `RestoreItemAction` for `PersistentVolumeClaim`s named `velero.io/csi-pvc`

On restore, `PersistentVolumeClaims` will need to be created from the snapshot, and thus will require editing before submission.

Only `PersistentVolumeClaim` objects with the `velero.io/volume-snapshot-name` label will be processed; if the label is missing, the plugin will return the unmodified object.
Metadata (excluding labels) will be cleared, and the `velero.io/volume-snapshot-name` label will be used to find the relevant `VolumeSnapshot`.
A reference to the `VolumeSnapshot` will be added to the `PersistentVolumeClaim.DataSource` field.


No special logic is required to restore `VolumeSnapshotClass` objects.

These plugins should be provided with Velero, as there will also be some changes to core Velero code to enable association of a `Backup` to the included `VolumeSnapshot`s.

### Velero server changes

[`persistBackup`][8] will be extended to query for all `VolumeSnapshot`s associated with the backup, and persist the list to JSON.

[`BackupStore.PutBackup`][9] will receive an additional argument, `volumeSnapshots io.Reader`, that contains the JSON representation of `VolumeSnapshots`.
This will be written to a file named `csi-snapshots.json.gz`.

[`defaultRestorePriorities`][11] should be rewritten to the following to accomodate proper association between the CSI objects and PVCs. `CustomResourceDefinition`s are moved up because they're necessary for creating the CSI CRDs. The CSI CRDs are created before `PersistentVolume`s and `PersistentVolumeClaim`s so that they may be used as data sources.
GitHub issue [1565][17] represents this work.

```go
var defaultRestorePriorities = []string{
    "namespaces",
    "storageclasses",
    "customresourcedefinitions",
    "volumesnapshotclass.snapshot.storage.k8s.io",
    "volumesnapshotcontents.snapshot.storage.k8s.io",
    "volumesnapshots.snapshot.storage.k8s.io",
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

[`DescribeBackupStatus`][13] will be extended to download the `csi-snapshots.json.gz` file for processing. GitHub Issue [1568][19] captures this work.

A new `describeCSIVolumeSnapshots` function should be added to the [output][12] package that knows how to render the included `VolumeSnapshot` names referenced in the `csi-snapshots.json.gz` file.

### Snapshot mechanism selection

The most accurate, reliable way to detect if a PersistentVolume is a CSI volume is to check for a non-`nil` [`PersistentVolume.Spec.PersistentVolumeSource.CSI`][16] field.
Using the [`volume.beta.kubernetes.io/storage-provisioner`][14] is not viable, since the usage is for any PVC that should be dynamically provisioned, and is _not_ limited to CSI implementations.
It was [introduced with dynamic provisioning support][15] in 2016, predating CSI.

In the `BackupItemAction` for PVCs, the associated PV will be queried and checked for the presence of `PersistentVolume.Spec.PersistentVolumeSource.CSI`.
Volumes with any other `PersistentVolumeSource` set will use Velero's current VolumeSnapshotter plugin code path.

Volumes found in a `Pod`'s `backup.velero.io/backup-volumes` list will use Velero's current Restic code path.
This also means Velero will continue to offer Restic as an option for CSI volumes.


### Garbage collection and deletion

To ensure that all created resources are deleted when a backup expires or is deleted, `VolumeSnapshot`s will have an `ownerRef` defined pointing to the Velero backup that created them.

In order to fully delete these objects, each `VolumeSnapshotContent`s object will need to be edited to ensure the associated provider snapshot is deleted.
This will be done by editing the object and setting `VolumeSnapshotContent.Spec.DeletionPolicy` to `Delete`, regardless of whether or not the default policy for the class is `Retain`.
See the Deletion Policies section below.


## Alternatives Considered

* Implementing similar logic in a Velero VolumeSnapshotter plugin was considered.
However, this is inappropriate given CSI's data model, which requires a PVC/PV's StorageClass.
Given the arguments to the VolumeSnapshotter interface, the plugin would have to instantiate its own client and do queries against the Kubernetes API server to get the necessary information.
This is unnecessary given the fact that the `BackupItemAction` and `RestoreItemAction` APIs can act directly on the appropriate objects.

* Implement CSI logic directly in Velero core code.
The plugins could be packaged separately, but that doesn't necessarily make sense with server and client changes being made to accomodate CSI snapshot lookup.

* Implementing the CSI logic entirely in external plugins.
As mentioned above, the necessary plugins for `PersistentVolumeClaim`, `VolumeSnapshot`, and `VolumeSnapshotContent` could be hosted out-out-of-tree from Velero.
In fact, much of the logic for creating the CSI objects will be driven entirely inside of the plugin implementation.

However, Velero currently has no way for plugins to communicate that some arbitrary data should be stored in or retrieved from object storage, such as list of all `VolumeSnapshot` objects associated with a given `Backup`.
This is important, because to display snapshots included in a backup, whether as native snapshots or Restic backups, separate JSON-encoded lists are stored within the backup on object storage.
Snapshots are not listed directly on the `Backup` to fit within the etcd size limitations.
Additionally, there are no client-side Velero plugin mechanisms, which means that the `velero describe backup --details` command would have no way of displaying the objects to the user, even if they were stored.

## Deletion Policies

In order for underlying, provider-level snapshots to be retained similarly to Velero's current functionality, the `VolumeSnapshotContent.Spec.DeletionPolicy` field must be set to `Retain`.

This is most easily accomplished by setting the `VolumeSnapshotClass.DeletionPolicy` field to `Retain`, which will be inherited by all `VolumeSnapshotContent` objects associated with the `VolumeSnapshotClass`.

The current default for dynamically provisioned `VolumeSnapshotContent` objects is `Delete`, which will delete the provider-level snapshot when the `VolumeSnapshotContent` object representing it is deleted.
Additionally, the `Delete` policy will cascade a deletion of a `VolumeSnapshot`, removing the associated `VolumeSnapshotContent` object.

It is not currently possible to define a deletion policy on a `VolumeSnapshot` that gets passed to a `VolumeSnapshotContent` object on an individual basis.

## Security Considerations

This proposal does not significantly change Velero's security implications within a cluster.

If a deployment is using solely CSI volumes, Velero will no longer need privileges to interact with volumes or snapshots, as these will be handled by the CSI driver.
This reduces the provider permissions footprint of Velero.

Velero must still be able to access cluster-scoped resources in order to back up `VolumeSnapshotContent` objects.
Without these objects, the provider-level snapshots cannot be located in order to re-associate them with volumes in the event of a restore.



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
[14]: https://github.com/kubernetes/kubernetes/blob/8ea9edbb0290e9de1e6d274e816a4002892cca6f/pkg/controller/volume/persistentvolume/util/util.go#L69
[15]: https://github.com/kubernetes/kubernetes/pull/30285
[16]: https://github.com/kubernetes/kubernetes/blob/master/pkg/apis/core/types.go#L237
[17]: https://github.com/heptio/velero/issues/1565
[18]: https://github.com/heptio/velero/issues/1566
[19]: https://github.com/heptio/velero/issues/1568
