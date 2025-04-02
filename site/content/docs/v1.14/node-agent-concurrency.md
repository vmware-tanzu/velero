---
title: "Node-agent Concurrency"
layout: docs
---

Velero node-agent is a daemonset hosting modules to complete the concrete tasks of backups/restores, i.e., file system backup/restore, CSI snapshot data movement.  
Varying from the data size, data complexity, resource availability, the tasks may take a long time and remarkable resources (CPU, memory, network bandwidth, etc.). These tasks make the loads of node-agent.   

Node-agent concurrency configurations allow you to configure the concurrent number of node-agent loads per node. When the resources are sufficient in nodes, you can set a large concurrent number, so as to reduce the backup/restore time; otherwise, the concurrency should be reduced, otherwise, the backup/restore may encounter problems, i.e., time lagging, hang or OOM kill.  

To set Node-agent concurrency configurations, a configMap named ```node-agent-config``` should be created manually. The configMap should be in the same namespace where Velero is installed. If multiple Velero instances are installed in different namespaces, there should be one configMap in each namespace which applies to node-agent in that namespace only.  
Node-agent server checks these configurations at startup time. Therefore, you could edit this configMap any time, but in order to make the changes effective, node-agent server needs to be restarted.  

### Global concurrent number
You can specify a concurrent number that will be applied to all nodes if the per-node number is not specified. This number is set through ```globalConfig``` field in ```loadConcurrency```.  
The number starts from 1 which means there is no concurrency, only one load is allowed. There is no roof limit. If this number is not specified or not valid, a hard-coded default value will be used, the value is set to 1. 

### Per-node concurrent number
You can specify different concurrent number per node, for example, you can set 3 concurrent instances in Node-1, 2 instances in Node-2 and 1 instance in Node-3.  
The range of Per-node concurrent number is the same with Global concurrent number. Per-node concurrent number is preferable to Global concurrent number, so it will overwrite the Global concurrent number for that node.  

Per-node concurrent number is implemented through ```perNodeConfig``` field in ```loadConcurrency```.  
```perNodeConfig``` is a list of ```RuledConfigs``` each item of which matches one or more nodes by label selectors and specify the concurrent number for the matched nodes.  
Here is an example of the ```perNodeConfig``:
```
"nodeSelector: kubernetes.io/hostname=node1; number: 3"
"nodeSelector: beta.kubernetes.io/instance-type=Standard_B4ms; number: 5"
```
The first element means the node with host name ```node1``` gets the Per-node concurrent number of 3.  
The second element means all the nodes with label ```beta.kubernetes.io/instance-type``` of value ```Standard_B4ms``` get the Per-node concurrent number of 5. 
At least one node is expected to have a label with the specified ```RuledConfigs``` element (rule). If no node is with this label, the Per-node rule makes no effect.  
If one node falls into more than one rules, e.g., if node1 also has the label ```beta.kubernetes.io/instance-type=Standard_B4ms```, the smallest number (3) will be used.  

### Sample
A sample of the complete ```node-agent-config``` configMap is as below:
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
To create the configMap, save something like the above sample to a json file and then run below command:
```
kubectl create cm node-agent-config -n velero --from-file=<json file name>
```


