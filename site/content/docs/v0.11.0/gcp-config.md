---
title: "Run Velero on GCP"
layout: docs
---

You can run Kubernetes on Google Cloud Platform in either: 

* Kubernetes on Google Compute Engine virtual machines
* Google Kubernetes Engine 

If you do not have the `gcloud` and `gsutil` CLIs locally installed, follow the [user guide][16] to set them up.

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

## Create GCS bucket

Velero requires an object storage bucket in which to store backups, preferably unique to a single Kubernetes cluster (see the [FAQ][20] for more details). Create a GCS bucket, replacing the <YOUR_BUCKET> placeholder with the name of your bucket:

```bash
BUCKET=<YOUR_BUCKET>

gsutil mb gs://$BUCKET/
```

## Create service account

To integrate Velero with GCP, create an Velero-specific [Service Account][15]:

1. View your current config settings:

    ```bash
    gcloud config list
    ```

    Store the `project` value from the results in the environment variable `$PROJECT_ID`.
    
    ```bash
    PROJECT_ID=$(gcloud config get-value project)
    ```

2. Create a service account:

    ```bash
    gcloud iam service-accounts create velero \
        --display-name "Velero service account"
    ```

    > If you'll be using Velero to backup multiple clusters with multiple GCS buckets, it may be desirable to create a unique username per cluster rather than the default `velero`.

    Then list all accounts and find the `velero` account you just created:
    ```bash
    gcloud iam service-accounts list
    ```

    Set the `$SERVICE_ACCOUNT_EMAIL` variable to match its `email` value.
    
    ```bash
    SERVICE_ACCOUNT_EMAIL=$(gcloud iam service-accounts list \
      --filter="displayName:Velero service account" \
      --format 'value(email)')
    ```

3. Attach policies to give `velero` the necessary permissions to function:

    ```bash
    
    ROLE_PERMISSIONS=(
        compute.disks.get
        compute.disks.create
        compute.disks.createSnapshot
        compute.snapshots.get
        compute.snapshots.create
        compute.snapshots.useReadOnly
        compute.snapshots.delete
        compute.zones.get
    )

    gcloud iam roles create velero.server \
        --project $PROJECT_ID \
        --title "Velero Server" \
        --permissions "$(IFS=","; echo "${ROLE_PERMISSIONS[*]}")"    

    gcloud projects add-iam-policy-binding $PROJECT_ID \
        --member serviceAccount:$SERVICE_ACCOUNT_EMAIL \
        --role projects/$PROJECT_ID/roles/velero.server

    gsutil iam ch serviceAccount:$SERVICE_ACCOUNT_EMAIL:objectAdmin gs://${BUCKET}
    ```

4. Create a service account key, specifying an output file (`credentials-velero`) in your local directory:

    ```bash
    gcloud iam service-accounts keys create credentials-velero \
        --iam-account $SERVICE_ACCOUNT_EMAIL
    ```

## Credentials and configuration

If you run Google Kubernetes Engine (GKE), make sure that your current IAM user is a cluster-admin. This role is required to create RBAC objects.
See [the GKE documentation][22] for more information.

In the Velero directory (i.e. where you extracted the release tarball), run the following to first set up namespaces, RBAC, and other scaffolding. To run in a custom namespace, make sure that you have edited the YAML files to specify the namespace. See [Run in custom namespace][0].

```bash
kubectl apply -f config/common/00-prereqs.yaml
```

Create a Secret. In the directory of the credentials file you just created, run:

```bash
kubectl create secret generic cloud-credentials \
    --namespace velero \
    --from-file cloud=credentials-velero
```

**Note: If you use a custom namespace, replace `velero` with the name of the custom namespace**

Specify the following values in the example files:

* In file `config/gcp/05-backupstoragelocation.yaml`:

  * Replace `<YOUR_BUCKET>`. See the [BackupStorageLocation definition][7] for details.

* (Optional) If you run the nginx example, in file `config/nginx-app/with-pv.yaml`:

    * Replace `<YOUR_STORAGE_CLASS_NAME>` with `standard`. This is GCP's default `StorageClass` name.

* (Optional, use only if you need to specify multiple volume snapshot locations) In `config/gcp/10-deployment.yaml`:

  * Uncomment the `--default-volume-snapshot-locations` and replace provider locations with the values for your environment.

## Start the server

In the root of your Velero directory, run:

  ```bash
  kubectl apply -f config/gcp/05-backupstoragelocation.yaml
  kubectl apply -f config/gcp/06-volumesnapshotlocation.yaml
  kubectl apply -f config/gcp/10-deployment.yaml
  ```

  [0]: namespace.md
  [7]: api-types/backupstoragelocation.md#gcp
  [15]: https://cloud.google.com/compute/docs/access/service-accounts
  [16]: https://cloud.google.com/sdk/docs/
  [20]: faq.md
  [22]: https://cloud.google.com/kubernetes-engine/docs/how-to/role-based-access-control#prerequisites_for_using_role-based_access_control

