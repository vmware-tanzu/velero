# Build From Scratch

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

### Using a locally built image

To ensure that your Ark deployment uses your local image rather than pulling from an image registry, you'll need to explicitly set the `spec.template.spec.containers[*].imagePullPolicy` in your deployment file to `IfNotPresent`. This file is either `examples/azure/00-ark-deployment.yaml` if you're using Azure, or `examples/common/10-deployment.yaml` for all other cloud providers.

### Using your own registry

If you want to push the Heptio Ark images to your own registry, set the `$REGISTRY` environment variable (used in the `Makefile`). This allows any node in your cluster to pull your locally built image.

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
