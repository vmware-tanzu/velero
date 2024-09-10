# Node-agent Load Affinity Design

## Glossary & Abbreviation

**Velero Generic Data Path (VGDP)**: VGDP is the collective modules that is introduced in [Unified Repository design][1]. Velero uses these modules to finish data transfer for various purposes (i.e., PodVolume backup/restore, Volume Snapshot Data Movement). VGDP modules include uploaders and the backup repository.  

**Exposer**: Exposer is a module that is introduced in [Volume Snapshot Data Movement Design][2]. Velero uses this module to expose the volume snapshots to Velero node-agent pods or node-agent associated pods so as to complete the data movement from the snapshots.    

## Background

Velero node-agent is a daemonset hosting controllers and VGDP modules to complete the concrete work of backups/restores, i.e., PodVolume backup/restore, Volume Snapshot Data Movement backup/restore.  
Specifically, node-agent runs DataUpload controllers to watch DataUpload CRs for Volume Snapshot Data Movement backups, so there is one controller instance in each node. One controller instance takes a DataUpload CR and then launches a VGDP instance, which initializes a uploader instance and the backup repository connection, to finish the data transfer. The VGDP instance runs inside a node-agent pod or in a pod associated to the node-agent pod in the same node.  

Varying from the data size, data complexity, resource availability, VGDP may take a long time and remarkable resources (CPU, memory, network bandwidth, etc.).  
Technically, VGDP instances are able to run in any node that allows pod schedule. On the other hand, users may want to constrain the nodes where VGDP instances run for various reasons, below are some examples:  
- Prevent VGDP instances from running in specific nodes because users have more critical workloads in the nodes  
- Constrain VGDP instances to run in specific nodes because these nodes have more resources than others  
- Constrain VGDP instances to run in specific nodes because the storage allows volume/snapshot provisions in these nodes only  

Therefore, in order to improve the compatibility, it is worthy to configure the affinity of VGDP to nodes, especially for backups for which VGDP instances run frequently and centrally.  

## Goals

- Define the behaviors of node affinity of VGDP instances in node-agent for volume snapshot data movement backups  
- Create a mechanism for users to specify the node affinity of VGDP instances for volume snapshot data movement backups  

## Non-Goals
- It is also beneficial to support VGDP instances affinity for PodVolume backup/restore, however, it is not possible since VGDP instances for PodVolume backup/restore should always run in the node where the source/target pods are created.  
- It is also beneficial to support VGDP instances affinity for data movement restores, however, it is not possible in some cases. For example, when the `volumeBindingMode` in the StorageClass is `WaitForFirstConsumer`, the restore volume must be mounted in the node where the target pod is scheduled, so the VGDP instance must run in the same node. On the other hand, considering the fact that restores may not frequently and centrally run, we will not support data movement restores.  
- As elaborated in the [Volume Snapshot Data Movement Design][2], the Exposer may take different ways to expose snapshots, i.e., through backup pods (this is the only way supported at present). The implementation section below only considers this approach currently, if a new expose method is introduced in future, the definition of the affinity configurations and behaviors should still work, but we may need a new implementation.  

## Solution

We will use the ConfigMap specified by `velero node-agent` CLI's parameter `--node-agent-configmap` to host the node affinity configurations.
This configMap is not created by Velero, users should create it manually on demand. The configMap should be in the same namespace where Velero is installed. If multiple Velero instances are installed in different namespaces, there should be one configMap in each namespace which applies to node-agent in that namespace only.  
Node-agent server checks these configurations at startup time and use it to initiate the related VGDP modules. Therefore, users could edit this configMap any time, but in order to make the changes effective, node-agent server needs to be restarted.  
Inside the ConfigMap we will add one new kind of configuration as the data in the configMap, the name is ```loadAffinity```.  
Users may want to set different LoadAffinity configurations according to different conditions (i.e., for different storages represented by StorageClass, CSI driver, etc.), so we define ```loadAffinity``` as an array. This is for extensibility consideration, at present, we don't implement multiple configurations support, so if there are multiple configurations, we always take the first one in the array.  

The data structure is as below:
```go
type Configs struct {
	// LoadConcurrency is the config for load concurrency per node.
	LoadConcurrency *LoadConcurrency `json:"loadConcurrency,omitempty"`

	// LoadAffinity is the config for data path load affinity.
	LoadAffinity []*LoadAffinity `json:"loadAffinity,omitempty"`    
}

type LoadAffinity struct {
    // NodeSelector specifies the label selector to match nodes
    NodeSelector metav1.LabelSelector `json:"nodeSelector"`
}
```

