# Velero Backup performance Improvements and VolumeGroupSnapshot enablement

There are two different goals here, linked by a single primary missing feature in the Velero backup workflow.
The first goal is to enhance backup performance by allowing the primary backup controller to run in multiple threads, enabling Velero to back up multiple items at the same time for a given backup.
The second goal is to enable Velero to eventually support VolumeGroupSnapshots.
For both of these goals, Velero needs a way to determine which items should be backed up together.

This design proposal will include two development phases:
- Phase 1 will refactor the backup workflow to identify blocks of related items that should be backed up together, and then coordinate backup hooks among items in the block.
- Phase 2 will add multiple worker threads for backing up item blocks, so instead of backing up each block as it identified, the velero backup workflow will instead add the block to a channel and one of the workers will pick it up.
- Actual support for VolumeGroupSnapshots is out-of-scope here and will be handled in a future design proposal, but the item block refactor introduced in Phase 1 is a primary building block for this future proposal.

## Background
Currently, during backup processing, the main Velero backup controller runs in a single thread, completely finishing the primary backup processing for one resource before moving on to the next one.
We can improve the overall backup performance by backing up multiple items for a backup at the same time, but before we can do this we must first identify resources that need to be backed up together.
Generally speaking, resources that need to be backed up together are resources with interdependencies -- pods with their PVCs, PVCs with their PVs, groups of pods that form a single application, CRs, pods, and other resources that belong to the same operator, etc.
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
- Identify groups of related items to back up together (ItemBlocks).
- Manage backup hooks at the ItemBlock level rather than per-item.
- Using worker threads, back up ItemBlocks at the same time.

## Non Goals
- Support VolumeGroupSnapshots: this is a future feature, although certain prerequisites for this enhancement are included in this proposal.
- Process multiple backups in parallel: this is a future feature, although certain prerequisites for this enhancement are included in this proposal.
- Refactoring plugin infrastructure to avoid RPC calls for internal plugins.
- Restore performance improvements: this is potentially a future feature

## High-Level Design

### ItemBlock concept

The updated design is based on a new struct/type called `ItemBlock`.
Essentially, an `ItemBlock` is a group of items that must be backed up together in order to guarantee backup integrity.
When we eventually split item backup across multiple worker threads, `ItemBlocks` will be kept together as the basic unit of backup.
To facilitate this, a new plugin type, `ItemBlockAction` will allow relationships between items to be identified by velero -- any resources that must be backed up with other resources will need IBA plugins defined for them.
Examples of `ItemBlocks` include:
1. A pod, its mounted PVCs, and the bound PVs for those PVCs.
2. A VolumeGroup (related PVCs and PVs) along with any pods mounting these volumes.
3. For a ReadWriteMany PVC, the PVC, its bound PV, and all pods mounting this PVC.

### Phase 1: ItemBlock processing
- A new plugin type, `ItemBlockAction`, will be created
- `ItemBlockAction` will contain the API method `GetRelatedItems`, which will be needed for determining which items to group together into `ItemBlocks`.
- When processing the list of items returned from the item collector, instead of simply calling `BackupItem` on each in turn, we will use the `GetRelatedItems` API call to determine other items to include with the current item in an ItemBlock. Repeat recursively on each item returned.
- Don't include an item in more than one ItemBlock -- if the next item from the item collector is already in a block, skip it.
- Once ItemBlock is determined, call new func `BackupItemBlock` instead of `BackupItem`.
- New func `BackupItemBlock` will call pre hooks for any pods in the block, then back up the items in the block (`BackupItem` will no longer run hooks directly), then call post hooks for any pods in the block.
- The finalize phase will not be affected by the ItemBlock design, since this is just updating resources after async operations are completed on the items and there is no need to run these updates in parallel.

### Phase 2: Process ItemBlocks for a single backup in multiple threads
- Concurrent `BackupItemBlock` operations will be executed by worker threads invoked by the backup controller, which will communicate with the backup controller operation via a shared channel.
- The ItemBlock processing loop implemented in Phase 1 will be modified to send each newly-created ItemBlock to the shared channel rather than calling `BackupItemBlock` inline.
- Users will be able to configure the number of workers available for concurrent `BackupItemBlock` operations.
- Access to the BackedUpItems map must be synchronized

## Detailed Design

### Phase 1: ItemBlock processing

#### New ItemBlockAction plugin type

