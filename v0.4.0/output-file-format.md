# Output file format

A backup is a gzip-compressed tar file whose name matches the Backup API resource's `metadata.name` (what is specified during `ark backup create <NAME>`).

In cloud object storage, *each backup file is stored in its own subdirectory* beneath the bucket specified in the Ark server configuration. This subdirectory includes an additional file called `ark-backup.json`. The JSON file explicitly lists all info about your associated Backup resource--including any default values used--so that you have a complete historical record of its configuration. It also specifies `status.version`, which corresponds to the output file format.

All together, the directory structure in your cloud storage may look like:

```
rootBucket/
    backup1234/
        ark-backup.json
        backup1234.tar.gz
```

## `ark-backup.json`
An example of this file looks like the following:
```
{
  "kind": "Backup",
  "apiVersion": "ark.heptio.com/v1",
  "metadata": {
    "name": "test-backup",
    "namespace": "heptio-ark",
    "selfLink": "/apis/ark.heptio.com/v1/namespaces/heptio-ark/backups/testtest",
    "uid": "a12345cb-75f5-11e7-b4c2-abcdef123456",
    "resourceVersion": "337075",
    "creationTimestamp": "2017-07-31T13:39:15Z"
  },
  "spec": {
    "includedNamespaces": [
      "*"
    ],
    "excludedNamespaces": null,
    "includedResources": [
      "*"
    ],
    "excludedResources": null,
    "labelSelector": null,
    "snapshotVolumes": true,
    "ttl": "24h0m0s"
  },
  "status": {
    "version": 1,
    "expiration": "2017-08-01T13:39:15Z",
    "phase": "Completed",
    "volumeBackups": {
      "pvc-e1e2d345-7583-11e7-b4c2-abcdef123456": {
        "snapshotID": "snap-04b1a8e11dfb33ab0",
        "type": "gp2",
        "iops": 100
      }
    },
    "validationErrors": null
  }
}
```
Note that this file includes detailed info about your volume snapshots in the `status.volumeBackups` field, which can be helpful if you want to manually check them in your cloud provider GUI.

## file format version: 1

When unzipped, a typical backup directory (e.g. `backup1234.tar.gz`) looks like the following:

```
cluster/
    persistentvolumes/
        pv01.json
        ...
namespaces/
    namespace1/
        configmaps/
            myconfigmap.json
            ...
        pods
            mypod.json
            ...
        jobs
            awesome-job.json
            ...
        deployments
            cool-deployment.json
            ...
        ...
    namespace2/
        ...
    ...
```
