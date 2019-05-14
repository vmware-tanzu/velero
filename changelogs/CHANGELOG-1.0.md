## v1.0.0
#### 2019-05-20

### Highlights
- We've added a new command, `velero install`, to make it easier to get up and running with Velero. This CLI command replaces the static YAML installation files that were previously part of release tarballs. See the updated [install instructions][3] for more information.
- We've made a number of improvements to the plugin framework:
    - we've reorganized the relevant packages to minimize the import surface for plugin authors
    - all plugins are now wrapped in panic handlers that will report information on panics back to Velero
    - Velero's `--log-level` flag is now passed to plugin implementations
    - Errors logged within plugins are now annotated with the file/line of where the error occurred
    - Restore item actions can now optionally return a list of additional related items that should be restored
    - Restore item actions can now indicate that an item *should not* be restored
- For Azure installation, the `cloud-credentials` secret can now be created from a file containing a list of environment variables. Note that `velero install` always uses this method of providing credentials for Azure. For more details, see [Run on Azure][0].
- We've added a new phase, `PartiallyFailed`, for both backups and restores. This new phase is used for backups/restores that successfully process some but not all of their items.
- We removed all legacy Ark references, including API types, prometheus metrics, restic & hook annotations, etc.
- The restic integration remains a **beta feature**. Please continue to try it out and provide feedback, and we'll be working over the next couple of releases to bring it to GA.

### Breaking Changes

#### API
* All legacy Ark data types and pre-1.0 compatibility code has been removed. Users should migrate any backups created pre-v0.11.0 with the `velero migrate-backups` command, available in [v0.11.1][2].

#### Image
* The base container image has been switched to `ubuntu:bionic`

#### Labels/Annotations/Metrics
* The "ark" annotations for specifying hooks are no longer supported, and have been replaced with "velero"-based equivalents.
* The "ark" annotation for specifying restic backups is no longer supported, and has been replaced with a "velero"-based equivalent.
* The "ark" prometheus metrics no longer exist, and have been replaced with "velero"-based equivalents.

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

### Download
- https://github.com/heptio/velero/releases/tag/v1.0.0

### Container Image
`gcr.io/heptio-images/velero:v1.0.0`

### Documentation
https://velero.io/docs/v1.0.0/

### Upgrading
To upgrade from a previous version of Velero, see our [upgrade instructions][1].

