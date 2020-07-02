---
title: "Cloud Provider Specifics"
layout: docs
---

> NOTE: Documentation may change between releases. See the [Changelog][20] for links to previous versions of this repository and its docs.
>
> To ensure that you are working off a specific release, `git checkout <VERSION_TAG>` where `<VERSION_TAG>` is the appropriate tag for the Ark version you wish to use (e.g. "v0.3.3"). You should `git checkout master` only if you're planning on [building the Ark image from scratch][21].

While the [Quickstart][0] uses a local storage service to quickly set up Heptio Ark as a demonstration, this document details additional configurations that are required when integrating with the cloud providers below:

* [Setup][12]
  * [AWS][1]
  * [GCP][2]
  * [Azure][3]
* [Run][13]
  * [Ark server][9]
  * [Basic example (no PVs)][10]
  * [Snapshot example (with PVs)][11]


## Setup
### AWS

#### IAM user creation

To integrate Heptio Ark with AWS, you should follow the instructions below to create an Ark-specific [IAM user][14].

1. If you do not have the AWS CLI locally installed, follow the [user guide][5] to set it up.

2. Create an IAM user:

    ```bash
    aws iam create-user --user-name heptio-ark
    ```

3. Attach a policy to give `heptio-ark` the necessary permissions:

    ```bash
    aws iam attach-user-policy \
        --policy-arn arn:aws:iam::aws:policy/AmazonS3FullAccess \
        --user-name heptio-ark
    aws iam attach-user-policy \
        --policy-arn arn:aws:iam::aws:policy/AmazonEC2FullAccess \
        --user-name heptio-ark
    ```

4. Create an access key for the user:

    ```bash
    aws iam create-access-key --user-name heptio-ark
    ```

    The result should look like:

    ```json
     {
        "AccessKey": {
              "UserName": "heptio-ark",
              "Status": "Active",
              "CreateDate": "2017-07-31T22:24:41.576Z",
              "SecretAccessKey": <AWS_SECRET_ACCESS_KEY>,
              "AccessKeyId": <AWS_ACCESS_KEY_ID>
          }
     }
    ```
5. Using the output from the previous command, create an Ark-specific credentials file (`credentials-ark`) in your local directory that looks like the following:

    ```
    [default]
    aws_access_key_id=<AWS_ACCESS_KEY_ID>
    aws_secret_access_key=<AWS_SECRET_ACCESS_KEY>
    ```


#### Credentials and configuration

In the Ark root directory, run the following to first set up namespaces, RBAC, and other scaffolding:

```bash
kubectl apply -f examples/common/00-prereqs.yaml
```

Create a Secret, running this command in the local directory of the credentials file you just created:

```bash
kubectl create secret generic cloud-credentials \
    --namespace heptio-ark \
    --from-file cloud=credentials-ark
```

Now that you have your IAM user credentials stored in a Secret, you need to replace some placeholder values in the template files. Specifically, you need to change the following:

* In file `examples/aws/00-ark-config.yaml`:

  * Replace `<YOUR_BUCKET>` and `<YOUR_REGION>`. See the [Config definition][6] for details.


* In file `examples/common/10-deployment.yaml`:

  * Make sure that `spec.template.spec.containers[*].env.name` is "AWS_SHARED_CREDENTIALS_FILE".


* (Optional) If you are running the Nginx example, in file `examples/nginx-app/with-pv.yaml`:

    * Replace `<YOUR_STORAGE_CLASS_NAME>` with `gp2`. This is AWS's default `StorageClass` name.


### GCP

#### Service account creation

To integrate Heptio Ark with GCP, you should follow the instructions below to create an Ark-specific [Service Account][15].

1. If you do not have the gcloud CLI locally installed, follow the [user guide][16] to set it up.

2. View your current config settings:

    ```bash
    gcloud config list
    ```

    Store the `project` value from the results in the environment variable `$PROJECT_ID`.

