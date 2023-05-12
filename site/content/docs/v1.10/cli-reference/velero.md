---
layout: docs
title: velero
---
Back up and restore Kubernetes cluster resources.

### Synopsis

Velero is a tool for managing disaster recovery, specifically for Kubernetes
cluster resources. It provides a simple, configurable, and operationally robust
way to back up your application state and associated data.

If you're familiar with kubectl, Velero supports a similar model, allowing you to
execute commands such as 'velero get backup' and 'velero create schedule'. The same
operations can also be performed as 'velero backup get' and 'velero schedule create'.

### Options

```
      --add_dir_header                   If true, adds the file directory to the header of the log messages
      --alsologtostderr                  log to standard error as well as files
      --colorized optionalBool           Show colored output in TTY. Overrides 'colorized' value from $HOME/.config/velero/config.json if present. Enabled by default
      --features stringArray             Comma-separated list of features to enable for this Velero process. Combines with values from $HOME/.config/velero/config.json if present
  -h, --help                             help for velero
      --kubeconfig string                Path to the kubeconfig file to use to talk to the Kubernetes apiserver. If unset, try the environment variable KUBECONFIG, as well as in-cluster configuration
      --kubecontext string               The context to use to talk to the Kubernetes apiserver. If unset defaults to whatever your current-context is (kubectl config current-context)
      --log_backtrace_at traceLocation   when logging hits line file:N, emit a stack trace (default :0)
      --log_dir string                   If non-empty, write log files in this directory
      --log_file string                  If non-empty, use this log file
      --log_file_max_size uint           Defines the maximum size a log file can grow to. Unit is megabytes. If the value is 0, the maximum file size is unlimited. (default 1800)
      --logtostderr                      log to standard error instead of files (default true)
  -n, --namespace string                 The namespace in which Velero should operate (default "velero")
      --one_output                       If true, only write logs to their native severity level (vs also writing to each lower severity level)
      --skip_headers                     If true, avoid header prefixes in the log messages
      --skip_log_headers                 If true, avoid headers when opening log files
      --stderrthreshold severity         logs at or above this threshold go to stderr (default 2)
  -v, --v Level                          number for the log level verbosity
      --vmodule moduleSpec               comma-separated list of pattern=N settings for file-filtered logging
```

### SEE ALSO

* [velero backup](velero_backup.md)	 - Work with backups
* [velero backup-location](velero_backup-location.md)	 - Work with backup storage locations
* [velero bug](velero_bug.md)	 - Report a Velero bug
* [velero client](velero_client.md)	 - Velero client related commands
* [velero completion](velero_completion.md)	 - Generate completion script
* [velero create](velero_create.md)	 - Create velero resources
* [velero debug](velero_debug.md)	 - Generate debug bundle
* [velero delete](velero_delete.md)	 - Delete velero resources
* [velero describe](velero_describe.md)	 - Describe velero resources
* [velero get](velero_get.md)	 - Get velero resources
* [velero install](velero_install.md)	 - Install Velero
* [velero plugin](velero_plugin.md)	 - Work with plugins
* [velero repo](velero_repo.md)	 - Work with repositories
* [velero restore](velero_restore.md)	 - Work with restores
* [velero schedule](velero_schedule.md)	 - Work with schedules
* [velero snapshot-location](velero_snapshot-location.md)	 - Work with snapshot locations
* [velero uninstall](velero_uninstall.md)	 - Uninstall Velero
* [velero version](velero_version.md)	 - Print the velero version and associated image

