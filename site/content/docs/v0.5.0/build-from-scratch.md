---
title: "Build From Scratch"
layout: docs
---

While the [README][0] pulls from the Heptio image registry, you can also build your own Heptio Ark container with the following steps:

* [0. Prerequisites][1]
* [1. Download][2]
* [2. Build][3]
* [3. Test][12]
* [4. Run][7]
* [5. Vendoring dependencies][10]

## 0. Prerequisites

In addition to the handling the prerequisites mentioned in the [Quickstart][4], you should have [Go][5] installed (minimum version 1.8).

## 1. Download

Install with go:
```
go get github.com/heptio/ark
```
The files are installed in `$GOPATH/src/github.com/heptio/ark`.

## 2. Build

You can build your Ark image locally on the machine where you run your cluster, or you can push it to a private registry. This section covers both workflows.

Set the `$REGISTRY` environment variable (used in the `Makefile`) if you want to push the Heptio Ark images to your own registry. This allows any node in your cluster to pull your locally built image.

`$PROJECT` and `$VERSION` environment variables are also specified in the `Makefile`, and can be similarly modified as desired.

Run the following in the Ark root directory to build your container with the tag `$REGISTRY/$PROJECT:$VERSION`:
```
make container
```

To push your image to a registry, use `make push`.

### Updating generated files

There are several files that are automatically generated based on the source code in the repository.
These include:
* The clientset
* Listers
* Shared informers
* Documentation

If you make any of the following changes, you will need to run `make update` to regenerate
automatically generated files:
* Add/edit/remove command line flags and/or their help text
* Add/edit/remove commands or subcommands
* Add new API types

### Cross compiling

By default, `make` will build an `ark` binary that runs on your host operating system and
architecture. If you want to build for another platform, you can do so with `make
build-<GOOS>-<GOARCH` - for example, to build for the Mac, you would do `make build-darwin-amd64`.
All binaries are placed in `_output/bin/<GOOS>/<GOARCH>`, e.g. `_output/bin/darwin/amd64/ark`.

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

### Considerations

When running Heptio Ark, you will need to account for the following (all of which are handled in the [`/examples`][6] manifests):
* Appropriate RBAC permissions in the cluster
  * *Read access* for all data from the source cluster and namespaces
  * *Write access* to the target cluster and namespaces
* Cloud provider credentials
  * *Read/write access* to volumes
  * *Read/write access* to object storage for backup data
* A [Config object][8] definition for the Ark server

See [Cloud Provider Specifics][9] for a more detailed guide.

### Specifying your image

Once your Ark deployment is up and running, **you need to replace the Heptio-provided Ark image with the specific one that you built.** You can do so with the following command:
```
kubectl set image deployment/ark ark=$REGISTRY/$PROJECT:$VERSION
```
where `$REGISTRY`, `$PROJECT`, and `$VERSION` match what you used in the [build step][3].

## 5. Vendoring dependencies
If you need to add or update the vendored dependencies, please see [Vendoring dependencies][11].

[0]: ../README.md
[1]: #0-prerequisites
[2]: #1-download
[3]: #2-build
[4]: ../README.md#quickstart
[5]: https://golang.org/doc/install
[6]: /examples
[7]: #4-run
[8]: reference.md#ark-config-definition
[9]: cloud-provider-specifics.md
[10]: #4-vendoring-dependencies
[11]: vendoring-dependencies.md
[12]: #3-test
