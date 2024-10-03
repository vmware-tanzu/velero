---
title: "Upgrading to Velero 1.5"
layout: docs
---

## Prerequisites

- Velero [v1.5.2][9], [v1.5.1][8] or [v1.4.x][5] installed.

If you're not yet running at least Velero v1.4, see the following:

- [Upgrading to v1.1][1]
- [Upgrading to v1.2][2]
- [Upgrading to v1.3][3]
- [Upgrading to v1.4][4]

Before upgrading, check the [Velero compatibility matrix](https://github.com/vmware-tanzu/velero#velero-compatabilty-matrix) to make sure your version of Kubernetes is supported by the new version of Velero.

## Instructions

1. Install the Velero v1.5 command-line interface (CLI) by following the [instructions here][0].

    Verify that you've properly installed it by running:

    ```bash
    velero version --client-only
    ```

    You should see the following output:

    ```bash
    Client:
        Version: v1.5.3
        Git commit: <git SHA>
    ```

1. Update the Velero custom resource definitions (CRDs) to include schema changes across all CRDs that are at the core of the new features in this release:

    ```bash
    velero install --crds-only --dry-run -o yaml | kubectl apply -f -
    ```

    **NOTE:** If you are upgrading Velero in Kubernetes 1.14.x or earlier, you will need to use `kubectl apply`'s `--validate=false` option when applying the CRD configuration above. See [issue 2077][6] and [issue 2311][7] for more context.

1. Update the container image used by the Velero deployment and, optionally, the restic daemon set:

    ```bash
    kubectl set image deployment/velero \
        velero=velero/velero:v1.5.3 \
        --namespace velero

    # optional, if using the restic daemon set
    kubectl set image daemonset/restic \
        restic=velero/velero:v1.5.3 \
        --namespace velero
    ```

1. Confirm that the deployment is up and running with the correct version by running:

    ```bash
    velero version
    ```

    You should see the following output:

    ```bash
    Client:
        Version: v1.5.3
        Git commit: <git SHA>

    Server:
        Version: v1.5.3
    ```

[0]: basic-install.md#install-the-cli
[1]: https://velero.io/docs/v1.1.0/upgrade-to-1.1/
[2]: https://velero.io/docs/v1.2.0/upgrade-to-1.2/
[3]: https://velero.io/docs/v1.3.2/upgrade-to-1.3/
[4]: https://velero.io/docs/v1.4/upgrade-to-1.4/
[5]: https://github.com/vmware-tanzu/velero/releases/tag/v1.4.2
[6]: https://github.com/vmware-tanzu/velero/issues/2077
[7]: https://github.com/vmware-tanzu/velero/issues/2311
[8]: https://github.com/vmware-tanzu/velero/releases/tag/v1.5.1
[9]: https://github.com/vmware-tanzu/velero/releases/tag/v1.5.2
