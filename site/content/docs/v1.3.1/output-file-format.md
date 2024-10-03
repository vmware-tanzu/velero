---
title: "Output file format"
layout: docs
---

A backup is a gzip-compressed tar file whose name matches the Backup API resource's `metadata.name` (what is specified during `velero backup create <NAME>`).

In cloud object storage, each backup file is stored in its own subdirectory in the bucket specified in the Velero server configuration. This subdirectory includes an additional file called `velero-backup.json`. The JSON file lists all information about your associated Backup resource, including any default values. This gives you a complete historical record of the backup configuration. The JSON file also specifies `status.version`, which corresponds to the output file format.

The directory structure in your cloud storage looks something like:

```
rootBucket/
    backup1234/
        velero-backup.json
        backup1234.tar.gz
```

## Example backup JSON file

```json
{
  "kind": "Backup",
  "apiVersion": "velero.io/v1",
  "metadata": {
    "name": "test-backup",
    "namespace": "velero",
    "selfLink": "/apis/velero.io/v1/namespaces/velero/backups/test-backup",
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
resources/
    persistentvolumes/
        cluster/
            pv01.json
            ...
    configmaps/
        namespaces/
            namespace1/
                myconfigmap.json
                ...
            namespace2/
                ...
    pods/
        namespaces/
            namespace1/
                mypod.json
                ...
            namespace2/
                ...
    jobs/
        namespaces/
            namespace1/
                awesome-job.json
                ...
            namespace2/
                ...
    deployments/
        namespaces/
            namespace1/
                cool-deployment.json
                ...
            namespace2/
                ...
    ...
```
