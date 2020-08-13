---
title: "Upgrading to Velero 1.4"
layout: docs
---

## Prerequisites

- Velero [v1.3.x][4] installed.

If you're not yet running at least Velero v1.3, see the following:

- [Upgrading to v1.1][1]
- [Upgrading to v1.2][2]
- [Upgrading to v1.3][3]

## Instructions

1. Install the Velero v1.4 command-line interface (CLI) by following the [instructions here][0].

    Verify that you've properly installed it by running:

    ```bash
    velero version --client-only
    ```

    You should see the following output:

    ```bash
    Client:
        Version: v1.4.0
        Git commit: <git SHA>
    ```

1. Update the container image used by the Velero deployment and, optionally, the restic daemon set:

    ```bash
    kubectl set image deployment/velero \
        velero=velero/velero:v1.4.0 \
        --namespace velero

    # optional, if using the restic daemon set
    kubectl set image daemonset/restic \
        restic=velero/velero:v1.4.0 \
        --namespace velero
    ```

1. Update the Velero custom resource definitions (CRDs) to include the new backup progress fields:

    ```bash
    velero install --crds-only --dry-run -o yaml | kubectl apply -f -
    ```

1. Confirm that the deployment is up and running with the correct version by running:

    ```bash
    velero version
    ```

    You should see the following output:

    ```bash
    Client:
        Version: v1.4.0
        Git commit: <git SHA>

    Server:
        Version: v1.4.0
    ```

[0]: basic-install.md#install-the-cli
[1]: https://velero.io/docs/v1.1.0/upgrade-to-1.1/
[2]: https://velero.io/docs/v1.2.0/upgrade-to-1.2/
[3]: https://velero.io/docs/v1.3.2/upgrade-to-1.3/
[4]: https://github.com/vmware-tanzu/velero/releases/tag/v1.3.2
