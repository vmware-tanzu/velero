# Set up Velero on your platform

You can run Velero with a cloud provider or on-premises. For detailed information about the platforms that Velero supports, see [Compatible Storage Providers][99].

You can run Velero in any namespace, which requires additional customization. See [Run in custom namespace][3].

You can also use Velero's integration with restic, which requires additional setup. See [restic instructions][20].

## Cloud provider

The Velero client includes an `install` command to specify the settings for each supported cloud provider. You can install Velero for the included cloud providers using the following command:

```bash
velero install \
    --provider <YOUR_PROVIDER> \
    --bucket <YOUR_BUCKET> \
    [--secret-file <PATH_TO_FILE>] \
    [--no-secret] \
    [--backup-location-config] \
    [--snapshot-location-config] \
    [--namespace] \
    [--use-volume-snapshots] \
    [--use-restic] \
    [--pod-annotations] \
```

When using node-based IAM policies, `--secret-file` is not required, but `--no-secret` is required for confirmation.

For provider-specific instructions, see:

* [Run Velero on AWS][0]
* [Run Velero on GCP][1]
* [Run Velero on Azure][2]
* [Use IBM Cloud Object Store as Velero's storage destination][4]

When using restic on a storage provider that doesn't currently have Velero support for snapshots, the `--use-volume-snapshots=false` flag prevents an unused `VolumeSnapshotLocation` from being created on installation.

To see the YAML applied by the `velero install` command, use the `--dry-run -o yaml` arguments.

For more complex installation needs, use either the generated YAML, or the Helm chart.

## On-premises

You can run Velero in an on-premises cluster in different ways depending on your requirements.

First, you must select an object storage backend that Velero can use to store backup data. [Compatible Storage Providers][99] contains information on various
options that are supported or have been reported to work by users. [Minio][101] is an option if you want to keep your backup data on-premises and you are
not using another storage platform that offers an S3-compatible object storage API.

Second, if you need to back up persistent volume data, you must select a volume backup solution. [Volume Snapshot Providers][100] contains information on
the supported options. For example, if you use [Portworx][102] for persistent storage, you can install their Velero plugin to get native Portworx snapshots as part
of your Velero backups. If there is no native snapshot plugin available for your storage platform, you can use Velero's [restic integration][20], which provides a
platform-agnostic backup solution for volume data.

## Customize configuration

Whether you run Velero on a cloud provider or on-premises, if you have more than one volume snapshot location for a given volume provider, you can specify its default location for backups by setting a server flag in your Velero deployment YAML.

For details, see the documentation topics for individual cloud providers.

## Velero resource requirements

By default, the Velero deployment requests 500m CPU, 128Mi memory and sets a limit of 1000m CPU, 256Mi.
Default requests and limits are not set for the restic pods as CPU/Memory usage can depend heavily on the size of volumes being backed up. 
If you need to customize these resource requests and limits, you can set the following flags in your `velero install` command:

```
velero install \
    --provider <YOUR_PROVIDER> \
    --bucket <YOUR_BUCKET> \
    --secret-file <PATH_TO_FILE> \
    --velero-pod-cpu-request <CPU_REQUEST> \
    --velero-pod-mem-request <MEMORY_REQUEST> \
    --velero-pod-cpu-limit <CPU_LIMIT> \
    --velero-pod-mem-limit <MEMORY_LIMIT> \
    [--use-restic] \
    [--restic-pod-cpu-request <CPU_REQUEST>] \
    [--restic-pod-mem-request <MEMORY_REQUEST>] \
    [--restic-pod-cpu-limit <CPU_LIMIT>] \
    [--restic-pod-mem-limit <MEMORY_LIMIT>]
```


Values for these flags follow the same format as [Kubernetes resource requirements][103].

## Removing Velero

If you would like to completely uninstall Velero from your cluster, the following commands will remove all resources created by `velero install`:

```bash
kubectl delete namespace/velero clusterrolebinding/velero
kubectl delete crds -l component=velero
```

## Installing with the Helm chart

When installing using the Helm chart, the provider's credential information will need to be appended into your values.

The easiest way to do this is with the `--set-file` argument, available in Helm 2.10 and higher.

```bash
helm install --set-file credentials.secretContents.cloud=./credentials-velero stable/velero
```

See your cloud provider's documentation for the contents and creation of the `credentials-velero` file.

## Examples

After you set up the Velero server, try these examples:

### Basic example (without PersistentVolumes)

1. Start the sample nginx app:

    ```bash
    kubectl apply -f examples/nginx-app/base.yaml
    ```

1. Create a backup:

    ```bash
    velero backup create nginx-backup --include-namespaces nginx-example
    ```

1. Simulate a disaster:

    ```bash
    kubectl delete namespaces nginx-example
    ```

    Wait for the namespace to be deleted.

1. Restore your lost resources:

    ```bash
    velero restore create --from-backup nginx-backup
    ```

### Snapshot example (with PersistentVolumes)

> NOTE: For Azure, you must run Kubernetes version 1.7.2 or later to support PV snapshotting of managed disks.

1. Start the sample nginx app:

    ```bash
    kubectl apply -f examples/nginx-app/with-pv.yaml
    ```

1. Create a backup with PV snapshotting:

    ```bash
    velero backup create nginx-backup --include-namespaces nginx-example
    ```

1. Simulate a disaster:

    ```bash
    kubectl delete namespaces nginx-example
    ```

    Because the default [reclaim policy][19] for dynamically-provisioned PVs is "Delete", these commands should trigger your cloud provider to delete the disk that backs the PV. Deletion is asynchronous, so this may take some time. **Before continuing to the next step, check your cloud provider to confirm that the disk no longer exists.**

1. Restore your lost resources:

    ```bash
    velero restore create --from-backup nginx-backup
    ```

[0]: aws-config.md
[1]: gcp-config.md
[2]: azure-config.md
[3]: namespace.md
[4]: ibm-config.md
[19]: https://kubernetes.io/docs/concepts/storage/persistent-volumes/#reclaiming
[20]: restic.md
[99]: support-matrix.md
[100]: support-matrix.md#volume-snapshot-providers
[101]: https://www.minio.io
[102]: https://portworx.com
[103]: https://kubernetes.io/docs/concepts/configuration/manage-compute-resources-container/#meaning-of-cpu
