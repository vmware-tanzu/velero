---
title: "Restic Integration"
layout: docs
---

Velero has support for backing up and restoring Kubernetes volumes using a free open-source backup tool called [restic][1]. This support is considered beta quality. Please see the list of [limitations](#limitations) to understand if it currently fits your use case.

Velero has always allowed you to take snapshots of persistent volumes as part of your backups if you’re using one of
the supported cloud providers’ block storage offerings (Amazon EBS Volumes, Azure Managed Disks, Google Persistent Disks).
We also provide a plugin model that enables anyone to implement additional object and block storage backends, outside the
main Velero repository.

We integrated restic with Velero so that users have an out-of-the-box solution for backing up and restoring almost any type of Kubernetes
volume*. This is a new capability for Velero, not a replacement for existing functionality. If you're running on AWS, and
taking EBS snapshots as part of your regular Velero backups, there's no need to switch to using restic. However, if you've
been waiting for a snapshot plugin for your storage platform, or if you're using EFS, AzureFile, NFS, emptyDir,
local, or any other volume type that doesn't have a native snapshot concept, restic might be for you.

Restic is not tied to a specific storage platform, which means that this integration also paves the way for future work to enable
cross-volume-type data migrations. Stay tuned as this evolves!

\* hostPath volumes are not supported, but the [new local volume type][4] is supported.

## Setup

### Prerequisites

- Velero's restic integration requires the Kubernetes [MountPropagation feature][6], which is enabled by default in Kubernetes v1.10.0 and later.

### Instructions

Ensure you've [downloaded latest release][3].

To install restic, use the `--use-restic` flag on the `velero install` command. See the [install overview][2] for more details. When using restic on a storage provider that doesn't currently have Velero support for snapshots, the `--use-volume-snapshots=false` flag prevents an unused `VolumeSnapshotLocation` from being created on installation.

Please note: For some PaaS/CaaS platforms based on Kubernetes such as RancherOS, OpenShift and Enterprise PKS, some modifications are required to the restic DaemonSet spec.

**RancherOS**

The host path for volumes is not `/var/lib/kubelet/pods`, rather it is `/opt/rke/var/lib/kubelet/pods`

```yaml
hostPath:
  path: /var/lib/kubelet/pods
```

to

```yaml
hostPath:
  path: /opt/rke/var/lib/kubelet/pods
```

**OpenShift**

The restic containers should be running in a `privileged` mode to be able to mount the correct hostpath to pods volumes.

1. Add the `velero` ServiceAccount to the `privileged` SCC:

    ```
    $ oc adm policy add-scc-to-user privileged -z velero -n velero
    ```

2. For OpenShift version  >= `4.1`, Modify the DaemonSet yaml to request a privileged mode:

    ```diff
    @@ -67,3 +67,5 @@ spec:
                  value: /credentials/cloud
                - name: VELERO_SCRATCH_DIR
                  value: /scratch
    +          securityContext:
    +            privileged: true
    ```

    or

    ```shell
    oc patch ds/restic \
      --namespace velero \
      --type json \
      -p '[{"op":"add","path":"/spec/template/spec/containers/0/securityContext","value": { "privileged": true}}]'
    ```

3. For OpenShift version  < `4.1`, Modify the DaemonSet yaml to request a privileged mode and mount the correct hostpath to pods volumes.

    ```diff
    @@ -35,7 +35,7 @@ spec:
                secretName: cloud-credentials
            - name: host-pods
              hostPath:
    -            path: /var/lib/kubelet/pods
    +            path: /var/lib/origin/openshift.local.volumes/pods
            - name: scratch
              emptyDir: {}
          containers:
    @@ -67,3 +67,5 @@ spec:
                  value: /credentials/cloud
                - name: VELERO_SCRATCH_DIR
                  value: /scratch
    +          securityContext:
    +            privileged: true
    ```

    or 

    ```shell
    oc patch ds/restic \
      --namespace velero \
      --type json \
      -p '[{"op":"add","path":"/spec/template/spec/containers/0/securityContext","value": { "privileged": true}}]'

    oc patch ds/restic \
      --namespace velero \
      --type json \
      -p '[{"op":"replace","path":"/spec/template/spec/volumes/0/hostPath","value": { "path": "/var/lib/origin/openshift.local.volumes/pods"}}]'
    ```


If restic is not running in a privileged mode, it will not be able to access pods volumes within the mounted hostpath directory because of the default enforced SELinux mode configured in the host system level. You can [create a custom SCC](https://docs.openshift.com/container-platform/3.11/admin_guide/manage_scc.html) in order to relax the security in your cluster so that restic pods are allowed to use the hostPath volume plug-in without granting them access to the `privileged` SCC.

By default a userland openshift namespace will not schedule pods on all nodes in the cluster.

To schedule on all nodes the namespace needs an annotation:

```
oc annotate namespace <velero namespace> openshift.io/node-selector=""
```

This should be done before velero installation.

Or the ds needs to be deleted and recreated:

```
oc get ds restic -o yaml -n <velero namespace> > ds.yaml
oc annotate namespace <velero namespace> openshift.io/node-selector=""
oc create -n <velero namespace> -f ds.yaml
```

**Enterprise PKS**

You need to enable the `Allow Privileged` option in your plan configuration so that restic is able to mount the hostpath.

The hostPath should be changed from `/var/lib/kubelet/pods` to `/var/vcap/data/kubelet/pods`

```yaml
hostPath:
  path: /var/vcap/data/kubelet/pods
```

**Microsoft Azure**

If you are using [Azure Files][8], you need to add `nouser_xattr` to your storage class's `mountOptions`. See [this restic issue][9] for more details.

You can use the following command to patch the storage class:

```bash
kubectl patch storageclass/<YOUR_AZURE_FILE_STORAGE_CLASS_NAME> \
  --type json \
  --patch '[{"op":"add","path":"/mountOptions/-","value":"nouser_xattr"}]'
```

You're now ready to use Velero with restic.

## Back up

1. Run the following for each pod that contains a volume to back up:

    ```bash
    kubectl -n YOUR_POD_NAMESPACE annotate pod/YOUR_POD_NAME backup.velero.io/backup-volumes=YOUR_VOLUME_NAME_1,YOUR_VOLUME_NAME_2,...
    ```

    where the volume names are the names of the volumes in the pod spec.

    For example, for the following pod:

    ```yaml
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
    kubectl -n foo annotate pod/sample backup.velero.io/backup-volumes=pvc-volume,emptydir-volume
    ```

    This annotation can also be provided in a pod template spec if you use a controller to manage your pods.

1. Take a Velero backup:

    ```bash
    velero backup create NAME OPTIONS...
    ```

1. When the backup completes, view information about the backups:

    ```bash
    velero backup describe YOUR_BACKUP_NAME
    ```
    ```bash
    kubectl -n velero get podvolumebackups -l velero.io/backup-name=YOUR_BACKUP_NAME -o yaml
    ```

## Restore

1. Restore from your Velero backup:

    ```bash
    velero restore create --from-backup BACKUP_NAME OPTIONS...
    ```

1. When the restore completes, view information about your pod volume restores:

    ```bash
    velero restore describe YOUR_RESTORE_NAME
    ```
    ```bash
    kubectl -n velero get podvolumerestores -l velero.io/restore-name=YOUR_RESTORE_NAME -o yaml
    ```

## Limitations

- `hostPath` volumes are not supported. [Local persistent volumes][4] are supported.
- Those of you familiar with [restic][1] may know that it encrypts all of its data. We've decided to use a static,
common encryption key for all restic repositories created by Velero. **This means that anyone who has access to your
bucket can decrypt your restic backup data**. Make sure that you limit access to the restic bucket
appropriately. We plan to implement full Velero backup encryption, including securing the restic encryption keys, in
a future release.
- An incremental backup chain will be maintained across pod reschedules for PVCs. However, for pod volumes that are *not*
PVCs, such as `emptyDir` volumes, when a pod is deleted/recreated (e.g. by a ReplicaSet/Deployment), the next backup of those
volumes will be full rather than incremental, because the pod volume's lifecycle is assumed to be defined by its pod.
- Restic scans each file in a single thread. This means that large files (such as ones storing a database) will take a long time to scan for data deduplication, even if the actual
difference is small.

## Customize Restore Helper Container

Velero uses a helper init container when performing a restic restore. By default, the image for this container is `velero/velero-restic-restore-helper:<VERSION>`,
where `VERSION` matches the version/tag of the main Velero image. You can customize the image that is used for this helper by creating a ConfigMap in the Velero namespace with
the alternate image. 

In addition, you can customize the resource requirements for the init container, should you need.

The ConfigMap must look like the following:

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  # any name can be used; Velero uses the labels (below)
  # to identify it rather than the name
  name: restic-restore-action-config
  # must be in the velero namespace
  namespace: velero
  # the below labels should be used verbatim in your
  # ConfigMap.
  labels:
    # this value-less label identifies the ConfigMap as
    # config for a plugin (i.e. the built-in restic restore
    # item action plugin)
    velero.io/plugin-config: ""
    # this label identifies the name and kind of plugin
    # that this ConfigMap is for.
    velero.io/restic: RestoreItemAction
data:
  # The value for "image" can either include a tag or not;
  # if the tag is *not* included, the tag from the main Velero
  # image will automatically be used.
  image: myregistry.io/my-custom-helper-image[:OPTIONAL_TAG]

  # "cpuRequest" sets the request.cpu value on the restic init containers during restore.
  # If not set, it will default to "100m". A value of "0" is treated as unbounded.
  cpuRequest: 200m
  
  # "memRequest" sets the request.memory value on the restic init containers during restore.
  # If not set, it will default to "128Mi". A value of "0" is treated as unbounded.
  memRequest: 128Mi

  # "cpuLimit" sets the request.cpu value on the restic init containers during restore.
  # If not set, it will default to "100m". A value of "0" is treated as unbounded.
  cpuLimit: 200m
  
  # "memLimit" sets the request.memory value on the restic init containers during restore.
  # If not set, it will default to "128Mi". A value of "0" is treated as unbounded.
  memLimit: 128Mi


```

## Troubleshooting

Run the following checks:

Are your Velero server and daemonset pods running?

```bash
kubectl get pods -n velero
```

Does your restic repository exist, and is it ready?

```bash
velero restic repo get

velero restic repo get REPO_NAME -o yaml
```

Are there any errors in your Velero backup/restore?

```bash
velero backup describe BACKUP_NAME
velero backup logs BACKUP_NAME

velero restore describe RESTORE_NAME
velero restore logs RESTORE_NAME
```

What is the status of your pod volume backups/restores?

```bash
kubectl -n velero get podvolumebackups -l velero.io/backup-name=BACKUP_NAME -o yaml

kubectl -n velero get podvolumerestores -l velero.io/restore-name=RESTORE_NAME -o yaml
```

Is there any useful information in the Velero server or daemon pod logs?

```bash
kubectl -n velero logs deploy/velero
kubectl -n velero logs DAEMON_POD_NAME
```

**NOTE**: You can increase the verbosity of the pod logs by adding `--log-level=debug` as an argument
to the container command in the deployment/daemonset pod template spec.

## How backup and restore work with restic

We introduced three custom resource definitions and associated controllers:

- `ResticRepository` - represents/manages the lifecycle of Velero's [restic repositories][5]. Velero creates
a restic repository per namespace when the first restic backup for a namespace is requested. The controller
for this custom resource executes restic repository lifecycle commands -- `restic init`, `restic check`,
and `restic prune`.

    You can see information about your Velero restic repositories by running `velero restic repo get`.

- `PodVolumeBackup` - represents a restic backup of a volume in a pod. The main Velero backup process creates
one or more of these when it finds an annotated pod. Each node in the cluster runs a controller for this
resource (in a daemonset) that handles the `PodVolumeBackups` for pods on that node. The controller executes
`restic backup` commands to backup pod volume data.

- `PodVolumeRestore` - represents a restic restore of a pod volume. The main Velero restore process creates one
or more of these when it encounters a pod that has associated restic backups. Each node in the cluster runs a
controller for this resource (in the same daemonset as above) that handles the `PodVolumeRestores` for pods
on that node. The controller executes `restic restore` commands to restore pod volume data.

### Backup

1. The main Velero backup process checks each pod that it's backing up for the annotation specifying a restic backup
should be taken (`backup.velero.io/backup-volumes`)
1. When found, Velero first ensures a restic repository exists for the pod's namespace, by:
    - checking if a `ResticRepository` custom resource already exists
    - if not, creating a new one, and waiting for the `ResticRepository` controller to init/check it
1. Velero then creates a `PodVolumeBackup` custom resource per volume listed in the pod annotation
1. The main Velero process now waits for the `PodVolumeBackup` resources to complete or fail
1. Meanwhile, each `PodVolumeBackup` is handled by the controller on the appropriate node, which:
    - has a hostPath volume mount of `/var/lib/kubelet/pods` to access the pod volume data
    - finds the pod volume's subdirectory within the above volume
    - runs `restic backup`
    - updates the status of the custom resource to `Completed` or `Failed`
1. As each `PodVolumeBackup` finishes, the main Velero process adds it to the Velero backup in a file named `<backup-name>-podvolumebackups.json.gz`. This file gets uploaded to object storage alongside the backup tarball. It will be used for restores, as seen in the next section.

### Restore

1. The main Velero restore process checks each existing `PodVolumeBackup` custom resource in the cluster to backup from.
1. For each `PodVolumeBackup` found, Velero first ensures a restic repository exists for the pod's namespace, by:
    - checking if a `ResticRepository` custom resource already exists
    - if not, creating a new one, and waiting for the `ResticRepository` controller to init/check it (note that
    in this case, the actual repository should already exist in object storage, so the Velero controller will simply
    check it for integrity)
1. Velero adds an init container to the pod, whose job is to wait for all restic restores for the pod to complete (more
on this shortly)
1. Velero creates the pod, with the added init container, by submitting it to the Kubernetes API
1. Velero creates a `PodVolumeRestore` custom resource for each volume to be restored in the pod
1. The main Velero process now waits for each `PodVolumeRestore` resource to complete or fail
1. Meanwhile, each `PodVolumeRestore` is handled by the controller on the appropriate node, which:
    - has a hostPath volume mount of `/var/lib/kubelet/pods` to access the pod volume data
    - waits for the pod to be running the init container
    - finds the pod volume's subdirectory within the above volume
    - runs `restic restore`
    - on success, writes a file into the pod volume, in a `.velero` subdirectory, whose name is the UID of the Velero restore
    that this pod volume restore is for
    - updates the status of the custom resource to `Completed` or `Failed`
1. The init container that was added to the pod is running a process that waits until it finds a file
within each restored volume, under `.velero`, whose name is the UID of the Velero restore being run
1. Once all such files are found, the init container's process terminates successfully and the pod moves
on to running other init containers/the main containers.

## 3rd party controllers

### Monitor backup annotation

Velero does not currently provide a mechanism to detect persistent volume claims that are missing the restic backup annotation.

To solve this, a controller was written by Thomann Bits&Beats: [velero-pvc-watcher][7]

### Add backup annotation

Velero does not currently provide a single command or automatic way to backup all volume resources in the cluster without annotating pods or pod templates.

The [velero-volume-controller][10] written by duyanghao helps to solve this problem by adding backup annotation to pods with volumes automatically.

[1]: https://github.com/restic/restic
[2]: customize-installation.md#enable-restic-integration
[3]: https://github.com/vmware-tanzu/velero/releases/
[4]: https://kubernetes.io/docs/concepts/storage/volumes/#local
[5]: http://restic.readthedocs.io/en/latest/100_references.html#terminology
[6]: https://kubernetes.io/docs/concepts/storage/volumes/#mount-propagation
[7]: https://github.com/bitsbeats/velero-pvc-watcher
[8]: https://docs.microsoft.com/en-us/azure/aks/azure-files-dynamic-pv
[9]: https://github.com/restic/restic/issues/1800
[10]: https://github.com/duyanghao/velero-volume-controller
