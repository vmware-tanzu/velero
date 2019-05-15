# Use IBM Cloud Object Storage as Ark's storage destination.
You can deploy Ark on IBM [Public][5] or [Private][4] clouds, or even on any other Kubernetes cluster, but anyway you can use IBM Cloud Object Store as a destination for Ark's backups. 

To set up IBM Cloud Object Storage (COS) as Ark's destination, you:

* Create your COS instance
* Create an S3 bucket
* Define a service that can store data in the bucket
* Configure and start the Ark server


## Create COS instance
If you don’t have a COS instance, you can create a new one, according to the detailed instructions in [Creating a new resource instance][1].

## Create an S3 bucket
Heptio Ark requires an object storage bucket to store backups in. See instructions in [Create some buckets to store your data][2].

## Define a service that can store data in the bucket.
The process of creating service credentials is described in [Service credentials][3].
Several comments:

1. The Ark service will write its backup into the bucket, so it requires the “Writer” access role.

2. Ark uses an AWS S3 compatible API. Which means it authenticates using a signature created from a pair of access and secret keys — a set of HMAC credentials. You can create these HMAC credentials by specifying `{“HMAC”:true}` as an optional inline parameter. See step 3 in the [Service credentials][3] guide.

3. After successfully creating a Service credential, you can view the JSON definition of the credential. Under the `cos_hmac_keys` entry there are `access_key_id` and `secret_access_key`. We will use them in the next step.

4. Create an Ark-specific credentials file (`credentials-ark`) in your local directory:

    ```
    [default]
    aws_access_key_id=<ACCESS_KEY_ID>
    aws_secret_access_key=<SECRET_ACCESS_KEY>
    ```

    where the access key id and secret are the values that we got above.

## Credentials and configuration

In the Ark directory (i.e. where you extracted the release tarball), run the following to first set up namespaces, RBAC, and other scaffolding. To run in a custom namespace, make sure that you have edited the YAML files to specify the namespace. See [Run in custom namespace][0].

```bash
kubectl apply -f config/common/00-prereqs.yaml
```

Create a Secret. In the directory of the credentials file you just created, run:

```bash
kubectl create secret generic cloud-credentials \
    --namespace <ARK_NAMESPACE> \
    --from-file cloud=credentials-ark
```

Specify the following values in the example files:

* In `config/ibm/05-ark-backupstoragelocation.yaml`:

  * Replace `<YOUR_BUCKET>`, `<YOUR_REGION>` and `<YOUR_URL_ACCESS_POINT>`. See the [BackupStorageLocation definition][6] for details.

* (Optional) If you run the nginx example, in file `config/nginx-app/with-pv.yaml`:

    * Replace `<YOUR_STORAGE_CLASS_NAME>` with your `StorageClass` name.

## Start the Ark server

In the root of your Ark directory, run:

  ```bash
  kubectl apply -f config/ibm/05-ark-backupstoragelocation.yaml
  kubectl apply -f config/ibm/10-deployment.yaml
  ```

  [0]: namespace.md
  [1]: https://console.bluemix.net/docs/services/cloud-object-storage/basics/order-storage.html#creating-a-new-resource-instance
  [2]: https://console.bluemix.net/docs/services/cloud-object-storage/getting-started.html#create-buckets
  [3]: https://console.bluemix.net/docs/services/cloud-object-storage/iam/service-credentials.html#service-credentials
  [4]: https://www.ibm.com/support/knowledgecenter/SSBS6K_2.1.0/kc_welcome_containers.html
  [5]: https://console.bluemix.net/docs/containers/container_index.html#container_index
  [6]: api-types/backupstoragelocation.md#aws
  [14]: http://docs.aws.amazon.com/IAM/latest/UserGuide/introduction.html
