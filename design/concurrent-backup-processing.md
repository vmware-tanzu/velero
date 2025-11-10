# Concurrent Backup Processing

This enhancement will enable Velero to process multiple backups at the same time. This is largely a usability enhancement rather than a performance enhancement, since the overall backup throughput may not be significantly improved over the current implementation, since we are already processing individual backup items in parallel. It is a significant usability improvement, though, as with the current design, a user who submits a small backup may have to wait significantly longer than expected if the backup is submitted immediately after a large backup.

## Background

With the current implementation, only one backup may be `InProgress` at a time. A second backup created will not start processing until the first backup moves on to `WaitingForPluginOperations` or `Finalizing`. This is a usability concern, especially in clusters when multiple users are initiating backups. With this enhancement, we intend to allow multiple backups to be processed concurrently. This will allow backups to start processing immediately, even if a large backup was just submitted by another user. This enhancement will build on top of the prior parallel item processing feature by creating a dedicatede ItemBlock worker pool for each running backup. The pool will be created at the beginning of the backup reconcile, and the input channel will be passed to the Kubernetes backupper just like it is in the current release.

The primary challenge is to make sure that the same workload in multiple backups is not backed up concurrently. If that were to happen, we would risk data corruption, especially around the processing of pod hooks and volume backup. For this first release we will take a conservative, high-level approach to overlap detection. Two backups will not run concurrently if there is any overlap in included namespaces. For example, if a backup that includes `ns1` and `ns2` is running, then a second backup for `ns2` and `ns3` will not be started. If a backup which does not filter namespaces is running (either a whole cluster backup or a non-namespace-limited backup with a label selector) then no other backups will be started, since a backup across all namespaces overlaps with any other backup. Calculating item-level overlap for queued backups is problematic since we don't know which items are included in a backup until backup processing has begun. A future release may add ItemBlock overlap detection, where at the item block worker level, the same item will not be processed by two different workers at the same time. This works together with workload conflict detection to further detect conflicts in a more granular level for shared resources between backups. Eventually, with a more complete understanding of individual workloads (either via ItemBlocks or some higher level model), the namespace level overlap detection may be relaxed in future versions.

## Goals
- Process multiple backups concurrently
- Detect namespace overlap to avoid conflicts
- For queued backups (not yet runnable due to concurrency limits or overlap), indicate the queue position in status

## Non Goals
- Handling NFS PVs when more than one PV point to the same underlying NFS share
- Handling VGDP cancellation for failed backups on restart
- Mounting a PVC for scenarios in which /tmp is too small for the number of concurrent backups
- Providing a mechanism to identify high priority backups which get preferential treatment in terms of ItemBlock worker availability
- Item-level overlap detection (future feature)
- Providing the ability to disable namespace-level overlap detection once Item-level overlap detection is in place (although this may be supported in a future version).

## High-Level Design

### Backup CRD changes

Two new backup phases will be added: `Queued` and `ReadyToStart`. In the Backup workflow, new backups will be moved to the Queued phase when they are added to the backup queue. When a backup is removed from the queue because it is now able to run, it will be moved to the `ReadyToStart` phase, which will allow the backup controller to start processing it.

In addition, a new Status field, `QueuePosition`, will be added to track the backup's current position in the queue.

### New Controller: `backupQueueReconciler`

A new reconciler will be added, `backupQueueReconciler` which will use the current `backupReconciler` logic for reconciling `New` backups but instead of running the backup, it will move the Backup to the `Queued` phase and set `QueuePosition`.

In addition, this reconciler will periodically reconcile all queued backups (on some configurable time interval) and if there is a runnable backup, remove it from the queue, update `QueuePosition` for any queued backups behind it, and update its phase to `ReadyToStart`.

Queued backups will be reconciled in order based on `QueuePosition`, so the first runnable backup found will be processed. A backup is runnable if both of the following conditions are true:
1) The total number of backups either `InProgress` or `ReadyToStart` is less than the configured number of concurrent backups.
2) The backup has no overlap with any backups currently `InProgress` or `ReadyToStart` or with any `Queued` backups with a higher (i.e. closer to 1) queue position than this backup.

### Updates to Backup controller

The current `backupReconciler` will change its reconciling rules. Instead of watching and reconciling New backups, it will reconcile `ReadyToStart` backups. In addition, it will be configured to run in parallel by setting `MaxConcurrentReconciles` based on the `concurrent-backups` server arg.

