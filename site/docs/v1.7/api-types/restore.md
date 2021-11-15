# Restore API Type

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
  # Array of resources to include in the restore. Resources may be shortcuts (e.g. 'po' for 'pods')
  # or fully-qualified. If unspecified, all resources are included. Optional.
  includedResources:
  - '*'
  # Array of resources to exclude from the restore. Resources may be shortcuts (e.g. 'po' for 'pods')
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
