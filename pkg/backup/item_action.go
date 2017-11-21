package backup

import (
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"

	api "github.com/heptio/ark/pkg/apis/ark/v1"
)

// ItemAction is an actor that performs an operation on an individual item being backed up.
type ItemAction interface {
	// AppliesTo returns information about which resources this action should be invoked for.
	AppliesTo() (ResourceSelector, error)

	// Execute allows the ItemAction to perform arbitrary logic with the item being backed up and the
	// backup itself. Implementations may return additional ResourceIdentifiers that indicate specific
	// items that also need to be backed up.
	Execute(item runtime.Unstructured, backup *api.Backup) (runtime.Unstructured, []ResourceIdentifier, error)
}

// ResourceIdentifier describes a single item by its group, resource, namespace, and name.
type ResourceIdentifier struct {
	schema.GroupResource
	Namespace string
	Name      string
}

// ResourceSelector is a collection of included/excluded namespaces,
// included/excluded resources, and a label-selector that can be used
// to match a set of items from a cluster.
type ResourceSelector struct {
	IncludedNamespaces []string
	ExcludedNamespaces []string
	IncludedResources  []string
	ExcludedResources  []string
	LabelSelector      string
}
