## v1.1.0
#### 2019-08-22

### Download
- https://github.com/heptio/velero/releases/tag/v1.1.0

### Container Image
`gcr.io/heptio-images/velero:v1.1.0`

### Documentation
https://velero.io/docs/v1.1.0/

### Upgrading

**If you are running Velero in a non-default namespace**, i.e. any namespace other than `velero`, manual intervention is required when upgrading to v1.1. See [upgrading to v1.1](https://velero.io/docs/v1.1.0/upgrade-to-1.1/) for details.

### Highlights

#### Improved Restic Support

A big focus of our work this cycle was continuing to improve support for restic. To that end, we’ve fixed the following bugs:


- Prior to version 1.1, restic backups could be delayed or failed due to long-lived locks on the repository. Now, Velero removes stale locks from restic repositories every 5 minutes, ensuring they do not interrupt normal operations.  
- Previously, the PodVolumeBackup custom resources that represented a restic backup within a cluster were not synchronized between clusters, making it unclear what restic volumes were available to restore into a new cluster. In version 1.1, these resources are synced into clusters, so they are more visible to you when you are trying to restore volumes.  
- Originally, Velero would not validate the host path in which volumes were mounted on a given node. If a node did not expose the filesystem correctly, you wouldn’t know about it until a backup failed. Now, Velero’s restic server will validate that the directory structure is correct on startup, providing earlier feedback when it’s not.  
- Velero’s restic support is intended to work on a broad range of volume types. With the general release of the [Container Storage Interface API](https://kubernetes.io/blog/2019/01/15/container-storage-interface-ga/), Velero can now use restic to back up CSI volumes.  

Along with our bug fixes, we’ve provided an easier way to move restic backups between storage providers. Different providers often have different StorageClasses, requiring user intervention to make restores successfully complete.

To make cross-provider moves simpler, we’ve introduced a StorageClass remapping plug-in. It allows you to automatically translate one StorageClass on PersistentVolumeClaims and PersistentVolumes to another. You can read more about it in our [documentation](https://velero.io/docs/v1.1.0/restore-reference/#changing-pv-pvc-storage-classes).

#### Quality-of-Life Improvements

We’ve also made several other enhancements to Velero that should benefit all users.

Users sometimes ask about recommendations for Velero’s resource allocation within their cluster. To help with this concern, we’ve added default resource requirements to the Velero Deployment and restic init containers, along with configurable requests and limits for the restic DaemonSet. All these values can be adjusted if your environment requires it.

We’ve also taken some time to improve Velero for the future by updating the Deployment and DaemonSet to use the apps/v1 API group, which will be the [default in Kubernetes 1.16](https://github.com/kubernetes/kubernetes/blob/master/CHANGELOG-1.16.md#action-required-3). This change means that `velero install` and the `velero plugin` commands will require Kubernetes 1.9 or later to work. Existing Velero installs will continue to work without needing changes, however.

In order to help you better understand what resources have been backed up, we’ve added a list of resources in the `velero backup describe --details` command. This change makes it easier to inspect a backup without having to download and extract it.

In the same vein, we’ve added the ability to put custom tags on cloud-provider snapshots. This approach should provide a better way to keep track of the resources being created in your cloud account. To add a label to a snapshot at backup time, use the `--labels` argument in the `velero backup create` command.

Our final change for increasing visibility into your Velero installation is the `velero plugin get` command. This command will report all the plug-ins within the Velero deployment..

Velero has previously used a restore-only flag on the server to control whether a cluster could write backups to object storage. With Velero 1.1, we’ve now moved the restore-only behavior into read-only BackupStorageLocations. This move means that the Velero server can use a BackupStorageLocation as a source to restore from, but not for backups, while still retaining the ability to back up to other configured locations. In the future, the `--restore-only` flag will be removed in favor of configuring read-only BackupStorageLocations.

#### Community Contributions

We appreciate all community contributions, whether they be pull requests, bug reports, feature requests, or just questions. With this release, we wanted to draw attention to a few contributions in particular:

For users of node-based IAM authentication systems such as kube2iam, `velero install` now supports the `--pod-annotations` argument for applying necessary annotations at install time. This support should make `velero install` more flexible for scenarios that do not use Secrets for access to their cloud buckets and volumes. You can read more about how to use this new argument in our [AWS documentation](https://velero.io/docs/v1.1.0/aws-config/#alternative-setup-permissions-using-kube2iam). Huge thanks to [Traci Kamp](https://github.com/tlkamp) for this contribution.

Structured logging is important for any application, and Velero is no different. Starting with version 1.1, the Velero server can now output its logs in a JSON format, allowing easier parsing and ingestion. Thank you to [Donovan Carthew](https://github.com/carthewd) for this feature.

AWS supports multiple profiles for accessing object storage, but in the past Velero only used the default. With v.1.1, you can set the `profile` key on yourBackupStorageLocation to specify an alternate profile. If no profile is set, the default one is used, making this change backward compatible. Thanks [Pranav Gaikwad](https://github.com/pranavgaikwad) for this change.

Finally, thanks to testing by [Dylan Murray](https://github.com/dymurray) and [Scott Seago](https://github.com/sseago), an issue with running Velero in non-default namespaces was found in our beta version for this release. If you’re running Velero in a namespace other than `velero`, please follow the [upgrade instructions](https://velero.io/docs/v1.1.0/upgrade-to-1.1/).

### All Changes
  * Add the prefix to BSL config map so that object stores can use it when initializing (#1767, @betta1)
  * Use `VELERO_NAMESPACE` to determine what namespace Velero server is running in. For any v1.0 installations using a different namespace, the `VELERO_NAMESPACE` environment variable will need to be set to the correct namespace. (#1748, @nrb)
  * support setting CPU/memory requests with unbounded limits using velero install (#1745, @prydonius)
  * sort output of resource list in `velero backup describe --details` (#1741, @prydonius)
  * adds the ability to define custom tags to be added to snapshots by specifying custom labels on the Backup CR with the velero backup create --labels flag (#1729, @prydonius)
  * Restore restic volumes from PodVolumeBackups CRs (#1723, @carlisia)
  * properly restore PVs backed up with restic and a reclaim policy of "Retain" (#1713, @skriss)
  * Make `--secret-file` flag on `velero install` optional, add `--no-secret` flag for explicit confirmation (#1699, @nrb)
  * Add low cpu/memory limits to the restic init container. This allows for restoration into namespaces with quotas defined. (#1677, @nrb)
  * Adds configurable CPU/memory requests and limits to the restic DaemonSet generated by velero install. (#1710, @prydonius)
  * remove any stale locks from restic repositories every 5m (#1708, @skriss)
  * error if backup storage location's Bucket field also contains a prefix, and gracefully handle leading/trailing slashes on Bucket and Prefix fields. (#1694, @skriss)
  * enhancement: allow option to choose JSON log output  (#1654, @carthewd)
  * Adds configurable CPU/memory requests and limits to the Velero Deployment generated by velero install. (#1678, @prydonius)
  * Store restic PodVolumeBackups in obj storage & use that as source of truth like regular backups. (#1577, @carlisia)
  * Update Velero Deployment to use apps/v1 API group. `velero install` and `velero plugin add/remove` commands will now require Kubernetes 1.9+ (#1673, @nrb)
  * Respect the --kubecontext and --kubeconfig arguments for `velero install`. (#1656, @nrb)
  * add plugin for updating PV & PVC storage classes on restore based on a config map (#1621, @skriss)
  * Add restic support for CSI volumes (#1615, @nrb)
  * bug fix: Fixed namespace usage with cli command 'version' (#1630, @jwmatthews)
  * enhancement: allow users to specify additional Velero/Restic pod annotations on the command line with the pod-annotations flag. (#1626, @tlkamp)
  * adds validation for pod volumes hostPath mount on restic server startup (#1616, @prydonius)
  * enable support for ppc64le architecture (#1605, @prajyot)
  * bug fix: only restore additional items returned from restore item actions if they match the restore's namespace/resource selectors (#1612, @skriss)
  * add startTimestamp and completionTimestamp to PodVolumeBackup and PodVolumeRestore status fields (#1609, @prydonius)
  * bug fix: respect namespace selector when determining which restore item actions to run (#1607, @skriss)
  * ensure correct backup item actions run with namespace selector (#1601, @prydonius)
  * allows excluding resources from backups with the `velero.io/exclude-from-backup=true` label (#1588, @prydonius)
  * ensures backup item action modifications to an item's namespace/name are saved in the file path in the tarball (#1587, @prydonius)
  * Hides `velero server` and `velero restic server` commands from the list of available commands as these are not intended for use by the velero CLI user. (#1561, @prydonius)
  * remove dependency on glog, update to klog (#1559, @skriss)
  * move issue-template-gen from docs/ to hack/ (#1558, @skriss)
  * fix panic when processing DeleteBackupRequest objects without labels (#1556, @prydonius)
  * support for multiple AWS profiles (#1548, @pranavgaikwad)
  * Add CLI command to list (get) all Velero plugins (#1535, @carlisia)
  * Added author as a tag on blog post.  Should fix 404 error when trying to follow link as specified in issue #1522. (#1522, @coonsd)
  * Allow individual backup storage locations to be read-only (#1517, @skriss)
  * Stop returning an error when a restic volume is empty since it is a valid scenario. (#1480, @carlisia)
  * add ability to use wildcard in includes/excludes (#1428, @guilhem)
