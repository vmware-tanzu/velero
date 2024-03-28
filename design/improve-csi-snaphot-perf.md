# Proposal to improve performance of CSI snapshotting through velero.

- [Proposal to improve performance of CSI snapshotting through velero.]

## Abstract
Currently velero uses the CSI plugin for taking CSI snapshots. The CSI Plugin is modeled as a BIA, where whenever the velero code encounters a PVC, it invokes the PVCAction BIA of CSI Plugin. In the Execute() phase of the plugin the CSI plugin waits for a default 10mins for snapshotting to complete. This is a blocking call and the velero code waits for the snapshotting to complete before proceeding to the next resource. In case of failures due to permissions etc, velero will keep waiting for 10*N minutes. This tracking cannot be made async since we need to ensure the appreance of snapshotHandle on the VolumeSnapshotContent before proceeding. This is because the pre-hooks run on pods first, then PVCs are snapshotted, and then posthooks. Ensuring waiting for actual snapshotting is key here to ensure that the posthooks are not executed before the snapshotting is complete.
Further the Core velero code waits on the CSI Snapshot to have readyToUse=true for 10 minutes.

<!-- ## Background -->

## Goals
- Reduce the time to take CSI snapshots.
- Ensure current behaviour of pre and post hooks in context of PVCs attached to a pod.

## Non Goals

## Considerations: 
- Ensure no existing flows break.
- Ensure thread safety in case of concurrency.
- Provide control over max concurrency tweakable by end user. 

## How to Group Snapshots 
- PVCs which are being used by a pod/deployment should be snapshotting together.
- PVCs which are not being used by any pod can be snapshotted at any time.
- If there are no hooks provided, snapshotting for all PVCs can be done in parallel.

## How to group Resources:
- This is an additional thinking of how to group resources efficiently in velero so that they can be backed up more efficiently - potentially in parallel.
- Current issues with PVs being backed up through different flows will be potentially solved through some in discussion items. 

## Approaches

### Invoke CSI Plugin in parallel for a group of PVCs.
- Invoke current CSI plugin's Execute() in parallel for a group of PVCs.

## Implementation
- Current code flow `backupItem` in backup.go is invoked for each resource -> this further invokes `itembackupper.backupItem` -> `backupItemInternal`
- Now for a Pod -> First Pre Hooks are run -> Then `executeActions` -> iterate over all BIA applicable on Pod -> which will invoke the `PodAction`
- After all actions are run, `executeActions` gets the additionalItems to backup(PVCs)
- For all these PVCs  and other additional items we iterate and call `itembackupper.backupItem`.
- After all additional items are backed up -> control returns to `backupItemInternal` -> Post Hooks are run -> and then `backupItem` returns.
- Here the change we will do is that when backup for additionalItems is done, for PVCs, we will run `itembackupper.backupItem` in an async way.

## Open Problems
- Ensuring thread safety
- Ensuring that PVBs triggered in parallel work as expected.

## Alternatives Considered
### Approach 1: Add support for VolumeGroupSnapshot in Velero. 
- (Volume Group Snapshots)[https://kubernetes.io/blog/2023/05/08/kubernetes-1-27-volume-group-snapshot-alpha/] is introduced as an Alpha feature in Kubernetes v1.27. This feature introduces a Kubernetes API that allows users to take crash consistent snapshots for multiple volumes together. It uses a **label selector to group multiple PersistentVolumeClaims** for snapshotting
- This is out of scope for current design since the API is not even Beta yet and not impacting current perf improvements.

## Approach 2: Create a Pod BIA Plugin which will invoke CSI Plugin in parallel for a group of PVCs.
- Create a Pod BIA Plugin which will invoke CSI Plugin in parallel for a group of PVCs.
- This would lead to code and logic duplication across CSI Plugin and the pod plugin.
- With BIAv2 it is complicated to achieve since a single pod plugin would have to return N operation IDs for N PVCs, while there is only support for 1 operation id at a time. Hacking around using 1 operation id field would lead to code complications and would not be a clean approach.

## Security Considerations
No security impact.

## Compatibility



## Future enhancement

## Open Issues
NA
