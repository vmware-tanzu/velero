

## Prerequisites

* Access to a Kubernetes cluster, version 1.7 or later. Version 1.7.5 or later is required to run `ark backup delete`.
* A DNS server on the cluster
* kubectl installed
* [Go][5] installed (minimum version 1.8)

### Prerequisites 

When running Heptio Ark, you will need to account for the following (all of which are handled in the [`/examples`][6] manifests):

* Appropriate RBAC permissions in the cluster
  * Read access for all data from the source cluster and namespaces
  * Write access to the target cluster and namespaces
* Cloud provider credentials
  * Read/write access to volumes
  * Read/write access to object storage for backup data
* A [BackupStorageLocation][20] object definition for the Ark server
* (Optional) A [VolumeSnapshotLocation][21] object definition for the Ark server, to take PV snapshots

## Getting the source

```bash
mkdir $HOME/go
export GOPATH=$HOME/go
go get github.com/heptio/ark
```

Where `go` is your [import path][4] for Go.

For Go development, it is recommended to add the Go import path (`$HOME/go` in this example) to your path.

