---
title: "Run in custom namespace"
layout: docs
---

In Velero version 0.7.0 and later, you can run Velero in any namespace. To do so, you specify the
namespace in the YAML files that configure the Velero server. You then also specify the namespace when
you run Velero client commands.

## Edit the example files

The Velero release tarballs include a set of example configs that you can use to set up your Velero server. The
examples place the server and backup/schedule/restore/etc. data in the `velero` namespace.

To run the server in another namespace, you edit the relevant files, changing `velero` to
your desired namespace.

To store your backups, schedules, restores, and config in another namespace, you edit the relevant
files, changing `velero` to your desired namespace. You also need to create the
`cloud-credentials` secret in your desired namespace.

First, ensure you've [downloaded & extracted the latest release][0].

For all cloud providers, edit `config/common/00-prereqs.yaml`. This file defines:

* CustomResourceDefinitions for the Velero objects (backups, schedules, restores, downloadrequests, etc.)
* The namespace where the Velero server runs
* The namespace where backups, schedules, restores, etc. are stored
* The Velero service account
* The RBAC rules to grant permissions to the Velero service account


### AWS

For AWS, edit:

* `config/aws/05-backupstoragelocation.yaml`
* `config/aws/06-volumesnapshotlocation.yaml`
* `config/aws/10-deployment.yaml`


### Azure

For Azure, edit:

* `config/azure/00-deployment.yaml`
* `config/azure/05-backupstoragelocation.yaml`
* `config/azure/06-volumesnapshotlocation.yaml`

### GCP

For GCP, edit:

* `config/gcp/05-backupstoragelocation.yaml`
* `config/gcp/06-volumesnapshotlocation.yaml`
* `config/gcp/10-deployment.yaml`


### IBM

For IBM, edit:

* `config/ibm/05-backupstoragelocation.yaml`
* `config/ibm/10-deployment.yaml`


## Specify the namespace in client commands

To specify the namespace for all Velero client commands, run:

```
velero client config set namespace=<NAMESPACE_VALUE>
```



[0]: get-started.md#download
