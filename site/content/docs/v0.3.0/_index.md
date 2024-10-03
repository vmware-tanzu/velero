---
version: v0.3.0
---
# Heptio Ark

**Maintainers:** [Heptio][0]

[![Build Status][1]][2]

## Overview
Heptio Ark is a utility for managing disaster recovery, specifically for your [Kubernetes][14] cluster resources and persistent volumes. It provides a simple, configurable, and operationally robust way to back up and restore applications and PVs from a series of checkpoints. This allows you to better automate in the following scenarios:

* **Disaster recovery** with reduced TTR (time to respond), in the case of:
    * Infrastructure loss
    * Data corruption
    * Service outages

* **Cross-cloud-provider migration** for Kubernetes API objects (cross-cloud-provider migration of persistent volume snapshots not yet supported)

* **Dev and testing environment setup (+ CI)**, via replication of prod environment

More concretely, Heptio Ark combines an in-cluster service with a CLI that allows you to record both:
1. *Configurable subsets of Kubernetes API objects* -- as tarballs stored in object storage
2. *Disk snapshots of Persistent Volumes* -- via the cloud provider APIs

Heptio Ark currently supports the [AWS][15], [GCP][16], and [Azure][17] cloud provider platforms.

## Quickstart

This guide gets Ark up and running on your cluster, and goes through an example using the following:
* **Minio, an S3-compatible storage service** that runs locally on your cluster. This is the storage service where backup files are uploaded. *Note that Ark is intended to run on a cloud provider--we are using Minio here to keep the example convenient and self-contained.*

* **A sample nginx app** under the `nginx-example` namespace, used to demonstrate Ark's backup and restore functionality.

Note that this example *does not* include a demonstration of PV disk snapshots, because that feature requires integration with a cloud provider API. For snapshotting examples and instructions specific to AWS, GCP, and Azure, see [Cloud Provider Specifics][23].

### 0. Prerequisites

* *You should have access to an up-and-running Kubernetes cluster (minimum version 1.7).* If you do not have a cluster, [choose a setup solution][9] from the official Kubernetes docs.

* *You will need to have a DNS server set up on your cluster for the example files to work.* You can check this with `kubectl get svc -l k8s-app=kube-dns --namespace=kube-system`. If said service does not exist, [these instructions][12] may help.

* *You should have `kubectl` installed.* If not, follow the instructions for [installing via Homebrew (MacOS)][10] or [building the binary (Linux)][11].

### 1. Download
Clone or fork the Heptio Ark repo:
```
git clone git@github.com:heptio/ark.git
```

### 2. Setup

There are two types of Ark instances that work in tandem:
1. **Ark server**: Runs persistently on the cluster.
2. **Ark client**: Launched by the user whenever they want to initiate an operation (e.g. a backup).

To get the server started on your cluster (as well as the local storage service), execute the following commands in Ark's root directory:

```
kubectl apply -f examples/common/00-prereqs.yaml
kubectl apply -f examples/minio/
kubectl apply -f examples/common/10-deployment.yaml
```

*NOTE: If you encounter an error related to Config creation, wait for a minute and run the command again. (The Config CRD does not always finish registering in time.)*

Now deploy the example nginx app:
```
kubectl apply -f examples/nginx-app/base.yaml
```

Check to see that both the Ark and nginx deployments have been successfully created:
```
kubectl get deployments -l component=ark --namespace=heptio-ark
kubectl get deployments --namespace=nginx-example
```

Finally, create an alias for the Ark client's Docker executable. (Make sure that your `KUBECONFIG` environment variable is pointing at the proper config first). This will save a lot of future typing:

```
alias ark='docker run --rm -v $(dirname $KUBECONFIG):/kubeconfig -e KUBECONFIG=/kubeconfig/$(basename $KUBECONFIG) gcr.io/heptio-images/ark:latest'
```
*NOTE*: Depending on how your Kubeconfig is written--if it refers to the Kubernetes API server using the host machine's `localhost`, for instance--you may need to add an additional `--net="host"` flag to the `docker run` command.


### 3. Back up and restore
First, create a backup specifically for any object matching the `app=nginx` label selector:

```
ark backup create nginx-backup --selector app=nginx
```

Now you can mimic a disaster with the following:
```
kubectl delete namespace nginx-example
```
Oh no! The nginx deployment and service are both gone, as you can see (though you may have to wait a minute or two for the namespace be fully cleaned up):
```
kubectl get deployments --namespace=nginx-example
kubectl get services --namespace=nginx-example
```
Neither commands should yield any results. However, because Ark has your back(up), you can run this command:
```
ark restore create nginx-backup
```