The startup (and shutdown) of the ItemBlock worker pool will be moved from reconciler startup to the backup reconcile, which will give each running backup its own dedicated worker pool. The per-backup worker pool will will use the existing `--item-block-worker-count` installer/server arg. This means that the maximum number of ItemBlock workers for the entire Velero pod will be the ItemBlock worker count multiplied by concurrentBackups. For example, if concurrentBackups is 5, and itemBlockWorkerCount is 6, then there will be, at most, 30 worker threads active, 5 dedicated to each InProgress backup, but this maximum will only be achieved when the maximum number of backups are InProgress. This also means that each InProgress backup will have a dedicated ItemBlock input channel with the same fixed buffer size.

## Detailed Design

### New Install/Server configuration args

A new install/server arg, `concurrent-backups` will be added. This will be an int-valued field specifying the number of backups which may be processed concurrently (with phase `InProgress`). If not specified, the default value of 1 will be used.

### Consideration of backup overlap and concurrent backup processing

The primary consideration for running additional backups concurrently is the configured `concurrent-backups` parameter. If the total number of `InProgress` and `ReadyToStart` backups is equal to `concurrent-backups` then any `Queued` backups will remain in the queue.

The second consideration is backup overlap. In order to prevent interaction between running backups (particularly around volume backup and pod hooks), we cannot allow two overlapping backups to run at the same time. For now, we will define overlap broadly -- requiring that two concurrent backups don't include any of the same namespaces. A backup for `ns1` can run concurrently with a backup for `ns2`, but a backup for `[ns1,ns2]` cannot run concurrently with a backup for `ns1`. One consequence of this approach is that a backup which includes all namespaces (even if further filtered by resource or label) cannot run concurrently with *any other backup*.

When determining which queued backup to run next, velero will look for the next queued backup which has no overlap with any InProgress backup or any Queued backup ahead of it. The reason we need to consider queued as well as running backups for overlap detection is as follows.

Consider the following scenario. These are the current not-completed backups (ordered from oldest to newest)
1. backup1, includedNamespaces: [ns1, ns2], phase: InProgress
2. backup2, includedNamespaces: [ns2, ns3, ns5], phase: Queued, QueuePosition: 1
3. backup3, includedNamespaces: [ns4, ns3], phase: Queued, QueuePosition: 2
4. backup4, includedNamespaces: [ns5, ns6], phase: Queued, QueuePosition: 2
5. backup5, includedNamespaces: [ns8, ns9], phase: Queued, QueuePosition: 3

Assuming `concurrent-backups` is 2, on the next reconcile, Velero will be able to start a second backup if there is one with no overlap. `backup2` cannot run, since `ns2` overlaps between it and the running `backup1`. If we only considered running overlap (and not queued overlap), then `backup3` could run now. It conflicts with the queued `backup2` on `ns3` but it does not conflict with the running backup. However, if it runs now, then when `backup1` completes, then `backup2` still can't run (since it now overlaps with running `backup3`on `ns3`), so `backup4` starts instead. Now when `backup3` completes, `backup2` still can't run (since it now conflicts with `backup4` on `ns5`). This means that even though it was the second backup created, it's the fourth to run -- providing worse time to completion than without parallel backups. If a queued backup has a large number of namespaces (a full-cluster backup for example), it would never run as long as new single-namespace backups keep being added to the queue.

To resolve this problem we consider both running backups as well as backups ahead in the queue when resolving overlap conflicts. In the above scenario, `backup2` can't run yet since it overlaps with the running backup on `ns2`. In addition, `backup3` and `backup4` also can't run yet since they overlap with queued `backup2`. Therefore, `backup5` will run now. Once `backup1` completes, `backup2` will be free to run.

### Backup CRD changes

New Backup phases:
```go
const (
	// BackupPhaseQueued means the backup has been added to the
	// queue by the BackupQueueReconciler.
	BackupPhaseQueued BackupPhase = "Queued"

	// BackupPhaseReadyToStart means the backup has been removed from the
	// queue by the BackupQueueReconciler and is ready to start.
	BackupPhaseReadyToStart BackupPhase = "ReadyToStart"
)
```

In addition, a new Status field, `queuePosition`, will be added to track the backup's current position in the queue.
```go
	// QueuePosition is the position held by the backup in the queue.
	// QueuePosition=1 means this backup is the next to be considered.
	// Only relevant when Phase is "Queued"
	// +optional
	QueuePosition int `json:"queuePosition,omitempty"`
```

