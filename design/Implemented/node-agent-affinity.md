# Velero Generic Data Path Load Affinity Design

## Glossary & Abbreviation

**Velero Generic Data Path (VGDP)**: VGDP is the collective modules that is introduced in [Unified Repository design][1]. Velero uses these modules to finish data transfer for various purposes (i.e., PodVolume backup/restore, Volume Snapshot Data Movement). VGDP modules include uploaders and the backup repository.  

**Exposer**: Exposer is a module that is introduced in [Volume Snapshot Data Movement Design][2]. Velero uses this module to expose the volume snapshots to Velero node-agent pods or node-agent associated pods so as to complete the data movement from the snapshots.

## Background

Velero node-agent is a DaemonSet hosting controllers and VGDP modules to complete the concrete work of backups/restores, i.e., PodVolume backup/restore, Volume Snapshot Data Movement backup/restore.  
Specifically, node-agent runs DataUpload and DataDownload controllers to watch DataUpload CRs for Volume Snapshot Data Movement backups and restores, so there is one controller instance in each node. One controller instance takes a DataUpload CR and then launches a VGDP instance, which initializes a uploader instance and the backup repository connection, to finish the data transfer. The VGDP instance runs inside a node-agent pod or in a pod associated to the node-agent pod in the same node.  

Varying from the data size, data complexity, resource availability, VGDP may take a long time and remarkable resources (CPU, memory, network bandwidth, etc.).  
Technically, VGDP instances are able to run in any node that allows pod schedule. On the other hand, users may want to constrain the nodes where VGDP instances run for various reasons, below are some examples:  
- Prevent VGDP instances from running in specific nodes because users have more critical workloads in the nodes  
- Constrain VGDP instances to run in specific nodes because these nodes have more resources than others  
- Constrain VGDP instances to run in specific nodes because the storage allows volume/snapshot provisions in these nodes only  

Therefore, in order to improve the compatibility, it is worthy to configure the affinity of VGDP to nodes.

## Goals

- Define the behaviors of node affinity of VGDP instances in node-agent for volume snapshot data movement backups
- Define the behaviors of node affinity of VGDP instances in node-agent for volume snapshot data movement restore, when the PVC restore doesn't require delay binding.
- Create a mechanism for users to specify the node affinity of VGDP instances for volume snapshot data movement

## Non-Goals
- It is also beneficial to support VGDP instances affinity for PodVolume backup/restore, however, it is not possible since VGDP instances for PodVolume backup/restore should always run in the node where the source/target pods are created.  
- It is also beneficial to support VGDP instances affinity for data movement restores, however, it is not possible in some cases. For example, when the `volumeBindingMode` in the StorageClass is `WaitForFirstConsumer`, the restore volume must be mounted in the node where the target pod is scheduled, so the VGDP instance must run in the same node. The VGDP affinity setting doesn't apply to this case.
- As elaborated in the [Volume Snapshot Data Movement Design][2], the Exposer may take different ways to expose snapshots, i.e., through VGDP pods (this is the only way supported at present). The implementation section below only considers this approach currently, if a new expose method is introduced in future, the definition of the affinity configurations and behaviors should still work, but we may need a new implementation.  

## Solution

We will use the ConfigMap specified by `velero node-agent` CLI's parameter `--node-agent-configmap` to host the node affinity configurations.
This ConfigMap is not created by Velero, users should create it manually on demand. The configMap should be in the same namespace where Velero is installed. If multiple Velero instances are installed in different namespaces, there should be one configMap in each namespace which applies to node-agent in that namespace only.  
Node-agent server checks these configurations at startup time and use it to initiate the related VGDP modules. Therefore, users could edit this ConfigMap any time, but in order to make the changes effective, node-agent server needs to be restarted.  
Inside the ConfigMap we will add two new kinds of configuration as the data in the ConfigMap, the name is `loadAffinity` and `loadAffinityPerStorageClass`.  
Users may want to set different LoadAffinity configurations according to different conditions (i.e., for different storages represented by StorageClass, CSI driver, etc.).
* `loadAffinity` is effective to all VGDP instances, but `loadAffinity`'s priority is lower than `loadAffinityPerStorageClass`.
* `loadAffinityPerStorageClass` is only effective the VGDP, which mounting volume matches the specified StorageClass.

