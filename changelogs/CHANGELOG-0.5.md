- [v0.5.1](#v051)
- [v0.5.0](#v050)

## v0.5.1
#### 2017-11-06
### Download
  - https://github.com/heptio/ark/tree/v0.5.1

### Bug fixes
  * If a Service is headless, retain ClusterIP = None when backing up and restoring.
  * Use the specified --label-selector when listing backups, schedules, and restores.
  * Restore namespace mapping functionality that was accidentally broken in 0.5.0.
  * Always include namespaces in the backup, regardless of the --include-cluster-resources setting.


## v0.5.0
#### 2017-10-26
### Download
  - https://github.com/heptio/ark/tree/v0.5.0

### Breaking changes
  * The backup tar file format has changed. Backups created using previous versions of Ark cannot be restored using v0.5.0.
  * When backing up one or more specific namespaces, cluster-scoped resources are no longer backed up by default, with the exception of PVs that are used within the target namespace(s). Cluster-scoped resources can still be included by explicitly specifying `--include-cluster-resources`.

### New features
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
  
### Bug fixes
  * Make config change detection more robust
