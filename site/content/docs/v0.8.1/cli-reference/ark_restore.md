---
title: "ark restore"
layout: docs
---

Work with restores

### Synopsis


Work with restores

### Options

```
  -h, --help   help for restore
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
* [ark restore create](ark_restore_create.md)	 - Create a restore
* [ark restore delete](ark_restore_delete.md)	 - Delete a restore
* [ark restore describe](ark_restore_describe.md)	 - Describe restores
* [ark restore get](ark_restore_get.md)	 - Get restores
* [ark restore logs](ark_restore_logs.md)	 - Get restore logs

