# Design proposal for handling restoration of Kubernetes Jobs

This aids in review of the document as changes to a line are not obscured by the reflowing those changes caused and has a side effect of avoiding debate about one or two space after a period.

## Abstract
This design proposes a solution for handling Kubernetes Jobs during Velero restore operations, specifically addressing the challenges with running, failed, and completed Jobs.
The goal is to provide users with configurable options to control Job restoration behavior to prevent unintended re-execution while maintaining the ability to restore Job resources when needed.

## Background
Velero currently skips restoring completed Jobs (those with a completionTime set) but restores failed Jobs and running Jobs.
This can lead to unintended consequences where Jobs are executed again during restore operations, potentially causing side effects or duplicate work.
Jobs in running status from a backup will most likely be completed by the time a restore is run, but the current behavior would still attempt to restore and run them.
Failed Jobs with a restartPolicy of OnFailure will be rerun when restored, which may not be the desired behavior.
Some users have workflows that depend on the presence of Job resources (even completed ones), while others want to avoid re-execution of Jobs.

## Goals
- Provide a configurable way to handle running Jobs during restore operations
- Prevent unintended re-execution of Jobs during restore
- Support users who need Job resources to be present after restore, even if they were completed or failed
- Maintain backward compatibility with existing Velero behavior

## Non Goals
- Modifying the backup behavior for Jobs
- Handling CronJob resources differently than they are currently handled
- Implementing a solution that requires changes to the Kubernetes Jobs controller
- Providing a mechanism to selectively restore only certain Jobs based on complex criteria

## High-Level Design
The proposed solution introduces annotations and restore options to control how Velero handles Jobs during restore operations.
By default, Velero will continue to skip completed Jobs but will restore running or failed Jobs with parallelism set to 0, effectively pausing them to prevent immediate execution.
Users will be able to override this behavior using annotations on individual Jobs or through restore options that apply to all Jobs in a restore operation.

## Detailed Design

### Job Restoration Logic

The current logic for handling Jobs during restore will be modified as follows:

1. **Completed Jobs** (with completionTime set):
   - Default behavior: Skip restoration (current behavior)
   - Optional behavior: Restore with original configuration or with parallelism=0

2. **Running Jobs** (active but not completed):
   - Default behavior: Restore with parallelism=0 (new behavior)
   - Optional behavior: Skip restoration or restore with original configuration

3. **Failed Jobs** (failed but not completed):
   - Default behavior: Restore with parallelism=0 (new behavior)
   - Optional behavior: Skip restoration or restore with original configuration

### Implementation Details

#### 1. Annotations

We will introduce the following annotations to control Job restoration behavior:

##### Phase-Specific Restoration Policies

```
velero.io/job-restore-policy-completed: <policy>
velero.io/job-restore-policy-failed: <policy>
velero.io/job-restore-policy-running: <policy>
```

Where `<policy>` can be one of:
- `skip`: Skip restoration of this Job
- `restore-paused`: Restore the Job with parallelism=0 (paused)
- `restore-as-is`: Restore the Job with its original configuration

These annotations can be applied to individual Jobs before backup to control their restoration behavior based on their phase at backup time.

##### General Restoration Policy (applies to all phases)

```
velero.io/job-restore-policy: <policy>
```

This annotation provides a fallback policy for any phase that doesn't have a specific policy set. If both a phase-specific and general policy are present, the phase-specific policy takes precedence.

##### Backup Phase Recording

During backup, Velero will automatically add the following annotation to record the Job's phase:

```
velero.io/job-phase-at-backup: <phase>
```

Where `<phase>` can be one of:
- `completed`: Job had a completionTime set
- `failed`: Job had failed status
- `running`: Job was actively running
- `pending`: Job was created but not yet started

This annotation allows users to understand why a Job was restored with specific modifications and helps with debugging restore behavior. The annotation is preserved during the restore process since Velero's `resetMetadata` function only removes specific metadata fields (like uid, resourceVersion) but preserves annotations and labels.

