## v1.15.1

### Download
https://github.com/vmware-tanzu/velero/releases/tag/v1.15.1

### Container Image
`velero/velero:v1.15.1`

### Documentation
https://velero.io/docs/v1.15/

### Upgrading
https://velero.io/docs/v1.15/upgrade-to-1.15/

### All Changes
  * Fix backup post hook issue #8159 (caused by #7571): always execute backup post hooks after PVBs are handled (#8517, @ywk253100)
  * Fix issue #8125, log diagnostic info for data mover exposers when expose timeout (#8511, @Lyndon-Li)
  * Set hinting region to use for GetBucketRegion() in pkg/repository/config/aws.go (#8505, @kaovilai)
  * Fix issue #8433, add third party labels to data mover pods when the same labels exist in node-agent pods (#8501, @Lyndon-Li)
  * Fix issue #8485, add an accepted time so as to count the prepare timeout (#8496, @Lyndon-Li)
  * Add SecurityContext to restore-helper (#8495, @reasonerjt)
  * Add nil check for updating DataUpload VolumeInfo in finalizing phase. (#8465, @blackpiglet)
  * Fix issue #8391, check ErrCancelled from suffix of data mover pod's termination message (#8404, @Lyndon-Li)
  * Fix issue #8394, don't call closeDataPath in VGDP callbacks, otherwise, the VGDP cleanup will hang (#8402, @Lyndon-Li)
  * Reduce minimum required go toolchain in release-1.15 go.mod (#8399, @kaovilai)
  * Fix issue #8539, validate uploader types when o.CRDsOnly is set to false only since CRD installation doesn't rely on uploader types (#8540, @Lyndon-Li)


## v1.15

### Download
https://github.com/vmware-tanzu/velero/releases/tag/v1.15.0

### Container Image
`velero/velero:v1.15.0`

### Documentation
https://velero.io/docs/v1.15/

### Upgrading
https://velero.io/docs/v1.15/upgrade-to-1.15/

### Highlights
#### Data mover micro service
Data transfer activities for CSI Snapshot Data Movement are moved from node-agent pods to dedicate backupPods or restorePods. This brings many benefits such as:  
- This avoids to access volume data through host path, while host path access is privileged and may involve security escalations, which are concerned by users.
- This enables users to to control resource (i.e., cpu, memory) allocations in a granular manner, e.g., control them per backup/restore of a volume.
- This enhances the resilience, crash of one data movement activity won't affect others.
- This prevents unnecessary full backup because of host path changes after workload pods restart.
- For more information, check the design https://github.com/vmware-tanzu/velero/blob/main/design/Implemented/vgdp-micro-service/vgdp-micro-service.md.

#### Item Block concepts and ItemBlockAction (IBA) plugin
Item Block concepts are introduced for resource backups to help to achieve multiple thread backups. Specifically, correlated resources are categorized in the same item block and item blocks could be processed concurrently in multiple threads.  
ItemBlockAction plugin is introduced to help Velero to categorize resources into item blocks. At present, Velero provides built-in IBAs for pods and PVCs and Velero also supports customized IBAs for any resources.  
In v1.15, Velero doesn't support multiple thread process of item blocks though item block concepts and IBA plugins are fully supported. The multiple thread support will be delivered in future releases.  
For more information, check the design https://github.com/vmware-tanzu/velero/blob/main/design/backup-performance-improvements.md.  

#### Node selection for repository maintenance job
Repository maintenance are resource consuming tasks, Velero now allows you to configure the nodes to run repository maintenance jobs, so that you can run repository maintenance jobs in idle nodes or avoid them to run in nodes hosting critical workloads.  
To support the configuration, a new repository maintenance configuration configMap is introduced.  
For more information, check the document https://velero.io/docs/v1.15/repository-maintenance/.  

#### Backup PVC read-only configuration
In 1.15, Velero allows you to configure the data mover backupPods to read-only mount the backupPVCs. In this way, the data mover expose process could be significantly accelerated for some storages (i.e., ceph).  
To support the configuration, a new backup PVC configuration configMap is introduced.  
For more information, check the document https://velero.io/docs/v1.15/data-movement-backup-pvc-configuration/.  

#### Backup PVC storage class configuration
In 1.15, Velero allows you to configure the storageclass used by the data mover backupPods. In this way, the provision of backupPVCs don't need to adhere to the same pattern as workload PVCs, e.g., for a backupPVC, it only needs one replica, whereas, the a workload PVC may have multiple replicas.  
To support the configuration, the same backup PVC configuration configMap is used.  
For more information, check the document https://velero.io/docs/v1.15/data-movement-backup-pvc-configuration/.  

#### Backup repository data cache configuration
The backup repository may need to cache data on the client side during various repository operations, i.e., read, write, maintenance, etc. The cache consumes the root file system space of the pod where the repository access happens.  
In 1.15, Velero allows you to configure the total size of the cache per repository. In this way, if your pod doesn't have enough space in its root file system, the pod won't be evicted due to running out of ephemeral storage.  
To support the configuration, a new backup repository configuration configMap is introduced.  
For more information, check the document https://velero.io/docs/v1.15/backup-repository-configuration/.  

#### Performance improvements
In 1.15, several performance related issues/enhancements are included, which makes significant performance improvements in specific scenarios:  
- There was a memory leak of Velero server after plugin calls, now it is fixed, see issue https://github.com/vmware-tanzu/velero/issues/7925
- The `client-burst/client-qps` parameters are automatically inherited to plugins, so that you can use the same velero server parameters to accelerate the plugin executions when large number of API server calls happen, see issue https://github.com/vmware-tanzu/velero/issues/7806
- Maintenance of Kopia repository takes huge memory in scenarios that huge number of files have been backed up, Velero 1.15 has included the Kopia upstream enhancement to fix the problem, see issue https://github.com/vmware-tanzu/velero/issues/7510

### Runtime and dependencies
Golang runtime: v1.22.8  
kopia: v0.17.0

### Limitations/Known issues
#### Read-only backup PVC may not work on SELinux environments
Due to an issue of Kubernetes upstream, if a volume is mounted as read-only in SELinux environments, the read privilege is not granted to any user, as a result, the data mover backup will fail. On the other hand, the backupPVC must be mounted as read-only in order to accelerate the data mover expose process.  
Therefore, a user option is added in the same backup PVC configuration configMap, once the option is enabled, the backupPod container will run as a super privileged container and disable SELinux access control. If you have concern in this super privileged container or you have configured [pod security admissions](https://kubernetes.io/docs/concepts/security/pod-security-admission/) and don't allow super privileged containers, you will not be able to use this read-only backupPVC feature and lose the benefit to accelerate the data mover expose process.  

### Breaking changes
#### Deprecation of Restic
Restic path for fs-backup is in deprecation process starting from 1.15. According to [Velero deprecation policy](https://github.com/vmware-tanzu/velero/blob/v1.15/GOVERNANCE.md#deprecation-policy), for 1.15, if Restic path is used the backup/restore of fs-backup still creates and succeeds, but you will see warnings in below scenarios:  
- When `--uploader-type=restic` is used in Velero installation
- When Restic path is used to create backup/restore of fs-backup

#### node-agent configuration name is configurable
Previously, a fixed name is searched for node-agent configuration configMap. Now in 1.15, Velero allows you to customize the name of the configMap, on the other hand, the name must be specified by node-agent server parameter `node-agent-configmap`.  

#### Repository maintenance job configurations in Velero server parameter are moved to repository maintenance job configuration configMap
In 1.15, below Velero server parameters for repository maintenance jobs are moved to the repository maintenance job configuration configMap. While for back compatibility reason, the same Velero sever parameters are preserved as is. But the configMap is recommended and the same values in the configMap take preference if they exist in both places:  
```
--keep-latest-maintenance-jobs
--maintenance-job-cpu-request
--maintenance-job-mem-request
--maintenance-job-cpu-limit
--maintenance-job-mem-limit
```

#### Changing PVC selected-node feature is deprecated
In 1.15, the [Changing PVC selected-node feature](https://velero.io/docs/v1.15/restore-reference/#changing-pvc-selected-node) enters deprecation process and will be removed in future releases according to [Velero deprecation policy](https://github.com/vmware-tanzu/velero/blob/v1.15/GOVERNANCE.md#deprecation-policy). Usage of this feature for any purpose is not recommended.  

### All Changes
  * add no-relabeling option to backupPVC configmap (#8288, @sseago)
  * only set spec.volumes readonly if PVC is readonly for datamover (#8284, @sseago)
  * Add labels to maintenance job pods (#8256, @shubham-pampattiwar)
  * Add the Carvel package related resources to the restore priority list (#8228, @ywk253100)
  * Reduces indirect imports for plugin/framework importers (#8208, @kaovilai)
  * Add controller name to periodical_enqueue_source. The logger parameter now includes an additional field with the value of reflect.TypeOf(objList).String() and another field with the value of controllerName. (#8198, @kaovilai)
  * Update Openshift SCC docs link (#8170, @shubham-pampattiwar)
  * Partially fix issue #8138, add doc for node-agent memory preserve (#8167, @Lyndon-Li)
  * Pass Velero server command args to the plugins (#8166, @ywk253100)
  * Fix issue #8155, Merge Kopia upstream commits for critical issue fixes and performance improvements (#8158, @Lyndon-Li)
  * Implement the Repo maintenance Job configuration. (#8145, @blackpiglet)
  * Add document for data mover micro service (#8144, @Lyndon-Li)
  * Fix issue #8134, allow to config resource request/limit for data mover micro service pods (#8143, @Lyndon-Li)
  * Apply backupPVCConfig to backupPod volume spec (#8141, @shubham-pampattiwar)
  * Add resource modifier for velero restore describe CLI (#8139, @blackpiglet)
  * Fix issue #7620, add doc for backup repo config (#8131, @Lyndon-Li)
  * Modify E2E and perf test report generated directory (#8129, @blackpiglet)
  * Add docs for backup pvc config support (#8119, @shubham-pampattiwar)
  * Delete generated k8s client and informer. (#8114, @blackpiglet)
  * Add support for backup PVC configuration (#8109, @shubham-pampattiwar)
  * ItemBlock model and phase 1 (single-thread) workflow changes (#8102, @sseago)
  * Fix issue #8032, make node-agent configMap name configurable (#8097, @Lyndon-Li)
  * Fix issue #8072, add the warning messages for restic deprecation (#8096, @Lyndon-Li)
  * Fix issue #7620, add backup repository configuration implementation and support cacheLimit configuration for Kopia repo (#8093, @Lyndon-Li)
  * Patch dbr's status when error happens (#8086, @reasonerjt)
  * According to design #7576, after node-agent restarts, if a DU/DD is in InProgress status, re-capture the data mover ms pod and continue the execution (#8085, @Lyndon-Li)
  * Updates to IBM COS documentation to match current version (#8082, @gjanders)
  * Data mover micro service DUCR/DDCR controller refactor according to design #7576 (#8074, @Lyndon-Li)
  * add retries with timeout to existing patch calls that moves a backup/restore from InProgress/Finalizing to a final status phase. (#8068, @kaovilai)
  * Data mover micro service restore according to design #7576 (#8061, @Lyndon-Li)
  * Internal ItemBlockAction plugins (#8054, @sseago)
  * Data mover micro service backup according to design #7576 (#8046, @Lyndon-Li)
  * Avoid wrapping failed PVB status with empty message. (#8028, @mrnold)
  * Created new ItemBlockAction (IBA) plugin type (#8026, @sseago)
  * Make PVPatchMaximumDuration timeout configurable (#8021, @shubham-pampattiwar)
  * Reuse existing plugin manager for get/put volume info (#8012, @sseago)
  * Data mover ms watcher according to design #7576 (#7999, @Lyndon-Li)
  * New data path for data mover ms according to design #7576 (#7988, @Lyndon-Li)
  * For issue #7700 and #7747, add the design for backup PVC configurations (#7982, @Lyndon-Li)
  * Only get VolumeSnapshotClass when DataUpload exists. (#7974, @blackpiglet)
  * Fix issue #7972, sync the backupPVC deletion in expose clean up (#7973, @Lyndon-Li)
  * Expose the VolumeHelper to third-party plugins. (#7969, @blackpiglet)
  * Check whether the volume's source is PVC before fetching its PV. (#7967, @blackpiglet)
  * Check whether the namespaces specified in namespace filter exist. (#7965, @blackpiglet)
  * Add design for backup repository configurations for issue #7620, #7301 (#7963, @Lyndon-Li)
  * New data path for data mover ms according to design #7576 (#7955, @Lyndon-Li)
  * Skip PV patch step in Restoe workflow for WaitForFirstConsumer VolumeBindingMode Pending state PVCs (#7953, @shubham-pampattiwar)
  * Fix issue #7904, add the deprecation and limitation clarification for change PVC selected-node feature (#7948, @Lyndon-Li)
  * Expose the VolumeHelper to third-party plugins. (#7944, @blackpiglet)
  * Don't consider unschedulable pods unrecoverable (#7899, @sseago)
  * Upgrade to robfig/cron/v3 to support time zone specification. (#7793, @kaovilai)
  * Add the result in the backup's VolumeInfo. (#7775, @blackpiglet)
  * Migrate from github.com/golang/protobuf to google.golang.org/protobuf (#7593, @mmorel-35)
  * Add the design for data mover micro service (#7576, @Lyndon-Li)
  * Descriptive restore error when restoring into a terminating namespace. (#7424, @kaovilai)
  * Ignore missing path error in conditional match (#7410, @seanblong)
  * Propose a deprecation process for velero (#5532, @shubham-pampattiwar)
