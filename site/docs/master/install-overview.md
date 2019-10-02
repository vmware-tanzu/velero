# Install overview

You can run Velero in clusters on a cloud provider or on-premises. For detailed information, see our list of [supported providers][0].

We strongly recommend that you use an [official release][1] of Velero. The tarballs for each release contain the
`velero` command-line client.

_The code and sample YAML files in the master branch of the Velero repository are under active development and are not guaranteed to be stable. Use them at your own risk!_

## Set up your platform

You can run Velero in any namespace, which requires additional customization. See [Run in custom namespace][3].

You can also use Velero's integration with restic, which requires additional setup. See [restic instructions][4].

## Cloud provider

The Velero client includes an `install` command to specify the settings for each supported cloud provider. For provider-specific instructions, see our list of [supported providers][0].

To see the YAML applied by the `velero install` command, use the `--dry-run -o yaml` arguments.

For more complex installation needs, use either the generated YAML, or the [Helm chart][7].

When using node-based IAM policies, `--secret-file` is not required, but `--no-secret` is required for confirmation.

## On-premises

You can run Velero in an on-premises cluster in different ways depending on your requirements.

First, you must select an object storage backend that Velero can use to store backup data. [compatible storage providers][0] contains information on various
options that are supported or have been reported to work by users. [Minio][5] is an option if you want to keep your backup data on-premises and you are
not using another storage platform that offers an S3-compatible object storage API.

Second, if you need to back up persistent volume data, you must select a volume backup solution. [volume snapshot providers][0] contains information on the supported options. For example, if you use [Portworx][6] for persistent storage, you can install their Velero plugin to get native Portworx snapshots as part of your Velero backups. If there is no native snapshot plugin available for your storage platform, you can use Velero's [restic integration][4], which provides a
platform-agnostic backup solution for volume data.

## Customize configuration

Whether you run Velero on a cloud provider or on-premises, if you have more than one volume snapshot location for a given volume provider, you can specify its default location for backups by setting a server flag in your Velero deployment YAML.

If you need to install Velero without a default backup storage location (without specifying `--bucket` or `--provider`), the `--no-default-backup-location` flag is required for confirmation.

For details, see the documentation topics for individual cloud providers.

## Installing with the Helm chart

When installing using the Helm chart, the provider's credential information will need to be appended into your values.

The easiest way to do this is with the `--set-file` argument, available in Helm 2.10 and higher.

```bash
helm install --set-file credentials.secretContents.cloud=./credentials-velero stable/velero
```

See your cloud provider's documentation for the contents and creation of the `credentials-velero` file.



[0]: supported-providers.md
[1]: https://github.com/vmware-tanzu/velero/releases
[3]: namespace.md
[4]: restic.md
[5]: contributions/minio.md
[6]: https://portworx.com
[7]: #installing-with-the-Helm-chart
