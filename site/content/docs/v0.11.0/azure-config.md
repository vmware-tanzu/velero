---
title: "Run Velero on Azure"
layout: docs
---

To configure Velero on Azure, you:

* Download an official release of Velero
* Create your Azure storage account and blob container
* Create Azure service principal for Velero
* Configure the server
* Create a Secret for your credentials

If you do not have the `az` Azure CLI 2.0 installed locally, follow the [install guide][18] to set it up. 

Run:

```bash
az login
```

## Kubernetes cluster prerequisites

Ensure that the VMs for your agent pool allow Managed Disks. If I/O performance is critical,
consider using Premium Managed Disks, which are SSD backed.

## Download Velero

1. Download the [latest release's](https://github.com/heptio/velero/releases) tarball for your client platform.

1. Extract the tarball:
    ```bash
    tar -xvf <RELEASE-TARBALL-NAME>.tar.gz -C /dir/to/extract/to 
    ```
    We'll refer to the directory you extracted to as the "Velero directory" in subsequent steps.

1. Move the `velero` binary from the Velero directory to somewhere in your PATH.

_We strongly recommend that you use an [official release](https://github.com/heptio/velero/releases) of Velero. The tarballs for each release contain the
`velero` command-line client **and** version-specific sample YAML files for deploying Velero to your cluster. The code and sample YAML files in the master 
branch of the Velero repository are under active development and are not guaranteed to be stable. Use them at your own risk!_

## Create Azure storage account and blob container

Velero requires a storage account and blob container in which to store backups.

The storage account can be created in the same Resource Group as your Kubernetes cluster or
separated into its own Resource Group. The example below shows the storage account created in a
separate `Velero_Backups` Resource Group.

The storage account needs to be created with a globally unique id since this is used for dns. In
the sample script below, we're generating a random name using `uuidgen`, but you can come up with 
this name however you'd like, following the [Azure naming rules for storage accounts][19]. The 
storage account is created with encryption at rest capabilities (Microsoft managed keys) and is 
configured to only allow access via https.

```bash
# Create a resource group for the backups storage account. Change the location as needed.
AZURE_BACKUP_RESOURCE_GROUP=Velero_Backups
az group create -n $AZURE_BACKUP_RESOURCE_GROUP --location WestUS

# Create the storage account
AZURE_STORAGE_ACCOUNT_ID="velero$(uuidgen | cut -d '-' -f5 | tr '[A-Z]' '[a-z]')"
az storage account create \
    --name $AZURE_STORAGE_ACCOUNT_ID \
    --resource-group $AZURE_BACKUP_RESOURCE_GROUP \
    --sku Standard_GRS \
    --encryption-services blob \
    --https-only true \
    --kind BlobStorage \
    --access-tier Hot
```

Create the blob container named `velero`. Feel free to use a different name, preferably unique to a single Kubernetes cluster. See the [FAQ][20] for more details.

```bash
az storage container create -n velero --public-access off --account-name $AZURE_STORAGE_ACCOUNT_ID
```

## Get resource group for persistent volume snapshots

1. Set the name of the Resource Group that contains your Kubernetes cluster's virtual machines/disks.

    > **WARNING**: If you're using [AKS][22], `AZURE_RESOURCE_GROUP` must be set to the name of the auto-generated resource group that is created 
    when you provision your cluster in Azure, since this is the resource group that contains your cluster's virtual machines/disks.

    ```bash
    AZURE_RESOURCE_GROUP=<NAME_OF_RESOURCE_GROUP>
    ```

    If you are unsure of the Resource Group name, run the following command to get a list that you can select from. Then set the `AZURE_RESOURCE_GROUP` environment variable to the appropriate value.

    ```bash
    az group list --query '[].{ ResourceGroup: name, Location:location }'
    ```

    Get your cluster's Resource Group name from the `ResourceGroup` value in the response, and use it to set `$AZURE_RESOURCE_GROUP`.

## Create service principal

To integrate Velero with Azure, you must create an Velero-specific [service principal][17].

1. Obtain your Azure Account Subscription ID and Tenant ID:

    ```bash
    AZURE_SUBSCRIPTION_ID=`az account list --query '[?isDefault].id' -o tsv`
    AZURE_TENANT_ID=`az account list --query '[?isDefault].tenantId' -o tsv`
    ```

1. Create a service principal with `Contributor` role. This will have subscription-wide access, so protect this credential. You can specify a password or let the `az ad sp create-for-rbac` command create one for you.

    > If you'll be using Velero to backup multiple clusters with multiple blob containers, it may be desirable to create a unique username per cluster rather than the default `velero`.

    ```bash
    # Create service principal and specify your own password
    AZURE_CLIENT_SECRET=super_secret_and_high_entropy_password_replace_me_with_your_own
    az ad sp create-for-rbac --name "velero" --role "Contributor" --password $AZURE_CLIENT_SECRET

    # Or create service principal and let the CLI generate a password for you. Make sure to capture the password.
    AZURE_CLIENT_SECRET=`az ad sp create-for-rbac --name "velero" --role "Contributor" --query 'password' -o tsv`

    # After creating the service principal, obtain the client id
    AZURE_CLIENT_ID=`az ad sp list --display-name "velero" --query '[0].appId' -o tsv`
    ```

## Credentials and configuration

In the Velero directory (i.e. where you extracted the release tarball), run the following to first set up namespaces, RBAC, and other scaffolding. To run in a custom namespace, make sure that you have edited the YAML file to specify the namespace. See [Run in custom namespace][0].

```bash
kubectl apply -f config/common/00-prereqs.yaml
```

Now you need to create a Secret that contains all the environment variables you just set. The command looks like the following:

```bash
kubectl create secret generic cloud-credentials \
    --namespace <VELERO_NAMESPACE> \
    --from-literal AZURE_SUBSCRIPTION_ID=${AZURE_SUBSCRIPTION_ID} \
    --from-literal AZURE_TENANT_ID=${AZURE_TENANT_ID} \
    --from-literal AZURE_CLIENT_ID=${AZURE_CLIENT_ID} \
    --from-literal AZURE_CLIENT_SECRET=${AZURE_CLIENT_SECRET} \
    --from-literal AZURE_RESOURCE_GROUP=${AZURE_RESOURCE_GROUP}
```

Now that you have your Azure credentials stored in a Secret, you need to replace some placeholder values in the template files. Specifically, you need to change the following:

* In file `config/azure/05-backupstoragelocation.yaml`:

  * Replace `<YOUR_BLOB_CONTAINER>`, `<YOUR_STORAGE_RESOURCE_GROUP>`, and `<YOUR_STORAGE_ACCOUNT>`. See the [BackupStorageLocation definition][21] for details.

* In file `config/azure/06-volumesnapshotlocation.yaml`:

  * Replace `<YOUR_TIMEOUT>`. See the [VolumeSnapshotLocation definition][8] for details.

* (Optional, use only if you need to specify multiple volume snapshot locations) In `config/azure/00-deployment.yaml`:

  * Uncomment the `--default-volume-snapshot-locations` and replace provider locations with the values for your environment.

## Start the server

In the root of your Velero directory, run:

  ```bash
  kubectl apply -f config/azure/
  ```

[0]: namespace.md
[8]: api-types/volumesnapshotlocation.md#azure
[17]: https://docs.microsoft.com/en-us/azure/active-directory/develop/active-directory-application-objects
[18]: https://docs.microsoft.com/en-us/cli/azure/install-azure-cli
[19]: https://docs.microsoft.com/en-us/azure/architecture/best-practices/naming-conventions#storage
[20]: faq.md
[21]: api-types/backupstoragelocation.md#azure
[22]: https://azure.microsoft.com/en-us/services/kubernetes-service/
