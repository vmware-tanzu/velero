package backup

import (
	arkv1api "github.com/heptio/ark/pkg/apis/ark/v1"
	"github.com/heptio/ark/pkg/util/collections"
	"github.com/heptio/ark/pkg/volume"
)

// Request is a request for a backup, with all references to other objects
// materialized (e.g. backup/snapshot locations, includes/excludes, etc.)
type Request struct {
	*arkv1api.Backup

	StorageLocation           *arkv1api.BackupStorageLocation
	SnapshotLocations         []*arkv1api.VolumeSnapshotLocation
	NamespaceIncludesExcludes *collections.IncludesExcludes
	ResourceIncludesExcludes  *collections.IncludesExcludes
	ResourceHooks             []resourceHook
	ResolvedActions           []resolvedAction

	VolumeSnapshots []*volume.Snapshot
}
