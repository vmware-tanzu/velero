  - [v0.10.2](#v0102)
  - [v0.10.1](#v0101)
  - [v0.10.0](#v0100)

## v0.10.2
#### 2019-02-28

### Download
- https://github.com/heptio/ark/releases/tag/v0.10.2

### Changes
  * upgrade restic to v0.9.4 & replace --hostname flag with --host (#1156, @skriss)
  * use 'restic stats' instead of 'restic check' to determine if repo exists (#1171, @skriss)
  * Fix concurrency bug in code ensuring restic repository exists (#1235, @skriss)

## v0.10.1
#### 2019-01-10

### Download
- https://github.com/heptio/ark/releases/tag/v0.10.1

### Changes
  * Fix minio setup job command (#1118, @acbramley)
  * Add debugging-install link in doc get-started.md (#1131, @hex108)
  * `ark version`: show full git SHA & combine git tree state indicator with git SHA line (#1124, @skriss)
  * Delete spec.priority in pod restore action (#879, @mwieczorek)
  * Allow to use AWS Signature v1 for creating signed AWS urls (#811, @bashofmann)
  * add multizone/regional support to gcp (#765, @wwitzel3)
  * Fixed the newline output when deleting a schedule. (#1120, @jwhitcraft)
  * Remove obsolete make targets and rename 'make goreleaser' to 'make release' (#1114, @skriss)
  * Update to go 1.11 (#1069, @gliptak)
  * Update CHANGELOGs (#1063, @wwitzel3)
  * Initialize empty schedule metrics on server init (#1054, @cbeneke)
  * Added brew reference (#1051, @omerlh)
  * Remove default token from all service accounts (#1048, @ncdc)
  * Add pprof support to the Ark server (#234, @ncdc)

## v0.10.0
#### 2018-11-15

### Download
- https://github.com/heptio/ark/releases/tag/v0.10.0

### Highlights
- We've introduced two new custom resource definitions, `BackupStorageLocation` and `VolumeSnapshotLocation`, that replace the `Config` CRD from
previous versions. As part of this, you may now configure more than one possible location for where backups and snapshots are stored, and when you
create a `Backup` you can select the location where you'd like that particular backup to be stored. See the [Locations documentation][2] for an overview
of this feature.
- Ark's plugin system has been significantly refactored to improve robustness and ease of development. Plugin processes are now automatically restarted
if they unexpectedly terminate. Additionally, plugin binaries can now contain more than one plugin implementation (e.g. and object store *and* a block store,
or many backup item actions).
- The sync process, which ensures that Backup custom resources exist for each backup in object storage, has been revamped to run much more frequently (once
per minute rather than once per hour), to use significantly fewer cloud provider API calls, and to not generate spurious Kubernetes API errors.
- Ark can now be configured to store all data under a prefix within an object storage bucket. This means that you no longer need a separate bucket per Ark
instance; you can now have all of your clusters' Ark backups go into a single bucket, with each cluster having its own prefix/subdirectory
within that bucket.
- Restic backup data is now automatically stored within the same bucket/prefix as the rest of the Ark data. A separate bucket is no longer required (or allowed).
- Ark resources (backups, restores, schedules) can now be bulk-deleted through the `ark` CLI, using the `--all` or `--selector` flags, or by specifying 
multiple resource names as arguments to the `delete` commands.
- The `ark` CLI now supports waiting for backups and restores to complete with the `--wait` flag for `ark backup create` and `ark restore create`
- Restores can be created directly from the most recent backup for a schedule, using `ark restore create --from-schedule SCHEDULE_NAME`

### Breaking Changes

Heptio Ark v0.10 contains a number of breaking changes.  Upgrading will require some additional steps beyond just updating your client binary and your
container image tag. We've provided a [detailed set of instructions][1] to help you with the upgrade process. **Please read and follow these instructions 
carefully to ensure a successful upgrade!**

- The `Config` CRD has been replaced by `BackupStorageLocation` and `VolumeSnapshotLocation` CRDs. 
- The interface for external plugins (object/block stores, backup/restore item actions) has changed. If you have authored any custom plugins, they'll 
need to be updated for v0.10.
    - The [`ObjectStore.ListCommonPrefixes`](https://github.com/vmware-tanzu/velero/blob/main/pkg/cloudprovider/object_store.go#L50) signature has changed to add a `prefix` parameter.
    - Registering plugins has changed. Create a new plugin server with the `NewServer` function, and register plugins with the appropriate functions. See the [`Server`](https://github.com/vmware-tanzu/velero/blob/main/pkg/plugin/server.go#L37) interface for details.
- The organization of Ark data in object storage has changed. Existing data will need to be moved around to conform to the new layout.

### All Changes
-   [b9de44ff](https://github.com/heptio/ark/commit/b9de44ff)	update docs to reference config/ dir within release tarballs
-   [eace0255](https://github.com/heptio/ark/commit/eace0255)	goreleaser: update example image tags to match version being released
-   [cff02159](https://github.com/heptio/ark/commit/cff02159)	add rbac content, rework get-started for NodePort and publicUrl, add versioning information
-   [fa14255e](https://github.com/heptio/ark/commit/fa14255e)	add content for docs issue 819
-   [22959071](https://github.com/heptio/ark/commit/22959071)	add doc explaining locations
-   [e5556fe6](https://github.com/heptio/ark/commit/e5556fe6)	Added qps and burst to server's client
-   [9ae861c9](https://github.com/heptio/ark/commit/9ae861c9)	Support a separate URL base for pre-signed URLs
-   [698420b6](https://github.com/heptio/ark/commit/698420b6)	Update storage-layout-reorg-v0.10.md
-   [6c9e1f18](https://github.com/heptio/ark/commit/6c9e1f18)	lower some noisy logs to debug level
-   [318fd8a8](https://github.com/heptio/ark/commit/318fd8a8)	add troubleshooting for loadbalancer restores
-   [defb8aa8](https://github.com/heptio/ark/commit/defb8aa8)	remove code that checks directly for a backup from restore controller
-   [7abe1156](https://github.com/heptio/ark/commit/7abe1156)	Move clearing up of metadata before plugin's actions
-   [ec013e6f](https://github.com/heptio/ark/commit/ec013e6f)	Document upgrading plugins in the deployment
-   [d6162e94](https://github.com/heptio/ark/commit/d6162e94)	fix goreleaser bugs
-   [a15df276](https://github.com/heptio/ark/commit/a15df276)	Add correct link and change role
-   [46bed015](https://github.com/heptio/ark/commit/46bed015)	add 0.10 breaking changes warning to readme in main
-   [e3a7d6a2](https://github.com/heptio/ark/commit/e3a7d6a2)	add content for issue 994
-   [400911e9](https://github.com/heptio/ark/commit/400911e9)	address docs issue #978
-   [b818cc27](https://github.com/heptio/ark/commit/b818cc27)	don't require a default provider VSL if there's only 1
-   [90638086](https://github.com/heptio/ark/commit/90638086)	v0.10 changelog
-   [6e2166c4](https://github.com/heptio/ark/commit/6e2166c4)	add docs page on versions and upgrading
-   [18b434cb](https://github.com/heptio/ark/commit/18b434cb)	goreleaser scripts for building/creating a release on a workstation
-   [bb65d67a](https://github.com/heptio/ark/commit/bb65d67a)	update restic prerequisite with min k8s version
-   [b5a2ccd5](https://github.com/heptio/ark/commit/b5a2ccd5)	Silence git detached HEAD advice in build container
-   [67749141](https://github.com/heptio/ark/commit/67749141)	instructions for upgrading to v0.10
-   [516422c2](https://github.com/heptio/ark/commit/516422c2)	sync controller: fill in missing .spec.storageLocation
- 	[195e6aaf](https://github.com/heptio/ark/commit/195e6aaf)	fix bug preventing PV snapshots from v0.10 backups from restoring
- 	[bca58516](https://github.com/heptio/ark/commit/bca58516)	Run 'make update' to update formatting
- 	[573ce7d0](https://github.com/heptio/ark/commit/573ce7d0)	Update formatting script
- 	[90d9be59](https://github.com/heptio/ark/commit/90d9be59)	support restoring/deleting legacy backups with .status.volumeBackups
- 	[ef194972](https://github.com/heptio/ark/commit/ef194972)	rename variables #967
- 	[6d4e702c](https://github.com/heptio/ark/commit/6d4e702c)	fix broken link
- 	[596eea1b](https://github.com/heptio/ark/commit/596eea1b)	restore storageclasses before pvs and pvcs
- 	[f014cab1](https://github.com/heptio/ark/commit/f014cab1)	backup describer: show snapshot summary by default, details optionally
- 	[8acc66d0](https://github.com/heptio/ark/commit/8acc66d0)	remove pvProviderExists param from NewRestoreController
- 	[57ce590f](https://github.com/heptio/ark/commit/57ce590f)	create a struct for multiple return of same type in restore_contoroller #967
- 	[028fafb6](https://github.com/heptio/ark/commit/028fafb6)	Corrected grammatical  error
- 	[db856aff](https://github.com/heptio/ark/commit/db856aff)	Specify return arguments
- 	[9952dfb0](https://github.com/heptio/ark/commit/9952dfb0)	Address #424: Add CRDs to list of prioritized resources
- 	[cf2c2714](https://github.com/heptio/ark/commit/cf2c2714)	fix bugs in GetBackupVolumeSnapshots and add test
- 	[ec124673](https://github.com/heptio/ark/commit/ec124673)	remove all references to Config from docs/examples
- 	[c36131a0](https://github.com/heptio/ark/commit/c36131a0)	remove Config-related code
- 	[406b50a7](https://github.com/heptio/ark/commit/406b50a7)	update restore process using snapshot locations
- 	[268080ad](https://github.com/heptio/ark/commit/268080ad)	avoid panics if can't get block store during deletion
- 	[4a03370f](https://github.com/heptio/ark/commit/4a03370f)	update backup deletion controller for snapshot locations
- 	[38c72b8c](https://github.com/heptio/ark/commit/38c72b8c)	include snapshot locations in created schedule's backup spec
- 	[0ec2de55](https://github.com/heptio/ark/commit/0ec2de55)	azure: update blockstore to allow storing snaps in different resource group
- 	[35bb533c](https://github.com/heptio/ark/commit/35bb533c)	close gzip writer before uploading volumesnapshots file
- 	[da9ed38c](https://github.com/heptio/ark/commit/da9ed38c)	store volume snapshot info as JSON in backup storage
- 	[e24248e0](https://github.com/heptio/ark/commit/e24248e0)	add --volume-snapshot-locations flag to ark backup create
- 	[df07b7dc](https://github.com/heptio/ark/commit/df07b7dc)	update backup code to work with volume snapshot locations
- 	[4af89fa8](https://github.com/heptio/ark/commit/4af89fa8)	add unit test for getDefaultVolumeSnapshotLocations
- 	[02f50b9c](https://github.com/heptio/ark/commit/02f50b9c)	add default-volume-snapshot-locations to server cmd
- 	[1aa712d2](https://github.com/heptio/ark/commit/1aa712d2)	Default and validate VolumeSnapshotLocations
- 	[bbf76985](https://github.com/heptio/ark/commit/bbf76985)	add create CLI command for snapshot locations
- 	[aeb221ea](https://github.com/heptio/ark/commit/aeb221ea)	Add printer for snapshot locations
- 	[ffc612ac](https://github.com/heptio/ark/commit/ffc612ac)	Add volume snapshot CLI get command
- 	[f20342aa](https://github.com/heptio/ark/commit/f20342aa)	Add VolumeLocation and Snapshot.
- 	[7172db8a](https://github.com/heptio/ark/commit/7172db8a)	upgrade to restic v0.9.3
- 	[99adc4fa](https://github.com/heptio/ark/commit/99adc4fa)	Remove broken references to docs that are not existing
- 	[474efde6](https://github.com/heptio/ark/commit/474efde6)	Fixed relative link for image
- 	[41735154](https://github.com/heptio/ark/commit/41735154)	don't require a default backup storage location to exist
- 	[0612c5de](https://github.com/heptio/ark/commit/0612c5de)	templatize error message in DeleteOptions
- 	[66bcbc05](https://github.com/heptio/ark/commit/66bcbc05)	add support for bulk deletion to ark schedule delete
- 	[3af43b49](https://github.com/heptio/ark/commit/3af43b49)	add azure-specific code to support multi-location restic
- 	[d009163b](https://github.com/heptio/ark/commit/d009163b)	update restic to support multiple backup storage locations
- 	[f4c99c77](https://github.com/heptio/ark/commit/f4c99c77)	Change link for the support matrix
- 	[91e45d56](https://github.com/heptio/ark/commit/91e45d56)	Fix broken storage providers link
- 	[ed0eb865](https://github.com/heptio/ark/commit/ed0eb865)	fix backup storage location example YAMLs
- 	[eb709b8f](https://github.com/heptio/ark/commit/eb709b8f)	only sync a backup location if it's changed since last sync
- 	[af3af1b5](https://github.com/heptio/ark/commit/af3af1b5)	clarify Azure resource group usage in docs
- 	[9fdf8513](https://github.com/heptio/ark/commit/9fdf8513)	Minor code cleanup
- 	[2073e15a](https://github.com/heptio/ark/commit/2073e15a)	Fix formatting for live site
- 	[0fc3e8d8](https://github.com/heptio/ark/commit/0fc3e8d8)	add documentation on running Ark on-premises
- 	[e46e89cb](https://github.com/heptio/ark/commit/e46e89cb)	have restic share main Ark bucket
- 	[42b54586](https://github.com/heptio/ark/commit/42b54586)	refactor to make valid dirs part of an object store layout
- 	[8bc7e4f6](https://github.com/heptio/ark/commit/8bc7e4f6)	store backups & restores in backups/, restores/ subdirs in obj storage
- 	[e3232b7e](https://github.com/heptio/ark/commit/e3232b7e)	add support for bulk deletion to ark restore delete
- 	[17be71e1](https://github.com/heptio/ark/commit/17be71e1)	remove deps used for docs gen
- 	[20635106](https://github.com/heptio/ark/commit/20635106)	remove script for generating docs
- 	[6fd9ea9d](https://github.com/heptio/ark/commit/6fd9ea9d)	remove cli reference docs and related scripts
- 	[4833607a](https://github.com/heptio/ark/commit/4833607a)	Fix infinite sleep in fsfreeze container
- 	[7668bfd4](https://github.com/heptio/ark/commit/7668bfd4)	Add links for Portworx plugin support
- 	[468006e6](https://github.com/heptio/ark/commit/468006e6)	Fix Portworx name in doc
- 	[e6b44539](https://github.com/heptio/ark/commit/e6b44539)	Make fsfreeze image building consistent
- 	[fcd27a13](https://github.com/heptio/ark/commit/fcd27a13)	get a new metadata accessor after calling backup item actions
- 	[ffef86e3](https://github.com/heptio/ark/commit/ffef86e3)	Adding support for the AWS_CLUSTER_NAME env variable allowing to claim volumes ownership
- 	[cda3dff8](https://github.com/heptio/ark/commit/cda3dff8)	Document single binary plugins
- 	[f049e078](https://github.com/heptio/ark/commit/f049e078)	Remove ROADMAP.md, update ZenHub link to Ark board
- 	[94617b30](https://github.com/heptio/ark/commit/94617b30)	convert all controllers to use genericController, logContext -> log
- 	[779cb428](https://github.com/heptio/ark/commit/779cb428)	Document SignatureDoesNotMatch error and triaging
- 	[7d8813a9](https://github.com/heptio/ark/commit/7d8813a9)	move ObjectStore mock into pkg/cloudprovider/mocks
- 	[f0edf733](https://github.com/heptio/ark/commit/f0edf733)	add a BackupStore to pkg/persistence that supports prefixes
- 	[af64069d](https://github.com/heptio/ark/commit/af64069d)	create pkg/persistence and move relevant code from pkg/cloudprovider into it
- 	[29d75d72](https://github.com/heptio/ark/commit/29d75d72)	move object and block store interfaces to their own files
- 	[211aa7b7](https://github.com/heptio/ark/commit/211aa7b7)	Set schedule labels to subsequent backups
- 	[d34994cb](https://github.com/heptio/ark/commit/d34994cb)	set azure restic env vars based on default backup location's config
- 	[a50367f1](https://github.com/heptio/ark/commit/a50367f1)	Regenerate CLI docs
- 	[7bc27bbb](https://github.com/heptio/ark/commit/7bc27bbb)	Pin cobra version
- 	[e94277ac](https://github.com/heptio/ark/commit/e94277ac)	Update pflag version
- 	[df69b274](https://github.com/heptio/ark/commit/df69b274)	azure: update documentation and examples
- 	[cb321db2](https://github.com/heptio/ark/commit/cb321db2)	azure: refactor to not use helpers/ pkg, validate all env/config inputs
- 	[9d7ea748](https://github.com/heptio/ark/commit/9d7ea748)	azure: support different RGs/storage accounts per backup location
- 	[cd4e9f53](https://github.com/heptio/ark/commit/cd4e9f53)	azure: fix for breaking change in blob.GetSASURI
- 	[a440029c](https://github.com/heptio/ark/commit/a440029c)	bump Azure SDK version and include storage mgmt package
- 	[b31e25bf](https://github.com/heptio/ark/commit/b31e25bf)	server: remove unused code, replace deprecated func
- 	[729d7339](https://github.com/heptio/ark/commit/729d7339)	controllers: take a newPluginManager func in constructors
- 	[6445dbf1](https://github.com/heptio/ark/commit/6445dbf1)	Update examples and docs for backup locations
- 	[133dc185](https://github.com/heptio/ark/commit/133dc185)	backup sync: process the default location first
- 	[7a1e6d16](https://github.com/heptio/ark/commit/7a1e6d16)	generic controller: allow controllers with only a resync func
- 	[6f7bfe54](https://github.com/heptio/ark/commit/6f7bfe54)	remove Config CRD's BackupStorageProvider & other obsolete code
- 	[bd4d97b9](https://github.com/heptio/ark/commit/bd4d97b9)	move server's defaultBackupLocation into config struct
- 	[0e94fa37](https://github.com/heptio/ark/commit/0e94fa37)	update sync controller for backup locations
- 	[2750aa71](https://github.com/heptio/ark/commit/2750aa71)	Use backup storage location during restore
- 	[20f89fbc](https://github.com/heptio/ark/commit/20f89fbc)	use the default backup storage location for restic
- 	[833a6307](https://github.com/heptio/ark/commit/833a6307)	Add storage location to backup get/describe
- 	[cf7c8587](https://github.com/heptio/ark/commit/cf7c8587)	download request: fix setting of log level for plugin manager
- 	[3234124a](https://github.com/heptio/ark/commit/3234124a)	backup deletion: fix setting of log level in plugin manager
- 	[74043ab4](https://github.com/heptio/ark/commit/74043ab4)	download request controller: fix bug in determining expiration
- 	[7007f198](https://github.com/heptio/ark/commit/7007f198)	refactor download request controller test and add test cases
- 	[8f534615](https://github.com/heptio/ark/commit/8f534615)	download request controller: use backup location for object store
- 	[bab08ed1](https://github.com/heptio/ark/commit/bab08ed1)	backup deletion controller: use backup location for object store
- 	[c6f488f7](https://github.com/heptio/ark/commit/c6f488f7)	Use backup location in the backup controller
- 	[06b5af44](https://github.com/heptio/ark/commit/06b5af44)	add create and get CLI commands for backup locations
- 	[adbcd370](https://github.com/heptio/ark/commit/adbcd370)	add --default-backup-storage-location flag to server cmd
- 	[2a34772e](https://github.com/heptio/ark/commit/2a34772e)	Add --storage-location argument to create commands
- 	[56f16170](https://github.com/heptio/ark/commit/56f16170)	Correct metadata for BackupStorageLocationList
- 	[345c3c39](https://github.com/heptio/ark/commit/345c3c39)	Generate clients for BackupStorageLocation
- 	[a25eb032](https://github.com/heptio/ark/commit/a25eb032)	Add BackupStorageLocation API type
- 	[575c4ddc](https://github.com/heptio/ark/commit/575c4ddc)	apply annotations on single line, no restore mode
- 	[030ea6c0](https://github.com/heptio/ark/commit/030ea6c0)	minor word updates and command wrapping
- 	[d32f8dbb](https://github.com/heptio/ark/commit/d32f8dbb)	Update hooks/fsfreeze example
- 	[342a1c64](https://github.com/heptio/ark/commit/342a1c64)	add an ark bug command
- 	[9c11ba90](https://github.com/heptio/ark/commit/9c11ba90)	Add DigitalOcean to S3-compatible backup providers
- 	[ea50ebf2](https://github.com/heptio/ark/commit/ea50ebf2)	Fix map merging logic
- 	[9508e4a2](https://github.com/heptio/ark/commit/9508e4a2)	Switch Config CRD elements to server flags
- 	[0c3ac67b](https://github.com/heptio/ark/commit/0c3ac67b)	start using a namespaced label on restored objects, deprecate old label
- 	[6e53aa03](https://github.com/heptio/ark/commit/6e53aa03)	Bring back 'make local'
- 	[5acccaa7](https://github.com/heptio/ark/commit/5acccaa7)	add bulk deletion support to ark backup delete
- 	[3aa241a7](https://github.com/heptio/ark/commit/3aa241a7)	Preserve node ports during restore when annotations hold specification.
- 	[c5f5862c](https://github.com/heptio/ark/commit/c5f5862c)	Add --wait support to ark backup create
- 	[eb6f742b](https://github.com/heptio/ark/commit/eb6f742b)	Document CRD not found errors
- 	[fb4d507c](https://github.com/heptio/ark/commit/fb4d507c)	Extend doc about synchronization
- 	[e7bb5926](https://github.com/heptio/ark/commit/e7bb5926)	Add --wait support to `ark restore create`
- 	[8ce513ac](https://github.com/heptio/ark/commit/8ce513ac)	Only delete unused backup if they are complete
- 	[1c26fbde](https://github.com/heptio/ark/commit/1c26fbde)	remove SnapshotService, replace with direct BlockStore usage
- 	[13051218](https://github.com/heptio/ark/commit/13051218)	Refactor plugin management
- 	[74dbf387](https://github.com/heptio/ark/commit/74dbf387)	Add restore failed phase and metrics
- 	[8789ae5c](https://github.com/heptio/ark/commit/8789ae5c)	update testify to latest released version
- 	[fe9d61a9](https://github.com/heptio/ark/commit/fe9d61a9)	Add schedule command info to quickstart
- 	[ca5656c2](https://github.com/heptio/ark/commit/ca5656c2)	fix bug preventing backup item action item updates from saving
- 	[d2e629f5](https://github.com/heptio/ark/commit/d2e629f5)	Delete backups from etcd if they're not in storage
- 	[625ba481](https://github.com/heptio/ark/commit/625ba481)	Fix ZenHub link on Readme.md
- 	[dcae6eb0](https://github.com/heptio/ark/commit/dcae6eb0)	Update gcp-config.md
- 	[06d6665a](https://github.com/heptio/ark/commit/06d6665a)	check s3URL scheme upon AWS ObjectStore Init()
- 	[cc359f6e](https://github.com/heptio/ark/commit/cc359f6e)	Add contributor docs for our ZenHub usage
- 	[f6204562](https://github.com/heptio/ark/commit/f6204562)	cleanup service account action log statement
- 	[450fa72f](https://github.com/heptio/ark/commit/450fa72f)	Initialize schedule Prometheus metrics to have them created beforehand (see https://prometheus.io/docs/practices/instrumentation/#avoid-missing-metrics)
- 	[39c4267a](https://github.com/heptio/ark/commit/39c4267a)	Clarify that object storage should per-cluster
- 	[78cbdf95](https://github.com/heptio/ark/commit/78cbdf95)	delete old deletion requests for backup when processing a new one
- 	[85a61b8e](https://github.com/heptio/ark/commit/85a61b8e)	return nil error if 404 encountered when deleting snapshots
- 	[a2a7dbda](https://github.com/heptio/ark/commit/a2a7dbda)	fix tagging latest by using make's ifeq
- 	[b4a52e45](https://github.com/heptio/ark/commit/b4a52e45)	Add commands for context to the bug template
- 	[3efe6770](https://github.com/heptio/ark/commit/3efe6770)	Update Ark library code to work with Kubernetes 1.11
- 	[7e8c8c69](https://github.com/heptio/ark/commit/7e8c8c69)	Add some basic troubleshooting commands
- 	[d1955120](https://github.com/heptio/ark/commit/d1955120)	require namespace for backups/etc. to exist at server startup
- 	[683f7afc](https://github.com/heptio/ark/commit/683f7afc)	switch to using .status.startTimestamp for sorting backups
- 	[b71a37db](https://github.com/heptio/ark/commit/b71a37db)	Record backup completion time before uploading
- 	[217084cd](https://github.com/heptio/ark/commit/217084cd)	Add example ark version command to issue templates
- 	[040788bb](https://github.com/heptio/ark/commit/040788bb)	Add minor improvements and aws example<Plug>delimitMateCR
- 	[5b89f7b6](https://github.com/heptio/ark/commit/5b89f7b6)	Skip backup sync if it already exists in k8s
- 	[c6050845](https://github.com/heptio/ark/commit/c6050845)	restore controller: switch to 'c' for receiver name
- 	[706ae07d](https://github.com/heptio/ark/commit/706ae07d)	enable a schedule to be provided as the source for a restore
- 	[aea68414](https://github.com/heptio/ark/commit/aea68414)	fix up Slack link in troubleshooting on main branch
- 	[bb8e2e91](https://github.com/heptio/ark/commit/bb8e2e91)	Document how to run the Ark server locally
- 	[dc84e591](https://github.com/heptio/ark/commit/dc84e591)	Remove outdated namespace deletion content
- 	[23abbc9a](https://github.com/heptio/ark/commit/23abbc9a)	fix paths
- 	[f0426538](https://github.com/heptio/ark/commit/f0426538)	use posix-compliant conditional for checking TAG_LATEST
- 	[cf336d80](https://github.com/heptio/ark/commit/cf336d80)	Added new templates
- 	[795dc262](https://github.com/heptio/ark/commit/795dc262)	replace pkg/restore's osFileSystem with pkg/util/filesystem's
- 	[eabef085](https://github.com/heptio/ark/commit/eabef085)	Update generated Ark code based on the 1.11 k8s.io/code-generator script
- 	[f5eac0b4](https://github.com/heptio/ark/commit/f5eac0b4)	Update vendored library code for Kubernetes 1.11

[1]: https://heptio.github.io/velero/v0.10.0/upgrading-to-v0.10
[2]: locations.md
