---
layout: docs
title: velero create schedule
---
Create a schedule

### Synopsis

The --schedule flag is required, in cron notation, using UTC time:

| Character Position | Character Period | Acceptable Values |
| -------------------|:----------------:| -----------------:|
| 1                  | Minute           | 0-59,*            |
| 2                  | Hour             | 0-23,*            |
| 3                  | Day of Month     | 1-31,*            |
| 4                  | Month            | 1-12,*            |
| 5                  | Day of Week      | 0-6,*             |

The schedule can also be expressed using "@every <duration>" syntax. The duration
can be specified using a combination of seconds (s), minutes (m), and hours (h), for
example: "@every 2h30m".

```
velero create schedule NAME --schedule [flags]
```

### Examples

```
  # Create a backup every 6 hours.
  velero create schedule NAME --schedule="0 */6 * * *"

  # Create a backup every 6 hours with the @every notation.
  velero create schedule NAME --schedule="@every 6h"

  # Create a daily backup of the web namespace.
  velero create schedule NAME --schedule="@every 24h" --include-namespaces web

  # Create a weekly backup, each living for 90 days (2160 hours).
  velero create schedule NAME --schedule="@every 168h" --ttl 2160h0m0s
```

### Options

```
      --csi-snapshot-timeout duration                      How long to wait for CSI snapshot creation before timeout.
      --default-volumes-to-fs-backup optionalBool[=true]   Use pod volume file system backup by default for volumes
      --exclude-cluster-scoped-resources stringArray       Cluster-scoped resources to exclude from the backup, formatted as resource.group, such as storageclasses.storage.k8s.io(use '*' for all resources). Cannot work with include-resources, exclude-resources and include-cluster-resources.
      --exclude-namespace-scoped-resources stringArray     Namespaced resources to exclude from the backup, formatted as resource.group, such as deployments.apps(use '*' for all resources). Cannot work with include-resources, exclude-resources and include-cluster-resources.
      --exclude-namespaces stringArray                     Namespaces to exclude from the backup.
      --exclude-resources stringArray                      Resources to exclude from the backup, formatted as resource.group, such as storageclasses.storage.k8s.io. Cannot work with include-cluster-scoped-resources, exclude-cluster-scoped-resources, include-namespace-scoped-resources and exclude-namespace-scoped-resources.
  -h, --help                                               help for schedule
      --include-cluster-resources optionalBool[=true]      Include cluster-scoped resources in the backup. Cannot work with include-cluster-scoped-resources, exclude-cluster-scoped-resources, include-namespace-scoped-resources and exclude-namespace-scoped-resources.
      --include-cluster-scoped-resources stringArray       Cluster-scoped resources to include in the backup, formatted as resource.group, such as storageclasses.storage.k8s.io(use '*' for all resources). Cannot work with include-resources, exclude-resources and include-cluster-resources.
      --include-namespace-scoped-resources stringArray     Namespaced resources to include in the backup, formatted as resource.group, such as deployments.apps(use '*' for all resources). Cannot work with include-resources, exclude-resources and include-cluster-resources.
      --include-namespaces stringArray                     Namespaces to include in the backup (use '*' for all namespaces). (default *)
      --include-resources stringArray                      Resources to include in the backup, formatted as resource.group, such as storageclasses.storage.k8s.io (use '*' for all resources). Cannot work with include-cluster-scoped-resources, exclude-cluster-scoped-resources, include-namespace-scoped-resources and exclude-namespace-scoped-resources.
      --item-operation-timeout duration                    How long to wait for async plugin operations before timeout.
  -L, --label-columns stringArray                          Accepts a comma separated list of labels that are going to be presented as columns. Names are case-sensitive. You can also use multiple flag options like -L label1 -L label2...
      --labels mapStringString                             Labels to apply to the backup.
      --ordered-resources string                           Mapping Kinds to an ordered list of specific resources of that Kind.  Resource names are separated by commas and their names are in format 'namespace/resourcename'. For cluster scope resource, simply use resource name. Key-value pairs in the mapping are separated by semi-colon.  Example: 'pods=ns1/pod1,ns1/pod2;persistentvolumeclaims=ns1/pvc4,ns1/pvc8'.  Optional.
  -o, --output string                                      Output display format. For create commands, display the object but do not send it to the server. Valid formats are 'table', 'json', and 'yaml'. 'table' is not valid for the install command.
      --paused                                             Specifies whether the newly created schedule is paused or not.
      --resource-policies-configmap string                 Reference to the resource policies configmap that backup using
      --schedule string                                    A cron expression specifying a recurring schedule for this backup to run
  -l, --selector labelSelector                             Only back up resources matching this label selector. (default <none>)
      --show-labels                                        Show labels in the last column
      --snapshot-volumes optionalBool[=true]               Take snapshots of PersistentVolumes as part of the backup. If the parameter is not set, it is treated as setting to 'true'.
      --storage-location string                            Location in which to store the backup.
      --ttl duration                                       How long before the backup can be garbage collected.
      --use-owner-references-in-backup                     Specifies whether to use OwnerReferences on backups created by this Schedule. Notice: if set to true, when schedule is deleted, backups will be deleted too.
      --volume-snapshot-locations strings                  List of locations (at most one per provider) where volume snapshots should be stored.
```

### Options inherited from parent commands

```
      --add_dir_header                   If true, adds the file directory to the header of the log messages
      --alsologtostderr                  log to standard error as well as files (no effect when -logtostderr=true)
      --colorized optionalBool           Show colored output in TTY. Overrides 'colorized' value from $HOME/.config/velero/config.json if present. Enabled by default
      --features stringArray             Comma-separated list of features to enable for this Velero process. Combines with values from $HOME/.config/velero/config.json if present
      --kubeconfig string                Path to the kubeconfig file to use to talk to the Kubernetes apiserver. If unset, try the environment variable KUBECONFIG, as well as in-cluster configuration
      --kubecontext string               The context to use to talk to the Kubernetes apiserver. If unset defaults to whatever your current-context is (kubectl config current-context)
      --log_backtrace_at traceLocation   when logging hits line file:N, emit a stack trace (default :0)
      --log_dir string                   If non-empty, write log files in this directory (no effect when -logtostderr=true)
      --log_file string                  If non-empty, use this log file (no effect when -logtostderr=true)
      --log_file_max_size uint           Defines the maximum size a log file can grow to (no effect when -logtostderr=true). Unit is megabytes. If the value is 0, the maximum file size is unlimited. (default 1800)
      --logtostderr                      log to standard error instead of files (default true)
  -n, --namespace string                 The namespace in which Velero should operate (default "velero")
      --one_output                       If true, only write logs to their native severity level (vs also writing to each lower severity level; no effect when -logtostderr=true)
      --skip_headers                     If true, avoid header prefixes in the log messages
      --skip_log_headers                 If true, avoid headers when opening log files (no effect when -logtostderr=true)
      --stderrthreshold severity         logs at or above this threshold go to stderr when writing to files and stderr (no effect when -logtostderr=true or -alsologtostderr=false) (default 2)
  -v, --v Level                          number for the log level verbosity
      --vmodule moduleSpec               comma-separated list of pattern=N settings for file-filtered logging
```

### SEE ALSO

* [velero create](velero_create.md)	 - Create velero resources

