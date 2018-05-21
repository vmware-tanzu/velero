package plugin

import (
	"k8s.io/apimachinery/pkg/util/sets"
)

// PluginKind is a type alias for a string that describes
// the kind of an Ark-supported plugin.
type PluginKind string

// String returns the string for k.
func (k PluginKind) String() string {
	return string(k)
}

const (
	// PluginKindObjectStore represents an object store plugin.
	PluginKindObjectStore PluginKind = "ObjectStore"

	// PluginKindBlockStore represents a block store plugin.
	PluginKindBlockStore PluginKind = "BlockStore"

	// PluginKindBackupItemAction represents a backup item action plugin.
	PluginKindBackupItemAction PluginKind = "BackupItemAction"

	// PluginKindRestoreItemAction represents a restore item action plugin.
	PluginKindRestoreItemAction PluginKind = "RestoreItemAction"

	// PluginKindPluginLister represents a plugin lister plugin.
	PluginKindPluginLister PluginKind = "PluginLister"
)

// allPluginKinds contains all the valid plugin kinds that Ark supports, excluding PluginLister because that is not a
// kind that a developer would ever need to implement (it's handled by Ark and the Ark plugin library code).
var allPluginKinds = sets.NewString(
	PluginKindObjectStore.String(),
	PluginKindBlockStore.String(),
	PluginKindBackupItemAction.String(),
	PluginKindRestoreItemAction.String(),
)
