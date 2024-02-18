## v1.13
### 2024-01-10

### Download
https://github.com/vmware-tanzu/velero/releases/tag/v1.13.0

### Container Image
`velero/velero:v1.13.0`

### Documentation
https://velero.io/docs/v1.13/

### Upgrading
https://velero.io/docs/v1.13/upgrade-to-1.13/

### Highlights

#### Resource Modifier Enhancement
Velero introduced the Resource Modifiers in v1.12.0. This feature allows users to specify a ConfigMap with a set of rules to modify the resources during restoration. However, only the JSON Patch is supported when creating the rules, and JSON Patch has some limitations, which cannot cover all use cases. In v1.13.0, Velero adds new support for JSON Merge Patch and Strategic Merge Patch, which provide more power and flexibility and allow users to use the same ConfigMap to apply patches on the resources. More design details can be found in [Support JSON Merge Patch and Strategic Merge Patch in Resource Modifiers](https://github.com/vmware-tanzu/velero/blob/main/design/Implemented/merge-patch-and-strategic-in-resource-modifier.md) design. For instructions on how to use the feature, please refer to the [Resource Modifiers](https://velero.io/docs/v1.13/restore-resource-modifiers/) doc.

#### Node-Agent Concurrency
Velero data movement activities from fs-backups and CSI snapshot data movements run in Velero node-agent, so may be hosted by every node in the cluster and consume resources (i.e. CPU, memory, network bandwidth) from there. With v1.13, users are allowed to configure how many data movement activities (a.k.a, loads) run in each node globally or by node, so that users can better leverage the performance of Velero data movement activities and the resource consumption in the cluster. For more information, check the [Node-Agent Concurrency](https://velero.io/docs/v1.13/node-agent-concurrency/) document. 

#### Parallel Files Upload Options
Velero now supports configurable options for parallel files upload when using Kopia uploader to do fs-backups or CSI snapshot data movements which makes speed up backup possible.
For more information, please check [Here](https://velero.io/docs/v1.13/backup-reference/#parallel-files-upload).

#### Write Sparse Files Options
If using fs-restore or CSI snapshot data movements, it’s supported to write sparse files during restore. For more information, please check [Here](https://velero.io/docs/v1.13/restore-reference/#write-sparse-files).

#### Backup Describe
In v1.13, the Backup Volume section is added to the velero backup describe command output. The backup Volumes section describes information for all the volumes included in the backup of various backup types, i.e. native snapshot, fs-backup, CSI snapshot, and CSI snapshot data movement. Particularly, the velero backup description now supports showing the information of CSI snapshot data movements, which is not supported in v1.12.

Additionally, backup describe command will not check EnableCSI feature gate from client side, so if a backup has volumes with CSI snapshot or CSI snapshot data movement, backup describe command always shows the corresponding information in its output.

#### Backup's new VolumeInfo metadata
Create a new metadata file in the backup repository's backup name sub-directory to store the backup-including PVC and PV information. The information includes the backing-up method of the PVC and PV data, snapshot information, and status. The VolumeInfo metadata file determines how the PV resource should be restored. The Velero downstream software can also use this metadata file to get a summary of the backup's volume data information.

#### Enhancement for CSI Snapshot Data Movements when Velero Pod Restart
When performing backup and restore operations, enhancements have been implemented for Velero server pods or node agents to ensure that the current backup or restore process is not stuck or interrupted after restart due to certain exceptional circumstances.

#### New status fields added to show hook execution details
Hook execution status is now included in the backup/restore CR status and displayed in the backup/restore describe command output. Specifically, it will show the number of hooks which attempted to execute under the HooksAttempted  field and the number of hooks which failed to execute under the HooksFailed  field. 

#### AWS SDK Bump Up
Bump up AWS SDK for Go to version 2, which offers significant performance improvements in CPU and memory utilization over version 1.

#### Azure AD/Workload Identity Support
Azure AD/Workload Identity is the recommended approach to do the authentication with Azure services/AKS, Velero has introduced support for Azure AD/Workload Identity on the Velero Azure plugin side in previous releases, and in v1.13.0 Velero adds new support for Kopia operations(file system backup/data mover/etc.) with Azure AD/Workload Identity.

#### Runtime and dependencies
To fix CVEs and keep pace with Golang, Velero made changes as follows:
* Bump Golang runtime to v1.21.6.
* Bump several dependent libraries to new versions.
* Bump Kopia to v0.15.0.


### Breaking changes
* Backup describe command: due to the backup describe output enhancement, some existing information (i.e. the output for native snapshot, CSI snapshot, and fs-backup) has been moved to the Backup Volumes section with some format changes.
* API type changes: changes the field [DataMoverConfig](https://github.com/vmware-tanzu/velero/blob/v1.13.0/pkg/apis/velero/v2alpha1/data_upload_types.go#L54) in DataUploadSpec from `*map[string][string]`` to `map[string]string`
* Velero install command: due to the issue [#7264](https://github.com/vmware-tanzu/velero/issues/7264), v1.13.0 introduces a break change that make the informer cache enabled by default to keep the actual behavior consistent with the helper message(the informer cache is disabled by default before the change).


### Limitations/Known issues
* The backup's VolumeInfo metadata doesn't have the information updated in the async operations. This function could be supported in v1.14 release. 

### Note
* Velero introduces the informer cache which is enabled by default. The informer cache improves the restore performance but may cause higher memory consumption. Increase the memory limit of the Velero pod or disable the informer cache by specifying the `--disable-informer-cache` option when installing Velero if you get the OOM error.

### Deprecation announcement
* The generated k8s clients, informers, and listers are deprecated in the Velero v1.13 release. They are put in the Velero repository's pkg/generated directory. According to the n+2 supporting policy, the deprecated are kept for two more releases. The pkg/generated directory should be deleted in the v1.15 release.
* After the backup VolumeInfo metadata file is added to the backup, Velero decides how to restore the PV resource according to the VolumeInfo content. To support the backup generated by the older version of Velero, the old logic is also kept. The support for the backup without the VolumeInfo metadata file will be kept for two releases. The support logic will be deleted in the v1.15 release.

### All Changes
  * Make "disable-informer-cache" option false(enabled) by default to keep it consistent with the help message (#7294, @ywk253100)
  * Fix issue #6928, remove snapshot deletion timeout for PVB (#7282, @Lyndon-Li)
  * Do not set "targetNamespace" to namespace items (#7274, @reasonerjt)
  * Fix issue #7244. By the end of the upload, check the outstanding incomplete snapshots and delete them by calling ApplyRetentionPolicy (#7245, @Lyndon-Li)
  * Adjust the newline output of resource list in restore describer (#7238, @allenxu404)
  * Remove the redundant newline in backup describe output (#7229, @allenxu404)
  * Fix issue #7189, data mover generic restore - don't assume the first volume as the restore volume (#7201, @Lyndon-Li)
  * Update CSIVolumeSnapshotsCompleted in backup's status and the metric
during backup finalize stage according to async operations content. (#7184, @blackpiglet)
  * Refactor DownloadRequest Stream function (#7175, @blackpiglet)
  * Add `--skip-immediately` flag to schedule commands; `--schedule-skip-immediately` server and install (#7169, @kaovilai)
  * Add node-agent concurrency doc and change the config name from dataPathConcurrency to loadCocurrency (#7161, @Lyndon-Li)
  * Enhance hooks tracker by adding a returned error to record function (#7153, @allenxu404)
  * Track the skipped PV when SnapshotVolumes set as false (#7152, @reasonerjt)
  * Add more linters part 2. (#7151, @blackpiglet)
  * Fix issue #7135, check pod status before checking node-agent pod status (#7150, @Lyndon-Li)
  * Treat namespace as a regular restorable item (#7143, @reasonerjt)
  * Allow sparse option for Kopia & Restic restore  (#7141, @qiuming-best)
  * Use VolumeInfo to help restore the PV. (#7138, @blackpiglet)
  * Node agent restart enhancement (#7130, @qiuming-best)
  * Fix issue #6695, add describe for data mover backups (#7125, @Lyndon-Li)
  * Add hooks status to backup/restore CR (#7117, @allenxu404)
  * Include plugin name in the error message by operations (#7115, @reasonerjt)
  * Fix issue #7068, due to a behavior of CSI external snapshotter, manipulations of VS and VSC may not be handled in the same order inside external snapshotter as the API is called. So add a protection finalizer to ensure the order (#7102, @Lyndon-Li)
  * Generate VolumeInfo for backup. (#7100, @blackpiglet)
  * Fix issue #7094, fallback to full backup if previous snapshot is not found (#7096, @Lyndon-Li)
  * Fix issue #7068, due to an behavior of CSI external snapshotter, manipulations of VS and VSC may not be handled in the same order inside external snapshotter as the API is called. So add a protection finalizer to ensure the order (#7095, @Lyndon-Li)
  * Skip syncing the backup which doesn't contain backup metadata (#7081, @ywk253100)
  * Fix issue #6693, partially fail restore if CSI snapshot is involved but CSI feature is not ready, i.e., CSI feature gate is not enabled or CSI plugin is not installed. (#7077, @Lyndon-Li)
  * Truncate the credential file to avoid the change of secret content messing it up (#7072, @ywk253100)
  * Add VolumeInfo metadata structures. (#7070, @blackpiglet)
  * improve discoveryHelper.Refresh() in restore (#7069, @27149chen)
  * Add DataUpload Result and CSI VolumeSnapshot check for restore PV. (#7061, @blackpiglet)
  * Add the implementation for design #6950, configurable data path concurrency (#7059, @Lyndon-Li)
  * Make data mover fail early (#7052, @qiuming-best)
  * Remove dependency of generated client part 3. (#7051, @blackpiglet)
  * Update Backup.Status.CSIVolumeSnapshotsCompleted during finalize (#7046, @kaovilai)
  * Remove the Velero generated client. (#7041, @blackpiglet)
  * Fix issue #7027, data mover backup exposer should not assume the first volume as the backup volume in backup pod (#7038, @Lyndon-Li)
  * Read information from the credential specified by BSL (#7034, @ywk253100)
  * Fix #6857. Added check for matching Owner References when synchronizing backups, removing references that are not found/have mismatched uid. (#7032, @deefdragon)
  * Add description markers for dataupload and datadownload CRDs (#7028, @shubham-pampattiwar)
  * Add HealthCheckNodePort deletion logic for Service restore. (#7026, @blackpiglet)
  * Fix inconsistent behavior of Backup and Restore hook execution (#7022, @allenxu404)
  * Fix #6964. Don't use csiSnapshotTimeout (10 min) for waiting snapshot to readyToUse for data mover, so as to make the behavior complied with CSI snapshot backup (#7011, @Lyndon-Li)
  * restore: Use warning when Create IsAlreadyExist and Get error (#7004, @kaovilai)
  * Bump kopia to 0.15.0 (#7001, @Lyndon-Li)
  * Make Kopia file parallelism configurable (#7000, @qiuming-best)
  * Fix unified repository (kopia) s3 credentials profile selection (#6995, @kaovilai)
  * Fix #6988, always get region from BSL if it is not empty (#6990, @Lyndon-Li)
  * Limit PVC block mode logic to non-Windows platform. (#6989, @blackpiglet)
  * It is a valid case that the Status.RestoreSize field in VolumeSnapshot is not set, if so, get the volume size from the source PVC to create the backup PVC (#6976, @Lyndon-Li)
  * Check whether the action is a CSI action and whether CSI feature is enabled, before executing the action. (#6968, @blackpiglet)
  * Add the PV backup information design document. (#6962, @blackpiglet)
  * Change controller-runtime List option from MatchingFields to ListOptions (#6958, @blackpiglet)
  * Add the design for node-agent concurrency (#6950, @Lyndon-Li)
  * Import auth provider plugins (#6947, @0x113)
  * Fix #6668, add a limitation for file system restore parallelism with other types of restores (CSI snapshot restore, CSI snapshot movement restore) (#6946, @Lyndon-Li)
  * Add MSI Support for Azure plugin. (#6938, @yanggangtony)
  * Partially fix #6734, guide Kubernetes' scheduler to spread backup pods evenly across nodes as much as possible, so that data mover backup could achieve better parallelism (#6926, @Lyndon-Li)
  * Bump up aws sdk to aws-sdk-go-v2 (#6923, @reasonerjt)
  * Optional check if targeted container is ready before executing a hook (#6918, @Ripolin)
  * Support JSON Merge Patch and Strategic Merge Patch in Resource Modifiers (#6917, @27149chen)
  * Fix issue 6913: Velero Built-in Datamover: Backup stucks in phase WaitingForPluginOperations when Node Agent pod gets restarted (#6914, @shubham-pampattiwar)
  * Set ParallelUploadAboveSize as MaxInt64 and flush repo after setting up policy so that policy is retrieved correctly by TreeForSource (#6885, @Lyndon-Li)
  * Replace the base image with paketobuildpacks image (#6883, @ywk253100)
  * Fix issue #6859, move plugin depending podvolume functions to util pkg, so as to remove the dependencies to unnecessary repository packages like kopia, azure, etc. (#6875, @Lyndon-Li)
  * Fix #6861. Only Restic path requires repoIdentifier, so for non-restic path, set the repoIdentifier fields as empty in PVB and PVR and also remove the RepoIdentifier column in the get output of PVBs and PVRs (#6872, @Lyndon-Li)
  * Add volume types filter in resource policies (#6863, @qiuming-best)
  * change the metrics backup_attempt_total default value to 1. (#6838, @yanggangtony)
  * Bump kopia to v0.14 (#6833, @Lyndon-Li)
  * Retry failed create when using generateName (#6830, @sseago)
  * Fix issue #6786, always delete VSC regardless of the deletion policy (#6827, @Lyndon-Li)
  * Proposal to support JSON Merge Patch and Strategic Merge Patch in Resource Modifiers (#6797, @27149chen)
  * Fix the node-agent missing metrics-address defines. (#6784, @yanggangtony)
  * Fix default BSL setting not work (#6771, @qiuming-best)
  * Update restore controller logic for restore deletion (#6770, @ywk253100)
  * Fix #6752: add namespace exclude check. (#6760, @blackpiglet)
  * Fix issue #6753, remove the check for read-only BSL in restore async operation controller since Velero cannot fully support read-only mode BSL in restore at present (#6757, @Lyndon-Li)
  * Fix issue #6647, add the --default-snapshot-move-data parameter to Velero install, so that users don't need to specify --snapshot-move-data per backup when they want to move snapshot data for all backups (#6751, @Lyndon-Li)
  * Use old(origin) namespace in resource modifier conditions in case namespace may change during restore (#6724, @27149chen)
  * Perf improvements for existing resource restore (#6723, @sseago)
  * Remove schedule-related metrics on schedule delete (#6715, @nilesh-akhade)
  * Kubernetes 1.27 new job label batch.kubernetes.io/controller-uid are deleted during restore per https://github.com/kubernetes/kubernetes/pull/114930 (#6712, @kaovilai)
  * This pr made some improvements in Resource Modifiers: 1. add label selector 2. change the field name from groupKind to groupResource (#6704, @27149chen)
  * Make Kopia support Azure AD (#6686, @ywk253100)
  * Add support for block volumes with Kopia (#6680, @dzaninovic)
  * Delete PartiallyFailed orphaned backups as well as Completed ones (#6649, @sseago)
  * Add CSI snapshot data movement doc (#6637, @Lyndon-Li)
  * Fixes #6636, skip subresource in resource discovery (#6635, @27149chen)
  * Add `orLabelSelectors` for backup, restore commands (#6475, @nilesh-akhade)
  * fix run preHook and postHook on completed pods (#5211, @cleverhu)