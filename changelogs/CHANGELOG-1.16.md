## v1.16.2

### Download
https://github.com/vmware-tanzu/velero/releases/tag/v1.16.2

### Container Image
`velero/velero:v1.16.2`

### Documentation
https://velero.io/docs/v1.16/

### Upgrading
https://velero.io/docs/v1.16/upgrade-to-1.16/

### All Changes
  * Update "Default Volumes to Fs Backup" to "File System Backup (Default)" (#9105, @shubham-pampattiwar)
  * Fix missing defaultVolumesToFsBackup flag output in Velero describe backup cmd (#9103, @shubham-pampattiwar)
  * Add imagePullSecrets inheritance for VGDP pod and maintenance job. (#9102, @blackpiglet)
  * Fix issue #9077, don't block backup deletion on list VS error (#9101, @Lyndon-Li)
  * Mounted cloud credentials should not be world-readable (#9094, @sseago)
  * Allow for proper tracking of multiple hooks per container (#9060, @sseago)
  * Add BSL status check for backup/restore operations. (#9010, @blackpiglet)


## v1.16.1

### Download
https://github.com/vmware-tanzu/velero/releases/tag/v1.16.1

### Container Image
`velero/velero:v1.16.1`

### Documentation
https://velero.io/docs/v1.16/

### Upgrading
https://velero.io/docs/v1.16/upgrade-to-1.16/

### All Changes
  * Call WaitGroup.Done() once only when PVB changes to final status the first time to avoid panic (#8940, @ywk253100)
  * Add VolumeSnapshotContent into the RIA and the mustHave resource list. (#8926, @blackpiglet)
  * Warn for not found error in patching managed fields (#8916, @sseago)
  * Fix issue 8878, relief node os deduction error checks (#8911, @Lyndon-Li)


## v1.16

### Download
https://github.com/vmware-tanzu/velero/releases/tag/v1.16.0

### Container Image
`velero/velero:v1.16.0`

### Documentation
https://velero.io/docs/v1.16/

### Upgrading
https://velero.io/docs/v1.16/upgrade-to-1.16/

### Highlights
#### Windows cluster support
In v1.16, Velero supports to run in Windows clusters and backup/restore Windows workloads, either stateful or stateless:
 * Hybrid build and all-in-one image: the build process is enhanced to build an all-in-one image for hybrid CPU architecture and hybrid platform. For more information, check the design https://github.com/vmware-tanzu/velero/blob/main/design/multiple-arch-build-with-windows.md
 * Deployment in Windows clusters: Velero node-agent, data mover pods and maintenance jobs now support to run in both linux and Windows nodes
 * Data mover backup/restore Windows workloads: Velero built-in data mover supports Windows workloads throughout its full cycle, i.e., discovery, backup, restore, pre/post hook, etc. It automatically identifies Windows workloads and schedules data mover pods to the right group of nodes

Check the epic issue https://github.com/vmware-tanzu/velero/issues/8289 for more information.  

#### Parallel Item Block backup
v1.16 now supports to back up item blocks in parallel. Specifically, during backup, correlated resources are grouped in item blocks and Velero backup engine creates a thread pool to back up the item blocks in parallel. This significantly improves the backup throughput, especially when there are large scale of resources.  
Pre/post hooks also belongs to item blocks, so will also run in parallel along with the item blocks.  
Users are allowed to configure the parallelism through the `--item-block-worker-count` Velero server parameter. If not configured, the default parallelism is 1.  

For more information, check issue https://github.com/vmware-tanzu/velero/issues/8334.  

#### Data mover restore enhancement in scalability
In previous releases, for each volume of WaitForFirstConsumer mode, data mover restore is only allowed to happen in the node that the volume is attached. This severely degrades the parallelism and the balance of node resource(CPU, memory, network bandwidth) consumption for data mover restore (https://github.com/vmware-tanzu/velero/issues/8044).  

In v1.16, users are allowed to configure data mover restores running and spreading evenly across all nodes in the cluster. The configuration is done through a new flag `ignoreDelayBinding` in node-agent configuration (https://github.com/vmware-tanzu/velero/issues/8242).  

#### Data mover enhancements in observability 
In 1.16, some observability enhancements are added:
 * Output various statuses of intermediate objects for failures of data mover backup/restore (https://github.com/vmware-tanzu/velero/issues/8267)
 * Output the errors when Velero fails to delete intermediate objects during clean up (https://github.com/vmware-tanzu/velero/issues/8125)

The outputs are in the same node-agent log and enabled automatically.  

#### CSI snapshot backup/restore enhancement in usability
In previous releases, a unnecessary VolumeSnapshotContent object is retained for each backup and synced to other clusters sharing the same backup storage location. And during restore, the retained VolumeSnapshotContent is also restored unnecessarily.  

In 1.16, the retained VolumeSnapshotContent is removed from the backup, so no unnecessary CSI objects are synced or restored.  

For more information, check issue https://github.com/vmware-tanzu/velero/issues/8725.  

#### Backup Repository Maintenance enhancement in resiliency and observability
In v1.16, some enhancements of backup repository maintenance are added to improve the observability and resiliency:
 * A new backup repository maintenance history section, called `RecentMaintenance`, is added to the BackupRepository CR. Specifically, for each BackupRepository, including start/completion time, completion status and error message. (https://github.com/vmware-tanzu/velero/issues/7810)
 * Running maintenance jobs are now recaptured after Velero server restarts. (https://github.com/vmware-tanzu/velero/issues/7753)
 * The maintenance job will not be launched for readOnly BackupStorageLocation. (https://github.com/vmware-tanzu/velero/issues/8238)
 * The backup repository will not try to initialize a new repository for readOnly BackupStorageLocation. (https://github.com/vmware-tanzu/velero/issues/8091)
 * Users now are allowed to configure the intervals of an effective maintenance in the way of `normalGC`, `fastGC` and `eagerGC`, through the `fullMaintenanceInterval` parameter in backupRepository configuration. (https://github.com/vmware-tanzu/velero/issues/8364)

#### Volume Policy enhancement of filtering volumes by PVC labels
In v1.16, Volume Policy is extended to support filtering volumes by PVC labels. (https://github.com/vmware-tanzu/velero/issues/8256).   

#### Resource Status restore per object
In v1.16, users are allowed to define whether to restore resource status per object through an annotation `velero.io/restore-status` set on the object. (https://github.com/vmware-tanzu/velero/issues/8204).  

#### Velero Restore Helper binary is merged into Velero image 
In v1.16, Velero banaries, i.e., velero, velero-helper and velero-restore-helper, are all included into the single Velero image. (https://github.com/vmware-tanzu/velero/issues/8484).  

### Runtime and dependencies
Golang runtime: 1.23.7  
kopia: 0.19.0

### Limitations/Known issues
#### Limitations of Windows support
  * fs-backup is not supported for Windows workloads and so fs-backup runs only in linux nodes for linux workloads
  * Backup/restore of NTFS extended attributes/advanced features are not supported, i.e., Security Descriptors, System/Hidden/ReadOnly attributes, Creation Time, NTFS Streams, etc.

### All Changes
  * Add third party annotation support for maintenance job, so that the declared third party annotations could be added to the maintenance job pods (#8812, @Lyndon-Li)
  * Fix issue #8803, use deterministic name to create backupRepository (#8808, @Lyndon-Li)
  * Refactor restoreItem and related functions to differentiate the backup resource name and the restore target resource name. (#8797, @blackpiglet)
  * ensure that PV is removed before VS is deleted (#8777, @ix-rzi)
  * host_pods should not be mandatory to node-agent (#8774, @mpryc)
  * Log doesn't show pv name, but displays %!s(MISSING) instead (#8771, @hu-keyu)
  * Fix issue #8754, add third party annotation support for data mover (#8770, @Lyndon-Li)
  * Add docs for volume policy with labels as a criteria (#8759, @shubham-pampattiwar)
  * Move pvc annotation removal from CSI RIA to regular PVC RIA (#8755, @sseago)
  * Add doc for maintenance history (#8747, @Lyndon-Li)
  * Fix issue #8733, add doc for restorePVC (#8737, @Lyndon-Li)
  * Fix issue #8426, add doc for Windows support (#8736, @Lyndon-Li)
  * Fix issue #8475, refactor build-from-source doc for hybrid image build (#8729, @Lyndon-Li)
  * Return directly if no pod volme backup are tracked (#8728, @ywk253100)
  * Fix issue #8706, for immediate volumes, there is no selected-node annotation on PVC, so deduce the attached node from VolumeAttachment CRs (#8715, @Lyndon-Li)
  * Add labels as a criteria for volume policy (#8713, @shubham-pampattiwar)
  * Copy SecurityContext from Containers[0] if present for PVR (#8712, @sseago)
  * Support pushing images to an insecure registry (#8703, @ywk253100)
  * Modify golangci configuration to make it work. (#8695, @blackpiglet)
  * Run backup post hooks inside ItemBlock synchronously (#8694, @ywk253100)
  * Add docs for object level status restore (#8693, @shubham-pampattiwar)
  * Clean artifacts generated during CSI B/R. (#8684, @blackpiglet)
  * Don't run maintenance on the ReadOnly BackupRepositories. (#8681, @blackpiglet)
  * Fix #8657: WaitGroup panic issue (#8679, @ywk253100)
  * Fixes issue #8214, validate `--from-schedule` flag in create backup command to prevent empty or whitespace-only values. (#8665, @aj-2000)
  * Implement parallel ItemBlock processing via backup_controller goroutines (#8659, @sseago)
  * Clean up leaked CSI snapshot for incomplete backup (#8637, @raesonerjt)
  * Handle update conflict when restoring the status (#8630, @ywk253100)
  * Fix issue #8419, support repo maintenance job to run on Windows nodes (#8626, @Lyndon-Li)
  * Always create DataUpload configmap in restore namespace (#8621, @sseago)
  * Fix issue #8091, avoid to create new repo when BSL is readonly (#8615, @Lyndon-Li)
  * Fix issue #8242, distribute dd evenly across nodes (#8611, @Lyndon-Li)
  * Fix issue #8497, update du/dd progress on completion (#8608, @Lyndon-Li)
  * Fix issue #8418, add Windows toleration to data mover pods (#8606, @Lyndon-Li)
  * Check the PVB status via podvolume Backupper rather than calling API server to avoid API server issue (#8603, @ywk253100)
  * Fix issue #8067, add tmp folder (/tmp for linux, C:\Windows\Temp for Windows) as an alternative of udmrepo's config file location (#8602, @Lyndon-Li)
  * Data mover restore for Windows (#8594, @Lyndon-Li)
  * Skip patching the PV in finalization for failed operation (#8591, @reasonerjt)
  * Fix issue #8579, set event burst to block event broadcaster from filtering events (#8590, @Lyndon-Li)
  * Configurable Kopia Maintenance Interval. backup-repository-configmap adds an option for configurable`fullMaintenanceInterval` where fastGC (12 hours), and eagerGC (6 hours) allowing for faster removal of deleted velero backups from kopia repo. (#8581, @kaovilai)
  * Fix issue #7753, recall repo maintenance history on Velero server restart (#8580, @Lyndon-Li)
  * Clear validation errors when schedule is valid (#8575, @ywk253100)
  * Merge restore helper image into Velero server image (#8574, @ywk253100)
  * Don't include excluded items in ItemBlocks (#8572, @sseago)
  * fs uploader and block uploader support Windows nodes (#8569, @Lyndon-Li)
  * Fix issue #8418, support data mover backup for Windows nodes (#8555, @Lyndon-Li)
  * Fix issue #8044, allow users to ignore delay binding the restorePVC of data mover when it is in WaitForFirstConsumer mode (#8550, @Lyndon-Li)
  * Fix issue #8539, validate uploader types when o.CRDsOnly is set to false only since CRD installation doesn't rely on uploader types (#8538, @Lyndon-Li)
  * Fix issue #7810, add maintenance history for backupRepository CRs (#8532, @Lyndon-Li)
  * Make fs-backup work on linux nodes with the new Velero deployment and disable fs-backup if the source/target pod is running in non-linux node (#8424) (#8518, @Lyndon-Li)
  * Fix issue: backup schedule pause/unpause doesn't work (#8512, @ywk253100)
  * Fix backup post hook issue #8159 (caused by #7571): always execute backup post hooks after PVBs are handled (#8509, @ywk253100)
  * Fix issue #8267, enhance the error message when expose fails (#8508, @Lyndon-Li)
  * Fix issue #8416, #8417, deploy Velero server and node-agent in linux/Windows hybrid env (#8504, @Lyndon-Li)
  * Design to add label selector as a criteria for volume policy (#8503, @shubham-pampattiwar)
  * Related to issue #8485, move the acceptedByNode and acceptedTimestamp to Status of DU/DD CRD (#8498, @Lyndon-Li)
  * Add SecurityContext to restore-helper (#8491, @reasonerjt)
  * Fix issue #8433, add third party labels to data mover pods when the same labels exist in node-agent pods (#8487, @Lyndon-Li)
  * Fix issue #8485, add an accepted time so as to count the prepare timeout (#8486, @Lyndon-Li)
  * Fix issue #8125, log diagnostic info for data mover exposers when expose timeout (#8482, @Lyndon-Li)
  * Fix issue #8415, implement multi-arch build and Windows build (#8476, @Lyndon-Li)
  * Pin kopia to 0.18.2 (#8472, @Lyndon-Li)
  * Add nil check for updating DataUpload VolumeInfo in finalizing phase (#8471, @blackpiglet)
  * Allowing Object-Level Resource Status Restore (#8464, @shubham-pampattiwar)
  * For issue #8429. Add the design for multi-arch build and windows build (#8459, @Lyndon-Li)
  * Upgrade go.mod k8s.io/ go.mod to v0.31.3 and implemented proper logger configuration for both client-go and controller-runtime libraries. This change ensures that logging format and level settings are properly applied throughout the codebase. The update improves logging consistency and control across the Velero system. (#8450, @kaovilai)
  * Add Design for Allowing Object-Level Resource Status Restore (#8403, @shubham-pampattiwar)
  * Fix issue #8391, check ErrCancelled from suffix of data mover pod's termination message (#8396, @Lyndon-Li)
  * Fix issue #8394, don't call closeDataPath in VGDP callbacks, otherwise, the VGDP cleanup will hang (#8395, @Lyndon-Li)
  * Adding support in velero Resource Policies for filtering PVs based on additional VolumeAttributes properties under CSI PVs (#8383, @mayankagg9722)
  * Add --item-block-worker-count flag to velero install and server (#8380, @sseago)
  * Make BackedUpItems thread safe (#8366, @sseago)
  * Include --annotations flag in backup and restore create commands (#8354, @alromeros)
  * Use aggregated discovery API to discovery API groups and resources (#8353, @ywk253100)
  * Copy "envFrom" from Velero server when creating maintenance jobs (#8343, @evhan)
  * Set hinting region to use for GetBucketRegion() in pkg/repository/config/aws.go (#8297, @kaovilai)
  * Bump up version of client-go and controller-runtime (#8275, @ywk253100)
  * fix(pkg/repository/maintenance): don't panic when there's no container statuses (#8271, @mcluseau)
  * Add Backup warning for inclusion of NS managed by ArgoCD (#8257, @shubham-pampattiwar)
  * Added tracking for deleted namespace status check in restore flow. (#8233, @sangitaray2021)