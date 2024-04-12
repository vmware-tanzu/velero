---
title: "Upgrading to Velero 1.14"
layout: docs
---

## Prerequisites

- Velero [v1.13.x][5] installed.

If you're not yet running at least Velero v1.8, see the following:

- [Upgrading to v1.8][1]
- [Upgrading to v1.9][2]
- [Upgrading to v1.10][3]
- [Upgrading to v1.11][4]
- [Upgrading to v1.12][5]
- [Upgrading to v1.13][6]

Before upgrading, check the [Velero compatibility matrix](https://github.com/vmware-tanzu/velero#velero-compatibility-matrix) to make sure your version of Kubernetes is supported by the new version of Velero.

## Instructions

**Caution:** Starting in Velero v1.10, kopia has replaced restic as the default uploader. It is now possible to upgrade from a version >= v1.10 directly. However, the procedure for upgrading to v1.13 from a Velero release lower than v1.10 is different.

### Upgrade from v1.10 or higher
1. Install the Velero v1.13 command-line interface (CLI) by following the [instructions here][0].

    Verify that you've properly installed it by running:

    ```bash
    velero version --client-only
    ```

    You should see the following output:

    ```bash
    Client:
        Version: v1.14.0
        Git commit: <git SHA>
    ```

2. Update the Velero custom resource definitions (CRDs) to include schema changes across all CRDs that are at the core of the new features in this release:

    ```bash
    velero install --crds-only --dry-run -o yaml | kubectl apply -f -
    ```

    **NOTE:** Since velero v1.10.0 only v1 CRD will be supported during installation, therefore, the v1.10.0 will only work on Kubernetes version >= v1.16

3. Delete the CSI plugin. Because the Velero CSI plugin is already merged into the Velero, need to remove the existing CSI plugin InitContainer. Otherwise, the Velero server plugin would fail to start due to same plugin registered twice.
Please find more information of CSI plugin merging in this page [csi].
If the plugin move CLI fails due to `not found`, that is caused by the Velero CSI plugin not installed before upgrade. It's safe to ignore the error.
   
   ``` bash
   velero plugin remove velero-velero-plugin-for-csi; echo 0
   ```

4. Update the container image used by the Velero deployment, plugin and (optionally) the node agent daemon set:
    ```bash
   # set the container and image of the init container for plugin accordingly,
   # if you are using other plugin
    kubectl set image deployment/velero \
        velero=velero/velero:v1.14.0 \
        velero-plugin-for-aws=velero/velero-plugin-for-aws:v1.10.0 \
        --namespace velero

    # optional, if using the node agent daemonset
    kubectl set image daemonset/node-agent \
        node-agent=velero/velero:v1.14.0 \
        --namespace velero
    ```
5. Confirm that the deployment is up and running with the correct version by running:

    ```bash
    velero version
    ```

    You should see the following output:

    ```bash
    Client:
        Version: v1.14.0
        Git commit: <git SHA>

    Server:
        Version: v1.14.0
    ```


### Upgrade from version lower than v1.10.0
The procedure for upgrading from a version lower than v1.10.0 is identical to the procedure above, except for step 4 as shown below.

1. Update the container image and objects fields used by the Velero deployment and, optionally, the restic daemon set:

    ```bash
    # uploader_type value could be restic or kopia
    kubectl get deploy -n velero -ojson \
    | sed "s#\"image\"\: \"velero\/velero\:v[0-9]*.[0-9]*.[0-9]\"#\"image\"\: \"velero\/velero\:v1.14.0\"#g" \
    | sed "s#\"server\",#\"server\",\"--uploader-type=$uploader_type\",#g" \
    | sed "s#default-volumes-to-restic#default-volumes-to-fs-backup#g" \
    | sed "s#default-restic-prune-frequency#default-repo-maintain-frequency#g" \
    | sed "s#restic-timeout#fs-backup-timeout#g" \
    | kubectl apply -f -

    # optional, if using the restic daemon set
    echo $(kubectl get ds -n velero restic -ojson) \
    | sed "s#\"image\"\: \"velero\/velero\:v[0-9]*.[0-9]*.[0-9]\"#\"image\"\: \"velero\/velero\:v1.14.0\"#g" \
    | sed "s#\"name\"\: \"restic\"#\"name\"\: \"node-agent\"#g" \
    | sed "s#\[ \"restic\",#\[ \"node-agent\",#g" \
    | kubectl apply -f -
    kubectl delete ds -n velero restic --force --grace-period 0
    ```

## Notes
If upgrading from Velero v1.9.x or lower, there will likely remain some unused resources leftover in the cluster.These can be deleted manually (e.g. using kubectl) at your own discretion:

    - resticrepository CRD and related CRs
    - velero-restic-credentials secret in velero install namespace


[0]: basic-install.md#install-the-cli
[1]: https://velero.io/docs/v1.8/upgrade-to-1.8
[2]: https://velero.io/docs/v1.9/upgrade-to-1.9
[3]: https://velero.io/docs/v1.10/upgrade-to-1.10
[4]: https://velero.io/docs/v1.11/upgrade-to-1.11
[5]: https://velero.io/docs/v1.12/upgrade-to-1.12
[6]: https://velero.io/docs/v1.13/upgrade-to-1.13
