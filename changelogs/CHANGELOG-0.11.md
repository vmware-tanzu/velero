## v0.11.1
#### 2019-05-17

### Download
- https://github.com/heptio/velero/releases/tag/v0.11.1

### Highlights
* Added the `velero migrate-backups` command to migrate legacy Ark backup metadata to the current Velero format in object storage. This command needs to be run in preparation for upgrading to v1.0, **if** you have backups that were originally created prior to v0.11 (i.e. when the project was named Ark).

## v0.11.0
#### 2019-02-28

### Download
- https://github.com/heptio/velero/releases/tag/v0.11.0

### Highlights
* Heptio Ark is now Velero! This release is the first one to use the new name. For details on the changes and how to migrate to v0.11, see the [migration instructions][1]. **Please follow the instructions to ensure a successful upgrade to v0.11.**
* Restic has been upgraded to v0.9.4, which brings significantly faster restores thanks to a new multi-threaded restorer.
* Velero now waits for terminating namespaces and persistent volumes to delete before attempting to restore them, rather than trying and failing to restore them while they're being deleted.

### All Changes
* Fix concurrency bug in code ensuring restic repository exists (#1235, @skriss)
* Wait for PVs and namespaces to delete before attempting to restore them. (#826, @nrb)
* Set the zones for GCP regional disks on restore. This requires the `compute.zones.get` permission on the GCP serviceaccount in order to work correctly. (#1200, @nrb)
* Renamed Heptio Ark to Velero. Changed internal imports, environment variables, and binary name. (#1184, @nrb)
* use 'restic stats' instead of 'restic check' to determine if repo exists (#1171, @skriss)
* upgrade restic to v0.9.4 & replace --hostname flag with --host (#1156, @skriss)
* Clarify restore log when object unchanged (#1153, @daved)
* Add backup-version file in backup tarball. (#1117, @wwitzel3)
* add ServerStatusRequest CRD and show server version in `ark version` output (#1116, @skriss)

[1]: https://heptio.github.io/velero/v0.11.0/migrating-to-velero
