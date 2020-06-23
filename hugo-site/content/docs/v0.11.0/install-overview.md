# Set up Velero on your platform

You can run Velero with a cloud provider or on-premises. For detailed information about the platforms that Velero supports, see [Compatible Storage Providers][99].

In version 0.7.0 and later, you can run Velero in any namespace, which requires additional customization. See [Run in custom namespace][3].

In version 0.9.0 and later, you can use Velero's integration with restic, which requires additional setup. See [restic instructions][20].

## Customize configuration

Whether you run Velero on a cloud provider or on-premises, if you have more than one volume snapshot location for a given volume provider, you can specify its default location for backups by setting a server flag in your Velero deployment YAML.

For details, see the documentation topics for individual cloud providers.

## Cloud provider

The Velero repository includes a set of example YAML files that specify the settings for each supported cloud provider. For provider-specific instructions, see:

* [Run Velero on AWS][0]
* [Run Velero on GCP][1]
* [Run Velero on Azure][2]
* [Use IBM Cloud Object Store as Velero's storage destination][4]

## On-premises

You can run Velero in an on-premises cluster in different ways depending on your requirements. 

First, you must select an object storage backend that Velero can use to store backup data. [Compatible Storage Providers][99] contains information on various
options that are supported or have been reported to work by users. [Minio][101] is an option if you want to keep your backup data on-premises and you are 
not using another storage platform that offers an S3-compatible object storage API.

Second, if you need to back up persistent volume data, you must select a volume backup solution. [Volume Snapshot Providers][100] contains information on
the supported options. For example, if you use [Portworx][102] for persistent storage, you can install their Velero plugin to get native Portworx snapshots as part
of your Velero backups. If there is no native snapshot plugin available for your storage platform, you can use Velero's [restic integration][20], which provides a
platform-agnostic backup solution for volume data.

## Examples

After you set up the Velero server, try these examples:

### Basic example (without PersistentVolumes)

1. Start the sample nginx app:

    ```bash
    kubectl apply -f config/nginx-app/base.yaml
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
    kubectl apply -f config/nginx-app/with-pv.yaml
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
