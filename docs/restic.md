# Restic Integration

As of v0.9.0, Ark has support for backing up and restoring Kubernetes volumes using a free open-source backup tool called 
[restic][1].

Ark has always allowed you to take snapshots of persistent volumes as part of your backups, provided you’re using one of 
the supported cloud providers’ block storage offerings (Amazon EBS Volumes, Azure Managed Disks, Google Persistent Disks). 
And, since v0.6.0, we’ve had a plugin model which enables anyone to easily implement additional object and block storage 
backends, outside of the main Ark repository.

We integrated restic with Ark so that users have an out-of-the-box solution for backing up and restoring almost any type of Kubernetes
volume*. We view this as a new capability for Ark, rather than a replacement for existing functionality. If you're running on AWS, and
taking EBS snapshots as part of your regular Ark backups, great! There's no need to switch to using restic. However, if you've
been waiting for a snapshot plugin to be implemented for your storage platform, or you're using EFS, AzureFile, NFS, emptyDir, 
local, or any other volume type that doesn't have a native snapshot concept, restic may be for you.

Additionally, since restic is not tied to a specific storage platform, this integration paves the way for future work to enable
cross-volume-type data migrations. Stay tuned as this evolves!

\* hostPath volumes are not supported, but the [new local volume type][4] is!

## Setup

This setup guide assumes you already have a working Ark v0.8.1+ installation. If not, go [here][2] for instructions. It
also assumes you have the [latest release tag][3] of the Ark repo cloned locally.

### Pre-Install Steps - Upgrading from a Previous v0.9 Alpha

If you're upgrading from a previous pre-release version of Ark with restic, before you proceed you'll need to:
- manually delete all of the repositories/data from your existing restic bucket
- delete all Ark backups from your cluster using `ark backup delete`
- delete all secrets named `ark-restic-credentials` across all namespaces in your cluster

### Instructions

1. Download an updated Ark client from the [latest release][3], and move it to a location within your PATH.

1. From the Ark root directory, run the following to create new custom resource definitions:
    ```bash
    kubectl apply -f examples/common/00-prereqs.yaml
    ```

1. Run one of the following for your platform to create the daemonset:

    - AWS: `kubectl apply -f examples/aws/20-restic-daemonset.yaml`
    - Azure: `kubectl apply -f examples/azure/20-restic-daemonset.yaml`
    - GCP: `kubectl apply -f examples/gcp/20-restic-daemonset.yaml`
    - Minio: `kubectl apply -f examples/minio/30-restic-daemonset.yaml`

1. Update the image tag on the Ark daemonset and deployment to match the release version you used in Step 1 (e.g. `v0.9.0-alpha.3`):
    ```bash
    kubectl -n heptio-ark set image deployment/ark ark=gcr.io/heptio-images/ark:<RELEASE_VERSION>
    kubectl -n heptio-ark set image daemonset/restic ark=gcr.io/heptio-images/ark:<RELEASE_VERSION>
    ```

1. Create a new bucket for restic to store its data in, and give the `heptio-ark` IAM user access to it, similarly to
the main Ark bucket you've already set up. Note that for now, this must be a different bucket than the main Ark bucket,
but we plan to remove this limitation in a future release.

1. Uncomment `resticLocation` in your Ark config and set the value appropriately, then apply:
    
    - AWS: `kubectl apply -f examples/aws/00-ark-config.yaml`
    - Azure: `kubectl apply -f examples/azure/10-ark-config.yaml`
    - GCP: `kubectl apply -f examples/gcp/00-ark-config.yaml`
    - Minio: `kubectl apply -f examples/minio/10-ark-config.yaml`

    Note that `resticLocation` may either be just a bucket name, e.g. `my-restic-bucket`, or a bucket name plus a prefix under
    which you'd like the restic data to be stored, e.g. `my-restic-bucket/ark-repos`.

You're now ready to use Ark with restic.

## Backing Up

