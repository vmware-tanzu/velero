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

#### Restore the VolumeSnapshotContent
The current behavior of VSC restoration is that the VSC from the backup is restore, and the restored VS also triggers creating a new VSC dynamically.

Two VSCs created for the same VS in one restore seems not right.

Skip restore the VSC from the backup is not a viable alternative, because VSC may reference to a [snapshot create secret](https://kubernetes-csi.github.io/docs/secrets-and-credentials-volume-snapshot-class.html?highlight=snapshotter-secret-name#createdelete-volumesnapshot-secret).

If the `SkipRestore` is set true in the restore action's result, the secret returned in the additional items is ignored too.

As a result, restore the VSC from the backup, and setup the VSC and the VS's relation is a better choice.

Another consideration is the VSC name should not be the same as the backed-up VSC's, because the older version Velero's restore and backup keep the VSC after completion.

There's high possibility that the restore will fail due to the VSC already exists in the cluster.

Multiple restores of the same backup will also meet the same problem.

The proposed solution is using the restore's UID and the VS's name to generate sha256 hash value as the new VSC name. Both the VS and VSC RestoreItemAction can access those UIDs, and it will avoid the conflicts issues.

The restored VS name also shares the same generated name.

The VS-referenced VSC name and the VSC's snapshot handle name are in their status.

Velero restore process purges the restore resources' metadata and status before running the RestoreItemActions.

As a result, we cannot read these information in the VS and VSC RestoreItemActions.

Fortunately, RestoreItemAction input parameters includes the `ItemFromBackup`. The status is intact in `ItemFromBackup`.

``` go
func (p *volumeSnapshotRestoreItemAction) Execute(
	input *velero.RestoreItemActionExecuteInput,
) (*velero.RestoreItemActionExecuteOutput, error) {
	p.log.Info("Starting VolumeSnapshotRestoreItemAction")

	if boolptr.IsSetToFalse(input.Restore.Spec.RestorePVs) {
		p.log.Infof("Restore %s/%s did not request for PVs to be restored.",
			input.Restore.Namespace, input.Restore.Name)
		return &velero.RestoreItemActionExecuteOutput{SkipRestore: true}, nil
	}

	var vs snapshotv1api.VolumeSnapshot
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(
		input.Item.UnstructuredContent(), &vs); err != nil {
		return &velero.RestoreItemActionExecuteOutput{},
			errors.Wrapf(err, "failed to convert input.Item from unstructured")
	}

	var vsFromBackup snapshotv1api.VolumeSnapshot
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(
		input.ItemFromBackup.UnstructuredContent(), &vsFromBackup); err != nil {
		return &velero.RestoreItemActionExecuteOutput{},
			errors.Wrapf(err, "failed to convert input.Item from unstructured")
	}

	// If cross-namespace restore is configured, change the namespace
	// for VolumeSnapshot object to be restored
	newNamespace, ok := input.Restore.Spec.NamespaceMapping[vs.GetNamespace()]
	if !ok {
		// Use original namespace
		newNamespace = vs.Namespace
	}

	if csiutil.IsVolumeSnapshotExists(newNamespace, vs.Name, p.crClient) {
		p.log.Debugf("VolumeSnapshot %s already exists in the cluster. Return without change.", vs.Namespace+"/"+vs.Name)
		return &velero.RestoreItemActionExecuteOutput{UpdatedItem: input.Item}, nil
	}

	newVSCName := generateSha256FromRestoreAndVsUID(string(input.Restore.UID), string(vsFromBackup.UID))
	// Reset Spec to convert the VolumeSnapshot from using
	// the dynamic VolumeSnapshotContent to the static one.
	resetVolumeSnapshotSpecForRestore(&vs, &newVSCName)

	// Reset VolumeSnapshot annotation. By now, only change
	// DeletionPolicy to Retain.
	resetVolumeSnapshotAnnotation(&vs)

	vsMap, err := runtime.DefaultUnstructuredConverter.ToUnstructured(&vs)
	if err != nil {
		p.log.Errorf("Fail to convert VS %s to unstructured", vs.Namespace+"/"+vs.Name)
		return nil, errors.WithStack(err)
	}

	p.log.Infof(`Returning from VolumeSnapshotRestoreItemAction with 
		no additionalItems`)

	return &velero.RestoreItemActionExecuteOutput{
		UpdatedItem:     &unstructured.Unstructured{Object: vsMap},
		AdditionalItems: []velero.ResourceIdentifier{},
	}, nil
}

// generateSha256FromRestoreAndVsUID Use the restore UID and the VS UID to generate the new VSC name.
// By this way, VS and VSC RIA action can get the same VSC name.
func generateSha256FromRestoreAndVsUID(restoreUID string, vsUID string) string {
	sha256Bytes := sha256.Sum256([]byte(restoreUID + "/" + vsUID))
	return "vsc-" + hex.EncodeToString(sha256Bytes[:])
}
```

#### Restore the VolumeSnapshot
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

	var vsc snapshotv1api.VolumeSnapshotContent
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(
		input.Item.UnstructuredContent(), &vsc); err != nil {
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
	newNamespace, ok := input.Restore.Spec.NamespaceMapping[vsc.Spec.VolumeSnapshotRef.Namespace]
	if ok {
		// Update the referenced VS namespace to the mapping one.
		vsc.Spec.VolumeSnapshotRef.Namespace = newNamespace
	}

	// Reset VSC name to align with VS.
	vsc.Name = generateSha256FromRestoreAndVsUID(string(input.Restore.UID), string(vscFromBackup.Spec.VolumeSnapshotRef.UID))

	// Reset the ResourceVersion and UID of referenced VolumeSnapshot.
	vsc.Spec.VolumeSnapshotRef.ResourceVersion = ""
	vsc.Spec.VolumeSnapshotRef.UID = ""

	// Set the DeletionPolicy to Retain to avoid VS deletion will not trigger snapshot deletion
	vsc.Spec.DeletionPolicy = snapshotv1api.VolumeSnapshotContentRetain

	if vscFromBackup.Status != nil && vscFromBackup.Status.SnapshotHandle != nil {
		vsc.Spec.Source.VolumeHandle = nil
		vsc.Spec.Source.SnapshotHandle = vscFromBackup.Status.SnapshotHandle
	} else {
		p.log.Errorf("fail to get snapshot handle from VSC %s status", vsc.Name)
		return nil, errors.Errorf("fail to get snapshot handle from VSC %s status", vsc.Name)
	}

	additionalItems := []velero.ResourceIdentifier{}
	if csi.IsVolumeSnapshotContentHasDeleteSecret(&vsc) {
		additionalItems = append(additionalItems,
			velero.ResourceIdentifier{
				GroupResource: schema.GroupResource{Group: "", Resource: "secrets"},
				Name:          vsc.Annotations[velerov1api.PrefixedSecretNameAnnotation],
				Namespace:     vsc.Annotations[velerov1api.PrefixedSecretNamespaceAnnotation],
			},
		)
	}

	vscMap, err := runtime.DefaultUnstructuredConverter.ToUnstructured(&vsc)
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

To avoid the created VSC conflict with older version Velero B/R generated ones, the VSC name is set to `vsc-uuid`.

The following is an example of the implementation.
``` go
	uuid, err := uuid.NewRandom()
	if err != nil {
		p.log.WithError(err).Errorf("Fail to generate the UUID to create VSC %s", snapCont.Name)
		return errors.Wrapf(err, "Fail to generate the UUID to create VSC %s", snapCont.Name)
	}
	snapCont.Name = "vsc-" + uuid.String()

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