2. Create a service account:

    ```bash
    gcloud iam service-accounts create heptio-ark \
        --display-name "Heptio Ark service account"
    ```

    Then list all accounts and find the `heptio-ark` account you just created:
    ```bash
    gcloud iam service-accounts list
    ```

    Set the `$SERVICE_ACCOUNT_EMAIL` variable to match its `email` value.

3. Attach policies to give `heptio-ark` the necessary permissions to function (replacing placeholders appropriately):

    ```bash
    gcloud projects add-iam-policy-binding $PROJECT_ID \
        --member serviceAccount:$SERVICE_ACCOUNT_EMAIL \
        --role roles/compute.storageAdmin
    gcloud projects add-iam-policy-binding $PROJECT_ID \
        --member serviceAccount:$SERVICE_ACCOUNT_EMAIL \
        --role roles/storage.admin
    ```

4. Create a service account key, specifying an output file (`credentials-ark`) in your local directory:

    ```bash
    gcloud iam service-accounts keys create credentials-ark \
        --iam-account $SERVICE_ACCOUNT_EMAIL
    ```

#### Credentials and configuration

In the Ark root directory, run the following to first set up namespaces, RBAC, and other scaffolding:

```bash
kubectl apply -f examples/common/00-prereqs.yaml
```

Create a Secret, running this command in the local directory of the credentials file you just created:

```bash
kubectl create secret generic cloud-credentials \
    --namespace heptio-ark \
    --from-file cloud=credentials-ark
```

Now that you have your Google Cloud credentials stored in a Secret, you need to replace some placeholder values in the template files. Specifically, you need to change the following:

* In file `examples/gcp/00-ark-config.yaml`:

  * Replace `<YOUR_BUCKET>` and `<YOUR_PROJECT>`. See the [Config definition][7] for details.


* In file `examples/common/10-deployment.yaml`:

  * Change `spec.template.spec.containers[*].env.name` to "GOOGLE_APPLICATION_CREDENTIALS".


* (Optional) If you are running the Nginx example, in file `examples/nginx-app/with-pv.yaml`:

    * Replace `<YOUR_STORAGE_CLASS_NAME>` with `standard`. This is GCP's default `StorageClass` name.

### Azure

#### Kubernetes cluster prerequisites

Ensure that the VMs for your agent pool allow Managed Disks. If I/O performance is critical,
consider using Premium Managed Disks, as these are SSD backed.

#### Service principal creation
To integrate Heptio Ark with Azure, you should follow the instructions below to create an Ark-specific [service principal][17].

1. If you do not have the `az` Azure CLI 2.0 locally installed, follow the [install guide][18] to set it up. Once done, run:

    ```bash
    az login
    ```

2. There are seven environment variables that need to be set for Heptio Ark to work properly. The following steps detail how to acquire these, in the process of setting up the necessary RBAC.

3. Obtain your Azure Account Subscription ID and Tenant ID:

    ```bash
    AZURE_SUBSCRIPTION_ID=`az account list --query '[?isDefault].id' -o tsv`
    AZURE_TENANT_ID=`az account list --query '[?isDefault].tenantId' -o tsv`
    ```

4. Set the name of the Resource Group that contains your Kubernetes cluster.

    ```bash
    # Change "Kubernetes" as needed
    AZURE_RESOURCE_GROUP=Kubernetes
    ```

    If you are unsure of the Resource Group name, run the following command to get a list that you can select from. Then set the `AZURE_RESOURCE_GROUP` environment variable to the appropriate value.

    ```bash
    az group list --query '[].{ ResourceGroup: name, Location:location }'
    ```

    Get your cluster's Resource Group name from the `ResourceGroup` value in the response, and use it to set `$AZURE_RESOURCE_GROUP`. (Also note the `Location` value in the response -- this is later used in the Azure-specific portion of the Ark Config).

