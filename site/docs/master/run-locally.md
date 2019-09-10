# Run Velero locally in development

Running the Velero server locally can speed up iterative development. This eliminates the need to rebuild the Velero server
image and redeploy it to the cluster with each change.

## Run Velero locally with a remote cluster

Velero runs against the Kubernetes API server as the endpoint (as per the `kubeconfig` configuration), so both the Velero server and client use the same `client-go` to communicate with Kubernetes. This means the Velero server can be run locally just as functionally as if it was running in the remote cluster.

### Prerequisites

When running Velero, you will need to ensure that you set up all of the following:

* Appropriate RBAC permissions in the cluster
  * Read access for all data from the source cluster and namespaces
  * Write access to the target cluster and namespaces
* Cloud provider credentials
  * Read/write access to volumes
  * Read/write access to object storage for backup data
* A [BackupStorageLocation][20] object definition for the Velero server
* (Optional) A [VolumeSnapshotLocation][21] object definition for the Velero server, to take PV snapshots

### 1. Create a cluster

To provision a cluster on AWS using Amazon’s official CloudFormation templates, here are two options:

* EC2 [Quick Start for Kubernetes][17]

* eksctl - [a CLI for Amazon EKS][18]

#### 2. Set enviroment variables

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

#### 3. Create required Velero resources in the cluster

You can use the `velero install` command to install velero into your cluster, then remove the deployment from the cluster, leaving you
with all of the required in-cluster resources.

##### Example install

This examples assumes you are using an existing cluster in AWS.

Using the `velero` binary that you've built, run `velero install`:

```bash
# velero install requires a credentials file to exist, but we will
# not be using it since we're running the server locally, so just
# create an empty file to pass to the install command.
touch fake-credentials-file

velero install \
  --provider aws \
  --bucket $BUCKET \
  --backup-location-config region=$REGION \
  --snapshot-location-config region=$REGION \
  --secret-file ./fake-credentials-file

# 'velero install' creates an in-cluster deployment of the
# velero server using an official velero image, but we want
# to run the velero server on our local machine using the
# binary we built, so delete the in-cluster deployment.
kubectl --namespace velero delete deployment velero

rm fake-credentials-file
```

##### Scale deployyment down to zero

Scale the Velero deployment down to 0 so it is not simultaneously being run on the remote cluster and potentially causing things to get out of sync:

`kubectl scale --replicas=0 deployment velero -n velero`

#### 4. Start the Velero server

* To run the server locally, use the full path according to the binary you need. Example, if you are on a Mac, and using `AWS` as a provider, this is how to run the binary you built from source using the full path: `AWS_SHARED_CREDENTIALS_FILE=<path-to-real-aws-credentials-file> /_output/bin/darwin/amd64/velero`. Alternatively, you may add the `velero` binary to your `PATH`.

* Start the server: `velero server [CLI flags]`. The following CLI flags may be useful to customize, but see `velero server --help` for full details:
  * `--log-level`: set the Velero server's log level (default `info`, use `debug` for the most logging)
  * `--kubeconfig`: set the path to the kubeconfig file the Velero server uses to talk to the Kubernetes apiserver (default `$KUBECONFIG`)
  * `--namespace`: the set namespace where the Velero server should look for backups, schedules, restores (default `velero`)
  * `--plugin-dir`: set the directory where the Velero server looks for plugins (default `/plugins`)
  * `--metrics-address`: set the bind address and port where Prometheus metrics are exposed (default `:8085`)

[15]: https://docs.aws.amazon.com/cli/latest/topic/config-vars.html#the-shared-credentials-file
[16]: https://cloud.google.com/docs/authentication/getting-started#setting_the_environment_variable
[17]: https://aws.amazon.com/quickstart/architecture/heptio-kubernetes/
[18]: https://eksctl.io/
[20]: api-types/backupstoragelocation.md
[21]: api-types/volumesnapshotlocation.md