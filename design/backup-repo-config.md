# Backup Repository Configuration Design

## Glossary & Abbreviation

**Backup Storage**: The storage to store the backup data. Check [Unified Repository design][1] for details.  
**Backup Repository**: Backup repository is layered between BR data movers and Backup Storage to provide BR related features that is introduced in [Unified Repository design][1].    

## Background

According to the [Unified Repository design][1] Velero uses selectable backup repositories for various backup/restore methods, i.e., fs-backup, volume snapshot data movement, etc. To achieve the best performance, backup repositories may need to be configured according to the running environments.  
For example, if there are sufficient CPU and memory resources in the environment, users may enable compression feature provided by the backup repository, so as to achieve the best backup throughput.  
As another example, if the local disk space is not sufficient, users may want to constraint the backup repository's cache size, so as to prevent the repository from running out of the disk space.  
Therefore, it is worthy to allow users to configure some essential parameters of the backup repsoitories, and the configuration may vary from backup repositories.  

## Goals

- Create a mechanism for users to specify configurations for backup repositories  

## Non-Goals

## Solution

### BackupRepository CRD

After a backup repository is initialized, a BackupRepository CR is created to represent the instance of the backup repository. The BackupRepository's spec is a core parameter used by Unified Repo modules when interactive with the backup repsoitory. Therefore, we can add the configurations into the BackupRepository CR called ```repositoryConfig```.  
The configurations may be different varying from backup repositories, therefore, we will not define each of the configurations explicitly. Instead, we add a map in the BackupRepository's spec to take any configuration to be set to the backup repository.  

During various operations to the backup repository, the Unified Repo modules will retrieve from the map for the specific configuration that is required at that time. So even though it is specified, a configuration may not be visited/hornored if the operations don't require it for the specific backup repository, this won't bring any issue. When and how a configuration is hornored is decided by the configuration itself and should be clarified in the configuration's specification.  

Below is the new BackupRepository's spec after adding the configuration map:  
```yaml
          spec:
            description: BackupRepositorySpec is the specification for a BackupRepository.
            properties:
              backupStorageLocation:
                description: |-
                  BackupStorageLocation is the name of the BackupStorageLocation
                  that should contain this repository.
                type: string
              maintenanceFrequency:
                description: MaintenanceFrequency is how often maintenance should
                  be run.
                type: string
              repositoryConfig:
                additionalProperties:
                  type: string
                description: RepositoryConfig contains configurations for the specific
                  repository.
                type: object
              repositoryType:
                description: RepositoryType indicates the type of the backend repository
                enum:
                - kopia
                - restic
                - ""
                type: string
              resticIdentifier:
                description: |-
                  ResticIdentifier is the full restic-compatible string for identifying
                  this repository.
                type: string
              volumeNamespace:
                description: |-
                  VolumeNamespace is the namespace this backup repository contains
                  pod volume backups for.
                type: string
            required:
            - backupStorageLocation
            - maintenanceFrequency
            - resticIdentifier
            - volumeNamespace
            type: object
```            

### BackupRepository configMap

The BackupRepository CR is not created explicitly by a Velero CLI, but created as part of the backup/restore/maintenance operation if the CR doesn't exist. As a result, users don't have any way to specify the configurations before the BackupRepository CR is created.  
Therefore, a BackupRepository configMap is introduced as a template of the configurations to be applied to the backup repository CR.  
When the backup repository CR is created by the BackupRepository controller, the configurations in the configMap are copied to the ```repositoryConfig``` field.   
For an existing BackupRepository CR, the configMap is never visited, if users want to modify the configuration value, they should directly edit the BackupRepository CR.  

The BackupRepository configMap is created by users in velero installation namespace. The configMap name must be specified in the velero server parameter ```--backup-repository-config```, otherwise, it won't effect.  
If the configMap name is specified but the configMap doesn't exist by the time of a backup repository is created, the configMap name is ignored.  
For any reason, if the configMap doesn't effect, nothing is specified to the backup repository CR, so the Unified Repo modules use the hard-coded values to configure the backup repository.  

The BackupRepository configMap supports backup repository type specific configurations, even though users can only specify one configMap.  
So in the configMap struct, multiple entries are supported, indexed by the backup repository type. During the backup repository creation, the configMap is searched by the repository type.  

### Configurations

With the above mechanisms, any kind of configuration could be added. Here list the configurations defined at present:  
```cacheLimitMB```: specifies the size limit(in MB) for the local data cache. The more data is cached locally, the less data may be downloaded from the backup storage, so the better performance may be achieved. Practically, users can specify any size that is smaller than the free space so that the disk space won't run out. This parameter is for each repository connection, that is, users could change it before connecting to the repository. If a backup repository doesn't use local cache, this parameter will be ignored. For Kopia repository, this parameter is supported.  
```enableCompression```: specifies to enable/disable compression for a backup repsotiory. Most of the backup repositories support the data compression feature, if it is not supported by a backup repository, this parameter is ignored. Most of the backup repositories support to dynamically enable/disable compression, so this parameter is defined to be used whenever creating a write connection to the backup repository, if the dynamically changing is not supported, this parameter will be hornored only when initializing the backup repository. For Kopia repository, this parameter is supported and can be dynamically modified.  

### Sample
Below is an example of the BackupRepository configMap with the configurations:     
```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: <config-name>
  namespace: velero
data:
  <repository-type-1>: |
    {
      "cacheLimitMB": 2048,
      "enableCompression": true    
    }
  <repository-type-2>: |
    {
      "cacheLimitMB": 1,
      "enableCompression": false    
    }        
```

To create the configMap, users need to save something like the above sample to a file and then run below commands:  
```
kubectl apply -f <yaml file name>
```  



[1]: Implemented/unified-repo-and-kopia-integration/unified-repo-and-kopia-integration.md