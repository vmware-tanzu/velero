---
layout: docs
title: velero create backup-location
---
Create a backup storage location

```
velero create backup-location NAME [flags]
```

### Options

```
      --access-mode                  Access mode for the backup storage location. Valid values are ReadWrite,ReadOnly (default ReadWrite)
      --backup-sync-period 0s        How often to ensure all Velero backups in object storage exist as Backup API objects in the cluster. Optional. Set this to 0s to disable sync. Default: 1 minute.
      --bucket string                Name of the object storage bucket where backups should be stored.
      --cacert string                File containing a certificate bundle to use when verifying TLS connections to the object store. Optional.
      --config mapStringString       Configuration key-value pairs.
      --credential mapStringString   The credential to be used by this location as a key-value pair, where the key is the Kubernetes Secret name, and the value is the data key name within the Secret. Optional, one value only.
      --default                      Sets this new location to be the new default backup storage location. Optional.
  -h, --help                         help for backup-location
  -L, --label-columns stringArray    Accepts a comma separated list of labels that are going to be presented as columns. Names are case-sensitive. You can also use multiple flag options like -L label1 -L label2...
      --labels mapStringString       Labels to apply to the backup storage location.
  -o, --output string                Output display format. For create commands, display the object but do not send it to the server. Valid formats are 'table', 'json', and 'yaml'. 'table' is not valid for the install command.
      --prefix string                Prefix under which all Velero data should be stored within the bucket. Optional.
      --provider string              Name of the backup storage provider (e.g. aws, azure, gcp).
      --show-labels                  Show labels in the last column
      --validation-frequency 0s      How often to verify if the backup storage location is valid. Optional. Set this to 0s to disable sync. Default 1 minute.
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

