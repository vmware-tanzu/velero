## v1.8.0
### 2022-01-14

### Download
https://github.com/vmware-tanzu/velero/releases/tag/v1.8.0

### Container Image
`velero/velero:v1.8.0`

### Documentation
https://velero.io/docs/v1.8

### Upgrading
https://velero.io/docs/v1.8/upgrade-to-1.8/

### Highlights

#### Velero plugins now support handling volumes created by the CSI drivers of cloud providers
Versions 1.4 of the Velero plugins for AWS, Azure and GCP now support snapshotting and restoring the persistent volumes provisioned by CSI driver via the APIs of the cloud providers. With this enhancement, users can backup and restore the persistent volumes on these cloud providers without using the Velero CSI plugin. The CSI plugin will remain beta and the feature flag `EnableCSI` will be disabled by default.

For the version of the plugins and the CSI drivers they support respectively please see the table:

| Plugin | Version | CSI Driver |
| --- | ----------- | ---------- |
| velero-plugin-for-aws | v1.4.0 | ebs.csi.aws.com |
| velero-plugin-for-microsoft-azure | v1.4.0 | disk.csi.azure.com |
| velero-plugin-for-gcp | v1.4.0 | pd.csi.storage.gke.io |

#### IPv6 dual stack support
We've verified the functionality of Velero on IPv6 dual stack by successfully running the E2E test on IPv6 dual stack environment.
#### Refactor the controllers using Kubebuilder v3
In this release we continued our code modernization work, rewriting some controllers using Kubebuilder v3. This work is ongoing and we will continue to make progress in future releases.
#### Enhancements to E2E test cases
More test cases have been added to the E2E test suite to improve the release health.
#### Respect the cron setting of scheduled backup
The creation time is now taken into account to calculate the next run for scheduled backup.

#### Deleting BSLs also cleans up related resources

When a Backup Storage Location (BSL) is deleted, backup and Restic repository resources will also be deleted.

#### Breaking changes

Starting in v1.8, Velero will only support Kubernetes v1 CRD meaning that Velero v1.8+ will only run on Kubernetes v1.16+. Before upgrading, make sure you are running a supported Kubernetes version. For more information, see our [compatibility matrix](https://github.com/vmware-tanzu/velero#velero-compatibility-matrix).

#### Upload Progress Monitoring and Item Snapshotter
Item Snapshotter plugin API was merged.  This will support both Upload Progress
monitoring and the planned Data Mover.  Upload Progress monitoring PRs are
in progress for 1.9.

### All changes

* E2E test on ssr object with controller namespace mix-ups (#4521, @mqiu)
* Check whether the volume is provisioned by CSI driver or not by the annotation as well (#4513, @ywk253100)
* Initialize the labels field of `velero backup-location create` option to avoid #4484 (#4491, @ywk253100)
* Fix e2e 2500 namespaces scale test timeout problem (#4480, @mqiu)
* Add backup deletion e2e test  (#4401, @danfengliu)
* Return the error when getting backup store in backup deletion controller (#4465, @reasonerjt)
* Ignore the provided port is already allocated error when restoring the LoadBalancer service (#4462, @ywk253100)
* Revert #4423 migrate backup sync controller to kubebuilder. (#4457, @jxun)
* Add rbac and annotation test cases (#4455, @mqiu)
* remove --crds-version in velero install command. (#4446, @jxun)
* Upgrade e2e test vsphere plugin (#4440, @mqiu)
* Fix e2e test failures for the inappropriate optimaze of velero install (#4438, @mqiu)
* Limit backup namespaces on test resource filtering cases (#4437, @mqiu)
* Bump up Go to 1.17 (#4431, @reasonerjt)
* Added `<backup name>`-itemsnapshots.json.gz to the backup format.  This file exists
  when item snapshots are taken and contains an array of volume.Itemsnapshots
  containing the information about the snapshots.  This will not be used unless
  upload progress monitoring and item snapshots are enabled and an ItemSnapshot
  plugin is used to take snapshots.

Also added DownloadTargetKindBackupItemSnapshots for retrieving the signed URL to download only the `<backup name>`-itemsnapshots.json.gz part of a backup for use by
`velero backup describe`. (#4429, @dsmithuchida)
* Migrate backup sync controller from code-generator to kubebuilder. (#4423, @jxun)
* Added UploadProgressFeature flag to enable Upload Progress Monitoring and Item
  Snapshotters. (#4416, @dsmithuchida)
* Added BackupWithResolvers and RestoreWithResolvers calls.  Will eventually replace Backup and Restore methods.
  Adds ItemSnapshotters to Backup and Restore workflows. (#4410, @dsu)
* Build for darwin-arm64 (#4409, @epk)
* Add resource filtering test cases (#4404, @mqiu)
* Fix the issue that the backup cannot be deleted after the application uninstalled (#4398, @ywk253100)
* Add restoreactionitem plugin to handle admission webhook configurations (#4397, @reasonerjt)
* Keep the annotation "pv.kubernetes.io/provisioned-by" when restoring PVs (#4391, @ywk253100)
* Adjust structure of e2e test codes (#4386, @mqiu)
* feat: migrate velero controller from kubebuilder v2 to v3
  From Velero v1.8, apiextesions.k8s.io/v1beta1 is no longer supported,
  which means only CRD of apiextensions.k8s.io/v1 is supported,
  and the supported Kubernetes version is updated to v1.16 and later. (#4382, @jxun)
* Delete backups and Restic repos associated with deleted BSL(s) (#4377, @codegold79)
* Add the key for GKE zone for AZ collection (#4376, @reasonerjt)
* Fix statefulsets volumeClaimTemplates storageClassName when use Changing PV/PVC Storage Classes (#4375, @Box-Cube)
* Fix snapshot e2e test issue of jsonpath (#4372, @danfengliu)
* Modify the timestamp in the name of a backup generated from schedule to use UTC. (#4353, @jxun)
* Read Availability zone from nodeAffinity requirements  (#4350, @reasonerjt)
* Use factory.Namespace() to replace hardcoded velero namespace (#4346, @half-life666)
* Return the error if velero failed to detect S3 region for restic repo (#4343, @reasonerjt)
* Add init log option for velero controller-runtime manager. (#4341, @jxun)
* Ignore the `provided port is already allocated` error when restoring the `NodePort` service (#4336, @ywk253100)
* Fixed an issue with the `backup-location create` command where the BSL Credential field would be set to an invalid empty SecretKeySelector when no credential details were provided. (#4322, @zubron)
* fix buggy pager func (#4306, @alaypatel07)
* Don't create a backup immediately after creating a schedule (#4281, @ywk253100)
* Fix CVE-2020-29652 and CVE-2020-26160 (#4274, @ywk253100)
* Refine tag-release.sh to align with change in release process (#4185, @reasonerjt)
* Fix plugins incompatible issue in upgrade test (#4141, @danfengliu)
* Verify group before treating resource as cohabiting (#4126, @sseago)
* Added ItemSnapshotter plugin definition and plugin framework - addresses #3533.
  Part of the Upload Progress enhancement (#3533) (#4077, @dsmithuchida)
* Add upgrade test in E2E test (#4058, @danfengliu)
* Handle namespace mapping for PVs without snapshots on restore (#3708, @sseago)
