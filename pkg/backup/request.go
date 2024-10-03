/*
Copyright 2020 the Velero contributors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package backup

import (
	"fmt"
	"sort"

	"github.com/vmware-tanzu/velero/internal/hook"
	"github.com/vmware-tanzu/velero/internal/resourcepolicies"
	"github.com/vmware-tanzu/velero/internal/volume"
	velerov1api "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	"github.com/vmware-tanzu/velero/pkg/itemoperation"
	"github.com/vmware-tanzu/velero/pkg/plugin/framework"
	"github.com/vmware-tanzu/velero/pkg/util/collections"
)

type itemKey struct {
	resource  string
	namespace string
	name      string
}

// Request is a request for a backup, with all references to other objects
// materialized (e.g. backup/snapshot locations, includes/excludes, etc.)
type Request struct {
	*velerov1api.Backup

	StorageLocation           *velerov1api.BackupStorageLocation
	SnapshotLocations         []*velerov1api.VolumeSnapshotLocation
	NamespaceIncludesExcludes *collections.IncludesExcludes
	ResourceIncludesExcludes  collections.IncludesExcludesInterface
	ResourceHooks             []hook.ResourceHook
	ResolvedActions           []framework.BackupItemResolvedActionV2
	ResolvedItemBlockActions  []framework.ItemBlockResolvedAction
	VolumeSnapshots           []*volume.Snapshot
	PodVolumeBackups          []*velerov1api.PodVolumeBackup
	BackedUpItems             map[itemKey]struct{}
	itemOperationsList        *[]*itemoperation.BackupOperation
	ResPolicies               *resourcepolicies.Policies
	SkippedPVTracker          *skipPVTracker
	VolumesInformation        volume.BackupVolumesInformation
}

// BackupVolumesInformation contains the information needs by generating
// the backup BackupVolumeInfo array.

// GetItemOperationsList returns ItemOperationsList, initializing it if necessary
func (r *Request) GetItemOperationsList() *[]*itemoperation.BackupOperation {
	if r.itemOperationsList == nil {
		list := []*itemoperation.BackupOperation{}
		r.itemOperationsList = &list
	}
	return r.itemOperationsList
}

// BackupResourceList returns the list of backed up resources grouped by the API
// Version and Kind
func (r *Request) BackupResourceList() map[string][]string {
	resources := map[string][]string{}
	for i := range r.BackedUpItems {
		entry := i.name
		if i.namespace != "" {
			entry = fmt.Sprintf("%s/%s", i.namespace, i.name)
		}
		resources[i.resource] = append(resources[i.resource], entry)
	}

	// sort namespace/name entries for each GVK
	for _, v := range resources {
		sort.Strings(v)
	}

	return resources
}

func (r *Request) FillVolumesInformation() {
	skippedPVMap := make(map[string]string)

	for _, skippedPV := range r.SkippedPVTracker.Summary() {
		skippedPVMap[skippedPV.Name] = skippedPV.SerializeSkipReasons()
	}

	r.VolumesInformation.SkippedPVs = skippedPVMap
	r.VolumesInformation.NativeSnapshots = r.VolumeSnapshots
	r.VolumesInformation.PodVolumeBackups = r.PodVolumeBackups
	r.VolumesInformation.BackupOperations = *r.GetItemOperationsList()
	r.VolumesInformation.BackupName = r.Backup.Name
}
