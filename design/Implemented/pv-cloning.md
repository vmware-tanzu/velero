# Cloning PVs While Remapping Namespaces

Status: Approved

Velero supports restoring resources into different namespaces than they were backed up from.
This enables a user to, among other things, clone a namespace within a cluster.
However, if the namespace being cloned uses persistent volume claims, Velero cannot currently create a second copy of the original persistent volume when restoring.
This limitation is documented in detail in [issue #192](https://github.com/heptio/velero/issues/192).
This document proposes a solution that allows new copies of persistent volumes to be created during a namespace clone.

## Goals

- Enable persistent volumes to be cloned when using `velero restore create --namespace-mappings ...` to create a second copy of a namespace within a cluster.

## Non Goals

- Cloning of persistent volumes in any scenario other than when using `velero restore create --namespace-mappings ...` flag.
- [CSI-based cloning](https://kubernetes.io/docs/concepts/storage/volume-pvc-datasource/).

## Background

(Omitted, see introduction)

## High-Level Design

During a restore, Velero will detect that it needs to assign a new name to a persistent volume being restored if and only if both of the following conditions are met:
- the persistent volume is claimed by a persistent volume claim in a namespace that's being remapped using `velero restore create --namespace-mappings ...`
- a persistent volume already exists in the cluster with the original name

If these conditions exist, Velero will give the persistent volume a new arbitrary name before restoring it.
It will also update the `spec.volumeName` of the related persistent volume claim. 

## Detailed Design

In `pkg/restore/restore.go`, around [line 872](https://github.com/vmware-tanzu/velero/blob/main/pkg/restore/restore.go#L872), Velero has special-case code for persistent volumes.
This code will be updated to check for the two preconditions described in the previous section.
If the preconditions are met, the object will be given a new name.
The persistent volume will also be annotated with the original name, e.g. `velero.io/original-pv-name=NAME`.
Importantly, the name change will occur **before** [line 890](https://github.com/vmware-tanzu/velero/blob/main/pkg/restore/restore.go#L890), where Velero checks to see if it should restore the persistent volume.
Additionally, the old and new persistent volume names will be recorded in a new field that will be added to the `context` struct, `renamedPVs map[string]string`.

In the special-case code for persistent volume claims starting on [line 987](https://github.com/heptio/velero/blob/main/pkg/restore/restore.go#L987), Velero will check to see if the claimed persistent volume has been renamed by looking in `ctx.renamedPVs`.
If so, Velero will update the persistent volume claim's `spec.volumeName` to the new name.

## Alternatives Considered

One alternative approach is to add a new CLI flag and API field for restores, e.g. `--clone-pvs`, that a user could provide to indicate they want to create copies of persistent volumes.
This approach would work fine, but it does require the user to be aware of this flag/field and to properly specify it when needed.
It seems like a better UX to detect the typical conditions where this behavior is needed, and to automatically apply it.
Additionally, the design proposed here does not preclude such a flag/field from being added later, if it becomes necessary to cover other use cases.

## Security Considerations

N/A
