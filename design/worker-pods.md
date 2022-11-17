# Worker Pods for Backup, Restore

This document proposes a new approach to executing backups and restores where each operation is run in its own "worker pod" rather than in the main Velero server pod. This approach has significant benefits for concurrency, scalability, and observability.

## Goals

- Enable multiple backups/restores to be run concurrently by running each operation in its own worker pod.
- Improve Velero's scalability by distributing work across multiple pods.
- Allow logs for in-progress backups/restores to be streamed by the user.

## Non Goals

- Adding concurrency *within* a single backup or restore (including restic).
- Creating CronJobs for scheduled backups.

## Background

Today, the Velero server runs as a Kubernetes deployment with a single replica. The deployment's one pod runs all of the Velero controllers, including the backup and restore controllers. The backup and restore controllers are responsible for doing all of the work associated with backups/restores, including interacting with the Kubernetes API server, writing to or reading from object storage, creating or restoring disk snapshots, etc. 

Because each Velero controller is configured to run with a single worker, only one backup or one restore can be executed at a time. This can lead to performance issues if a large number of backups/restores are triggered within a short time period. This issue will only be exacerbated as Velero moves towards a multi-tenancy model and better supports on-premises snapshot + backup scenarios.

## High-Level Design

Velero controllers will no longer directly execute backup/restore logic themselves (note: the rest of this document will refer only to backups, for brevity, but it applies equally to restores). Instead, when the controller is informed of a new backup custom resource, it will immediately create a new worker pod which is responsible for end-to-end processing of the backup, including validating the spec, scraping the Kubernetes API server for resources, triggering persistent volume snapshots, writing data to object storage, and updating the custom resource's status as appropriate.

A worker pod will be given a deterministic name based on the name of the backup it's executing. This will prevent Velero from inadvertently creating multiple worker pods for the same backup, since any subsequent attempts to create a pod with the specified name will fail. Additionally, velero can check Backup Storage Location if the backup exists in storage already, if so, worker pod would creation would not be attempted in the first place.

This design trivially enables running multiple backups concurrently, as each one runs in its own isolated pod and the Velero server's backup controller does not need to wait for the backup to complete before spawning a worker pod for the next one. Additionally, Velero becomes much more scalable, as the resource requirements for the Velero server itself are largely unaffacted by the number of backups. Instead, Velero's scalability becomes limited only by the total amount of resources available in the cluster to process worker pods.

## Detailed Design

### Commands for worker pods

A new hidden command will be added to the velero binary, `velero backup run BACKUP_NAME`. This command will be used solely by the worker pod. The vast majority of the backup processing logic currently in the backup controller will be moved into this new command.

`velero backup run` will accept the following flags, which will be passed down from the Velero server:

```bash
--client-burst
--client-qps
--backup-storage-location
--backup-ttl
--volume-snapshot-locations
--log-level
--unified-repo-timeout
```

`velero restore run` will accept the following flags:

```bash
--client-burst
--client-qps
--log-level
--unified-repo-timeout
--restore-resource-priorities
--terminating-resource-timeout
```

### Controller changes

Most of the logic currently in the backup controller will be moved into the `velero backup run` command. The controller will be updated so that when it's informed of a new backup custom resource, it attempts to create a new worker pod for the backup. If the create is successful or a worker pod already exists with the specified name, no further action is needed and the backup will not be re-added to the controller's work queue.

Additionally, the backup controller will be given a resync function that periodically re-enqueues all backups, to ensure that no new backups are missed.

The spec for the worker pod will look like the following:

```yaml
apiVersion: v1
kind: Pod
metadata:
  labels:
    component: velero
    velero.io/backup-name: <BACKUP_NAME>
  name: backup-<BACKUP_NAME>
  namespace: velero
  ownerReferences:
  - apiVersion: velero.io/v1
    controller: true
    kind: Backup
    name: <BACKUP_NAME>
    uid: <BACKUP_UID>
spec:
  containers:
  - args:
    - <BACKUP_NAME>
    - --client-burst=<VAL>
    - --client-qps=<VAL>
    - --default-backup-storage-location=<VAL>
    - --default-backup-ttl=<VAL>
    - --default-volume-snapshot-locations=<VAL>
    - --log-level=<VAL>
    - --restic-timeout=<VAL>
    command:
    - /velero
    - backup
    - run
    env:
    - name: VELERO_SCRATCH_DIR
      value: /scratch
    - name: AWS_SHARED_CREDENTIALS_FILE
      value: /credentials/cloud
    image: <VELERO_SERVER_IMAGE>
    imagePullPolicy: IfNotPresent
    name: velero
    volumeMounts:
    - mountPath: /plugins
      name: plugins
    - mountPath: /scratch
      name: scratch
    - mountPath: /credentials
      name: cloud-credentials
    - mountPath: /var/run/secrets/kubernetes.io/serviceaccount
      name: velero-token-plwsw
      readOnly: true
  serviceAccount: velero
  volumes:
  - emptyDir: {}
    name: plugins
  - emptyDir: {}
    name: scratch
  - name: cloud-credentials
    secret:
      defaultMode: 420
      secretName: cloud-credentials
  - name: velero-token-plwsw
    secret:
      defaultMode: 420
      secretName: velero-token-plwsw
```

