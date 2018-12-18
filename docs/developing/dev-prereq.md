# Set up

## Prerequisites

* Access to a Kubernetes cluster, version 1.7 or later. Version 1.7.5 or later is required to run `ark backup delete`.
* A DNS server on the cluster
* kubectl installed
* [Go][5] installed (minimum version 1.8)

Also make sure you have:

* Appropriate RBAC permissions in the cluster
  * Read access for all data from the source cluster and namespaces
  * Write access to the target cluster and namespaces
* Cloud provider credentials
  * Read/write access to volumes
  * Read/write access to object storage for backup data
* A [BackupStorageLocation][20] object definition for the Ark server
* (Optional) A [VolumeSnapshotLocation][21] object definition for the Ark server, to take PV snapshots

For detailed examples, see the YAML files in the [examples directory][6].

## Get source

```bash
mkdir $HOME/go
export GOPATH=$HOME/go
go get github.com/heptio/ark
```

Where `go` is your [import path][4] for Go.

For Go development, it's a good idea to add the Go import path (`$HOME/go` in this example) to your path.

## Extras

If you add, edit, or remove protobuf message or service definitions, you must run [generate-proto.sh][13] to regenerate files. These changes require the [proto compiler][14].

[4]: https://blog.golang.org/organizing-go-code
[5]: https://golang.org/doc/install
[6]: https://github.com/heptio/ark/tree/master/examples
[13]: https://github.com/heptio/ark/blob/master/hack/generate-proto.sh
[14]: https://grpc.io/docs/quickstart/go.html#install-protocol-buffers-v3

