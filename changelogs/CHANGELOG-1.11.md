## v1.11
### 2023-04-07

### Download
https://github.com/vmware-tanzu/velero/releases/tag/v1.11.0

### Container Image
`velero/velero:v1.11.0`

### Documentation
https://velero.io/docs/v1.11/

### Upgrading
https://velero.io/docs/v1.11/upgrade-to-1.11/

### Highlights

#### BackupItemAction v2
This feature implements the BackupItemAction v2. BIA v2 has two new methods: Progress() and Cancel() and modifies the Execute() return value.

The API change is needed to facilitate long-running BackupItemAction plugin actions that may not be complete when the Execute() method returns. This will allow long-running BackupItemAction plugin actions to continue in the background while the Velero moves to the following plugin or the next item.

#### RestoreItemAction v2
This feature implemented the RestoreItemAction v2. RIA v2 has three new methods: Progress(), Cancel(), and AreAdditionalItemsReady(), and it modifies RestoreItemActionExecuteOutput() structure in the RIA return value.

The Progress() and Cancel() methods are needed to facilitate long-running RestoreItemAction plugin actions that may not be complete when the Execute() method returns. This will allow long-running RestoreItemAction plugin actions to continue in the background while the Velero moves to the following plugin or the next item. The AreAdditionalItemsReady() method is needed to allow plugins to tell Velero to wait until the returned additional items have been restored and are ready for use in the cluster before restoring the current item.

