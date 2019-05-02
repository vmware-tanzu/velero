# Expose Minio outside your cluster

When you run commands to get logs or describe a backup, the Velero server generates a pre-signed URL to download the requested items. To access these URLs from outside the cluster -- that is, from your Velero client -- you need to make Minio available outside the cluster. You can:

- Change the Minio Service type from `ClusterIP` to `NodePort`.
- Set up Ingress for your cluster, keeping Minio Service type `ClusterIP`.

You can also specify a `publicUrl` config field for the pre-signed URL in your backup storage location config.

For basic instructions on how to install the Velero server and client, see [the getting started example][1].

## Expose Minio with Service of type NodePort

The Minio deployment by default specifies a Service of type `ClusterIP`. You can change this to `NodePort` to easily expose a cluster service externally if you can reach the node from your Velero client.

You must also get the Minio URL, which you can then specify as the value of the `publicUrl` field in your backup storage location config.

1.  In `examples/minio/00-minio-deployment.yaml`, change the value of Service `spec.type` from `ClusterIP` to `NodePort`.

1.  Get the Minio URL:

    - if you're running Minikube:

      ```shell
      minikube service minio --namespace=velero --url
      ```

    - in any other environment:

      1.  Get the value of an external IP address or DNS name of any node in your cluster. You must be able to reach this address from the Velero client.

      1.  Append the value of the NodePort to get a complete URL. You can get this value by running:

          ```shell
          kubectl -n velero get svc/minio -o jsonpath='{.spec.ports[0].nodePort}'
          ```

1.  Edit your `BackupStorageLocation` YAML, adding `publicUrl: <URL_FROM_PREVIOUS_STEP>` as a field under `spec.config`. You must include the `http://` or `https://` prefix.

## Work with Ingress

Configuring Ingress for your cluster is out of scope for the Velero documentation. If you have already set up Ingress, however, it makes sense to continue with it while you run the example Velero configuration with Minio.

In this case: 

1.  Keep the Service type as `ClusterIP`.

1.  Edit your `BackupStorageLocation` YAML, adding `publicUrl: <URL_AND_PORT_OF_INGRESS>` as a field under `spec.config`.

[1]: get-started.md