To check on the status of the Restore:
```
ark restore get
```

The output should look something like the table below:
```
NAME                          BACKUP         STATUS      WARNINGS   ERRORS    CREATED                         SELECTOR
nginx-backup-20170727200524   nginx-backup   Completed   0          0         2017-07-27 20:05:24 +0000 UTC   <none>
```

If the Restore's `STATUS` column is "Completed", and `WARNINGS` and `ERRORS` are both zero, the restore is a success. All of the objects in the `nginx-example` namespace should be just as they were before.

Otherwise, if there are warnings or errors indicated, you can run the following command to look at them in more detail:
```
ark restore get <RESTORE NAME> -o yaml
```
See the [debugging documentation][18] for more details.

*NOTE*: In the example files, the `storage` volume is defined via `hostPath` for better visibility. If you're curious to see the [structure of the backup files][13] firsthand, you can find the compressed results in `/tmp/minio/ark/nginx-backup`.

### 4. Tear Down
Using the following command, you can remove all Kubernetes objects associated with this example:
```
kubectl delete -f examples/common/
kubectl delete -f examples/minio/
kubectl delete -f examples/nginx-app/base.yaml
```

## Architecture

Each of Heptio Ark's operations (Backups, Schedules, and Restores) are custom resources themselves, defined using [CRDs][20]. Their accompanying [custom controllers][21] handle them when they are submitted to the Kubernetes API server.

As mentioned before, Ark runs in two different modes:

* **Ark client**: Allows you to query, create, and delete the Ark resources as desired.

* **Ark server**: Runs all of the Ark controllers. Each controller watches its respective custom resource for API operations, performs validation, and handles the majority of the cloud API logic (e.g. interfacing with object storage and persistent volumes).

Looking at a specific example--an `ark backup create test-backup --snapshot-volumes` command triggers the following operations:

![19]

1. The *ark client* makes a call to the Kubernetes API server, creating a `Backup` custom resource (which is stored in [etcd][22]).

2. The `BackupController` sees that a new `Backup` has been created, and validates it.

3. Once validation passes, the `BackupController` begins the backup process. It collects data by querying the Kubernetes API Server for resources.

4. Once the data has been aggregated, the `BackupController` makes a call to the object storage service (e.g. Amazon S3) to upload the backup file.

5. If the `--snapshot-volumes` flag is specified, Ark also makes disk snapshots of any persistent volumes, using the appropriate cloud service API.

## Further documentation

 To learn more about Heptio Ark operations and their applications, see the [`/docs` directory][3].

## Troubleshooting

If you encounter any problems that the documentation does not address, [file an issue][4].  

## Contributing

Thanks for taking the time to join our community and start contributing!

#### Before you start

* Please familiarize yourself with the [Code of Conduct][8] before contributing.
* See [CONTRIBUTING.md][5] for instructions on the developer certificate of origin that we require.

#### Pull requests

* We welcome pull requests. Feel free to dig through the [issues][4] and jump in.


## Changelog

See [the list of releases][6] to find out about feature changes.

[0]: https://github.com/heptio
[1]: https://jenkins.i.heptio.com/buildStatus/icon?job=ark-prbuilder
[2]: https://jenkins.i.heptio.com/job/ark-prbuilder/
[3]: /
[4]: https://github.com/heptio/ark/issues
[5]: https://github.com/heptio/ark/tree/v0.3.0/CONTRIBUTING.md
[6]: https://github.com/heptio/ark/tree/v0.3.0/CHANGELOG.md
[7]: /build-from-scratch.md
[8]: https://github.com/heptio/ark/tree/v0.3.0/CODE_OF_CONDUCT.md
[9]: https://kubernetes.io/docs/setup/
[10]: https://kubernetes.io/docs/tasks/tools/install-kubectl/#install-with-homebrew-on-macos
[11]: https://kubernetes.io/docs/tasks/tools/install-kubectl/#tabset-1
[12]: https://github.com/kubernetes/kubernetes/blob/main/cluster/addons/dns/README.md
[13]: /output-file-format.md
[14]: https://github.com/kubernetes/kubernetes
[15]: https://aws.amazon.com/
[16]: https://cloud.google.com/
[17]: https://azure.microsoft.com/
[18]: /debugging-restores.md
[19]: /img/backup-process.png
[20]: https://kubernetes.io/docs/concepts/api-extension/custom-resources/#customresourcedefinitions
[21]: https://kubernetes.io/docs/concepts/api-extension/custom-resources/#custom-controllers
[22]: https://github.com/coreos/etcd
[23]: /cloud-provider-specifics.md
