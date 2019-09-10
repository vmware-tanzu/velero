# Build from source

## Prerequisites

* Access to a Kubernetes cluster, version 1.7 or later.
* A DNS server on the cluster
* `kubectl` installed
* [Go][5] installed (minimum version 1.8)

## Get the source

### Option 1) Get latest (recommended)

```bash
mkdir $HOME/go
export GOPATH=$HOME/go
go get github.com/heptio/velero
```

Where `go` is your [import path][4] for Go.

For Go development, it is recommended to add the Go import path (`$HOME/go` in this example) to your path.

### Option 2) Release archive

Download the archive named `Source code` from the [release page][22] and extract it in your Go import path as `src/github.com/heptio/velero`.

Note that the Makefile targets assume building from a git repository. When building from an archive, you will be limited to the `go build` commands described below.

## Build

There are a number of different ways to build `velero` depending on your needs. This section outlines the main possibilities.

### Build the binary

To build the `velero` binary on your local machine, compiled for your OS and architecture, run:

```bash
go build ./cmd/velero
```

### Cross compiling

When using `make`, it will place all binaries under `_output/bin/$GOOS/$GOARCH` and will splice version and git commit information in so that `velero version` displays proper output. `velero install` will also use the version information to determine which tagged image to deploy.

```bash
make local
```

This builds binaries for both linux and darwin. You will find the binary for darwing here: `_output/bin/darwin/amd64/velero`, and the binary for linux here: `_output/bin/linux/amd64/velero`.

```bash
make build
```

This builds an `velero` binary for `linux-amd64`. 

To build for another platform, run `make build-<GOOS>-<GOARCH>`.

For example, to build for the Mac, run `make build-darwin-amd64`.

Velero's `Makefile` has a convenience target, `all-build`, that builds the following platforms:

* linux-amd64
* linux-arm
* linux-arm64
* darwin-amd64
* windows-amd64

## Images

To build a Velero container image, first set the `$REGISTRY` environment variable. For example, if you want to build the `gcr.io/my-registry/velero:master` image, set `$REGISTRY` to `gcr.io/my-registry`. Optionally, set the `$VERSION` environment variable to change the image tag. Then, run:

```bash
make container
```

To push your image to a registry, run:

```bash
make push
```

[4]: https://blog.golang.org/organizing-go-code
[5]: https://golang.org/doc/install
[22]: https://github.com/heptio/velero/releases