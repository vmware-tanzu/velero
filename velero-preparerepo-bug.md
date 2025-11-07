# Velero PrepareRepo Bug: Missing Local Kopia Config File

## Summary

The `PrepareRepo()` function in Velero's unified repository provider fails to create the local Kopia configuration file when connecting to an existing remote repository. This causes subsequent backup operations to fail with "config file not found" errors in multi-pod environments, after pod restarts, or during sequential backup operations.

## Bug Location

**File**: `pkg/repository/provider/unified_repo.go`
**Lines**: 196-200
**Velero Version**: Current main branch (observed in OpenShift OADP operator using Velero)

## Root Cause

When `PrepareRepo()` detects that a repository already exists remotely (via `IsCreated()`), it returns immediately without calling `Connect()`:

```go
if created, err := urp.repoService.IsCreated(ctx, *repoOption); err != nil {
    return errors.Wrap(err, "error to check backup repo")
} else if created {
    log.Debug("Repo has already been initialized remotely")
    return nil  // ❌ BUG: Returns without creating local config file!
}
```

**Problem**: The `Connect()` function is responsible for creating the local Kopia config file (e.g., `/home/velero/udmrepo/repo-<uid>.conf`). Skipping this call means the config file is never created, causing later `Open()` operations to fail.

## Kopia Architecture Context

Kopia requires **two components** to function:
1. **Backend Storage**: Repository metadata stored in S3/Azure/GCS (created by `Create()`)
2. **Local Config File**: Connection information stored locally (created by `Connect()`)

The current code only creates the local config file during initial repository creation, not when connecting to existing repositories.

## Reproduction Scenarios

1. **Multi-node environments (node-agent daemonset)**:
   - Node-agent pod on Node A creates the repository (config file exists only on Node A's pod)
   - Node-agent pod on Node B tries to use the same repository for a different PVC/backup
   - Result: Node B's node-agent pod fails with "config file not found"

2. **Node-agent pod restarts**:
   - Node-agent pod creates repository with config in ephemeral storage (`/home/velero/udmrepo/`)
   - Node-agent pod restarts (upgrade, crash, node drain), losing the config file
   - Next backup attempt by the restarted node-agent fails

3. **Sequential backups across nodes (node-agent daemonset)**:
   - First backup creates repository via node-agent pod on Node A
   - Second backup of a PVC on Node B is handled by that node's node-agent pod
   - Node B's node-agent pod can't access the repository (no local config file)

## Error Message

```
error to initialize data path: error to boost backup repository connection:
error to connect backup repo: error to connect repo with storage:
error to connect to repository: error loading config file:
open /home/velero/udmrepo/repo-<uuid>.conf: no such file or directory
```

## Proposed Fix

Replace the early return with a `Connect()` call:

```go
if created, err := urp.repoService.IsCreated(ctx, *repoOption); err != nil {
    return errors.Wrap(err, "error to check backup repo")
} else if created {
    log.Debug("Repo has already been initialized remotely, connecting to it")
    return urp.repoService.Connect(ctx, *repoOption)  // ✅ FIX
}
```

## Impact

**Severity**: High
**Affected Scenarios**:
- Multi-node Kubernetes clusters
- Any environment with pod restarts
- Sequential backup operations
- CSI Data Mover backups in OADP/OpenShift

**Current Workaround**: None known. The issue manifests as intermittent backup failures.

## Testing

The bug was discovered while investigating HCP (Hosted Control Plane) backup test failures in OpenShift OADP operator:
- **PR**: openshift/oadp-operator#2013
- **Test**: HCP backup/restore with CSI Data Mover
- **Build Log**: [CI Build Log](https://gcsweb-ci.apps.ci.l2s4.p1.openshiftapps.com/gcs/test-platform-results/pr-logs/pull/openshift_oadp-operator/2013/pull-ci-openshift-oadp-operator-oadp-dev-4.20-e2e-test-hcp-aws/1986610135258107904/artifacts/e2e-test-hcp-aws/e2e/build-log.txt)

## Related Code

- `pkg/repository/udmrepo/kopialib/repo_init.go`: `ConnectBackupRepo()` creates local config
- `pkg/repository/udmrepo/kopialib/lib_repo.go`: `Open()` requires existing config file
- `pkg/repository/provider/unified_repo.go`: `PrepareRepo()` contains the bug

## Status

- **Discovered**: 2025-11-07
- **Velero Issue Tracker**: No existing issues found matching this bug
- **Fix Status**: Proposed but not yet submitted
