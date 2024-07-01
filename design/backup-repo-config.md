# Backup Repository Configuration Design

## Glossary & Abbreviation

**Backup Storage**: The storage to store the backup data. Check [Unified Repository design][1] for details.  
**Backup Repository**: Backup repository is layered between BR data movers and Backup Storage to provide BR related features that is introduced in [Unified Repository design][1].    

## Background

According to the [Unified Repository design][1] Velero uses selectable backup respositories for various backup/restore methods, i.e., fs-backup, volume snapshot data movement, etc. To achieve the best performance, backup respositories may need to be configured according to the running environments.  
For example, if there are sufficient CPU and memory resources in the environment, users may enable compression feature provided by the backup repository, so as to achieve the best backup throughput.  
As another example, if the local disk space is not sufficent, users may want to constraint the backup repository's cache size, so as to prevent from running out of the disk space.  
Therefore, it is worthy to allow users to configure some essential prameters of the backup repsoitories, and the configuration may vary from backup respositories.  

## Goals

- Create a mechnism for users to specify configurations for backup repositories

## Non-Goals

## Solution

### BackupRepository CRD

After a backup repository is initialized, a BackupRepository CR is created to represent the instance of the backup repository. The BackupRepository's spec is a core parameter used by Uinified Repo when interactive with the backup repsoitory. Therefore, we can add the configurations into the BackupRepository CR.  
The configurations may be different varying from backup repositories, therefore, we will not define each of the configurations explictly. Instead, we add a map in the BackupRepository's spec to take any configuration to be set to the backup repository.  
During various operations to the backup repository, the Unified Repo modules will retrieve from the map for the specific configuration that is required at that time. So even though it is specified, a configuration may not be visite/hornored if the operations don't require it for the specific backup repository, this won't bring any issue.  

Below is the new BackupRepository's spec after adding the configuration map:  

### BackupRepository configMap

The BackupRepository CR is not created explicitly but a Velero CLI, but created as part of the backup/restore/maintenance operation if the CR doesn't exist. As a result, users don't have any way to specify the configurations before the BackupRepository CR is created.  
Therefore, we create a BackupRepository configMap as a template of the configurations to be applied to the backup repository CR created during the backup/restore/maintenance operation. For an existing BackupRepository CR, the configMap is never visited, if users want to modify the configuration value, they should directly edit the BackupRepository CR.   
The BackupRepository configMap is created by users. If the configMap is not there, nothing is specified to the backup repository CR, so the Unified Repo modules use the hard-coded values to configre the backup repository.  
The BackupRepository configMap is backup repository type specific, that is, for each type of backup repository, there is one configMap. Therefore, a ```backup-repository-config``` label should be applied to the configMap with the value of the repository's type. During the backup repository creation, the configMap is searched by the label.  

### Configurations

With the above mechanisms, all kinds of configurations could be added. Here list the configurations supported at present:  
```cache-limit```: specify the size limit for the local data cache. The more data is cached locally, the less data is donwloaded from the backup storage, so the better performance may be achieved. You can specify any size that is smaller than the free space so that the disk space won't run out. This parameter is for each repository connection, that is, users could change it before connecting to the repository.    
```compression-algo```: specify the compression algorithm to be used for a backup repsotiory. Most of the backup repositories support the data compression feature, but they may support different compression algorithms or performe differently to compression algorithms. This parameter is used when intializing the backup repository, once the backup repository is initialized, the compression algorithm won't be changed, so this parameter is ignored.  

Below is an example of the BackupRepository configMap with the supported configurations:  
```go
```

To create the configMap, users need to save something like the above sample to a json file and then run below command:
```
kubectl create cm node-agent-config -n velero --from-file=<json file name>
```



[1]: Implemented/unified-repo-and-kopia-integration/unified-repo-and-kopia-integration.md