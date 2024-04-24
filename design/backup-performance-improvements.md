# Velero Backup performance Improvements and VolumeGroupSnapshot enablement

There are two different goals here, linked by a single primary missing feature in the Velero backup workflow.
The first goal is to enhance backup performance by allowing the primary backup controller to run in multiple threads, enabling Velero to back up multiple items at the same time for a given backup.
The second goal is to enable Velero to eventually support VolumeGroupSnapshots.
For both of these goals, Velero needs a way to determine which items should be backed up together.

This design proposal will include two development phases:
- Phase 1 will refactor the backup workflow to identify blocks of items that should be backed up together, and then coordinate backup hooks among items in the block.
- Phase 2 will add multiple worker threads for backing up item blocks, so instead of backing up each block as it identified, the velero backup workflow will instead add the block to a channel and one of the workers will pick it up.
- Actual support for VolumeGroupSnapshots is out-of-scope here and will be handled in a future design proposal, but the item block refactor introduced in Phase 1 is a primary building block for this future proposal.

## Background
Currently, during backup processing, the main Velero backup controller runs in a single thread, completely finishing the primary backup processing for one resource before moving on to the next one.
We can improve the overall backup performance by backing up multiple items for a backup at the same time, but before we can do this we must first identify resources that need to be backed up together.
As part of this initial refactoring, once these "Item Blocks" are identified, an additional change will be to move pod hook processing up to the ItemBlock level.
If there are multiple pods in the ItemBlock, pre-hooks for all pods will be run before backing up the items, followed by post-hooks for all pods.
This change to hook processing is another prerequisite for future VolumeGroupSnapshot support, since supporting this will require backing up the pods and volumes together for any volumes which belong to the same group.
Once we are backing up items by block, the next step will be to create multiple worker threads to process and back up ItemBlocks, so that we can back up multiple ItemBlocks at the same time.

In looking at the different kinds of large backups that Velero must deal with, two obvious scenarios come to mind:
1. Backups with a relatively small number of large volumes
2. Backups with a large number of relatively small volumes.

In case 1, the majority of the time spent on the backup is in the asynchronous phases -- CSI snapshot creation actions after the snaphandle exists, and DataUpload processing. In that case, parallel item processing will likely have a minimal impact on overall backup completion time.

In case 2, the majority of time spent on the backup will likely be during the synchronous actions. Especially as regards CSI snapshot creation, the waiting for the VSC snaphandle to exist will result in significant passage of time with thousands of volumes. This is the sort of use case which will benefit the most from parallel item processing.

## Goals
- Identify groups of items to back up together (ItemBlocks).
- Manage backup hooks at the ItemBlock level rather than per-item.
- Using worker threads, back up ItemBlocks at the same time.

## Non Goals
- Support VolumeGroupSnapshots: this is a future feature, although certain prerequisites for this enhancement are included in this proposal.
- Process multiple backups in parallel: this is a future feature, although certain prerequisites for this enhancement are included in this proposal.
- Refactoring plugin infrastructure to avoid RPC calls for internal plugins.

## High-Level Design

### Phase 1: ItemBlock processing
- A new BIA method, `GetAdditionalItems`, will be needed for pre-processing ItemBlocks (this will require a new BIAv3 API).
- When processing the list of items returned from the item collector, instead of simply calling `BackupItem` on each in turn, we will use the `GetAdditionalItems` BIAv3 API call to determine other items to include with the current item in an ItemBlock. Repeat recursively on each item returned.
- Don't include an item in more than one ItemBlock -- if the next item from the item collector is already in a block, skip it.
- Once ItemBlock is determined, call new func `BackupItemBlock` instead of `BackupItem`.
- New func `BackupItemBlock` will call pre hooks for any pods in the block, then back up the items in the block (`BackupItem` will no longer run hooks directly), then call post hooks for any pods in the block.

