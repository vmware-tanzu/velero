## v1.17.1

### Download
https://github.com/vmware-tanzu/velero/releases/tag/v1.17.1

### Container Image
`velero/velero:v1.17.1`

### Documentation
https://velero.io/docs/v1.17/

### Upgrading
https://velero.io/docs/v1.17/upgrade-to-1.17/

### All Changes
  * Fix issue #9365, prevent fake completion notification due to multiple update of single PVR (#9376, @Lyndon-Li)
  * Fix issue #9332, add bytesDone for cache files (#9341, @Lyndon-Li)
  * VerifyJSONConfigs verify every elements in Data. (#9303, @blackpiglet)
  * Add option for privileged fs-backup pod (#9300, @sseago)
  * Fix repository maintenance jobs to inherit allowlisted tolerations from Velero deployment (#9299, @shubham-pampattiwar)
  * Fix issue #9229, don't attach backupPVC to the source node (#9297, @Lyndon-Li)
  * Protect VolumeSnapshot field from race condition during multi-thread backup (#9292, @0xLeo258)
  * Implement concurrency control for cache of native VolumeSnapshotter plugin. (#9290, @0xLeo258)
  * Backport to 1.17 (PR#9244 Update AzureAD Microsoft Authentication Library to v1.5.0) (#9285, @priyansh17)
  * Fix schedule controller to prevent backup queue accumulation during extended blocking scenarios by properly handling empty backup phases (#9277, @shubham-pampattiwar)
  * Get pod list once per namespace in pvc IBA (#9266, @sseago)
  * Update AzureAD Microsoft Authentication Library to v1.5.0 (#9244, @priyansh17)
  * feat: Permit specifying annotations for the BackupPVC (#9173, @clementnuss)


## v1.17

### Download
https://github.com/vmware-tanzu/velero/releases/tag/v1.17.0

### Container Image
`velero/velero:v1.17.0`

### Documentation
https://velero.io/docs/v1.17/

### Upgrading
https://velero.io/docs/v1.17/upgrade-to-1.17/

### Highlights
#### Modernized fs-backup
In v1.17, Velero fs-backup is modernized to the micro-service architecture, which brings below benefits:  
- Many features that were absent to fs-backup are now available, i.e., load concurrency control, cancel, resume on restart, etc.
- fs-backup is more robust, the running backup/restore could survive from node-agent restart; and the resource allocation is in a more granular manner, the failure of one backup/restore won't impact others.  
- The resource usage of node-agent is steady, especially, the node-agent pods won't request huge memory and hold it for a long time.  

Check design https://github.com/vmware-tanzu/velero/blob/main/design/vgdp-micro-service-for-fs-backup/vgdp-micro-service-for-fs-backup.md for more details.  

#### fs-backup support Windows cluster
In v1.17, Velero fs-backup supports to backup/restore Windows workloads. By leveraging the new micro-service architecture for fs-backup, data mover pods could run in Windows nodes and backup/restore Windows volumes. Together with CSI snapshot data movement for Windows which is delivered in 1.16, Velero now supports Windows workload backup/restore in full scenarios.  
Check design https://github.com/vmware-tanzu/velero/blob/main/design/vgdp-micro-service-for-fs-backup/vgdp-micro-service-for-fs-backup.md for more details.  

#### Volume group snapshot support
In v1.17, Velero supports [volume group snapshots](https://kubernetes.io/blog/2024/12/18/kubernetes-1-32-volume-group-snapshot-beta/) which is a beta feature in Kubernetes upstream, for both CSI snapshot backup and CSI snapshot data movement. This allows a snapshot to be taken from multiple volumes at the same point-in-time to achieve write order consistency, which is helpful to achieve better data consistency when multiple volumes being backed up are correlated.  
Check the document https://velero.io/docs/main/volume-group-snapshots/ for more details.  

#### Priority class support
In v1.17, [Kubernetes priority class](https://kubernetes.io/docs/concepts/scheduling-eviction/pod-priority-preemption/#priorityclass) is supported for all modules across Velero. Specifically, users are allowed to configure priority class to Velero server, node-agent, data mover pods, backup repository maintenance jobs separately.  
Check design https://github.com/vmware-tanzu/velero/blob/main/design/Implemented/priority-class-name-support_design.md for more details.  

#### Scalability and Resiliency improvements of data movers
##### Reduce excessive number of data mover pods in Pending state
In v1.17, Velero allows users to set a `PrepareQueueLength` in the node-agent configuration, data mover pods and volumes out of this number won't be created until data path quota is available, so that excessive number cluster resources won't  be taken unnecessarily, which is particularly helpful for large scale environments. This improvement applies to all kinds of data movements, including fs-backup and CSI snapshot data movement.  
Check design https://github.com/vmware-tanzu/velero/blob/main/design/node-agent-load-soothing.md for more details.  

##### Enhancement on node-agent restart handling for data movements
In v1.17, data movements in all phases could survive from node-agent restart and resume themselves; when a data movement gets orphaned in special cases, e.g., cluster node absent, it could also be canceled appropriately after the restart. This improvement applies to all kinds of data movements, including fs-backup and CSI snapshot data movement.  
Check issue https://github.com/vmware-tanzu/velero/issues/8534 for more details.  

##### CSI snapshot data movement restore node-selection and node-selection by storage class
In v1.17, CSI snapshot data movement restore acquires the same node-selection capability as backup, that is, users could specify which nodes can/cannot run data mover pods for both backup and restore now. And users are also allowed to configure the node-selection per storage class, which is particularly helpful to the environments where a storage class are not usable by all cluster nodes.  
Check issue https://github.com/vmware-tanzu/velero/issues/8186 and https://github.com/vmware-tanzu/velero/issues/8223 for more details.  

#### Include/exclude policy support for resource policy
In v1.17, Velero resource policy supports `includeExcludePolicy` besides the existing `volumePolicy`. This allows users to set include/exclude filters for resources in a resource policy configmap, so that these filters are reusable among multiple backups.  
Check the document https://velero.io/docs/main/resource-filtering/#creating-resource-policies:~:text=resources%3D%22*%22-,Resource%20policies,-Velero%20provides%20resource for more details.  

### Runtime and dependencies
Golang runtime: 1.24.6  
kopia: 0.21.1  

### Limitations/Known issues

### Breaking changes
#### Deprecation of Restic
According to [Velero deprecation policy](https://github.com/vmware-tanzu/velero/blob/main/GOVERNANCE.md#deprecation-policy), backup of fs-backup under Restic path is removed in v1.17, so `--uploader-type=restic` is not a valid installation configuration anymore. This means you cannot create a backup under Restic path, but you can still restore from the previous backups under Restic path until v1.19.  

#### Repository maintenance job configurations are removed from Velero server parameter
Since the repository maintenance job configurations are moved to repository maintenance job configMap, in v1.17 below Velero sever parameters are removed:
- --keep-latest-maintenance-jobs
- --maintenance-job-cpu-request
- --maintenance-job-mem-request
- --maintenance-job-cpu-limit
- --maintenance-job-mem-limit

### All Changes
  * Add ConfigMap parameters validation for install CLI and server start. (#9200, @blackpiglet)
  * Add priorityclasses to high priority restore list (#9175, @kaovilai)
  * Introduced context-based logger for backend implementations (Azure, GCS, S3, and Filesystem) (#9168, @priyansh17)
  * Fix issue #9140, add os=windows:NoSchedule toleration for Windows pods (#9165, @Lyndon-Li)
  * Remove the repository maintenance job parameters from velero server. (#9147, @blackpiglet)
  * Add include/exclude policy to resources policy (#9145, @reasonerjt)
  * Add ConfigMap support for keepLatestMaintenanceJobs with CLI parameter fallback (#9135, @shubham-pampattiwar)
  * Fix the dd and du's node affinity issue. (#9130, @blackpiglet)
  * Remove the WaitUntilVSCHandleIsReady from vs BIA. (#9124, @blackpiglet)
  * Add comprehensive Volume Group Snapshots documentation with workflow diagrams and examples (#9123, @shubham-pampattiwar)
  * Fix issue #9065, add doc for node-agent prepare queue length (#9118, @Lyndon-Li)
  * Fix issue #9095, update restore doc for PVC selected-node (#9117, @Lyndon-Li)
  * Update CSI Snapshot Data Movement doc for issue #8534, #8185 (#9113, @Lyndon-Li)
  * Fix issue #8986, refactor fs-backup doc after VGDP Micro Service for fs-backup (#9112, @Lyndon-Li)
  * Return error if timeout when checking server version (#9111, @ywk253100)
  * Update "Default Volumes to Fs Backup" to "File System Backup (Default)" (#9105, @shubham-pampattiwar)
  * Fix issue #9077, don't block backup deletion on list VS error (#9100, @Lyndon-Li)
  * Bump up Kopia to v0.21.1 (#9098, @Lyndon-Li)
  * Add imagePullSecrets inheritance for VGDP pod and maintenance job. (#9096, @blackpiglet)
  * Avoid checking the VS and VSC status in the backup finalizing phase. (#9092, @blackpiglet)
  * Fix issue #9053, Always remove selected-node annotation during PVC restore when no node mapping exists. Breaking change: Previously, the annotation was preserved if the node existed. (#9076, @Lyndon-Li)
  * Enable parameterized kubelet mount path during node-agent installation (#9074, @longxiucai)
  * Fix issue #8857, support third party tolerations for data mover pods (#9072, @Lyndon-Li)
  * Fix issue #8813, remove restic from the valid uploader type (#9069, @Lyndon-Li)
  * Fix issue #8185, allow users to disable pod volume host path mount for node-agent (#9068, @Lyndon-Li)
  * Fix #8344, add the design for a mechanism to soothe creation of data mover pods for DataUpload, DataDownload, PodVolumeBackup and PodVolumeRestore (#9067, @Lyndon-Li)
  * Fix #8344, add a mechanism to soothe creation of data mover pods for DataUpload, DataDownload, PodVolumeBackup and PodVolumeRestore (#9064, @Lyndon-Li)
  * Add Gauge metric for BSL availability (#9059, @reasonerjt)
  * Fix missing defaultVolumesToFsBackup flag output in Velero describe backup cmd (#9056, @shubham-pampattiwar)
  * Allow for proper tracking of multiple hooks per container (#9048, @sseago)
  * Make the backup repository controller doesn't invalidate the BSL on restart (#9046, @blackpiglet)
  * Removed username/password credential handling from newConfigCredential as azidentity.UsernamePasswordCredentialOptions is reported as deprecated. (#9041, @priyansh17)
  * Remove dependency with VolumeSnapshotClass in DataUpload. (#9040, @blackpiglet)
  * Fix issue #8961, cancel PVB/PVR on Velero server restart (#9031, @Lyndon-Li)
  * Fix issue #8962, resume PVB/PVR during node-agent restarts (#9030, @Lyndon-Li)
  * Bump kopia v0.20.1 (#9027, @Lyndon-Li)
  * Fix issue #8965, support PVB/PVR's cancel state in the backup/restore (#9026, @Lyndon-Li)
  * Fix Issue 8816 When specifying LabelSelector on restore, related items such as PVC and VolumeSnapshot are not included (#9024, @amastbau)
  * Fix issue #8963, add legacy PVR controller for Restic path (#9022, @Lyndon-Li)
  * Fix issue #8964, add Windows support for VGDP MS for fs-backup (#9021, @Lyndon-Li)
  * Accommodate VGS workflows in PVC CSI plugin (#9019, @shubham-pampattiwar)
  * Fix issue #8958, add VGDP MS PVB controller (#9015, @Lyndon-Li)
  * Fix issue #8959, add VGDP MS PVR controller (#9014, @Lyndon-Li)
  * Fix issue #8988, add data path for VGDP ms PVR (#9005, @Lyndon-Li)
  * Fix issue #8988, add data path for VGDP ms pvb (#8998, @Lyndon-Li)
  * Skip VS and VSC not created by backup. (#8990, @blackpiglet)
  * Make ResticIdentifier optional for kopia BackupRepositories (#8987, @kaovilai)
  * Fix issue #8960, implement PodVolume exposer for PVB/PVR (#8985, @Lyndon-Li)
  * fix: update mc command in minio-deployment example (#8982, @vishal-chdhry)
  * Fix issue #8957, add design for VGDP MS for fs-backup (#8979, @Lyndon-Li)
  * Add BSL status check for backup/restore operations. (#8976, @blackpiglet)
  * Mark BackupRepository not ready when BSL changed (#8975, @ywk253100)
  * Add support for [distributed snapshotting](https://github.com/kubernetes-csi/external-snapshotter/tree/4cedb3f45790ac593ebfa3324c490abedf739477?tab=readme-ov-file#distributed-snapshotting) (#8969, @flx5)
  * Fix issue #8534, refactor dm controllers to tolerate cancel request in more cases, e.g., node restart, node drain (#8952, @Lyndon-Li)
  * The backup and restore VGDP affinity enhancement implementation. (#8949, @blackpiglet)
  * Remove CSI VS and VSC metadata from backup. (#8946, @blackpiglet)
  * Extend PVCAction itemblock plugin to support grouping PVCs under VGS label key (#8944, @shubham-pampattiwar)
  * Copy security context from origin pod (#8943, @farodin91)
  * Add support for configuring VGS label key (#8938, @shubham-pampattiwar)
  * Add VolumeSnapshotContent into the RIA and the mustHave resource list. (#8924, @blackpiglet)
  * Mounted cloud credentials should not be world-readable (#8919, @sseago)
  * Warn for not found error in patching managed fields (#8902, @sseago)
  * Fix issue 8878, relief node os deduction error checks (#8891, @Lyndon-Li)
  * Skip namespace in terminating state in backup resource collection. (#8890, @blackpiglet)
  * Implement PriorityClass Support (#8883, @kaovilai)
  * Fix Velero adding restore-wait init container when not needed. (#8880, @kaovilai)
  * Pass the logger in kopia related operations. (#8875, @hu-keyu)
  * Inherit the dnsPolicy and dnsConfig from the node agent pod. This is done so that the kopia task uses the same configuration. (#8845, @flx5)
  * Add design for VolumeGroupSnapshot support (#8778, @shubham-pampattiwar)
  * Inherit k8s default volumeSnapshotClass. (#8719, @hu-keyu)
  * CLI automatically discovers and uses cacert from BSL for download requests (#8557, @kaovilai)
  * This PR aims to add s390x support to Velero binary. (#7505, @pandurangkhandeparker)