The data structure is as below:
```go
type Configs struct {
    ...

	// LoadAffinity is the config for data path load affinity.
	LoadAffinity []*LoadAffinity `json:"loadAffinity,omitempty"`    

	// LoadAffinityPerStorageClass is the config for data path load affinity per StorageClass.
	// Because this one is more specific for a data mover pod, it has higher priority than LoadAffinity.
	LoadAffinityPerStorageClass map[string][]*LoadAffinity `json:"loadAffinityPerStorageClass,omitempty"`

    ...
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

### Examples

#### LoadAffinity
A sample of the ConfigMap is as below:
``` json
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
                        "key": "xxx/critical-workload",
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
- matchExpressions: VGDP instances will not run in nodes with label key `xxx/critical-workload`

To create the ConfigMap, users need to save something like the above sample to a json file and then run below command:
```
kubectl create cm <ConfigMap name> -n velero --from-file=<json file name>
```

#### Multiple LoadAffinities

``` json
{
    "loadAffinity": [
        {
            "nodeSelector": {
                "matchLabels": {
                    "beta.kubernetes.io/instance-type": "Standard_B4ms"
                }
            }
        },
        {
            "nodeSelector": {
                "matchExpressions": [
                    {
                        "key": "topology.kubernetes.io/zone",
                        "operator": "In",
                        "values": [
                            "us-central1-a",
                        ]
                    }
                ]
            }
        }
    ]
}
```

This sample demonstrates how to use multiple affinities in `loadAffinity`. That can support more complicated scenarios, e.g. need to filter nodes satisfied either of two conditions, instead of satisfied both of two conditions.

In this example, the VGDP pods will be assigned to nodes, which instance type is `Standard_B4ms` or which zone is `us-central1-a`.


#### LoadAffinity interacts with LoadAffinityPerStorageClass

``` json
{
    "loadAffinity": [
        {
            "nodeSelector": {
                "matchLabels": {
                    "beta.kubernetes.io/instance-type": "Standard_B4ms"
                }
            }
        }
    ],
    "loadAffinityPerStorageClass": {
        "kibishii-storage-class": [
            {
                "nodeSelector": {
                    "matchExpressions": [
                        {
                            "key": "kubernetes.io/os",
                            "values": [
                                "linux"
                            ],
                            "operator": "In"
                        }
                    ]          
                }
            }
        ]
    }
}
```

This sample demonstrates how the `loadAffinity` and `loadAffinityPerStorageClass` work together.
If the VGDP mounting volume is created from StorageClass `kibishii-storage-class`, its pod will only run Linux nodes.
The other VGDP instances will run on nodes, which instance type is `Standard_B4ms`.

#### LoadAffinity interacts with BackupPVC
``` json
{
    "loadAffinityPerStorageClass": {
        "kibishii-storage-class": [
            {
                "nodeSelector": {
                    "matchLabels": {
                        "beta.kubernetes.io/instance-type": "Standard_B4ms"
                    }
                }
            }
        ],
        "worker-storagepolicy": [
            {
                "nodeSelector": {
                    "matchLabels": {
                        "beta.kubernetes.io/instance-type": "Standard_B2ms"
                    }
                }
            }
        ]
    },
    "backupPVC": {
        "kibishii-storage-class": {
            "storageClass": "worker-storagepolicy"
        }
    }
}
```

