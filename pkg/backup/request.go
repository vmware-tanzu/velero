package backup

import (
	velerov1api "github.com/heptio/velero/pkg/apis/velero/v1"
	"github.com/heptio/velero/pkg/util/collections"
	"github.com/heptio/velero/pkg/volume"
)

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

	VolumeSnapshots []*volume.Snapshot
}
