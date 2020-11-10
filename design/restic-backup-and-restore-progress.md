# Progress reporting for restic backups and restores

Status: Accepted

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

The `restic stats` command returns the total size of a backup.
This can be compared with the total size the volume periodically to calculate the completion percentage of the restore.
The PodVolumeRestore CR's status will be updated with this information.

## Detailed Design

## Changes to PodVolumeBackup and PodVolumeRestore Status type

A new `Progress` field will be added to PodVolumeBackupStatus and PodVolumeRestoreStatus of type `PodVolumeOperationProgress`:

```
type PodVolumeOperationProgress struct {
  TotalBytes int64
  BytesDone int64
}
```

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
The Goroutine will periodically read the stream (every 10 seconds) and get the last printed status line, which will be converted to JSON.
If `bytes_done` is empty, restic has not finished scanning the volume and hasn't calculated the `total_bytes`.
In this case, we will not update the PodVolumeBackup and instead will wait for the next iteration.
Once we get a non-zero value for `bytes_done`, the `bytes_done` and `total_bytes` properties will be read and the PodVolumeBackup will be patched to update `status.Progress.BytesDone` and `status.Progress.TotalBytes` respectively.

Once the backup has completed successfully, the PodVolumeBackup will be patched to set `status.Progress.BytesDone = status.Progress.TotalBytes`.
This is done since the main thread may cause early termination of the Goroutine once the operation has finished, preventing a final update to the `BytesDone` property.

### restic restore progress

The `restic stats <snapshot_id> --json` command provides information about the size of backups:

```
{"total_size":10558111744,"total_file_count":11}
```

Before beginning the restore operation, we can use the output of `restic stats` to get the total size of the backup.
The PodVolumeRestore will be patched to set `status.Progress.TotalBytes` to the total size of the backup.

The code to run the `restic restore` command will be changed to include a Goroutine that periodically (every 10 seconds) gets the current size of the volume.
To get the current size of the volume, we will recursively walkthrough all files in the volume to accumulate the total size.
The current total size is the number of bytes transferred so far and the PodVolumeRestore will be patched to update `status.Progress.BytesDone`.

Once the restore has completed successfully, the PodVolumeRestore will be patched to set `status.Progress.BytesDone = status.Progress.TotalBytes`.
This is done since the main thread may cause early termination of the Goroutine once the operation has finished, preventing a final update to the `BytesDone` property.

### Velero CLI changes

The output that describes detailed information about [PodVolumeBackups](https://github.com/heptio/velero/blob/559d62a2ec99f7a522924348fc4a173a0699813a/pkg/cmd/util/output/backup_describer.go#L349) and [PodVolumeRestores](https://github.com/heptio/velero/blob/559d62a2ec99f7a522924348fc4a173a0699813a/pkg/cmd/util/output/restore_describer.go#L160) will be updated to calculate and display a completion percentage from `status.Progress.TotalBytes` and `status.Progress.BytesDone` if available.

## Open Questions

- Can we assume that the volume we are restoring in will be empty? Can it contain other artefacts?
  - Based on discussion in this PR, we are okay making the assumption that the PVC is empty and will proceed with the above proposed approach.

## Alternatives Considered

### restic restore progress

If we cannot assume that the volume we are restoring into will be empty, we can instead use the output from `restic snapshot` to get the list of files in the backup.
This can then be used to calculate the current total size of just those files in the volume, so that we avoid considering any other files unrelated to the backup.
The above proposed approach is simpler than this one, as we don't need to keep track of each file in the backup, but this will be more robust if the volume could contain other files not included in the backup.
It's possible that certain volume types may contain hidden files that could attribute to the total size of the volume, though these should be small enough that the BytesDone calculation will only be slightly inflated.

Another option is to contribute progress reporting similar to `restic backup` for `restic restore` upstream.
This may take more time, but would give us a more native view on the progress of a restore.
There are several issues about this already in the restic repo (https://github.com/restic/restic/issues/426, https://github.com/restic/restic/issues/1154), and what looks like an abandoned attempt (https://github.com/restic/restic/pull/2003) which we may be able to pick up.

## Security Considerations

N/A
