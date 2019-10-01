# Compatible providers

Velero can store its backups in any S3 or S3 compatible object storage.

## S3-Compatible object store providers

Velero uses [Amazon's Go SDK][0] to connect to the AWS S3 API. Some third-party storage providers also support the S3 API, and users have reported the following providers work with Velero:

_Note that these storage providers are not regularly tested by the Velero team._

 * [IBM Cloud][1]
 * [Oracle Cloud][2]
 * [Minio][3]
 * [DigitalOcean][4]
 * [NooBaa][5]
 * Ceph RADOS v12.2.7
 * Quobyte

_Some storage providers, like Quobyte, may need a different [signature algorithm version][6]._

## Velero supported providers

| Provider                   | Object Store        | Volume Snapshotter           | Plugin                    |
|----------------------------|---------------------|------------------------------|---------------------------|
| [AWS S3][7]                | AWS S3              | AWS EBS                      | [Velero plugin AWS][8]    |
| [Azure Blob Storage][9]    | Azure Blob Storage  | Azure Managed Disks          | [Velero plugin Azure][10] |
| [Google Cloud Storage][11] | Google Cloud Storage| Google Compute Engine Disks  | [Velero plugin GCP][12]   |

Contact: [Slack][28], [GitHub Issue][29]

## Community supported providers

| Provider                  | Object Store                 | Volume Snapshotter                 | Plugin                 | Contact                         |
|---------------------------|------------------------------|------------------------------------|------------------------|---------------------------------|
| [Portworx][11]             | 🚫                          | Portworx Volume                    | [Portworx][12]         | [Slack][13], [GitHub Issue][14] |
| [DigitalOcean][15]         | DigitalOcean Object Storage | DigitalOcean Volumes Block Storage | [StackPointCloud][16]  |                                 |
| [OpenEBS][17]             | 🚫                           | OpenEBS CStor Volume               | [OpenEBS][18]          | [Slack][19], [GitHub Issue][20] |
| [AlibabaCloud][21]        | 🚫                           | Alibaba Cloud                      | [AlibabaCloud][22]     | [GitHub Issue][23]              |
| [Hewlett Packard][24]     | 🚫                           | HPE Storage                        | [Hewlett Packard][25]  | [Slack][26], [GitHub Issue][27] |

## Non-supported volume snapshots

In the case you want to take volume snapshots but didn't find a plugin for your provider, Velero has support for snapshotting using restic. Please see the [restic integration][30] documentation.

[0]: https://github.com/aws/aws-sdk-go/aws
[1]: contributions/ibm-config.md
[2]: contributions/oracle-config.md
[3]: contributions/minio.md
[4]: https://github.com/StackPointCloud/ark-plugin-digitalocean
[5]: http://www.noobaa.com/
[6]: api-types/backupstoragelocation.md#aws
[7]: https://aws.amazon.com/s3/
[8]: aws-config.md
[9]: https://azure.microsoft.com/en-us/services/storage/blobs
[10]: azure-config.md
[11]: https://cloud.google.com/storage/
[12]: gcp-config.md
[11]: https://portworx.com/
[12]: https://docs.portworx.com/scheduler/kubernetes/ark.html
[13]: https://portworx.slack.com/messages/px-k8s
[14]: https://github.com/portworx/ark-plugin/issues
[15]: https://www.digitalocean.com/
[16]: https://github.com/StackPointCloud/ark-plugin-digitalocean
[17]: https://openebs.io/
[18]: https://github.com/openebs/velero-plugin
[19]: https://openebs-community.slack.com/
[20]: https://github.com/openebs/velero-plugin/issues
[21]: https://www.alibabacloud.com/
[22]: https://github.com/AliyunContainerService/velero-plugin
[23]: https://github.com/AliyunContainerService/velero-plugin/issues
[24]: https://www.hpe.com/us/en/storage.html
[25]: https://github.com/hpe-storage/velero-plugin
[26]: https://slack.hpedev.io/
[27]: https://github.com/hpe-storage/velero-plugin/issues
[28]: https://kubernetes.slack.com/messages/velero
[29]: https://github.com/vmware-tanzu/velero/issues
[30]: restic.md
