---
title: "Data Movement Pod Resource Configuration"
layout: docs
---

During [CSI Snapshot Data Movement][1], Velero built-in data mover launches data mover pods to to run the data transfer. While the data transfer is a time and resource consuming activity.  

Velero built-in data mover by default uses the [BestEffort QoS][2] for the data mover pods, which guarantees the best performance of the data movement activities. On the other hand, it may take lots of cluster resource, i.e., CPU, memory, and how many resources are taken is decided by the concurrency and the scale of data to be moved.  

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
    }
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

[1]: csi-snapshot-data-movement.md
[2]: https://kubernetes.io/docs/concepts/workloads/pods/pod-qos/
[3]: performance-guidance.md