#### 2. Restore Annotations

Instead of modifying the Restore CRD, we will use annotations on the Velero Restore object to control the default behavior for all Jobs:

##### Phase-Specific Restore Policies

```
velero.io/job-restore-policy-completed: <policy>
velero.io/job-restore-policy-failed: <policy>
velero.io/job-restore-policy-running: <policy>
```

##### General Restore Policy

```
velero.io/job-restore-policy: <policy>
```

Where `<policy>` can be one of the same values as the Job annotation.

These annotations can be added to the Restore object through the CLI:

```
# Set a general policy for all Jobs
velero restore create --from-backup=my-backup \
  --annotations velero.io/job-restore-policy=<policy>

# Set phase-specific policies
velero restore create --from-backup=my-backup \
  --annotations velero.io/job-restore-policy-completed=skip \
  --annotations velero.io/job-restore-policy-failed=restore-paused \
  --annotations velero.io/job-restore-policy-running=restore-paused
```

The default behavior (if no annotation is specified) will be:
- `skip` for completed Jobs
- `restore-paused` for running and failed Jobs

#### 3. Implementation Changes

The implementation will require changes to the following components:

1. **Backup Controller**:
   - Modify the backup process to automatically add the `velero.io/job-phase-at-backup` annotation to all Jobs
   - Determine the Job phase based on its status (completed, failed, running, or pending)
   - Preserve any existing Job restore policy annotations during backup

2. **Restore Controller**:
   - Modify the `restoreItem` function in `pkg/restore/restore.go` to check for the Job restore policy
   - Implement logic to determine which policy to apply based on the Job's phase at backup time
   - Check for phase-specific policies first, then fall back to general policies
   - Implement logic to modify the Job spec based on the determined policy
   - Add logic to check Job status and apply the appropriate default policy

3. **Restore Annotations**:
   - Implement support for the new annotations on Restore objects
   - Update the CLI documentation to explain the new annotations
   - Implement validation for the annotation values
   - Ensure proper precedence of phase-specific over general policies

4. **Documentation**:
   - Update documentation to explain the new behavior and options
   - Document the automatic phase recording during backup
   - Explain why parallelism is modified for certain Jobs
   - Provide examples of common use cases

#### 4. Modification of Job Resources

When a Job is restored with the `restore-paused` policy, the following changes will be made to the Job spec:

```yaml
spec:
  parallelism: 0  # Set to 0 regardless of original value
```

This change effectively pauses the Job, preventing it from creating new pods until the user explicitly increases the parallelism.

##### Transparency and Documentation

To ensure users understand why a Job's parallelism was modified:

1. The `velero.io/job-phase-at-backup` annotation will be present on all restored Jobs, indicating their phase at backup time
2. The Velero logs will include information about which policy was applied to each Job and why
3. The documentation will clearly explain:
   - The default behavior for each Job phase
   - Why parallelism is set to 0 for certain Jobs (to prevent unintended re-execution)
   - How to restore Jobs with their original configuration if needed
   - The precedence order for annotations (phase-specific > general, Job-level > Restore-level)

### User Experience

#### Example 1: Default Behavior

By default, a restore operation will:
- Skip completed Jobs
- Restore running and failed Jobs with parallelism=0

This prevents unintended re-execution while preserving the Job resources. After restore, Jobs will include the `velero.io/job-phase-at-backup` annotation showing their phase at backup time, helping users understand why certain modifications were made.

#### Example 2: Using Phase-Specific Annotations on Jobs

A user can annotate specific Jobs before backup to control behavior based on phase:

```yaml
apiVersion: batch/v1
kind: Job
metadata:
  name: important-job
  annotations:
    velero.io/job-restore-policy-completed: restore-as-is
    velero.io/job-restore-policy-failed: restore-paused
    velero.io/job-restore-policy-running: skip
spec:
  # ...
```

