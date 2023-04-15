---
layout: docs
title: velero install
---
Install Velero

### Synopsis

Install Velero onto a Kubernetes cluster using the supplied provider information, such as
the provider's name, a bucket name, and a file containing the credentials to access that bucket.
A prefix within the bucket and configuration for the backup store location may also be supplied.
Additionally, volume snapshot information for the same provider may be supplied.

All required CustomResourceDefinitions will be installed to the server, as well as the
Velero Deployment and associated node-agent DaemonSet.

The provided secret data will be created in a Secret named 'cloud-credentials'.

All namespaced resources will be placed in the 'velero' namespace by default. 

The '--namespace' flag can be used to specify a different namespace to install into.

Use '--wait' to wait for the Velero Deployment to be ready before proceeding.

Use '-o yaml' or '-o json'  with '--dry-run' to output all generated resources as text instead of sending the resources to the server.
This is useful as a starting point for more customized installations.
		

```
velero install [flags]
```

### Examples

```
  # velero install --provider gcp --plugins velero/velero-plugin-for-gcp:v1.0.0 --bucket mybucket --secret-file ./gcp-service-account.json

  # velero install --provider aws --plugins velero/velero-plugin-for-aws:v1.0.0 --bucket backups --secret-file ./aws-iam-creds --backup-location-config region=us-east-2 --snapshot-location-config region=us-east-2

  # velero install --provider aws --plugins velero/velero-plugin-for-aws:v1.0.0 --bucket backups --secret-file ./aws-iam-creds --backup-location-config region=us-east-2 --snapshot-location-config region=us-east-2 --use-node-agent

  # velero install --provider gcp --plugins velero/velero-plugin-for-gcp:v1.0.0 --bucket gcp-backups --secret-file ./gcp-creds.json --wait

  # velero install --provider aws --plugins velero/velero-plugin-for-aws:v1.0.0 --bucket backups --backup-location-config region=us-west-2 --snapshot-location-config region=us-west-2 --no-secret --pod-annotations iam.amazonaws.com/role=arn:aws:iam::<AWS_ACCOUNT_ID>:role/<VELERO_ROLE_NAME>

  # velero install --provider gcp --plugins velero/velero-plugin-for-gcp:v1.0.0 --bucket gcp-backups --secret-file ./gcp-creds.json --velero-pod-cpu-request=1000m --velero-pod-cpu-limit=5000m --velero-pod-mem-request=512Mi --velero-pod-mem-limit=1024Mi

  # velero install --provider gcp --plugins velero/velero-plugin-for-gcp:v1.0.0 --bucket gcp-backups --secret-file ./gcp-creds.json --node-agent-pod-cpu-request=1000m --node-agent-pod-cpu-limit=5000m --node-agent-pod-mem-request=512Mi --node-agent-pod-mem-limit=1024Mi

  # velero install --provider azure --plugins velero/velero-plugin-for-microsoft-azure:v1.0.0 --bucket $BLOB_CONTAINER --secret-file ./credentials-velero --backup-location-config resourceGroup=$AZURE_BACKUP_RESOURCE_GROUP,storageAccount=$AZURE_STORAGE_ACCOUNT_ID[,subscriptionId=$AZURE_BACKUP_SUBSCRIPTION_ID] --snapshot-location-config apiTimeout=<YOUR_TIMEOUT>[,resourceGroup=$AZURE_BACKUP_RESOURCE_GROUP,subscriptionId=$AZURE_BACKUP_SUBSCRIPTION_ID]
```

### Options

