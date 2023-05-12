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
Velero Deployment and associated Restic DaemonSet.

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

  # velero install --provider aws --plugins velero/velero-plugin-for-aws:v1.0.0 --bucket backups --secret-file ./aws-iam-creds --backup-location-config region=us-east-2 --snapshot-location-config region=us-east-2 --use-restic

  # velero install --provider gcp --plugins velero/velero-plugin-for-gcp:v1.0.0 --bucket gcp-backups --secret-file ./gcp-creds.json --wait

  # velero install --provider aws --plugins velero/velero-plugin-for-aws:v1.0.0 --bucket backups --backup-location-config region=us-west-2 --snapshot-location-config region=us-west-2 --no-secret --pod-annotations iam.amazonaws.com/role=arn:aws:iam::<AWS_ACCOUNT_ID>:role/<VELERO_ROLE_NAME>

  # velero install --provider gcp --plugins velero/velero-plugin-for-gcp:v1.0.0 --bucket gcp-backups --secret-file ./gcp-creds.json --velero-pod-cpu-request=1000m --velero-pod-cpu-limit=5000m --velero-pod-mem-request=512Mi --velero-pod-mem-limit=1024Mi

  # velero install --provider gcp --plugins velero/velero-plugin-for-gcp:v1.0.0 --bucket gcp-backups --secret-file ./gcp-creds.json --restic-pod-cpu-request=1000m --restic-pod-cpu-limit=5000m --restic-pod-mem-request=512Mi --restic-pod-mem-limit=1024Mi

  # velero install --provider azure --plugins velero/velero-plugin-for-microsoft-azure:v1.0.0 --bucket $BLOB_CONTAINER --secret-file ./credentials-velero --backup-location-config resourceGroup=$AZURE_BACKUP_RESOURCE_GROUP,storageAccount=$AZURE_STORAGE_ACCOUNT_ID[,subscriptionId=$AZURE_BACKUP_SUBSCRIPTION_ID] --snapshot-location-config apiTimeout=<YOUR_TIMEOUT>[,resourceGroup=$AZURE_BACKUP_RESOURCE_GROUP,subscriptionId=$AZURE_BACKUP_SUBSCRIPTION_ID]
```

### Options

```
      --backup-location-config mapStringString     Configuration to use for the backup storage location. Format is key1=value1,key2=value2
      --bucket string                              Name of the object storage bucket where backups should be stored
      --cacert string                              File containing a certificate bundle to use when verifying TLS connections to the object store. Optional.
      --crds-only                                  Only generate CustomResourceDefinition resources. Useful for updating CRDs for an existing Velero install.
      --crds-version string                        The version to generate CustomResourceDefinition resources if Velero can't discover the Kubernetes preferred CRD API version. Optional. (default "v1")
      --default-restic-prune-frequency duration    How often 'restic prune' is run for restic repositories by default. Optional.
      --default-volumes-to-restic                  Bool flag to configure Velero server to use restic by default to backup all pod volumes on all backups. Optional.
      --dry-run                                    Generate resources, but don't send them to the cluster. Use with -o. Optional.
  -h, --help                                       help for install
      --image string                               Image to use for the Velero and restic server pods. Optional. (default "velero/velero:latest")
      --label-columns stringArray                  A comma-separated list of labels to be displayed as columns
      --no-default-backup-location                 Flag indicating if a default backup location should be created. Must be used as confirmation if --bucket or --provider are not provided. Optional.
      --no-secret                                  Flag indicating if a secret should be created. Must be used as confirmation if --secret-file is not provided. Optional.
  -o, --output string                              Output display format. For create commands, display the object but do not send it to the server. Valid formats are 'table', 'json', and 'yaml'. 'table' is not valid for the install command.
      --plugins stringArray                        Plugin container images to install into the Velero Deployment
      --pod-annotations mapStringString            Annotations to add to the Velero and restic pods. Optional. Format is key1=value1,key2=value2
      --prefix string                              Prefix under which all Velero data should be stored within the bucket. Optional.
      --provider string                            Provider name for backup and volume storage
      --restic-pod-cpu-limit string                CPU limit for restic pod. A value of "0" is treated as unbounded. Optional. (default "1000m")
      --restic-pod-cpu-request string              CPU request for restic pod. A value of "0" is treated as unbounded. Optional. (default "500m")
      --restic-pod-mem-limit string                Memory limit for restic pod. A value of "0" is treated as unbounded. Optional. (default "1Gi")
      --restic-pod-mem-request string              Memory request for restic pod. A value of "0" is treated as unbounded. Optional. (default "512Mi")
      --restore-only                               Run the server in restore-only mode. Optional.
      --sa-annotations mapStringString             Annotations to add to the Velero ServiceAccount. Add iam.gke.io/gcp-service-account=[GSA_NAME]@[PROJECT_NAME].iam.gserviceaccount.com for workload identity. Optional. Format is key1=value1,key2=value2
      --secret-file string                         File containing credentials for backup and volume provider. If not specified, --no-secret must be used for confirmation. Optional.
      --show-labels                                Show labels in the last column
      --snapshot-location-config mapStringString   Configuration to use for the volume snapshot location. Format is key1=value1,key2=value2
      --use-restic                                 Create restic daemonset. Optional.
      --use-volume-snapshots                       Whether or not to create snapshot location automatically. Set to false if you do not plan to create volume snapshots via a storage provider. (default true)
      --velero-pod-cpu-limit string                CPU limit for Velero pod. A value of "0" is treated as unbounded. Optional. (default "1000m")
      --velero-pod-cpu-request string              CPU request for Velero pod. A value of "0" is treated as unbounded. Optional. (default "500m")
      --velero-pod-mem-limit string                Memory limit for Velero pod. A value of "0" is treated as unbounded. Optional. (default "512Mi")
      --velero-pod-mem-request string              Memory request for Velero pod. A value of "0" is treated as unbounded. Optional. (default "128Mi")
      --wait                                       Wait for Velero deployment to be ready. Optional.
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

* [velero](velero.md)	 - Back up and restore Kubernetes cluster resources.

