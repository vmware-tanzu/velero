---
layout: docs
title: velero backup create
---
Create a backup

```
velero backup create NAME [flags]
```

### Examples

```
  # Create a backup containing all resources.
  velero backup create backup1

  # Create a backup including only the nginx namespace.
  velero backup create nginx-backup --include-namespaces nginx

  # Create a backup excluding the velero and default namespaces.
  velero backup create backup2 --exclude-namespaces velero,default

  # Create a backup based on a schedule named daily-backup.
  velero backup create --from-schedule daily-backup

  # View the YAML for a backup that doesn't snapshot volumes, without sending it to the server.
  velero backup create backup3 --snapshot-volumes=false -o yaml

  # Wait for a backup to complete before returning from the command.
  velero backup create backup4 --wait
```

### Options

```
      --default-volumes-to-restic optionalBool[=true]   Use restic by default to backup all pod volumes
      --exclude-namespaces stringArray                  Namespaces to exclude from the backup.
      --exclude-resources stringArray                   Resources to exclude from the backup, formatted as resource.group, such as storageclasses.storage.k8s.io.
      --from-schedule string                            Create a backup from the template of an existing schedule. Cannot be used with any other filters. Backup name is optional if used.
  -h, --help                                            help for create
      --include-cluster-resources optionalBool[=true]   Include cluster-scoped resources in the backup
      --include-namespaces stringArray                  Namespaces to include in the backup (use '*' for all namespaces). (default *)
      --include-resources stringArray                   Resources to include in the backup, formatted as resource.group, such as storageclasses.storage.k8s.io (use '*' for all resources).
      --label-columns stringArray                       A comma-separated list of labels to be displayed as columns
      --labels mapStringString                          Labels to apply to the backup.
      --ordered-resources string                        Mapping Kinds to an ordered list of specific resources of that Kind.  Resource names are separated by commas and their names are in format 'namespace/resourcename'. For cluster scope resource, simply use resource name. Key-value pairs in the mapping are separated by semi-colon.  Example: 'pods=ns1/pod1,ns1/pod2;persistentvolumeclaims=ns1/pvc4,ns1/pvc8'.  Optional.
  -o, --output string                                   Output display format. For create commands, display the object but do not send it to the server. Valid formats are 'table', 'json', and 'yaml'. 'table' is not valid for the install command.
  -l, --selector labelSelector                          Only back up resources matching this label selector. (default <none>)
      --show-labels                                     Show labels in the last column
      --snapshot-volumes optionalBool[=true]            Take snapshots of PersistentVolumes as part of the backup.
      --storage-location string                         Location in which to store the backup.
      --ttl duration                                    How long before the backup can be garbage collected. (default 720h0m0s)
      --volume-snapshot-locations strings               List of locations (at most one per provider) where volume snapshots should be stored.
  -w, --wait                                            Wait for the operation to complete.
```

### Options inherited from parent commands

```
      --add_dir_header                   If true, adds the file directory to the header
      --alsologtostderr                  log to standard error as well as files
      --colorized optionalBool           Show colored output in TTY. Overrides 'colorized' value from $HOME/.config/velero/config.json if present. Enabled by default
      --features stringArray             Comma-separated list of features to enable for this Velero process. Combines with values from $HOME/.config/velero/config.json if present
      --kubeconfig string                Path to the kubeconfig file to use to talk to the Kubernetes apiserver. If unset, try the environment variable KUBECONFIG, as well as in-cluster configuration
      --kubecontext string               The context to use to talk to the Kubernetes apiserver. If unset defaults to whatever your current-context is (kubectl config current-context)
      --log_backtrace_at traceLocation   when logging hits line file:N, emit a stack trace (default :0)
      --log_dir string                   If non-empty, write log files in this directory
      --log_file string                  If non-empty, use this log file
      --log_file_max_size uint           Defines the maximum size a log file can grow to. Unit is megabytes. If the value is 0, the maximum file size is unlimited. (default 1800)
      --logtostderr                      log to standard error instead of files (default true)
  -n, --namespace string                 The namespace in which Velero should operate (default "velero")
      --skip_headers                     If true, avoid header prefixes in the log messages
      --skip_log_headers                 If true, avoid headers when opening log files
      --stderrthreshold severity         logs at or above this threshold go to stderr (default 2)
  -v, --v Level                          number for the log level verbosity
      --vmodule moduleSpec               comma-separated list of pattern=N settings for file-filtered logging
```

### SEE ALSO

* [velero backup](velero_backup.md)	 - Work with backups

