---
title: "Velero Volume Snapshot Location"
layout: docs
---

## Volume Snapshot Location

A volume snapshot location is the location in which to store the volume snapshots created for a backup.

Velero can be configured to take snapshots of volumes from multiple providers. Velero also allows you to configure multiple possible `VolumeSnapshotLocation` per provider, although you can only select one location per provider at backup time.

Each VolumeSnapshotLocation describes a provider + location. These are represented in the cluster via the `VolumeSnapshotLocation` CRD. Velero must have at least one `VolumeSnapshotLocation` per cloud provider.

A sample YAML `VolumeSnapshotLocation` looks like the following:

```yaml
apiVersion: velero.io/v1
kind: VolumeSnapshotLocation
metadata:
  name: aws-default
  namespace: velero
spec:
  provider: aws
  config:
    region: us-west-2
    profile: "default"
```

### Parameter Reference

The configurable parameters are as follows:

#### Main config parameters

| Key | Type | Default | Meaning |
| --- | --- | --- | --- |
| `provider` | String | Required Field | The name for whichever storage provider will be used to create/store the volume snapshots. See [your volume snapshot provider's plugin documentation][0] for the appropriate value to use. |
| `config` | map[string]string | None (Optional) |  Provider-specific configuration keys/values to be passed to the volume snapshotter plugin. See [your volume snapshot provider's plugin documentation][0] for details. |


[0]: ../supported-providers.md
