## ark backup download

Download a backup

### Synopsis


Download a backup

```
ark backup download NAME [flags]
```

### Options

```
      --force              forces the download and will overwrite file if it exists already
  -h, --help               help for download
  -o, --output string      path to output file. Defaults to <NAME>-data.tar.gz in the current directory
      --timeout duration   maximum time to wait to process download request (default 1m0s)
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
* [ark backup](ark_backup.md)	 - Work with backups

