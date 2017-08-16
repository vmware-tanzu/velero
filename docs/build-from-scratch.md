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

### Using your own registry

If you want to push the Heptio Ark images to your own registry, set the `$REGISTRY` environment variable (used in the `Makefile`). This allows any node in your cluster to pull your locally built image.

`$PROJECT` and `$VERSION` environment variables are also specified in the `Makefile`, and can be similarly modified as desired.

Run the following in the Ark root directory to build your container with the tag `$REGISTRY/$PROJECT:$VERSION`:
```
sudo make all
```

To push your image to a registry, use `make push`.

### Keeping the Heptio registry tag

If you don't wish to push your image to your own registry (not changing `$REGISTRY`), but simply want to build it locally with the Heptio tag, *you need to modify the deployment file that are using.* This is either `examples/azure/00-ark-deployment.yaml` if you're using Azure, or `examples/common/10-deployment.yaml` for all other cloud providers.

Because the existing deployment files reference `gcr.io/heptio-images/ark:latest`, and Kubernetes handles the `latest` tag with the `imagePullPolicy` of "Always", your Ark deployment will pull from the Heptio image registry instead of using the local image. In order to use your local image, you'll need to specify a `$VERSION` tag (consistent with the value in the `Makefile`). If your `$VERSION` is "v0.3.3", for example, you need to set `spec.template.spec.containers[*].image` in your deployment file to be `gcr.io/heptio0images/ark:v0.3.3`.

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
