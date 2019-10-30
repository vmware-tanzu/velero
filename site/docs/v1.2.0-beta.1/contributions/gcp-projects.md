# Restore snapshots from GCP across projects

This guide addresses confusion regarding [github issue](https://github.com/vmware-tanzu/velero/issues/1268).
These steps are heavily inspired by the [gcp documentation](https://cloud.google.com/compute/docs/images/sharing-images-across-projects).

Assume the following...

Project A [project-a]: GCP project we want to restore TO
Project B [project-b]: GCP Project we want to restore FROM

The steps below assume that you have not setup Velero yet. So make sure to skip any steps you've already completed.

## Migrate from project B to project A

- In [project-a]

  - Create serviceaccount [sa-a]

- In [project-b]

  - Create "Velero Server" IAM role [role-b] with required role permissions
  - Create serviceaccount [sa-b]
    - Assign [sa-a] with [role-b]
    - Assign [sa-b] with [role-b]
  - Create a bucket [bucket-b]
    - Assign [sa-a] "Storage Object Admin" permissions to [bucket-b]
    - Assign [sa-b] "Storage Object Admin" permissions to [bucket-b]
  - Install velero on the k8s cluster in this project with configs
    - credentials: [sa-b]
    - snapshotlocation: projectid=[project-b] and bucket=[bucket-b]
  - Create velero backup with the pvc snapshots desired [backup-b]

- In [project-a]

  - NOTE: Make sure to disable any scheduled backups.
  - Install velero on the k8s cluster in this project with configs
    - credentials: [sa-a]
    - snapshotlocation: projectid=[project-b] and bucket=[bucket-b]
  - Create velero restore [restore-a] from [backup-b]

If all was setup correctly, PVCs should be created from [project-b] snapshots.

## Enable backups on Project A

Optionally, we could enable backups for [project-a] after restore is complete.
To do so, apply the following remaining steps

- In [project-a]

  - Create "Velero Server" IAM role [role-a] with required role permissions
  - Assign [sa-a] with [role-a]
  - Create a bucket [bucket-a]
  - Assign [sa-a] "Storage Object Admin" permissions to [bucket-a]
  - Redeploy velero on the k8s cluster in this project with configs
    - credentials: [sa-a]
    - snapshotlocation: projectid=[project-a] and bucket=[bucket-a]
  - Enable scheduled backups