### New Controller: `backupQueueReconciler`

A new reconciler will be added, `backupQueueReconciler` which will reconcile backups under these conditions:
1) Watching Create/Update for backups in `New` (or empty) phase
2) Watching for Backup phase transition from `InProgress` to something else to reconcile all `Queued` backups
2) Watching for Backup phase transition from `New` (or empty) to `Queued` to reconcile all `Queued` backups
2) Periodic reconcile of `Queued` backups to handle backups queued at server startup as well as to make sure we never have a situation where backups are queued indefinitely because of a race condition or was otherwise missed in the reconcile on prior backup completion.

The reconciler will be set up as follows -- note that New backups are reconciled on Create/Update, while Queued backups are reconciled when an InProgress backup moves on to another state or when a new backup moves to the Queued state. We also reconcile Queued backups periodically to handle the case of a Velero pod restart with Queued backups, as well as to handle possible edge cases where a queued backup doesn't get moved out of the queue at the point of backup completion or an error occurs during a prior Queued backup reconcile.

```go
func (c *backupOperationsReconciler) SetupWithManager(mgr ctrl.Manager) error {
        // only consider Queued backups, order by QueuePosition
	gp := kube.NewGenericEventPredicate(func(object client.Object) bool {
		backup := object.(*velerov1api.Backup)
		return (backup.Status.Phase == velerov1api.BackupPhaseQueued)
	})
	s := kube.NewPeriodicalEnqueueSource(c.logger.WithField("controller", constant.ControllerBackupOperations), mgr.GetClient(), &velerov1api.BackupList{}, c.frequency, kube.PeriodicalEnqueueSourceOption{
		Predicates: []predicate.Predicate{gp},
		OrderFunc: queuePositionOrderFunc,
	})

	return ctrl.NewControllerManagedBy(mgr).
		For(&velerov1api.Backup{}, builder.WithPredicates(predicate.Funcs{
				UpdateFunc: func(ue event.UpdateEvent) bool {
					backup := ue.ObjectNew.(*velerov1api.Backup)
					return backup.Status.Phase == "" || backup.status.Phase == velerov1api.BackupPhaseNew
				},
				CreateFunc: func(event.CreateEvent) bool {
					return backup.Status.Phase == "" || backup.status.Phase == velerov1api.BackupPhaseNew
				},
				DeleteFunc: func(de event.DeleteEvent) bool {
					return false
				},
				GenericFunc: func(ge event.GenericEvent) bool {
					return false
				},
		})).
		Watch(
			&source.Kind{Type: &velerov1api.Backup{}},
			&handler.EnqueueRequestsFromMapFunc{
				ToRequests: handler.ToRequestsFunc(func(a handler.MapObject) []reconcile.Request {
					backupList := velerov1api.BackupList{}
					if err := p.List(ctx, backupList); err != nil {
						p.logger.WithError(err).Error("error listing backups")
						return
					}
					requests = []reconcile.request{}
					// filter backup list by Phase=queued
					// sort backup list by queuePosition
					return requests
				}),
		},
		builder.WithPredicates(predicate.Funcs{
				UpdateFunc: func(ue event.UpdateEvent) bool {
					oldBackup := ue.ObjectOld.(*velerov1api.Backup)
					newBackup := ue.ObjectNew.(*velerov1api.Backup)
					return oldBackup.Status.Phase == velerov1api.BackupPhaseInProgress &&
						newBackup.Status.Phase != velerov1api.BackupPhaseInProgress ||
						oldBackup.Status.Phase != velerov1api.BackupPhaseQueued &&
						newBackup.Status.Phase == velerov1api.BackupPhaseQueued
				},
				CreateFunc: func(event.CreateEvent) bool {
					return false
				},
				DeleteFunc: func(de event.DeleteEvent) bool {
					return false
				},
				GenericFunc: func(ge event.GenericEvent) bool {
					return false
				},
		}).
		WatchesRawSource(s).
		Named(constant.ControllerBackupQueue).
		Complete(c)
}
```

New backups will be queued: Phase will be set to `Queued`, and `QueuePosition` will be set to a int value incremented from the highest current `QueuePosition` value among Queued backups.

