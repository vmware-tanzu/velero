# Build from source

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

## Getting the source

```bash
mkdir $HOME/go
export GOPATH=$HOME/go
go get github.com/heptio/ark
```

Where `go` is your [import path][4] for Go.

For Go development, it is recommended to add the Go import path (`$HOME/go` in this example) to your path.


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

## 4. Run

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

### Create a cluster

To provision a cluster on AWS using Amazon’s official CloudFormation templates, here are two options:

* EC2 [Quick Start for Kubernetes][17]

* eksctl - [a CLI for Amazon EKS][18]

### Option 1: Run your Ark server locally

Running the Ark server locally can speed up iterative development. This eliminates the need to rebuild the Ark server
image and redeploy it to the cluster with each change.

#### 1. Set enviroment variables

Set the appropriate environment variable for your cloud provider:

AWS: [AWS_SHARED_CREDENTIALS_FILE][15]

GCP: [GOOGLE_APPLICATION_CREDENTIALS][16]

Azure:

  1. AZURE_CLIENT_ID

  2. AZURE_CLIENT_SECRET

  3. AZURE_SUBSCRIPTION_ID

  4. AZURE_TENANT_ID

  5. AZURE_STORAGE_ACCOUNT_ID

  6. AZURE_STORAGE_KEY

  7. AZURE_RESOURCE_GROUP

#### 2. Create resources in a cluster

You may create resources on a cluster using our [example configurations][19].

##### Example

Here is how to setup using an existing cluster in AWS: At the root of the Ark repo:

- Edit `examples/aws/05-ark-backupstoragelocation.yaml` to point to your AWS S3 bucket and region. Note: you can run `aws s3api list-buckets` to get the name of all your buckets.

- (Optional) Edit `examples/aws/06-ark-volumesnapshotlocation.yaml` to point to your AWS region.

Then run the commands below.

`00-prereqs.yaml` contains all our CustomResourceDefinitions (CRDs) that allow us to perform CRUD operations on backups, restores, schedules, etc. it also contains the `heptio-ark` namespace, the `ark` ServiceAccount, and a cluster role binding to grant the `ark` ServiceAccount the cluster-admin role:

```bash
kubectl apply -f examples/common/00-prereqs.yaml
```

`10-deployment.yaml` is a sample Ark config resource for AWS:

```bash
kubectl apply -f examples/aws/10-deployment.yaml
```

And `05-ark-backupstoragelocation.yaml` specifies the location of your backup storage, together with the optional `06-ark-volumesnapshotlocation.yaml`:

```bash
kubectl apply -f examples/aws/05-ark-backupstoragelocation.yaml
```

or

```bash
kubectl apply -f examples/aws/05-ark-backupstoragelocation.yaml examples/aws/06-ark-volumesnapshotlocation.yaml
```

### 3. Start the Ark server

* Make sure `ark` is in your `PATH` or specify the full path.

* Set variable for Ark as needed. The variables below can be exported as environment variables or passed as CLI cmd flags:
  * `--kubeconfig`: set the path to the kubeconfig file the Ark server uses to talk to the Kubernetes apiserver
  * `--namespace`: the set namespace where the Ark server should look for backups, schedules, restores
  * `--log-level`: set the Ark server's log level
  * `--plugin-dir`: set the directory where the Ark server looks for plugins
  * `--metrics-address`: set the bind address and port where Prometheus metrics are exposed

* Start the server: `ark server`

### Option 2: Run your Ark server in a deployment

1. Install Ark using a deployment:

We have examples of deployments for different cloud providers in `examples/<cloud-provider>/10-deployment.yaml`.

2. Replace the deployment's default Ark image with the image that you built. Run:

```
kubectl --namespace=heptio-ark set image deployment/ark ark=$REGISTRY/ark:$VERSION
```

where `$REGISTRY` and `$VERSION` are the values that you built Ark with.

## 5. Vendoring dependencies

If you need to add or update the vendored dependencies, see [Vendoring dependencies][11].

[0]: ../README.md
[1]: #prerequisites
[2]: #download
[3]: #build
[4]: https://blog.golang.org/organizing-go-code
[5]: https://golang.org/doc/install
[6]: https://github.com/heptio/ark/tree/main/examples
[7]: #run
[8]: config-definition.md
[10]: #vendoring-dependencies
[11]: vendoring-dependencies.md
[12]: #test
[13]: https://github.com/heptio/ark/blob/main/hack/generate-proto.sh
[14]: https://grpc.io/docs/quickstart/go.html#install-protocol-buffers-v3
[15]: https://docs.aws.amazon.com/cli/latest/topic/config-vars.html#the-shared-credentials-file
[16]: https://cloud.google.com/docs/authentication/getting-started#setting_the_environment_variable
[17]: https://aws.amazon.com/quickstart/architecture/heptio-kubernetes/
[18]: https://eksctl.io/
[19]: ../examples/README.md
[20]: /api-types/backupstoragelocation.md
[21]: /api-types/volumesnapshotlocation.md
