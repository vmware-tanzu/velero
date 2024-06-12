## v1.14

### Download
https://github.com/vmware-tanzu/velero/releases/tag/v1.14.0

### Container Image
`velero/velero:v1.14.0`

### Documentation
https://velero.io/docs/v1.14/

### Upgrading
https://velero.io/docs/v1.14/upgrade-to-1.14/

### Highlights

#### The maintenance work for kopia/restic backup repositories is run in jobs
Since velero started using kopia as the approach for filesystem-level backup/restore, we've noticed an issue when velero connects to the kopia backup repositories and performs maintenance, it sometimes consumes excessive memory that can cause the velero pod to get OOM Killed.  To mitigate this issue, the maintenance work will be moved out of velero pod to a separate kubernetes job, and the user will be able to specify the resource request in "velero install".
#### Volume Policies are extended to support more actions to handle volumes
In an earlier release, a flexible volume policy was introduced to skip certain volumes from a backup.  In v1.14 we've made enhancement to this policy to allow the user to set how the volumes should be backed up.  The user will be able to set "fs-backup" or "snapshot" as value of “action" in the policy and velero will backup the volumes accordingly.  This enhancement allows the user to achieve a fine-grained control like "opt-in/out" without having to update the target workload.  For more details please refer to https://velero.io/docs/v1.14/resource-filtering/#supported-volumepolicy-actions
#### Node Selection for Data Movement Backup
In velero the data movement flow relies on datamover pods, and these pods may take substantial resources and keep running for a long time.  In v1.14, the user will be able to create a configmap to define the eligible nodes on which the datamover pods are launched.  For more details refer to https://velero.io/docs/v1.14/data-movement-backup-node-selection/ 
#### VolumeInfo metadata for restored volumes
In v1.13, we introduced volumeinfo metadata for backup to help velero CLI and downstream adopter understand how velero handles each volume during backup.  In v1.14, similar metadata will be persisted for each restore. velero CLI is also updated to bring more info in the output of "velero restore describe".
#### "Finalizing" phase is introduced to restores
The "Finalizing" phase is added to the state transition flow to restore, which helps us fix several issues: The labels added to PVs will be restored after the data in the PV is restored via volumesnapshotter.  The post restore hook will be executed after datamovement is finished.
#### Certificate-based authentication support for Azure
Besides the service principal with secret(password)-based authentication, Velero introduces the new support for service principal with certificate-based authentication in v1.14.0. This approach enables you to adopt a phishing resistant authentication by using conditional access policies, which better protects Azure resources and is the recommended way by Azure.

### Runtime and dependencies
* Golang runtime: v1.22.2
* kopia: v0.17.0

### Limitations/Known issues
* For the external BackupItemAction plugins that take snapshots for PVs, such as vsphere plugin.  If the plugin checks the value of the field "snapshotVolumes" in the backup spec as a criteria for snapshot, the settings in the volume policy will not take effect.  For example, if the "snapshotVolumes" is set to False in the backup spec, but a volume meets the condition in the volume policy for "snapshot" action, because the plugin will not check the settings in the volume policy, the plugin will not take snapshot for the volume.  For more details please refer to #7818

### Breaking changes
* CSI plugin has been merged into velero repo in v1.14 release.  It will be installed by default as an internal plugin, and should not be installed via "–plugins " parameter in "velero install" command.
* The default resource requests and limitations for node agent are removed in v1.14, to make the node agent pods have the QoS class of "BestEffort", more details please refer to #7391
* There's a change in namespace filtering behavior during backup:  In v1.14, when the includedNamespaces/excludedNamespaces fields are not set and the labelSelector/OrLabelSelectors are set in the backup spec, the backup will only include the namespaces which contain the resources that match the label selectors, while in previous releases all namespaces will be included in the backup with such settings.  More details refer to #7105
* Patching the PV in the "Finalizing" state may cause the restore to be in "PartiallyFailed" state when the PV is blocked in "Pending" state, while in the previous release the restore may end up being in "Complete" state. For more details refer to #7866

