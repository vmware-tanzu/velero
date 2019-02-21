## Getting started

The following example sets up the Velero server and client, then backs up and restores a sample application. 

For simplicity, the example uses Minio, an S3-compatible storage service that runs locally on your cluster. 
For additional functionality with this setup, see the docs on how to [expose Minio outside your cluster][31].

**NOTE** The example lets you explore basic Velero functionality. Configuring Minio for production is out of scope.

See [Set up Velero on your platform][3] for how to configure Velero for a production environment.

If you encounter issues with installing or configuring, see [Debugging Installation Issues](debugging-install.md).

### Prerequisites

* Access to a Kubernetes cluster, version 1.7 or later. Version 1.7.5 or later is required to run `velero backup delete`.
* A DNS server on the cluster
* `kubectl` installed

## Download Velero

1. Download the [latest release's](https://github.com/heptio/velero/releases) tarball for your client platform.

1. Extract the tarball:
    ```bash
    tar -xvf <RELEASE-TARBALL-NAME>.tar.gz -C /dir/to/extract/to 
    ```
    We'll refer to the directory you extracted to as the "Velero directory" in subsequent steps.

1. Move the `velero` binary from the Velero directory to somewhere in your PATH.

_We strongly recommend that you use an [official release](https://github.com/heptio/velero/releases) of Velero. The tarballs for each release contain the
`velero` command-line client **and** version-specific sample YAML files for deploying Velero to your cluster. The code and sample YAML files in the master 
branch of the Velero repository are under active development and are not guaranteed to be stable. Use them at your own risk!_

#### MacOS Installation

On Mac, you can use [HomeBrew](https://brew.sh) to install the `velero` client:
```bash
brew install velero
```

### Set up server

These instructions start the Velero server and a Minio instance that is accessible from within the cluster only. See [Expose Minio outside your cluster][31] for information about configuring your cluster for outside access to Minio. Outside access is required to access logs and run `velero describe` commands.

1.  Start the server and the local storage service. In the Velero directory, run:

    ```bash
    kubectl apply -f config/common/00-prereqs.yaml
    kubectl apply -f config/minio/
    ```

1. Deploy the example nginx application:

    ```bash
    kubectl apply -f config/nginx-app/base.yaml
    ```

1. Check to see that both the Velero and nginx deployments are successfully created:

    ```
    kubectl get deployments -l component=velero --namespace=velero
    kubectl get deployments --namespace=nginx-example
    ```

### Back up

1. Create a backup for any object that matches the `app=nginx` label selector:

    ```
    velero backup create nginx-backup --selector app=nginx
    ```

   Alternatively if you want to backup all objects *except* those matching the label `backup=ignore`:

   ```
   velero backup create nginx-backup --selector 'backup notin (ignore)'
   ```

1. (Optional) Create regularly scheduled backups based on a cron expression using the `app=nginx` label selector:

    ```
    velero schedule create nginx-daily --schedule="0 1 * * *" --selector app=nginx
    ```

    Alternatively, you can use some non-standard shorthand cron expressions:

    ```
    velero schedule create nginx-daily --schedule="@daily" --selector app=nginx
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

### Restore

1. Run:

    ```
    velero restore create --from-backup nginx-backup
    ```

1. Run:

    ```
    velero restore get
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
velero restore describe <RESTORE_NAME>
```

For more information, see [the debugging information][18].

### Clean up

If you want to delete any backups you created, including data in object storage and persistent
volume snapshots, you can run:

```
velero backup delete BACKUP_NAME
```

This asks the Velero server to delete all backup data associated with `BACKUP_NAME`.  You need to do
this for each backup you want to permanently delete. A future version of Velero will allow you to
delete multiple backups by name or label selector.

Once fully removed, the backup is no longer visible when you run:

```
velero backup get BACKUP_NAME
```

If you want to uninstall Velero but preserve the backup data in object storage and persistent volume
snapshots, it is safe to remove the `velero` namespace and everything else created for this
example:

```
kubectl delete -f config/common/
kubectl delete -f config/minio/
kubectl delete -f config/nginx-app/base.yaml
```

[31]: expose-minio.md
[3]: install-overview.md
[18]: debugging-restores.md
[26]: https://github.com/heptio/velero/releases
[30]: https://godoc.org/github.com/robfig/cron
