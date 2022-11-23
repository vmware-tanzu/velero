  ## v1.10.0
### 2022-11-23

### Download
https://github.com/vmware-tanzu/velero/releases/tag/v1.10.0

### Container Image
`velero/velero:v1.10.0`

### Documentation
https://velero.io/docs/v1.10/

### Upgrading
https://velero.io/docs/v1.10/upgrade-to-1.10/

### Highlights

#### Unified Repository and Kopia integration
In this release, we introduced the Unified Repository architecture to build a data path where data movers and the backup repository are decoupled and a unified backup repository could serve various data movement activities.

In this release, we also deeply integrate Velero with Kopia, specifically, Kopia's uploader modules are isolated as a generic file system uploader; Kopia's repository modules are encapsulated as the unified backup repository.

For more information, refer to the [design document](https://github.com/vmware-tanzu/velero/blob/v1.10.0/design/unified-repo-and-kopia-integration/unified-repo-and-kopia-integration.md).

#### File system backup refactor
Velero's file system backup (a.k.s. pod volume backup or formerly restic backup) is refactored as the first user of the Unified Repository architecture. Specifically, we added a new path, the Kopia path, besides the existing Restic path. While Restic path is still available and set as default, you can opt in Kopia path by specifying the `uploader-type` parameter at installation time. Meanwhile, you are free to restore from existing backups under either path, Velero dynamically switches to the correct path to process the restore.

Because of the new path, we renamed some modules and parameters, refer to the Break Changes section for more details.

For more information, visit the [file system backup document](https://velero.io/docs/v1.10/file-system-backup/) and [v1.10 upgrade guide document](https://velero.io/docs/v1.10/upgrade-to-1.10/).

Meanwhile, we've created a performance guide for both Restic path and Kopia path, which helps you to choose between the two paths and provides you the best practice to configure them under different scenarios. Please note that the results in the guide are based on our testing environments, you may get different results when testing in your own ones. For more information, visit the [performance guide document](https://velero.io/docs/v1.10/performance-guidance/).

#### Plugin versioning V1 refactor
In this release, Velero moves plugins BackupItemAction, RestoreItemAction and VolumeSnapshotterAction to version v1, this allows future plugin changes that do not support backward compatibility, so is a preparation for various complex tasks, for example, data movement tasks.
For more information, refer to the [plugin versioning design document](https://github.com/vmware-tanzu/velero/blob/v1.10.0/design/plugin-versioning.md).

#### Refactor the controllers using Kubebuilder v3
In this release we continued our code modernization work, rewriting some controllers using Kubebuilder v3. This work is ongoing and we will continue to make progress in future releases.

#### Add credentials to volume snapshot locations
In this release, we enabled dedicate credentials options to volume snapshot locations so that you can specify credentials per volume snapshot location as same as backup storage location.

For more information, please visit the [locations document](https://velero.io/docs/v1.10/locations/).

#### CSI snapshot enhancements
In this release we added several changes to enhance the robustness of CSI snapshot procedures, for example, some protection code for error handling, and a mechanism to skip exclusion checks so that CSI snapshot works with various backup resource filters.

#### Backup schedule pause/unpause
In this release, Velero supports to pause/unpause a backup schedule during or after its creation. Specifically:

At creation time, you can specify `–paused` flag to `velero schedule create` command, if so, you will create a paused schedule that will not run until it is unpaused
After creation, you can run `velero schedule pause` or `velero schedule unpause` command to pause/unpause a schedule

#### Runtime and dependencies
In order to fix CVEs, we changed Velero's runtime and dependencies as follows:

Bump go runtime to v1.18.8
Bump some core dependent libraries to newer versions
Compile Restic (v0.13.1) with go 1.18.8 instead of packaging the official binary


#### Breaking changes
Due to file system backup refactor, below modules and parameters name have been changed in this release:

`restic` daemonset is renamed to `node-agent`
`resticRepository` CR is renamed to `backupRepository`
`velero restic repo` command is renamed to `velero repo`
`velero-restic-credentials` secret is renamed to `velero-repo-credentials`
`default-volumes-to-restic` parameter is renamed to `default-volumes-to-fs-backup`
`restic-timeout` parameter is renamed to `fs-backup-timeout`
`default-restic-prune-frequency` parameter is renamed to `default-repo-maintain-frequency`

#### Upgrade
Due to the major changes of file system backup, the old upgrade steps are not suitable any more. For the new upgrade steps, visit [v1.10 upgrade guide document](https://velero.io/docs/v1.10/upgrade-to-1.10/).

#### Limitations/Known issues
In this release, Kopia backup repository (so the Kopia path of file system backup) doesn't support self signed certificate for S3 compatible storage. To track this problem, refer to this [Velero issue](https://github.com/vmware-tanzu/velero/issues/5123) or [Kopia issue](https://github.com/kopia/kopia/issues/1443). 

Due to the code change in Velero, there will be some code change required in vSphere plugin, without which the functionality may be impacted.  Therefore, if you are using vSphere plugin in your workflow, please hold the upgrade until the issue [#485](https://github.com/vmware-tanzu/velero-plugin-for-vsphere/issues/485) is fixed in vSphere plugin.

### All changes
  
  * Restore ClusterBootstrap before Cluster otherwise a new default ClusterBootstrap object is create for the cluster  (#5616, @ywk253100)
  * Add compile restic binary for CVE fix  (#5574, @qiuming-best)
  * Fix controller problematic log output  (#5572, @qiuming-best)
  * Enhance the restore priorities list to support specifying the low prioritized resources that need to be restored in the last (#5535, @ywk253100)
  * fix restic backup progress error (#5534, @qiuming-best)
  * fix restic backup failure with self-signed certification backend storage (#5526, @qiuming-best)
  * Add credential store in backup deletion controller to support VSL credential. (#5521, @blackpiglet)
  * Fix issue 5505: the pod volume backups/restores except the first one fail under the kopia path if "AZURE_CLOUD_NAME" is specified (#5512, @Lyndon-Li)
  * After Pod Volume Backup/Restore refactor, remove all the unreasonable appearance of "restic" word from documents (#5499, @Lyndon-Li)
  * Refactor Pod Volume Backup/Restore doc to match the new behavior (#5484, @Lyndon-Li)
  * Remove redundancy code block left by #5388. (#5483, @blackpiglet)
  * Issue fix 5477: create the common way to support S3 compatible object storages that work for both Restic and Kopia; Keep the resticRepoPrefix parameter for compatibility (#5478, @Lyndon-Li)
  * Update the k8s.io dependencies to 0.24.0.
This also required an update to github.com/bombsimon/logrusr/v3.
Removed the `WithClusterName` method
as it is a "legacy field that was
always cleared by the system and never used" as per upstream k8s
https://github.com/kubernetes/apimachinery/blob/release-1.24/pkg/apis/meta/v1/types.go#L257-L259 (#5471, @kcboyle)
  * Add v1.10 velero upgrade doc (#5468, @qiuming-best)
  * Upgrade velero docker image to use go 1.18 and upgrade golangci-lint to 1.45.0 (#5459, @Lyndon-Li)
  * Add VolumeSnapshot client back. (#5449, @blackpiglet)
  * Change subcommand `velero restic repo` to `velero repo` (#5446, @allenxu404)
  * Remove irrational "Restic" names in Velero code after the PVBR refactor (#5444, @Lyndon-Li)
  * moved RIA execute input/output structs back to velero package (#5441, @sseago)
  * Rename Velero pod volume restore init helper from "velero-restic-restore-helper" to "velero-restore-helper" (#5432, @Lyndon-Li)
  * Skip the exclusion check for additional resources returned by BIA (#5429, @reasonerjt)
  * Change B/R describe CLI to support Kopia (#5412, @allenxu404)
  * Add nil check before execution of csi snapshot delete (#5401, @shubham-pampattiwar)
  * update velero using klog to version v2.9.0 (#5396, @blackpiglet)
  * Fix Test_prepareBackupRequest_BackupStorageLocation UT failure. (#5394, @blackpiglet)
  * Rename Velero daemonset from "restic" to "node-agent" (#5390, @Lyndon-Li)
  * Add some corner cases checking for CSI snapshot in backup controller. (#5388, @blackpiglet)
  * Fix issue 5386: Velero providers a full URL as the S3Url while the underlying minio client only accept the host part of the URL as the endpoint and the schema should be specified separately. (#5387, @Lyndon-Li)
  * Fix restore error with flag namespace-mappings (#5377, @qiuming-best)
  * Pod Volume Backup/Restore Refactor: Rename parameters in CRDs and commands to remove "Restic" word (#5370, @Lyndon-Li)
  * Added backupController's UT to test the prepareBackupRequest() method BackupStorageLocation processing logic (#5362, @niulechuan)
  * Fix a repoEnsurer problem introduced by the refactor - The repoEnsurer didn't check "" state of BackupRepository, as a result, the function GetBackupRepository always returns without an error even though the ensreReady is specified. (#5359, @Lyndon-Li)
  * Add E2E test for schedule backup (#5355, @danfengliu)
  * Add useOwnerReferencesInBackup field doc for schedule. (#5353, @cleverhu)
  * Clarify the help message for the default value of parameter --snapshot-volumes, when it's not set. (#5350, @blackpiglet)
  * Fix restore cmd extraflag overwrite bug (#5347, @qiuming-best)
  * Resolve gopkg.in/yaml.v3 vulnerabilities by upgrading gopkg.in/yaml.v3 to v3.0.1 (#5344, @kaovilai)
  * Increase ensure restic repository timeout to 5m (#5335, @shubham-pampattiwar)
  * Add opt-in and opt-out PersistentVolume backup to E2E tests (#5331, @danfengliu)
  * Cancel downloadRequest when timeout without downloadURL (#5329, @kaovilai)
  * Fix PVB finds wrong parent snapshot (#5322, @qiuming-best)
  * Fix issue 4874 and 4752: check the daemonset pod is running in the node where the workload pod resides before running the PVB for the pod (#5319, @Lyndon-Li)
  * plugin versioning v1 refactor for VolumeSnapshotter (#5318, @sseago)
  * Change the status of restore to completed from partially failed when restore empty backup (#5314, @allenxu404)
  * RestoreItemAction v1 refactoring for plugin api versioning (#5312, @sseago)
  * Refactor the repoEnsurer code to use controller runtime client and wrap some common BackupRepository operations to share with other modules (#5308, @Lyndon-Li)
  * Remove snapshot related lister, informer and client from backup controller. (#5299, @jxun)
  * Remove github.com/apex/log logger. (#5297, @blackpiglet)
  * change CSISnapshotTimeout from pointer to normal variables. (#5294, @cleverhu)
  * Optimize code for restore exists resources. (#5293, @cleverhu)
  * Add more detailed comments for labels columns. (#5291, @cleverhu)
  * Add backup status checking in schedule controller. (#5283, @blackpiglet)
  * Add changes for problems/enhancements found during smoking test for Kopia pod volume backup/restore (#5282, @Lyndon-Li)
  * Support pause/unpause schedules (#5279, @ywk253100)
  *  plugin/clientmgmt refactoring for BackupItemAction v1 (#5271, @sseago)
  * Don't move velero v1 plugins to new proto dir (#5263, @sseago)
  * Fill gaps for Kopia path of PVBR: integrate Repo Manager with Unified Repo; pass UploaderType to PVBR backupper and restorer; pass RepositoryType to BackupRepository controller and Repo Ensurer (#5259, @Lyndon-Li)
  * Add csiSnapshotTimeout for describe backup (#5252, @cleverhu)
  * equip gc controller with configurable frequency (#5248, @allenxu404)
  * Fix nil pointer panic when restoring StatefulSets (#5247, @divolgin)
  * Controller refactor code modifications. (#5241, @jxun)
  * Fix edge cases for already exists resources (#5239, @shubham-pampattiwar)
  * Check for empty ns list before checking nslist[0] (#5236, @sseago)
  * Remove reference to non-existent doc (#5234, @reasonerjt)
  * Add changes for Kopia Integration: Kopia Lib - method implementation. Add changes to write Kopia Repository logs to Velero log (#5233, @Lyndon-Li)
  * Add changes for Kopia Integration: Kopia Lib - initialize Kopia repo (#5231, @Lyndon-Li)
  * Uploader Implementation: Kopia backup and restore (#5221, @qiuming-best)
  * Migrate backup sync controller from code-generator to kubebuilder. (#5218, @jxun)
  * check vsc null pointer (#5217, @lilongfeng0902)
  * Refactor GCController with kubebuilder (#5215, @allenxu404)
  * Uploader Implementation: Restic backup and restore (#5214, @qiuming-best)
  * Add parameter "uploader-type" to velero server (#5212, @reasonerjt)
  * Add annotation "pv.kubernetes.io/migrated-to" for CSI checking. (#5181, @jxun)
  * Add changes for Kopia Integration: Unified Repository Provider - method implementation (#5179, @Lyndon-Li)
  * Treat namespaces with exclude label as excludedNamespaces
Related issue: #2413 (#5178, @allenxu404)
  * Reduce CRD size. (#5174, @jxun)
  * Fix restic backups to multiple backup storage locations bug (#5172, @qiuming-best)
  * Add changes for Kopia Integration: Unified Repository Provider - Repo Password (#5167, @Lyndon-Li)
  * Skip registering "crd-remap-version" plugin when feature flag "EnableAPIGroupVersions" is set (#5165, @reasonerjt)
  * Kopia uploader integration on shim progress uploader module (#5163, @qiuming-best)
  * Add labeled and unlabeled events for PR changelog check action. (#5157, @jxun)
  * VolumeSnapshotLocation refactor with kubebuilder. (#5148, @jxun)
  * Delay CA file deletion in PVB controller. (#5145, @jxun)
  * This commit splits the pkg/restic package into several packages to support Kopia integration works (#5143, @ywk253100)
  * Kopia Integration: Add the Unified Repository Interface definition. Kopia Integration: Add the changes for Unified Repository storage config. Related Issues; #5076, #5080 (#5142, @Lyndon-Li)
  * Update the CRD for kopia integration (#5135, @reasonerjt)
  * Let "make shell xxx" respect GOPROXY (#5128, @reasonerjt)
  * Modify BackupStoreGetter to avoid BSL spec changes (#5122, @sseago)
  * Dump stack trace when the plugin server handles panic (#5110, @reasonerjt)
  * Make CSI snapshot creation timeout configurable. (#5104, @jxun)
  *  Fix bsl validation bug: the BSL is validated continually and doesn't respect the validation period configured (#5101, @ywk253100)
  * Exclude "csinodes.storage.k8s.io" and "volumeattachments.storage.k8s.io" from restore by default. (#5064, @jxun)
  * Move 'velero.io/exclude-from-backup' label string to const (#5053, @niulechuan)
  * Modify Github actions. (#5052, @jxun)
  * Fix typo in doc, in https://velero.io/docs/main/restore-reference/ "Restore order" section, "Mamespace" should be "Namespace". (#5051, @niulechuan)
  * Delete opened issues triage action. (#5041, @jxun)
  * When spec.RestoreStatus is empty, don't restore status (#5008, @sseago)
  * Added DownloadTargetKindCSIBackupVolumeSnapshots for retrieving the signed URL to download only the `<backup name>`-csi-volumesnapshots.json.gz  and DownloadTargetKindCSIBackupVolumeSnapshotContents to download only `<backup name>`-csi-volumesnapshotcontents.json.gz in the DownloadRequest CR structure. These files are already present in the backup layout.  (#4980, @anshulahuja98)
  * Refactor BackupItemAction proto and related code to backupitemaction/v1 package.  This is part of implementation of the plugin version design https://github.com/vmware-tanzu/velero/blob/main/design/plugin-versioning.md (#4943, @phuongatemc)
  * Unified Repository Design (#4926, @Lyndon-Li)
  * Add credentials to volume snapshot locations (#4864, @sseago)