# Restic Integration

Ark now has support for backups/restores of pod volume data using [restic][1], an open-source tool for doing
backups and restores of filesystem data. This enables you to take backups of additional Kubernetes volume 
types beyond those with snapshot APIs that are integrated with Ark (i.e. AWS EBS, GCP PD, Azure PD). It also lays 
the foundation for future work to support cross-cloud stateful migrations. Note that the details of this feature,
including names, commands, etc., may change as we receive feedback and refine our implementation. 

Two new Ark custom resources have been created to support this feature: `PodVolumeBackup` and `PodVolumeRestore`.
Additionally, a new Ark daemonset has been created that runs two controllers, one for each of the two new CRDs, on
each node in the cluster. When an Ark backup is created that includes pods annotated for restic backup, the main Ark 
backup controller will create a `PodVolumeBackup` custom resource that's owned by the `Backup`. The pod volume backup
controller running on the pod's node will observe the new custom resource, and will run a restic backup of the volume 
(accessing the volume's data via a hostPath mount of `/var/lib/kubelet/pods`). The main Ark backup controller will 
wait for the `PodVolumeBackup` to complete before completing the Ark backup. Restores proceed similarly with some 
minor differences to account for the fact that a new pod/volume is being created.

## Setup

This setup guide assumes you already have a working Ark v0.8.1+ installation. If not, go [here][2] for instructions.

1. From the Ark root directory, run the following to create new custom resource definitions:
```bash
kubectl apply -f examples/common/00-prereqs.yaml
```

2. Run one of the following for your platform to create the daemonset:

    - AWS: `kubectl apply -f examples/aws/20-restic-daemonset.yaml`
    - Azure: `kubectl apply -f examples/azure/20-restic-daemonset.yaml`
    - GCP: `kubectl apply -f examples/gcp/20-restic-daemonset.yaml`
    - Minio: `kubectl apply -f examples/minio/30-restic-daemonset.yaml`

3. Use the `master` image tag for both the Ark deployment and daemonset:
```bash
kubectl -n heptio-ark set image deployment/ark ark=gcr.io/heptio-images/ark:master
kubectl -n heptio-ark set image daemonset/restic ark=gcr.io/heptio-images/ark:master
```

4. Create a new bucket for restic to store its data in, and give the `heptio-ark` IAM user access to it, similarly to
the main Ark bucket you've already set up.

5. Update the Ark config to specify the restic bucket:
```bash
kubectl -n heptio-ark get config default -o json | \
jq '.backupStorageProvider.resticLocation = "YOUR_RESTIC_BUCKET_NAME"' |\
kubectl apply -f -
```

6. For each namespace that has pod volumes to be backed up using restic, configure a restic encryption key using
one of the following commands:

```bash
# provide the encryption key on the command line
ark restic init-repository --namespace YOUR_NAMESPACE --key-data YOUR_ENCRYPTION_KEY
```

```bash
# provide the encryption key via file
ark restic init-repository --namespace YOUR_NAMESPACE --key-file YOUR_ENCRYPTION_KEY_FILE
```

```bash
# have Ark generate a random encryption key
ark restic init-repository --namespace YOUR_NAMESPACE --key-size ENCRYPTION_KEY_SIZE
```

**IMPORTANT**: store this key safely and securely. All restic backup data is encrypted and cannot be accessed
without this key. We will be adding support for key rotation shortly.

## Run

1. Run the following for each pod containing a volume that you'd like to backup using restic:
```bash
kubectl -n YOUR_POD_NAMESPACE annotate pod/YOUR_POD_NAME backup.ark.heptio.com/backup-volumes=YOUR_VOLUME_NAME_1,YOUR_VOLUME_NAME_2,...
```

Note that this annotation can also be provided in the pod template spec if using a deployment, daemonset, etc.
to manage your pods.

2. Take an Ark backup as usual:
```bash
ark backup create NAME OPTIONS...
```

3. When the backup has completed, view information about your pod volume backups:
```bash
kubectl -n heptio-ark get podvolumebackups -l ark.heptio.com/backup-name=YOUR_BACKUP_NAME -o yaml
```

[1]: https://github.com/restic/restic
[2]: https://heptio.github.io/ark/v0.8.1/cloud-common