Queued backups will be removed from the queue if runnable:
1) If the total number of backups either InProgress or ReadyToStart is greater than or equal to the concurrency limit, then exit without removing from the queue.
2) If the current backup overlaps with any InProgress, ReadyToStart, or Queued backup with `QueuePosition < currentBackup.QueuePosition` then exit without removing from the queue.
3) If we get here, the backup is runnable. To resolve a potential race condition where an InProgress backup completes between reconciling the backup with QueuePosition `n-1` and reconciling the current backup with QueuePosition `n`, we also check to see whether there are any runnable backups in the queue ahead of this one. The only time this will happen is if a backup completes immediately before reconcile starts which either frees up a concurrency slot or removes a namespace conflict. In this case, we don't want to run the current backup since the one ahead of this one in the queue (which was recently passed over before the InProgress backup completed) must run first. In this case, exit without removing from the queue.
4) If we get here, remove the backup from the queue by setting Phase to `ReadyToStart` and `QueuePosition` to zero. Decrement the `QueuePosition` of any other Queued backups with a `QueuePosition` higher than the current backup's queue position prior to dequeuing. At this point, the backup reconciler will start the backup.

`if len(inProgressBackups)+len(pendingStartBackups) >= concurrentBackups`

```
	switch original.Status.Phase {
	case "", velerov1api.BackupPhaseNew:
		// enqueue backup -- set phase=Queued, set queuePosition=maxCurrentQueuePosition+1
	}
	// We should only ever get these events when added in order by the periodical enqueue source
	// so as long as the current backup has not conflicts ahead of it or running, we should be good to
	// dequeue
	case "", velerov1api.BackupPhaseQueued:
		// list backups, filter on Queued, ReadyToStart, and InProgress
		// if number of InProgress backups + number of ReadyToStart backups >= concurrency limit, exit
		// generate list of all namespaces included in InProgress, ReadyToStart, and Queued backups with
		// queuePosition < backup.Status.QueuePosition
		// if overlap found, exit
		// check backups ahead of this one in the queue for runnability. If any are runnable, exit
		// dequeue backup: set Phase to ReadyToStart, QueuePosition to 0, and decrement QueuePosition
		// for all QueuedBackups behind this one in the queue
	}

```

The queue controller will run as a single reconciler thread, so we will not need to deal with concurrency issues when moving backups from New to Queued or from Queued to ReadyToStart, and all of the updates to QueuePosition will be from a single thread.

### Updates to Backup controller

The Reconcile logic will be updated to respond to ReadyToStart backups instead of New backups:

```
@@ -234,8 +234,8 @@ func (b *backupReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctr
        // InProgress, we still need this check so we can return nil to indicate we've finished processing
        // this key (even though it was a no-op).
        switch original.Status.Phase {
-       case "", velerov1api.BackupPhaseNew:
-               // only process new backups
+       case velerov1api.BackupPhaseReadyToStart:
+               // only process ReadyToStart backups
        default:
                b.logger.WithFields(logrus.Fields{
                        "backup": kubeutil.NamespaceAndName(original),
```

In addition, it will be configured to run in parallel by setting `MaxConcurrentReconciles` based on the `concurrent-backups` server arg.

```
@@ -149,6 +149,9 @@ func NewBackupReconciler(
 func (b *backupReconciler) SetupWithManager(mgr ctrl.Manager) error {
        return ctrl.NewControllerManagedBy(mgr).
                For(&velerov1api.Backup{}).
+                WithOptions(controller.Options{
+                       MaxConcurrentReconciles: concurrentBackups,
+               }).
                Named(constant.ControllerBackup).
                Complete(b)
 }
```

The controller-runtime core reconciler logic already prevents the same resource from being reconciled by two different reconciler threads, so we don't need to worry about concurrency issues at the controller level.

The workerPool reference will be moved from the backupReconciler to the backupRequest, since this will now be backup-specific, and the initialization code for the worker pool will be moved from the reconciler init into the backup reconcile. This worker pool will be shut down upon exiting the Reconcile method.

### Resilience to restart of velero pod

The new backup phases (`Queued` and `ReadyToStart`) will be resilient to velero pod restarts. If the velero pod crashes or is restarted, only backups in the `InProgress` phase will be failed, so there is no change to current behavior. Queued backups will retain their queue position on restart, and ReadyToStart backups will move to InProgress when reconciled.

### Observability

#### Logging

When a backup is dequeued, an info log message will also include the wait time, calculated as `now - creationTimestamp`. When a backup is passed over due to overlap, an info log message will indicate which namespaces were in conflict.

#### Velero CLI

The `velero backup describe` output will include the current queue position for queued backups.
