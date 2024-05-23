# Volume information for restore design

## Background
Velero has different ways to handle data in the volumes during restore.  The users want to have more clarity in terms of how
the volumes are handled in restore process via either Velero CLI or other downstream product which consumes Velero.

## Goals
- Create new metadata to store the information of the restored volume, which will have the same life-cycle as the restore CR.
- Consume the metadata in velero CLI to enable it display more details for volumes in the output of `velero restore describe --details`

## Non Goals
- Provide finer grained control of the volume restore process. The focus of the design is to enable displaying more details.
- Persist additional metadata like podvolume, datadownloads etc to the restore folder in backup-location.

## Design

### Structure of the restore volume info
The restore volume info will be stored in a file named like `${restore_name}-vol-info.json`. The content of the file will
be a list of volume info objects, each of which will map to a volume that is restored, and will contain the information 
like name of the restored PV/PVC, restore method and related objects to provide details depending on the way it's restored,
it will look like this:
```
[
  {
    "pvcName": "nginx-logs-2",
    "pvcNamespace": "nginx-app-restore",
    "pvName": "pvc-e320d75b-a788-41a3-b6ba-267a553efa5e",
    "restoreMethod": "PodVolumeRestore",
    "snapshotDataMoved": false,
    "pvrInfo": {
      "snapshotHandle": "81973157c3a945a5229285c931b02c68",
      "uploaderType": "kopia",
      "volumeName": "nginx-logs",
      "podName": "nginx-deployment-79b56c644b-mjdhp",
      "podNamespace": "nginx-app-restore"
    }
  },
  {
    "pvcName": "nginx-logs-1",
    "pvcNamespace": "nginx-app-restore",
    "pvName": "pvc-98c151f4-df47-4980-ba6d-470842f652cc",
    "restoreMethod": "CSISnapshot",
    "snapshotDataMoved": false,
    "csiSnapshotInfo": {
      "snapshotHandle": "snap-01a3b21a5e9f85528",
      "size": 2147483648,
      "driver": "ebs.csi.aws.com",
      "vscName": "velero-velero-nginx-logs-1-jxmbg-hx9x5"
    }
  }
......  
]
```
Each field will have the same meaning as the corresponding field in the backup volume info.  It will not have the fields 
that were introduced to help with the backup process, like `pvInfo`, `dataupload` etc.

### How the restore volume info is generated
Two steps are involved in generating the restore volume info, the first is "collection", which is to gather the information 
for restoration of the volumes, the second is "generation", which is to iterate through the data collected in the first step
and generate the volume info list as is described above.

Unlike backup, the CR objects created during the restore process will not be persisted to the backup storage location.  
Therefore, to gather the information needed to generate volume information, we either need to collect the CRs in the middle
of the restore process, or retrieve the objects based on the `resouce-list.json` of the restore via API server.
The information to be collected are:
- **PV/PVC mapping relationship:** It will be collected via the `restore-resource-list.json`, b/c at the time the json is ready, all
PVCs and PVs are already created.
- **Native snapshot information:** It will be collected in the restore workflow when each snapshot is restored.
- **podvolumerestore CRs:** It will be collected in the restore workflow after each pvr is created.
- **volumesnapshot CRs for CSI snapshot:** It will be collected in the step of collecting PVC info, by reading the `dataSource`
field in the spec of the PVC.
- **datadownload CRs** It will be collected in the phase of collecting PVC info, by querying the API-server to list the datadownload
CRs labeled with the restore name.

After the collection step, the generation step is relatively straight-forward, as we have all the information needed in 
the data structures.  

The whole collection and generation steps will be done with the "best-effort" manner, i.e. if there are any failures we 
will only log the error in restore log, rather than failing the whole restore process, we will not put these errors or warnings
into the `result.json`, b/c it won't impact the restored resources.

Depending on the number of the restored PVCs the "collection" step may involve many API calls, but it's considered acceptable
b/c at that time the resources are already created, so the actual RTO is not impacted.  By using the client of controller runtime
we can make the collection step more efficient by using the cache of the API server.  We may consider to make improvements if 
we observe performance issues, like using multiple go-routines in the collection.

### Implementation
Because the restore volume info shares the same data structures with the backup volume info, we will refactor the code in 
package `internal/volume` to make the sub-components in backup volume info shared by both backup and restore volume info.  

We'll introduce a struct called `RestoreVolumeInfoTracker` which encapsulates the logic of collecting and generating the restore volume info:
```
// RestoreVolumeInfoTracker is used to track the volume information during restore.
// It is used to generate the RestoreVolumeInfo array.
type RestoreVolumeInfoTracker struct {
	*sync.Mutex
	restore *velerov1api.Restore
	log     logrus.FieldLogger
	client  kbclient.Client
	pvPvc   *pvcPvMap

	// map of PV name to the NativeSnapshotInfo from which the PV is restored
	pvNativeSnapshotMap map[string]NativeSnapshotInfo
	// map of PV name to the CSISnapshot object from which the PV is restored
	pvCSISnapshotMap map[string]snapshotv1api.VolumeSnapshot
	datadownloadList *velerov2alpha1.DataDownloadList
	pvrs             []*velerov1api.PodVolumeRestore
}
```
The `RestoreVolumeInfoTracker` will be created when the restore request is initialized, and it will be passed to the `restoreContext`
and carried over the whole restore process.  

The `client` in this struct is to be used to query the resources in the restored namespace, and the current client in restore 
reconciler only watches the resources in the namespace where velero is installed.  Therefore, we need to introduce the 
`CrClient` which has the same life-cycle of velero server to the restore reconciler, because this is the client that watches all the 
resources on the cluster.

In addition to that, we will make small changes in the restore workflow to collect the information needed.  We'll make the 
changes un-intrusive and make sure not to change the logic of the restore to avoid break change or regression.
We'll also introduce routine changes in the package `pkg/persistence` to persist the restore volume info to the backup storage location.

Last but not least, the `velero restore describe --details` will be updated to display the volume info in the output.  

## Alternatives Considered
There used to be suggestion that to provide more details about volume, we can query the `backup-vol-info.json` with the resource 
identifier in `restore-resource-list.json`.  This will not work when there're resource modifiers involved in the restore process,
which may change the metadata of PVC/PV.  In addition, we may add more detailed restore-specific information about the volumes that is not available
in the `backup-vol-info.json`.  Therefore, the `restore-vol-info.json` is a better approach.

## Security Considerations
There should be no security impact introduced by this design.

## Compatibility
The restore volume info will be consumed by Velero CLI and downstream products for displaying details.  So the functionality 
of backup and restore will not be impacted for restores created by older versions of Velero which do not have the restore volume info
metadata.  The client should properly handle the case when the restore volume info does not exist.

The data structures referenced by volume info is shared between both restore and backup and it's not versioned, so in the future 
we must make sure there will only be incremental changes to the metadata, such that no break change will be introduced to the client.

## Open Issues
https://github.com/vmware-tanzu/velero/issues/7546
https://github.com/vmware-tanzu/velero/issues/6478
