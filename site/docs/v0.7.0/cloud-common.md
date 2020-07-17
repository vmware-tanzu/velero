# Set up Ark with your cloud provider

To run Ark with your cloud provider, you specify provider-specific settings for the Ark server. In version 0.7.0 and later, you can run Ark in any namespace, which requires additional customization. See [Run in custom namespace][3].

The Ark repository includes a set of example YAML files that specify the settings for each cloud provider. For provider-specific instructions, see:

* [Run Ark on AWS][0]
* [Run Ark on GCP][1]
* [Run Ark on Azure][2]

## Examples

After you set up the Ark server, try these examples:

### Basic example (without PersistentVolumes)

1. Start the sample nginx app:

    ```bash
    kubectl apply -f examples/nginx-app/base.yaml
    ```

1. Create a backup:

    ```bash
    ark backup create nginx-backup --include-namespaces nginx-example
    ```

1. Simulate a disaster:

    ```bash
    kubectl delete namespaces nginx-example
    ```

    Wait for the namespace to be deleted.

1. Restore your lost resources:

    ```bash
    ark restore create nginx-backup
    ```

### Snapshot example (with PersistentVolumes)

> NOTE: For Azure, your Kubernetes cluster needs to be version 1.7.2+ to support PV snapshotting of its managed disks.

1. Start the sample nginx app:

    ```bash
    kubectl apply -f examples/nginx-app/with-pv.yaml
    ```

1. Create a backup with PV snapshotting:

    ```bash
    ark backup create nginx-backup --include-namespaces nginx-example
    ```

1. Simulate a disaster:

    ```bash
    kubectl delete namespaces nginx-example
    ```

    Because the default [reclaim policy][19] for dynamically-provisioned PVs is "Delete", these commands should trigger your cloud provider to delete the disk backing the PV. The deletion process is asynchronous so this may take some time. **Before continuing to the next step, check your cloud provider to confirm that the disk no longer exists.**

1. Restore your lost resources:

    ```bash
    ark restore create nginx-backup
    ```

[0]: /aws-config.md
[1]: /gcp-config.md
[2]: /azure-config.md
[3]: /namespace.md
[19]: https://kubernetes.io/docs/concepts/storage/persistent-volumes/#reclaiming
