# Restore Hooks

This document proposes a solution that allows a user to specify Restore Hooks, much like Backup Hooks, that can be executed during the restore process.

## Goals

- Enable custom commands to be run during a restore in order to mirror the commands that are available to the backup process.
- Provide observability into the result of commands run in restored pods.

## Non Goals

- Handling any application specific scenarios (postgres, mongo, etc)

## Background

Velero supports Backup Hooks to execute commands before and/or after a backup.
This enables a user to, among other things, prepare data to be backed up without having to freeze an in-use volume.
An example of this would be to attach an empty volume to a Postgres pod, use a backup hook to execute `pg_dump` from the data volume, and back up the volume containing the export.
The problem is that there's no easy or automated way to include an automated restore process.
After a restore with the example configuration above, the postgres pod will be empty, but there will be a need to manually exec in and run `pg_restore`.

## High-Level Design

The Restore spec will have a `spec.hooks` section matching the same section on the Backup spec except no `pre` hooks can be defined - only `post`.
Annotations comparable to the annotations used during backup can also be set on pods.
For each restored pod, the Velero server will check if there are any hooks applicable to the pod.
If a restored pod has any applicable hooks, Velero will wait for the container where the hook is to be executed to reach status Running.
The Restore log will include the results of each post-restore hook and the Restore object status will incorporate the results of hooks.
The Restore log will include the results of each hook and the Restore object status will incorporate the results of hooks.

A new section at `spec.hooks.resources.initContainers` will allow for injecting initContainers into restored pods.
Annotations can be set as an alternative to defining the initContainers in the Restore object.

## Detailed Design

Post-restore hooks can be defined by annotation and/or by an array of resource hooks in the Restore spec.

The following annotations are supported:
- post.hook.restore.velero.io/container
- post.hook.restore.velero.io/command
- post.hook.restore.velero.io/on-error
- post.hook.restore.velero.io/exec-timeout
- post.hook.restore.velero.io/wait-timeout

Init restore hooks can be defined by annotation and/or in the new `initContainers` section in the Restore spec.
The initContainers schema is `pod.spec.initContainers`.

The following annotations are supported:
- init.hook.restore.velero.io/timeout
- init.hook.restore.velero.io/initContainers

This is an example of defining hooks in the Restore spec.

```yaml
apiVersion: velero.io/v1
kind: Restore
spec:
  ...
  hooks:
    resources:
      -
        name: my-hook
        includedNamespaces:
        - '*'
        excludedNamespaces:
        - some-namespace
        includedResources:
        - pods
        excludedResources: []
        labelSelector:
          matchLabels:
            app: velero
            component: server
        post:
          -
            exec:
              container: postgres
              command:
                - /bin/bash
                - -c
                - rm /docker-entrypoint-initdb.d/dump.sql
              onError: Fail
              timeout: 10s
              readyTimeout: 60s
        init:
          timeout: 120s
          initContainers:
          - name: restore
            image: postgres:12
            command: ["/bin/bash", "-c", "mv /backup/dump.sql /docker-entrypoint-initdb.d/"]
            volumeMounts:
            - name: backup
              mountPath: /backup
```

As with Backups, if an annotation is defined on a pod then no hooks from the Restore spec will be applied.

### Implementation

The types and function in pkg/backup/item_hook_handler.go will be moved to a new package (pkg/hooks) and exported so they can be used for both backups and restores.

The post-restore hooks implementation will closely follow the design of restoring pod volumes with restic.
The pkg/restore.context type will have new fields `hooksWaitGroup` and `hooksErrs` comparable to `resticWaitGroup` and `resticErr`.
The pkg/restore.context.execute function will start a goroutine for each pod with applicable hooks and then continue with restoring other items.
Each hooks goroutine will create a pkg/util/hooks.ItemHookHandler for each pod and send any error on the context.hooksErrs channel.
The ItemHookHandler already includes stdout and stderr and other metadata in the Backup log so the same logs will automatically be added to the Restore log (passed as the first argument to the ItemHookhandler.HandleHooks method.)

The pkg/restore.context.execute function will wait for the hooksWaitGroup before returning.
Any errors received on context.hooksErrs will be added to errs.Velero.

One difference compared to the restic restore design is that any error on the context.hooksErrs channel will cancel the context of all hooks, since errors are only reported on this channel if the hook specified `onError: Fail`.
However, canceling the hooks goroutines will not cancel the restic goroutines.
In practice the restic goroutines will complete before the hooks since the hooks do not run until a pod is ready, but it's possible a hook will be executed and fail while a different pod is still in the pod volume restore phase.

Failed hooks with `onError: Continue` will appear in the Restore log but will not affect the status of the parent Restore.
Failed hooks with `onError: Fail` will cause the parent Restore to have status Partially Failed.

If initContainers are specified for a pod, Velero will inject the containers into the beginning of the pod's initContainers list.
If a restic initContainer is also being injected, the restore initContainers will be injected directly after the restic initContainer.
The restore will use a RestoreItemAction to inject the initContainers.
Stdout and stderr of the restore initContainers will not be added to the Restore logs.
InitContainers that fail will not affect the parent Restore's status.

## Alternatives Considered

Wait for all restored Pods to report Ready, then execute the first hook in all applicable Pods simultaneously, then proceed to the next hook, etc.
That could introduce deadlock, e.g. if an API pod cannot be ready until the DB pod is restored.

Put the restore hooks on the Backup spec as a third lifecycle event named `restore` along with `pre` and `post`.
That would be confusing since `pre` and `post` would appear in the Backup log but `restore` would only be in the Restore log.

Execute restore hooks in parallel for each Pod.
That would not match the behavior of Backups.

Wait for PodStatus ready before executing the post-restore hooks in any container.
There are cases where the pod should not report itself ready until after the restore hook has run.

Include the logs from initContainers in the Restore log.
Unlike exec hooks where stdout and stderr are permanently lost if not added to the Restore log, the logs of the injected initContainers are available through the K8s API with kubectl or another client.

## Security Considerations

Stdout or stderr in the Restore log may contain sensitive information, but the same risk already exists for Backup hooks.
