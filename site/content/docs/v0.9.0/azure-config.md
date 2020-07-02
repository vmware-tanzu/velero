---
title: "Run Ark on Azure"
layout: docs
---

To configure Ark on Azure, you:

* Create your Azure storage account and blob container
* Create Azure service principal for Ark
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

## Create Azure storage account and blob container

Heptio Ark requires a storage account and blob container in which to store backups.

The storage account can be created in the same Resource Group as your Kubernetes cluster or
separated into its own Resource Group. The example below shows the storage account created in a
separate `Ark_Backups` Resource Group.

The storage account needs to be created with a globally unique id since this is used for dns. In
the sample script below, we're generating a random name using `uuidgen`, but you can come up with 
this name however you'd like, following the [Azure naming rules for storage accounts][19]. The 
storage account is created with encryption at rest capabilities (Microsoft managed keys) and is 
configured to only allow access via https.

```bash
# Create a resource group for the backups storage account. Change the location as needed.
AZURE_BACKUP_RESOURCE_GROUP=Ark_Backups
az group create -n $AZURE_BACKUP_RESOURCE_GROUP --location WestUS

# Create the storage account
AZURE_STORAGE_ACCOUNT_ID="ark$(uuidgen | cut -d '-' -f5 | tr '[A-Z]' '[a-z]')"
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
    --query '[0].value' \
    -o tsv`
```

## Create service principal

To integrate Ark with Azure, you must create an Ark-specific [service principal][17]. Note that seven environment variables must be set for Ark to work properly.

1. Obtain your Azure Account Subscription ID and Tenant ID:

    ```bash
    AZURE_SUBSCRIPTION_ID=`az account list --query '[?isDefault].id' -o tsv`
    AZURE_TENANT_ID=`az account list --query '[?isDefault].tenantId' -o tsv`
    ```

1. Set the name of the Resource Group that contains your Kubernetes cluster.

    ```bash
    # Make sure this is the name of the second resource group. See warning.
    AZURE_RESOURCE_GROUP=<NAME_OF_RESOURCE_GROUP_2>
    ```

    WARNING: `AZURE_RESOURCE_GROUP` must be set to the name of the second resource group that is created when you provision your cluster in Azure. Your cluster is provisioned in the resource group that you specified when you created the cluster. Your disks, however, are provisioned in the second resource group.

    If you are unsure of the Resource Group name, run the following command to get a list that you can select from. Then set the `AZURE_RESOURCE_GROUP` environment variable to the appropriate value.

    ```bash
    az group list --query '[].{ ResourceGroup: name, Location:location }'
    ```

    Get your cluster's Resource Group name from the `ResourceGroup` value in the response, and use it to set `$AZURE_RESOURCE_GROUP`.

1. Create a service principal with `Contributor` role. This will have subscription-wide access, so protect this credential. You can specify a password or let the `az ad sp create-for-rbac` command create one for you.

    ```bash
    # Create service principal and specify your own password
    AZURE_CLIENT_SECRET=super_secret_and_high_entropy_password_replace_me_with_your_own
    az ad sp create-for-rbac --name "heptio-ark" --role "Contributor" --password $AZURE_CLIENT_SECRET

    # Or create service principal and let the CLI generate a password for you. Make sure to capture the password.
    AZURE_CLIENT_SECRET=`az ad sp create-for-rbac --name "heptio-ark" --role "Contributor" --query 'password' -o tsv`

    # After creating the service principal, obtain the client id
    AZURE_CLIENT_ID=`az ad sp list --display-name "heptio-ark" --query '[0].appId' -o tsv`
    ```

## Credentials and configuration

In the Ark root directory, run the following to first set up namespaces, RBAC, and other scaffolding. To run in a custom namespace, make sure that you have edited the YAML file to specify the namespace. See [Run in custom namespace][0].

```bash
kubectl apply -f examples/common/00-prereqs.yaml
```

Now you need to create a Secret that contains all the seven environment variables you just set. The command looks like the following:

```bash
kubectl create secret generic cloud-credentials \
    --namespace <ARK_NAMESPACE> \
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

  * Replace `<YOUR_BUCKET>` and `<YOUR_TIMEOUT>`. See the [Config definition][8] for details.

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
    apiTimeout: 15m
backupStorageProvider:
  name: azure
  bucket: ark
backupSyncPeriod: 30m
gcSyncPeriod: 30m
scheduleSyncPeriod: 1m
restoreOnlyMode: false
```

## Start the server

In the root of your Ark directory, run:

  ```bash
  kubectl apply -f examples/azure/
  ```

  [0]: namespace.md
  [8]: config-definition.md#azure
  [17]: https://docs.microsoft.com/en-us/azure/active-directory/develop/active-directory-application-objects
  [18]: https://docs.microsoft.com/en-us/cli/azure/install-azure-cli
  [19]: https://docs.microsoft.com/en-us/azure/architecture/best-practices/naming-conventions#storage