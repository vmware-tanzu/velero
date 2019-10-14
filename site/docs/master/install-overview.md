# Install Overview

- [Prerequisites](#prerequisites)
- [Install the command-line interface (CLI)](#install-the-cli)
- [Install and configure the server components](#install-and-configure-the-server-components)
- [Advanced installation topics](#advanced-installation-topics)

## Prerequisites
- access to a Kubernetes cluster, v1.TODO or later, with DNS and container networking enabled.
- `kubectl` installed locally

Additionally, Velero uses object storage to store backups and associated artifacts. It also optionally integrates with supported block storage systems to snapshot your persistent volumes. Before beginning the installation process, you should identify the object storage provider and optional block storage provider(s) you'll be using from the list of [compatible providers][0]. 

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

The Velero installation can be customized to meet your needs. TODO

#### Run in any namespace

Velero runs in the `velero` namespace by default. However, you can run Velero in any namespace. See [run in custom namespace][2] for details.

#### Use non-file-based identity mechanisms

By default, `velero install` expects a credentials file for your `velero` IAM account to be provided via the `--secret-file` flag.

If you are using an alternate identity mechanism, such as kube2iam/kiam on AWS, Workload Identity on GKE, etc., that does not require a credentials file, you can specify the `--no-secret` flag instead of `--secret-file`.

#### Enable restic integration

By default, `velero install` does not install Velero's [restic integration][3]. To enable it, specify the `--use-restic` flag. 

If you've already run `velero install` without the `--use-restic` flag, you can run the same command again, including the `--use-restic` flag, to add the restic integration to your existing install. 

#### Configure more than one storage location for backups or volume snapshots

Velero supports any number of backup storage locations and volume snapshot locations. For more details, see [about locations](locations.md).

However, `velero install` only supports configuring at most one backup storage location and one volume snapshot location.

To configure additional locations after running `velero install`, use the `velero backup-location create` and/or `velero snapshot-location create` commands along with provider-specific configuration. Use the `--help` flag on each of these commands for more details.

#### Don't configure a backup storage location during install

If you need to install Velero without a default backup storage location (without specifying `--bucket` or `--provider`), the `--no-default-backup-location` flag is required for confirmation.

#### Install an additional volume snapshot provider

Velero supports using different providers for volume snapshots than for object storage -- for example, you can use AWS S3 for object storage, and Portworx for block volume snapshots.

However, `velero install` only supports configuring a single matching provider for both object storage and volume snapshots.

To use a different volume snapshot provider:

1. Install the Velero server components by following the instructions for your **object storage** provider

1. Add your volume snapshot provider's plugin to Velero (look in [your provider][0]'s documentation for the image name):

    ```bash
    velero plugin add <PLUGIN-IMAGE>
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

#### Providing credentials when installing with the Helm chart

TODO can this go in the Helm chart docs?

When installing using the Helm chart, the provider's credential information will need to be appended into your values.

The easiest way to do this is with the `--set-file` argument, available in Helm 2.10 and higher.

```bash
helm install --set-file credentials.secretContents.cloud=./credentials-velero stable/velero
```

See your cloud provider's documentation for the contents and creation of the `credentials-velero` file.

#### Additional options

Run `velero install --help` or see the [Helm chart documentation](https://github.com/helm/charts/tree/master/stable/velero) for the full set of installation options.


[0]: supported-providers.md
[1]: https://github.com/vmware-tanzu/velero/releases/latest
[2]: namespace.md
[3]: restic.md
[4]: on-premises.md