5. Create a service principal with `Contributor` role. This will have subscription-wide access, so protect this credential. You can specify a password or let the `az ad sp create-for-rbac` command create one for you.

    ```bash
    # Create service principal and specify your own password
    AZURE_CLIENT_SECRET=super_secret_and_high_entropy_password_replace_me_with_your_own
    az ad sp create-for-rbac --name "heptio-ark" --role "Contributor" --password $AZURE_CLIENT_SECRET

    # Or create service principal and let the cli generate a password for you. ensure we capture the password though.
    AZURE_CLIENT_SECRET=`az ad sp create-for-rbac --name "heptio-ark" --role "Contributor" --query 'password' -o tsv`

    # After creating the service principal, obtain the client id
    AZURE_CLIENT_ID=`az ad sp list --display-name "heptio-ark" --query '[0].appId' -o tsv`
    ```

6. Create the storage account and blob container for Ark to store the backups in.

    The storage account can be created in the same Resource Group as your Kubernetes cluster or
    separated into its own Resource Group. The example below shows the storage account created in a
    separate `Ark_Backups` Resource Group.

    The storage account needs to be created with a globally unique id since this is used for dns. The
    random function ensures you don't have to come up with a unique name. The storage account is
    created with encryption at rest capabilities (Microsoft managed keys) and is configured to only
    allow access via https.

   ```bash
    # Create a resource group for the backups storage account. Change the location as needed.
    AZURE_BACKUP_RESOURCE_GROUP=Ark_Backups
    az group create -n $AZURE_BACKUP_RESOURCE_GROUP --location WestUS

    # Create the storage account
    AZURE_STORAGE_ACCOUNT_ID="ark`cat /proc/sys/kernel/random/uuid | cut -d '-' -f5`"
    az storage account create \
      --name $AZURE_STORAGE_ACCOUNT_ID \
      --resource-group $AZURE_BACKUP_RESOURCE_GROUP \
      --sku Standard_GRS \
      --encryption-services blob \
      --https-only true \
      --kind BlobStorage \
      --access-tier Hot

    # Create the blob container named "ark". Feel free to use a different name; you'll need to
    # adjust the `bucket` field under `backupStorageProvider` in the Ark Config accordingly if you do.
    az storage container create -n ark --public-access off --account-name $AZURE_STORAGE_ACCOUNT_ID

    # Obtain the storage access key for the storage account just created
    AZURE_STORAGE_KEY=`az storage account keys list \
      --account-name $AZURE_STORAGE_ACCOUNT_ID \
      --resource-group $AZURE_BACKUP_RESOURCE_GROUP \
      --query [0].value \
      -o tsv`
   ```

#### Credentials and configuration

In the Ark root directory, run the following to first set up namespaces, RBAC, and other scaffolding:

```bash
kubectl apply -f examples/common/00-prereqs.yaml
```

Now you need to create a Secret that contains all the seven environment variables you just set. The command looks like the following:

```bash
kubectl create secret generic cloud-credentials \
    --namespace heptio-ark \
    --from-literal AZURE_SUBSCRIPTION_ID=${AZURE_SUBSCRIPTION_ID} \
    --from-literal AZURE_TENANT_ID=${AZURE_TENANT_ID} \
    --from-literal AZURE_RESOURCE_GROUP=${AZURE_RESOURCE_GROUP} \
    --from-literal AZURE_CLIENT_ID=${AZURE_CLIENT_ID} \
    --from-literal AZURE_CLIENT_SECRET=${AZURE_CLIENT_SECRET} \
    --from-literal AZURE_STORAGE_ACCOUNT_ID=${AZURE_STORAGE_ACCOUNT_ID} \
    --from-literal AZURE_STORAGE_KEY=${AZURE_STORAGE_KEY}
```

Now that you have your Azure credentials stored in a Secret, you need to replace some placeholder values in the template files. Specifically, you need to change the following:

* In file `examples/azure/10-ark-config.yaml`:

  * Replace `<YOUR_BUCKET>`, `<YOUR_LOCATION>`, and `<YOUR_TIMEOUT>`. See the [Config definition][8] for details.

