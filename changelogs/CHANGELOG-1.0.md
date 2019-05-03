## v1.0.0-beta.1
#### 2019-05-03

We're excited to release our first beta for v1.0! This beta includes all key features for v1.0 plus a number of bug fixes and documentation updates. See the **All Changes** section below for details. Please test it out in your non-critical environments!

We'll continue to fix bugs and make minor changes, and we expect to ship at least one more beta or release candidate prior to the general availability of v1.0.0.

### Download
- https://github.com/heptio/velero/releases/tag/v1.0.0-beta.1

### Container Image
`gcr.io/heptio-images/velero:v1.0.0-beta.1`

### Documentation
https://heptio.github.io/velero/v1.0.0-beta.1/

### All Changes
* Add PartiallyFailed phase for restores (#1389, @skriss)
* Add PartiallyFailed phase for backups, log + continue on errors during backup process (#1386, @skriss)
* Switch from `restic stats` to `restic snapshots` for checking restic repository existence (#1416, @skriss)
* Disallow bucket names starting with '-' (#1407, @nrb)
* Shorten label values when they're longer than 63 characters (#1392, @anshulc)
* Fail backup if it already exists in object storage. (#1390, @ncdc,carlisia)
* Install command: Use `latest` image tag if no version information is provided at build time (#1439, @nrb)
* GCP: add optional 'project' config to volume snapshot location for if snapshots are in a different project than the IAM account (#1405, @skriss)
* Azure: restore disks with zone information if it exists (#1298, @sylr)
* Replace config/ with examples/ in release tarball (#1406, @skriss)


## v1.0.0-alpha.2
#### 2019-04-24

### Download
- https://github.com/heptio/velero/releases/tag/v1.0.0-alpha.2

### Container Image
`gcr.io/heptio-images/velero:v1.0.0-alpha.2`

### Highlights
Our second v1.0 alpha is ready for testing! Please try it out in your non-critial environments. This alpha contains a bunch of bug fixes and smaller enhancements. See the **All Changes** section below for details.

We expect that our next release will be `v1.0.0-beta.1`, meaning that all key features for v1.0.0 will be included. Following that release, we'll continue to fix
bugs and make minor improvements, and we expect to ship at least one more beta and/or release candidate prior to the general availability of v1.0.0.

### All Changes
* restic repo ensurer: return error if new repository does not become ready within a minute, and fix channel closing/deletion (#1367, @skriss)
* remove deprecated "hooks" for backups (they've been replaced by "pre hooks") (#1384, @skriss)
* fix setting up restic identifiers when fully-qualified plugin names are used (#1377, @jmontleon)
* add `--namespace` flag to `velero install` (@1380, @nrb)
* GCP: allow `storageLocation` to be specified as a config parameter for VolumeSnapshotLocations (#1375, @ctrox)
* add new prometheus gauge metrics `backup_total` and `restore_total` (#1353, @fabito)
* update install docs to use `velero install` (#1376 #1393 #1394, @nrb and @skriss)
* fix panic in API discovery when 1+ API groups cannot be reached (#1399, @skriss)
* fail backup if it already exists in object storage (#1390, @carlisia and @ncdc)

## v1.0.0-alpha.1
#### 2019-04-15

### Download
- https://github.com/heptio/velero/releases/tag/v1.0.0-alpha.1

### Highlights
We're excited to release our first alpha for v1.0! Please take it for a spin in your non-critical environments. Although we've finished the majority of the planned development work for v1.0, we are still working on a handful of items, so don't consider this alpha release to be fully feature-complete. Here's a quick rundown of the major changes in this release:

- We've added a new command, `velero install`, to make it easier to get up and running with Velero
- We've made a bunch of improvements to the plugin framework:
    - we've reorganized the relevant packages to minimize the import surface for plugin authors
    - all plugins are now wrapped in panic handlers that will report information on panics back to Velero
    - Velero's `--log-level` flag is now passed to plugin implementations
    - Errors logged within plugins are now annotated with the file/line of where the error occurred
    - Restore item actions can now optionally return a list of additional related items that should be restored
    - Restore item actions can now indicate that an item *should not* be restored
- The restic restore helper image used by Velero can now optionally be overridden via config map

### Breaking & Notable Changes

#### API
* All legacy Ark data types and pre-1.0 compatibility code has been removed. Users should migrate any backups created pre-v0.11.0 with the v0.11.1 migration command (not yet released)

#### Azure
* During installation, the `cloud-credentials` secret can now be created from a file, whose contents look like the following:
    ```
    AZURE_TENANT_ID=${AZURE_TENANT_ID}
    AZURE_SUBSCRIPTION_ID=${AZURE_SUBSCRIPTION_ID}
    AZURE_CLIENT_ID=${AZURE_CLIENT_ID}
    AZURE_CLIENT_SECRET=${AZURE_CLIENT_SECRET}
    AZURE_RESOURCE_GROUP=${AZURE_RESOURCE_GROUP}
    ```
  When using this method, the `cloud-credentials` secret should be mounted as a volume into the Velero deployment and daemon set, at the path `/credentials`. Additionally, the `$AZURE_CREDENTIALS_FILE` environment variable should be set to `/credentials/cloud` (the location of the file within the Velero pods). Note that `velero install` always uses this method of providing credentials for Azure.

#### Image
* The base container image has been switched to `debian:stretch-slim`

#### Plugin Development
* `BlockStore` plugins are now named `VolumeSnapshotter` plugins
* Plugin APIs have moved to reduce the import surface:
    * Plugin gRPC servers live in `github.com/heptio/velero/pkg/plugin/framework`
    * Plugin interface types live in `github.com/heptio/velero/pkg/plugin/velero`
* RestoreItemAction interface now takes the original item from the backup as a parameter
* RestoreItemAction plugins can now return additional items to restore
* RestoreItemAction plugins can now skip restoring an item
* Plugins may now send stack traces with errors to the Velero server, so that the errors may be put into the server log
* Plugins must now be "namespaced," using `example.domain.com/plugin-name` format
    * For external ObjectStore and VolumeSnapshotter plugins. this name will also be the provider name in BackupStorageLoction and VolumeSnapshotLocation objects
* `--log-level` flag is now passed to all plugins

#### Validation
* Configs for Azure, AWS, and GCP are now checked for invalid or extra keys, and the server is halted if any are found

### All Changes
* change container base images to debian:stretch-slim and upgrade to go 1.12 (#1365, @skriss)
* Azure: allow credentials to be provided in a .env file (#1364, @skriss)
* remove deprecated code in preparation for v1.0 release:
    - remove ark.heptio.com API group
    - remove support for reading ark-backup.json files from object storage
    - remove Ark field from RestoreResult type
    - remove support for "hook.backup.ark.heptio.com/..." annotations for specifying hooks
    - remove support for $HOME/.config/ark/ client config directory
    - remove support for restoring Azure snapshots using short snapshot ID formats in backup metadata
    - stop applying "velero-restore" label to restored resources and remove it from the API pkg
    - remove code that strips the "gc.ark.heptio.com" finalizer from backups
    - remove support for "backup.ark.heptio.com/..." annotations for requesting restic backups
    - remove "ark"-prefixed prometheus metrics
    - remove VolumeBackups field and related code from Backup's status (#1323, @skriss)
* Add velero install command for basic use cases. (#1287, @nrb)
* Support non-namespaced names for built-in plugins (#1366, @nrb)
* instantiate the plugin manager with the per-restore logger so plugin logs are captured in the per-restore log (#1358, @skriss)
* Validate that there can't be any duplicate plugin name, and that the name format is `example.io/name`. (#1339, @carlisia)
* Added ability to dynamically disable controllers (#1326, @amanw)
* set default TTL for backups (#1352, @vorar)
* aws/azure/gcp: fail fast if unsupported keys are provided in BackupStorageLocation/VolumeSnapshotLocation config (#1338, @skriss)
* velero backup logs & velero restore logs: show helpful error message if backup/restore does not exist or is not finished processing (#1337, @skriss)
* Add support for allowing a RestoreItemAction to skip item restore. (#1336, @sseago)
* Improve error message around invalid S3 URLs, and gracefully handle trailing backslashes. (#1331, @skriss)
* set backup's start timestamp before patching it to InProgress so start times display in `velero backup get` while in progress (#1330, @skriss)
* rename BlockStore plugin to VolumeSnapshotter (#1321, @skriss)
* Bump plugin ProtocolVersion to version 2 (#1319, @carlisia)
* remove Warning field from restore item action output (#1318, @skriss)
* Fix for #1312, use describe to determine if AWS EBS snapshot is encrypted and explicitly pass that value in EC2 CreateVolume call. (#1316, @mstump)
* Allow restic restore helper image name to be optionally specified via ConfigMap (#1311, @skriss)
* compile only once to lower the initialization cost for regexp.MustCompile. (#1306, @pei0804)
* enable restore item actions to return additional related items to be restored; have pods return PVCs and PVCs return PVs (#1304, @skriss)
* log error locations from plugin logger, and don't overwrite them in the client logger if they exist already (#1301, @skriss)
* Send stack traces from plugin errors to Velero via gRPC so error location info can be logged (#1300, @skriss)
* check for and exclude hostPath-based persistent volumes from restic backup (#1297, @skriss)
* make resticrepositories non-restorable resources (#1296, @skriss)
* gracefully handle failed API groups from the discovery API (#1293, @fabito)
* Collect 3 new metrics: backup_deletion_{attempt|failure|success}_total (#1280, @fabito)
* Pass --log-level flag to internal/external plugins, matching Velero server's log level (#1278, @skriss)
* AWS EBS Volume IDs now contain AZ (#1274, @tsturzl)
* add panic handlers to all server-side plugin methods (#1270, @skriss)
* Move all the interfaces and associated types necessary to implement all of the Velero plugins to under the new package `pkg/plugin/velero`. (#1264, @carlisia)
* Update velero restore to not open every single file open during extraction of the data (#1261, @asaf)
* remove restore code that waits for a PV to become Available (#1254, @skriss)
* Improve `describe` output:
    * Move Phase to right under Metadata(name/namespace/label/annotations)
    * Move Validation errors: section right after Phase: section and only show it if the item has a phase of FailedValidation
    * For restores move Warnings and Errors under Validation errors. Leave their display as is. (#1248, @DheerajSShetty)
* don't remove storageclass from a persistent volume when restoring it (#1246, @skriss)
* Need to defer closing the the ReadCloser in ObjectStoreGRPCServer.GetObject (#1236, @DheerajSShetty)
* update Kubernetes dependencies to match v1.12, and update Azure SDK to v19.0.0 (GA) (#1231, @skriss)
* remove pkg/util/collections/map_utils.go, replace with structured API types and apimachinery's unstructured helpers (#1146, @skriss)
* Add original resource (from backup) to restore item action interface (#1123, @mwieczorek)

### Coming in Future Alpha/Beta Releases:
- backup & restore phases will be modified to more clearly indicate successes, failures, and partial failures
- additional safety checks to ensure backups are never overwritten in object storage
- revised installation documentation that takes advantage of the `velero install` command
- as many additional stability and UX issues as we can get to
