## ark server

Run the ark server

### Synopsis


Run the ark server

```
ark server [flags]
```

### Options

```
      --backup-sync-period duration               how often to ensure all Ark backups in object storage exist as Backup API objects in the cluster (default 1h0m0s)
  -h, --help                                      help for server
      --log-level                                 the level at which to log. Valid values are debug, info, warning, error, fatal, panic. (default info)
      --metrics-address string                    the address to expose prometheus metrics (default ":8085")
      --plugin-dir string                         directory containing Ark plugins (default "/plugins")
      --restic-timeout duration                   how long backups/restores of pod volumes should be allowed to run before timing out (default 1h0m0s)
      --restore-only                              run in a mode where only restores are allowed; backups, schedules, and garbage-collection are all disabled
      --restore-resource-priorities stringSlice   desired order of resource restores; any resource not in the list will be restored alphabetically after the prioritized resources (default [namespaces,persistentvolumes,persistentvolumeclaims,secrets,configmaps,serviceaccounts,limitranges,pods])
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
* [ark](ark.md)	 - Back up and restore Kubernetes cluster resources.