### Phase 2: Process ItemBlocks for a single backup in multiple threads
- Concurrent `BackupItemBlock` operations will be executed by worker threads invoked by the backup controller, which will communicate with the backup controller operation via a shared channel.
- The ItemBlock processing loop implemented in Phase 1 will be modified to send each newly-created ItemBlock to the shared channel rather than calling `BackupItemBlock` inline.
- Users will be able to configure the number of workers available for concurrent `BackupItemBlock` operations.
- Access to the BackedUpItems map must be synchronized

## Detailed Design

### Phase 1: ItemBlock processing

#### BackupItemAction plugin changes

In order for Velero to identify groups of items to back up together in an ItemBlock, we need a way to identify items which need to be backed up along with the current item. While the current `Execute` BackupItemAction method does return a list of additional items which are required by the current item, we need to know this *before* we start the item backup. To support this, we need a new API method, `GetAdditionalItems` which Velero will call on each item as it processes it for an ItemBlock. The expectation is that this method will return the same items as currently returned as additional items by the current `Execute` method, with the exception that items which are not created until calling `Execute` should not be returned here, as they don't exist yet.

#### Proto changes (compiled into golang by protoc)

The BackupItemAction service gets one new rpc method:
```
service BackupItemAction {
    rpc GetAdditionalItems(BackupItemActionGetAdditionalItemsRequest) returns (BackupItemActionGetAdditionalItemsResponse);
}
```

To support this new rpc method, we define new request/response message types:
```
message BackupItemActionAdditionalItemsRequest {
    string plugin = 1;
    bytes item = 2;
    bytes backup = 3;
}

message BackupItemActionAdditionalItemsResponse {
    repeated generated.ResourceIdentifier additionalItems = 1;
}
```

A new PluginKind, `BackupItemActionV3`, will be created, and the backup process will be modified to use this plugin kind. Existing v1/v2 plugins will be adapted to v3 with an empty `GetAdditionalItems` method, meaning that those plugins will not add anything to the ItemBlock for the item being backed up.

Any BIA plugins which return additional items from `Execute()` that need to be backed up at the same time or sequentially in the same worker thread as the current items need to be upgraded to v3. This mainly applies to plugins that operate on pods which reference resources which must be backed up along with the pod and are potentially affected by pod hooks or for plugins which connect multiple pods whose volumes should be backed up at the same time.

### Changes to processing item list from the Item Collector

#### New structs ItemBlock and ItemBlockItem
```
type ItemBlock struct {
    log           logrus.FieldLogger
    itemBackupper *itemBackupper
    Items         []ItemBlockItem
}

type ItemBlockItem struct {
    gr           schema.GroupResource
    item         *unstructured.Unstructured
    preferredGVR schema.GroupVersionResource
}
```

#### Current workflow
In the `BackupWithResolvers` func, the current Velero implementation iterates over the list of items for backup returned by the Item Collector. For each item, Velero loads the item from the file created by the Item Collector, we call `backupItem`, update the GR map if successful, remove the (temporary) file containing item metadata, and update progress for the backup.

#### Modifications to the loop over ItemCollector results
The `kubernetesResource` struct used by the item collector will be modified to add an `orderedResource` bool which will be set true for all of the resources moved to the beginning of the list as a result of being ordered resources.

