# Run in custom namespace

In Ark version 0.7.0 and later, you can run Ark in any namespace. To do so, you specify the
namespace in the YAML files that configure the Ark server. You then also specify the namespace when
you run Ark client commands.

## Edit the example files

The Ark release tarballs include a set of example configs that you can use to set up your Ark server. The
examples place the server and backup/schedule/restore/etc. data in the `heptio-ark` namespace.

To run the server in another namespace, you edit the relevant files, changing `heptio-ark` to
your desired namespace.

To store your backups, schedules, restores, and config in another namespace, you edit the relevant
files, changing `heptio-ark` to your desired namespace. You also need to create the
`cloud-credentials` secret in your desired namespace.

First, ensure you've [downloaded & extracted the latest release][0].

For all cloud providers, edit `config/common/00-prereqs.yaml`. This file defines:

* CustomResourceDefinitions for the Ark objects (backups, schedules, restores, downloadrequests, etc.)
* The namespace where the Ark server runs
* The namespace where backups, schedules, restores, etc. are stored
* The Ark service account
* The RBAC rules to grant permissions to the Ark service account


### AWS

For AWS, edit:

* `config/aws/05-ark-backupstoragelocation.yaml`
* `config/aws/06-ark-volumesnapshotlocation.yaml`
* `config/aws/10-deployment.yaml`


### Azure

For Azure, edit:

* `config/azure/00-ark-deployment.yaml`
* `config/azure/05-ark-backupstoragelocation.yaml`
* `config/azure/06-ark-volumesnapshotlocation.yaml`

### GCP

For GCP, edit:

* `config/gcp/05-ark-backupstoragelocation.yaml`
* `config/gcp/06-ark-volumesnapshotlocation.yaml`
* `config/gcp/10-deployment.yaml`


### IBM

For IBM, edit:

* `config/ibm/05-ark-backupstoragelocation.yaml`
* `config/ibm/10-deployment.yaml`


## Specify the namespace in client commands

To specify the namespace for all Ark client commands, run:

```
ark client config set namespace=<NAMESPACE_VALUE>
```



[0]: get-started.md#download