#### Plugin Progress Monitoring
This is intended as a replacement for the previously-approved Upload Progress Monitoring design ([Upload Progress Monitoring](https://github.com/vmware-tanzu/velero/blob/main/design/upload-progress.md)) to expand the supported use cases beyond snapshot upload to include what was previously called Async Backup/Restore Item Actions.

#### Flexible resource policy that can filter volumes to skip in the backup
This feature provides a flexible policy to filter volumes in the backup without requiring patching any labels or annotations to the pods or volumes. This policy is configured as k8s ConfigMap and maintained by the users themselves, and it can be extended to more scenarios in the future. By now, the policy rules out volumes from backup depending on the CSI driver, NFS setting, volume size, and StorageClass setting. Please refer to [Resource policies rules](https://velero.io/docs/v1.11/resource-filtering/#resource-policies) for the policy's ConifgMap format. It is not guaranteed to work on unofficial third-party plugins as it may not follow the existing backup workflow code logic of Velero.

#### Resource Filters that can distinguish cluster scope and namespace scope resources
This feature adds four new resource filters for backup. The new filters are separated into cluster scope and namespace scope. Before this feature, Velero could not filter cluster scope resources precisely. This feature provides the ability and refactors existing resource filter parameters.

#### New parameter in installation to customize the serviceaccount name
The `velero install` sub-command now includes a new parameter,`--service-account-name`, which allows users to specify the ServiceAccountName for the Velero and node-agent pods. This feature may be particularly useful for users who utilize IRSA (IAM Roles for Service Accounts) in Amazon EKS (Elastic Kubernetes Service)."

#### Add a parameter for setting the Velero server connection with the k8s API server's timeout
In Velero, some code pieces need to communicate with the k8s API server. Before v1.11, these code pieces used hard-code timeout settings. This feature adds a resource-timeout parameter in the velero server binary to make it configurable.

#### Add resource list in the output of the restore describe command
Before this feature, Velero restore didn't have a restored resources list as the Velero backup. It's not convenient for users to learn what is restored. This feature adds the resources list and the handling result of the resources (including created, updated, failed, and skipped).

#### Support JSON format output of backup describe command 
Before the Velero v1.11 release, users could not choose Velero's backup describe command's output format. The command output format is friendly for human reading, but it's not a structured output, and it's not easy for other programs to get information from it. Velero v1.11 adds a JSON format output for the backup describe command.

#### Refactor controllers with controller-runtime
In v1.11, Backup Controller and Restore controller are refactored with controller-runtime. Till v1.11, all Velero controllers use the controller-runtime framework.

#### Runtime and dependencies
To fix CVEs and keep pace with Golang, Velero made changes as follows:
* Bump Golang runtime to v1.19.8.
* Bump several dependent libraries to new versions.
* Compile Restic (v0.15.0) with Golang v1.19.8 instead of packaging the official binary.


### Breaking changes
* The Velero CSI plugin now determines whether to restore Volume's data from snapshots on the restore's restorePVs setting. Before v1.11, the CSI plugin doesn't check the restorePVs parameter setting. 


### Limitations/Known issues
* The Flexible resource policy that can filter volumes to skip in the backup is not guaranteed to work on unofficial third-party plugins because the plugins may not follow the existing backup workflow code logic of Velero. The ConfigMap used as the policy is supposed to be maintained by users.


### All Changes
* Ignore not found error during patching managedFields (#6110, @ywk253100)
* Modify new scope resource filters name. (#6089, @blackpiglet)
* Make Velero not exits when EnableCSI is on and CSI snapshot not installed (#6062, @blackpiglet)
* Restore Services before Clusters (#6057, @ywk253100)
* Fixed backup deletion bug related to async operations (#6041, @sseago)
* Update Golang version to v1.19 for branch main. (#6039, @blackpiglet)
* Fix issue #5972, don't assume errorField as error type when dealing with logger.WithError (#6028, @Lyndon-Li)
* distinguish between New and InProgress operations (#6012, @sseago)
* Modify golangci.yaml file. Resolve found lint issues. (#6008, @blackpiglet)
* Remove Reference of itemsnapshotter (#5997, @reasonerjt)
* minor fixes for backup_operations_controller (#5996, @sseago)
* RIAv2 async operations controller work (#5993, @sseago)
* Follow-on fixes for BIAv2 controller work (#5971, @sseago)
* Refactor backup controller based on the controller-runtime framework. (#5969, @qiuming-best)
* Fix client wait problem after async operation change, velero backup/restore --wait should check a full list of the terminal status (#5964, @Lyndon-Li)
* Fix issue #5935, refactor the logics for backup/restore persistent log, so as to remove the contest to gzip writer (#5956, @Lyndon-Li)
* Switch the base image to distroless/base-nossl-debian11 to reduce the CVE triage efforts (#5939, @ywk253100)
* Wait for additional items to be ready before restoring current item (#5933, @sseago)
* Add configurable server setting for default timeouts (#5926, @eemcmullan)
* Add warning/error result to cmd `velero backup describe` (#5916, @allenxu404)
* Fix Dependabot alerts. Use 1.18 and 1.19 golang instead of patch image in dockerfile. Add release-1.10 and release-1.9 in Trivy daily scan. (#5911, @blackpiglet)
* Update client-go to v0.25.6 (#5907, @kaovilai)
* Limit the concurrent number for backup's VolumeSnapshot operation. (#5900, @blackpiglet)
* Fix goreleaser issue for resolving tags and updated it's version. (#5899, @anshulahuja98)
* This is to fix issue 5881, enhance the PVB tracker in two modes, Track and Taken (#5894, @Lyndon-Li)
* Add labels for velero installed namespace to support PSA. (#5873, @blackpiglet)
* Add restored resource list in the restore describe command (#5867, @ywk253100)
* Add a json output to cmd velero backup describe  (#5865, @allenxu404)
* Make restore controller adopting the controller-runtime framework. (#5864, @blackpiglet)
* Replace k8s.io/apimachinery/pkg/util/clock with k8s.io/utils/clock (#5859, @hezhizhen)
* Restore finalizer and managedFields of metadata during the restoration (#5853, @ywk253100)
* BIAv2 async operations controller work (#5849, @sseago)
* Add secret restore item action to handle service account token secret (#5843, @ywk253100)
* Add new resource filters can separate cluster and namespace scope resources. (#5838, @blackpiglet)
* Correct PVB/PVR Failed Phase patching during startup (#5828, @kaovilai)
* bump up golang net to fix CVE-2022-41721 (#5812, @Lyndon-Li)
* Update CRD descriptions for SnapshotVolumes and restorePVs (#5807, @shubham-pampattiwar)
* Add mapped selected-node existence check (#5806, @blackpiglet)
* Add option "--service-account-name" to install cmd (#5802, @reasonerjt)
* Enable staticcheck linter. (#5788, @blackpiglet)
* Set Kopia IgnoreUnknownTypes in ErrorHandlingPolicy to True for ignoring backup unknown file type (#5786, @qiuming-best)
* Bump up Restic version to 0.15.0  (#5784, @qiuming-best)
* Add File system backup related matrics to Grafana dashboard
  - Add metrics backup_warning_total for record of total warnings
  - Add metrics backup_last_status for record of last status of the backup  (#5779, @allenxu404)
* Design for Handling backup of volumes by resources filters (#5773, @qiuming-best)
* Add PR container build action, which will not push image. Add GOARM parameter. (#5771, @blackpiglet)
* Fix issue 5458, track pod volume backup until the CR is submitted in case it is skipped half way (#5769, @Lyndon-Li)
* Fix issue 5226, invalidate the related backup repositories whenever the backup storage info change in BSL (#5768, @Lyndon-Li)
* Add Restic builder in Dockerfile, and keep the used built Golang image version in accordance with upstream Restic. (#5764, @blackpiglet)
* Fix issue 5043, after the restore pod is scheduled, check if the node-agent pod is running in the same node. (#5760, @Lyndon-Li)
* Remove restore controller's redundant client. (#5759, @blackpiglet)
* Define itemoperations.json format and update DownloadRequest API (#5752, @sseago)
* Add Trivy nightly scan. (#5740, @jxun)
* Fix issue 5696, check if the repo is still openable before running the prune and forget operation, if not, try to reconnect the repo (#5715, @Lyndon-Li)
* Fix error with Restic backup empty volumes (#5713, @qiuming-best)
* new backup and restore phases to support async plugin operations:
  - WaitingForPluginOperations
  - WaitingForPluginOperationsPartiallyFailed (#5710, @sseago)
* Prevent nil panic on exec restore hooks (#5675, @dymurray)
* Fix CVEs scanned by trivy (#5653, @qiuming-best)
* Publish backupresults json to enhance error info during backups. (#5576, @anshulahuja98)
* RestoreItemAction v2 API implementation (#5569, @sseago)
* add new RestoreItemAction of "velero.io/change-image-name" to handle the issue mentioned at #5519 (#5540, @wenterjoy)
* BackupItemAction v2 API implementation (#5442, @sseago)
* Proposal to separate resource filter into cluster scope and namespace scope (#5333, @blackpiglet)
