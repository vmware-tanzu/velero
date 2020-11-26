---
title: "Use Tencent Cloud Object Storage as Velero's storage destination."
layout: docs
---


You can deploy Velero on Tencent [TKE](https://console.cloud.tencent.com/tke2/) , or even on any other Kubernetes cluster, but anyway you can use Tencent Cloud Object Store as a destination for Velero’s backups.


## Prerequisites

- Registered [Tencent Cloud Account](https://cloud.tencent.com/register).
- Tencent Cloud COS service, referred to as COS, has been launched.
- A Kubernetes cluster has been created, cluster version v1.10 or later, and the cluster can use DNS and Internet services normally. If you need to create a TKE cluster, Please refer to [Create a cluster](https://cloud.tencent.com/document/product/457/32189).

## Create a Tencent cloud COS bucket

Create an object bucket for Velero to store backups in the Tencent Cloud COS console. About how to create a bucket, please refer to Tencent Cloud COS [Create a bucket](https://cloud.tencent.com/document/product/436/13309) usage instructions.

Set access to the bucket through the object storage console, the bucket needs to be **read** and **written**, so the account is granted data reading,  data writing permissions, About how to configure See [Permission Access Settings](https://cloud.tencent.com/document/product/436/13315.E5.8D.95.E4.B8.AA.E6.8E.88.E6.9D.83).

## Get bucket access credentials

Velero uses an AWS S3-compatible API to access Tencent Cloud COS storage, which requires authentication using a pair of access key IDs and key-created signatures, and in the S3 API parameter, the "access_key_id" field is the access key ID and the "secret_access_key" field is the key.

Create and acquire a new and acquired COS authorized account in the [Tencent Cloud Access Management Console](https://console.cloud.tencent.com/cam/capi) for Tencent Cloud Keys "SecretId" and "SecretKey". Where the 'SecretId' value corresponds to the "access_key_id" field, the "SecretKey" value corresponds to the "secret_access_key" field.

Create the credential profile "credentials-velero" required by Velero in the local directory based on the above correspondence:

```bash
[default]
aws_access_key_id=<SecretId>
aws_secret_access_key=<SecretKey>
```

## Install Velero 

Download the latest official [release](https://github.com/vmware-tanzu/velero/releases) Velero package into a clustered environment, as an example of the latest version v1.5.2 at the time of writing this document.

```bash
wget https://github.com/vmware-tanzu/velero/releases/download/v1.5.2/velero-v1.5.2-linux-amd64.tar.gz
```

Extract the compression package, which contains the velero command line execution file and some sample files.

```bash
tar -xvf velero-v1.5.2-linux-amd64.tar.gz
```

Move the 'velero' executable from the decompressed directory to the system environment variable directory for direct use, and this example moves to the /usr/bin directory.

```bash
mv velero-v1.5.2-linux-amd64/velero /usr/bin/
```
Follow the Velero installation command below to create velero and restic workloads and other necessary resource objects.
```bash
velero install  --provider aws --plugins velero/velero-plugin-for-aws:v1.1.0 --bucket  <BucketName> \
--secret-file ./credentials-velero \
--use-restic \
--default-volumes-to-restic \
--backup-location-config \
region=ap-guangzhou,s3ForcePathStyle="true",s3Url=https://cos.ap-guangzhou.myqcloud.com
```

Description of the parameters:

- `--provider`: Declares the type of plug-in provided by "aws".

- `--plugins`: Use the AWS S3 compatible API plug-in "velero-plugin-for-aws".

- `--bucket`: The bucket name created at Tencent Cloud COS.

- `--secret-file`: Access tencent cloud COS access credential file for the "credentials-velero" credential file created above.

- `--use-restic`: Back up and restore persistent volume data using the open source free backup tool [restic](https://github.com/restic/restic).  However, 'hostPath' volumes are not supported, see the [restic limit](https://velero.io/docs/v1.5/restic/#limitations) for details), an integration 		that complements Velero's backup capabilities and is recommended to be turned on.

- `--default-volumes-to-restic`: Enable the use of Restic to back up all Pod volumes, provided that the `--use-restic`parameter needs to be turned on.

- `--backup-location-config`: Back up the bucket access-related configuration:

  `region`: Tencent cloud COS bucket area, for example, if the created region is Guangzhou, the Region parameter value is "ap-guangzhou".

  `s3ForcePathStyle`: Use the S3 file path format.

  `s3Url`: Tencent Cloud COS-compatible S3 API access address,Note that instead of creating a COS bucket for public network access domain name, you must use a format of "https://cos.`region`.myqcloud.com" URL, for example, if the region is Guangzhou, the parameter value is "https://cos.ap-guangzhou.myqcloud.com.".

There are other installation parameters that can be viewed using `velero install --help`, such as setting `--use-volume-snapshots-false`  to close the storage volume data snapshot backup if you do not want to back up the storage volume data.

After executing the installation commands above, the installation process looks like this:

{{< figure src="https://main.qcloudimg.com/raw/9015313121ed7987558c88081b052574.png" width="100%">}}

After the installation command is complete, wait for the velero and restic workloads to be ready to see if the configured storage location is available.

Executing the 'velero backup-location get' command to view the storage location status and display "Available" indicates that access to Tencent Cloud COS is OK, as shown in the following image:

{{< figure src="https://main.qcloudimg.com/raw/69194157ccd5e377d1e7d914fd8c0336.png" width="100%">}}

At this point, The installation using Tencent Cloud COS as Velero storage location is complete, If you need more installation information about Velero, You can see the official website [Velero documentation](https://velero.io/docs/) .

## Velero backup and restore example

In the cluster, use the helm tool to create a minio test service with a persistent volume, and the minio installation method can be found in the [minio installation](https://github.com/minio/charts), in which case I have bound a load balancer for the minio service to access the management page using a public address in the browser.

{{< figure src="https://main.qcloudimg.com/raw/f0fff5228527edc72d6e71a50d5dc966.png" width="100%">}}

Sign in to the minio web management page and upload some image data for the test, as shown below:


{{< figure src="https://main.qcloudimg.com/raw/e932223585c0b19891cc085ad7f438e1.png" width="100%">}}

With Velero Backup, you can back up all objects in the cluster directly, or filter objects by type, namespace, and/or label, and in this example I use the following command to back up only all resources under the 'default' namespace:

```
velero backup create default-backup --include-namespaces <Namespace>
```

Use the `velero backup get` command to see if the backup task is complete, and when the backup task status is "Completed," the backup task is completed without any errors, as shown in the following below:

{{< figure src="https://main.qcloudimg.com/raw/eb2bbabae48b188748f5278bedf177f1.png" width="100%">}}


At this point we delete all of minio's resources, including its PVC persistence volume, as shown below:

{{< figure src="https://main.qcloudimg.com/raw/15ccaacf00640a04ae29ceed4c86195b.png" width="100%">}}

After deleting the minio resource, we can test using the previous backup to restore the deleted minio resource, and temporarily update the backup storage location to read-only mode (this prevents the backup object from being created or deleted in the backup storage location during the restore process):

```bash
kubectl patch backupstoragelocation default --namespace velero \
    --type merge \
    --patch '{"spec":{"accessMode":"ReadOnly"}}'
   
```

Modifying access to velero's storage location is "ReadOnly," as shown in the following image:

{{< figure src="https://main.qcloudimg.com/raw/e8c2ab4e5e31d1370c62fad25059a8a8.png" width="100%">}}


Now use the backup "default-backup" that Velero just created to create the restore task:

```bash
velero restore create --from-backup <BackupObject>
```

You can also use `velero restore get` to see the status of the restore task, and if the restore status is "Completed," the restore task is complete, as shown in the following image:

{{< figure src="https://main.qcloudimg.com/raw/effe8a0a7ce3aa8e422db00bfdddc375.png" width="100%">}}

When the restore is complete, you can see that the previously deleted minio-related resources have been restored successfully, as shown in the following image:

{{< figure src="https://main.qcloudimg.com/raw/1d53b0115644d43657c2a5ece805c9b4.png" width="100%">}}

Log in to minio's management page on your browser and you can see that the previously uploaded picture data is still there, indicating that the persistent volume's data was successfully restored, as shown below:

{{< figure src="https://main.qcloudimg.com/raw/ceaca9ce6bc92bdce987c63d2fe71561.png" width="100%">}}

When the restore is complete, don't forget to restore the backup storage location to read and write mode so that the next backup task can be used successfully:

```bash
kubectl patch backupstoragelocation default --namespace velero \
   --type merge \
   --patch '{"spec":{"accessMode":"ReadWrite"}}'
```



## Uninstall Velero

To uninstall velero resources in a cluster, you can do so using the following command:

```bash
kubectl delete namespace/velero clusterrolebinding/velero
kubectl delete crds -l component=velero
```



## Additional Reading

- [Official Velero Documentation](https://velero.io/docs/)
- [Tencent Cloud Documentation](https://cloud.tencent.com/document/product)