In order for Velero to identify groups of items to back up together in an ItemBlock, we need a way to identify items which need to be backed up along with the current item. While the current `Execute` BackupItemAction method does return a list of additional items which are required by the current item, we need to know this *before* we start the item backup. To support this, we need a new plugin type, `ItemBlockAction` (IBA) with an API method, `GetRelatedItems` which Velero will call on each item as it processes. The expectation is that the registered IBA plugins will return the same items as returned as additional items by the BIA `Execute` method, with the exception that items which are not created until calling `Execute` should not be returned here, as they don't exist yet.

#### Proto definition (compiled into golang by protoc)

The ItemBlockAction plugin type is defined as follows:
```
service ItemBlockAction {
    rpc AppliesTo(ItemBlockActionAppliesToRequest) returns (ItemBlockActionAppliesToResponse);
    rpc GetRelatedItems(ItemBlockActionGetRelatedItemsRequest) returns (ItemBlockActionGetRelatedItemsResponse);
}

message ItemBlockActionAppliesToRequest {
    string plugin = 1;
}

message ItemBlockActionAppliesToResponse {
    ResourceSelector ResourceSelector = 1;
}

message ItemBlockActionGetRelatedItemsRequest {
    string plugin = 1;
    bytes item = 2;
    bytes backup = 3;
}

message ItemBlockActionGetRelatedItemsResponse {
    repeated generated.ResourceIdentifier relatedItems = 1;
}
```

A new PluginKind, `ItemBlockActionV1`, will be created, and the backup process will be modified to use this plugin kind.