### All Changes
* Fix backup log to show error string, not index (#7805, @piny940)
* Modify the volume helper logic. (#7794, @blackpiglet)
* Add documentation for extension of volume policy feature (#7779, @shubham-pampattiwar)
* Surface errors when waiting for backupRepository and timeout occurs (#7762, @kaovilai)
* Add existingResourcePolicy restore CR validation to controller (#7757, @kaovilai)
* Fix condition matching in resource modifier when there are multiple rules (#7715, @27149chen)
* Bump up the version of KinD and k8s in github actions (#7702, @reasonerjt)
* Implementation for Extending VolumePolicies to support more actions (#7664, @shubham-pampattiwar)
* Migrate from `github.com/Azure/azure-storage-blob-go` to `github.com/Azure/azure-sdk-for-go/sdk/storage/azblob` (#7598, @mmorel-35)
* When Included/ExcludedNamespaces are omitted, and LabelSelector or OrLabelSelector is used, namespaces without selected items are excluded from backup. (#7697, @blackpiglet)
* Display CSI snapshot restores in restore describe (#7687, @reasonerjt)
* Use specific credential rather than the credential chain for Azure (#7680, @ywk253100)
* Modify hook docs for clarity on displaying hook execution results (#7679, @allenxu404)
* Wait for results of restore exec hook executions in Finalizing phase instead of InProgress phase (#7619, @allenxu404)
* migrating to `sdk/resourcemanager/**/arm**` from `services/**/mgmt/**` (#7596, @mmorel-35)
* Bump up to go1.22 (#7666, @reasonerjt)
* Fix issue #7648. Adjust the exposing logic to avoid exposing failure and snapshot leak when expose fails (#7662, @Lyndon-Li)
* Track and persist restore volume info (#7630, @reasonerjt)
* Check the existence of the namespaces provided in the "--include-namespaces" option (#7569, @ywk253100)
* Add the finalization phase to the restore workflow (#7377, @allenxu404)
* Upgrade the version of go plugin related libs/tools (#7373, @ywk253100)
* Check resource Group Version and Kind is available in cluster before attempting restore to prevent being stuck. (#7322, @kaovilai)
* Merge CSI plugin code into Velero. (#7609, @blackpiglet)
* Fix issue #7391, remove the default constraint for node-agent pods (#7488, @Lyndon-Li)
* Fix DataDownload fails during restore for empty PVC workload (#7521, @qiuming-best)
* Add repository maintenance job (#7451, @qiuming-best)
* Check whether the VolumeSnapshot's source PVC is nil before using it.
  Skip populate VolumeInfo for data-moved PV when CSI is not enabled. (#7515, @blackpiglet)
* Fix issue #7308, change the data path requeue time to 5 second for data mover backup/restore, PVB and PVR. (#7458, @Lyndon-Li)
*  Patch newly dynamically provisioned PV with volume info to restore custom setting of PV (#7504, @allenxu404)
* Adjust the logic for the backup_last_status metrics to stop incorrectly incrementing over time (#7445, @allenxu404)
* dependabot: support github-actions updates (#7594, @mmorel-35)
* Include the design for adding the finalization phase to the restore workflow (#7317, @allenxu404)
* Fix issue #7211. Enable advanced feature capability and add support to concatenate objects for unified repo. (#7452, @Lyndon-Li)
* Add design to introduce restore volume info (#7610, @reasonerjt)
* Increase the k8s client QPS/burst to avoid throttling request errors (#7311, @ywk253100)
* Support update the backup VolumeInfos by the Async ops result. (#7554, @blackpiglet)
* FS backup create PodVolumeBackup when the backup excluded PVC,
  so I added logic to skip PVC volume type when PVC is not included in the backup resources to be backed up. (#7472, @sbahar619)
* Respect and use `credentialsFile` specified in BSL.spec.config when IRSA is configured over Velero Pod Environment credentials (#7374, @reasonerjt)
* Move the native snapshot definition code into internal directory (#7544, @blackpiglet)
* Fix issue #7036. Add the implementation of node selection for data mover backups (#7437, @Lyndon-Li)
* Fix issue #7535, add the MustHave resource check during item collection and item filter for restore (#7585, @Lyndon-Li)
* build(deps): bump json-patch to v5.8.0 (#7584, @mmorel-35)
* Add confirm flag to velero plugin add (#7566, @kaovilai)
* do not skip unknown gvr at the beginning and get new gr when kind is changed (#7523, @27149chen)
* Fix snapshot leak for backup (#7558, @qiuming-best)
* For issue #7036, add the document for data mover node selection (#7640, @Lyndon-Li)
* Add design for Extending VolumePolicies to support more actions (#6956, @shubham-pampattiwar)
* BackupRepositories associated with a BSL are invalidated when BSL is (re-)created. (#7380, @kaovilai)
* Improve the concurrency for PVBs in different pods  (#7571, @ywk253100)
* Bump up Kopia to v0.16.0 and open kopia repo with no index change (#7559, @Lyndon-Li)
* Bump up the versions of several Kubernetes-related libs (#7489, @ywk253100)
* Make parallel restore configurable (#7512, @qiuming-best)
* Support certificate-based authentication for Azure (#7549, @ywk253100)
* Fix issue #7281, batch delete snapshots in the same repo (#7438, @Lyndon-Li)
* Add CRD name to error message when it is not ready to use (#7295, @josemarevalo)
* Add the design for node selection for data mover backup (#7383, @Lyndon-Li)
* Bump up aws-sdk to latest version to leverage Pod Identity credentials. (#7307, @guikcd)
* Fix issue #7246. Document the behavior for repo snapshot deletion (#7622, @Lyndon-Li)
* Fix issue #7583, set backupName optional for Restore CRD (#7617, @Lyndon-Li)

