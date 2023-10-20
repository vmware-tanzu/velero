## v1.12.1
### 2023-10-20

### Download
https://github.com/vmware-tanzu/velero/releases/tag/v1.12.1

### Container Image
`velero/velero:v1.12.1`

### Documentation
https://velero.io/docs/v1.12/

### Upgrading
https://velero.io/docs/v1.12/upgrade-to-1.12/

### Highlights

#### Data Mover Adds Support for Block Mode Volumes
For PersistentVolumes with volumeMode set as Block, the volumes are mounted as raw block devices in pods, in 1.12.1, Velero CSI snapshot data movement supports to backup and restore this kind of volumes under linux based Kubernetes clusters.

#### New Parameter in Installation to Enable Data Mover
The `velero install` sub-command now includes a new parameter,`--default-snapshot-move-data`, which configures Velero server to move data by default for all snapshots supporting data movement. This feature is useful for users who will always want to use VBDM for backups instead of plain CSI , as they no longer need to specify the `--snapshot-move-data` flag for each individual backup. 

#### Velero Base Image change
The base image previously used by Velero was `distroless`,  which contains several CVEs cannot be addressed quickly. As a result, Velero will now use `paketobuildpacks` image starting from this new version.

### Limitations/Known issues
* The data mover's support for block mode volumes is currently only applicable to Linux environments.

