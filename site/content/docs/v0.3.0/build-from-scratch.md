---
title: "Build From Scratch"
layout: docs
---

While the [README][0] pulls from the Heptio image registry, you can also build your own Heptio Ark container with the following steps:

* [0. Prerequisites][1]
* [1. Download][2]
* [2. Build][3]
* [3. Run][7]

## 0. Prerequisites

In addition to the handling the prerequisites mentioned in the [Quickstart][4], you should have [Go][5] installed (minimum version 1.8).

## 1. Download

Install with go:
```
go get github.com/heptio/ark
```
The files are installed in `$GOPATH/src/github.com/heptio/ark`.

## 2. Build

Set the `$REGISTRY` environment variable (used in the `Makefile`) if you want to push the Heptio Ark images to your own registry. This allows any node in your cluster to pull your locally built image.

`$PROJECT` and `$VERSION` environment variables are also specified in the `Makefile`, and can be similarly modified as desired.

Run the following in the Ark root directory to build your container with the tag `$REGISTRY/$PROJECT:$VERSION`:
```
sudo make all
```

To push your image to a registry, use `make push`.

## 3. Run
When running Heptio Ark, you will need to account for the following (all of which are handled in the [`/examples`][6] manifests):
* Appropriate RBAC permissions in the cluster
  * *Read access* for all data from the source cluster and namespaces
  * *Write access* to the target cluster and namespaces
* Cloud provider credentials
  * *Read/write access* to volumes
  * *Read/write access* to object storage for backup data
* A [Config object][8] definition for the Ark server

See [Cloud Provider Specifics][9] for a more detailed guide.

[0]: ../README.md
[1]: #0-prerequisites
[2]: #1-download
[3]: #2-build
[4]: ../README.md#quickstart
[5]: https://golang.org/doc/install
[6]: /examples
[7]: #3-run
[8]: reference.md#ark-config-definition
[9]: cloud-provider-specifics.md
