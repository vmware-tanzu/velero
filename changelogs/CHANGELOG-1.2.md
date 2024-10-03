## v1.2.0
#### 2019-11-07

### Download
https://github.com/vmware-tanzu/velero/releases/tag/v1.2.0

### Container Image
`velero/velero:v1.2.0`

Please note that as of this release we are no longer publishing new container images to `gcr.io/heptio-images`. The existing ones will remain there for the foreseeable future.

### Documentation
https://velero.io/docs/v1.2.0/

### Upgrading
https://velero.io/docs/v1.2.0/upgrade-to-1.2/

### Highlights
## Moving Cloud Provider Plugins Out of Tree

Velero has had built-in support for AWS, Microsoft Azure, and Google Cloud Platform (GCP)  since day 1. When Velero moved to a plugin architecture for object store providers and volume snapshotters in version 0.6, the code for these three providers was converted to use the plugin interface provided by this new architecture, but the cloud provider code still remained inside the Velero codebase. This put the AWS, Azure, and GCP plugins in a different position compared with other providers’ plugins, since they automatically shipped with the Velero binary and could include documentation in-tree.

With version 1.2, we’ve extracted the AWS, Azure, and GCP plugins into their own repositories, one per provider. We now also publish one plugin image per provider. This change brings these providers to parity with other providers’ plugin implementations, reduces the size of the core Velero binary by not requiring each provider’s SDK to be included, and opens the door for the plugins to be maintained and released independently of core Velero.

## Restic Integration Improvements

We’ve continued to work on improving Velero’s restic integration. With this release, we’ve made the following enhancements:

- Restic backup and restore progress is now captured during execution and visible to the user through the `velero backup/restore describe --details` command. The details are updated every 10 seconds. This provides a new level of visibility into restic operations for users.
- Restic backups of persistent volume claims (PVCs) now remain incremental across the rescheduling of a pod. Previously, if the pod using a PVC was rescheduled, the next restic backup would require a full rescan of the volume’s contents. This improvement potentially makes such backups significantly faster.
- Read-write-many volumes are no longer backed up once for every pod using the volume, but instead just once per Velero backup. This improvement speeds up backups and prevents potential restore issues due to multiple copies of the backup being processed simultaneously.


## Clone PVs When Cloning a Namespace

Before version 1.2, you could clone a Kubernetes namespace by backing it up and then restoring it to a different namespace in the same cluster by using the `--namespace-mappings` flag with the `velero restore create` command. However, in this scenario, Velero was unable to clone persistent volumes used by the namespace, leading to errors for users.

In version 1.2, Velero automatically detects when you are trying to clone an existing namespace, and clones the persistent volumes used by the namespace as well. This doesn’t require the user to specify any additional flags for the `velero restore create` command.  This change lets you fully achieve your goal of cloning namespaces using persistent storage within a cluster.

## Improved Server-Side Encryption Support

To help you secure your important backup data, we’ve added support for more forms of server-side encryption of backup data on both AWS and GCP. Specifically:
- On AWS, Velero now supports Amazon S3-managed encryption keys (SSE-S3), which uses AES256 encryption, by specifying `serverSideEncryption: AES256` in a backup storage location’s config.
- On GCP, Velero now supports using a specific Cloud KMS key for server-side encryption by specifying `kmsKeyName: <key name>` in a backup storage location’s config.

## CRD Structural Schema

In Kubernetes 1.16, custom resource definitions (CRDs) reached general availability. Structural schemas are required for CRDs created in the `apiextensions.k8s.io/v1` API group. Velero now defines a structural schema for each of its CRDs and automatically applies it the user runs the `velero install` command.  The structural schemas enable the user to get quicker feedback when their backup, restore, or schedule request is invalid, so they can immediately remediate their request.

### All Changes
  * Ensure object store plugin processes are cleaned up after restore and after BSL validation during server start up (#2041, @betta1)
  * bug fix: don't try to restore pod volume backups that don't have a snapshot ID (#2031, @skriss)
  * Restore Documentation: Updated Restore Documentation with Clarification implications of removing restore object. (#1957, @nainav)
  * add `--allow-partially-failed` flag to `velero restore create` for use with `--from-schedule` to allow partially-failed backups to be restored (#1994, @skriss)
  * Allow backup storage locations to specify backup sync period or toggle off sync (#1936, @betta1)
  * Remove cloud provider code (#1985, @carlisia)
  * Restore action for cluster/namespace role bindings (#1974, @alexander-demichev)
  * Add `--no-default-backup-location` flag to `velero install` (#1931, @Frank51)
  * If includeClusterResources is nil/auto, pull in necessary CRDs in backupResource (#1831, @sseago)
  * Azure: add support for Azure China/German clouds (#1938, @andyzhangx)
  * Add a new required `--plugins` flag for `velero install` command. `--plugins` takes a list of container images to add as initcontainers. (#1930, @nrb)
  * restic: only backup read-write-many PVCs at most once, even if they're annotated for backup from multiple pods. (#1896, @skriss)
  * Azure: add support for cross-subscription backups (#1895, @boxcee)
  * adds `insecureSkipTLSVerify` server config for AWS storage and `--insecure-skip-tls-verify` flag on client for self-signed certs (#1793, @s12chung)
  * Add check to update resource field during backupItem (#1904, @spiffcs)
  * Add `LD_LIBRARY_PATH` (=/plugins) to the env variables of velero deployment. (#1893, @lintongj)
  * backup sync controller: stop using `metadata/revision` file, do a full diff of bucket contents vs. cluster contents each sync interval (#1892, @skriss)
  * bug fix: during restore, check item's original namespace, not the remapped one, for inclusion/exclusion (#1909, @skriss)
  * adds structural schema to Velero CRDs created on Velero install, enabling validation of Velero API fields (#1898, @prydonius)
  * GCP: add support for specifying a Cloud KMS key name to use for encrypting backups in a storage location. (#1879, @skriss)
  * AWS: add support for SSE-S3 AES256 encryption via `serverSideEncryption` config field in BackupStorageLocation (#1869, @skriss)
  * change default `restic prune` interval to 7 days, add `velero server/install` flags for specifying an alternate default value. (#1864, @skriss)
  * velero install: if `--use-restic` and `--wait` are specified, wait up to a minute for restic daemonset to be ready (#1859, @skriss)
  * report restore progress in PodVolumeRestores and expose progress in the velero restore describe --details command (#1854, @prydonius)
  * Jekyll Site updates - modifies documentation to use a wider layout; adds better markdown table formatting (#1848, @ccbayer)
  * fix excluding additional items with the velero.io/exclude-from-backup=true label (#1843, @prydonius)
  * report backup progress in PodVolumeBackups and expose progress in the velero backup describe --details command. Also upgrades restic to v0.9.5 (#1821, @prydonius)
  * Add `--features` argument to all velero commands to provide feature flags that can control enablement of pre-release features. (#1798, @nrb)
  * when backing up PVCs with restic, specify `--parent` flag to prevent full volume rescans after pod reschedules (#1807, @skriss)
  * remove 'restic check' calls from before/after 'restic prune' since they're redundant (#1794, @skriss)
  * fix error formatting due interpreting % as printf formatted strings (#1781, @s12chung)
  * when using `velero restore create --namespace-mappings ...` to create a second copy of a namespace in a cluster, create copies of the PVs used (#1779, @skriss)
  * adds --from-schedule flag to the `velero create backup` command to create a Backup from an existing Schedule (#1734, @prydonius)