### All changes
* Import auth provider plugins (#6970, @0x113)
* Perf improvements for existing resource restore (#6948, @sseago)
* Retry failed create when using generateName (#6943, @sseago)
* Fix issue #6647, add the --default-snapshot-move-data parameter to Velero install, so that users don't need to specify --snapshot-move-data per backup when they want to move snapshot data for all backups (#6940, @Lyndon-Li)
* Partially fix #6734, guide Kubernetes' scheduler to spread backup pods evenly across nodes as much as possible, so that data mover backup could achieve better parallelism (#6935, @Lyndon-Li)
* Replace the base image with paketobuildpacks image (#6934, @ywk253100)
* Add support for block volumes with Kopia (#6897, @dzaninovic)
* Set ParallelUploadAboveSize as MaxInt64 and flush repo after setting up policy so that policy is retrieved correctly by TreeForSource (#6886, @Lyndon-Li)
* Kubernetes 1.27 new job label batch.kubernetes.io/controller-uid are deleted during restore per https://github.com/kubernetes/kubernetes/pull/114930 (#6713, @kaovilai)
* Add `orLabelSelectors` for backup, restore commands (#6881, @nilesh-akhade)
* Fix issue #6859, move plugin depending podvolume functions to util pkg, so as to remove the dependencies to unnecessary repository packages like kopia, azure, etc. (#6877, @Lyndon-Li)
* Fix issue #6786, always delete VSC regardless of the deletion policy (#6873, @Lyndon-Li)
* Fix #6988, always get region from BSL if it is not empty (#6991, @Lyndon-Li)
* Add both non-Windows version and Windows version code for PVC block mode logic. (#6986, @blackpiglet)

## v1.12
### 2023-08-18

### Download
https://github.com/vmware-tanzu/velero/releases/tag/v1.12.0

### Container Image
`velero/velero:v1.12.0`

### Documentation
https://velero.io/docs/v1.12/

### Upgrading
https://velero.io/docs/v1.12/upgrade-to-1.12/

### Highlights

#### CSI Snapshot Data Movement
CSI Snapshot Data Movement refers to back up CSI snapshot data from the volatile and limited production environment into durable, heterogeneous, and scalable backup storage in a consistent manner; and restore the data to volumes in the original or alternative environment.

CSI Snapshot Data Movement is useful in below scenarios:

* For on-premises users, the storage usually doesn't support durable snapshots, so it is impossible/less efficient/cost ineffective to keep volume snapshots by the storage This feature helps to move the snapshot data to a storage with lower cost and larger scale for long time preservation.
* For public cloud users, this feature helps users to fulfill the multiple cloud strategy. It allows users to back up volume snapshots from one cloud provider and preserve or restore the data to another cloud provider. Then users will be free to flow their business data across cloud providers based on Velero backup and restore

CSI Snapshot Data Movement is built according to the Volume Snapshot Data Movement design ([Volume Snapshot Data Movement design](https://github.com/vmware-tanzu/velero/blob/main/design/volume-snapshot-data-movement/volume-snapshot-data-movement.md)). Additionally, guidance on how to use the feature can be found in the Volume Snapshot Data Movement doc([Volume Snapshot Data Movement doc](https://velero.io/docs/v1.12/csi-snapshot-data-movement)).

#### Resource Modifiers
In many use cases, customers often need to substitute specific values in Kubernetes resources during the restoration process like changing the namespace, changing the storage class, etc. 

To address this need, Resource Modifiers (also known as JSON Substitutions) offer a generic solution in the restore workflow. It allows the user to define filters for specific resources and then specify a JSON patch (operator, path, value) to apply to the resource. This feature simplifies the process of making substitutions without requiring the implementation of a new RestoreItemAction plugin. More design details can be found in Resource Modifiers design ([Resource Modifiers design](https://github.com/vmware-tanzu/velero/blob/main/design/Implemented/json-substitution-action-design.md)). For instructions on how to use the feature, please refer to Resource Modifiers doc([Resource Modifiers doc](https://velero.io/docs/v1.12/restore-resource-modifiers)). 

#### Multiple VolumeSnapshotClasses
Prior to version 1.12, the Velero CSI plugin would choose the VolumeSnapshotClass in the cluster based on matching driver names and the presence of the "velero.io/csi-volumesnapshot-class" label. However,  this approach proved inadequate for many user scenarios.

With the introduction of version 1.12, Velero now offers support for multiple VolumeSnapshotClasses in the CSI Plugin, enabling users to select a specific class for a particular backup. More design details can be found in Multiple VolumeSnapshotClasses design ([Multiple VolumeSnapshotClasses design](https://github.com/vmware-tanzu/velero/blob/main/design/Implemented/multiple-csi-volumesnapshotclass-support.md)). For instructions on how to use the feature, please refer to Multiple VolumeSnapshotClasses doc ([Multiple VolumeSnapshotClasses doc](https://velero.io/docs/v1.12/csi/#implementation-choices)). 

#### Restore Finalizer
Before v1.12, the restore controller would only delete restore resources but wouldn’t delete restore data from the backup storage location when the command `velero restore delete` was executed. The only chance Velero deletes restores data from the backup storage location is when the associated backup is deleted.

In this version, Velero introduces a finalizer that ensures the cleanup of all associated data for restores when running the command `velero restore delete`.

#### Runtime and dependencies
To fix CVEs and keep pace with Golang, Velero made changes as follows:
* Bump Golang runtime to v1.20.7.
* Bump several dependent libraries to new versions.
* Bump Kopia to v0.13.


### Breaking changes
* Prior to v1.12, the parameter `uploader-type` for Velero installation had a default value of "restic". However, starting from this version, the default value has been changed to "kopia". This means that Velero will now use Kopia as the default path for file system backup.
* The ways of setting CSI snapshot time have changed in v1.12. First, the sync waiting time for creating a snapshot handle in the CSI plugin is changed from the fixed 10 minutes into backup.Spec.CSISnapshotTimeout. The second, the async waiting time for VolumeSnapshot and VolumeSnapshotContent's status turning into `ReadyToUse` in operation uses the operation's timeout. The default value is 4 hours.
* As from [Velero helm chart v4.0.0](https://github.com/vmware-tanzu/helm-charts/releases/tag/velero-4.0.0), it supports multiple BSL and VSL, and the BSL and VSL have changed from the map into a slice, and[ this breaking change](https://github.com/vmware-tanzu/helm-charts/pull/413) is not backward compatible. So it would be best to change the BSL and VSL configuration into slices before the Upgrade.
* Prior to v1.12, deleting the Velero namespace would easily remove all the resources within it. However, with the introduction of finalizers attached to the Velero CR including `restore`, `dataupload`, and `datadownload` in this version, directly deleting Velero namespace may get stuck indefinitely because the pods responsible for handling the finalizers might be deleted before the resources attached to the finalizers. To avoid this issue, please use the command `velero uninstall` to delete all the Velero resources or ensure that you handle the finalizer appropriately before deleting the Velero namespace.


### Limitations/Known issues
* The Azure plugin supports Azure AD Workload identity way, but it only works for Velero native snapshots. It cannot support filesystem backup and snapshot data mover scenarios. 
* File System backup under Kopia path and CSI Snapshot Data Movement backup fail to back up files that are large the 2GiB due to issue https://github.com/vmware-tanzu/velero/issues/6668.


### All Changes
* Fixes #6498. Get resource client again after restore actions in case resource's gv is changed. This is an improvement of pr #6499, to support group changes. A group change usually happens in a restore plugin which is used for resource conversion: convert a resource from a not supported gv to a supported gv (#6634, @27149chen)
* Add API support for volMode block, only error for now. (#6608, @shawn-hurley)
* Fix how the AWS credentials are obtained from configuration (#6598, @aws_creds)
* Add performance E2E test (#6569, @qiuming-best)
* Non default s3 credential profiles work on Unified Repository Provider (kopia) (#6558, @kaovilai)
* Fix issue #6571, fix the problem for restore item operation to set the errors correctly so that they can be recorded by Velero restore and then reflect the correct status for Velero restore. (#6594, @Lyndon-Li)
* Fix issue 6575, flush the repo after delete the snapshot, otherwise, the changes(deleting repo snapshot) cannot be committed to the repo. (#6587, @Lyndon-Li)
* Delete moved snapshots when the backup is deleted (#6547, @reasonerjt)
* check if restore crd exist before operating restores (#6544, @allenxu404)
* Remove PVC's selector in backup's PVC action. (#6481, @blackpiglet)
* Delete the expired deletebackuprequests that are stuck in "InProgress" (#6476, @reasonerjt)
* Fix issue #6534, reset PVB CR's StorageLocation to the latest one during backup sync as same as the backup CR. Also fix similar problem with DataUploadResult for data mover restore. (#6533, @Lyndon-Li)
* Fix issue #6519. Restrict the client manager of node-agent server to include only Velero resources from the server's namespace, otherwise, the controllers will try to reconcile CRs from all the installed Velero namespaces. (#6523, @Lyndon-Li)
* Track the skipped PVC and print the summary in backup log  (#6496, @reasonerjt)
* Add restore finalizer to clean up external resources (#6479, @allenxu404)
* fix: Typos and add more spell checking rules to CI (#6415, @mateusoliveira43)
* Add missing CompletionTimestamp and metrics when restore moved into terminal phase in restoreOperationsReconciler (#6397, @Nutrymaco)
* Add support for resource Modifications in the restore flow. Also known as JSON Substitutions. (#6452, @anshulahuja98)
* Remove dependency of the legacy client code from pkg/cmd directory part 2 (#6497, @blackpiglet)
* Add data upload and download metrics (#6493, @allenxu404)
* Fix issue 6490, If a backup/restore has multiple async operations and one operation fails while others are still in-progress, when all the operations finish, the backup/restore will be set as Completed falsely (#6491, @Lyndon-Li)
* Velero Plugins no longer need kopia indirect dependency in their go.mod (#6484, @kaovilai)
* Remove dependency of the legacy client code from pkg/cmd directory (#6469, @blackpiglet)
* Add support for OpenStack CSI drivers topology keys (#6464, @openstack-csi-topology-keys)
* Add exit code log and possible memory shortage warning log for Restic command failure. (#6459, @blackpiglet)
* Modify DownloadRequest controller logic (#6433, @blackpiglet)
* Add data download controller for data mover (#6436, @qiuming-best)
* Fix hook filter display issue for backup describer (#6434, @allenxu404)
* Retrieve DataUpload into backup result ConfigMap during volume snapshot restore. (#6410, @blackpiglet)
* Design to add support for Multiple VolumeSnapshotClasses in CSI Plugin. (#5774, @anshulahuja98)
* Clarify the deletion frequency for gc controller (#6414, @allenxu404)
* Add unit tests for pkg/archive (#6396, @allenxu404)
* Add UT for pkg/discovery (#6394, @qiuming-best)
* Add UT for pkg/util (#6368, @Lyndon-Li)
* Add the code for data mover restore expose (#6357, @Lyndon-Li)
* Restore Endpoints before Services (#6315, @ywk253100)
* Add warning message for volume snapshotter in data mover case. (#6377, @blackpiglet)
* Add unit test for pkg/uploader (#6374, @qiuming-best)
* Change kopia as the default path of PVB (#6370, @Lyndon-Li)
* Do not persist VolumeSnapshot and VolumeSnapshotContent for snapshot DataMover case. (#6366, @blackpiglet)
* Add data mover related options in CLI (#6365, @ywk253100)
* Add dataupload controller (#6337, @qiuming-best)
* Add UT cases for pkg/podvolume (#6336, @Lyndon-Li)
* Remove Wait VolumeSnapshot to ReadyToUse logic. (#6327, @blackpiglet)
* Enhance the code because of #6297, the return value of GetBucketRegion is not recorded, as a result, when it fails, we have no way to get the cause (#6326, @Lyndon-Li)
* Skip updating status when CRDs are restored (#6325, @reasonerjt)
* Include namespaces needed by namespaced-scope resources in backup. (#6320, @blackpiglet)
* Update metrics when backup failed with validation error (#6318, @ywk253100)
* Add the code for data mover backup expose (#6308, @Lyndon-Li)
* Fix a PVR issue for generic data path -- the namespace remap was not honored, and enhance the code for better error handling (#6303, @Lyndon-Li)
* Add default values for defaultItemOperationTimeout and itemOperationSyncFrequency in velero CLI  (#6298, @shubham-pampattiwar)
* Add UT cases for pkg/repository (#6296, @Lyndon-Li)
* Fix issue #5875. Since Kopia has supported IAM, Velero should not require static credentials all the time (#6283, @Lyndon-Li)
* Fixed a bug where status.progress is not getting updated for backups. (#6276, @kkothule)
* Add code change for async generic data path that is used by both PVB/PVR and data mover (#6226, @Lyndon-Li)
* Add data mover CRD under v2alpha1, include DataUpload CRD and DataDownload CRD (#6176, @Lyndon-Li)
* Remove any dataSource or dataSourceRef fields from PVCs in PVC BIA for cases of
prior PVC restores with CSI (#6111, @eemcmullan)
* Add the design for Volume Snapshot Data Movement (#5968, @Lyndon-Li)
* Fix issue #5123, Kopia repository supports self-cert CA for S3 compatible storage. (#6268, @Lyndon-Li)
* Bump up Kopia to v0.13 (#6248, @Lyndon-Li)
* log volumes to backup to help debug why `IsPodRunning` is called. (#6232, @kaovilai)
* Enable errcheck linter and resolve found issues (#6208, @blackpiglet)
* Enable more linters, and remove mal-functioned milestoned issue action. (#6194, @blackpiglet)
* Enable stylecheck linter and resolve found issues. (#6185, @blackpiglet)
* Fix issue #6182. If pod is not running, don't treat it as an error, let it go and leave a warning. (#6184, @Lyndon-Li)
* Enable staticcheck and resolve found issues (#6183, @blackpiglet)
* Enable linter revive and resolve found errors: part 2 (#6177, @blackpiglet)
* Enable linter revive and resolve found errors: part 1 (#6173, @blackpiglet)
* Fix usestdlibvars and whitespace linters issues. (#6162, @blackpiglet)
* Update Golang to v1.20 for main. (#6158, @blackpiglet)
* Make GetPluginConfig accessible from other packages. (#6151, @tkaovila)
* Ignore not found error during patching managedFields (#6136, @ywk253100)
* Fix the goreleaser issues and add a new goreleaser action (#6109, @blackpiglet)
* Add CSI snapshot data movement doc (#6793, @Lyndon-Li)
* Use old(origin) namespace in resource modifier conditions in case namespace may change during restore (#6724, @27149chen)
* Fix #6752: add namespace exclude check. (#6762, @blackpiglet)
* Update restore controller logic for restore deletion (#6761, @ywk253100)
* Fix issue #6753, remove the check for read-only BSL in restore async operation controller since Velero cannot fully support read-only mode BSL in restore at present (#6758, @Lyndon-Li)
* Fixes #6636, skip subresource in resource discovery (#6688, @27149chen)
* This pr made some improvements in Resource Modifiers:1. add label selector 2. change the field name from groupKind to groupResource (#6704, @27149chen)
