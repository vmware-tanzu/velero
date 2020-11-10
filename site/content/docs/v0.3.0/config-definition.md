---
title: "Ark Config definition"
layout: docs
---

* [Overview][10]
* [Example][11]
* [Parameter Reference][8]
  * [Main config][9]
  * [AWS][0]
  * [GCP][1]
  * [Azure][2]

## Overview

Heptio Ark defines its own Config object (a custom resource) for specifying Ark backup and cloud provider settings. When the Ark server is first deployed, it waits until you create a Config--specifically one named `default`--in the `heptio-ark` namespace.

> *NOTE*: There is an underlying assumption that you're running the Ark server as a Kubernetes deployment. If the `default` Config is modified, the server shuts down gracefully. Once the kubelet restarts the Ark server pod, the server then uses the updated Config values.

## Example

A sample YAML `Config` looks like the following:
```
apiVersion: ark.heptio.com/v1
kind: Config
metadata:
  namespace: heptio-ark
  name: default
persistentVolumeProvider:
  aws:
    region: minio
    availabilityZone: minio
    s3ForcePathStyle: true
    s3Url: http://minio:9000
backupStorageProvider:
  bucket: ark
  aws:
    region: minio
    availabilityZone: minio
    s3ForcePathStyle: true
    s3Url: http://minio:9000
backupSyncPeriod: 60m
gcSyncPeriod: 60m
scheduleSyncPeriod: 1m
restoreOnlyMode: false
```

## Parameter Reference

The configurable parameters are as follows:

### Main config parameters

| Key | Type | Default | Meaning |
| --- | --- | --- | --- |
| `persistentVolumeProvider` | CloudProviderConfig<br><br>(Supported key values are `aws`, `gcp`, and `azure`, but only one can be present. See the corresponding [AWS][0], [GCP][1], and [Azure][2]-specific configs.) | Required Field | The specification for whichever cloud provider the cluster is using for persistent volumes (to be snapshotted).<br><br> *NOTE*: For Azure, your Kubernetes cluster needs to be version 1.7.2+ in order to support PV snapshotting of its managed disks. |
| `backupStorageProvider`/(inline) | CloudProviderConfig<br><br>(Supported key values are `aws`, `gcp`, and `azure`, but only one can be present. See the corresponding [AWS][0], [GCP][1], and [Azure][2]-specific configs.) | Required Field | The specification for whichever cloud provider will be used to actually store the backups. |
| `backupStorageProvider/bucket` | String | Required Field | The storage bucket where backups are to be uploaded. |
| `backupSyncPeriod` | metav1.Duration | 60m0s | How frequently Ark queries the object storage to make sure that the appropriate Backup resources have been created for existing backup files. |
| `gcSyncPeriod` | metav1.Duration | 60m0s | How frequently Ark queries the object storage to delete backup files that have passed their TTL. |
| `scheduleSyncPeriod` | metav1.Duration | 1m0s | How frequently Ark checks its Schedule resource objects to see if a backup needs to be initiated. |
| `resourcePriorities` | []string | `[namespaces, persistentvolumes, persistentvolumeclaims, secrets, configmaps]` | An ordered list that describes the order in which Kubernetes resource objects should be restored (also specified with the `<RESOURCE>.<GROUP>` format.<br><br>If a resource is not in this list, it is restored after all other prioritized resources. |
| `restoreOnlyMode` | bool | `false` | When RestoreOnly mode is on, functionality for backups, schedules, and expired backup deletion is *turned off*. Restores are made from existing backup files in object storage. |

### AWS

**(Or other S3-compatible storage)**

| Key | Type | Default | Meaning |
| --- | --- | --- | --- |
| `region` | string | Required Field | *Example*: "us-east-1"<br><br>See [AWS documentation][3] for the full list. |
| `availabilityZone` | string | Required Field | *Example*: "us-east-1a"<br><br>See [AWS documentation][4] for details. |
| `disableSSL` | bool | `false` | Set this to `true` if you are using Minio (or another local, S3-compatible storage service) and your deployment is not secured. |
| `s3ForcePathStyle` | bool | `false` | Set this to `true` if you are using a local storage service like Minio. |
| `s3Url` | string | Required field for non-AWS-hosted storage| *Example*: http://minio:9000<br><br>You can specify the AWS S3 URL here for explicitness, but Ark can already generate it from `region`, `availabilityZone`, and `bucket`. This field is primarily for local storage services like Minio.|

### GCP
| Key | Type | Default | Meaning |
| --- | --- | --- | --- |
| `project` | string | Required Field | *Example*: "project-example-3jsn23"<br><br> See the [Project ID documentation][5] for details. |
| `zone` | string | Required Field | *Example*: "us-central1-a"<br><br>See [GCP documentation][6] for the full list. |

### Azure
| Key | Type | Default | Meaning |
| --- | --- | --- | --- |
| `location` | string | Required Field | *Example*: "Canada East"<br><br>See [the list of available locations][7] (note that this particular page refers to them as "Regions"). |
| `apiTimeout` | metav1.Duration | 1m0s | How long to wait for an API Azure request to complete before timeout. |

[0]: #aws
[1]: #gcp
[2]: #azure
[3]: http://docs.aws.amazon.com/AWSEC2/latest/UserGuide/using-regions-availability-zones.html#concepts-available-regions
[4]: http://docs.aws.amazon.com/AWSEC2/latest/UserGuide/using-regions-availability-zones.html#concepts-availability-zones
[5]: https://cloud.google.com/resource-manager/docs/creating-managing-projects#identifying_projects
[6]: https://cloud.google.com/compute/docs/regions-zones/regions-zones
[7]: https://azure.microsoft.com/en-us/regions/
[8]: #parameter-reference
[9]: #main-config-parameters
[10]: #overview
[11]: #example
