---
title: "Data Movement Pod Resource Configuration"
layout: docs
---

During [CSI Snapshot Data Movement][1], Velero built-in data mover launches data mover pods to run the data transfer.  
During [fs-backup][2], Velero also launches data mover pods to run the data transfer.  
The data transfer is a time and resource consuming activity.  

Velero by default uses the [BestEffort QoS][2] for the data mover pods, which guarantees the best performance of the data movement activities. On the other hand, it may take lots of cluster resource, i.e., CPU, memory, and how many resources are taken is decided by the concurrency and the scale of data to be moved.  

If the cluster nodes don't have sufficient resource, Velero also allows you to customize the resources for the data mover pods.    
Note: If less resources are assigned to data mover pods, the data movement activities may take longer time; or the data mover pods may be OOM killed if the assigned memory resource doesn't meet the requirements. Consequently, the dataUpload/dataDownload may run longer or fail.  

Refer to [Performance Guidance][3] for a guidance of performance vs. resource usage, and it is highly recommended that you perform your own testing to find the best resource limits for your data.  

Velero introduces a new section in the node-agent configMap, called ```podResources```, through which you can set customized resources configurations for data mover pods.  
If it is not there, a configMap should be created manually. The configMap should be in the same namespace where Velero is installed. If multiple Velero instances are installed in different namespaces, there should be one configMap in each namespace which applies to node-agent in that namespace only. The name of the configMap should be specified in the node-agent server parameter ```--node-agent-config```.  
Node-agent server checks these configurations at startup time. Therefore, you could edit this configMap any time, but in order to make the changes effective, node-agent server needs to be restarted.  

### Sample
Here is a sample of the configMap with ```podResources```:  
```json
{
    "podResources": {
        "cpuRequest": "1000m",
        "cpuLimit": "1000m",
        "memoryRequest": "512Mi",
        "memoryLimit": "1Gi"        
    },
    "priorityClassName": "high-priority"
}
```

The string values in ```podResources``` must match Kubernetes Quantity expressions; for each resource, the "request" value must not be larger than the "limit" value. Otherwise, if any one of the values fail, the entire ```podResources``` configuration will be ignored (so the default policy will be used).  

To create the configMap, save something like the above sample to a json file and then run below command:
```
kubectl create cm node-agent-config -n velero --from-file=<json file name>
```

To provide the configMap to node-agent, edit the node-agent daemonset and add the ```- --node-agent-config``` argument to the spec:
1. Open the node-agent daemonset spec  
```
kubectl edit ds node-agent -n velero
```
2. Add ```- --node-agent-config``` to ```spec.template.spec.containers```  
```
spec:
  template:
    spec:
      containers:
      - args:
        - --node-agent-config=<configMap name>
```

### Priority Class

Data mover pods will use the priorityClassName configured in the node-agent configmap. The priorityClassName for data mover pods is configured through the node-agent configmap (specified via the `--node-agent-configmap` flag), while the node-agent daemonset itself uses the priority class set by the `--node-agent-priority-class-name` flag during Velero installation.

#### When to Use Priority Classes

**Higher Priority Classes** (e.g., `system-cluster-critical`, `system-node-critical`, or custom high-priority):
- When you have dedicated nodes for backup operations
- When backup/restore operations are time-critical
- When you want to ensure data mover pods are scheduled even during high cluster utilization
- For disaster recovery scenarios where restore speed is critical

**Lower Priority Classes** (e.g., `low-priority` or negative values):
- When you want to protect production workload performance
- When backup operations can be delayed during peak hours
- When cluster resources are limited and production workloads take precedence
- For non-critical backup operations that can tolerate delays

#### Consequences of Priority Class Settings

**High Priority**:
- ✅ Data mover pods are more likely to be scheduled quickly
- ✅ Less likely to be preempted by other workloads
- ❌ May cause resource pressure on production workloads
- ❌ Could lead to production pod evictions in extreme cases

**Low Priority**:
- ✅ Production workloads are protected from resource competition
- ✅ Cluster stability is maintained during backup operations
- ❌ Backup/restore operations may take longer to start
- ❌ Data mover pods may be preempted, causing backup failures
- ❌ In resource-constrained clusters, backups might not run at all

#### Example Configuration

To configure priority class for data mover pods, include it in your node-agent configmap:

```json
{
    "podResources": {
        "cpuRequest": "1000m",
        "cpuLimit": "2000m",
        "memoryRequest": "1Gi",
        "memoryLimit": "4Gi"
    },
    "priorityClassName": "backup-priority"
}
```

First, create the priority class in your cluster:

```yaml
apiVersion: scheduling.k8s.io/v1
kind: PriorityClass
metadata:
  name: backup-priority
value: 1000
globalDefault: false
description: "Priority class for Velero data mover pods"
```

Then create or update the node-agent configmap:

```bash
kubectl create cm node-agent-config -n velero --from-file=node-agent-config.json
```

**Note**: If the specified priority class doesn't exist in the cluster when data mover pods are created, the pods will fail to schedule. Velero validates the priority class at startup and logs a warning if it doesn't exist, but the pods will still attempt to use it.

[1]: csi-snapshot-data-movement.md
[2]: file-system-backup.md
[3]: https://kubernetes.io/docs/concepts/workloads/pods/pod-qos/
[4]: performance-guidance.md