### All Changes
* Change base images to ubuntu:bionic (#1488, @skriss)
* Expose the timestamp of the last successful backup in a gauge (#1448, @fabito)
* check backup existence before download (#1447, @fabito)
* Use `latest` image tag if no version information is provided at build time (#1439, @nrb)
* switch from `restic stats` to `restic snapshots` for checking restic repository existence (#1416, @skriss)
* GCP: add optional 'project' config to volume snapshot location for if snapshots are in a different project than the IAM account (#1405, @skriss)
* Disallow bucket names starting with '-' (#1407, @nrb)
* Shorten label values when they're longer than 63 characters (#1392, @anshulc)
* Fail backup if it already exists in object storage. (#1390, @ncdc,carlisia)
* Add PartiallyFailed phase for backups, log + continue on errors during backup process (#1386, @skriss)
* Remove deprecated "hooks" for backups (they've been replaced by "pre hooks") (#1384, @skriss)
* Restic repo ensurer: return error if new repository does not become ready within a minute, and fix channel closing/deletion (#1367, @skriss)
* Support non-namespaced names for built-in plugins (#1366, @nrb)
* Change container base images to debian:stretch-slim and upgrade to go 1.12 (#1365, @skriss)
* Azure: allow credentials to be provided in a .env file (path specified by $AZURE_CREDENTIALS_FILE), formatted like (#1364, @skriss):
    ```
    AZURE_TENANT_ID=${AZURE_TENANT_ID}
    AZURE_SUBSCRIPTION_ID=${AZURE_SUBSCRIPTION_ID}
    AZURE_CLIENT_ID=${AZURE_CLIENT_ID}
    AZURE_CLIENT_SECRET=${AZURE_CLIENT_SECRET}
    AZURE_RESOURCE_GROUP=${AZURE_RESOURCE_GROUP} 
    ```
* Instantiate the plugin manager with the per-restore logger so plugin logs are captured in the per-restore log (#1358, @skriss)
* Add gauge metrics for number of existing backups and restores (#1353, @fabito)
* Set default TTL for backups (#1352, @vorar)
* Validate that there can't be any duplicate plugin name, and that the name format is `example.io/name`. (#1339, @carlisia)
* AWS/Azure/GCP: fail fast if unsupported keys are provided in BackupStorageLocation/VolumeSnapshotLocation config (#1338, @skriss)
* `velero backup logs` & `velero restore logs`: show helpful error message if backup/restore does not exist or is not finished processing (#1337, @skriss)
* Add support for allowing a RestoreItemAction to skip item restore. (#1336, @sseago)
* Improve error message around invalid S3 URLs, and gracefully handle trailing backslashes. (#1331, @skriss)
* Set backup's start timestamp before patching it to InProgress so start times display in `velero backup get` while in progress (#1330, @skriss)
* Added ability to dynamically disable controllers (#1326, @amanw)
* Remove deprecated code in preparation for v1.0 release (#1323, @skriss):
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
    - remove VolumeBackups field and related code from Backup's status
* Rename BlockStore plugin to VolumeSnapshotter (#1321, @skriss)
* Bump plugin ProtocolVersion to version 2 (#1319, @carlisia)
* Remove Warning field from restore item action output (#1318, @skriss)
* Fix for #1312, use describe to determine if AWS EBS snapshot is encrypted and explicitly pass that value in EC2 CreateVolume call. (#1316, @mstump)
* Allow restic restore helper image name to be optionally specified via ConfigMap (#1311, @skriss)
* Compile only once to lower the initialization cost for regexp.MustCompile. (#1306, @pei0804)
* Enable restore item actions to return additional related items to be restored; have pods return PVCs and PVCs return PVs (#1304, @skriss)
* Log error locations from plugin logger, and don't overwrite them in the client logger if they exist already (#1301, @skriss)
* Send stack traces from plugin errors to Velero via gRPC so error location info can be logged (#1300, @skriss)
* Azure: restore volumes in the original region's zone (#1298, @sylr)
* Check for and exclude hostPath-based persistent volumes from restic backup (#1297, @skriss)
* Make resticrepositories non-restorable resources (#1296, @skriss)
* Gracefully handle failed API groups from the discovery API (#1293, @fabito)
* Add `velero install` command for basic use cases. (#1287, @nrb)
* Collect 3 new metrics: backup_deletion_{attempt|failure|success}_total (#1280, @fabito)
* Pass --log-level flag to internal/external plugins, matching Velero server's log level (#1278, @skriss)
* AWS EBS Volume IDs now contain AZ (#1274, @tsturzl)
* Add panic handlers to all server-side plugin methods (#1270, @skriss)
* Move all the interfaces and associated types necessary to implement all of the Velero plugins to under the new package `velero`. (#1264, @carlisia)
* Update `velero restore` to not open every single file open during extraction of the data (#1261, @asaf)
* Remove restore code that waits for a PV to become Available (#1254, @skriss)
* Improve `describe` output
* Move Phase to right under Metadata(name/namespace/label/annotations)
* Move Validation errors: section right after Phase: section and only show it if the item has a phase of FailedValidation
* For restores move Warnings and Errors under Validation errors. Leave their display as is. (#1248, @DheerajSShetty)
* Don't remove storage class from a persistent volume when restoring it (#1246, @skriss)
* Need to defer closing the the ReadCloser in ObjectStoreGRPCServer.GetObject (#1236, @DheerajSShetty)
* Update Kubernetes dependencies to match v1.12, and update Azure SDK to v19.0.0 (GA) (#1231, @skriss)
* Remove pkg/util/collections/map_utils.go, replace with structured API types and apimachinery's unstructured helpers (#1146, @skriss)
* Add original resource (from backup) to restore item action interface (#1123, @mwieczorek)


[0]: https://velero.io/docs/v1.0.0/azure-config
[1]: https://velero.io/docs/v1.0.0/upgrade-to-1.0
[2]: https://github.com/heptio/velero/releases/tag/v0.11.1
[3]: https://velero.io/docs/v1.0.0/install-overview