For any BIA plugins which return additional items from `Execute()` that need to be backed up at the same time or sequentially in the same worker thread as the current items should add a new IBA plugin to return these same items (minus any which won't exist before BIA `Execute()` is called).
This mainly applies to plugins that operate on pods which reference resources which must be backed up along with the pod and are potentially affected by pod hooks or for plugins which connect multiple pods whose volumes should be backed up at the same time.

### Changes to processing item list from the Item Collector

#### New structs BackupItemBlock, ItemBlock, and ItemBlockItem
```go
package backup

type BackupItemBlock struct {
    itemblock.ItemBlock
    // This is a reference to the  shared itemBackupper for the backup
    itemBackupper *itemBackupper
}

package itemblock

type ItemBlock struct {
    Log           logrus.FieldLogger
    Items         []ItemBlockItem
}

type ItemBlockItem struct {
    Gr           schema.GroupResource
    Item         *unstructured.Unstructured
    PreferredGVR schema.GroupVersionResource
}
```

#### Current workflow
In the `BackupWithResolvers` func, the current Velero implementation iterates over the list of items for backup returned by the Item Collector. For each item, Velero loads the item from the file created by the Item Collector, we call `backupItem`, update the GR map if successful, remove the (temporary) file containing item metadata, and update progress for the backup.

#### Modifications to the loop over ItemCollector results
The `kubernetesResource` struct used by the item collector will be modified to add an `orderedResource` bool which will be set true for all of the resources moved to the beginning of the list as a result of being ordered resources.
While the item collector already puts ordered resources first, there is no indication in the list which of these initial items are from the ordered resources list and which are the remaining (unordered) items.
Velero needs to know which resources are ordered because when we process them later, these initial resources must be processed sequentially, one at a time, before processing the remaining resources in a parallel manner.

The current workflow within each iteration of the ItemCollector.items loop will replaced with the following:
- (note that some of the below should be pulled out into a helper func to facilitate recursive call to it for items returned from `GetRelatedItems`.)
- Before loop iteration, create a new `itemsInBlock` map of type map[velero.ResourceIdentifier]bool which represents the set of items already included in a block.
- If `item` is already in `itemsInBlock`, continue. This one has already been processed.
- Add `item` to `itemsInBlock`.
- Load item from ItemCollector file. Close/remove file after loading (on error return or not, possibly with similar anonymous func to current impl)
- Get matching IBA plugins for item, call `GetRelatedItems` for each. For each item returned, get full item content from ItemCollector (if present in item list, pulling from file, removing file when done) or from cluster (if not present in item list), add item to the current block, add item to `itemsInBlock` map, and then recursively apply current step to each (i.e. call IBA method, add to block, etc.)
- Once full ItemBlock list is generated, call `backupItemBlock(block ItemBlock)
- Add `backupItemBlock` return values to `backedUpGroupResources` map


#### New func `backupItemBlock`

Method signature for new func `backupItemBlock` is as follows:
```go
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
```go
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
```go
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

When processing the responses, the main thing is to set `backedUpGroupResources[item.groupResource]=true` for each GR returned, which will give the same result as the current implementation calling items one-by-one and setting that field as needed.

The ItemBlock processing loop described above will be split into two separate iterations. For the first iteration, velero will only process those items at the beginning of the loop identified as `orderedResources` -- when the groups generated from these resources are passed to the worker channel, velero will wait for the response before moving on to the next ItemBlock.
This is to ensure that the ordered resources are processed in the required order. Once the last ordered resource is processed, the remaining ItemBlocks will be processed and sent to the worker channel without waiting for a response, in order to allow these ItemBlocks to be processed in parallel.
The reason we must execute `ItemBlocks` with ordered resources first (and one at a time) is that this is a list of resources identified by the user as resources which must be backed up first, and in a particular order.

#### Synchronize access to the BackedUpItems map

Velero uses a map of BackedUpItems to track which items have already been backed up. This prevents velero from attempting to back up an item more than once, as well as guarding against creating infinite loops due to circular dependencies in the additional items returns. Since velero will now be accessing this map from the parallel goroutines, access to the map must be synchronized with mutexes.

### Backup Finalize phase

The finalize phase will not be affected by the ItemBlock design, since this is just updating resources after async operations are completed on the items and there is no need to run these updates in parallel.

## Alternatives considered

### BackpuItemAction v3 API

Instead of adding a new  `ItemBlockAction` plugin type, we could add a `GetAdditionalItems` method to BackupItemAction.
This was rejected because the new plugin type provides a cleaner interface, and keeps the function of grouping related items separate from the function of modifying item content for the backup.

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

### Example IBA implementation for BIA plugins which return additional items

Included below is an example of what might be required for a BIA  plugin which returns additional items.
The code is taken from the internal velero `pod_action.go` which identifies the items required for a given pod.

In this particular case, the only function of pod_action is to return additional items, so we can really just convert this plugin to an IBA plugin. If there were other actions, such as modifying the pod content on backup, then we would still need the pod action, and the related items vs. content manipulation functions would need to be separated.

```go
// PodAction implements ItemBlockAction.
type PodAction struct {
	log logrus.FieldLogger
}

// NewPodAction creates a new ItemAction for pods.
func NewPodAction(logger logrus.FieldLogger) *PodAction {
	return &PodAction{log: logger}
}

// AppliesTo returns a ResourceSelector that applies only to pods.
func (a *PodAction) AppliesTo() (velero.ResourceSelector, error) {
	return velero.ResourceSelector{
		IncludedResources: []string{"pods"},
	}, nil
}

// GetRelatedItems scans the pod's spec.volumes for persistentVolumeClaim volumes and returns a
// ResourceIdentifier list containing references to all of the persistentVolumeClaim volumes used by
// the pod. This ensures that when a pod is backed up, all referenced PVCs are backed up too.
func (a *PodAction) GetRelatedItems(item runtime.Unstructured, backup *v1.Backup) (runtime.Unstructured, []velero.ResourceIdentifier, error) {
	pod := new(corev1api.Pod)
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(item.UnstructuredContent(), pod); err != nil {
		return nil, errors.WithStack(err)
	}

	var relatedItems []velero.ResourceIdentifier
	if pod.Spec.PriorityClassName != "" {
		a.log.Infof("Adding priorityclass %s to relatedItems", pod.Spec.PriorityClassName)
		relatedItems = append(relatedItems, velero.ResourceIdentifier{
			GroupResource: kuberesource.PriorityClasses,
			Name:          pod.Spec.PriorityClassName,
		})
	}

	if len(pod.Spec.Volumes) == 0 {
		a.log.Info("pod has no volumes")
		return relatedItems, nil
	}

	for _, volume := range pod.Spec.Volumes {
		if volume.PersistentVolumeClaim != nil && volume.PersistentVolumeClaim.ClaimName != "" {
			a.log.Infof("Adding pvc %s to relatedItems", volume.PersistentVolumeClaim.ClaimName)

			relatedItems = append(relatedItems, velero.ResourceIdentifier{
				GroupResource: kuberesource.PersistentVolumeClaims,
				Namespace:     pod.Namespace,
				Name:          volume.PersistentVolumeClaim.ClaimName,
			})
		}
	}

	return relatedItems, nil
}

// API call
func (a *PodAction) Name() string {
	return "PodAction"
}

```


## Implementation
Phase 1 and Phase 2 could be implemented within the same Velero release cycle, but they need not be.
Phase 1 is expected to be implemented in Velero 1.15.
Phase 2 could either be in 1.15 as well, or in a later release, depending on the release timing and resource availability.
