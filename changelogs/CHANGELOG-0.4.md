- [v0.4.0](#v040)

## v0.4.0
#### 2017-09-14
### Download
  - https://github.com/heptio/ark/tree/v0.4.0

### Breaking changes
  * Snapshotting and restoring volumes is now enabled by default
  * The --namespaces flag for 'ark restore create' has been replaced by --include-namespaces and
    --exclude-namespaces

### New features
  * Support for S3 SSE with KMS
  * Cloud provider configurations are validated at startup
  * The persistentVolumeProvider is now optional
  * Restore objects are garbage collected
  * Each backup now has an associated log file, viewable via 'ark backup logs'
  * Each restore now has an associated log file, viewable via 'ark restore logs'
  * Add --include-resources/--exclude-resources for restores

### Bug fixes
  * Only save/use iops for io1 volumes on AWS
  * When restoring, try to retrieve the Backup directly from object storage if it's not found
  * When syncing Backups from object storage to Kubernetes, don't return at the first error
    encountered
  * More closely match how kubectl performs kubeconfig resolution
  * Increase default Azure API request timeout to 2 minutes
  * Update Azure diskURI to match diskName
