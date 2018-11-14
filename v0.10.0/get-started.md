# Getting started

The following example sets up the Ark server and client, then backs up and restores a sample application.

For simplicity, the example uses Minio, an S3-compatible storage service that runs locally on your cluster.

**NOTE** The example lets you explore basic Ark functionality. Configuring Minio for production is out of scope.

See [Set up Ark on your platform][3] for how to configure Ark for a production environment.

## Prerequisites

* Access to a Kubernetes cluster, version 1.7 or later. Version 1.7.5 or later is required to run `ark backup delete`.
* A DNS server on the cluster
* `kubectl` installed

### Download

1. Download the [latest release's][26] tarball for your platform.

1. Extract the tarball:
    ```bash
    tar -xzf <RELEASE-TARBALL-NAME>.tar.gz -C /dir/to/extract/to 
    ```
    We'll refer to the directory you extracted to as the "Ark directory" in subsequent steps.

1. Move the `ark` binary from the Ark directory to somewhere in your PATH.

## Set up server

These instructions start the Ark server and a Minio instance that is accessible from within the cluster only. See the following section for information about configuring your cluster for outside access to Minio. Outside access is required to access logs and run `ark describe` commands.

1.  Start the server and the local storage service. In the Ark directory, run:

    ```bash
    kubectl apply -f config/common/00-prereqs.yaml
    kubectl apply -f config/minio/
    ```

1. Deploy the example nginx application:

    ```bash
    kubectl apply -f config/nginx-app/base.yaml
    ```

1. Check to see that both the Ark and nginx deployments are successfully created:

    ```
    kubectl get deployments -l component=ark --namespace=heptio-ark
    kubectl get deployments --namespace=nginx-example
    ```

## (Optional) Expose Minio outside your cluster

When you run commands to get logs or describe a backup, the Ark server generates a pre-signed URL to download the requested items. To access these URLs from outside the cluster -- that is, from your Ark client -- you need to make Minio available outside the cluster. You can:

- Change the Minio Service type from `ClusterIP` to `NodePort`.
- Set up Ingress for your cluster, keeping Minio Service type `ClusterIP`.

In Ark 0.10, you can also specify the value of a new `publicUrl` field for the pre-signed URL in your backup storage config.

### Expose Minio with Service of type NodePort

The Minio deployment by default specifies a Service of type `ClusterIP`. You can change this to `NodePort` to easily expose a cluster service externally if you can reach the node from your Ark client.

You must also get the Minio URL, which you can then specify as the value of the new `publicUrl` field in your backup storage config.

1.  In `examples/minio/00-minio-deployment.yaml`, change the value of Service `spec.type` from `ClusterIP` to `NodePort`.

1.  Get the Minio URL:

    - if you're running Minikube:

      ```shell
      minikube service minio --namespace=heptio-ark --url
      ```

    - in any other environment:

      1.  Get the value of an external IP address or DNS name of any node in your cluster. You must be able to reach this address from the Ark client.

      1.  Append the value of the NodePort to get a complete URL. You can get this value by running:

          ```shell
          kubectl -n heptio-ark get svc/minio -o jsonpath='{.spec.ports[0].nodePort}'
          ```

1.  In `examples/minio/05-ark-backupstoragelocation.yaml`, uncomment the `publicUrl` line and provide this Minio URL as the value of the `publicUrl` field. You must include the `http://` or `https://` prefix.

### Work with Ingress

Configuring Ingress for your cluster is out of scope for the Ark documentation. If you have already set up Ingress, however, it makes sense to continue with it while you run the example Ark configuration with Minio.

In this case: 

1.  Keep the Service type as `ClusterIP`.

1.  In `examples/minio/05-ark-backupstoragelocation.yaml`, uncomment the `publicUrl` line and provide the URL and port of your Ingress as the value of the `publicUrl` field.

## Back up

1. Create a backup for any object that matches the `app=nginx` label selector:

    ```
    ark backup create nginx-backup --selector app=nginx
    ```

   Alternatively if you want to backup all objects *except* those matching the label `backup=ignore`:

   ```
   ark backup create nginx-backup --selector 'backup notin (ignore)'
   ```

1. (Optional) Create regularly scheduled backups based on a cron expression using the `app=nginx` label selector:

    ```
    ark schedule create nginx-daily --schedule="0 1 * * *" --selector app=nginx
    ```

    Alternatively, you can use some non-standard shorthand cron expressions:

    ```
    ark schedule create nginx-daily --schedule="@daily" --selector app=nginx
    ```

    See the [cron package's documentation][30] for more usage examples.

1. Simulate a disaster:

    ```
    kubectl delete namespace nginx-example
    ```

1. To check that the nginx deployment and service are gone, run:

    ```
    kubectl get deployments --namespace=nginx-example
    kubectl get services --namespace=nginx-example
    kubectl get namespace/nginx-example
    ```

    You should get no results.
    
    NOTE: You might need to wait for a few minutes for the namespace to be fully cleaned up.

## Restore

1. Run:

    ```
    ark restore create --from-backup nginx-backup
    ```

1. Run:

    ```
    ark restore get
    ```

    After the restore finishes, the output looks like the following:

    ```
    NAME                          BACKUP         STATUS      WARNINGS   ERRORS    CREATED                         SELECTOR
    nginx-backup-20170727200524   nginx-backup   Completed   0          0         2017-07-27 20:05:24 +0000 UTC   <none>
    ```

NOTE: The restore can take a few moments to finish. During this time, the `STATUS` column reads `InProgress`.

After a successful restore, the `STATUS` column is `Completed`, and `WARNINGS` and `ERRORS` are 0. All objects in the `nginx-example` namespace should be just as they were before you deleted them.

If there are errors or warnings, you can look at them in detail:

```
ark restore describe <RESTORE_NAME>
```

For more information, see [the debugging information][18].

## Clean up

If you want to delete any backups you created, including data in object storage and persistent
volume snapshots, you can run:

```
ark backup delete BACKUP_NAME
```

This asks the Ark server to delete all backup data associated with `BACKUP_NAME`.  You need to do
this for each backup you want to permanently delete. A future version of Ark will allow you to
delete multiple backups by name or label selector.

Once fully removed, the backup is no longer visible when you run:

```
ark backup get BACKUP_NAME
```

If you want to uninstall Ark but preserve the backup data in object storage and persistent volume
snapshots, it is safe to remove the `heptio-ark` namespace and everything else created for this
example:

```
kubectl delete -f config/common/
kubectl delete -f config/minio/
kubectl delete -f config/nginx-app/base.yaml
```

[3]: install-overview.md
[18]: debugging-restores.md
[26]: https://github.com/heptio/ark/releases
[30]: https://godoc.org/github.com/robfig/cron
