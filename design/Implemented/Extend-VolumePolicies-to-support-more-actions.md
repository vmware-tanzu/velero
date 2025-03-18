# Extend VolumePolicies to support more actions

## Abstract

Currently, the [VolumePolicies feature](https://github.com/vmware-tanzu/velero/blob/main/design/Implemented/handle-backup-of-volumes-by-resources-filters.md) which can be used to filter/handle volumes during backup only supports the skip action on matching conditions. Users need more actions to be supported.

## Background

The `VolumePolicies` feature was introduced in Velero 1.11 as a flexible way to handle volumes. The main agenda of
introducing the VolumePolicies feature was to improve the overall user experience when performing backup operations
for volume resources, the feature enables users to group volumes according the `conditions` (criteria) specified and
also lets you specify the `action` that velero needs to take for these grouped volumes during the backup operation.
The limitation being that currently `VolumePolicies` only supports `skip` as an action, We want to extend the `action`
functionality to support more usable options like `fs-backup` (File system backup) and `snapshot` (VolumeSnapshots).

## Goals
- Extending the VolumePolicies to support more actions like `fs-backup` (File system backup) and `snapshot` (VolumeSnapshots).
- Improve user experience when backing up Volumes via Velero

## Non-Goals
- No changes to existing approaches to opt-in/opt-out annotations for volumes
- No changes to existing `VolumePolicies` functionalities
- No additions or implementations to support more granular actions like `snapshot-csi` and `snapshot-datamover`. These actions can be implemented as a future enhancement


## Use-cases/Scenarios

**Use-case 1:**
- A user wants to use `snapshot` (volumesnapshots) backup option for all the csi supported volumes and `fs-backup` for the rest of the volumes.
- Currently, velero supports this use-case but the user experience is not that great.
- The user will have to individually annotate the volume mounting pod with the annotation "backup.velero.io/backup-volumes" for `fs-backup`
- This becomes cumbersome at scale.
- Using `VolumePolicies`, the user can just specify 2 simple `VolumePolicies` like for csi supported volumes as `snapshot` action and rest can be backed up`fs-backup` action:
```yaml
version: v1
volumePolicies:
  - conditions:
      storageClass:
        - gp2
    action:
      type: snapshot
  - conditions: {}
    action:
      type: fs-backup
```

**Use-case 2:**
- A user wants to use `fs-backup` for nfs volumes pertaining to a particular server
- In such a scenario the user can just specify a `VolumePolicy` like:
```yaml
version: v1
volumePolicies:
- conditions:
    nfs:
      server: 192.168.200.90
  action:
    type: fs-backup
```
## High-Level Design
- When the VolumePolicy action is set as `fs-backup` the backup workflow modifications would be:
  - We call [backupItem() -> backupItemInternal()](https://github.com/vmware-tanzu/velero/blob/main/pkg/backup/item_backupper.go#L95) on all the items that are to be backed up
  - Here when we encounter [Pod as an item ](https://github.com/vmware-tanzu/velero/blob/main/pkg/backup/item_backupper.go#L195)
  - We will have to modify the backup workflow to account for the `fs-backup` VolumePolicy action


- When the VolumePolicy action is set as `snapshot` the backup workflow modifications would be:
  - Once again, We call [backupItem() -> backupItemInternal()](https://github.com/vmware-tanzu/velero/blob/main/pkg/backup/item_backupper.go#L95) on all the items that are to be backed up
  - Here when we encounter [Persistent Volume as an item](https://github.com/vmware-tanzu/velero/blob/d4128542590470b204a642ee43311921c11db880/pkg/backup/item_backupper.go#L253)
  - And we call the [takePVSnapshot func](https://github.com/vmware-tanzu/velero/blob/d4128542590470b204a642ee43311921c11db880/pkg/backup/item_backupper.go#L508)
  - We need to modify the takePVSnapshot function to account for the `snapshot` VolumePolicy action.
  - In case of csi snapshots for PVC objects, these snapshot actions are taken by the velero-plugin-for-csi, we need to modify the [executeActions()](https://github.com/vmware-tanzu/velero/blob/512fe0dabdcb3bbf1ca68a9089056ae549663bcf/pkg/backup/item_backupper.go#L232) function to account for the `snapshot` VolumePolicy action.

**Note:** `Snapshot` action can either be a native snapshot or a csi snapshot, as is the case with the current flow where velero itself makes the decision based on the backup CR.

## Detailed Design
- Update VolumePolicy action type validation to account for `fs-backup` and `snapshot` as valid VolumePolicy actions.
- Modifications needed for `fs-backup` action:
  - Now based on the specification of volume policy on backup request we will decide whether to go via legacy pod annotations approach or the newer volume policy based fs-backup action approach.
  - If there is a presence of volume policy(fs-backup/snapshot) on the backup request that matches as an action for a volume we use the newer volume policy approach to get the list of the volumes for `fs-backup` action
  - Else continue with the annotation based legacy approach workflow.

- Modifications needed for `snapshot` action:
  - In the [takePVSnapshot function](https://github.com/vmware-tanzu/velero/blob/d4128542590470b204a642ee43311921c11db880/pkg/backup/item_backupper.go#L508) we will check the PV fits the volume policy criteria and see if the associated action is `snapshot`
  - If it is not snapshot then we skip the further workflow and avoid taking the snapshot of the PV
  - Similarly, For csi snapshot of PVC object, we need to do similar changes in [executeAction() function](https://github.com/vmware-tanzu/velero/blob/512fe0dabdcb3bbf1ca68a9089056ae549663bcf/pkg/backup/item_backupper.go#L348). we will check the PVC fits the volume policy criteria and see if the associated action is `snapshot` via csi plugin
  - If it is not snapshot then we skip the csi BIA execute action and avoid taking the snapshot of the PVC by not invoking the csi plugin action for the PVC

**Note:**
- When we are using the `VolumePolicy` approach for backing up the volumes then the volume policy criteria and action need to be specific and explicit, there is no default behavior, if a volume matches `fs-backup` action then `fs-backup` method will be used for that volume and similarly if the volume matches the criteria for `snapshot` action then the snapshot workflow will be used for the volume backup.
- Another thing to note is the workflow proposed in this design uses the legacy `opt-in/opt-out` approach as a fallback option. For instance, the user specifies a VolumePolicy but for a particular volume included in the backup there are no actions(fs-backup/snapshot) matching in the volume policy for that volume, in such a scenario the legacy approach will be used for backing up the particular volume.
- The relation between the `VolumePolicy` and the backup's legacy parameter `SnapshotVolumes`: 
  - The `VolumePolicy`'s `snapshot` action matching for volume has higher priority. When there is a `snapshot` action matching for the selected volume, it will be backed by the snapshot way, no matter of the `backup.Spec.SnapshotVolumes` setting.
  - If there is no `snapshot` action matching the selected volume in the `VolumePolicy`, then the volume will be backed up by `snapshot` way, if the `backup.Spec.SnapshotVolumes` is not set to false.
- The relation between the `VolumePolicy` and the backup's legacy filesystem `opt-in/opt-out` approach:
  - The `VolumePolicy`'s `fs-backup` action matching for volume has higher priority. When there is a `fs-backup` action matching for the selected volume, it will be backed by the fs-backup way, no matter of the `backup.Spec.DefaultVolumesToFsBackup` setting and the pod's `opt-in/opt-out` annotation setting.
  - If there is no `fs-backup` action matching the selected volume in the `VolumePolicy`, then the volume will be backed up by the legacy `opt-in/opt-out` way.

## Implementation

- The implementation should be included in velero 1.14

- We will introduce a `VolumeHelper` interface. It will consist of two methods:
```go
type VolumeHelper interface {
	ShouldPerformSnapshot(obj runtime.Unstructured, groupResource schema.GroupResource) (bool, error)
	ShouldPerformFSBackup(volume corev1api.Volume, pod corev1api.Pod) (bool, error)
}
```
-  The `VolumeHelperImpl` struct will implement the `VolumeHelper` interface and will consist of the functions that we will use through the backup workflow to accommodate volume policies for PVs and PVCs.
```go
type volumeHelperImpl struct {
	volumePolicy             *resourcepolicies.Policies
	snapshotVolumes          *bool
	logger                   logrus.FieldLogger
	client                   crclient.Client
	defaultVolumesToFSBackup bool
	backupExcludePVC         bool
}

```

- We will create an instance of the structure `volumeHelperImpl` in `item_backupper.go`
```go
	itemBackupper := &itemBackupper{
		...
		volumeHelperImpl: volumehelper.NewVolumeHelperImpl(
			resourcePolicy,
			backupRequest.Spec.SnapshotVolumes,
			log,
			kb.kbClient,
			boolptr.IsSetToTrue(backupRequest.Spec.DefaultVolumesToFsBackup),
			!backupRequest.ResourceIncludesExcludes.ShouldInclude(kuberesource.PersistentVolumeClaims.String()),
		),
	}
```


#### FS-Backup
- Regarding `fs-backup` action to decide whether to use legacy annotation based approach or volume policy based approach:
  - We will use the `vh.ShouldPerformFSBackup()` function from the `volumehelper` package
  - Functions involved in processing `fs-backup` volume policy action will somewhat look like:

```go
func (v volumeHelperImpl) ShouldPerformFSBackup(volume corev1api.Volume, pod corev1api.Pod) (bool, error) {
	if !v.shouldIncludeVolumeInBackup(volume) {
		v.logger.Debugf("skip fs-backup action for pod %s's volume %s, due to not pass volume check.", pod.Namespace+"/"+pod.Name, volume.Name)
		return false, nil
	}

	if v.volumePolicy != nil {
		pvc, err := kubeutil.GetPVCForPodVolume(&volume, &pod, v.client)
		if err != nil {
			v.logger.WithError(err).Errorf("fail to get PVC for pod %s", pod.Namespace+"/"+pod.Name)
			return false, err
		}
		pv, err := kubeutil.GetPVForPVC(pvc, v.client)
		if err != nil {
			v.logger.WithError(err).Errorf("fail to get PV for PVC %s", pvc.Namespace+"/"+pvc.Name)
			return false, err
		}

		action, err := v.volumePolicy.GetMatchAction(pv)
		if err != nil {
			v.logger.WithError(err).Errorf("fail to get VolumePolicy match action for PV %s", pv.Name)
			return false, err
		}

		if action != nil {
			if action.Type == resourcepolicies.FSBackup {
				v.logger.Infof("Perform fs-backup action for volume %s of pod %s due to volume policy match",
					volume.Name, pod.Namespace+"/"+pod.Name)
				return true, nil
			} else {
				v.logger.Infof("Skip fs-backup action for volume %s for pod %s because the action type is %s",
					volume.Name, pod.Namespace+"/"+pod.Name, action.Type)
				return false, nil
			}
		}
	}

	if v.shouldPerformFSBackupLegacy(volume, pod) {
		v.logger.Infof("Perform fs-backup action for volume %s of pod %s due to opt-in/out way",
			volume.Name, pod.Namespace+"/"+pod.Name)
		return true, nil
	} else {
		v.logger.Infof("Skip fs-backup action for volume %s of pod %s due to opt-in/out way",
			volume.Name, pod.Namespace+"/"+pod.Name)
		return false, nil
	}
}
```

- The main function from the above will be called when we encounter Pods during the backup workflow:
```go
			for _, volume := range pod.Spec.Volumes {
				shouldDoFSBackup, err := ib.volumeHelperImpl.ShouldPerformFSBackup(volume, *pod)
				if err != nil {
					backupErrs = append(backupErrs, errors.WithStack(err))
				}
				...
			}
```

#### Snapshot (PV)

- Making sure that `snapshot` action is skipped for PVs that do not fit the volume policy criteria, for this we will use the `vh.ShouldPerformSnapshot` from the `VolumeHelperImpl(vh)` receiver.
```go
func (v *volumeHelperImpl) ShouldPerformSnapshot(obj runtime.Unstructured, groupResource schema.GroupResource) (bool, error) {
	// check if volume policy exists and also check if the object(pv/pvc) fits a volume policy criteria and see if the associated action is snapshot
	// if it is not snapshot then skip the code path for snapshotting the PV/PVC
	pvc := new(corev1api.PersistentVolumeClaim)
	pv := new(corev1api.PersistentVolume)
	var err error

	if groupResource == kuberesource.PersistentVolumeClaims {
		if err = runtime.DefaultUnstructuredConverter.FromUnstructured(obj.UnstructuredContent(), &pvc); err != nil {
			return false, err
		}

		pv, err = kubeutil.GetPVForPVC(pvc, v.client)
		if err != nil {
			return false, err
		}
	}

	if groupResource == kuberesource.PersistentVolumes {
		if err = runtime.DefaultUnstructuredConverter.FromUnstructured(obj.UnstructuredContent(), &pv); err != nil {
			return false, err
		}
	}

	if v.volumePolicy != nil {
		action, err := v.volumePolicy.GetMatchAction(pv)
		if err != nil {
			return false, err
		}

		// If there is a match action, and the action type is snapshot, return true,
		// or the action type is not snapshot, then return false.
		// If there is no match action, go on to the next check.
		if action != nil {
			if action.Type == resourcepolicies.Snapshot {
				v.logger.Infof(fmt.Sprintf("performing snapshot action for pv %s", pv.Name))
				return true, nil
			} else {
				v.logger.Infof("Skip snapshot action for pv %s as the action type is %s", pv.Name, action.Type)
				return false, nil
			}
		}
	}

		// If this PV is claimed, see if we've already taken a (pod volume backup)
	// snapshot of the contents of this PV. If so, don't take a snapshot.
	if pv.Spec.ClaimRef != nil {
		pods, err := podvolumeutil.GetPodsUsingPVC(
			pv.Spec.ClaimRef.Namespace,
			pv.Spec.ClaimRef.Name,
			v.client,
		)
		if err != nil {
			v.logger.WithError(err).Errorf("fail to get pod for PV %s", pv.Name)
			return false, err
		}

		for _, pod := range pods {
			for _, vol := range pod.Spec.Volumes {
				if vol.PersistentVolumeClaim != nil &&
					vol.PersistentVolumeClaim.ClaimName == pv.Spec.ClaimRef.Name &&
					v.shouldPerformFSBackupLegacy(vol, pod) {
					v.logger.Infof("Skipping snapshot of pv %s because it is backed up with PodVolumeBackup.", pv.Name)
					return false, nil
				}
			}
		}
	}

	if !boolptr.IsSetToFalse(v.snapshotVolumes) {
		// If the backup.Spec.SnapshotVolumes is not set, or set to true, then should take the snapshot.
		v.logger.Infof("performing snapshot action for pv %s as the snapshotVolumes is not set to false", pv.Name)
		return true, nil
	}

	v.logger.Infof(fmt.Sprintf("skipping snapshot action for pv %s possibly due to no volume policy setting or snapshotVolumes is false", pv.Name))
	return false, nil
}
```

- The function `ShouldPerformSnapshot` will be used as follows in `takePVSnapshot` function of the backup workflow:
```go
	snapshotVolume, err := ib.volumeHelperImpl.ShouldPerformSnapshot(obj, kuberesource.PersistentVolumes)
	if err != nil {
		return err
	}

	if !snapshotVolume {
		log.Info(fmt.Sprintf("skipping volume snapshot for PV %s as it does not fit the volume policy criteria specified by the user for snapshot action", pv.Name))
		ib.trackSkippedPV(obj, kuberesource.PersistentVolumes, volumeSnapshotApproach, "does not satisfy the criteria for volume policy based snapshot action", log)
		return nil
	}
```

#### Snapshot (PVC)

- Making sure that `snapshot` action is skipped for PVCs that do not fit the volume policy criteria, for this we will again use the `vh.ShouldPerformSnapshot` from the `VolumeHelperImpl(vh)` receiver.
- We will pass the `VolumeHelperImpl(vh)` instance in `executeActions` method so that it is available to use.
```go

```
- The above function will be used as follows in the `executeActions` function of backup workflow.
- Considering the vSphere plugin doesn't support the VolumePolicy yet, don't use the VolumePolicy for vSphere plugin by now.
```go
		if groupResource == kuberesource.PersistentVolumeClaims {
			if actionName == csiBIAPluginName {
				snapshotVolume, err := ib.volumeHelperImpl.ShouldPerformSnapshot(obj, kuberesource.PersistentVolumeClaims)
				if err != nil {
					return nil, itemFiles, errors.WithStack(err)
				}

				if !snapshotVolume {
					log.Info(fmt.Sprintf("skipping csi volume snapshot for PVC %s as it does not fit the volume policy criteria specified by the user for snapshot action", namespace+"/"+name))
					ib.trackSkippedPV(obj, kuberesource.PersistentVolumeClaims, volumeSnapshotApproach, "does not satisfy the criteria for volume policy based snapshot action", log)
					continue
				}
			}
		}
```


## Future Implementation
It makes sense to add more specific actions in the future, once we deprecate the legacy opt-in/opt-out approach to keep things simple. Another point of note is, csi related action can be
easier to implement once we decide to merge the csi plugin into main velero code flow.
In the future, we envision the following actions that can be implemented:
- `snapshot-native`: only use volume snapshotter (native cloud provider snapshots), do nothing if not present/not compatible
- `snapshot-csi`: only use csi-plugin, don't use volume snapshotter(native cloud provider snapshots), don't use datamover even if snapshotMoveData is true
- `snapshot-datamover`: only use csi with datamover, don't use volume snapshotter (native cloud provider snapshots), use datamover even if snapshotMoveData is false

**Note:** The above actions are just suggestions for future scope, we may not use/implement them as is. We could definitely merge these suggested actions as `Snapshot` actions and use volume policy parameters and criteria to segregate them instead of making the user explicitly supply the action names to such granular levels.

## Related to Design

[Handle backup of volumes by resources filters](https://github.com/vmware-tanzu/velero/blob/main/design/Implemented/handle-backup-of-volumes-by-resources-filters.md)

## Alternatives Considered
Same as the earlier design as this is an extension of the original VolumePolicies design