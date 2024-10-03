# Node-agent Concurrency Design

## Glossary & Abbreviation

**Velero Generic Data Path (VGDP)**: VGDP is the collective of modules that is introduced in [Unified Repository design][1]. Velero uses these modules to finish data transfer for various purposes (i.e., PodVolume backup/restore, Volume Snapshot Data Movement). VGDP modules include uploaders and the backup repository.  

## Background

Velero node-agent is a daemonset hosting controllers and VGDP modules to complete the concrete work of backups/restores, i.e., PodVolume backup/restore, Volume Snapshot Data Movement backup/restore.  
For example, node-agent runs DataUpload controllers to watch DataUpload CRs for Volume Snapshot Data Movement backups, so there is one controller instance in each node. One controller instance takes a DataUpload CR and then launches a VGDP instance, which initializes a uploader instance and the backup repository connection, to finish the data transfer. The VGDP instance runs inside the node-agent pod or in a pod associated to the node-agent pod in the same node.  

Varying from the data size, data complexity, resource availability, VGDP may take a long time and remarkable resources (CPU, memory, network bandwidth, etc.).  
Technically, VGDP instances are able to run in concurrent regardless of the requesters. For example, a VGDP instance for a PodVolume backup could run in parallel with another VGDP instance for a DataUpload. Then the two VGDP instances share the same resources if they are running in the same node.  

Therefore, in order to gain the optimized performance with the limited resources, it is worthy to configure the concurrent number of VGDP per node. When the resources are sufficient in nodes, users can set a large concurrent number, so as to reduce the backup/restore time; otherwise, the concurrency should be reduced, otherwise, the backup/restore may encounter problems, i.e., time lagging, hang or OOM kill.  

## Goals

- Define the behaviors of concurrent VGDP instances in node-agent
- Create a mechanism for users to specify the concurrent number of VGDP per node

## Non-Goals
- VGDP instances from different nodes always run in concurrent since in most common cases the resources are isolated. For special cases that some resources are shared across nodes, there is no support at present
- In practice, restores run in prioritized scenarios, e.g., disaster recovery. However, the current design doesn't consider this difference, a VGDP instance for a restore is blocked if it reaches to the limit of the concurrency, even though the ones block it are for backups. If users do meet some problems here, they should consider to stop the backups first
- Sometimes, users wants to totally block backups/restores from running in a specific node, this is out of the scope the current design. To archive this, more modules need to be considered (i.e., expoers of data movers), simply blocking the VGDP (e.g., by setting its concurrent number to 0) doesn't work. E.g., for a fs backup, VGDP instance must run in the node the source pod is running in, if we simply block from VGDP instance, the PodVolumeBackup CR is still submitted but never processed.  

## Solution

We introduce a ConfigMap specified by `velero node-agent` CLI's parameter `--node-agent-configmap` for users to specify the node-agent related configurations. This configMap is not created by Velero, users should create it manually on demand. The configMap should be in the same namespace where Velero is installed. If multiple Velero instances are installed in different namespaces, there should be one configMap in each namespace which applies to node-agent in that namespace only.  
Node-agent server checks these configurations at startup time and use it to initiate the related VGDP modules. Therefore, users could edit this configMap any time, but in order to make the changes effective, node-agent server needs to be restarted.  
The ConfigMap may be used for other purpose of configuring node-agent in future, at present, there is only one kind of configuration as the data in the configMap, the name is ```loadConcurrency```.  

The data structure is as below:
```go
type Configs struct {
	// LoadConcurrency is the config for load concurrency per node.
	LoadConcurrency *LoadConcurrency `json:"loadConcurrency,omitempty"`
}

type LoadConcurrency struct {
    // GlobalConfig specifies the concurrency number to all nodes for which per-node config is not specified
    GlobalConfig int `json:"globalConfig,omitempty"`

    // PerNodeConfig specifies the concurrency number to nodes matched by rules
    PerNodeConfig []RuledConfigs `json:"perNodeConfig,omitempty"`
}

type RuledConfigs struct {
    // NodeSelector specifies the label selector to match nodes
    NodeSelector metav1.LabelSelector `json:"nodeSelector"`

    // Number specifies the number value associated to the matched nodes
    Number int `json:"number"`
}
```

### Global concurrent number
We allow users to specify a concurrent number that will be applied to all nodes if the per-node number is not specified. This number is set through ```globalConfig```.  
The number starts from 1 which means there is no concurrency, only one instance of VGDP is allowed. There is no roof limit.    
If this number is not specified or not valid, a hard-coded default value will be used, the value is set to 1. 

### Per-node concurrent number
We allow users to specify different concurrent number per node, for example, users can set 3 concurrent instances in Node-1, 2 instances in Node-2 and 1 instance in Node-3. This is for below considerations:
- The resources may be different among nodes. Then users could specify smaller concurrent number for nodes with less resources while larger number for the ones with more resources
- Help users to isolate critical environments. Users may run some critical workloads in some specified nodes, since VGDP instances may take large resource consumption, users may want to run less number of instances in the nodes with critical workloads

The range of Per-node concurrent number is the same with Global concurrent number.  
Per-node concurrent number is preferable to Global concurrent number, so it will overwrite the Global concurrent number for that node.  

Per-node concurrent number is implemented through ```perNodeConfig``` field.  

```perNodeConfig``` is a list of ```RuledConfigs``` each item of which matches one or more nodes by label selectors and specify the concurrent number for the matched nodes. This means, the nodes are identified by labels.  

For example, the ```perNodeConfig`` could have below elements:
```
"nodeSelector: kubernetes.io/hostname=node1; number: 3"
"nodeSelector: beta.kubernetes.io/instance-type=Standard_B4ms; number: 5"
```
The first element means the node with host name ```node1``` gets the Per-node concurrent number of 3.  
The second element means all the nodes with label ```beta.kubernetes.io/instance-type``` of value ```Standard_B4ms``` get the Per-node concurrent number of 5. 
At least one node is expected to have a label with the specified ```RuledConfigs``` element (rule). If no node is with this label, the Per-node rule makes no effect.  
If one node falls into more than one rules, e.g., if node1 also has the label ```beta.kubernetes.io/instance-type=Standard_B4ms```, the smallest number (3) will be used.  

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
        ]
    }
}
```
To create the configMap, users need to save something like the above sample to a json file and then run below command:
```
kubectl create cm <ConfigMap name> -n velero --from-file=<json file name>
```

### Global data path manager
As for the code implementation, data path manager is to maintain the total number of the running VGDP instances and ensure the limit is not excceeded. At present, there is one data path manager instance per controller, as a result, the concurrent numbers are calculated separately for each controller. This doesn't help to limit the concurrency among different requesters.  
Therefore, we need to create one global data path manager instance server-wide, and pass it to different controllers. The instance will be created at node-agent server startup.  
The concurrent number is required to initiate a data path manager, the number comes from either Per-node concurrent number or Global concurrent number.    
Below are some prototypes related to data path manager:  

```go
func NewManager(cocurrentNum int) *Manager
func (m *Manager) CreateFileSystemBR(jobName string, requestorType string, ctx context.Context, client client.Client, namespace string, callbacks Callbacks, log logrus.FieldLogger) (AsyncBR, error)
```





[1]: Implemented/unified-repo-and-kopia-integration/unified-repo-and-kopia-integration.md