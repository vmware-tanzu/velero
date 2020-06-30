## Getting started

The following example sets up the Ark server and client, then backs up and restores a sample application.

For simplicity, the example uses Minio, an S3-compatible storage service that runs locally on your cluster.

**NOTE** The example lets you explore basic Ark functionality. In the real world, however, you would back your cluster up to external storage.

See [Set up Ark on your platform][3] for how to configure Ark for a production environment.


### Prerequisites

* Access to a Kubernetes cluster, version 1.7 or later. Version 1.7.5 or later is required to run `ark backup delete`.
* A DNS server on the cluster
* `kubectl` installed

### Download

Clone or fork the Ark repository:

```
git clone git@github.com:heptio/ark.git
```

NOTE: Make sure to check out the appropriate version. We recommend that you check out the latest tagged version. The main branch is under active development and might not be stable.

### Set up server

1. Start the server and the local storage service. In the root directory of Ark, run:

    ```bash
    kubectl apply -f examples/common/00-prereqs.yaml
    kubectl apply -f examples/minio/
    ```

    NOTE: If you get an error about Config creation, wait for a minute, then run the commands again.

1. Deploy the example nginx application:

    ```bash
    kubectl apply -f examples/nginx-app/base.yaml
    ```

1. Check to see that both the Ark and nginx deployments are successfully created:

    ```
    kubectl get deployments -l component=ark --namespace=heptio-ark
    kubectl get deployments --namespace=nginx-example
    ```

### Install client

[Download the client][26].

Make sure that you install somewhere in your PATH.

### Back up

1. Create a backup for any object that matches the `app=nginx` label selector:

    ```
    ark backup create nginx-backup --selector app=nginx
    ```

   Alternatively if you want to backup all objects *except* those matching the label `backup=ignore`:

   ```
   ark backup create nginx-backup --selector 'backup notin (ignore)'
   ```

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

### Restore

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

### Clean up

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
kubectl delete -f examples/common/
kubectl delete -f examples/minio/
kubectl delete -f examples/nginx-app/base.yaml
```

[3]: install-overview.md
[18]: debugging-restores.md
[26]: https://github.com/heptio/ark/releases
