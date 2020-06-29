# Ark Backup Storage Locations

## Backup Storage Location

Ark can store backups in a number of locations. These are represented in the cluster via the `BackupStorageLocation` CRD.

Ark must have at least one `BackupStorageLocation`. By default, this is expected to be named `default`, however the name can be changed by specifying `--default-backup-storage-location` on `ark server`.  Backups that do not explicitly specify a storage location will be saved to this `BackupStorageLocation`.

> *NOTE*: `BackupStorageLocation` takes the place of the `Config.backupStorageProvider` key as of v0.10.0

A sample YAML `BackupStorageLocation` looks like the following:

```yaml
apiVersion: ark.heptio.com/v1
kind: BackupStorageLocation
metadata:
  name: default
  namespace: heptio-ark
spec:
  provider: aws
  objectStorage:
    bucket: myBucket
    config:
      region: us-west-2
```

### Parameter Reference

The configurable parameters are as follows:

#### Main config parameters

| Key | Type | Default | Meaning |
| --- | --- | --- | --- |
| `provider` | String (Ark natively supports `aws`, `gcp`, and `azure`. Other providers may be available via external plugins.)| Required Field | The name for whichever cloud provider will be used to actually store the backups. |
| `objectStorage` | ObjectStorageLocation | Specification of the object storage for the given provider. |
| `objectStorage/bucket` | String | Required Field | The storage bucket where backups are to be uploaded. |
| `objectStorage/prefix` | String | Optional Field | The directory inside a storage bucket where backups are to be uploaded. |
| `objectStorage/config` | map[string]string<br><br>(See the corresponding [AWS][0], [GCP][1], and [Azure][2]-specific configs or your provider's documentation.) | None (Optional) | Configuration keys/values to be passed to the cloud provider for backup storage. |

#### AWS

**(Or other S3-compatible storage)**

##### objectStorage/config

| Key | Type | Default | Meaning |
| --- | --- | --- | --- |
| `region` | string | Empty | *Example*: "us-east-1"<br><br>See [AWS documentation][3] for the full list.<br><br>Queried from the AWS S3 API if not provided. |
| `s3ForcePathStyle` | bool | `false` | Set this to `true` if you are using a local storage service like Minio. |
| `s3Url` | string | Required field for non-AWS-hosted storage| *Example*: http://minio:9000<br><br>You can specify the AWS S3 URL here for explicitness, but Ark can already generate it from `region`, and `bucket`. This field is primarily for local storage services like Minio.|
| `publicUrl` | string | Empty | *Example*: https://minio.mycluster.com<br><br>If specified, use this instead of `s3Url` when generating download URLs (e.g., for logs). This field is primarily for local storage services like Minio.|
| `kmsKeyId` | string | Empty | *Example*: "502b409c-4da1-419f-a16e-eif453b3i49f" or "alias/`<KMS-Key-Alias-Name>`"<br><br>Specify an [AWS KMS key][10] id or alias to enable encryption of the backups stored in S3. Only works with AWS S3 and may require explicitly granting key usage rights.|

#### Azure

##### objectStorage/config

| Key | Type | Default | Meaning |
| --- | --- | --- | --- |
| `resourceGroup` | string | Required Field | Name of the resource group containing the storage account for this backup storage location. |
| `storageAccount` | string | Required Field | Name of the storage account for this backup storage location. |

#### GCP

No parameters required.

[0]: #aws
[1]: #gcp
[2]: #azure
[3]: http://docs.aws.amazon.com/AWSEC2/latest/UserGuide/using-regions-availability-zones.html#concepts-available-regions
