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
- Pass a list of PVCs to CSI Plugin for Approach 1?

## How to Group Snapshots 
- PVCs which are being used by a pod/deployment should be snapshotting together.
- PVCs which are not being used by any pod can be snapshotted at any time.
- If there are no hooks provided, snapshotting for all PVCs can be done in parallel.

## Approaches

### Approach 1: Add support for VolumeGroupSnapshot in Velero.
- (Volume Group Snapshots)[https://kubernetes.io/blog/2023/05/08/kubernetes-1-27-volume-group-snapshot-alpha/] is introduced as an Alpha feature in Kubernetes v1.27. This feature introduces a Kubernetes API that allows users to take crash consistent snapshots for multiple volumes together. It uses a **label selector to group multiple PersistentVolumeClaims** for snapshotting

### Approach 2: Invoke CSI Plugin in parallel for a group of PVCs.
- Invoke current CSI plugin's Execute() in parallel for a group of PVCs.
- Perf implications of parallel calls?

## Approach 3: Create a Pod BIA Plugin which will invoke CSI Plugin in parallel for a group of PVCs.
- Create a Pod BIA Plugin which will invoke CSI Plugin in parallel for a group of PVCs.
- This would lead to code and logic duplication across CSI Plugin and the pod plugin.

## Key challenge to solve in all above approaches.
- How to group PVCs in the core velero flow and further send them for processing.

## Detailed Design
-
## Alternatives Considered

## Security Considerations
No security impact.

## Compatibility

## Implementation


## Future enhancement

## Open Issues
NA
