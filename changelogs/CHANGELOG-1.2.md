## v1.2.0-beta.1
#### 2019-10-24

### Download
- https://github.com/vmware-tanzu/velero/releases/tag/v1.2.0-beta.1

### Container Image
`velero/velero:v1.2.0-beta.1`

### Documentation
https://velero.io/docs/v1.2.0-beta.1/

### Upgrading

If you're upgrading from a previous version of Velero, there are several changes you'll need to be aware of:

- Container images are now published to Docker Hub. To upgrade your server, use the new image `velero/velero:v1.2.0-beta.1`.
- The AWS, Microsoft Azure, and GCP provider plugins that were previously part of the Velero binary have been extracted to their own standalone repositories/plugin images. If you are using one of these three providers, you will need to explicitly add the appropriate plugin to your Velero install:
  - [AWS] `velero plugin add velero/velero-plugin-for-aws:v1.0.0-beta.1`
  - [Azure] `velero plugin add velero/velero-plugin-for-microsoft-azure:v1.0.0-beta.1`
  - [GCP] `velero plugin add velero/velero-plugin-for-gcp:v1.0.0-beta.1`

### Highlights

- The AWS, Microsoft Azure, and GCP provider plugins that were previously part of the Velero binary have been extracted to their own standalone repositories/plugin images. They now function like any other provider plugin.
- Container images are now published to Docker Hub: `velero/velero:v1.2.0-beta.1`.
- Several improvements have been made to the restic integration:
  - Backup and restore progress is now updated on the `PodVolumeBackup` and `PodVolumeRestore` custom resources and viewable via `velero backup/restore describe` while operations are in progress.
  - Read-write-many PVCs are now only backed up once.
  - Backups of PVCs remain incremental across pod reschedules.
- A structural schema has been added to the Velero CRDs that are created by `velero install` to enable validation of API fields.
- During restores that use the `--namespace-mappings` flag to clone a namespace within a cluster, PVs will now be cloned as needed.

### All Changes
  * Allow backup storage locations to specify backup sync period or toggle off sync (#1936, @betta1)
  * Remove cloud provider code (#1985, @carlisia)
  * Restore action for cluster/namespace role bindings (#1974, @alexander)
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
  * add `--allow-partially-failed` flag to `velero restore create` for use with `--from-schedule` to allow partially-failed backups to be restored (#1994, @skriss)