```
      --backup-location-config mapStringString     Configuration to use for the backup storage location. Format is key1=value1,key2=value2
      --bucket string                              Name of the object storage bucket where backups should be stored
      --cacert string                              File containing a certificate bundle to use when verifying TLS connections to the object store. Optional.
      --crds-only                                  Only generate CustomResourceDefinition resources. Useful for updating CRDs for an existing Velero install.
      --default-repo-maintain-frequency duration   How often 'maintain' is run for backup repositories by default. Optional.
      --default-volumes-to-fs-backup               Bool flag to configure Velero server to use pod volume file system backup by default for all volumes on all backups. Optional.
      --dry-run                                    Generate resources, but don't send them to the cluster. Use with -o. Optional.
      --garbage-collection-frequency duration      How often the garbage collection runs for expired backups.(default 1h)
  -h, --help                                       help for install
      --image string                               Image to use for the Velero and node agent pods. Optional. (default "velero/velero:latest")
  -L, --label-columns stringArray                  Accepts a comma separated list of labels that are going to be presented as columns. Names are case-sensitive. You can also use multiple flag options like -L label1 -L label2...
      --no-default-backup-location                 Flag indicating if a default backup location should be created. Must be used as confirmation if --bucket or --provider are not provided. Optional.
      --no-secret                                  Flag indicating if a secret should be created. Must be used as confirmation if --secret-file is not provided. Optional.
      --node-agent-pod-cpu-limit string            CPU limit for node-agent pod. A value of "0" is treated as unbounded. Optional. (default "1000m")
      --node-agent-pod-cpu-request string          CPU request for node-agent pod. A value of "0" is treated as unbounded. Optional. (default "500m")
      --node-agent-pod-mem-limit string            Memory limit for node-agent pod. A value of "0" is treated as unbounded. Optional. (default "1Gi")
      --node-agent-pod-mem-request string          Memory request for node-agent pod. A value of "0" is treated as unbounded. Optional. (default "512Mi")
  -o, --output string                              Output display format. For create commands, display the object but do not send it to the server. Valid formats are 'table', 'json', and 'yaml'. 'table' is not valid for the install command.
      --plugins stringArray                        Plugin container images to install into the Velero Deployment
      --pod-annotations mapStringString            Annotations to add to the Velero and node agent pods. Optional. Format is key1=value1,key2=value2
      --pod-labels mapStringString                 Labels to add to the Velero and node agent pods. Optional. Format is key1=value1,key2=value2
      --prefix string                              Prefix under which all Velero data should be stored within the bucket. Optional.
      --provider string                            Provider name for backup and volume storage
      --restore-only                               Run the server in restore-only mode. Optional.
      --sa-annotations mapStringString             Annotations to add to the Velero ServiceAccount. Add iam.gke.io/gcp-service-account=[GSA_NAME]@[PROJECT_NAME].iam.gserviceaccount.com for workload identity. Optional. Format is key1=value1,key2=value2
      --secret-file string                         File containing credentials for backup and volume provider. If not specified, --no-secret must be used for confirmation. Optional.
      --service-account-name string                ServiceAccountName to be set to the Velero and node agent pods, it should be created before the installation, and the user also needs to create the rolebinding for it.  Optional, if this attribute is set, the default service account 'velero' will not be created, and the flag --sa-annotations will be disregarded.
      --show-labels                                Show labels in the last column
      --snapshot-location-config mapStringString   Configuration to use for the volume snapshot location. Format is key1=value1,key2=value2
      --uploader-type string                       The type of uploader to transfer the data of pod volumes, the supported values are 'restic', 'kopia' (default "restic")
      --use-node-agent                             Create Velero node-agent daemonset. Optional. Velero node-agent hosts Velero modules that need to run in one or more nodes(i.e. Restic, Kopia).
      --use-volume-snapshots                       Whether or not to create snapshot location automatically. Set to false if you do not plan to create volume snapshots via a storage provider. (default true)
      --velero-pod-cpu-limit string                CPU limit for Velero pod. A value of "0" is treated as unbounded. Optional. (default "1000m")
      --velero-pod-cpu-request string              CPU request for Velero pod. A value of "0" is treated as unbounded. Optional. (default "500m")
      --velero-pod-mem-limit string                Memory limit for Velero pod. A value of "0" is treated as unbounded. Optional. (default "512Mi")
      --velero-pod-mem-request string              Memory request for Velero pod. A value of "0" is treated as unbounded. Optional. (default "128Mi")
      --wait                                       Wait for Velero deployment to be ready. Optional.
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

* [velero](velero.md)	 - Back up and restore Kubernetes cluster resources.

