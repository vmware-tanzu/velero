---
layout: docs
title: velero backup delete
---
Delete backups

```
velero backup delete [NAMES] [flags]
```

### Examples

```
  # delete a backup named "backup-1"
  velero backup delete backup-1

  # delete a backup named "backup-1" without prompting for confirmation
  velero backup delete backup-1 --confirm

  # delete backups named "backup-1" and "backup-2"
  velero backup delete backup-1 backup-2

  # delete all backups triggered by schedule "schedule-1"
  velero backup delete --selector velero.io/schedule-name=schedule-1
 
  # delete all backups
  velero backup delete --all
  
```

### Options

```
      --all                      Delete all backups
      --confirm                  Confirm deletion
  -h, --help                     help for delete
  -l, --selector labelSelector   Delete all backups matching this label selector (default <none>)
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

* [velero backup](velero_backup.md)	 - Work with backups

