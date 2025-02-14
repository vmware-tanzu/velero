# Design to clean the artifacts generated in the CSI backup and restore workflows

## Terminology

* VSC: VolumeSnapshotContent
* VS: VolumeSnapshot

## Abstract
* The design aims to delete the unnecessary VSs and VSCs generated during CSI backup and restore process. 
* The design stop creating related VSCs during backup syncing.

## Background
In the current CSI backup and restore workflows, please notice the CSI B/R workflows means only using the CSI snapshots in the B/R, not including the CSI snapshot data movement workflows, some generated artifacts are kept after the backup or the restore process completion.

Some of them are kept due to design, for example, the VolumeSnapshotContents generated during the backup are kept to make sure the backup deletion can clean the snapshots in the storage providers.

Some of them are kept by accident, for example, after restore, two VolumeSnapshotContents are generated for the same VolumeSnapshot. One is from the backup content, and one is dynamically generated from the restore's VolumeSnapshot.

The design aims to clean the unnecessary artifacts, and make the CSI B/R workflow more concise and reliable.

## Goals
- Clean the redundant VSC generated during CSI backup and restore.
- Remove the VSCs in the backup sync process.

## Non Goals
- There were some discussion about whether Velero backup should include VSs and VSCs not generated in during the backup. By far, the conclusion is not including them is a better option. Although that is a useful enhancement, that is not included this design.
- Delete all the CSI-related metadata files in the BSL is not the aim of this design. 

## Detailed Design
### Backup
During backup, the main change is the backup-generated VSCs should not kept anymore.

The reasons is we don't need them to ensure the snapshots clean up during backup deletion. Please reference to the [Backup Deletion section](#backup-deletion) section for detail.

As a result, we can simplify the VS deletion logic in the backup. Before, we need to not only delete the VS, but also recreate a static VSC pointing a non-exiting VS.

The deletion code in VS BackupItemAction can be simplify to the following:

``` go
	if backup.Status.Phase == velerov1api.BackupPhaseFinalizing ||
		backup.Status.Phase == velerov1api.BackupPhaseFinalizingPartiallyFailed {
		p.log.
			WithField("Backup", fmt.Sprintf("%s/%s", backup.Namespace, backup.Name)).
			WithField("BackupPhase", backup.Status.Phase).Debugf("Cleaning VolumeSnapshots.")

		if vsc == nil {
			vsc = &snapshotv1api.VolumeSnapshotContent{}
		}

		csi.DeleteReadyVolumeSnapshot(*vs, *vsc, p.crClient, p.log)
		return item, nil, "", nil, nil
	}


func DeleteReadyVolumeSnapshot(
	vs snapshotv1api.VolumeSnapshot,
	vsc snapshotv1api.VolumeSnapshotContent,
	client crclient.Client,
	logger logrus.FieldLogger,
) {
	logger.Infof("Deleting Volumesnapshot %s/%s", vs.Namespace, vs.Name)
	if vs.Status == nil ||
		vs.Status.BoundVolumeSnapshotContentName == nil ||
		len(*vs.Status.BoundVolumeSnapshotContentName) <= 0 {
		logger.Errorf("VolumeSnapshot %s/%s is not ready. This is not expected.",
			vs.Namespace, vs.Name)
		return
	}

	if vs.Status != nil && vs.Status.BoundVolumeSnapshotContentName != nil {
		// Patch the DeletionPolicy of the VolumeSnapshotContent to set it to Retain.
		// This ensures that the volume snapshot in the storage provider is kept.
		if err := SetVolumeSnapshotContentDeletionPolicy(
			vsc.Name,
			client,
			snapshotv1api.VolumeSnapshotContentRetain,
		); err != nil {
			logger.Warnf("Failed to patch DeletionPolicy of volume snapshot %s/%s",
				vs.Namespace, vs.Name)
			return
		}

		if err := client.Delete(context.TODO(), &vsc); err != nil {
			logger.Warnf("Failed to delete the VSC %s: %s", vsc.Name, err.Error())
		}
	}
	if err := client.Delete(context.TODO(), &vs); err != nil {
		logger.Warnf("Failed to delete volumesnapshot %s/%s: %v", vs.Namespace, vs.Name, err)
	} else {
		logger.Infof("Deleted volumesnapshot with volumesnapshotContent %s/%s",
			vs.Namespace, vs.Name)
	}
}
```

