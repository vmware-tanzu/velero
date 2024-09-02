# Backup PVC Configuration Design

## Glossary & Abbreviation

**Velero Generic Data Path (VGDP)**: VGDP is the collective modules that is introduced in [Unified Repository design][1]. Velero uses these modules to finish data transfer for various purposes (i.e., PodVolume backup/restore, Volume Snapshot Data Movement). VGDP modules include uploaders and the backup repository.  

**Exposer**: Exposer is a module that is introduced in [Volume Snapshot Data Movement Design][2]. Velero uses this module to expose the volume snapshots to Velero node-agent pods or node-agent associated pods so as to complete the data movement from the snapshots.  

**backupPVC**: The intermediate PVC created by the exposer for VGDP to access data from, see [Volume Snapshot Data Movement Design][2] for more details.  

**backupPod**: The pod consumes the backupPVC so that VGDP could access data from the backupPVC, see [Volume Snapshot Data Movement Design][2] for more details.  

**sourcePVC**: The PVC to be backed up, see [Volume Snapshot Data Movement Design][2] for more details. 

## Background

As elaberated in [Volume Snapshot Data Movement Design][2], a backupPVC may be created by the Exposer and the VGDP reads data from the backupPVC.  
In some scenarios, users may need to configure some advanced settings of the backupPVC so that the data movement could work in best performance in their environments. Specifically:  
- For some storage providers, when creating a read-only volume from a snapshot, it is very fast; whereas, if a writable volume is created from the snapshot, they need to clone the entire disk data, which is time consuming. If the backupPVC's `accessModes` is set as `ReadOnlyMany`, the volume driver is able to tell the storage to create a read-only volume, which may dramatically shorten the snapshot expose time. On the other hand,  `ReadOnlyMany` is not supported by all volumes. Therefore, users should be allowed to configure the `accessModes` for the backupPVC.  
- Some storage providers create one or more replicas when creating a volume, the number of replicas is defined in the storage class. However, it doesn't make any sense to keep replicas when an intermediate volume used by the backup. Therefore, users should be allowed to configure another storage class specifically used by the backupPVC.  

## Goals

- Create a mechanism for users to specify various configurations for backupPVC    

## Non-Goals

## Solution

We will use the ```node-agent-config``` configMap to host the backupPVC configurations.
This configMap is not created by Velero, users should create it manually on demand. The configMap should be in the same namespace where Velero is installed. If multiple Velero instances are installed in different namespaces, there should be one configMap in each namespace which applies to node-agent in that namespace only.  
Node-agent server checks these configurations at startup time and use it to initiate the related Exposer modules. Therefore, users could edit this configMap any time, but in order to make the changes effective, node-agent server needs to be restarted.  
Inside ```node-agent-config``` configMap we will add one new kind of configuration as the data in the configMap, the name is ```backupPVC```.  
Users may want to set different backupPVC configurations for different volumes, therefore, we define the configurations as a map and allow users to specific configurations by storage class. Specifically, the key of the map element is the storage class name used by the sourcePVC and the value is the set of configurations for the backupPVC created for the sourcePVC.   

The data structure for ```node-agent-config``` is as below:
```go
type Configs struct {
	// LoadConcurrency is the config for data path load concurrency per node.
	LoadConcurrency *LoadConcurrency `json:"loadConcurrency,omitempty"`

	// LoadAffinity is the config for data path load affinity.
	LoadAffinity []*LoadAffinity `json:"loadAffinity,omitempty"`

	// BackupPVC is the config for backupPVC of snapshot data movement.
	BackupPVC map[string]BackupPVC `json:"backupPVC,omitempty"`
}

type BackupPVC struct {
	// StorageClass is the name of storage class to be used by the backupPVC.
	StorageClass string `json:"storageClass,omitempty"`

	// ReadOnly sets the backupPVC's access mode as read only.
	ReadOnly bool `json:"readOnly,omitempty"`
}
```  

### Sample
A sample of the ```node-agent-config``` configMap is as below:
```json
{
    "backupPVC": {
        "storage-class-1": {
            "storageClass": "snapshot-storage-class",
            "readOnly": true
        },
        "storage-class-2": {
            "storageClass": "snapshot-storage-class"
        },
        "storage-class-3": {
            "readOnly": true
        }        
    }
}
```

To create the configMap, users need to save something like the above sample to a json file and then run below command:
```
kubectl create cm node-agent-config -n velero --from-file=<json file name>
``` 

### Implementation
The `backupPVC` is passed to the exposer and the exposer sets the related specification and create the backupPVC.  
If `backupPVC.storageClass` doesn't exist or set as empty, the sourcePVC's storage class will be used.  
If `backupPVC.readOnly` is set to true, `ReadOnlyMany` will be the only value set to the backupPVC's `accessModes`, otherwise, `ReadWriteOnce` is used.  

Once `backupPVC.storageClass` is set, users must make sure that the specified storage class exists in the cluster and can be used the the backupPVC, otherwise, the corresponding DataUpload CR will stay in `Accepted` phase until the prepare timeout (by default 30min).   
Once `backupPVC.readOnly` is set to true, users must make sure that the storage supports to create a `ReadOnlyMany` PVC from a snapshot, otherwise, the corresponding DataUpload CR will stay in `Accepted` phase until the prepare timeout (by default 30min).  

Once above problems happen, the DataUpload CR is cancelled after prepare timeout and the backupPVC and backupPod will be deleted, so there is no way to tell the cause is one of the above problems or others.  
To help the troubleshooting, we can add some diagnostic mechanism to discover the status of the backupPod before deleting it as a result of the prepare timeout.  

[1]: Implemented/unified-repo-and-kopia-integration/unified-repo-and-kopia-integration.md
[2]: volume-snapshot-data-movement/volume-snapshot-data-movement.md