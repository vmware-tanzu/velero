# Velero Backup Storage Locations

## Backup Storage Location

Velero can store backups in a number of locations. These are represented in the cluster via the `BackupStorageLocation` CRD.

Velero must have at least one `BackupStorageLocation`. By default, this is expected to be named `default`, however the name can be changed by specifying `--default-backup-storage-location` on `velero server`.  Backups that do not explicitly specify a storage location will be saved to this `BackupStorageLocation`.

A sample YAML `BackupStorageLocation` looks like the following:

```yaml
apiVersion: velero.io/v1
kind: BackupStorageLocation
metadata:
  name: default
  namespace: velero
spec:
  backupSyncPeriod: 2m0s
  provider: aws
  objectStorage:
    bucket: myBucket
  config:
    region: us-west-2
    profile: "default"
```

### Parameter Reference

The configurable parameters are as follows:

#### Main config parameters

| Key | Type | Default | Meaning |
| --- | --- | --- | --- |
| `provider` | String | Required Field | The name for whichever object storage provider will be used to store the backups. See [your object storage provider's plugin documentation][0] for the appropriate value to use. |
| `objectStorage` | ObjectStorageLocation | Required Field | Specification of the object storage for the given provider. |
| `objectStorage/bucket` | String | Required Field | The storage bucket where backups are to be uploaded. |
| `objectStorage/prefix` | String | Optional Field | The directory inside a storage bucket where backups are to be uploaded. |
| `objectStorage/caCert` | String | Optional Field | A base64 encoded CA bundle to be used when verifying TLS connections |
| `config` | map[string]string | None (Optional) | Provider-specific configuration keys/values to be passed to the object store plugin. See [your object storage provider's plugin documentation][0] for details. |
| `accessMode` | String | `ReadWrite` | How Velero can access the backup storage location. Valid values are `ReadWrite`, `ReadOnly`. |
| `backupSyncPeriod` | metav1.Duration | Optional Field | How frequently Velero should synchronize backups in object storage. Default is Velero's server backup sync period. Set this to `0s` to disable sync. |
| `validationFrequency` | metav1.Duration | Optional Field | How frequently Velero should validate the object storage . Default is Velero's server validation frequency. Set this to `0s` to disable validation. Default 1 minute. |


[0]: ../supported-providers.md
