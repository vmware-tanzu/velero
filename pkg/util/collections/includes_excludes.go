/*
Copyright The Velero Contributors.

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

package collections

import (
	"strings"

	"github.com/gobwas/glob"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/api/validation"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/sets"

	velerov1api "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	"github.com/vmware-tanzu/velero/pkg/discovery"
	"github.com/vmware-tanzu/velero/pkg/kuberesource"
	"github.com/vmware-tanzu/velero/pkg/util/boolptr"
)

type globStringSet struct {
	sets.String
}

func newGlobStringSet() globStringSet {
	return globStringSet{sets.NewString()}
}

func (gss globStringSet) match(match string) bool {
	for _, item := range gss.List() {
		g, err := glob.Compile(item)
		if err != nil {
			return false
		}
		if g.Match(match) {
			return true
		}
	}
	return false
}

// IncludesExcludes is a type that manages lists of included
// and excluded items. The logic implemented is that everything
// in the included list except those items in the excluded list
// should be included. '*' in the includes list means "include
// everything", but it is not valid in the exclude list.
type IncludesExcludes struct {
	includes globStringSet
	excludes globStringSet
}

func NewIncludesExcludes() *IncludesExcludes {
	return &IncludesExcludes{
		includes: newGlobStringSet(),
		excludes: newGlobStringSet(),
	}
}

// Includes adds items to the includes list. '*' is a wildcard
// value meaning "include everything".
func (ie *IncludesExcludes) Includes(includes ...string) *IncludesExcludes {
	ie.includes.Insert(includes...)
	return ie
}

// GetIncludes returns the items in the includes list
func (ie *IncludesExcludes) GetIncludes() []string {
	return ie.includes.List()
}

// Excludes adds items to the excludes list
func (ie *IncludesExcludes) Excludes(excludes ...string) *IncludesExcludes {
	ie.excludes.Insert(excludes...)
	return ie
}

// GetExcludes returns the items in the excludes list
func (ie *IncludesExcludes) GetExcludes() []string {
	return ie.excludes.List()
}

// ShouldInclude returns whether the specified item should be
// included or not. Everything in the includes list except those
// items in the excludes list should be included.
func (ie *IncludesExcludes) ShouldInclude(s string) bool {
	if ie.excludes.match(s) {
		return false
	}

	// len=0 means include everything
	return ie.includes.Len() == 0 || ie.includes.Has("*") || ie.includes.match(s)
}

// IncludesExcludesInterface is used as polymorphic IncludesExcludes for Global and scope
// resources Include/Exclude.
type IncludesExcludesInterface interface {
	// Check whether the type name passed in by parameter should be included.
	// typeName should be k8s.io/apimachinery/pkg/runtime/schema GroupResource's String() result.
	ShouldInclude(typeName string) bool

	// Check whether the type name passed in by parameter should be excluded.
	// typeName should be k8s.io/apimachinery/pkg/runtime/schema GroupResource's String() result.
	ShouldExclude(typeName string) bool
}

type GlobalIncludesExcludes struct {
	resourceFilter          IncludesExcludes
	includeClusterResources *bool
	namespaceFilter         IncludesExcludes

	helper discovery.Helper
	logger logrus.FieldLogger
}

// ShouldInclude returns whether the specified item should be
// included or not. Everything in the includes list except those
// items in the excludes list should be included.
// It has some exceptional cases. When IncludeClusterResources is set to false,
// no need to check the filter, all cluster resources are excluded.
func (ie *GlobalIncludesExcludes) ShouldInclude(typeName string) bool {
	_, resource, err := ie.helper.ResourceFor(schema.ParseGroupResource(typeName).WithVersion(""))
	if err != nil {
		ie.logger.Errorf("fail to get resource %s. %s", typeName, err.Error())
		return false
	}

	if !resource.Namespaced && boolptr.IsSetToFalse(ie.includeClusterResources) {
		ie.logger.Info("Skipping resource %s, because it's cluster-scoped, and IncludeClusterResources is set to false.", typeName)
		return false
	}

	// when IncludeClusterResources == nil (auto), only directly
	// back up cluster-scoped resources if we're doing a full-cluster
	// (all namespaces and all namespace scope types) backup. Note that in the case of a subset of
	// namespaces being backed up, some related cluster-scoped resources
	// may still be backed up if triggered by a custom action (e.g. PVC->PV).
	// If we're processing namespaces themselves, we will not skip here, they may be
	// filtered out later.
	if typeName != kuberesource.Namespaces.String() && !resource.Namespaced &&
		ie.includeClusterResources == nil && !ie.namespaceFilter.IncludeEverything() {
		ie.logger.Infof("Skipping resource %s, because it's cluster-scoped and only specific namespaces or namespace scope types are included in the backup.", typeName)
		return false
	}

	return ie.resourceFilter.ShouldInclude(typeName)
}

// ShouldExclude returns whether the resource type should be excluded or not.
func (ie *GlobalIncludesExcludes) ShouldExclude(typeName string) bool {
	// if the type name is specified in excluded list, it's excluded.
	if ie.resourceFilter.excludes.match(typeName) {
		return true
	}

	_, resource, err := ie.helper.ResourceFor(schema.ParseGroupResource(typeName).WithVersion(""))
	if err != nil {
		ie.logger.Errorf("fail to get resource %s. %s", typeName, err.Error())
		return true
	}

	// the resource type is cluster scope
	if !resource.Namespaced {
		// if includeClusterResources is set to false, cluster resource should be excluded.
		if boolptr.IsSetToFalse(ie.includeClusterResources) {
			return true
		}
		// if includeClusterResources is set to nil, check whether it's included by resource
		// filter.
		if ie.includeClusterResources == nil && !ie.resourceFilter.ShouldInclude(typeName) {
			return true
		}
	}

	return false
}

type ScopeIncludesExcludes struct {
	namespaceScopedResourceFilter IncludesExcludes // namespace-scoped resource filter
	clusterScopedResourceFilter   IncludesExcludes // cluster-scoped resource filter
	namespaceFilter               IncludesExcludes // namespace filter

	helper discovery.Helper
	logger logrus.FieldLogger
}

// ShouldInclude returns whether the specified resource should be included or not.
// The function will check whether the resource is namespace-scoped resource first.
// For namespace-scoped resource, except resources listed in excludes, other things should be included.
// For cluster-scoped resource, except resources listed in excludes, only include the resource specified by the included.
// It also has some exceptional checks. For namespace, as long as it's not excluded, it is involved.
// If all namespace-scoped resources are included, all cluster-scoped resource are returned to get a full backup.
func (ie *ScopeIncludesExcludes) ShouldInclude(typeName string) bool {
	_, resource, err := ie.helper.ResourceFor(schema.ParseGroupResource(typeName).WithVersion(""))
	if err != nil {
		ie.logger.Errorf("fail to get resource %s. %s", typeName, err.Error())
		return false
	}

	if resource.Namespaced {
		if ie.namespaceScopedResourceFilter.excludes.Has("*") || ie.namespaceScopedResourceFilter.excludes.match(typeName) {
			return false
		}

		// len=0 means include everything
		return ie.namespaceScopedResourceFilter.includes.Len() == 0 || ie.namespaceScopedResourceFilter.includes.Has("*") || ie.namespaceScopedResourceFilter.includes.match(typeName)
	}

	if ie.clusterScopedResourceFilter.excludes.Has("*") || ie.clusterScopedResourceFilter.excludes.match(typeName) {
		return false
	}

	// when IncludedClusterScopedResources and ExcludedClusterScopedResources are not specified,
	// only directly back up cluster-scoped resources if we're doing a full-cluster
	// (all namespaces and all namespace-scoped types) backup.
	if len(ie.clusterScopedResourceFilter.includes.List()) == 0 &&
		len(ie.clusterScopedResourceFilter.excludes.List()) == 0 &&
		ie.namespaceFilter.IncludeEverything() &&
		ie.namespaceScopedResourceFilter.IncludeEverything() {
		return true
	}

	// Also include namespace resource by default.
	return ie.clusterScopedResourceFilter.includes.Has("*") || ie.clusterScopedResourceFilter.includes.match(typeName) || typeName == kuberesource.Namespaces.String()
}

// ShouldExclude returns whether the resource type should be excluded or not.
// For ScopeIncludesExcludes, if the resource type is specified in the exclude
// list, it should be excluded.
func (ie *ScopeIncludesExcludes) ShouldExclude(typeName string) bool {
	_, resource, err := ie.helper.ResourceFor(schema.ParseGroupResource(typeName).WithVersion(""))
	if err != nil {
		ie.logger.Errorf("fail to get resource %s. %s", typeName, err.Error())
		return true
	}

	if resource.Namespaced {
		if ie.namespaceScopedResourceFilter.excludes.match(typeName) {
			return true
		}
	} else {
		if ie.clusterScopedResourceFilter.excludes.match(typeName) {
			return true
		}
	}
	return false
}

// IncludesString returns a string containing all of the includes, separated by commas, or * if the
// list is empty.
func (ie *IncludesExcludes) IncludesString() string {
	return asString(ie.GetIncludes(), "*")
}

// ExcludesString returns a string containing all of the excludes, separated by commas, or <none> if the
// list is empty.
func (ie *IncludesExcludes) ExcludesString() string {
	return asString(ie.GetExcludes(), "<none>")
}

func asString(in []string, empty string) string {
	if len(in) == 0 {
		return empty
	}
	return strings.Join(in, ", ")
}

// IncludeEverything returns true if the includes list is empty or '*'
// and the excludes list is empty, or false otherwise.
func (ie *IncludesExcludes) IncludeEverything() bool {
	return ie.excludes.Len() == 0 && (ie.includes.Len() == 0 || (ie.includes.Len() == 1 && ie.includes.Has("*")))
}

func newScopeIncludesExcludes(nsIncludesExcludes IncludesExcludes, helper discovery.Helper, logger logrus.FieldLogger) *ScopeIncludesExcludes {
	ret := &ScopeIncludesExcludes{
		namespaceScopedResourceFilter: IncludesExcludes{
			includes: newGlobStringSet(),
			excludes: newGlobStringSet(),
		},
		clusterScopedResourceFilter: IncludesExcludes{
			includes: newGlobStringSet(),
			excludes: newGlobStringSet(),
		},
		namespaceFilter: nsIncludesExcludes,
		helper:          helper,
		logger:          logger,
	}

	return ret
}

// ValidateIncludesExcludes checks provided lists of included and excluded
// items to ensure they are a valid set of IncludesExcludes data.
func ValidateIncludesExcludes(includesList, excludesList []string) []error {
	// TODO we should not allow an IncludesExcludes object to be created that
	// does not meet these criteria. Do a more significant refactoring to embed
	// this logic in object creation/modification.

	var errs []error

	includes := sets.NewString(includesList...)
	excludes := sets.NewString(excludesList...)

	if includes.Len() > 1 && includes.Has("*") {
		errs = append(errs, errors.New("includes list must either contain '*' only, or a non-empty list of items"))
	}

	if excludes.Has("*") {
		errs = append(errs, errors.New("excludes list cannot contain '*'"))
	}

	for _, itm := range excludes.List() {
		if includes.Has(itm) {
			errs = append(errs, errors.Errorf("excludes list cannot contain an item in the includes list: %v", itm))
		}
	}

	return errs
}

// ValidateNamespaceIncludesExcludes checks provided lists of included and
// excluded namespaces to ensure they are a valid set of IncludesExcludes data.
func ValidateNamespaceIncludesExcludes(includesList, excludesList []string) []error {
	errs := ValidateIncludesExcludes(includesList, excludesList)

	includes := sets.NewString(includesList...)
	excludes := sets.NewString(excludesList...)

	for _, itm := range includes.List() {
		if nsErrs := validateNamespaceName(itm); nsErrs != nil {
			errs = append(errs, nsErrs...)
		}
	}
	for _, itm := range excludes.List() {
		if nsErrs := validateNamespaceName(itm); nsErrs != nil {
			errs = append(errs, nsErrs...)
		}
	}

	return errs
}

// ValidateScopedIncludesExcludes checks provided lists of namespace-scoped or cluster-scoped
// included and excluded items to ensure they are a valid set of IncludesExcludes data.
func ValidateScopedIncludesExcludes(includesList, excludesList []string) []error {
	var errs []error

	includes := sets.NewString(includesList...)
	excludes := sets.NewString(excludesList...)

	if includes.Len() > 1 && includes.Has("*") {
		errs = append(errs, errors.New("includes list must either contain '*' only, or a non-empty list of items"))
	}

	if excludes.Len() > 1 && excludes.Has("*") {
		errs = append(errs, errors.New("excludes list must either contain '*' only, or a non-empty list of items"))
	}

	if includes.Len() > 0 && excludes.Has("*") {
		errs = append(errs, errors.New("when exclude is '*', include cannot have value"))
	}

	for _, itm := range excludes.List() {
		if includes.Has(itm) {
			errs = append(errs, errors.Errorf("excludes list cannot contain an item in the includes list: %v", itm))
		}
	}

	return errs
}

func validateNamespaceName(ns string) []error {
	var errs []error

	// Velero interprets empty string as "no namespace", so allow it even though
	// it is not a valid Kubernetes name.
	if ns == "" {
		return nil
	}

	// Kubernetes does not allow asterisks in namespaces but Velero uses them as
	// wildcards. Replace asterisks with an arbitrary letter to pass Kubernetes
	// validation.
	tmpNamespace := strings.ReplaceAll(ns, "*", "x")

	if errMsgs := validation.ValidateNamespaceName(tmpNamespace, false); errMsgs != nil {
		for _, msg := range errMsgs {
			errs = append(errs, errors.Errorf("invalid namespace %q: %s", ns, msg))
		}
	}

	return errs
}

// generateIncludesExcludes constructs an IncludesExcludes struct by taking the provided
// include/exclude slices, applying the specified mapping function to each item in them,
// and adding the output of the function to the new struct. If the mapping function returns
// an empty string for an item, it is omitted from the result.
func generateIncludesExcludes(includes, excludes []string, mapFunc func(string) string) *IncludesExcludes {
	res := NewIncludesExcludes()

	for _, item := range includes {
		if item == "*" {
			res.Includes(item)
			continue
		}

		key := mapFunc(item)
		if key == "" {
			continue
		}
		res.Includes(key)
	}

	for _, item := range excludes {
		// wildcards are invalid for excludes,
		// so ignore them.
		if item == "*" {
			continue
		}

		key := mapFunc(item)
		if key == "" {
			continue
		}
		res.Excludes(key)
	}

	return res
}

// generateScopedIncludesExcludes function is similar with generateIncludesExcludes,
// but it's used for scoped Includes/Excludes.
func generateScopedIncludesExcludes(namespacedIncludes, namespacedExcludes, clusterIncludes, clusterExcludes []string, mapFunc func(string, bool) string, nsIncludesExcludes IncludesExcludes, helper discovery.Helper, logger logrus.FieldLogger) *ScopeIncludesExcludes {
	res := newScopeIncludesExcludes(nsIncludesExcludes, helper, logger)

	generateFilter(res.namespaceScopedResourceFilter.includes, namespacedIncludes, mapFunc, true)
	generateFilter(res.namespaceScopedResourceFilter.excludes, namespacedExcludes, mapFunc, true)
	generateFilter(res.clusterScopedResourceFilter.includes, clusterIncludes, mapFunc, false)
	generateFilter(res.clusterScopedResourceFilter.excludes, clusterExcludes, mapFunc, false)

	return res
}

func generateFilter(filter globStringSet, resources []string, mapFunc func(string, bool) string, namespaced bool) {
	for _, item := range resources {
		if item == "*" {
			filter.Insert(item)
			continue
		}

		key := mapFunc(item, namespaced)
		if key == "" {
			continue
		}
		filter.Insert(key)
	}
}

// GetResourceIncludesExcludes takes the lists of resources to include and exclude, uses the
// discovery helper to resolve them to fully-qualified group-resource names, and returns an
// IncludesExcludes list.
func GetResourceIncludesExcludes(helper discovery.Helper, includes, excludes []string) *IncludesExcludes {
	resources := generateIncludesExcludes(
		includes,
		excludes,
		func(item string) string {
			gvr, _, err := helper.ResourceFor(schema.ParseGroupResource(item).WithVersion(""))
			if err != nil {
				// If we can't resolve it, return it as-is. This prevents the generated
				// includes-excludes list from including *everything*, if none of the includes
				// can be resolved. ref. https://github.com/vmware-tanzu/velero/issues/2461
				return item
			}

			gr := gvr.GroupResource()
			return gr.String()
		},
	)

	return resources
}

func GetGlobalResourceIncludesExcludes(helper discovery.Helper, logger logrus.FieldLogger, includes, excludes []string, includeClusterResources *bool, nsIncludesExcludes IncludesExcludes) *GlobalIncludesExcludes {
	ret := &GlobalIncludesExcludes{
		resourceFilter:          *GetResourceIncludesExcludes(helper, includes, excludes),
		includeClusterResources: includeClusterResources,
		namespaceFilter:         nsIncludesExcludes,
		helper:                  helper,
		logger:                  logger,
	}

	logger.Infof("Including resources: %s", ret.resourceFilter.IncludesString())
	logger.Infof("Excluding resources: %s", ret.resourceFilter.ExcludesString())
	return ret
}

// GetScopeResourceIncludesExcludes function is similar with GetResourceIncludesExcludes,
// but it's used for scoped Includes/Excludes, and can handle both cluster-scoped and namespace-scoped resources.
func GetScopeResourceIncludesExcludes(helper discovery.Helper, logger logrus.FieldLogger, namespaceIncludes, namespaceExcludes, clusterIncludes, clusterExcludes []string, nsIncludesExcludes IncludesExcludes) *ScopeIncludesExcludes {
	ret := generateScopedIncludesExcludes(
		namespaceIncludes,
		namespaceExcludes,
		clusterIncludes,
		clusterExcludes,
		func(item string, namespaced bool) string {
			gvr, resource, err := helper.ResourceFor(schema.ParseGroupResource(item).WithVersion(""))
			if err != nil {
				return item
			}
			if resource.Namespaced != namespaced {
				return ""
			}

			gr := gvr.GroupResource()
			return gr.String()
		},
		nsIncludesExcludes,
		helper,
		logger,
	)
	logger.Infof("Including namespace-scoped resources: %s", ret.namespaceScopedResourceFilter.IncludesString())
	logger.Infof("Excluding namespace-scoped resources: %s", ret.namespaceScopedResourceFilter.ExcludesString())
	logger.Infof("Including cluster-scoped resources: %s", ret.clusterScopedResourceFilter.GetIncludes())
	logger.Infof("Excluding cluster-scoped resources: %s", ret.clusterScopedResourceFilter.ExcludesString())

	return ret
}

// UseOldResourceFilters checks whether to use old resource filters (IncludeClusterResources,
// IncludedResources and ExcludedResources), depending the backup's filters setting.
// New filters are IncludedClusterScopedResources, ExcludedClusterScopedResources,
// IncludedNamespaceScopedResources and ExcludedNamespaceScopedResources.
func UseOldResourceFilters(backupSpec velerov1api.BackupSpec) bool {
	// If all resource filters are none, it is treated as using old parameter filters.
	if backupSpec.IncludeClusterResources == nil &&
		len(backupSpec.IncludedResources) == 0 &&
		len(backupSpec.ExcludedResources) == 0 &&
		len(backupSpec.IncludedClusterScopedResources) == 0 &&
		len(backupSpec.ExcludedClusterScopedResources) == 0 &&
		len(backupSpec.IncludedNamespaceScopedResources) == 0 &&
		len(backupSpec.ExcludedNamespaceScopedResources) == 0 {
		return true
	}

	if backupSpec.IncludeClusterResources != nil ||
		len(backupSpec.IncludedResources) > 0 ||
		len(backupSpec.ExcludedResources) > 0 {
		return true
	}

	return false
}
