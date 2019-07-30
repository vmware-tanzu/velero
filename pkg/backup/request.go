package backup

import (
	"fmt"

	velerov1api "github.com/heptio/velero/pkg/apis/velero/v1"
	"github.com/heptio/velero/pkg/util/collections"
	"github.com/heptio/velero/pkg/volume"
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
	ResourceIncludesExcludes  *collections.IncludesExcludes
	ResourceHooks             []resourceHook
	ResolvedActions           []resolvedAction

	VolumeSnapshots  []*volume.Snapshot
	PodVolumeBackups []*velerov1api.PodVolumeBackup
	BackedUpItems    map[itemKey]struct{}
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
	return resources
}
