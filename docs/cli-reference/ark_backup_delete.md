## ark backup delete

Delete backups

### Synopsis


Delete backups

```
ark backup delete [NAMES] [flags]
```

### Examples

```
  # delete a backup named "backup-1"
  ark backup delete backup-1

  # delete a backup named "backup-1" without prompting for confirmation
  ark backup delete backup-1 --confirm

  # delete backups named "backup-1" and "backup-2"
  ark backup delete backup-1 backup-2

  # delete all backups triggered by schedule "schedule-1"
  ark backup delete --selector ark-schedule=schedule-1
 
  # delete all backups
  ark backup delete --all
  
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
      --alsologtostderr                  log to standard error as well as files
      --kubeconfig string                Path to the kubeconfig file to use to talk to the Kubernetes apiserver. If unset, try the environment variable KUBECONFIG, as well as in-cluster configuration
      --kubecontext string               The context to use to talk to the Kubernetes apiserver. If unset defaults to whatever your current-context is (kubectl config current-context)
      --log_backtrace_at traceLocation   when logging hits line file:N, emit a stack trace (default :0)
      --log_dir string                   If non-empty, write log files in this directory
      --logtostderr                      log to standard error instead of files
  -n, --namespace string                 The namespace in which Ark should operate (default "heptio-ark")
      --stderrthreshold severity         logs at or above this threshold go to stderr (default 2)
  -v, --v Level                          log level for V logs
      --vmodule moduleSpec               comma-separated list of pattern=N settings for file-filtered logging
```

### SEE ALSO
* [ark backup](ark_backup.md)	 - Work with backups

