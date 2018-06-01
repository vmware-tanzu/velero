# Changelog

#### [v0.8.2](https://github.com/heptio/ark/releases/tag/v0.8.2) - 2018-06-01

##### Bug Fixes:
  * Don't crash when a PVC is missing spec.volumeName (#520, @ncdc)

#### [v0.8.1](https://github.com/heptio/ark/releases/tag/v0.8.1) - 2018-04-23

##### Bug Fixes:
  * Azure: allow pre-v0.8.0 backups with disk snapshots to be restored and deleted (#446 #449, @skriss)

#### [v0.8.0](https://github.com/heptio/ark/releases/tag/v0.8.0) - 2018-04-19

##### Highlights:
  * Backup deletion has been completely revamped to make it simpler and less error-prone. As a user, you still use the `ark backup delete` command to request deletion of a backup and its associated cloud
  resources; behind the scenes, we've switched to using a new `DeleteBackupRequest` Custom Resource and associated controller for processing deletion requests.
  * We've reduced the number of required fields in the Ark config. For Azure, `location` is no longer required, and for GCP, `project` is not needed.
  * Ark now copies tags from volumes to snapshots during backup, and from snapshots to new volumes during restore. 

##### Breaking Changes:
  * Ark has moved back to a single namespace (`heptio-ark` by default) as part of #383.

##### All New Features:
  * Add global `--kubecontext` flag to Ark CLI (#296, @blakebarnett)
  * Azure: support cross-resource group restores of volumes (#356 #378, @skriss)
  * AWS/Azure/GCP: copy tags from volumes to snapshots, and from snapshots to volumes (#341, @skriss)
  * Replace finalizer for backup deletion with `DeleteBackupRequest` custom resource & controller (#383 #431, @ncdc @nrb)
  * Don't log warnings during restore if an identical object already exists in the cluster (#405, @nrb)
  * Add bash & zsh completion support (#384, @containscafeine)
  
##### Bug Fixes / Other Changes:
  * Error from the Ark CLI if attempting to restore a non-existent backup (#302, @ncdc)
  * Enable running the Ark server locally for development purposes (#334, @ncdc)
  * Add examples to `ark schedule create` documentation (#331, @lypht)
  * GCP: Remove `project` requirement from Ark config (#345, @skriss)
  * Add `--from-backup` flag to `ark restore create` and allow custom restore names (#342 #409, @skriss)
  * Azure: remove `location` requirement from Ark config (#344, @skriss)
  * Add documentation/examples for storing backups in IBM Cloud Object Storage (#321, @roytman)
  * Reduce verbosity of hooks logging (#362, @skriss)
  * AWS: Add minimal IAM policy to documentation (#363 #419, @hopkinsth)
  * Don't restore events (#374, @sanketjpatel)
  * Azure: reduce API polling interval from 60s to 5s (#359, @skriss)
  * Switch from hostPath to emptyDir volume type for minio example (#386, @containscafeine)
  * Add limit ranges as a prioritized resource for restores (#392, @containscafeine)
  * AWS: Add documentation on using Ark with kube2iam (#402, @domderen)
  * Azure: add node selector so Ark pod is scheduled on a linux node (#415, @ffd2subroutine)
  * Error from the Ark CLI if attempting to get logs for a non-existent restore (#391, @containscafeine)
  * GCP: Add minimal IAM policy to documentation (#429, @skriss @jody-frankowski)

##### Upgrading from v0.7.1:
  Ark v0.7.1 moved the Ark server deployment into a separate namespace, `heptio-ark-server`. As of v0.8.0 we've
  returned to a single namespace, `heptio-ark`, for all Ark-related resources. If you're currently running v0.7.1,
  here are the steps you can take to upgrade:

1. Execute the steps from the **Credentials and configuration** section for your cloud:
    * [AWS](https://heptio.github.io/ark/v0.8.0/aws-config#credentials-and-configuration)
    * [Azure](https://heptio.github.io/ark/v0.8.0/azure-config#credentials-and-configuration)
    * [GCP](https://heptio.github.io/ark/v0.8.0/gcp-config#credentials-and-configuration)

    When you get to the secret creation step, if you don't have your `credentials-ark` file handy, 
    you can copy the existing secret from your `heptio-ark-server` namespace into the `heptio-ark` namespace:
    ```bash
    kubectl get secret/cloud-credentials -n heptio-ark-server --export -o json | \
      jq '.metadata.namespace="heptio-ark"' | \
      kubectl apply -f -
    ```

2. You can now safely delete the `heptio-ark-server` namespace:
    ```bash
    kubectl delete namespace heptio-ark-server
    ```

3. Execute the commands from the **Start the server** section for your cloud:
    * [AWS](https://heptio.github.io/ark/v0.8.0/aws-config#start-the-server)
    * [Azure](https://heptio.github.io/ark/v0.8.0/azure-config#start-the-server)
    * [GCP](https://heptio.github.io/ark/v0.8.0/gcp-config#start-the-server)


#### [v0.7.1](https://github.com/heptio/ark/releases/tag/v0.7.1) - 2018-02-22

Bug Fixes:
  * Run the Ark server in its own namespace, separate from backups/schedules/restores/config (#322, @ncdc)

#### [v0.7.0](https://github.com/heptio/ark/releases/tag/v0.7.0) - 2018-02-15

New Features:
  * Run the Ark server in any namespace (#272, @ncdc)
  * Add ability to delete backups and their associated data (#252, @skriss)
  * Support both pre and post backup hooks (#243, @ncdc)

Bug Fixes / Other Changes:
  * Switch from Update() to Patch() when updating Ark resources (#241, @skriss)
  * Don't fail the backup if a PVC is not bound to a PV (#256, @skriss)
  * Restore serviceaccounts prior to workload controllers (#258, @ncdc)
  * Stop removing annotations from PVs when restoring them (#263, @skriss)
  * Update GCP client libraries (#249, @skriss)
  * Clarify backup and restore creation messages (#270, @nrb)
  * Update S3 bucket creation docs for us-east-1 (#285, @lypht)

#### [v0.6.0](https://github.com/heptio/ark/tree/v0.6.0) - 2017-11-30

Highlights:
  * **Plugins** - We now support user-defined plugins that can extend Ark functionality to meet your custom backup/restore needs without needing to be compiled into the core binary. We support pluggable block and object stores as well as per-item backup and restore actions that can execute arbitrary logic, including modifying the items being backed up or restored. For more information see the [documentation](docs/plugins.md), which includes a reference to a fully-functional sample plugin repository. (#174 #188 #206 #213 #215 #217 #223 #226)
  * **Describers** - The Ark CLI now includes `describe` commands for `backups`, `restores`, and `schedules` that provide human-friendly representations of the relevant API objects.

Breaking Changes:
  * The config object format has changed. In order to upgrade to v0.6.0, the config object will have to be updated to match the new format. See the [examples](examples) and [documentation](docs/config-definition.md) for more information.
  * The restore object format has changed. The `warnings` and `errors` fields are now ints containing the counts, while full warnings and errors are now stored in the object store instead of etcd. Restore objects created prior to v.0.6.0 should be deleted, or a new bucket used, and the old restore objects deleted from Kubernetes (`kubectl -n heptio-ark delete restore --all`).

All New Features:
  * Add `ark plugin add` and `ark plugin remove` commands #217, @skriss
  * Add plugin support for block/object stores, backup/restore item actions #174 #188 #206 #213 #215 #223 #226, @skriss @ncdc
  * Improve Azure deployment instructions #216, @ncdc
  * Change default TTL for backups to 30 days #204, @nrb
  * Improve logging for backups and restores #199, @ncdc
  * Add `ark backup describe`, `ark schedule describe` #196, @ncdc
  * Add `ark restore describe` and move restore warnings/errors to object storage #173 #201 #202, @ncdc
  * Upgrade to client-go v5.0.1, kubernetes v1.8.2 #157, @ncdc
  * Add Travis CI support #165 #166, @ncdc

Bug Fixes:
  * Fix log location hook prefix stripping #222, @ncdc
  * When running `ark backup download`, remove file if there's an error #154, @ncdc
  * Update documentation for AWS KMS Key alias support #163, @lli-hiya
  * Remove clock from `volume_snapshot_action` #137, @athampy

#### [v0.5.1](https://github.com/heptio/ark/tree/v0.5.1) - 2017-11-06
Bug fixes:
  * If a Service is headless, retain ClusterIP = None when backing up and restoring.
  * Use the specifed --label-selector when listing backups, schedules, and restores.
  * Restore namespace mapping functionality that was accidentally broken in 0.5.0.
  * Always include namespaces in the backup, regardless of the --include-cluster-resources setting.

#### [v0.5.0](https://github.com/heptio/ark/tree/v0.5.0) - 2017-10-26
Breaking changes:
  * The backup tar file format has changed. Backups created using previous versions of Ark cannot be restored using v0.5.0.
  * When backing up one or more specific namespaces, cluster-scoped resources are no longer backed up by default, with the exception of PVs that are used within the target namespace(s). Cluster-scoped resources can still be included by explicitly specifying `--include-cluster-resources`.

New features:
  * Add customized user-agent string for Ark CLI
  * Switch from glog to logrus
  * Exclude nodes from restoration
  * Add a FAQ
  * Record PV availability zone and use it when restoring volumes from snapshots
  * Back up the PV associated with a PVC
  * Add `--include-cluster-resources` flag to `ark backup create`
  * Add `--include-cluster-resources` flag to `ark restore create`
  * Properly support resource restore priorities across cluster-scoped and namespace-scoped resources
  * Support `ark create ...` and `ark get ...`
  * Make ark run as cluster-admin
  * Add pod exec backup hooks
  * Support cross-compilation & upgrade to go 1.9
  
Bug fixes:
  * Make config change detection more robust

#### [v0.4.0](https://github.com/heptio/ark/tree/v0.4.0) - 2017-09-14
Breaking changes:
  * Snapshotting and restoring volumes is now enabled by default
  * The --namespaces flag for 'ark restore create' has been replaced by --include-namespaces and
    --exclude-namespaces

New features:
  * Support for S3 SSE with KMS
  * Cloud provider configurations are validated at startup
  * The persistentVolumeProvider is now optional
  * Restore objects are garbage collected
  * Each backup now has an associated log file, viewable via 'ark backup logs'
  * Each restore now has an associated log file, viewable via 'ark restore logs'
  * Add --include-resources/--exclude-resources for restores

Bug fixes:
  * Only save/use iops for io1 volumes on AWS
  * When restoring, try to retrieve the Backup directly from object storage if it's not found
  * When syncing Backups from object storage to Kubernetes, don't return at the first error
    encountered
  * More closely match how kubectl performs kubeconfig resolution
  * Increase default Azure API request timeout to 2 minutes
  * Update Azure diskURI to match diskName

#### [v0.3.3](https://github.com/heptio/ark/tree/v0.3.3) - 2017-08-10
  * Treat the first field in a schedule's cron expression as minutes, not seconds

#### [v0.3.2](https://github.com/heptio/ark/tree/v0.3.2) - 2017-08-07
  * Add client-go auth provider plugins for Azure, GCP, OIDC

#### [v0.3.1](https://github.com/heptio/ark/tree/v0.3.1) - 2017-08-03
  * Fix Makefile VERSION

#### [v0.3.0](https://github.com/heptio/ark/tree/v0.3.0) - 2017-08-03
  * Initial Release
