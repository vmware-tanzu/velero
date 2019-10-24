# Install Overview

- [Prerequisites](#prerequisites)
- [Install the command-line interface (CLI)](#install-the-cli)
- [Install and configure the server components](#install-and-configure-the-server-components)
- [Advanced installation topics](#advanced-installation-topics)

## Prerequisites
- access to a Kubernetes cluster, v1.10 or later, with DNS and container networking enabled.
- `kubectl` installed locally

Velero uses object storage to store backups and associated artifacts. It also optionally integrates with supported block storage systems to snapshot your persistent volumes. Before beginning the installation process, you should identify the object storage provider and optional block storage provider(s) you'll be using from the list of [compatible providers][0]. 

There are supported storage providers for both cloud-provider environments and on-premises environments. For more details on on-premises scenarios, see the [on-premises documentation][4].

## Install the CLI

#### Option 1: macOS - Homebrew

On macOS, you can use [Homebrew](https://brew.sh) to install the `velero` client:

```bash
brew install velero
```

#### Option 2: GitHub release

1. Download the [latest release][1]'s tarball for your client platform.
1. Extract the tarball:
   
   ```bash
   tar -xvf <RELEASE-TARBALL-NAME>.tar.gz
   ```
1. Move the extracted `velero` binary to somewhere in your `$PATH` (e.g. `/usr/local/bin` for most users).

## Install and configure the server components

There are two supported methods for installing the Velero server components:

- the `velero install` CLI command
- the [Helm chart](https://github.com/helm/charts/tree/master/stable/velero)

To install and configure the Velero server components, follow the provider-specific instructions documented by [your storage provider][0].

_Note: if your object storage provider is different than your volume snapshot provider, follow the installation instructions for your object storage provider first, then return here and follow the instructions to [add your volume snapshot provider](#install-an-additional-volume-snapshot-provider)._

## Advanced installation topics

- [Plugins](#plugins)
- [Install in any namespace](#install-in-any-namespace)
- [Use non-file-based identity mechanisms](#use-non-file-based-identity-mechanisms)
- [Enable restic integration](#enable-restic-integration)
- [Customize resource requests and limits](#customize-resource-requests-and-limits)
- [Configure more than one storage location for backups or volume snapshots](#configure-more-than-one-storage-location-for-backups-or-volume-snapshots)
- [Do not configure a backup storage location during install](#do-not-configure-a-backup-storage-location-during-install)
- [Install an additional volume snapshot provider](#install-an-additional-volume-snapshot-provider)
- [Generate YAML only](#generate-yaml-only)
- [Additional options](#additional-options)

#### Plugins

During install, Velero requires that at least one plugin is added (with the `--plugins` flag). Please see the documentation under [Plugins](overview-plugins.md)

#### Install in any namespace

Velero is installed in the `velero` namespace by default. However, you can install Velero in any namespace. See [run in custom namespace][2] for details.

#### Use non-file-based identity mechanisms

By default, `velero install` expects a credentials file for your `velero` IAM account to be provided via the `--secret-file` flag.

If you are using an alternate identity mechanism, such as kube2iam/kiam on AWS, Workload Identity on GKE, etc., that does not require a credentials file, you can specify the `--no-secret` flag instead of `--secret-file`.

#### Enable restic integration

By default, `velero install` does not install Velero's [restic integration][3]. To enable it, specify the `--use-restic` flag. 

If you've already run `velero install` without the `--use-restic` flag, you can run the same command again, including the `--use-restic` flag, to add the restic integration to your existing install. 

#### Customize resource requests and limits

By default, the Velero deployment requests 500m CPU, 128Mi memory and sets a limit of 1000m CPU, 256Mi.
Default requests and limits are not set for the restic pods as CPU/Memory usage can depend heavily on the size of volumes being backed up.

If you need to customize these resource requests and limits, you can set the following flags in your `velero install` command:

```bash
velero install \
    --provider <YOUR_PROVIDER> \
    --bucket <YOUR_BUCKET> \
    --secret-file <PATH_TO_FILE> \
    --velero-pod-cpu-request <CPU_REQUEST> \
    --velero-pod-mem-request <MEMORY_REQUEST> \
    --velero-pod-cpu-limit <CPU_LIMIT> \
    --velero-pod-mem-limit <MEMORY_LIMIT> \
    [--use-restic] \
    [--restic-pod-cpu-request <CPU_REQUEST>] \
    [--restic-pod-mem-request <MEMORY_REQUEST>] \
    [--restic-pod-cpu-limit <CPU_LIMIT>] \
    [--restic-pod-mem-limit <MEMORY_LIMIT>]
```

Values for these flags follow the same format as [Kubernetes resource requirements][5].

#### Configure more than one storage location for backups or volume snapshots

Velero supports any number of backup storage locations and volume snapshot locations. For more details, see [about locations](locations.md).

However, `velero install` only supports configuring at most one backup storage location and one volume snapshot location.

To configure additional locations after running `velero install`, use the `velero backup-location create` and/or `velero snapshot-location create` commands along with provider-specific configuration. Use the `--help` flag on each of these commands for more details.

#### Do not configure a backup storage location during install

If you need to install Velero without a default backup storage location (without specifying `--bucket` or `--provider`), the `--no-default-backup-location` flag is required for confirmation.

#### Install an additional volume snapshot provider

Velero supports using different providers for volume snapshots than for object storage -- for example, you can use AWS S3 for object storage, and Portworx for block volume snapshots.

However, `velero install` only supports configuring a single matching provider for both object storage and volume snapshots.

To use a different volume snapshot provider:

1. Install the Velero server components by following the instructions for your **object storage** provider

1. Add your volume snapshot provider's plugin to Velero (look in [your provider][0]'s documentation for the image name):

    ```bash
    velero plugin add <registry/image:version>
    ```

1. Add a volume snapshot location for your provider, following [your provider][0]'s documentation for configuration:

    ```bash
    velero snapshot-location create <NAME> \
        --provider <PROVIDER-NAME> \
        [--config <PROVIDER-CONFIG>]
    ```

#### Generate YAML only

By default, `velero install` generates and applies a customized set of Kubernetes configuration (YAML) to your cluster.

To generate the YAML without applying it to your cluster, use the `--dry-run -o yaml` flags.

This is useful for applying bespoke customizations, integrating with a GitOps workflow, etc.

#### Additional options

Run `velero install --help` or see the [Helm chart documentation](https://github.com/helm/charts/tree/master/stable/velero) for the full set of installation options.


[0]: supported-providers.md
[1]: https://github.com/vmware-tanzu/velero/releases/latest
[2]: namespace.md
[3]: restic.md
[4]: on-premises.md
[5]: https://kubernetes.io/docs/concepts/configuration/manage-compute-resources-container/#meaning-of-cpu
