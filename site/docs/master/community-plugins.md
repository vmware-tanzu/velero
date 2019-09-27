# Compatible providers and community plugins

## S3-Compatible Backup Storage Providers

Velero uses [Amazon's Go SDK][12] to connect to the AWS S3 API. Some third-party storage providers also support the S3 API, and users have reported the following providers work with Velero:

_Note that these providers are not regularly tested by the Velero team._

 * [IBM Cloud][5]
 * [Minio][9]
 * Ceph RADOS v12.2.7
 * [DigitalOcean][7]
 * Quobyte
 * [NooBaa][16]
 * [Oracle Cloud][23]

_Some storage providers, like Quobyte, may need a different [signature algorithm version][15]._

## Plugins supported by the community

### Volume snapshot plugins

| Plugin                         | Owner           | Contact                         |
|----------------------------------|-----------------|---------------------------------|
| [Portworx][6]                    | Portworx        | [Slack][13], [GitHub Issue][14] |
| [DigitalOcean][7]                | StackPointCloud |                                 |
| [OpenEBS][18]                     | OpenEBS       | [Slack][19], [GitHub Issue][20] |
| [AlibabaCloud][21]                     | AlibabaCloud       |  [GitHub Issue][22] |
| [HPE][24]                        | HPE                | [Slack][25], [GitHub Issue][26] |

[5]: contributions/ibm-config
[6]: https://docs.portworx.com/scheduler/kubernetes/ark.html
[7]: https://github.com/StackPointCloud/ark-plugin-digitalocean
[9]: get-started.md
[12]: https://github.com/aws/aws-sdk-go/aws
[13]: https://portworx.slack.com/messages/px-k8s
[14]: https://github.com/portworx/ark-plugin/issues
[15]: api-types/backupstoragelocation.md#aws
[16]: http://www.noobaa.com/
[18]: https://github.com/openebs/velero-plugin
[19]: https://openebs-community.slack.com/
[20]: https://github.com/openebs/velero-plugin/issues
[21]: https://github.com/AliyunContainerService/velero-plugin
[22]: https://github.com/AliyunContainerService/velero-plugin/issues
[23]: contributions/oracle-config
[24]: https://github.com/hpe-storage/velero-plugin
[25]: https://slack.hpedev.io/
[26]: https://github.com/hpe-storage/velero-plugin/issues
