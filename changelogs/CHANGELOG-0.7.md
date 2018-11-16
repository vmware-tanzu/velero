- [v0.7.1](#v071)
- [v0.7.0](#v070)

## v0.7.1
#### 2018-02-22
### Download
  - https://github.com/heptio/ark/releases/tag/v0.7.1

### Bug Fixes:
  * Run the Ark server in its own namespace, separate from backups/schedules/restores/config (#322, @ncdc)


## v0.7.0
#### 2018-02-15
### Download
  - https://github.com/heptio/ark/releases/tag/v0.7.0

### New Features:
  * Run the Ark server in any namespace (#272, @ncdc)
  * Add ability to delete backups and their associated data (#252, @skriss)
  * Support both pre and post backup hooks (#243, @ncdc)

### Bug Fixes / Other Changes:
  * Switch from Update() to Patch() when updating Ark resources (#241, @skriss)
  * Don't fail the backup if a PVC is not bound to a PV (#256, @skriss)
  * Restore serviceaccounts prior to workload controllers (#258, @ncdc)
  * Stop removing annotations from PVs when restoring them (#263, @skriss)
  * Update GCP client libraries (#249, @skriss)
  * Clarify backup and restore creation messages (#270, @nrb)
  * Update S3 bucket creation docs for us-east-1 (#285, @lypht)