Velero data mover supports to use different StorageClass to create backupPVC by [design](https://github.com/vmware-tanzu/velero/pull/7982).

In this example, if the backup target PVC's StorageClass is `kibishii-storage-class`, and its backupPVC should use StorageClass `worker-storagepolicy`. Because the final StorageClass is `worker-storagepolicy`, the backupPod uses the loadAffinity specified by loadAffinityPerStorageClass["worker-storagepolicy"]. backupPod will be assigned to nodes, which instance type is `Standard_B2ms`.


#### LoadAffinity interacts with RestorePVC
``` json
{
    "loadAffinityPerStorageClass": {
        "kibishii-storage-class": [
            {
                "nodeSelector": {
                    "matchLabels": {
                        "beta.kubernetes.io/instance-type": "Standard_B4ms"
                    }
                }
            }
        ]
    },
    "restorePVC": {
        "ignoreDelayBinding": false
    }
}
```

##### StorageClass's bind mode is WaitForFirstConsumer
``` yaml
apiVersion: storage.k8s.io/v1
kind: StorageClass
metadata:
  name: kibishii-storage-class
parameters:
  svStorageClass: worker-storagepolicy
provisioner: csi.vsphere.vmware.com
reclaimPolicy: Delete
volumeBindingMode: WaitForFirstConsumer
```
If restorePVC should be created from StorageClass `kibishii-storage-class`, and it's volumeBindingMode is `WaitForFirstConsumer`.
Although `loadAffinityPerStorageClass` has a section matches the StorageClass, the `ignoreDelayBinding` is set `false`, the Velero exposer will wait until the target Pod scheduled to a node, and returns the node as SelectedNode for the restorePVC.
As a result, the `loadAffinityPerStorageClass` will not take affect.

##### StorageClass's bind mode is Immediate
``` yaml
apiVersion: storage.k8s.io/v1
kind: StorageClass
metadata:
  name: kibishii-storage-class
parameters:
  svStorageClass: worker-storagepolicy
provisioner: csi.vsphere.vmware.com
reclaimPolicy: Delete
volumeBindingMode: Immediate
```
Because the StorageClass volumeBindingMode is `Immediate`, although `ignoreDelayBinding` is set to `false`, restorePVC will not be created according to the target Pod.
The restorePod will be assigned to nodes, which instance type is `Standard_B4ms`.


### Implementation
As mentioned in the [Volume Snapshot Data Movement Design][2], the exposer decides where to launch the VGDP instances. At present, for volume snapshot data movement, the exposer creates pods and the VGDP instances will be initiated in the nodes where pods are scheduled. The loadAffinity will be translated (from `metav1.LabelSelector` to `corev1.Affinity`) and set to the pods.  

It is possible that node-agent pods, as a DaemonSet, don't run in every worker nodes, users could fulfil this by specify `nodeSelector` or `nodeAffinity` to the node-agent DaemonSet spec. On the other hand, at present, VGDP instances must be assigned to nodes where node-agent pods are running. Therefore, if there is any node selection for node-agent pods, users must consider this into this load affinity configuration, so as to guarantee that VGDP instances are always assigned to nodes where node-agent pods are available. This is done by users, we don't inherit any node selection configuration from node-agent DaemonSet as we think DaemonSet scheduler works differently from plain pod scheduler, simply inheriting all the configurations may cause unexpected result of pod schedule.
Otherwise, if a VGDP pod is scheduled to a node where node-agent pod is absent, the corresponding DataUpload CR will stay in `Accepted` phase until the prepare timeout (by default 30min).

At present, as part of the expose operations, the exposer creates a volume, represented by VGDP PVC, from the snapshot. By default, The VGDP PVC uses the same StorageClass with the source volume. If the `volumeBindingMode` in the StorageClass is `Immediate`, the volume is immediately allocated from the underlying storage without waiting for the VGDP pod. On the other hand, the loadAffinity is set to the VGDP Pod's affinity. If the VGDP Pod is scheduled to a node where the snapshot volume is not accessible, e.g., because of storage topologies, the VGDP Pod won't get into Running state, consequently, the data movement won't complete.
Once this problem happens, the VGDP Pod stays in `Pending` phase, and the corresponding DataUpload CR stays in `Accepted` phase until the prepare timeout (by default 30min). Below is an example of the VGDP Pod's status when the problem happens:

``` yaml
  status:
    conditions:
    - lastProbeTime: null
      message: "0/2 nodes are available: 1 node(s) didn't match Pod's node affinity/selector,
        1 node(s) had volume node affinity conflict. preemption: 0/2 nodes are available:
        2 Preemption is not helpful for scheduling.."
      reason: Unschedulable
      status: "False"
      type: PodScheduled
    phase: Pending
```

On the other hand, the VGDP pod is deleted after the prepare timeout, so there is no way to tell the cause is one of the above problems or others.
To help the troubleshooting, we can add some diagnostic mechanism to discover the status of the VGDP pod and node-agent in the same node before deleting it as a result of the prepare timeout.

Users can also use `BackupPVC` to choose a different StorageClass to create backupPVC. Please check [this example](#loadaffinity-interacts-with-backuppvc) for details.

[1]: unified-repo-and-kopia-integration/unified-repo-and-kopia-integration.md
[2]: volume-snapshot-data-movement/volume-snapshot-data-movement.md