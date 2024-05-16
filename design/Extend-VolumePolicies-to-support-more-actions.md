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
  - If there is a presence of volume policy(fs-backup/snapshot)  on the backup request that matches as an action for a volume we use the newer volume policy approach to get the list of the volumes for `fs-backup` action
  - Else continue with the annotation based legacy approach workflow.

- Modifications needed for `snapshot` action:
  - In the [takePVSnapshot function](https://github.com/vmware-tanzu/velero/blob/d4128542590470b204a642ee43311921c11db880/pkg/backup/item_backupper.go#L508) we will check the PV fits the volume policy criteria and see if the associated action is `snapshot`
  - If it is not snapshot then we skip the further workflow and avoid taking the snapshot of the PV
  - Similarly, For csi snapshot of PVC object, we need to do similar changes in [executeAction() function](https://github.com/vmware-tanzu/velero/blob/512fe0dabdcb3bbf1ca68a9089056ae549663bcf/pkg/backup/item_backupper.go#L348). we will check the PVC fits the volume policy criteria and see if the associated action is `snapshot` via csi plugin
  - If it is not snapshot then we skip the csi BIA execute action and avoid taking the snapshot of the PVC by not invoking the csi plugin action for the PVC

**Note:**
- When we are using the `VolumePolicy` approach for backing up the volumes then the volume policy criteria and action need to be specific and explicit, there is no default behaviour, if a volume matches `fs-backup` action then `fs-backup` method will be used for that volume and similarly if the volume matches the criteria for `snapshot` action then the snapshot workflow will be used for the volume backup.
- Another thing to note is the workflow proposed in this design uses the legacy opt-in/opt-out approach as a fallback option. For instance, the user specifies a VolumePolicy but for a particular volume included in the backup there are no actions(fs-backup/snapshot) matching in the volume policy for that volume, in such a scenario the legacy approach will be used for backing up the particular volume.
## Implementation

- The implementation should be included in velero 1.14

- We will introduce a `VolumeHelper` interface. It will consist of two methods:
  - `ShouldPerformFSBackupForPodVolume(pod *corev1api.Pod)`
  - `ShouldPerformSnapshot(obj runtime.Unstructured)`
```go
type VolumeHelper interface {
	GetVolumesForFSBackup(pod *corev1api.Pod) ([]string, []string, error)
	ShouldPerformSnapshot(obj runtime.Unstructured) (bool, error)
}
```
-  The `VolumeHelperImpl` struct will implement the `VolumeHelper` interface and will consist of the functions that we will use through the backup workflow to accommodate volume policies for PVs and PVCs.
```go
type VolumeHelperImpl struct {
	Backup                   *velerov1api.Backup
	VolumePolicy             *resourcepolicies.Policies
	BackupExcludePVC         bool
	DefaultVolumesToFsBackup bool
	SnapshotVolumes          *bool
	Logger                   logrus.FieldLogger
}

```

- We will create an instance of the struct the `VolumeHelperImpl` in `item_backupper.go`
```go
	vh := &volumehelper.VolumeHelperImpl{
		Backup:                   ib.backupRequest.Backup,
		VolumePolicy:             ib.backupRequest.ResPolicies,
		BackupExcludePVC:         !ib.backupRequest.ResourceIncludesExcludes.ShouldInclude(kuberesource.PersistentVolumeClaims.String()),
		DefaultVolumesToFsBackup: boolptr.IsSetToTrue(ib.backupRequest.Spec.DefaultVolumesToFsBackup),
		SnapshotVolumes:          ib.backupRequest.Spec.SnapshotVolumes,
		Logger:                   logger,
	}
```


#### FS-Backup
- Regarding `fs-backup` action to decide whether to use legacy annotation based approach or volume policy based approach:
  - We will use the `vh.GetVolumesForFSBackup()` function from the `volumehelper` package
  - Functions involved in processing `fs-backup` volume policy action will somewhat look like:

```go
func (v *VolumeHelperImpl) GetVolumesForFSBackup(pod *corev1api.Pod) ([]string, []string, error) {
	// Check if there is a fs-backup/snapshot volumepolicy specified by the user, if yes then use the volume policy approach to
	// get the list volumes for fs-backup else go via the legacy annotation based approach

	var includedVolumes = make([]string, 0)
	var optedOutVolumes = make([]string, 0)

	FSBackupOrSnapshot, err := checkIfFsBackupORSnapshotPolicyForPodVolume(pod, v.VolumePolicy)
	if err != nil {
		return includedVolumes, optedOutVolumes, err
	}

	if v.VolumePolicy != nil && FSBackupOrSnapshot {
		// Get the list of volumes to back up using pod volume backup for the given pod matching fs-backup volume policy action
		includedVolumes, optedOutVolumes, err = GetVolumesMatchingFSBackupAction(pod, v.VolumePolicy)
		if err != nil {
			return includedVolumes, optedOutVolumes, err
		}
	} else {
		// Get the list of volumes to back up using pod volume backup from the pod's annotations.
		includedVolumes, optedOutVolumes = pdvolumeutil.GetVolumesByPod(pod, v.DefaultVolumesToFsBackup, v.BackupExcludePVC)
	}
	return includedVolumes, optedOutVolumes, err
}

func checkIfFsBackupORSnapshotPolicyForPodVolume(pod *corev1api.Pod, volumePolicies *resourcepolicies.Policies) (bool, error) {

	for volume := range pod.Spec.Volumes {
		action, err := volumePolicies.GetMatchAction(volume)
		if err != nil {
			return false, err
		}
		if action.Type == resourcepolicies.FSBackup || action.Type == resourcepolicies.Snapshot {
			return true, nil
		}
	}
	return false, nil
}

// GetVolumesMatchingFSBackupAction returns a list of volume names to backup for the provided pod having fs-backup volume policy action
func GetVolumesMatchingFSBackupAction(pod *corev1api.Pod, volumePolicy *resourcepolicies.Policies) ([]string, []string, error) {
	ActionMatchingVols := []string{}
	NonActionMatchingVols := []string{}
	for _, vol := range pod.Spec.Volumes {
		action, err := volumePolicy.GetMatchAction(vol)
		if err != nil {
			return nil, nil, err
		}
		// Now if the matched action is `fs-backup` then add that Volume to the fsBackupVolumeList
		if action != nil && action.Type == resourcepolicies.FSBackup {
			ActionMatchingVols = append(ActionMatchingVols, vol.Name)
		} else {
			NonActionMatchingVols = append(NonActionMatchingVols, vol.Name)
		}
	}

	return ActionMatchingVols, NonActionMatchingVols, nil
}
```
- The main function from the above `vph.ProcessVolumePolicyFSbackup` will be called when we encounter Pods during the backup workflow:
```go
includedVolumes, optedOutVolumes, err := vh.GetVolumesForFSBackup(pod)
			if err != nil {
				backupErrs = append(backupErrs, errors.WithStack(err))
			}

```
#### Snapshot (PV)

- Making sure that `snapshot` action is skipped for PVs that do not fit the volume policy criteria, for this we will use the `vh.ShouldPerformSnapshot` from the `VolumeHelperImpl(vh)` receiver.
```go
func (v *VolumeHelperImpl) ShouldPerformSnapshot(obj runtime.Unstructured) (bool, error) {
	// check if volume policy exists and also check if the object(pv/pvc) fits a volume policy criteria and see if the associated action is snapshot
	// if it is not snapshot then skip the code path for snapshotting the PV/PVC
	if v.VolumePolicy != nil {
		action, err := v.VolumePolicy.GetMatchAction(obj)
		if err != nil {
			return false, err
		}

		// check if the action is not nil and the type is snapshot
		if action != nil && action.Type == resourcepolicies.Snapshot {
			return true, nil
		}
	}

	// now if volumepolicy is not specified then just check for snapshotVolumes flag
	if boolptr.IsSetToTrue(v.SnapshotVolumes) {
		return true, nil
	}

	return false, nil

}
```

- The above function will be used as follows in `takePVSnapshot` function of the backup workflow:
```go
snapshotVolume, err := vh.ShouldPerformSnapshot(obj)

	if err != nil {
		return err
	}

	if !snapshotVolume {
		log.Info(fmt.Sprintf("skipping volume snapshot for PV %s as it does not fit the volume policy criteria for snapshot action", pv.Name))
		ib.trackSkippedPV(obj, kuberesource.PersistentVolumes, volumeSnapshotApproach, "does not satisfy the criteria for volume policy based snapshot action", log)
		return nil
	}
```
#### Snapshot (PVC)

- Making sure that `snapshot` action is skipped for PVCs that do not fit the volume policy criteria, for this we will again use the `vh.ShouldPerformSnapshot` from the `VolumeHelperImpl(vh)` receiver.
- We will pass the `VolumeHelperImpl(vh)` instance in `executeActions` method so that it is available to use.
```go
func (v *VolumeHelperImpl) ShouldPerformSnapshot(obj runtime.Unstructured) (bool, error) {
	// check if volume policy exists and also check if the object(pv/pvc) fits a volume policy criteria and see if the associated action is snapshot
	// if it is not snapshot then skip the code path for snapshotting the PV/PVC
	if v.VolumePolicy != nil {
		action, err := v.VolumePolicy.GetMatchAction(obj)
		if err != nil {
			return false, err
		}

        // check if the action is not nil and the type is snapshot
		if action != nil && action.Type == resourcepolicies.Snapshot {
			return true, nil
		}
	}

	// now if volumepolicy is not specified then just check for snapshotVolumes flag
	if boolptr.IsSetToTrue(v.SnapshotVolumes) {
		return true, nil
	}

	return false, nil

}

```
- The above function will be used as follows in the `executeActions` function of backup workflow:
```go
		if groupResource == kuberesource.PersistentVolumeClaims && actionName == csiBIAPluginName {
			snapshotVolume, err := vh.ShouldPerformSnapshot(obj)
			if err != nil {
				return nil, itemFiles, errors.WithStack(err)
			}

			if !snapshotVolume {
				log.Info(fmt.Sprintf("skipping csi volume snapshot for PVC %s as it does not fit the volume policy criteria for snapshot action", namespace+" /"+name))
				ib.trackSkippedPV(obj, kuberesource.PersistentVolumeClaims, volumeSnapshotApproach, "does not satisfy the criteria for volume policy based snapshot action", log)
				continue
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