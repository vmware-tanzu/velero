---
title: "ark"
layout: docs
---

Back up and restore Kubernetes cluster resources.

### Synopsis


Heptio Ark is a tool for managing disaster recovery, specifically for
Kubernetes cluster resources. It provides a simple, configurable,
and operationally robust way to back up your application state and
associated data.

### Options

```
      --alsologtostderr                  log to standard error as well as files
  -h, --help                             help for ark
      --kubeconfig string                Path to the kubeconfig file to use to talk to the Kubernetes apiserver. If unset, try the environment variable KUBECONFIG, as well as in-cluster configuration
      --log_backtrace_at traceLocation   when logging hits line file:N, emit a stack trace (default :0)
      --log_dir string                   If non-empty, write log files in this directory
      --logtostderr                      log to standard error instead of files
      --stderrthreshold severity         logs at or above this threshold go to stderr (default 2)
  -v, --v Level                          log level for V logs
      --vmodule moduleSpec               comma-separated list of pattern=N settings for file-filtered logging
```

### SEE ALSO
* [ark backup](ark_backup.md)	 - Work with backups
* [ark restore](ark_restore.md)	 - Work with restores
* [ark schedule](ark_schedule.md)	 - Work with schedules
* [ark server](ark_server.md)	 - Run the ark server
* [ark version](ark_version.md)	 - Print the ark version and associated image