Here is an example of a completed file.

```yaml
apiVersion: ark.heptio.com/v1
kind: Config
metadata:
  namespace: heptio-ark
  name: default
persistentVolumeProvider:
  name: azure
  config:
    location: "West US"
    apiTimeout: 15m
backupStorageProvider:
  name: azure
  bucket: ark
backupSyncPeriod: 30m
gcSyncPeriod: 30m
scheduleSyncPeriod: 1m
restoreOnlyMode: false
```

You can get a complete list of Azure locations with the following command:

```bash
az account list-locations --query "sort([].displayName)" -o tsv
```


## Run

### Ark server

Make sure that you have run `kubectl apply -f examples/common/00-prereqs.yaml` first (this command is incorporated in the previous setup instructions because it creates the necessary namespaces).

* **AWS and GCP**

  Start the Ark server itself, using the Config from the appropriate cloud-provider-specific directory:

  ```bash
  kubectl apply -f examples/common/10-deployment.yaml
  kubectl apply -f examples/<CLOUD-PROVIDER>/
  ```
* **Azure**

  Because Azure loads its credentials differently (from environment variables rather than a file), you need to instead run:

  ```bash
  kubectl apply -f examples/azure/
  ```

### Basic example (No PVs)

Start the sample nginx app:

```bash
kubectl apply -f examples/nginx-app/base.yaml
```

Now create a backup:

```bash
ark backup create nginx-backup --selector app=nginx
```

Simulate a disaster:

```bash
kubectl delete namespaces nginx-example
```

Now restore your lost resources:

```bash
ark restore create nginx-backup
```

### Snapshot example (With PVs)

> NOTE: For Azure, your Kubernetes cluster needs to be version 1.7.2+ in order to support PV snapshotting of its managed disks.

Start the sample nginx app:

```bash
kubectl apply -f examples/nginx-app/with-pv.yaml
```

Because Kubernetes does not automatically transfer labels from PVCs to dynamically generated PVs, you need to do so manually:

```bash
nginx_pv_name=$(kubectl get pv -o jsonpath='{.items[?(@.spec.claimRef.name=="nginx-logs")].metadata.name}')
kubectl label pv $nginx_pv_name app=nginx
```

Now create a backup with PV snapshotting:

```bash
ark backup create nginx-backup --selector app=nginx
```

Simulate a disaster:

```bash
kubectl delete namespaces nginx-example
kubectl delete pv $nginx_pv_name
```

Because the default [reclaim policy][19] for dynamically-provisioned PVs is "Delete", the above commands should trigger your cloud provider to delete the disk backing the PV. The deletion process is asynchronous so this may take some time. **Before continuing to the next step, check your cloud provider (via dashboard or CLI) to confirm that the disk no longer exists.**

Now restore your lost resources:

```bash
ark restore create nginx-backup
```

[0]: /README.md#quickstart
[1]: #aws
[2]: #gcp
[3]: #azure
[4]: /examples/aws
[5]: http://docs.aws.amazon.com/cli/latest/userguide/installing.html
[6]: config-definition.md#aws
[7]: config-definition.md#gcp
[8]: config-definition.md#azure
[9]: #ark-server
[10]: #basic-example-no-pvs
[11]: #snapshot-example-with-pvs
[12]: #setup
[13]: #run
[14]: http://docs.aws.amazon.com/IAM/latest/UserGuide/introduction.html
[15]: https://cloud.google.com/compute/docs/access/service-accounts
[16]: https://cloud.google.com/compute/docs/gcloud-compute
[17]: https://docs.microsoft.com/en-us/azure/active-directory/develop/active-directory-application-objects
[18]: https://docs.microsoft.com/en-us/cli/azure/install-azure-cli
[19]: https://kubernetes.io/docs/concepts/storage/persistent-volumes/#reclaiming
[20]: /CHANGELOG.md
[21]: /docs/build-from-scratch.md

