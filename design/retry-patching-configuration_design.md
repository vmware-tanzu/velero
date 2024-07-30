# Backup Restore Status Patch Retrying Configuration

## Abstract
When a restore completes, we want to ensure that the restore custom resource progress to the correct status.
If a patch call fails to update status to completion, it should be retried up to a certain time limit.

This design proposes a way to configure timeout for this retry time limit.

## Background
Original Issue: https://github.com/vmware-tanzu/velero/issues/7207

Velero was performing a restore when the API server was rolling out to a new version.
It had trouble connecting to the API server, but eventually, the restore was successful.
However, since the API server was still in the middle of rolling out, Velero failed to update the restore CR status and gave up.

After the connection was restored, it didn't attempt to update, causing the restore CR to be stuck at "In progress" indefinitely.
This can lead to incorrect decisions for other components that rely on the backup/restore CR status to determine completion.

## Goals
- Make timeout configurable for retry patching by reusing existing [`--resource-timeout` server flag](https://github.com/vmware-tanzu/velero/blob/d9ca14747925630664c9e4f85a682b5fc356806d/pkg/cmd/server/server.go#L245)

## Non Goals
- Create a new timeout flag
- Refactor backup/restore workflow


## High-Level Design
We will add retries with timeout to existing patch calls that moves a backup/restore from InProgress to a final status such as
- Completed
- PartiallyFailed
- Failed
- FailedValidation


## Detailed Design
Relevant reconcilers will have `resourceTimeout   time.Duration` added to its struct and to parameters of New[Backup|Restore]XReconciler functions.

pkg/cmd/server/server.go in `func (s *server) runControllers(..) error` also update the New[Backup|Restore]XCReconciler with added duration parameters using value from existing `--resource-timeout` server flag.

Current calls to kube.PatchResource involving status patch will be replaced with kube.PatchResourceWithRetriesOnErrors added to package `kube` below.

Calls where there is a ...client.Patch() will be wrapped with client.RetriesPhasePatchFuncOnErrors() added to package `client` below.

pkg/util/kube/client.go
```go
// PatchResourceWithRetries patches the original resource with the updated resource, retrying when the provided retriable function returns true.
func PatchResourceWithRetries(maxDuration time.Duration, original, updated client.Object, kbClient client.Client, retriable func(error) bool) error {
	return veleroPkgClient.RetriesPhasePatchFunc(maxDuration, func() error { return PatchResource(original, updated, kbClient) }, retriable)
}

// PatchResourceWithRetriesOnErrors patches the original resource with the updated resource, retrying when the operation returns an error.
func PatchResourceWithRetriesOnErrors(maxDuration time.Duration, original, updated client.Object, kbClient client.Client) error {
	return PatchResourceWithRetries(maxDuration, original, updated, kbClient, func(err error) bool {
		// retry using DefaultBackoff to resolve connection refused error that may occur when the server is under heavy load
		// TODO: consider using a more specific error type to retry, for now, we retry on all errors
		// specific errors:
		// - connection refused: https://pkg.go.dev/syscall#:~:text=Errno(0x67)-,ECONNREFUSED,-%3D%20Errno(0x6f
		return err != nil
	})
}
```

pkg/client/retry.go
```go
// CapBackoff provides a backoff with a set backoff cap
func CapBackoff(cap time.Duration) wait.Backoff {
	if cap < 0 {
		cap = 0
	}
	return wait.Backoff{
		Steps:    math.MaxInt,
		Duration: 10 * time.Millisecond,
		Cap:      cap,
		Factor:   retry.DefaultBackoff.Factor,
		Jitter:   retry.DefaultBackoff.Jitter,
	}
}

// RetriesPhasePatchFunc accepts a patch function param, retrying when the provided retriable function returns true.
func RetriesPhasePatchFunc(maxDuration time.Duration, fn func() error, retriable func(error) bool) error {
	return retry.OnError(CapBackoff(maxDuration), func(err error) bool { return retriable(err) }, fn)
}

// RetriesPhasePatchFuncOnErrors accepts a patch function param, retrying when the error is not nil.
func RetriesPhasePatchFuncOnErrors(maxDuration time.Duration, fn func() error) error {
	return RetriesPhasePatchFunc(maxDuration, fn, func(err error) bool { return err != nil })
}
```

## Alternatives Considered
 - Requeuing InProgress backups that is not known by current velero instance to still be in progress as failed (attempted in [#7863](https://github.com/vmware-tanzu/velero/pull/7863))
    - It was deemed as making backup restore flow hard to enhance for future reconciler updates such as adding cancel or adding parallel backups.

## Security Considerations
None

## Compatibility
Retry should only trigger a restore or backup that is already in progress and not patching successfully by current instance. Prior InProgress backups/restores will not be re-processed and will remain stuck InProgress until there is another velero server (re)start.

## Implementation
There is a past implementation in [#7845](https://github.com/vmware-tanzu/velero/pull/7845/) where implementation for this design will be based upon.

## Open Issues
A discussion of issues relating to this proposal for which the author does not know the solution. This section may be omitted if there are none.
