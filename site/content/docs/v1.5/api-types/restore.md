---
title: "Restore API Type"
layout: docs
---

## Use

The `Restore` API type is used as a request for the Velero server to perform a Restore. Once created, the
Velero Server immediately starts the Restore process.

## API GroupVersion

Restore belongs to the API group version `velero.io/v1`.

## Definition

Here is a sample `Restore` object with each of the fields documented:

```yaml
# Standard Kubernetes API Version declaration. Required.
apiVersion: velero.io/v1
# Standard Kubernetes Kind declaration. Required.
kind: Restore
# Standard Kubernetes metadata. Required.
metadata:
  # Restore name. May be any valid Kubernetes object name. Required.
  name: a-very-special-backup-0000111122223333
  # Restore namespace. Must be the namespace of the Velero server. Required.
  namespace: velero
# Parameters about the restore. Required.
spec:
  # BackupName is the unique name of the Velero backup to restore from.
  backupName: a-very-special-backup
  # Array of namespaces to include in the restore. If unspecified, all namespaces are included.
  # Optional.
  includedNamespaces:
  - '*'
  # Array of namespaces to exclude from the restore. Optional.
  excludedNamespaces:
  - some-namespace
  # Array of resources to include in the restore. Resources may be shortcuts (for example 'po' for 'pods')
  # or fully-qualified. If unspecified, all resources are included. Optional.
  includedResources:
  - '*'
  # Array of resources to exclude from the restore. Resources may be shortcuts (for example 'po' for 'pods')
  # or fully-qualified. Optional.
  excludedResources:
  - storageclasses.storage.k8s.io
  # Whether or not to include cluster-scoped resources. Valid values are true, false, and
  # null/unset. If true, all cluster-scoped resources are included (subject to included/excluded
  # resources and the label selector). If false, no cluster-scoped resources are included. If unset,
  # all cluster-scoped resources are included if and only if all namespaces are included and there are
  # no excluded namespaces. Otherwise, if there is at least one namespace specified in either
  # includedNamespaces or excludedNamespaces, then the only cluster-scoped resources that are backed
  # up are those associated with namespace-scoped resources included in the restore. For example, if a
  # PersistentVolumeClaim is included in the restore, its associated PersistentVolume (which is
  # cluster-scoped) would also be backed up.
  includeClusterResources: null
  # Individual objects must match this label selector to be included in the restore. Optional.
  labelSelector:
    matchLabels:
      app: velero
      component: server
  # NamespaceMapping is a map of source namespace names to
  # target namespace names to restore into. Any source namespaces not
  # included in the map will be restored into namespaces of the same name.
  namespaceMapping:
    namespace-backup-from: namespace-to-restore-to
  # RestorePVs specifies whether to restore all included PVs
  # from snapshot (via the cloudprovider).
  restorePVs: true
  # ScheduleName is the unique name of the Velero schedule
  # to restore from. If specified, and BackupName is empty, Velero will
  # restore from the most recent successful backup created from this schedule.
  scheduleName: my-scheduled-backup-name
  # Actions to perform during or post restore. The only hooks currently supported are
  # adding an init container to a pod before it can be restored and executing a command in a
  # restored pod's container. Optional.
  hooks:
    # Array of hooks that are applicable to specific resources. Optional.
    resources:
    # Name is the name of this hook.
    - name: restore-hook-1
      # Array of namespaces to which this hook applies. If unspecified, the hook applies to all
      # namespaces. Optional.
      includedNamespaces:
      - ns1
      # Array of namespaces to which this hook does not apply. Optional.
      excludedNamespaces:
      - ns3
      # Array of resources to which this hook applies. The only resource supported at this time is
      # pods.
      includedResources:
      - pods
      # Array of resources to which this hook does not apply. Optional.
      excludedResources: []
      # This hook only applies to objects matching this label selector. Optional.
      labelSelector:
        matchLabels:
          app: velero
          component: server
      # An array of hooks to run during or after restores. Currently only "init" and "exec" hooks
      # are supported.
      postHooks:
      # The type of the hook. This must be "init" or "exec".
      - init:
          # An array of container specs to be added as init containers to pods to which this hook applies to.
          initContainers:
          - name: restore-hook-init1
            image: alpine:latest
            # Mounting volumes from the podSpec to which this hooks applies to.
            volumeMounts:
            - mountPath: /restores/pvc1-vm
              # Volume name from the podSpec
              name: pvc1-vm
            command:
            - /bin/ash
            - -c
            - echo -n "FOOBARBAZ" >> /restores/pvc1-vm/foobarbaz
          - name: restore-hook-init2
            image: alpine:latest
            # Mounting volumes from the podSpec to which this hooks applies to.
            volumeMounts:
            - mountPath: /restores/pvc2-vm
              # Volume name from the podSpec
              name: pvc2-vm
            command:
            - /bin/ash
            - -c
            - echo -n "DEADFEED" >> /restores/pvc2-vm/deadfeed
      - exec:
          # The container name where the hook will be executed. Defaults to the first container.
          # Optional.
          container: foo
          # The command that will be executed in the container. Required.
          command:
          - /bin/bash
          - -c
          - "psql < /backup/backup.sql"
          # How long to wait for a container to become ready. This should be long enough for the
          # container to start plus any preceding hooks in the same container to complete. The wait
          # timeout begins when the container is restored and may require time for the image to pull
          # and volumes to mount. If not set the restore will wait indefinitely. Optional.
          waitTimeout: 5m
          # How long to wait once execution begins. Defaults to 30 seconds. Optional.
          execTimeout: 1m
          # How to handle execution failures. Valid values are `Fail` and `Continue`. Defaults to
          # `Continue`. With `Continue` mode, execution failures are logged only. With `Fail` mode,
          # no more restore hooks will be executed in any container in any pod and the status of the
          # Restore will be `PartiallyFailed`. Optional.
          onError: Continue
# RestoreStatus captures the current status of a Velero restore. Users should not set any data here.
status:
  # The current phase. Valid values are New, FailedValidation, InProgress, Completed, PartiallyFailed, Failed.
  phase: ""
  # An array of any validation errors encountered.
  validationErrors: null
  # Number of warnings that were logged by the restore.
  warnings: 2
  # Errors is a count of all error messages that were generated
  # during execution of the restore. The actual errors are stored in object
  # storage.
  errors: 0
  # FailureReason is an error that caused the entire restore
  # to fail.
  failureReason:

```
