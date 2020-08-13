---
title: "Run Ark on AWS"
layout: docs
---

To set up Ark on AWS, you:

* Create your S3 bucket
* Create an AWS IAM user for Ark
* Configure the server
* Create a Secret for your credentials

If you do not have the `aws` CLI locally installed, follow the [user guide][5] to set it up.

## Create S3 bucket

Heptio Ark requires an object storage bucket to store backups in. Create an S3 bucket, replacing placeholders appropriately:

```bash
aws s3api create-bucket \
    --bucket <YOUR_BUCKET> \
    --region <YOUR_REGION> \
    --create-bucket-configuration LocationConstraint=<YOUR_REGION>
```
NOTE: us-east-1 does not support a `LocationConstraint`.  If your region is `us-east-1`, omit the bucket configuration:

```bash
aws s3api create-bucket \
    --bucket <YOUR_BUCKET> \
    --region us-east-1
```

## Create IAM user

For more information, see [the AWS documentation on IAM users][14].

1. Create the IAM user:

    ```bash
    aws iam create-user --user-name heptio-ark
    ```

2. Attach policies to give `heptio-ark` the necessary permissions:

    ```bash
    aws iam attach-user-policy \
        --policy-arn arn:aws:iam::aws:policy/AmazonS3FullAccess \
        --user-name heptio-ark
    aws iam attach-user-policy \
        --policy-arn arn:aws:iam::aws:policy/AmazonEC2FullAccess \
        --user-name heptio-ark
    ```

3. Create an access key for the user:

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

4. Create an Ark-specific credentials file (`credentials-ark`) in your local directory:

    ```
    [default]
    aws_access_key_id=<AWS_ACCESS_KEY_ID>
    aws_secret_access_key=<AWS_SECRET_ACCESS_KEY>
    ```

    where the access key id and secret are the values returned from the `create-access-key` request.

## Credentials and configuration

In the Ark root directory, run the following to first set up namespaces, RBAC, and other scaffolding. To run in a custom namespace, make sure that you have edited the YAML files to specify the namespace. See [Run in custom namespace][0].

```bash
kubectl apply -f examples/common/00-prereqs.yaml
```

Create a Secret. In the directory of the credentials file you just created, run:

```bash
kubectl create secret generic cloud-credentials \
    --namespace <ARK_NAMESPACE> \
    --from-file cloud=credentials-ark
```

Specify the following values in the example files:

* In `examples/aws/00-ark-config.yaml`:

  * Replace `<YOUR_BUCKET>` and `<YOUR_REGION>`. See the [Config definition][6] for details.

* In `examples/common/10-deployment.yaml`:

  * Make sure that `spec.template.spec.containers[*].env.name` is "AWS_SHARED_CREDENTIALS_FILE".

* (Optional) If you run the nginx example, in file `examples/nginx-app/with-pv.yaml`:

    * Replace `<YOUR_STORAGE_CLASS_NAME>` with `gp2`. This is AWS's default `StorageClass` name.

## Start the server

In the root of your Ark directory, run:

  ```bash
  kubectl apply -f examples/aws/00-ark-config.yaml
  kubectl apply -f examples/common/10-deployment.yaml
  ```

  [0]: /namespace.md
  [6]: /config-definition.md#aws
  [14]: http://docs.aws.amazon.com/IAM/latest/UserGuide/introduction.html