### Restore

#### Restore the VolumeSnapshotContent from the backup instead of creating a new one dynamically
The current behavior of VSC restoration is that the VSC from the backup is restore, and the restored VS also triggers creating a new VSC dynamically.

Two VSCs created for the same VS in one restore seems not right.

Skip restore the VSC from the backup is not a viable alternative, because VSC may reference to a [snapshot create secret](https://kubernetes-csi.github.io/docs/secrets-and-credentials-volume-snapshot-class.html?highlight=snapshotter-secret-name#createdelete-volumesnapshot-secret).

If the `SkipRestore` is set true in the restore action's result, the secret returned in the additional items is ignored too.

As a result, restore the VSC from the backup, and setup the VSC and the VS's relation is a better choice.

The VS-referenced VSC name and the VSC's snapshot handle name are in their status.

Velero restore process purges the restore resources' metadata and status before running the RestoreItemActions.

As a result, we cannot read these information in the VS and VSC RestoreItemActions.

Fortunately, RestoreItemAction input parameters includes the `ItemFromBackup`. The status is intact in `ItemFromBackup`.

``` go
// Execute restores a VolumeSnapshotContent object without modification
// returning the snapshot lister secret, if any, as additional items to restore.
func (p *volumeSnapshotContentRestoreItemAction) Execute(
	input *velero.RestoreItemActionExecuteInput,
) (*velero.RestoreItemActionExecuteOutput, error) {
	if boolptr.IsSetToFalse(input.Restore.Spec.RestorePVs) {
		p.log.Infof("Restore did not request for PVs to be restored %s/%s",
			input.Restore.Namespace, input.Restore.Name)
		return &velero.RestoreItemActionExecuteOutput{SkipRestore: true}, nil
	}

	p.log.Info("Starting VolumeSnapshotContentRestoreItemAction")

	var snapCont snapshotv1api.VolumeSnapshotContent
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(
		input.Item.UnstructuredContent(), &snapCont); err != nil {
		return &velero.RestoreItemActionExecuteOutput{},
			errors.Wrapf(err, "failed to convert input.Item from unstructured")
	}

	var vscFromBackup snapshotv1api.VolumeSnapshotContent
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(
		input.ItemFromBackup.UnstructuredContent(), &vscFromBackup); err != nil {
		return &velero.RestoreItemActionExecuteOutput{},
			errors.Errorf(err.Error(), "failed to convert input.ItemFromBackup from unstructured")
	}

	// If cross-namespace restore is configured, change the namespace
	// for VolumeSnapshot object to be restored
	newNamespace, ok := input.Restore.Spec.NamespaceMapping[snapCont.Spec.VolumeSnapshotRef.Namespace]
	if ok {
		// Update the referenced VS namespace to the mapping one.
		snapCont.Spec.VolumeSnapshotRef.Namespace = newNamespace
	}

	// Reset the ResourceVersion and UID of referenced VolumeSnapshot.
	snapCont.Spec.VolumeSnapshotRef.ResourceVersion = ""
	snapCont.Spec.VolumeSnapshotRef.UID = ""

	// Set the DeletionPolicy to Retain to avoid VS deletion will not trigger snapshot deletion
	snapCont.Spec.DeletionPolicy = snapshotv1api.VolumeSnapshotContentRetain

	if vscFromBackup.Status != nil && vscFromBackup.Status.SnapshotHandle != nil {
		snapCont.Spec.Source.VolumeHandle = nil
		snapCont.Spec.Source.SnapshotHandle = vscFromBackup.Status.SnapshotHandle
	} else {
		p.log.Errorf("fail to get snapshot handle from VSC %s status", snapCont.Name)
		return nil, errors.Errorf("fail to get snapshot handle from VSC %s status", snapCont.Name)
	}

	additionalItems := []velero.ResourceIdentifier{}
	if csi.IsVolumeSnapshotContentHasDeleteSecret(&snapCont) {
		additionalItems = append(additionalItems,
			velero.ResourceIdentifier{
				GroupResource: schema.GroupResource{Group: "", Resource: "secrets"},
				Name:          snapCont.Annotations[velerov1api.PrefixedSecretNameAnnotation],
				Namespace:     snapCont.Annotations[velerov1api.PrefixedSecretNamespaceAnnotation],
			},
		)
	}

	vscMap, err := runtime.DefaultUnstructuredConverter.ToUnstructured(&snapCont)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	p.log.Infof("Returning from VolumeSnapshotContentRestoreItemAction with %d additionalItems",
		len(additionalItems))
	return &velero.RestoreItemActionExecuteOutput{
		UpdatedItem:     &unstructured.Unstructured{Object: vscMap},
		AdditionalItems: additionalItems,
	}, nil
}
```

Because VSC is not restored dynamically, if we run restores two times for the same backup in the same cluster, the second restore will fail due to the VSC is there in the cluster, and it is already bound to an existing VS.

To avoid this issue, it's better to delete the restored VS and VSC after restore completes.

The VolumeSnapshot and VolumeSnapshotContent are delete in the `restore_finalizer_controller`.

``` go
func (ctx *finalizerContext) deleteVolumeSnapshotAndVolumeSnapshotContent() (errs results.Result) {
	for _, operation := range ctx.restoreItemOperationList.items {
		if operation.Spec.RestoreItemAction == constant.PluginCsiVolumeSnapshotRestoreRIA &&
			operation.Status.Phase == itemoperation.OperationPhaseCompleted {
			if operation.Spec.OperationID == "" || !strings.Contains(operation.Spec.OperationID, "/") {
				ctx.logger.Errorf("invalid OperationID: %s", operation.Spec.OperationID)
				errs.Add("", errors.Errorf("invalid OperationID: %s", operation.Spec.OperationID))
				continue
			}

			operationIDParts := strings.Split(operation.Spec.OperationID, "/")

			vs := new(snapshotv1api.VolumeSnapshot)
			vsc := new(snapshotv1api.VolumeSnapshotContent)

			if err := ctx.crClient.Get(
				context.TODO(),
				client.ObjectKey{Namespace: operationIDParts[0], Name: operationIDParts[1]},
				vs,
			); err != nil {
				ctx.logger.Errorf("Fail to get the VolumeSnapshot %s: %s", operation.Spec.OperationID, err.Error())
				errs.Add(operationIDParts[0], errors.Errorf("Fail to get the VolumeSnapshot %s: %s", operation.Spec.OperationID, err.Error()))
				continue
			}

			if err := ctx.crClient.Delete(context.TODO(), vs); err != nil {
				ctx.logger.Errorf("Fail to delete VolumeSnapshot %s: %s", operation.Spec.OperationID, err.Error())
				errs.Add(vs.Namespace, err)
			}

			if vs.Status != nil && vs.Status.BoundVolumeSnapshotContentName != nil {
				vsc.Name = *vs.Status.BoundVolumeSnapshotContentName
			} else {
				ctx.logger.Errorf("VolumeSnapshotContent %s is not ready.", vsc.Name)
				errs.Add("", errors.Errorf("VolumeSnapshotContent %s is not ready.", vsc.Name))
				continue
			}

			if err := ctx.crClient.Delete(context.TODO(), vsc); err != nil {
				ctx.logger.Errorf("Fail to delete VolumeSnapshotContent %s: %s", vsc.Name, err.Error())
				errs.Add("", errors.Errorf("Fail to delete the VolumeSnapshotContent %s", err))
			}
		}
	}

	return errs
}
```

### Backup Sync
csi-volumesnapshotclasses.json, csi-volumesnapshotcontents.json, and csi-volumesnapshots.json are CSI-related metadata files in the BSL for each backup.

csi-volumesnapshotcontents.json and csi-volumesnapshots.json are not needed anymore, but csi-volumesnapshotclasses.json is still needed.

One concrete scenario is that a backup is created in cluster-A, then the backup is synced to cluster-B, and the backup is deleted in the cluster-B. In this case, we don't have a chance to create the VS and VSC needed VolumeSnapshotClass.

The VSC deletion workflow proposed by this design needs to create the VSC first. If the VSC's referenced VolumeSnapshotClass doesn't exist in cluster, the creation of VSC will fail.

As a result, the VolumeSnapshotClass should still be synced in the backup sync process.

### Backup Deletion
Two factors are worthy for consideration for the backup deletion change:
* Because the VSCs generated by the backup are not synced anymore, and the VSCs generated during the backup will not be kept too. The backup deletion needs to generate a VSC, then deletes it to make sure the snapshots in the storage provider are clean too.
* The VSs generated by the backup are already deleted in the backup process, we don't need a DeleteItemAction for the VS anymore. As a result, the `velero.io/csi-volumesnapshot-delete` plugin is unneeded.

For the VSC DeleteItemAction, we need to generate a VSC. Because we only care about the snapshot deletion, we don't need to create a VS associated with the VSC.

Create a static VSC, then point it to a pseudo VS, and reference to the snapshot handle should be enough.

The following is an example of the implementation.
``` go
	snapCont.Spec.DeletionPolicy = snapshotv1api.VolumeSnapshotContentDelete

	snapCont.Spec.Source = snapshotv1api.VolumeSnapshotContentSource{
		SnapshotHandle: snapCont.Status.SnapshotHandle,
	}

	snapCont.Spec.VolumeSnapshotRef = corev1api.ObjectReference{
		APIVersion: snapshotv1api.SchemeGroupVersion.String(),
		Kind:       "VolumeSnapshot",
		Namespace:  "ns-" + string(snapCont.UID),
		Name:       "name-" + string(snapCont.UID),
	}

	snapCont.ResourceVersion = ""

	if err := p.crClient.Create(context.TODO(), &snapCont); err != nil {
		return errors.Wrapf(err, "fail to create VolumeSnapshotContent %s", snapCont.Name)
	}

	// Read resource timeout from backup annotation, if not set, use default value.
	timeout, err := time.ParseDuration(
		input.Backup.Annotations[velerov1api.ResourceTimeoutAnnotation])
	if err != nil {
		p.log.Warnf("fail to parse resource timeout annotation %s: %s",
			input.Backup.Annotations[velerov1api.ResourceTimeoutAnnotation], err.Error())
		timeout = 10 * time.Minute
	}
	p.log.Debugf("resource timeout is set to %s", timeout.String())

	interval := 5 * time.Second

	// Wait until VSC created and ReadyToUse is true.
	if err := wait.PollUntilContextTimeout(
		context.Background(),
		interval,
		timeout,
		true,
		func(ctx context.Context) (bool, error) {
			tmpVSC := new(snapshotv1api.VolumeSnapshotContent)
			if err := p.crClient.Get(ctx, crclient.ObjectKeyFromObject(&snapCont), tmpVSC); err != nil {
				return false, errors.Wrapf(
					err, "failed to get VolumeSnapshotContent %s", snapCont.Name,
				)
			}

			if tmpVSC.Status != nil && boolptr.IsSetToTrue(tmpVSC.Status.ReadyToUse) {
				return true, nil
			}

			return false, nil
		},
	); err != nil {
		return errors.Wrapf(err, "fail to wait VolumeSnapshotContent %s becomes ready.", snapCont.Name)
	}
```

## Security Considerations
Security is not relevant to this design.

## Compatibility
In this design, no new information is added in backup and restore. As a result, this design doesn't have any compatibility issue.

## Open Issues
Please notice the CSI snapshot backup and restore mechanism not supporting all file-store-based volume, e.g. Azure Files, EFS or vSphere CNS File Volume. Only block-based volumes are supported.
Refer to [this comment](https://github.com/vmware-tanzu/velero/issues/3151#issuecomment-2623507686) for more details.