The current workflow within each iteration of the ItemCollector.items loop will replaced with the following:
- (note that some of the below should be pulled out into a helper func to facilitate recursive call to it for items returned from `GetAdditonalItems`.)
- Before loop iteration, create a new `itemsInBlock` map of type map[velero.ResourceIdentifier]bool which represents the set of items already included in a block.
- If `item` is already in `itemsInBlock`, continue. This one has already been processed.
- Add `item` to `itemsInBlock`.
- Load item from ItemCollector file. Close/remove file after loading (on error return or not, possibly with similar anonymous func to current impl)
- Get matching BIA plugins for item, call `GetAdditionalItems` for each. For each item returned, get full item content from ItemCollector (if present in item list, pulling from file, removing file when done) or from cluster (if not present in item list), add item to the current block, add item to `itemsInBlock` map, and then recursively apply current step to each (i.e. call BIA method, add to block, etc.)
- Once full ItemBlock list is generated, call `backupItemBlock(block ItemBlock)
- Add `backupItemBlock` return values to `backedUpGroupResources` map


#### New func `backupItemBlock`

Method signature for new func `backupItemBlock` is as follows:
```
func backupItemBlock(block ItemBlock) []schema.GroupResource
```
The return value is a slice of GRs for resources which were backed up. Velero tracks these to determine which CRDs need to be included in the backup. Note that we need to make sure we include in this not only those resources that were backed up directly, but also those backed up indirectly via additional items BIA execute returns.

In order to handle backup hooks, this func will first take the input item list (`block.items`) and get a list of included pods, filtered to include only those not yet backed up (using `block.itemBackupper.backupRequest.BackedUpItems`). Iterate over this list and execute pre hooks (pulled out of `itemBackupper.backupItemInternal`) for each item.
Now iterate over the full list (`block.items`) and call `backupItem` for each. After the first, the later items should already have been backed up, but calling a second time is harmless, since the first thing Velero does is check the `BackedUpItems` map, exiting if item is already backed up). We still need this call in case there's a plugin which returns something in `GetAdditionalItems` but forgets to return it in the `Execute` additional items return value. If we don't do this, we could end up missing items.

After backing up the items in the block, we now execute post hooks using the same filtered item list we used for pre hooks, again taking the logic from `itemBackupper.backupItemInternal`).

#### `itemBackupper.backupItemInternal` cleanup

After implementing backup hooks in `backupItemBlock`, hook processing should be removed from `itemBackupper.backupItemInternal`.

### Phase 2: Process ItemBlocks for a single backup in multiple threads

#### New input field for number of ItemBlock workers

The velero installer and server CLIs will get a new input field `itemBlockWorkerCount`, which will be passed along to the `backupReconciler`.
The `backupReconciler` struct will also have this new field added. 

#### Worker pool for item block processing

A new type, `ItemBlockWorker` will be added which will manage a pool of worker goroutines which will process item blocks, a shared input channel for passing blocks to workers, and a WaitGroup to shut down cleanly when the reconciler exits.
```
type ItemBlockWorkerPool struct {
    itemBlockChannel chan ItemBlockInput
    wg               *sync.WaitGroup
    logger           logrus.FieldLogger
}

type ItemBlockInput struct {
    itemBlock  ItemBlock
    returnChan chan ItemBlockReturn
}

type ItemBlockReturn struct {
    itemBlock  ItemBlock
    resources []schema.GroupResource
    err       error
}

