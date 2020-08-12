# Heptio Ark

**Maintainers:** [Heptio][0]

[![Build Status][1]][2]

## Overview

Ark gives you tools to back up and restore your Kubernetes cluster resources and persistent volumes. Ark lets you:

* Take backups of your cluster and restore in case of loss.
* Copy cluster resources across cloud providers. NOTE: Cloud volume migrations are not yet supported.
* Replicate your production environment for development and testing environments.

Ark consists of:

* A server that runs on your cluster
* A command-line client that runs locally

## More information

[The documentation][29] provides detailed information about building from source, architecture, extending Ark, and more.

## Getting started

The following example sets up the Ark server and client, then backs up and restores a sample application.

For simplicity, the example uses Minio, an S3-compatible storage service that runs locally on your cluster. See [Set up Ark with your cloud provider][3] for how to run on a cloud provider. 

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
    kubectl apply -f examples/common/10-deployment.yaml
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

For this example, we recommend that you [download a pre-built release][26].

You can also [build from source][7].

Make sure that you install somewhere in your `$PATH`.

### Back up

1. Create a backup for any object that matches the `app=nginx` label selector:

    ```
    ark backup create nginx-backup --selector app=nginx
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
    ark restore create nginx-backup
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

After a successful restore, the `STATUS` column is `Completed`, and `WARNINGS` and `ERRORS` are 0. All objects in the `nginx-example` namespacee should be just as they were before you deleted them.

If there are errors or warnings, you can look at them in detail:

```
ark restore describe <RESTORE_NAME>
```

For more information, see [the debugging information][18].

### Clean up

Delete any backups you created:

```
kubectl delete -n heptio-ark backup --all
```

Before you continue, wait for the following to show no backups:

```
ark backup get
```

To remove the Kubernetes objects for this example from your cluster, run:

```
kubectl delete -f examples/common/
kubectl delete -f examples/minio/
kubectl delete -f examples/nginx-app/base.yaml
```

## Troubleshooting

If you encounter issues, review the [troubleshooting docs][30], [file an issue][4], or talk to us on the [Kubernetes Slack team][25] channel `#ark-dr`.

## Contributing

Thanks for taking the time to join our community and start contributing!

Feedback and discussion is available on [the mailing list][24].

#### Before you start

* Please familiarize yourself with the [Code of Conduct][8] before contributing.
* See [CONTRIBUTING.md][5] for instructions on the developer certificate of origin that we require.

#### Pull requests

* We welcome pull requests. Feel free to dig through the [issues][4] and jump in.

## Changelog

See [the list of releases][6] to find out about feature changes.

[0]: https://github.com/heptio
[1]: https://travis-ci.org/heptio/ark.svg?branch=main
[2]: https://travis-ci.org/heptio/ark
[3]: /cloud-common.md
[4]: https://github.com/heptio/ark/issues
[5]: https://github.com/heptio/ark/blob/main/CONTRIBUTING.md
[6]: https://github.com/heptio/ark/releases
[7]: /build-from-scratch.md
[8]: https://github.com/heptio/ark/blob/main/CODE_OF_CONDUCT.md
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
[24]: http://j.hept.io/ark-list
[25]: http://slack.kubernetes.io/
[26]: https://github.com/heptio/ark/releases
[27]: /hooks.md
[28]: /plugins.md
[29]: https://velero.io/docs/v0.7.1/
[30]: /troubleshooting.md
