---
title: "ark schedule create"
layout: docs
---

Create a schedule

### Synopsis


Create a schedule

```
ark schedule create NAME [flags]
```

### Options

```
      --exclude-namespaces stringArray                  namespaces to exclude from the backup
      --exclude-resources stringArray                   resources to exclude from the backup, formatted as resource.group, such as storageclasses.storage.k8s.io
  -h, --help                                            help for create
      --include-cluster-resources optionalBool[=true]   include cluster-scoped resources in the backup
      --include-namespaces stringArray                  namespaces to include in the backup (use '*' for all namespaces) (default *)
      --include-resources stringArray                   resources to include in the backup, formatted as resource.group, such as storageclasses.storage.k8s.io (use '*' for all resources)
      --label-columns stringArray                       a comma-separated list of labels to be displayed as columns
      --labels mapStringString                          labels to apply to the backup
  -o, --output string                                   Output display format. For create commands, display the object but do not send it to the server. Valid formats are 'table', 'json', and 'yaml'.
      --schedule string                                 a cron expression specifying a recurring schedule for this backup to run
  -l, --selector labelSelector                          only back up resources matching this label selector (default <none>)
      --show-labels                                     show labels in the last column
      --snapshot-volumes optionalBool[=true]            take snapshots of PersistentVolumes as part of the backup
      --ttl duration                                    how long before the backup can be garbage collected (default 720h0m0s)
```

### Options inherited from parent commands

```
      --alsologtostderr                  log to standard error as well as files
      --kubeconfig string                Path to the kubeconfig file to use to talk to the Kubernetes apiserver. If unset, try the environment variable KUBECONFIG, as well as in-cluster configuration
      --log_backtrace_at traceLocation   when logging hits line file:N, emit a stack trace (default :0)
      --log_dir string                   If non-empty, write log files in this directory
      --logtostderr                      log to standard error instead of files
      --stderrthreshold severity         logs at or above this threshold go to stderr (default 2)
  -v, --v Level                          log level for V logs
      --vmodule moduleSpec               comma-separated list of pattern=N settings for file-filtered logging
```

### SEE ALSO
* [ark schedule](ark_schedule.md)	 - Work with schedules

