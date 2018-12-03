# Build from source

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

Run `make update` to regenerate files if you make the following changes:

* Add/edit/remove command line flags and/or their help text
* Add/edit/remove commands or subcommands
* Add new API types

Run [generate-proto.sh][13] to regenerate files if you make the following changes:

* Add/edit/remove protobuf message or service definitions. These changes require the [proto compiler][14]. 

### Cross compiling

By default, `make build` builds an `ark` binary for `linux-amd64`.
To build for another platform, run `make build-<GOOS>-<GOARCH>`.
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

## 5. Vendoring dependencies

If you need to add or update the vendored dependencies, see [Vendoring dependencies][11].

[0]: ../README.md
[1]: #prerequisites
[2]: #download
[3]: #build
[4]: https://blog.golang.org/organizing-go-code
[5]: https://golang.org/doc/install
[6]: https://github.com/heptio/ark/tree/master/examples
[7]: #run
[8]: config-definition.md
[10]: #vendoring-dependencies
[11]: vendoring-dependencies.md
[12]: #test
[13]: https://github.com/heptio/ark/blob/master/hack/generate-proto.sh
[14]: https://grpc.io/docs/quickstart/go.html#install-protocol-buffers-v3
[15]: https://docs.aws.amazon.com/cli/latest/topic/config-vars.html#the-shared-credentials-file
[16]: https://cloud.google.com/docs/authentication/getting-started#setting_the_environment_variable
[17]: https://aws.amazon.com/quickstart/architecture/heptio-kubernetes/
[18]: https://eksctl.io/
[19]: ../examples/README.md
[20]: api-types/backupstoragelocation.md
[21]: api-types/volumesnapshotlocation.md