1. Run the following for each pod containing a volume that you'd like to back up using restic:
    ```bash
    kubectl -n YOUR_POD_NAMESPACE annotate pod/YOUR_POD_NAME backup.ark.heptio.com/backup-volumes=YOUR_VOLUME_NAME_1,YOUR_VOLUME_NAME_2,...
    ```

    The volume names specified in the annotation should be the names of the volumes within the pod spec. 
    
    For example, for the following pod:

    ```bash
    apiVersion: v1
    kind: Pod
    metadata:
      name: sample
      namespace: foo
    spec:
      containers:
      - image: k8s.gcr.io/test-webserver
        name: test-webserver
        volumeMounts:
        - name: pvc-volume
          mountPath: /volume-1
        - name: emptydir-volume
          mountPath: /volume-2
      volumes:
      - name: pvc-volume
        persistentVolumeClaim: 
          claimName: test-volume-claim
      - name: emptydir-volume
        emptyDir: {}
    ```

    You'd run:
    ```bash
    kubectl -n foo annotate pod/sample backup.ark.heptio.com/backup-volumes=pvc-volume,emptydir-volume
    ```


    This annotation can also be provided in the pod template spec if using a deployment, daemonset, etc.
    to manage your pods.

1. Take an Ark backup as usual:
    ```bash
    ark backup create NAME OPTIONS...
    ```

1. When the backup has completed, view information about your pod volume backups:
    ```bash
    kubectl -n heptio-ark get podvolumebackups -l ark.heptio.com/backup-name=YOUR_BACKUP_NAME -o yaml
    ```

## Restoring

Restore from your Ark backup as usual:

```bash
ark restore create --from-backup BACKUP_NAME OPTIONS...
```

## Limitations

There are several limitations that users should be aware of:

- As mentioned previously, you cannot use the main Ark bucket for storing restic backups. We plan to address this
in a future release.
- `hostPath` volumes are not supported. [Local persistent volumes][4] are, though!
- Those of you familiar with [restic][1] may know that it encrypts all of its data. We've decided to use a static, 
common encryption key for all restic repositories created by Ark. **This means that anyone who has access to your
bucket can decrypt your restic backup data**. You should ensure that you are limiting access to the restic bucket
appropriately. We plan to implement full Ark backup encryption, including securing the restic encryption keys, in 
a future release.   

## Troubleshooting

If something's not working as expected, there are several places you can go to for more information on what might
be happening.

Are your Ark server and daemonset pods running?
```bash
kubectl get pods -n heptio-ark
```

Does your restic repository exist, and is it ready?
```bash
ark restic repo get

ark restic repo get REPO_NAME -o yaml
```

Are there any errors in your Ark backup/restore?
```bash
ark backup describe BACKUP_NAME
ark backup logs BACKUP_NAME

ark restore describe RESTORE_NAME
ark restore logs RESTORE_NAME
```

What is the status of your pod volume backups/restores?
```bash
kubectl -n heptio-ark get podvolumebackups -l ark.heptio.com/backup-name=BACKUP_NAME -o yaml

kubectl -n heptio-ark get podvolumerestores -l ark.heptio.com/restore-name=RESTORE_NAME -o yaml
```

Is there any useful information in the Ark server or daemon pod logs?
```bash
kubectl -n heptio-ark logs deploy/ark
kubectl -n heptio-ark logs DAEMON_POD_NAME
```
**NOTE**: You can increase the verbosity of the pod logs by adding `--log-level=debug` as an argument
to the container command in the deployment/daemonset pod template spec.

## Details of Backup & Restore Process with Restic

This section describes in detail how backups/restores with Ark and restic work.

As background, we've introduced three new custom resource definitions and associated controllers:

- `ResticRepository` - represents/manages the lifecycle of Ark's [restic repositories][5]. Ark creates
a restic repository per namespace, when the first restic backup for a namespace is requested. The controller
for this custom resource executes restic repository lifecycle commands -- `restic init`, `restic check`,
and `restic prune`.

    You can see information about your Ark restic repos with `ark restic repo get`.

