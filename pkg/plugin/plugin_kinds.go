package plugin

import (
	"k8s.io/apimachinery/pkg/util/sets"
)

// PluginKind is a type alias for a string that describes
// the kind of an Ark-supported plugin.
type PluginKind string

func (k PluginKind) String() string {
	return string(k)
}

const (
	// PluginKindObjectStore is the Kind string for
	// an Object Store plugin.
	PluginKindObjectStore PluginKind = "ObjectStore"

	// PluginKindBlockStore is the Kind string for
	// a Block Store plugin.
	PluginKindBlockStore PluginKind = "BlockStore"

	// PluginKindBackupItemAction is the Kind string for
	// a Backup ItemAction plugin.
	PluginKindBackupItemAction PluginKind = "BackupItemAction"

	// PluginKindRestoreItemAction is the Kind string for
	// a Restore ItemAction plugin.
	PluginKindRestoreItemAction PluginKind = "RestoreItemAction"

	// PluginKindPluginLister represents a plugin lister plugin.
	PluginKindPluginLister PluginKind = "PluginLister"
)

var AllPluginKinds = sets.NewString(
	PluginKindObjectStore.String(),
	PluginKindBlockStore.String(),
	PluginKindBackupItemAction.String(),
	PluginKindRestoreItemAction.String(),
)