### Updates to `velero backup logs`

The `velero backup logs` command will be modified so that if a backup is in progress, the logs are gotten from the worker pod's stdout (essentially proxying to `kubectl -n velero logs POD_NAME`). A  `-f/--follow` flag will be added to allow streaming of logs for an in-progress backup.

Once a backup is complete, the logs will continue to be uploaded to object storage, and `velero backup logs` will still fetch them from there.

### Changes to backup deletion controller

The Velero server keeps a record of current in-progress backups and disallows them from being deleted. This will need to be updated to account for backups running in worker pods. The most sensible approach is for the backup deletion controller to first check the backup's status via the API, and to allow deletion to proceed if it's not new/in-progress. If the backup is new or in-progress, the controller can check to see if a worker pod is running for the backup. If so, the deletion is disallowed because the backup is actively being processed. If there is no worker pod for the backup, the backup can be deleted as it's not actively being processed.

### Open items

- Given that multiple backups/restores could be running concurrently, we need to consider possible areas of contention/conflict between jobs, including (but not limited to):
  - exec hooks (i.e. don't want to run `fsfreeze` twice on the same pod)
  - unified-repo backups and restores on the same volume
- It probably makes sense to use Kubernetes Jobs to control worker pods, rather than directly creating "bare pods". The Job will ensure that a worker pod successfully runs to completion if the job pods can handle parallel jobs. Otherwise using Kubernetes Jobs can become problematic. [from k8s](https://kubernetes.io/docs/concepts/workloads/controllers/job/#handling-pod-and-container-failures)
> Note that even if you specify .spec.parallelism = 1 and .spec.completions = 1 and .spec.template.spec.restartPolicy = "Never", the same program may sometimes be started twice.
- Currently, unified-repo repository lock management is handled by an in-process lock manager in the Velero server. In order for backups/restores to safely run concurrently, the design for unified-repo lock management needs to change. There is an [open issue](https://github.com/heptio/velero/issues/1540) for this which is currently not prioritized.
- There are several prometheus metrics that are emitted as part of the backup process. Since backups will no longer be running in the Velero server, we need to find a way to expose those values. One option is to store any value that would feed a metric as a field on the backup's `status`, and to have the Velero server scrape values from there for completed backups. Another option is to use the [Prometheus push gateway](https://prometheus.io/docs/practices/pushing/).
- Over time, many completed worker pods will exist in the `velero` namespace. We need to consider whether this poses any issue and whether we should garbage-collect them more aggressively.

## Alternatives Considered

### Enable >1 worker per controller

One alternative option is to simply increase the number of workers being used per controller. This would, in theory, allow multiple backups/restores to be processed concurrently by the Velero server. The downsides to this option are that (1) this doesn't do anything to improve Velero's scalability, since all of the work is still being done within a single pod/process, and (2) it doesn't easily enable log streaming for in-progress backups.

### Run multiple replicas of the Velero server

Another option is to increase the number of replicas in the Velero deployment, so that multiple instances of the Velero server pod are running. This would require adding some sort of "leader-election" process for each backup to ensure that only one replica was processing it. It also would result in multiple replicas of the non-core controllers (backup deletion controller, GC controller, download request controller, etc.), which would require modifications to ensure there wasn't any duplicate processing. Additionally, this approach also doesn't easily enable log streaming for in-progress backups.

## Security Considerations

Each worker pod will be created in the `velero` namespace, will use the `velero` service account for identity, and will have the `cloud-credentials` secret mounted as a volume. As a result the worker pods will have the same level of privilege as the Velero server pod.
