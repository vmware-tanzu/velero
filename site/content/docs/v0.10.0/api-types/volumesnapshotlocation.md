---
title: "Ark Volume Snapshot Location"
layout: docs
---

## Volume Snapshot Location

A volume snapshot location is the location in which to store the volume snapshots created for a backup.

Ark can be configured to take snapshots of volumes from multiple providers. Ark also allows you to configure multiple possible `VolumeSnapshotLocation` per provider, although you can only select one location per provider at backup time.

Each VolumeSnapshotLocation describes a provider + location. These are represented in the cluster via the `VolumeSnapshotLocation` CRD. Ark must have at least one `VolumeSnapshotLocation` per cloud provider.

A sample YAML `VolumeSnapshotLocation` looks like the following:

```yaml
apiVersion: ark.heptio.com/v1
kind: VolumeSnapshotLocation
metadata:
  name: aws-default
  namespace: heptio-ark
spec:
  provider: aws
  config:
    region: us-west-2
```

### Parameter Reference

The configurable parameters are as follows:

#### Main config parameters

| Key | Type | Default | Meaning |
| --- | --- | --- | --- |
| `provider` | String (Ark natively supports `aws`, `gcp`, and `azure`. Other providers may be available via external plugins.)| Required Field | The name for whichever cloud provider will be used to actually store the volume. |
| `config` | See the corresponding [AWS][0], [GCP][1], and [Azure][2]-specific configs or your provider's documentation.

#### AWS

##### config

| Key | Type | Default | Meaning |
| --- | --- | --- | --- |
| `region` | string | Empty | *Example*: "us-east-1"<br><br>See [AWS documentation][3] for the full list.<br><br>Queried from the AWS S3 API if not provided. |

#### Azure

##### config

| Key | Type | Default | Meaning |
| --- | --- | --- | --- |
| `apiTimeout` | metav1.Duration | 2m0s | How long to wait for an Azure API request to complete before timeout. |
| `resourceGroup` | string | Optional | The name of the resource group where volume snapshots should be stored, if different from the cluster's resource group. |

#### GCP

No parameters required.

[0]: #aws
[1]: #gcp
[2]: #azure
[3]: http://docs.aws.amazon.com/AWSEC2/latest/UserGuide/using-regions-availability-zones.html#concepts-available-regions
