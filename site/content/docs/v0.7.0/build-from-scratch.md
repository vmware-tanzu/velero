---
title: "Build from source"
layout: docs
---

* [Prerequisites][1]
* [Download][2]
* [Build][3]
* [Test][12]
* [Run][7]
* [Vendoring dependencies][10]

## Prerequisites

* Access to a Kubernetes cluster, version 1.7 or later. Version 1.7.5 or later is required to run `ark backup delete`.
* A DNS server on the cluster
* `kubectl` installed
* [Go][5] installed (minimum version 1.8)

## Download

Install with go:
```
go get github.com/heptio/ark
```
The files are installed in `$GOPATH/src/github.com/heptio/ark`.

## Build

You can build your Ark image locally on the machine where you run your cluster, or you can push it to a private registry. This section covers both workflows.

Set the `$REGISTRY` environment variable (used in the `Makefile`) to push the Heptio Ark images to your own registry. This allows any node in your cluster to pull your locally built image.

In the Ark root directory, to build your container with the tag `$REGISTRY/ark:$VERSION`, run:

```
make container
```

To push your image to a registry, use `make push`.

### Update generated files

The following files are automatically generated from the source code:

* The clientset
* Listers
* Shared informers
* Documentation
* Protobuf/gRPC types

If you make any of the following changes, you must run `make update` to regenerate
the files:

* Add/edit/remove command line flags and/or their help text
* Add/edit/remove commands or subcommands
* Add new API types

If you make the following change, you must run [generate-proto.sh][13] to regenerate files:

* Add/edit/remove protobuf message or service definitions. These changes require the [proto compiler][14]. 

### Cross compiling

By default, `make` builds an `ark` binary that runs on your host operating system and architecture. 
To build for another platform, run `make build-<GOOS>-<GOARCH`.
For example, to build for the Mac, run `make build-darwin-amd64`.
All binaries are placed in `_output/bin/<GOOS>/<GOARCH>`-- for example, `_output/bin/darwin/amd64/ark`.

Ark's `Makefile` has a convenience target, `all-build`, that builds the following platforms:
* linux-amd64
* linux-arm
* linux-arm64
* darwin-amd64
* windows-amd64

## 3. Test

To run unit tests, use `make test`. You can also run `make verify` to ensure that all generated
files (clientset, listers, shared informers, docs) are up to date.

## 4. Run

When running Heptio Ark, you will need to account for the following (all of which are handled in the [`/examples`][6] manifests):

* Appropriate RBAC permissions in the cluster
  * Read access for all data from the source cluster and namespaces
  * Write access to the target cluster and namespaces
* Cloud provider credentials
  * Read/write access to volumes
  * Read/write access to object storage for backup data
* A [Config object][8] definition for the Ark server

See [Cloud Provider Specifics][9] for more details.

### Specifying your image

When your Ark deployment is up and running, you must replace the Heptio-provided Ark image with the image that you built. Run:

```
kubectl set image deployment/ark ark=$REGISTRY/ark:$VERSION
```
where `$REGISTRY` and `$VERSION` are the values that you built with.

## 5. Vendoring dependencies

If you need to add or update the vendored dependencies, see [Vendoring dependencies][11].

[0]: ../README.md
[1]: #prerequisites
[2]: #download
[3]: #build
[4]: ../README.md#quickstart
[5]: https://golang.org/doc/install
[6]: https://github.com/heptio/ark/tree/master/examples
[7]: #run
[8]: /config-definition.md
[9]: /cloud-common.md
[10]: #vendoring-dependencies
[11]: /vendoring-dependencies.md
[12]: #test
[13]: https://github.com/heptio/ark/blob/master/hack/generate-proto.sh
[14]: https://grpc.io/docs/quickstart/go.html#install-protocol-buffers-v3