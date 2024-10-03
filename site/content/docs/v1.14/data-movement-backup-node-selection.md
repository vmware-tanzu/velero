---
title: "Node Selection for Data Movement Backup"
layout: docs
---

Velero node-agent is a daemonset hosting the data movement modules to complete the concrete work of backups/restores.    
Varying from the data size, data complexity, resource availability, the data movement may take a long time and remarkable resources (CPU, memory, network bandwidth, etc.) during the backup and restore. 

Velero data movement backup supports to constrain the nodes where it runs. This is helpful in below scenarios:  
- Prevent the data movement backup from running in specific nodes because users have more critical workloads in the nodes  
- Constrain the data movement backup to run in specific nodes because these nodes have more resources than others  
- Constrain the data movement backup to run in specific nodes because the storage allows volume/snapshot provisions in these nodes only  

Velero introduces a new section in ```node-agent-config``` configMap, called ```loadAffinity```, through which you can specify the nodes to/not to run data movement backups, in the affinity and anti-affinity flavors.  
If it is not there, ```node-agent-config``` should be created manually. The configMap should be in the same namespace where Velero is installed. If multiple Velero instances are installed in different namespaces, there should be one configMap in each namespace which applies to node-agent in that namespace only.  
Node-agent server checks these configurations at startup time. Therefore, you could edit this configMap any time, but in order to make the changes effective, node-agent server needs to be restarted.  

### Sample
Here is a sample of the ```node-agent-config``` configMap with ```loadAffinity```:
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
To create the configMap, save something like the above sample to a json file and then run below command:
```
kubectl create cm node-agent-config -n velero --from-file=<json file name>
```

### Affinity
Affinity configuration means allowing the data movement backup to run in the nodes specified. There are two ways to define it:
-  It could be defined by `MatchLabels`. The labels defined in `MatchLabels` means a `LabelSelectorOpIn` operation by default, so in the current context, they will be treated as affinity rules. In the above sample, it defines to run data movement backups in nodes with label `beta.kubernetes.io/instance-type` of value `Standard_B4ms` (Run data movement backups in `Standard_B4ms` nodes only).    
- It could be defined by `MatchExpressions`. The labels are defined in `Key` and `Values` of `MatchExpressions` and the `Operator` should be defined as `LabelSelectorOpIn` or `LabelSelectorOpExists`. In the above sample, it defines to run data movement backups in nodes with label `kubernetes.io/hostname` of values `node-1`, `node-2` and `node-3` (Run data movement backups in `node-1`, `node-2` and `node-3` only).  

### Anti-affinity
Anti-affinity configuration means preventing the data movement backup from running in the nodes specified. Below is the way to define it:  
- It could be defined by `MatchExpressions`. The labels are defined in `Key` and `Values` of `MatchExpressions` and the `Operator` should be defined as `LabelSelectorOpNotIn` or `LabelSelectorOpDoesNotExist`. In the above sample, it disallows data movement backups to run in nodes with label `xxx/critial-workload`.  