func (*p ItemBlockWorkerPool) getInputChannel() chan ItemBlockInput
func RunItemBlockWorkers(context context.Context, workers int)
func processItemBlocksWorker(context context.Context, itemBlockChannel chan ItemBlockInput, logger logrus.FieldLogger, wg *sync.WaitGroup)
```

The worker pool will be started by calling `RunItemBlockWorkers` in `backupReconciler.SetupWithManager`, passing in the worker count and reconciler context.
`SetupWithManager` will also add the input channel to the `itemBackupper` so that it will be available during backup processing.
The func `RunItemBlockWorkers` will create the `ItemBlockWorkerPool` with a shared buffered input channel (fixed buffer size) and start `workers` gororoutines which will each call `processItemBlocksWorker`.
The `processItemBlocksWorker` func (run by the worker goroutines) will read from `itemBlockChannel`, call `BackupItemBlock` on the retrieved `ItemBlock`, and then send the return value to the retrieved `returnChan`, and then process the next block.

#### Modify ItemBlock processing loop to send ItemBlocks to the worker pool rather than backing them up directly

The ItemBlock processing loop implemented in Phase 1 will be modified to send each newly-created ItemBlock to the shared channel rather than calling `BackupItemBlock` inline, using a WaitGroup to manage in-process items. A separate goroutine will be created to process returns for this backup. After completion of the ItemBlock processing loop, velero will use the WaitGroup to wait for all ItemBlock processing to complete before moving forward.

A simplified example of what this response goroutine might look like:
```
    // omitting cancel handling, context, etc
    ret := make(chan ItemBlockReturn)
    wg := &sync.WaitGroup{}
    // Handle returns
    go func() {
        for {
            select {
            case response := <-ret: // process each BackupItemBlock response
                func() {
                    defer wg.Done()
                    responses = append(responses, response)
                }()
            case <-ctx.Done():
                return
            }
        }
    }()
    // Simplified illustration, looping over and assumed already-determined ItemBlock list
    for _, itemBlock := range itemBlocks {
        wg.Add(1)
        inputChan <- ItemBlockInput{itemBlock: itemBlock, returnChan: ret}
    }
    done := make(chan struct{})
    go func() {
        defer close(done)
        wg.Wait()
    }()
    // Wait for all the ItemBlocks to be processed
    select {
    case <-done:
        logger.Info("done processing ItemBlocks")
    }
    // responses from BackupItemBlock calls are in responses
