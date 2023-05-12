---
layout: docs
title: velero restore create
---
Create a restore

```
velero restore create [RESTORE_NAME] [--from-backup BACKUP_NAME | --from-schedule SCHEDULE_NAME] [flags]
```

### Examples

```
  # create a restore named "restore-1" from backup "backup-1"
  velero restore create restore-1 --from-backup backup-1

  # create a restore with a default name ("backup-1-<timestamp>") from backup "backup-1"
  velero restore create --from-backup backup-1
 
  # create a restore from the latest successful backup triggered by schedule "schedule-1"
  velero restore create --from-schedule schedule-1

  # create a restore from the latest successful OR partially-failed backup triggered by schedule "schedule-1"
  velero restore create --from-schedule schedule-1 --allow-partially-failed

  # create a restore for only persistentvolumeclaims and persistentvolumes within a backup
  velero restore create --from-backup backup-2 --include-resources persistentvolumeclaims,persistentvolumes
  
```

### Options

```
      --allow-partially-failed optionalBool[=true]      if using --from-schedule, whether to consider PartiallyFailed backups when looking for the most recent one. This flag has no effect if not using --from-schedule.
      --exclude-namespaces stringArray                  namespaces to exclude from the restore
      --exclude-resources stringArray                   resources to exclude from the restore, formatted as resource.group, such as storageclasses.storage.k8s.io
      --from-backup string                              backup to restore from
      --from-schedule string                            schedule to restore from
  -h, --help                                            help for create
      --include-cluster-resources optionalBool[=true]   include cluster-scoped resources in the restore
      --include-namespaces stringArray                  namespaces to include in the restore (use '*' for all namespaces) (default *)
      --include-resources stringArray                   resources to include in the restore, formatted as resource.group, such as storageclasses.storage.k8s.io (use '*' for all resources)
      --label-columns stringArray                       a comma-separated list of labels to be displayed as columns
      --labels mapStringString                          labels to apply to the restore
      --namespace-mappings mapStringString              namespace mappings from name in the backup to desired restored name in the form src1:dst1,src2:dst2,...
  -o, --output string                                   Output display format. For create commands, display the object but do not send it to the server. Valid formats are 'table', 'json', and 'yaml'. 'table' is not valid for the install command.
      --restore-volumes optionalBool[=true]             whether to restore volumes from snapshots
  -l, --selector labelSelector                          only restore resources matching this label selector (default <none>)
      --show-labels                                     show labels in the last column
  -w, --wait                                            wait for the operation to complete
```

### Options inherited from parent commands

```
      --add_dir_header                   If true, adds the file directory to the header
      --alsologtostderr                  log to standard error as well as files
      --features stringArray             Comma-separated list of features to enable for this Velero process. Combines with values from $HOME/.config/velero/config.json if present
      --kubeconfig string                Path to the kubeconfig file to use to talk to the Kubernetes apiserver. If unset, try the environment variable KUBECONFIG, as well as in-cluster configuration
      --kubecontext string               The context to use to talk to the Kubernetes apiserver. If unset defaults to whatever your current-context is (kubectl config current-context)
      --log_backtrace_at traceLocation   when logging hits line file:N, emit a stack trace (default :0)
      --log_dir string                   If non-empty, write log files in this directory
      --log_file string                  If non-empty, use this log file
      --log_file_max_size uint           Defines the maximum size a log file can grow to. Unit is megabytes. If the value is 0, the maximum file size is unlimited. (default 1800)
      --logtostderr                      log to standard error instead of files (default true)
  -n, --namespace string                 The namespace in which Velero should operate (default "velero")
      --skip_headers                     If true, avoid header prefixes in the log messages
      --skip_log_headers                 If true, avoid headers when opening log files
      --stderrthreshold severity         logs at or above this threshold go to stderr (default 2)
  -v, --v Level                          number for the log level verbosity
      --vmodule moduleSpec               comma-separated list of pattern=N settings for file-filtered logging
```

### SEE ALSO

* [velero restore](velero_restore.md)	 - Work with restores

