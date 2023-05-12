---
layout: docs
title: velero backup-location create
---
Create a backup storage location

```
velero backup-location create NAME [flags]
```

### Options

```
      --access-mode                 Access mode for the backup storage location. Valid values are ReadWrite,ReadOnly (default ReadWrite)
      --backup-sync-period 0s       How often to ensure all Velero backups in object storage exist as Backup API objects in the cluster. Optional. Set this to 0s to disable sync. Default: 1 minute.
      --bucket string               Name of the object storage bucket where backups should be stored.
      --cacert string               File containing a certificate bundle to use when verifying TLS connections to the object store. Optional.
      --config mapStringString      Configuration key-value pairs.
  -h, --help                        help for create
      --label-columns stringArray   a comma-separated list of labels to be displayed as columns
      --labels mapStringString      Labels to apply to the backup storage location.
  -o, --output string               Output display format. For create commands, display the object but do not send it to the server. Valid formats are 'table', 'json', and 'yaml'. 'table' is not valid for the install command.
      --prefix string               Prefix under which all Velero data should be stored within the bucket. Optional.
      --provider string             Name of the backup storage provider (e.g. aws, azure, gcp).
      --show-labels                 show labels in the last column
      --validation-frequency 0s     How often to verify if the backup storage location is valid. Optional. Set this to 0s to disable sync. Default 1 minute.
```

### Options inherited from parent commands

```
      --add_dir_header                   If true, adds the file directory to the header
      --alsologtostderr                  log to standard error as well as files
      --features stringArray             Comma-separated list of features to enable for this Velero process. Combines with values from $HOME/.config/velero/config.json if present
      --kubeconfig string                Path to the kubeconfig file to use to talk to the Kubernetes apiserver. If unset, try the environment variable KUBECONFIG, as well as in-cluster configuration
      --kubecontext string               The context to use to talk to the Kubernetes apiserver. If unset defaults to whatever your current-context is (kubectl config current-context)
      --log_backtrace_at traceLocation   when logging hits line file:N, emit a stack trace (default :0)
      --log_dir string                   If non-empty, write log files in this directory
      --log_file string                  If non-empty, use this log file
      --log_file_max_size uint           Defines the maximum size a log file can grow to. Unit is megabytes. If the value is 0, the maximum file size is unlimited. (default 1800)
      --logtostderr                      log to standard error instead of files (default true)
      --master --kubeconfig              (Deprecated: switch to --kubeconfig) The address of the Kubernetes API server. Overrides any value in kubeconfig. Only required if out-of-cluster.
  -n, --namespace string                 The namespace in which Velero should operate (default "velero")
      --skip_headers                     If true, avoid header prefixes in the log messages
      --skip_log_headers                 If true, avoid headers when opening log files
      --stderrthreshold severity         logs at or above this threshold go to stderr (default 2)
  -v, --v Level                          number for the log level verbosity
      --vmodule moduleSpec               comma-separated list of pattern=N settings for file-filtered logging
```

### SEE ALSO

* [velero backup-location](velero_backup-location.md)	 - Work with backup storage locations