```

The ItemBlock processing loop described above will be split into two separate iterations. For the first iteration, velero will only process those items at the beginning of the loop identified as `orderedResources` -- when the groups generated from these resources are passed to the worker channel, velero will wait for the response before moving on to the next ItemBlock. This is to ensure that the ordered resources are processed in the required order. Once the last ordered resource is processed, the remaining ItemBlocks will be processed and sent to the worker channel without waiting for a response, in order to allow these ItemBlocks to be processed in parallel.

#### Synchronize access to the BackedUpItems map

Velero uses a map of BackedUpItems to track which items have already been backed up. This prevents velero from attempting to back up an item more than once, as well as guarding against creating infinite loops due to circular dependencies in the additional items returns. Since velero will now be accessing this map from the parallel goroutines, access to the map must be synchronized with mutexes.

## Alternatives considered

### New ItemBlockAction plugin type

Instead of adding a new `GetAdditionalItems` method to BackupItemAction, another possibility would be to leave BIA alone and create a new plugin type, ItemBlockAction.
Rather than adding the `GetAdditionalItems` method to the existing BIA plugin type, velero would introduce a new ItemBlockAction plugin type which provides the single `GetAdditionalItems` method, described above as a new BIAv3 method.
For existing BIA implementations which return additional items in `Execute()`, instead of adding the new v3 method (and, if necessary, the missing v2 methods) to the existing BIA, they would create a new ItemBlockAction plugin to provide this method.

The change to the backup workflow would be relatively simple: resolve registered ItemBlockAction (IBA) plugins as well as BIA plugins, determine for both plugin types which apply to the current resource type, and when calling the new API method, iterate over IBA plugins rather than BIA plugins.

The change to the "BackupItemAction plugin changes" section would be to create a new plugin type rather than enhancing the existing plugin type. The API function needed in this new plugin type would be identical to the one described as a new BIAv3 method. The other work here would be to create the infrastructure/scaffolding to define, register, etc. the new plugin type.

Overall, the decision of extending BIA vs. creating a new plugin type won't change much in the design here.
The new plugin type will require more initial work, but ongoing maintenance should be similar between the options.

### Per-backup worker pool

The current design makes use of a permanent worker pool, started at backup controller startup time. With this design, when we follow on with running multiple backups in parallel, the same set of workers will take ItemBlock inputs from more than one backup. Another approach that was initially considered was a temporary worker pool, created while processing a backup, and deleted upon backup completion. 

#### User-visible API differences between the two approaches

The main user-visible difference here is in the configuration API. For the permanent worker approach, the worker count represents the total worker count for all backups. The concurrent backup count represents the number of backups running at the same time. At any given time, though, the maximum number of worker threads backing up items concurrently is equal to the worker count. If worker count is 15 and the concurrent backup count is 3, then there will be, at most, 15 items being processed at the same time, split among up to three running backups.

For the per-backup worker approach, the worker count represents the worker count for each backup. The concurrent backup count, as before, represents the number of backups running at the same time. If worker count is 15 and the concurrent backup count is 3, then there will be, at most, 45 items being processed at the same time, up to 15 for each of up to three running backups.
#### Comparison of the two approaches

- Permanent worker pool advantages:
  - This is the more commonly-followed Kubernetes pattern. It's generally better to follow standard practices, unless there are genuine reasons for the use case to go in a different way.
  - It's easier for users to understand the maximum number of concurrent items processed, which will have performance impact and impact on the resource requirements for the Velero pod. Users will not have to multiply the config numbers in their heads when working out how many total workers are present.
  - It will give us more flexibility for future enhancements around concurrent backups. One possible use case: backup priority. Maybe a user wants scheduled backups to have a lower priority than user-generated backups, since a user is sitting there waiting for completion -- a shared worker pool could react to the priority by taking ItemBlocks for the higher priority backup first, which would allow a large lower-priority backup's items to be preempted by a higher-priority backup's items without needing to explicitly stop the main controller flow for that backup.
- Per-backup worker pool advantages:
  - Lower memory consumption than permanent worker pool, but the total memory used by a worker blocked on input will be pretty low, so if we're talking only 10-20 workers, the impact will be minimal.

## Compatibility

### Example upgrade to BIAv3

Included below is an example of what might be required to upgrade a v1 plugin which returns additional items to BIAv3.
The code is taken from the internal velero `pod_action.go` which identifies the items required for a given pod.

Basically, the required changes are as follows:
- (for v1 plugins) Implement `Name()`. This returns a string. The content isn't particularly important, since Velero doesn't actually make an RPC call for this method, but it must be defined on the type in order to implement the interface.
- (for v1 plugins) Implement `Progress(`). Since the plugin doesn't have any asynchronous operations (or it would already be a v2 plugin), this should just return the error `biav2.AsyncOperationsNotSupportedError()`.
- (for v1 plugins) Implement `Cancel(`). Since the plugin doesn't have any asynchronous operations (or it would already be a v2 plugin), this should just return nil.
- (for v1 or v2 plugins) Implement `GetAdditionalItems()` If the additionalItems return value from `Execute()` is nil (or only returns items newly-created in Execute()), it should return nil. Otherwise, it should return the same items as `Execute()` minus any items that don't exist yet. In the example below, this was done by putting the additional item list generation code into `GetAdditionalItems()` and refactoring `Execute()` to call the new func to get the list. For plugins which return a combination of already-existing and newly-created items, `GetAdditionalItems()` should generate the already-existing list, and `Execute()` should append the newly-created items to the list.
- When registering the BIA, replace `RegisterBackupItemAction` (for v1 plugins) or `RegisterBackupItemActionV2` (for v2 plugins) with `RegisterBackupItemActionV3`

```diff --git a/pkg/backup/actions/pod_action.go b/pkg/backup/actions/pod_action.go
index ce6b1ade8..5625dcb5b 100644
--- a/pkg/backup/actions/pod_action.go
+++ b/pkg/backup/actions/pod_action.go
@@ -25,6 +25,7 @@ import (
        v1 "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
        "github.com/vmware-tanzu/velero/pkg/kuberesource"
        "github.com/vmware-tanzu/velero/pkg/plugin/velero"
+       biav2 "github.com/vmware-tanzu/velero/pkg/plugin/velero/backupitemaction/v2"
 )
 
 // PodAction implements ItemAction.
@@ -47,13 +48,19 @@ func (a *PodAction) AppliesTo() (velero.ResourceSelector, error) {
 // Execute scans the pod's spec.volumes for persistentVolumeClaim volumes and returns a
 // ResourceIdentifier list containing references to all of the persistentVolumeClaim volumes used by
 // the pod. This ensures that when a pod is backed up, all referenced PVCs are backed up too.
-func (a *PodAction) Execute(item runtime.Unstructured, backup *v1.Backup) (runtime.Unstructured, []velero.ResourceIdentifier, error) {
+func (a *PodAction) Execute(item runtime.Unstructured, backup *v1.Backup)  (runtime.Unstructured, []velero.ResourceIdentifier, string, []velero.ResourceIdentifier, error) {
        a.log.Info("Executing podAction")
        defer a.log.Info("Done executing podAction")
 
+       additionalItems, err := a.GetAdditionalItems(item, backup)
+       return item, additionalItems, "", nil, err
+}
+
+// v3 API call, return nil if no additional items for this resource
+func (a *PodAction) GetAdditionalItems(item runtime.Unstructured, backup *v1.Backup) (runtime.Unstructured, []velero.ResourceIdentifier, error) {
        pod := new(corev1api.Pod)
        if err := runtime.DefaultUnstructuredConverter.FromUnstructured(item.UnstructuredContent(), pod); err != nil {
-               return nil, nil, errors.WithStack(err)
+               return nil, errors.WithStack(err)
        }
 
        var additionalItems []velero.ResourceIdentifier
@@ -67,7 +74,7 @@ func (a *PodAction) Execute(item runtime.Unstructured, backup *v1.Backup) (runti
 
        if len(pod.Spec.Volumes) == 0 {
                a.log.Info("pod has no volumes")
-               return item, additionalItems, nil
+               return additionalItems, nil
        }
 
        for _, volume := range pod.Spec.Volumes {
@@ -82,5 +89,20 @@ func (a *PodAction) Execute(item runtime.Unstructured, backup *v1.Backup) (runti
                }
        }
 
-       return item, additionalItems, nil
+       return additionalItems, nil
+}
+
+// v2 API call
+func (a *PodAction) Name() string {
+       return "PodAction"
+}
+
+// v2 API call, just return this error for BIAs without async operations
+func (a *PodAction) Progress(operationID string, backup *velerov1api.Backup) (velero.OperationProgress, error) {
+       return velero.OperationProgress{}, biav2.AsyncOperationsNotSupportedError()
+}
+
+// v2 API call, just return nil for BIAs without async operations
+func (a *PodAction) Cancel(operationID string, backup *velerov1api.Backup) error {
+       return nil
 }
diff --git a/pkg/cmd/server/plugin/plugin.go b/pkg/cmd/server/plugin/plugin.go
index c375c5437..7717443dc 100644
--- a/pkg/cmd/server/plugin/plugin.go
+++ b/pkg/cmd/server/plugin/plugin.go
@@ -42,7 +42,7 @@ func NewCommand(f client.Factory) *cobra.Command {
                Run: func(c *cobra.Command, args []string) {
                        pluginServer = pluginServer.
                                RegisterBackupItemAction("velero.io/pv", newPVBackupItemAction).
-                               RegisterBackupItemAction("velero.io/pod", newPodBackupItemAction).
+                               RegisterBackupItemActionV3("velero.io/pod", newPodBackupItemAction).
                                RegisterBackupItemAction("velero.io/service-account", newServiceAccountBackupItemAction(f)).
                                RegisterRestoreItemAction("velero.io/job", newJobRestoreItemAction).
                                RegisterRestoreItemAction("velero.io/pod", newPodRestoreItemAction).
```


## Implementation
Phase 1 and Phase 2 could be implemented within the same Velero release cycle, but they need not be.
Phase 1 is expected to be implemented in Velero 1.15.
Phase 2 could either be in 1.15 as well, or in a later release, depending on the release timing and resource availability.
