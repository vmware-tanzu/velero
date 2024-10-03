---
title: "On-Premises Environments"
layout: docs
---

You can run Velero in an on-premises cluster in different ways depending on your requirements.

### Selecting an object storage provider

You must select an object storage backend that Velero can use to store backup data. [Supported providers][0] contains information on various
options that are supported or have been reported to work by users.

If you do not already have an object storage system, [MinIO][2] is an open-source S3-compatible object storage system that can be installed on-premises and is compatible with Velero. The details of configuring it for production usage are out of scope for Velero's documentation, but an [evaluation install guide][3] using MinIO is provided for convenience.

### (Optional) Selecting volume snapshot providers

If you need to back up persistent volume data, you must select a volume backup solution. [Supported providers][0] contains information on the supported options. 

For example, if you use [Portworx][4] for persistent storage, you can install their Velero plugin to get native Portworx snapshots as part of your Velero backups. 

If there is no native snapshot plugin available for your storage platform, you can use Velero's [restic integration][1], which provides a platform-agnostic file-level backup solution for volume data.

[0]: supported-providers.md
[1]: restic.md
[2]: https://min.io
[3]: contributions/minio.md
[4]: https://portworx.com
