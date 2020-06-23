## ark delete

Delete ark resources

### Synopsis


Delete ark resources

### Options

```
  -h, --help   help for delete
```

### Options inherited from parent commands

```
      --alsologtostderr                  log to standard error as well as files
      --kubeconfig string                Path to the kubeconfig file to use to talk to the Kubernetes apiserver. If unset, try the environment variable KUBECONFIG, as well as in-cluster configuration
      --log_backtrace_at traceLocation   when logging hits line file:N, emit a stack trace (default :0)
      --log_dir string                   If non-empty, write log files in this directory
      --logtostderr                      log to standard error instead of files
  -n, --namespace string                 The namespace in which Ark should operate (default "heptio-ark")
      --stderrthreshold severity         logs at or above this threshold go to stderr (default 2)
  -v, --v Level                          log level for V logs
      --vmodule moduleSpec               comma-separated list of pattern=N settings for file-filtered logging
```

### SEE ALSO
* [ark](ark.md)	 - Back up and restore Kubernetes cluster resources.
* [ark delete backup](ark_delete_backup.md)	 - Delete a backup
* [ark delete restore](ark_delete_restore.md)	 - Delete a restore
* [ark delete schedule](ark_delete_schedule.md)	 - Delete a schedule

