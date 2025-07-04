# Node-agent Load Soothing Design

## Glossary & Abbreviation

**Velero Generic Data Path (VGDP)**: VGDP is the collective of modules that is introduced in [Unified Repository design][1]. Velero uses these modules to finish data transfer for various purposes (i.e., PodVolume backup/restore, Volume Snapshot Data Movement). VGDP modules include uploaders and the backup repository.  

## Background

As mentioned in [node-agent Concurrency design][2], [CSI Snapshot Data Movement design][3], [VGDP Micro Service design][4] and [VGDP Micro Service for fs-backup design][5], all data movement activities for CSI snapshot data movement backups/restores and fs-backup respect the `loadConcurrency` settings configured in the `node-agent-configmap`. Once the number of existing loads exceeds the corresponding `loadConcurrency` setting, the loads will be throttled and some loads will be held until VGDP quotas are available.  
However, this throttling only happens after the data mover pod is started and gets to `running`. As a result, when there are large number of concurrent volume backups, there may be many data mover pods get created but the VGDP instances inside them are actually on hold because of the VGDP throttling.  
This could cause below problems:
- In some environments, there is a pod limit in each node of the cluster or a pod limit throughout the cluster, too many of the inactive data mover pods may block other pods from running
- In some environments, the system disk for each node of the cluster is limited, while pods also occupy system disk space, etc., many of the inactive data mover pods also take unnecessary space from system disk and cause other critical pods evicted
- For CSI snapshot data movement backup, before creation of the data mover pod, the volume snapshot has also created, this means excessive number of snapshots may also be created and live for longer time since the VGDP won't start until the quota is available. However, in some environments, large number of snapshots is not allowed or may cause degradation of the storage peroformance

On the other hand, the VGDP throttling mentioned in [node-agent Concurrency design][2] is an accurate controlling mechanism, that is, exactly the required number of data mover pods are throttled.  

Therefore, another mechanism is required to soothe the creation of the data mover pods and volume snapshots before the VGDP throttling. It doesn't need to accurately control these creations but should effectively reduce the exccessive number of inactive data mover pods and volume snapshots.  
It is not practical to make an accurate control as it is almost impossible to predict which group of nodes a data mover pod is scheduled to, under the consideration of many complex factors, i.e., selected node, affinity, node OS, etc.  


## Goals

- Allow users to configure the expected number of loads pending on waiting for VGDP load concurrency quota
- Create a soothing mechanism to prevent new loads from starting if the number of existing loads excceds the expected number

## Non-Goals
- Accurately controlling the loads from initiation is not a goal  

## Solution

We introduce a new field `waitQueueLength` in `loadConcurrency` of `node-agent-configmap` for the expected number. Once the value is set, the soothing mechanism takes effect, otherwise, node-agent works the same as the legacy behavior, no soothing will happen.    
Node-agent server checks this configuration at startup time and use it to initiate the related VGDP modules. Therefore, users could edit this configMap any time, but in order to make the changes effective, node-agent server needs to be restarted.  

The data structure is as below:
```go
type LoadConcurrency struct {
    // GlobalConfig specifies the concurrency number to all nodes for which per-node config is not specified
    GlobalConfig int `json:"globalConfig,omitempty"`

    // PerNodeConfig specifies the concurrency number to nodes matched by rules
    PerNodeConfig []RuledConfigs `json:"perNodeConfig,omitempty"`

    // WaitQueueLength specifies the max number of loads that are waiting for process
	WaitQueueLength int `json:"waitQueueLength,omitempty"`    
}
```

### Sample
A sample of the ConfigMap is as below:
```json
{
    "loadConcurrency": {
        "globalConfig": 2,
        "perNodeConfig": [
            {
                "nodeSelector": {
                    "matchLabels": {
                        "kubernetes.io/hostname": "node1"
                    }
                },
                "number": 3
            },
            {
                "nodeSelector": {
                    "matchLabels": {
                        "beta.kubernetes.io/instance-type": "Standard_B4ms"
                    }
                },
                "number": 5
            }
        ],
        "waitQueueLength": 2
    }
}
```
To create the configMap, users need to save something like the above sample to a json file and then run below command:
```
kubectl create cm <ConfigMap name> -n velero --from-file=<json file name>
```

## Detailed Design
Changes apply to the DataUpload Controller, DataDownload Controller, PodVolumeBackup Controller and PodVolumeRestore Controller, as below:
- The soothe happens to data mover CRs (DataUpload, DataDownload, PodVolumeBackup or PodVolumeRestore) that are in `New` state
- Before starting processing the CR, the corresponding controller counts the existing CRs in the cluster, that is a total number of existing DataUpload, DataDownload, PodVolumeBackup and PodVolumeRestore
- Once the total number exceeds the allowed number, the controller gives up processing the CR and have it requeued later
- The delay for the requrue is 5 seconds

The count happens for all the controllers in all nodes, to prevent the checks drain out the API server, the count happens to controller client caches for those CRs. And the count result is also cached, so that the count only happens whenever necessary. Below shows how it judges the necessity:
- When one or more CRs' phase change to `Accepted`
- When one or more CRs' phase change from `Accepted` to one of the terminal phases
- When one or more CRs' phase change from `Prepared` to one of the terminal phases
- When one or more CRs' phase change from `Prepared` to `InProgress`

The count requests are not synchronized among controllers. It is too expensive to make this synchronization, because it requires to synchronize part of the expose process among controllers, while the expose process is complex and takes time, we don't want to compromize the performance or stability of the system.  
Therefore, it is possible that more loads than the number of `waitQueueLength` are discharged if controllers make the count and expose in the overlaped time.  
For the normal case that only one type of loads are running (a data mover backup, a data mover restore, a pod volume backup or a pod volume restore), the limit is: 
```
max number of waiting loads = number defined by `waitQueueLength` + number of nodes in cluster
```
When a hybrid of loads are running, e.g., mix of data mover backups, data mover restores, pod volume backups or pod volume restores, more loads may be discharged.  
In either case, the soothe should still work -- the next time when the controllers see all the discharged loads (expected ones and over-discharged ones), they will stop creating new loads until the quota is available.  
Meanwhile, the count and part of the expose process that need to be synchronized could complete very quickly, so it won't reach to the worst result in theory.    





[1]: Implemented/unified-repo-and-kopia-integration/unified-repo-and-kopia-integration.md
[2]: Implemented/node-agent-concurrency.md
[3]: Implemented/volume-snapshot-data-movement/volume-snapshot-data-movement.md
[4]: Implemented/vgdp-micro-service/vgdp-micro-service.md
[5]: vgdp-micro-service-for-fs-backup/vgdp-micro-service-for-fs-backup.md