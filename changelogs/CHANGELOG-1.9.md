## v1.9.0
### 2022-06-13

### Download
https://github.com/vmware-tanzu/velero/releases/tag/v1.9.0

### Container Image
`velero/velero:v1.9.0`

### Documentation
https://velero.io/docs/v1.9/

### Upgrading
https://velero.io/docs/v1.9/upgrade-to-1.9/

### Highlights

#### Improvement to the CSI plugin
- Bump up to the CSI volume snapshot v1 API
- No VolumeSnapshot will be left in the source namespace of the workload
- Report metrics for CSI snapshots

More improvements please refer to [CSI plugin improvement](https://github.com/vmware-tanzu/velero/issues?q=is%3Aissue+label%3A%22CSI+plugin+-+GA+-+phase1%22+is%3Aclosed)

With these improvements we'll provide official support for CSI snapshots on AKS/EKS clusters. (with CSI plugin v0.3.0)

#### Refactor the controllers using Kubebuilder v3
In this release we continued our code modernization work, rewriting some controllers using Kubebuilder v3. This work is ongoing and we will continue to make progress in future releases.

#### Optionally restore status on selected resources
Options are added to the CLI and Restore spec to control the group of resources whose status will be restored.

#### ExistingResourcePolicy in the restore API
Users can choose to overwrite or patch the existing resources during restore by setting this policy.

#### Upgrade integrated Restic version and add skip TLS validation in Restic command
Upgrade integrated Restic version, which will resolve some of the CVEs, and support skip TLS validation in Restic backup/restore.

#### Breaking changes
With bumping up the API to v1 in CSI plugin, the v0.3.0 CSI plugin will only work for Kubernetes v1.20+

### All changes

  * restic: add full support for setting SecurityContext for restore init container from configMap. (#4084, @MatthieuFin)
  * Add metrics backup_items_total and backup_items_errors (#4296, @tobiasgiese)
  * Convert PodVolumebackup controller to the Kubebuilder framework (#4436, @fgold)
  * Skip not mounted volumes when backing up (#4497, @dkeven)
  * Update doc for v1.8 (#4517, @reasonerjt)
  * Fix bug to make the restic prune frequency configurable (#4518, @ywk253100)
  * Add E2E test of backups sync from BSL (#4545, @mqiu)
  * Fix: OrderedResources in Schedules (#4550, @dbrekau)
  * Skip volumes of non-running pods when backing up (#4584, @bynare)
  * E2E SSR test add retry mechanism and logs  (#4591, @mqiu)
  * Add pushing image to GCR in github workflow to facilitate some environments that have rate limitation to docker hub, e.g. vSphere. (#4623, @jxun)
  * Add existingResourcePolicy to Restore API (#4628, @shubham-pampattiwar)
  * Fix E2E backup namespaces test (#4634, @qiuming-best)
  * Update image used by E2E test to gcr.io (#4639, @jxun)
  * Add multiple label selector support to Velero Backup and Restore APIs (#4650, @shubham-pampattiwar)
  * Convert Pod Volume Restore resource/controller to the Kubebuilder framework (#4655, @ywk253100)
  * Update --use-owner-references-in-backup description in velero command line. (#4660, @jxun)
  * Avoid overwritten hook's exec.container parameter when running pod command executor. (#4661, @jxun)
  * Support regional pv for GKE (#4680, @jxun)
  * Bypass the remap CRD version plugin when v1beta1 CRD is not supported (#4686, @reasonerjt)
  * Add GINKGO_SKIP to support skip specific case in e2e test. (#4692, @jxun)
  * Add --pod-labels flag to velero install (#4694, @j4m3s-s)
  * Enable coverage in test.sh and upload to codecov (#4704, @reasonerjt)
  * Mark the BSL as "Unavailable" when gets any error and add a new field "Message" to the status to record the error message (#4719, @ywk253100)
  * Support multiple skip option for E2E test (#4725, @jxun)
  * Add PriorityClass to the AdditionalItems of Backup's PodAction and Restore's PodAction plugin to backup and restore PriorityClass if it is used by a Pod. (#4740, @phuongatemc)
  * Insert all restore errors and warnings into restore log. (#4743, @sseago)
  * Refactor schedule controller with kubebuilder (#4748, @ywk253100)
  * Garbage collector now adds labels to backups that failed to delete for BSLNotFound, BSLCannotGet, BSLReadOnly reasons. (#4757, @kaovilai)
  * Skip podvolumerestore creation when restore excludes pv/pvc (#4769, @half-life666)
  * Add parameter for e2e test to support modify kibishii install path. (#4778, @jxun)
  * Ensure the restore hook applied to new namespace based on the mapping (#4779, @reasonerjt)
  * Add ability to restore status on selected resources (#4785, @RafaeLeal)
  * Do not take snapshot for PV to avoid duplicated snapshotting, when CSI feature is enabled. (#4797, @jxun)
  * Bump up to v1 API for CSI snapshot (#4800, @reasonerjt)
  * fix: delete empty backups (#4817, @yuvalman)
  * Add CSI VolumeSnapshot related metrics. (#4818, @jxun)
  * Fix default-backup-ttl not work (#4831, @qiuming-best)
  * Make the vsc created by backup sync controller deletable (#4832, @reasonerjt)
  * Make in-progress backup/restore as failed when doing the reconcile to avoid hanging in in-progress status (#4833, @ywk253100)
  * Use controller-gen to generate the deep copy methods for objects (#4838, @ywk253100)
  * Update integrated Restic version and add insecureSkipTLSVerify for Restic CLI. (#4839, @jxun)
  * Modify CSI VolumeSnapshot metric related code. (#4854, @jxun)
  * Refactor backup deletion controller based on kubebuilder (#4855, @reasonerjt)
  * Remove VolumeSnapshots created during backup when CSI feature is enabled. (#4858, @jxun)
  * Convert Restic Repository resource/controller to the Kubebuilder framework (#4859, @qiuming-best)
  * Add ClusterClasses to the restore priority list (#4866, @reasonerjt)
  * Cleanup the .velero folder after restic done (#4872, @big-appled)
  * Delete orphan CSI snapshots in backup sync controller (#4887, @reasonerjt)
  * Make waiting VolumeSnapshot to ready process parallel. (#4889, @jxun)
  * continue rather than return for non-matching restore action label (#4890, @sseago)
  * Make in-progress PVB/PVR as failed when restic controller restarts to avoid hanging backup/restore (#4893, @ywk253100)
  * Refactor BSL controller with periodical enqueue source (#4894, @jxun)
  * Make garbage collection for expired backups configurable (#4897, @ywk253100)
  * Bump up the version of distroless to base-debian11 (#4898, @ywk253100)
  * Add schedule ordered resources E2E test (#4913, @qiuming-best)
  * Make velero completion zsh command output can be used by `source` command. (#4914, @jxun)
  * Enhance the map flag to support parsing input value contains entry delimiters (#4920, @ywk253100)
  * Fix E2E test [Backups][Deletion][Restic] on GCP. (#4968, @jxun)
  * Disable status as sub resource in CRDs (#4972, @ywk253100)
  * Add more information for failing to get path or snapshot in restic backup and restore. (#4988, @jxun)
