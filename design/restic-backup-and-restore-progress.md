# Progress reporting for restic backups and restores

Status: Draft

During long-running restic backups/restores, there is no visibility into what (if anything) is happening, making it hard to know if the backup/restore is making progress or hung, how long the operation might take, etc.
We should capture progress during restic operations and make it user-visible so that it's easier to reason about.
This document proposes an approach for capturing progress of backup and restore operations and exposing this information to users.

## Goals

- Provide basic visibility into restic operations to inform users about their progress.

## Non Goals

- Capturing progress for non-restic backups and restores.

## Background

(Omitted, see introduction)

## High-Level Design

### restic backup progress

The `restic backup` command provides progress reporting to stdout in JSON format, which includes the completion percentage of the backup.
This progress will be read on some interval and the PodVolumeBackup Custom Resource's (CR) status will be updated with this information.

### restic restore progress

The `restic ls` command returns the number of files in a backup, including the size of each file.
This can be compared with the total size of these file in the volume periodically to calculate the completion percentage of the restore.
The PodVolumeRestore CR's status will be updated with this information.

## Detailed Design

### restic backup progress

restic added support for [streaming JSON output for the `restic backup` command](https://github.com/restic/restic/pull/1944) in 0.9.5.
Our current images ship restic 0.9.4, and so the Dockerfile will be updated to pull the new version: https://github.com/heptio/velero/blob/af4b9373fc73047f843cd4bc3648603d780c8b74/Dockerfile-velero#L21.
With the `--json` flag, `restic backup` outputs single lines of JSON reporting the status of the backup:

```
{"message_type":"status","percent_done":0,"total_files":1,"total_bytes":21424504832}
{"message_type":"status","action":"scan_finished","item":"","duration":0.219241873,"data_size":49461329920,"metadata_size":0,"total_files":10}
{"message_type":"status","percent_done":0,"total_files":10,"total_bytes":49461329920,"current_files":["/file3"]}
{"message_type":"status","percent_done":0.0003815984736061056,"total_files":10,"total_bytes":49461329920,"bytes_done":18874368,"current_files":["/file1","/file3"]}
{"message_type":"status","percent_done":0.0011765952936188255,"total_files":10,"total_bytes":49461329920,"bytes_done":58195968,"current_files":["/file1","/file3"]}
{"message_type":"status","percent_done":0.0019503921984312064,"total_files":10,"total_bytes":49461329920,"bytes_done":96468992,"current_files":["/file1","/file3"]}
{"message_type":"status","percent_done":0.0028089887640449437,"total_files":10,"total_bytes":49461329920,"bytes_done":138936320,"current_files":["/file1","/file3"]}
```

The [command factory for backup](https://github.com/heptio/velero/blob/af4b9373fc73047f843cd4bc3648603d780c8b74/pkg/restic/command_factory.go#L37) will be updated to include the `--json` flag.
The code to run the `restic backup` command (https://github.com/heptio/velero/blob/af4b9373fc73047f843cd4bc3648603d780c8b74/pkg/controller/pod_volume_backup_controller.go#L241) will be changed to include a Goroutine that reads from the command's stdout stream.
The implementation of this will largely follow [@jmontleon's PoC](https://github.com/fusor/velero/pull/4/files) of this.
The Goroutine will periodically read the stream (every 10 seconds) and get the last printed status line.
The line will be decoded as JSON and the `percent_done` property will be read and formatted as a percentage value.

The PodVolumeBackupStatus type will be extended to include a `Progress` field.
The PodVolumeBackup will be patched to set the `status.Progress` field to the percentage value of the progress.

Once the backup has completed successfully, the PodVolumeBackup will be patched to set `status.Progress = 100%`.

### restic restore progress

The `restic ls <snapshot_id> --json` command provides information about the size of backed up files:

```
{"time":"2019-08-14T15:16:29.105557-07:00","tree":"1de5381473aecb5548959f6e5444bb7966fe781500b5f8ec1629088674bea6a6","paths":["/Users/aadnan/Playground/restic/files"],"hostname":"aadnan-a01.vmware.com","username":"aadnan","uid":501,"gid":20,"id":"5628926b4f0e105eb33d83c619f235a8cf8f695d19955fab44bf506ece195f3a","short_id":"5628926b","struct_type":"snapshot"}
{"name":"files","type":"dir","path":"/files","uid":501,"gid":20,"mode":2147484096,"mtime":"2019-08-14T15:16:20.53091354-07:00","atime":"2019-08-14T15:16:20.53091354-07:00","ctime":"2019-08-14T15:16:20.53091354-07:00","struct_type":"node"}
{"name":"file1","type":"file","path":"/files/file1","uid":501,"gid":20,"size":1435500544,"mode":384,"mtime":"2019-08-14T15:16:20.491206506-07:00","atime":"2019-08-14T15:16:20.491206506-07:00","ctime":"2019-08-14T15:16:20.491234451-07:00","struct_type":"node"}
...
```

Before beginning the restore operation, we can use the output of `restic ls` to get a list of the files that will be restored and calculate the total size of the backup.

The code to run the `restic restore` command will be changed to include a Goroutine that periodically (every 10 seconds) goes through the list of files in the backup and gets the current size of each file in the volume.
If the file doesn't exist in the volume yet (restic hasn't started restoring it), the size is 0.
The sum of the current size of each file is then compared with the total size of the backup to calculate a completion percentage (`cursize / totalsize * 100`).

The PodVolumeRestoreStatus type will be extended to include a `Progress` field.
The PodVolumeRestore will be patched to set the `status.Progress` field to the percentage value of the progress.

Once the restore has completed successfully, the PodVolumeRestore will be patched to set `status.Progress = 100%`.

## Open Questions

- Can we assume that the volume we are restoring in will be empty? Can it contain other artefacts? If we can assume it to be empty, we can go with the easier approach detailed below.

## Alternatives Considered

### restic restore progress

If we can assume that the volume we are restoring into will be empty, we can instead use the output from `restic stats` to get the total size of the backup.
This can then be periodically compared with the size of all files in the volume to calculate a completion percentage.
This will be simpler as we don't need to keep track of each file in the backup, but will not work if the volume could contain other files not included in the backup.
It's possible that certain volume types may contain hidden files that could attribute to the total size of the volume, and so this might be an overall less safe approach.

Another option is to contribute progress reporting similar to `restic backup` for `restic restore` upstream.
This may take more time, but would give us a more native view on the progress of a restore.
There are several issues about this already in the restic repo (https://github.com/restic/restic/issues/426, https://github.com/restic/restic/issues/1154), and what looks like an abandoned attempt (https://github.com/restic/restic/pull/2003) which we may be able to pick up.

## Security Considerations

N/A