This Job will be:
- Restored with its original configuration if it was completed
- Restored with parallelism=0 if it had failed
- Skipped if it was still running

#### Example 3: Using General Annotations on Jobs

A user can also use a general policy that applies to all phases:

```yaml
apiVersion: batch/v1
kind: Job
metadata:
  name: less-important-job
  annotations:
    velero.io/job-restore-policy: skip
spec:
  # ...
```

This Job will be skipped during restore regardless of its phase.

#### Example 4: Using Phase-Specific Restore Annotations

A user can specify different policies for different Job phases during restore:

```bash
velero restore create --from-backup=my-backup \
  --annotations velero.io/job-restore-policy-completed=skip \
  --annotations velero.io/job-restore-policy-failed=restore-paused \
  --annotations velero.io/job-restore-policy-running=restore-as-is
```

This will:
- Skip completed Jobs (default behavior)
- Restore failed Jobs with parallelism=0
- Restore running Jobs with their original configuration

#### Example 5: Using General Restore Annotations

A user can specify a global policy for all Jobs during restore:

```bash
velero restore create --from-backup=my-backup \
  --annotations velero.io/job-restore-policy=skip
```

This will skip restoration of all Jobs, regardless of their status.

## Alternatives Considered

### 1. Skip All Jobs During Restore

One alternative is to simply skip all Jobs during restore operations, regardless of their status.
This would prevent any unintended re-execution but would not satisfy users who need Job resources to be present after restore.

### 2. Restore All Jobs As-Is

Another alternative is to restore all Jobs with their original configuration.
This would satisfy users who need Job resources but would cause unintended re-execution of Jobs.

### 3. Use Velero Server Flag

We considered adding a server-side flag to the Velero server to control the default behavior for Job restoration.
However, this approach has several drawbacks:
- It would be a cluster-wide setting that applies to all users and all restore operations
- It may not be applicable to all Jobs in the cluster, requiring many overrides via annotations
- It would require restarting the Velero server to change the behavior
- It would make it difficult to have different behaviors for different restore operations in the same cluster

### 4. Modify the Restore CRD

We considered adding a new field to the Velero Restore CRD to control the Job restoration behavior.
While this would provide a more structured approach than using annotations, we decided against it for the following reasons:
- It would require changes to the CRD, which could impact backward compatibility
- CRD changes require more careful versioning and migration planning
- Annotations provide a more flexible and extensible mechanism for adding metadata without schema changes

However, if this feature proves valuable and widely used, adding a dedicated field to the Restore CRD could be considered in a future release with proper deprecation notices for the annotation-based approach.

## Security Considerations

The proposed changes do not introduce any new security considerations.
The solution operates within the existing security model of Velero and Kubernetes.

## Compatibility

### Backward Compatibility

The proposed changes maintain backward compatibility with existing Velero behavior for completed Jobs.
The default behavior for running and failed Jobs will change, but this change is intended to prevent unintended consequences rather than break existing workflows.

### Forward Compatibility

The design is flexible enough to accommodate future changes to the Kubernetes Jobs API.
If new Job states or features are introduced, the policy-based approach can be extended to handle them.

## Implementation

### Timeline

1. Phase 1: Implement the core functionality
   - Add the annotation support
   - Modify the restore logic to handle Jobs according to the policy
   - Update documentation

2. Phase 2: Add CLI support
   - Add documentation for the restore annotation
   - Implement validation for the annotation
   - Add examples to the CLI help text

### Resources

The implementation will be carried out by the Velero team with input from the community.
Community members who have expressed interest in this feature may be invited to review or contribute to the implementation.

## Open Issues

1. Should we provide more granular control over Job restoration based on other criteria, such as Job age or labels?
2. How should we handle Jobs that are part of a workflow or have dependencies on other resources?
3. Should we extend similar functionality to other resources that might have similar concerns, such as Pods or StatefulSets?
