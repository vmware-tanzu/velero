package backup

import "github.com/heptio/ark/pkg/util/collections"

type Options struct {
	Namespace       string
	Name            string
	Namespaces      *collections.IncludesExcludes
	Resources       *collections.IncludesExcludes
	LabelSelector   string
	SnapshotVolumes bool
}
