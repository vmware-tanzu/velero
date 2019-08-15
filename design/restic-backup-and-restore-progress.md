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

The `restic stats` command returns the total size of a given snapshot.
This can be compared with the total size of files in the volume periodically to calculate the completion percentage of the restore.

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

<!-- TODO: complete -->

- before restore, determine size of snapshot using `restic stats --json`
- periodically calculate size of files in the volume (see https://github.com/missedone/dugo/blob/master/du.go#L12 for a way to calculate size)
- compare with total size to get progress: cursize / totalsize

## Open Questions

- Can we assume that the volume we are restoring in will be empty? Can it contain other artefacts? If so, we won't be able to use the proposed approach to compare backup and current volume sizes and will need to use the alternative approach described below.

## Alternatives Considered

### restic restore progress

To deal with the case where a volume we are restoring into may contain additional files not included in the snapshot, we can use the information from `restic ls` to retrieve the list of filepaths we should include when calculating the size of restored files in the volume.
It's possible that certain volume types may contain hidden files that could attribute to the total size of the volume, and so this might be an overall safer approach.

Another option is to contribute progress reporting similar to `restic backup` for `restic restore` upstream.
This may take more time, but would give us a more native view on the progress of a restore.
There are several issues about this already in the restic repo (https://github.com/restic/restic/issues/426, https://github.com/restic/restic/issues/1154), and what looks like an abandoned attempt (https://github.com/restic/restic/pull/2003) which we may be able to pick up.

## Security Considerations

N/A