- `PodVolumeBackup` - represents a restic backup of a volume in a pod. The main Ark backup process creates
one or more of these when it finds an annotated pod. Each node in the cluster runs a controller for this
resource (in a daemonset) that handles the `PodVolumeBackups` for pods on that node. The controller executes
`restic backup` commands to backup pod volume data. 

- `PodVolumeRestore` - represents a restic restore of a pod volume. The main Ark restore process creates one
or more of these when it encounters a pod that has associated restic backups. Each node in the cluster runs a 
controller for this resource (in the same daemonset as above) that handles the `PodVolumeRestores` for pods 
on that node. The controller executes `restic restore` commands to restore pod volume data.

### Backups

1. The main Ark backup process checks each pod that it's backing up for the annotation specifying a restic backup
should be taken (`backup.ark.heptio.com/backup-volumes`)
1. When found, Ark first ensures a restic repository exists for the pod's namespace, by:
    - checking if a `ResticRepository` custom resource already exists
    - if not, creating a new one, and waiting for the `ResticRepository` controller to init/check it
1. Ark then creates a `PodVolumeBackup` custom resource per volume listed in the pod annotation
1. The main Ark process now waits for the `PodVolumeBackup` resources to complete or fail
1. Meanwhile, each `PodVolumeBackup` is handled by the controller on the appropriate node, which:
    - has a hostPath volume mount of `/var/lib/kubelet/pods` to access the pod volume data
    - finds the pod volume's subdirectory within the above volume
    - runs `restic backup`
    - updates the status of the custom resource to `Completed` or `Failed`
1. As each `PodVolumeBackup` finishes, the main Ark process captures its restic snapshot ID and adds it as an annotation
to the copy of the pod JSON that's stored in the Ark backup. This will be used for restores, as seen in the next section.


### Restores

1. The main Ark restore process checks each pod that it's restoring for annotations specifying a restic backup
exists for a volume in the pod (`snapshot.ark.heptio.com/<volume-name>`)
1. When found, Ark first ensures a restic repository exists for the pod's namespace, by:
    - checking if a `ResticRepository` custom resource already exists
    - if not, creating a new one, and waiting for the `ResticRepository` controller to init/check it (note that
    in this case, the actual repository should already exist in object storage, so the Ark controller will simply
    check it for integrity)
1. Ark adds an init container to the pod, whose job is to wait for all restic restores for the pod to complete (more
on this shortly)
1. Ark creates the pod, with the added init container, by submitting it to the Kubernetes API
1. Ark creates a `PodVolumeRestore` custom resource for each volume to be restored in the pod
1. The main Ark process now waits for each `PodVolumeRestore` resource to complete or fail
1. Meanwhile, each `PodVolumeRestore` is handled by the controller on the appropriate node, which:
    - has a hostPath volume mount of `/var/lib/kubelet/pods` to access the pod volume data
    - waits for the pod to be running the init container
    - finds the pod volume's subdirectory within the above volume
    - runs `restic restore`
    - on success, writes a file into the pod volume, in an `.ark` subdirectory, whose name is the UID of the Ark restore
    that this pod volume restore is for
    - updates the status of the custom resource to `Completed` or `Failed`
1. The init container that was added to the pod is running a process that waits until it finds a file
within each restored volume, under `.ark`, whose name is the UID of the Ark restore being run
1. Once all such files are found, the init container's process terminates successfully and the pod moves
on to running other init containers/the main containers.


[1]: https://github.com/restic/restic
[2]: https://heptio.github.io/ark/v0.8.1/cloud-common
[3]: https://github.com/heptio/ark/releases/
[4]: https://kubernetes.io/docs/concepts/storage/volumes/#local
[5]: http://restic.readthedocs.io/en/latest/100_references.html#terminology