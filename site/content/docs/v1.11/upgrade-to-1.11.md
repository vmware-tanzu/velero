---
title: "Upgrading to Velero 1.11"
layout: docs
---

## Prerequisites

- Velero [v1.10.x][6] installed.

If you're not yet running at least Velero v1.6, see the following:

- [Upgrading to v1.5][1]
- [Upgrading to v1.6][2]
- [Upgrading to v1.7][3]
- [Upgrading to v1.8][4]
- [Upgrading to v1.9][5]
- [Upgrading to v1.10][6]

Before upgrading, check the [Velero compatibility matrix](https://github.com/vmware-tanzu/velero#velero-compatibility-matrix) to make sure your version of Kubernetes is supported by the new version of Velero.

## Instructions

**Caution:** From Velero v1.10, except for using restic to do file-system level backup and restore, kopia is also been integrated, so there would be a little bit of difference when upgrading to v1.10 from a version lower than v1.10.0.

1. Install the Velero v1.11 command-line interface (CLI) by following the [instructions here][0].

    Verify that you've properly installed it by running:

    ```bash
    velero version --client-only
    ```

    You should see the following output:

    ```bash
    Client:
        Version: v1.11.0
        Git commit: <git SHA>
    ```

1. Update the Velero custom resource definitions (CRDs) to include schema changes across all CRDs that are at the core of the new features in this release:

    ```bash
    velero install --crds-only --dry-run -o yaml | kubectl apply -f -
    ```

1. Update the container image used by the Velero deployment and, optionally, the restic daemon set:

    ```bash
    kubectl set image deployment/velero \
        velero=velero/velero:v1.11.0 \
        --namespace velero

    # optional, if using the restic daemon set
    kubectl set image daemonset/restic \
        restic=velero/velero:v1.11.0 \
        --namespace velero
    ```

1. Confirm that the deployment is up and running with the correct version by running:

    ```bash
    velero version
    ```

    You should see the following output:

    ```bash
    Client:
        Version: v1.11.0
        Git commit: <git SHA>

    Server:
        Version: v1.11.0
    ```
## Notes
If upgraded from v1.9.x, there still remains some resources left over in the cluster and never used in v1.10.x and later, which could be deleted through kubectl and it is based on your desire:

    - resticrepository CRD and related CRs
    - velero-restic-credentials secret in velero install namespace


[0]: basic-install.md#install-the-cli
[1]: https://velero.io/docs/v1.5/upgrade-to-1.5
[2]: https://velero.io/docs/v1.6/upgrade-to-1.6
[3]: https://velero.io/docs/v1.7/upgrade-to-1.7
[4]: https://velero.io/docs/v1.8/upgrade-to-1.8
[5]: https://velero.io/docs/v1.9/upgrade-to-1.9
[6]: https://velero.io/docs/v1.10/upgrade-to-1.10
