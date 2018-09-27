# Compatible Storage Providers

Ark supports a variety of storage providers for different backup and snapshot operations. As of version 0.6.0, a plugin system allows anyone to add compatibility for additional backup and volume storage platforms without modifying the Ark codebase.

## Backup Storage Providers

| Provider                  | Owner    | Contact                         |
|---------------------------|----------|---------------------------------|
| [AWS S3][2]               | Ark Team | [Slack][10], [GitHub Issue][11] |
| [Azure Blob Storage][3]   | Ark Team | [Slack][10], [GitHub Issue][11] |
| [Google Cloud Storage][4] | Ark Team | [Slack][10], [GitHub Issue][11] |

## S3-Compatible Backup Storage Providers

Ark uses [Amazon's Go SDK][12] to connect to the S3 API. Some third-party storage providers also support the S3 API, and users have reported the following providers work with Ark:

_Note that these providers are not regularly tested by the Ark team._

 * [IBM Cloud][5]
 * [Minio][9]
 * Ceph RADOS v12.2.7
 * [DigitalOcean][7]

## Volume Snapshot Providers

| Provider                         | Owner           | Contact                         |
|----------------------------------|-----------------|---------------------------------|
| [AWS EBS][2]                     | Ark Team        | [Slack][10], [GitHub Issue][11] |
| [Azure Managed Disks][3]         | Ark Team        | [Slack][10], [GitHub Issue][11] |
| [Google Compute Engine Disks][4] | Ark Team        | [Slack][10], [GitHub Issue][11] |
| [Restic][1]                      | Ark Team        | [Slack][10], [GitHub Issue][11] |
| [Portworx][6]                    | Portworx        | [Slack][13], [GitHub Issue][14] |
| [DigitalOcean][7]                | StackPointCloud |                                 |

### Adding a new plugin

To write a plugin for a new backup or volume storage system, take a look at the [example repo][8].

After you publish your plugin, open a PR that adds your plugin to the appropriate list.

[1]: restic.md
[2]: aws-config.md
[3]: azure-config.md
[4]: gcp-config.md
[5]: ibm-config.md
[6]: https://docs.portworx.com/scheduler/kubernetes/ark.html
[7]: https://github.com/StackPointCloud/ark-plugin-digitalocean
[8]: https://github.com/heptio/ark-plugin-example/
[9]: quickstart.md
[10]: https://kubernetes.slack.com/messages/ark-dr
[11]: https://github.com/heptio/ark/issues
[12]: https://github.com/aws/aws-sdk-go/aws
[13]: https://portworx.slack.com/messages/px-k8s
[14]: https://github.com/portworx/ark-plugin/issues