### Affinity
Affinity configuration means allowing VGDP instances running in the nodes specified. There are two ways to define it:
-  It could be defined by `MatchLabels` of `metav1.LabelSelector`. The labels defined in `MatchLabels` means a `LabelSelectorOpIn` operation by default, so in the current context, they will be treated as affinity rules.  
- It could be defined by `MatchExpressions` of `metav1.LabelSelector`. The labels are defined in `Key` and `Values` of `MatchExpressions` and the `Operator` should be defined as `LabelSelectorOpIn` or `LabelSelectorOpExists`. 

### Anti-affinity
Anti-affinity configuration means preventing VGDP instances running in the nodes specified. Below is the way to define it:  
- It could be defined by `MatchExpressions` of `metav1.LabelSelector`. The labels are defined in `Key` and `Values` of `MatchExpressions` and the `Operator` should be defined as `LabelSelectorOpNotIn` or `LabelSelectorOpDoesNotExist`.   

### Sample
A sample of the ConfigMap is as below:
```json
{
    "loadAffinity": [
        {
            "nodeSelector": {
                "matchLabels": {
                    "beta.kubernetes.io/instance-type": "Standard_B4ms"
                },
                "matchExpressions": [
                    {
                        "key": "kubernetes.io/hostname",
                        "values": [
                            "node-1",
                            "node-2",
                            "node-3"
                        ],
                        "operator": "In"
                    },
                    {
                        "key": "xxx/critial-workload",
                        "operator": "DoesNotExist"
                    }
                ]          
            }
        }
    ]
}
```
This sample showcases two affinity configurations:
- matchLabels: VGDP instances will run only in nodes with label key `beta.kubernetes.io/instance-type` and value `Standard_B4ms`
- matchExpressions: VGDP instances will run in node `node1`, `node2` and `node3` (selected by `kubernetes.io/hostname` label)

This sample showcases one anti-affinity configuration:
- matchExpressions: VGDP instances will not run in nodes with label key `xxx/critial-workload`

To create the configMap, users need to save something like the above sample to a json file and then run below command:
```
kubectl create cm <ConfigMap name> -n velero --from-file=<json file name>
``` 

### Implementation
As mentioned in the [Volume Snapshot Data Movement Design][2], the exposer decides where to launch the VGDP instances. At present, for volume snapshot data movement backups, the exposer creates backupPods and the VGDP instances will be initiated in the nodes where backupPods are scheduled. So the loadAffinity will be translated (from `metav1.LabelSelector` to `corev1.Affinity`) and set to the backupPods.  

It is possible that node-agent pods, as a daemonset, don't run in every worker nodes, users could fulfil this by specify `nodeSelector` or `nodeAffinity` to the node-agent daemonset spec. On the other hand, at present, VGDP instances must be assigned to nodes where node-agent pods are running. Therefore, if there is any node selection for node-agent pods, users must consider this into this load affinity configuration, so as to guarantee that VGDP instances are always assigned to nodes where node-agent pods are available. This is done by users, we don't inherit any node selection configuration from node-agent daemonset as we think daemonset scheduler works differently from plain pod scheduler, simply inheriting all the configurations may cause unexpected result of backupPod schedule.    
Otherwise, if a backupPod are scheduled to a node where node-agent pod is absent, the corresponding DataUpload CR will stay in `Accepted` phase until the prepare timeout (by default 30min).  

At present, as part of the expose operations, the exposer creates a volume, represented by backupPVC, from the snapshot. The backupPVC uses the same storageClass with the source volume. If the `volumeBindingMode` in the storageClass is `Immediate`, the volume is immediately allocated from the underlying storage without waiting for the backupPod. On the other hand, the loadAffinity is set to the backupPod's affinity. If the backupPod is scheduled to a node where the snapshot volume is not accessible, e.g., because of storage topologies, the backupPod won't get into Running state, concequently, the data movement won't complete.  
Once this problem happens, the backupPod stays in `Pending` phase, and the corresponding DataUpload CR stays in `Accepted` phase until the prepare timeout (by default 30min). Below is an example of the backupPod's status when the problem happens:   
```
  status:
    conditions:
    - lastProbeTime: null
      message: '0/2 nodes are available: 1 node(s) didn''t match Pod''s node affinity/selector,
        1 node(s) had volume node affinity conflict. preemption: 0/2 nodes are available:
        2 Preemption is not helpful for scheduling..'
      reason: Unschedulable
      status: "False"
      type: PodScheduled
    phase: Pending
```    

On the other hand, the backupPod is deleted after the prepare timeout, so there is no way to tell the cause is one of the above problems or others.  
To help the troubleshooting, we can add some diagnostic mechanism to discover the status of the backupPod and node-agent in the same node before deleting it as a result of the prepare timeout.  

[1]: Implemented/unified-repo-and-kopia-integration/unified-repo-and-kopia-integration.md
[2]: volume-snapshot-data-movement/volume-snapshot-data-movement.md