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
    BUCKET=<YOUR_BUCKET>
    cat > heptio-ark-policy.json <<EOF
    {
        "Version": "2012-10-17",
        "Statement": [
            {
                "Effect": "Allow",
                "Action": [
                    "ec2:DescribeVolumes",
                    "ec2:DescribeSnapshots",
                    "ec2:CreateTags",
                    "ec2:CreateVolume",
                    "ec2:CreateSnapshot",
                    "ec2:DeleteSnapshot"
                ],
                "Resource": "*"
            },
            {
                "Effect": "Allow",
                "Action": [
                    "s3:GetObject",
                    "s3:DeleteObject",
                    "s3:PutObject",
                    "s3:AbortMultipartUpload",
                    "s3:ListMultipartUploadParts"
                ],
                "Resource": [
                    "arn:aws:s3:::${BUCKET}/*"
                ]
            },
            {
                "Effect": "Allow",
                "Action": [
                    "s3:ListBucket"
                ],
                "Resource": [
                    "arn:aws:s3:::${BUCKET}"
                ]
            }
        ]
    }
    EOF

    aws iam put-user-policy \
      --user-name heptio-ark \
      --policy-name heptio-ark \
      --policy-document file://heptio-ark-policy.json
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

* (Optional) If you run the nginx example, in file `examples/nginx-app/with-pv.yaml`:

    * Replace `<YOUR_STORAGE_CLASS_NAME>` with `gp2`. This is AWS's default `StorageClass` name.

## Start the server

In the root of your Ark directory, run:

  ```bash
  kubectl apply -f examples/aws/00-ark-config.yaml
  kubectl apply -f examples/aws/10-deployment.yaml
  ```

## ALTERNATIVE: Setup permissions using kube2iam

[Kube2iam](https://github.com/jtblin/kube2iam) is a Kubernetes application that allows managing AWS IAM permissions for pod via annotations rather than operating on API keys.

> This path assumes you have `kube2iam` already running in your Kubernetes cluster. If that is not the case, please install it first, following the docs here: https://github.com/jtblin/kube2iam

It can be set up for Ark by creating a role that will have required permissions, and later by adding the permissions annotation on the ark deployment to define which role it should use internally.

1. Create a Trust Policy document to allow the role being used for EC2 management & assume kube2iam role:

    ```bash
    cat > heptio-ark-trust-policy.json <<EOF
    {
        "Version": "2012-10-17",
        "Statement": [
            {
                "Effect": "Allow",
                "Principal": {
                    "Service": "ec2.amazonaws.com"
                },
                "Action": "sts:AssumeRole"
            },
            {
                "Effect": "Allow",
                "Principal": {
                    "AWS": "arn:aws:iam::<AWS_ACCOUNT_ID>:role/<ROLE_CREATED_WHEN_INITIALIZING_KUBE2IAM>"
                },
                "Action": "sts:AssumeRole"
            }
        ]
    }
    EOF
    ```

2. Create the IAM role:

    ```bash
    aws iam create-role --role-name heptio-ark --assume-role-policy-document file://./heptio-ark-trust-policy.json
    ```

3. Attach policies to give `heptio-ark` the necessary permissions:

    ```bash
    BUCKET=<YOUR_BUCKET>
    cat > heptio-ark-policy.json <<EOF
    {
        "Version": "2012-10-17",
        "Statement": [
            {
                "Effect": "Allow",
                "Action": [
                    "ec2:DescribeVolumes",
                    "ec2:DescribeSnapshots",
                    "ec2:CreateTags",
                    "ec2:CreateVolume",
                    "ec2:CreateSnapshot",
                    "ec2:DeleteSnapshot"
                ],
                "Resource": "*"
            },
            {
                "Effect": "Allow",
                "Action": [
                    "s3:GetObject",
                    "s3:DeleteObject",
                    "s3:PutObject",
                    "s3:AbortMultipartUpload",
                    "s3:ListMultipartUploadParts"
                ],
                "Resource": [
                    "arn:aws:s3:::${BUCKET}/*"
                ]
            },
            {
                "Effect": "Allow",
                "Action": [
                    "s3:ListBucket"
                ],
                "Resource": [
                    "arn:aws:s3:::${BUCKET}"
                ]
            }
        ]
    }
    EOF

    aws iam put-role-policy \
      --role-name heptio-ark \
      --policy-name heptio-ark-policy \
      --policy-document file://./heptio-ark-policy.json
    ```
4. Update AWS_ACCOUNT_ID & HEPTIO_ARK_ROLE_NAME in the file `examples/common/10-deployment-kube2iam.yaml`:

    ```
    ---
    apiVersion: apps/v1beta1
    kind: Deployment
    metadata:
        namespace: heptio-ark
        name: ark
    spec:
        replicas: 1
        template:
            metadata:
                labels:
                    component: ark
                annotations:
                    iam.amazonaws.com/role: arn:aws:iam::<AWS_ACCOUNT_ID>:role/heptio-ark
    ...
    ```

5. Run Ark deployment using the file `examples/aws/10-deployment-kube2iam.yaml`.

  [0]: namespace.md
  [6]: config-definition.md#aws
  [14]: http://docs.aws.amazon.com/IAM/latest/UserGuide/introduction.html
