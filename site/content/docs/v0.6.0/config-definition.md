# Ark Config definition

* [Overview][8]
* [Example][9]
* [Parameter Reference][6]
  * [Main config][7]
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
  name: aws
  config:
    region: us-west-2
backupStorageProvider:
  name: aws
  bucket: ark
  config:
    region: us-west-2
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
| `persistentVolumeProvider` | CloudProviderConfig | None (Optional) | The specification for whichever cloud provider the cluster is using for persistent volumes (to be snapshotted), if any.<br><br>If not specified, Backups and Restores requesting PV snapshots & restores, respectively, are considered invalid. <br><br> *NOTE*: For Azure, your Kubernetes cluster needs to be version 1.7.2+ in order to support PV snapshotting of its managed disks. |
| `persistentVolumeProvider/name` | String<br><br>(Ark natively supports `aws`, `gcp`, and `azure`. Other providers may be available via external plugins.) | None (Optional) | The name of the cloud provider the cluster is using for persistent volumes, if any. |
| `persistentVolumeProvider/config` | map[string]string<br><br>(See the corresponding [AWS][0], [GCP][1], and [Azure][2]-specific configs or your provider's documentation.) | None (Optional) | Configuration keys/values to be passed to the cloud provider for persistent volumes.  |
| `backupStorageProvider` | CloudProviderConfig | Required Field | The specification for whichever cloud provider will be used to actually store the backups. |
| `backupStorageProvider/name` | String<br><br>(Ark natively supports `aws`, `gcp`, and `azure`. Other providers may be available via external plugins.) | Required Field | The name of the cloud provider that will be used to actually store the backups. |
| `backupStorageProvider/bucket` | String | Required Field | The storage bucket where backups are to be uploaded. |
| `backupStorageProvider/config` | map[string]string<br><br>(See the corresponding [AWS][0], [GCP][1], and [Azure][2]-specific configs or your provider's documentation.) | None (Optional) | Configuration keys/values to be passed to the cloud provider for backup storage. |
| `backupSyncPeriod` | metav1.Duration | 60m0s | How frequently Ark queries the object storage to make sure that the appropriate Backup resources have been created for existing backup files. |
| `gcSyncPeriod` | metav1.Duration | 60m0s | How frequently Ark queries the object storage to delete backup files that have passed their TTL. |
| `scheduleSyncPeriod` | metav1.Duration | 1m0s | How frequently Ark checks its Schedule resource objects to see if a backup needs to be initiated. |
| `resourcePriorities` | []string | `[namespaces, persistentvolumes, persistentvolumeclaims, secrets, configmaps]` | An ordered list that describes the order in which Kubernetes resource objects should be restored (also specified with the `<RESOURCE>.<GROUP>` format.<br><br>If a resource is not in this list, it is restored after all other prioritized resources. |
| `restoreOnlyMode` | bool | `false` | When RestoreOnly mode is on, functionality for backups, schedules, and expired backup deletion is *turned off*. Restores are made from existing backup files in object storage. |

### AWS

**(Or other S3-compatible storage)**

#### backupStorageProvider/config

| Key | Type | Default | Meaning |
| --- | --- | --- | --- |
| `region` | string | Required Field | *Example*: "us-east-1"<br><br>See [AWS documentation][3] for the full list. |
| `s3ForcePathStyle` | bool | `false` | Set this to `true` if you are using a local storage service like Minio. |
| `s3Url` | string | Required field for non-AWS-hosted storage| *Example*: http://minio:9000<br><br>You can specify the AWS S3 URL here for explicitness, but Ark can already generate it from `region`, and `bucket`. This field is primarily for local storage services like Minio.|
| `kmsKeyId` | string | Empty | *Example*: "502b409c-4da1-419f-a16e-eif453b3i49f" or "alias/`<KMS-Key-Alias-Name>`"<br><br>Specify an [AWS KMS key][10] id or alias to enable encryption of the backups stored in S3. Only works with AWS S3 and may require explicitly granting key usage rights.|

#### persistentVolumeProvider/config (AWS Only)

| Key | Type | Default | Meaning |
| --- | --- | --- | --- |
| `region` | string | Required Field | *Example*: "us-east-1"<br><br>See [AWS documentation][3] for the full list. |

### GCP

#### backupStorageProvider/config

No parameters required.

#### persistentVolumeProvider/config

| Key | Type | Default | Meaning |
| --- | --- | --- | --- |
| `project` | string | Required Field | *Example*: "project-example-3jsn23"<br><br> See the [Project ID documentation][4] for details. |

### Azure

#### backupStorageProvider/config

No parameters required.

#### persistentVolumeProvider/config

| Key | Type | Default | Meaning |
| --- | --- | --- | --- |
| `location` | string | Required Field | *Example*: "Canada East"<br><br>See [the list of available locations][5] (note that this particular page refers to them as "Regions"). |
| `apiTimeout` | metav1.Duration | 2m0s | How long to wait for an Azure API request to complete before timeout. |

[0]: #aws
[1]: #gcp
[2]: #azure
[3]: http://docs.aws.amazon.com/AWSEC2/latest/UserGuide/using-regions-availability-zones.html#concepts-available-regions
[4]: https://cloud.google.com/resource-manager/docs/creating-managing-projects#identifying_projects
[5]: https://azure.microsoft.com/en-us/regions/
[6]: #parameter-reference
[7]: #main-config-parameters
[8]: #overview
[9]: #example
[10]: http://docs.aws.amazon.com/kms/latest/developerguide/overview.html
[11]: ../examples/gcp/00-ark-config.yaml
[12]: ../examples/azure/10-ark-config.yaml

