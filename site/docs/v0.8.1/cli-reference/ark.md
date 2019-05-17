## ark

Back up and restore Kubernetes cluster resources.

### Synopsis


Heptio Ark is a tool for managing disaster recovery, specifically for Kubernetes
cluster resources. It provides a simple, configurable, and operationally robust
way to back up your application state and associated data.

If you're familiar with kubectl, Ark supports a similar model, allowing you to
execute commands such as 'ark get backup' and 'ark create schedule'. The same
operations can also be performed as 'ark backup get' and 'ark schedule create'.

### Options

```
      --alsologtostderr                  log to standard error as well as files
  -h, --help                             help for ark
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
* [ark client](ark_client.md)	 - Ark client related commands
* [ark completion](ark_completion.md)	 - Output shell completion code for the specified shell (bash or zsh)
* [ark create](ark_create.md)	 - Create ark resources
* [ark delete](ark_delete.md)	 - Delete ark resources
* [ark describe](ark_describe.md)	 - Describe ark resources
* [ark get](ark_get.md)	 - Get ark resources
* [ark plugin](ark_plugin.md)	 - Work with plugins
* [ark restore](ark_restore.md)	 - Work with restores
* [ark schedule](ark_schedule.md)	 - Work with schedules
* [ark server](ark_server.md)	 - Run the ark server
* [ark version](ark_version.md)	 - Print the ark version and associated image

