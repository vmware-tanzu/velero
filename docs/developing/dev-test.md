## 4. Run

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