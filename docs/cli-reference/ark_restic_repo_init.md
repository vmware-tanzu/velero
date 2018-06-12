## ark restic repo init

initialize a restic repository for a specified namespace

### Synopsis


initialize a restic repository for a specified namespace

```
ark restic repo init NAMESPACE [flags]
```

### Options

```
  -h, --help              help for init
      --key-data string   Encryption key for the restic repository. Optional; if unset, Ark will generate a random key for you.
      --key-file string   Path to file containing the encryption key for the restic repository. Optional; if unset, Ark will generate a random key for you.
      --key-size int      Size of the generated key for the restic repository (default 1024)
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
* [ark restic repo](ark_restic_repo.md)	 - Work with restic repositories

