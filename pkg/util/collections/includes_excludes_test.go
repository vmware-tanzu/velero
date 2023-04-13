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
	"testing"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"

	velerov1api "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	"github.com/vmware-tanzu/velero/pkg/builder"
	"github.com/vmware-tanzu/velero/pkg/test"
)

func TestShouldInclude(t *testing.T) {
	tests := []struct {
		name     string
		includes []string
		excludes []string
		item     string
		want     bool
	}{
		{
			name: "empty string should include every item",
			item: "foo",
			want: true,
		},
		{
			name:     "include * should include every item",
			includes: []string{"*"},
			item:     "foo",
			want:     true,
		},
		{
			name:     "item in includes list should include item",
			includes: []string{"foo", "bar", "baz"},
			item:     "foo",
			want:     true,
		},
		{
			name:     "item not in includes list should not include item",
			includes: []string{"foo", "baz"},
			item:     "bar",
			want:     false,
		},
		{
			name:     "include *, excluded item should not include item",
			includes: []string{"*"},
			excludes: []string{"foo"},
			item:     "foo",
			want:     false,
		},
		{
			name:     "include *, exclude foo, bar should be included",
			includes: []string{"*"},
			excludes: []string{"foo"},
			item:     "bar",
			want:     true,
		},
		{
			name:     "an item both included and excluded should not be included",
			includes: []string{"foo"},
			excludes: []string{"foo"},
			item:     "foo",
			want:     false,
		},
		{
			name:     "wildcard should include item",
			includes: []string{"*.bar"},
			item:     "foo.bar",
			want:     true,
		},
		{
			name:     "wildcard mismatch should not include item",
			includes: []string{"*.bar"},
			item:     "bar.foo",
			want:     false,
		},
		{
			name:     "wildcard exclude should not include item",
			includes: []string{"*"},
			excludes: []string{"*.bar"},
			item:     "foo.bar",
			want:     false,
		},
		{
			name:     "wildcard mismatch should include item",
			includes: []string{"*"},
			excludes: []string{"*.bar"},
			item:     "bar.foo",
			want:     true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			includesExcludes := NewIncludesExcludes().Includes(tc.includes...).Excludes(tc.excludes...)

			if got := includesExcludes.ShouldInclude((tc.item)); got != tc.want {
				t.Errorf("want %t, got %t", tc.want, got)
			}
		})
	}
}

func TestValidateIncludesExcludes(t *testing.T) {
	tests := []struct {
		name     string
		includes []string
		excludes []string
		want     []error
	}{
		{
			name:     "empty includes (everything) is allowed",
			includes: []string{},
		},
		{
			name:     "include everything",
			includes: []string{"*"},
		},
		{
			name:     "include everything not allowed with other includes",
			includes: []string{"*", "foo"},
			want:     []error{errors.New("includes list must either contain '*' only, or a non-empty list of items")},
		},
		{
			name:     "exclude everything not allowed",
			includes: []string{"foo"},
			excludes: []string{"*"},
			want:     []error{errors.New("excludes list cannot contain '*'")},
		},
		{
			name:     "excludes cannot contain items in includes",
			includes: []string{"foo", "bar"},
			excludes: []string{"bar"},
			want:     []error{errors.New("excludes list cannot contain an item in the includes list: bar")},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			errs := ValidateIncludesExcludes(tc.includes, tc.excludes)

			require.Equal(t, len(tc.want), len(errs))

			for i := 0; i < len(tc.want); i++ {
				assert.Equal(t, tc.want[i].Error(), errs[i].Error())
			}
		})
	}
}

func TestIncludeExcludeString(t *testing.T) {
	tests := []struct {
		name         string
		includes     []string
		excludes     []string
		wantIncludes string
		wantExcludes string
	}{
		{
			name:         "unspecified includes/excludes should return '*'/'<none>'",
			includes:     nil,
			excludes:     nil,
			wantIncludes: "*",
			wantExcludes: "<none>",
		},
		{
			name:         "specific resources should result in sorted joined string",
			includes:     []string{"foo", "bar"},
			excludes:     []string{"baz", "xyz"},
			wantIncludes: "bar, foo",
			wantExcludes: "baz, xyz",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			includesExcludes := NewIncludesExcludes().Includes(tc.includes...).Excludes(tc.excludes...)
			assert.Equal(t, tc.wantIncludes, includesExcludes.IncludesString())
			assert.Equal(t, tc.wantExcludes, includesExcludes.ExcludesString())
		})
	}
}

func TestValidateNamespaceIncludesExcludes(t *testing.T) {
	tests := []struct {
		name     string
		includes []string
		excludes []string
		wantErr  bool
	}{
		{
			name:     "empty slice doesn't return error",
			includes: []string{},
			wantErr:  false,
		},
		{
			name:     "asterisk by itself is valid",
			includes: []string{"*"},
			wantErr:  false,
		},
		{
			name:     "alphanumeric names with optional dash inside are valid",
			includes: []string{"foobar", "bar-321", "foo123bar"},
			excludes: []string{"123bar", "barfoo", "foo-321", "bar123foo"},
			wantErr:  false,
		},
		{
			name:     "not starting or ending with an alphanumeric character is invalid",
			includes: []string{"-123foo"},
			excludes: []string{"foo321-", "foo321-"},
			wantErr:  true,
		},
		{
			name:     "special characters in name is invalid",
			includes: []string{"foo?", "foo.bar", "bar_321"},
			excludes: []string{"$foo", "foo>bar", "bar=321"},
			wantErr:  true,
		},
		{
			name:     "empty includes (everything) is valid",
			includes: []string{},
			wantErr:  false,
		},
		{
			name:     "empty string includes is valid (includes nothing)",
			includes: []string{""},
			wantErr:  false,
		},
		{
			name:     "empty string excludes is valid (excludes nothing)",
			excludes: []string{""},
			wantErr:  false,
		},
		{
			name:     "include everything using asterisk is valid",
			includes: []string{"*"},
			wantErr:  false,
		},
		{
			name:     "excludes can contain wildcard",
			includes: []string{"foo", "bar"},
			excludes: []string{"nginx-ingress-*", "*-bar", "*-ingress-*"},
			wantErr:  false,
		},
		{
			name:     "includes can contain wildcard",
			includes: []string{"*-foo", "kube-*", "*kube*"},
			excludes: []string{"bar"},
			wantErr:  false,
		},
		{
			name:     "include everything not allowed with other includes",
			includes: []string{"*", "foo"},
			wantErr:  true,
		},
		{
			name:     "exclude everything not allowed",
			includes: []string{"foo"},
			excludes: []string{"*"},
			wantErr:  true,
		},
		{
			name:     "excludes cannot contain items in includes",
			includes: []string{"foo", "bar"},
			excludes: []string{"bar"},
			wantErr:  true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			errs := ValidateNamespaceIncludesExcludes(tc.includes, tc.excludes)

			if tc.wantErr && len(errs) == 0 {
				t.Errorf("%s: wanted errors but got none", tc.name)
			}

			if !tc.wantErr && len(errs) != 0 {
				t.Errorf("%s: wanted no errors but got: %v", tc.name, errs)
			}
		})
	}
}

func TestValidateScopedIncludesExcludes(t *testing.T) {
	tests := []struct {
		name     string
		includes []string
		excludes []string
		wantErr  []error
	}{
		// includes testing
		{
			name:     "empty includes is valid",
			includes: []string{},
			wantErr:  []error{},
		},
		{
			name:     "asterisk includes is valid",
			includes: []string{"*"},
			wantErr:  []error{},
		},
		{
			name:     "include everything not allowed with other includes",
			includes: []string{"*", "foo"},
			wantErr:  []error{errors.New("includes list must either contain '*' only, or a non-empty list of items")},
		},
		// excludes testing
		{
			name:     "empty excludes is valid",
			excludes: []string{},
			wantErr:  []error{},
		},
		{
			name:     "asterisk excludes is valid",
			excludes: []string{"*"},
			wantErr:  []error{},
		},
		{
			name:     "exclude everything not allowed with other excludes",
			excludes: []string{"*", "foo"},
			wantErr:  []error{errors.New("excludes list must either contain '*' only, or a non-empty list of items")},
		},
		// includes and excludes combination testing
		{
			name:     "asterisk excludes doesn't work with non-empty includes",
			includes: []string{"foo"},
			excludes: []string{"*"},
			wantErr:  []error{errors.New("when exclude is '*', include cannot have value")},
		},
		{
			name:     "excludes cannot contain items in includes",
			includes: []string{"foo", "bar"},
			excludes: []string{"bar"},
			wantErr:  []error{errors.New("excludes list cannot contain an item in the includes list: bar")},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			errs := ValidateScopedIncludesExcludes(tc.includes, tc.excludes)

			require.Equal(t, len(tc.wantErr), len(errs))

			for i := 0; i < len(tc.wantErr); i++ {
				assert.Equal(t, tc.wantErr[i].Error(), errs[i].Error())
			}
		})
	}
}

func TestNamespaceScopedShouldInclude(t *testing.T) {
	tests := []struct {
		name                    string
		namespaceScopedIncludes []string
		namespaceScopedExcludes []string
		item                    string
		want                    bool
		apiResources            []*test.APIResource
	}{
		{
			name: "empty string should include every item",
			item: "pods",
			want: true,
			apiResources: []*test.APIResource{
				test.Pods(),
			},
		},
		{
			name:                    "include * should include every item",
			namespaceScopedIncludes: []string{"*"},
			item:                    "pods",
			want:                    true,
			apiResources: []*test.APIResource{
				test.Pods(),
			},
		},
		{
			name:                    "item in includes list should include item",
			namespaceScopedIncludes: []string{"foo", "bar", "pods"},
			item:                    "pods",
			want:                    true,
			apiResources: []*test.APIResource{
				test.Pods(),
			},
		},
		{
			name:                    "item not in includes list should not include item",
			namespaceScopedIncludes: []string{"foo", "baz"},
			item:                    "pods",
			want:                    false,
			apiResources: []*test.APIResource{
				test.Pods(),
			},
		},
		{
			name:                    "include *, excluded item should not include item",
			namespaceScopedIncludes: []string{"*"},
			namespaceScopedExcludes: []string{"pods"},
			item:                    "pods",
			want:                    false,
			apiResources: []*test.APIResource{
				test.Pods(),
			},
		},
		{
			name:                    "include *, exclude foo, bar should be included",
			namespaceScopedIncludes: []string{"*"},
			namespaceScopedExcludes: []string{"foo"},
			item:                    "pods",
			want:                    true,
			apiResources: []*test.APIResource{
				test.Pods(),
			},
		},
		{
			name:                    "an item both included and excluded should not be included",
			namespaceScopedIncludes: []string{"pods"},
			namespaceScopedExcludes: []string{"pods"},
			item:                    "pods",
			want:                    false,
			apiResources: []*test.APIResource{
				test.Pods(),
			},
		},
		{
			name:                    "wildcard should include item",
			namespaceScopedIncludes: []string{"*s"},
			item:                    "pods",
			want:                    true,
			apiResources: []*test.APIResource{
				test.Pods(),
			},
		},
		{
			name:                    "wildcard mismatch should not include item",
			namespaceScopedIncludes: []string{"*.bar"},
			item:                    "pods",
			want:                    false,
			apiResources: []*test.APIResource{
				test.Pods(),
			},
		},
		{
			name:                    "exclude * should include nothing",
			namespaceScopedExcludes: []string{"*"},
			item:                    "pods",
			want:                    false,
			apiResources: []*test.APIResource{
				test.Pods(),
			},
		},
		{
			name:                    "wildcard exclude should not include item",
			namespaceScopedIncludes: []string{"*"},
			namespaceScopedExcludes: []string{"*s"},
			item:                    "pods",
			want:                    false,
			apiResources: []*test.APIResource{
				test.Pods(),
			},
		},
		{
			name:                    "wildcard exclude mismatch should include item",
			namespaceScopedExcludes: []string{"*.bar"},
			item:                    "pods",
			want:                    true,
			apiResources: []*test.APIResource{
				test.Pods(),
			},
		},
		{
			name: "resource cannot be found by discovery client should not be include",
			item: "pods",
			want: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			discoveryHelper := setupDiscoveryClientWithResources(tc.apiResources)
			logger := logrus.StandardLogger()
			scopeIncludesExcludes := GetScopeResourceIncludesExcludes(discoveryHelper, logger, tc.namespaceScopedIncludes, tc.namespaceScopedExcludes, []string{}, []string{}, *NewIncludesExcludes())

			if got := scopeIncludesExcludes.ShouldInclude((tc.item)); got != tc.want {
				t.Errorf("want %t, got %t", tc.want, got)
			}
		})
	}
}

func TestClusterScopedShouldInclude(t *testing.T) {
	tests := []struct {
		name                  string
		clusterScopedIncludes []string
		clusterScopedExcludes []string
		nsIncludes            []string
		item                  string
		want                  bool
		apiResources          []*test.APIResource
	}{
		{
			name:       "empty string should include nothing",
			nsIncludes: []string{"default"},
			item:       "persistentvolumes",
			want:       false,
			apiResources: []*test.APIResource{
				test.PVs(),
			},
		},
		{
			name:                  "include * should include every item",
			clusterScopedIncludes: []string{"*"},
			item:                  "persistentvolumes",
			want:                  true,
			apiResources: []*test.APIResource{
				test.PVs(),
			},
		},
		{
			name:                  "item in includes list should include item",
			clusterScopedIncludes: []string{"namespaces", "bar", "baz"},
			item:                  "namespaces",
			want:                  true,
			apiResources: []*test.APIResource{
				test.Namespaces(),
			},
		},
		{
			name:                  "item not in includes list should not include item",
			clusterScopedIncludes: []string{"foo", "baz"},
			nsIncludes:            []string{"default"},
			item:                  "persistentvolumes",
			want:                  false,
			apiResources: []*test.APIResource{
				test.PVs(),
			},
		},
		{
			name:                  "include *, excluded item should not include item",
			clusterScopedIncludes: []string{"*"},
			clusterScopedExcludes: []string{"namespaces"},
			item:                  "namespaces",
			want:                  false,
			apiResources: []*test.APIResource{
				test.Namespaces(),
			},
		},
		{
			name:                  "include *, exclude foo, bar should be included",
			clusterScopedIncludes: []string{"*"},
			clusterScopedExcludes: []string{"foo"},
			item:                  "namespaces",
			want:                  true,
			apiResources: []*test.APIResource{
				test.Namespaces(),
			},
		},
		{
			name:                  "an item both included and excluded should not be included",
			clusterScopedIncludes: []string{"namespaces"},
			clusterScopedExcludes: []string{"namespaces"},
			item:                  "namespaces",
			want:                  false,
			apiResources: []*test.APIResource{
				test.Namespaces(),
			},
		},
		{
			name:                  "wildcard should include item",
			clusterScopedIncludes: []string{"*spaces"},
			item:                  "namespaces",
			want:                  true,
			apiResources: []*test.APIResource{
				test.Namespaces(),
			},
		},
		{
			name:                  "wildcard mismatch should not include item",
			clusterScopedIncludes: []string{"*.bar"},
			nsIncludes:            []string{"default"},
			item:                  "persistentvolumes",
			want:                  false,
			apiResources: []*test.APIResource{
				test.PVs(),
			},
		},
		{
			name:                  "exclude * should include nothing",
			clusterScopedExcludes: []string{"*"},
			item:                  "namespaces",
			want:                  false,
			apiResources: []*test.APIResource{
				test.Namespaces(),
			},
		},
		{
			name:                  "wildcard exclude should not include item",
			clusterScopedIncludes: []string{"*"},
			clusterScopedExcludes: []string{"*spaces"},
			item:                  "namespaces",
			want:                  false,
			apiResources: []*test.APIResource{
				test.Namespaces(),
			},
		},
		{
			name:                  "wildcard exclude mismatch should not include item",
			clusterScopedExcludes: []string{"*spaces"},
			item:                  "namespaces",
			want:                  false,
			apiResources: []*test.APIResource{
				test.Namespaces(),
			},
		},
		{
			name: "resource cannot be found by discovery client should not be include",
			item: "namespaces",
			want: false,
		},
		{
			name:                  "even namespaces is not in the include list, it should also be involved.",
			clusterScopedIncludes: []string{"foo", "baz"},
			item:                  "namespaces",
			want:                  true,
			apiResources: []*test.APIResource{
				test.Namespaces(),
			},
		},
		{
			name:                  "When all namespaces and namespace scope resources are included, cluster resource should be included.",
			clusterScopedIncludes: []string{},
			nsIncludes:            []string{"*"},
			item:                  "persistentvolumes",
			want:                  true,
			apiResources: []*test.APIResource{
				test.PVs(),
			},
		},
		{
			name:                  "When all namespaces and namespace scope resources are included, but cluster resource is excluded.",
			clusterScopedIncludes: []string{},
			clusterScopedExcludes: []string{"persistentvolumes"},
			nsIncludes:            []string{"*"},
			item:                  "persistentvolumes",
			want:                  false,
			apiResources: []*test.APIResource{
				test.PVs(),
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			discoveryHelper := setupDiscoveryClientWithResources(tc.apiResources)
			logger := logrus.StandardLogger()
			nsIncludeExclude := NewIncludesExcludes().Includes(tc.nsIncludes...)
			scopeIncludesExcludes := GetScopeResourceIncludesExcludes(discoveryHelper, logger, []string{}, []string{}, tc.clusterScopedIncludes, tc.clusterScopedExcludes, *nsIncludeExclude)

			if got := scopeIncludesExcludes.ShouldInclude((tc.item)); got != tc.want {
				t.Errorf("want %t, got %t", tc.want, got)
			}
		})
	}
}

func TestGetScopedResourceIncludesExcludes(t *testing.T) {
	tests := []struct {
		name                            string
		namespaceScopedIncludes         []string
		namespaceScopedExcludes         []string
		clusterScopedIncludes           []string
		clusterScopedExcludes           []string
		expectedNamespaceScopedIncludes []string
		expectedNamespaceScopedExcludes []string
		expectedClusterScopedIncludes   []string
		expectedClusterScopedExcludes   []string
		apiResources                    []*test.APIResource
	}{
		{
			name:                            "only include namespace-scoped resources in IncludesExcludes",
			namespaceScopedIncludes:         []string{"deployments.apps", "persistentvolumes"},
			namespaceScopedExcludes:         []string{"pods", "persistentvolumes"},
			expectedNamespaceScopedIncludes: []string{"deployments.apps"},
			expectedNamespaceScopedExcludes: []string{"pods"},
			expectedClusterScopedIncludes:   []string{},
			expectedClusterScopedExcludes:   []string{},
			apiResources: []*test.APIResource{
				test.Deployments(),
				test.PVs(),
				test.Pods(),
			},
		},
		{
			name:                            "only include cluster-scoped resources in IncludesExcludes",
			clusterScopedIncludes:           []string{"deployments.apps", "persistentvolumes"},
			clusterScopedExcludes:           []string{"pods", "persistentvolumes"},
			expectedNamespaceScopedIncludes: []string{},
			expectedNamespaceScopedExcludes: []string{},
			expectedClusterScopedIncludes:   []string{"persistentvolumes"},
			expectedClusterScopedExcludes:   []string{"persistentvolumes"},
			apiResources: []*test.APIResource{
				test.Deployments(),
				test.PVs(),
				test.Pods(),
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {

			logger := logrus.StandardLogger()
			nsIncludeExclude := NewIncludesExcludes()
			resources := GetScopeResourceIncludesExcludes(setupDiscoveryClientWithResources(tc.apiResources), logger, tc.namespaceScopedIncludes, tc.namespaceScopedExcludes, tc.clusterScopedIncludes, tc.clusterScopedExcludes, *nsIncludeExclude)

			assert.Equal(t, tc.expectedNamespaceScopedIncludes, resources.namespaceScopedResourceFilter.includes.List())
			assert.Equal(t, tc.expectedNamespaceScopedExcludes, resources.namespaceScopedResourceFilter.excludes.List())
			assert.Equal(t, tc.expectedClusterScopedIncludes, resources.clusterScopedResourceFilter.includes.List())
			assert.Equal(t, tc.expectedClusterScopedExcludes, resources.clusterScopedResourceFilter.excludes.List())
		})
	}
}

func TestUseOldResourceFilters(t *testing.T) {
	tests := []struct {
		name                  string
		backup                velerov1api.Backup
		useOldResourceFilters bool
	}{
		{
			name:                  "backup with no filters should use old filters",
			backup:                *defaultBackup().Result(),
			useOldResourceFilters: true,
		},
		{
			name:                  "backup with only old filters should use old filters",
			backup:                *defaultBackup().IncludeClusterResources(true).Result(),
			useOldResourceFilters: true,
		},
		{
			name:                  "backup with only new filters should use new filters",
			backup:                *defaultBackup().IncludedClusterScopedResources("StorageClass").Result(),
			useOldResourceFilters: false,
		},
		{
			// This case should not happen in Velero workflow, because filter validation not old and new
			// filters used together. So this is only used for UT checking, and I assume old filters
			// have higher priority, because old parameter should be the default one.
			name:                  "backup with both old and new filters should use old filters",
			backup:                *defaultBackup().IncludeClusterResources(true).IncludedClusterScopedResources("StorageClass").Result(),
			useOldResourceFilters: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.useOldResourceFilters, UseOldResourceFilters(test.backup.Spec))
		})
	}
}

func defaultBackup() *builder.BackupBuilder {
	return builder.ForBackup(velerov1api.DefaultNamespace, "backup-1").DefaultVolumesToFsBackup(false)
}

func TestShouldExcluded(t *testing.T) {
	falseBoolean := false
	trueBoolean := true
	tests := []struct {
		name                    string
		clusterIncludes         []string
		clusterExcludes         []string
		includeClusterResources *bool
		filterType              string
		resourceName            string
		apiResources            []*test.APIResource
		resourceIsExcluded      bool
	}{
		{
			name:                    "GlobalResourceIncludesExcludes: filters are all default",
			clusterIncludes:         []string{},
			clusterExcludes:         []string{},
			includeClusterResources: nil,
			filterType:              "global",
			resourceName:            "persistentvolumes",
			apiResources: []*test.APIResource{
				test.PVs(),
			},
			resourceIsExcluded: false,
		},
		{
			name:                    "GlobalResourceIncludesExcludes: IncludeClusterResources is set to true",
			clusterIncludes:         []string{},
			clusterExcludes:         []string{},
			includeClusterResources: &trueBoolean,
			filterType:              "global",
			resourceName:            "persistentvolumes",
			apiResources: []*test.APIResource{
				test.PVs(),
			},
			resourceIsExcluded: false,
		},
		{
			name:                    "GlobalResourceIncludesExcludes: IncludeClusterResources is set to false",
			clusterIncludes:         []string{"persistentvolumes"},
			clusterExcludes:         []string{},
			includeClusterResources: &falseBoolean,
			filterType:              "global",
			resourceName:            "persistentvolumes",
			apiResources: []*test.APIResource{
				test.PVs(),
			},
			resourceIsExcluded: true,
		},
		{
			name:                    "GlobalResourceIncludesExcludes: resource is in the include list",
			clusterIncludes:         []string{"persistentvolumes"},
			clusterExcludes:         []string{},
			includeClusterResources: nil,
			filterType:              "global",
			resourceName:            "persistentvolumes",
			apiResources: []*test.APIResource{
				test.PVs(),
			},
			resourceIsExcluded: false,
		},
		{
			name:            "ScopeResourceIncludesExcludes: resource is in the include list",
			clusterIncludes: []string{"persistentvolumes"},
			clusterExcludes: []string{},
			filterType:      "scope",
			resourceName:    "persistentvolumes",
			apiResources: []*test.APIResource{
				test.PVs(),
			},
			resourceIsExcluded: false,
		},
		{
			name:            "ScopeResourceIncludesExcludes: filters are all default",
			clusterIncludes: []string{},
			clusterExcludes: []string{},
			filterType:      "scope",
			resourceName:    "persistentvolumes",
			apiResources: []*test.APIResource{
				test.PVs(),
			},
			resourceIsExcluded: false,
		},
		{
			name:            "ScopeResourceIncludesExcludes: resource is not in the exclude list",
			clusterIncludes: []string{},
			clusterExcludes: []string{"namespaces"},
			filterType:      "scope",
			resourceName:    "persistentvolumes",
			apiResources: []*test.APIResource{
				test.PVs(),
			},
			resourceIsExcluded: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			logger := logrus.StandardLogger()

			var ie IncludesExcludesInterface
			if tc.filterType == "global" {
				ie = GetGlobalResourceIncludesExcludes(setupDiscoveryClientWithResources(tc.apiResources), logger, tc.clusterIncludes, tc.clusterExcludes, tc.includeClusterResources, *NewIncludesExcludes())
			} else if tc.filterType == "scope" {
				ie = GetScopeResourceIncludesExcludes(setupDiscoveryClientWithResources(tc.apiResources), logger, []string{}, []string{}, tc.clusterIncludes, tc.clusterExcludes, *NewIncludesExcludes())
			}
			assert.Equal(t, tc.resourceIsExcluded, ie.ShouldExclude(tc.resourceName))
		})
	}
}

func setupDiscoveryClientWithResources(APIResources []*test.APIResource) *test.FakeDiscoveryHelper {
	resourcesMap := make(map[schema.GroupVersionResource]schema.GroupVersionResource)
	resourceList := make([]*metav1.APIResourceList, 0)

	for _, resource := range APIResources {
		gvr := schema.GroupVersionResource{
			Group:    resource.Group,
			Version:  resource.Version,
			Resource: resource.Name,
		}
		resourcesMap[gvr] = gvr

		resourceList = append(resourceList,
			&metav1.APIResourceList{
				GroupVersion: gvr.GroupVersion().String(),
				APIResources: []metav1.APIResource{
					{
						Name:       resource.Name,
						Kind:       resource.Name,
						Namespaced: resource.Namespaced,
					},
				},
			},
		)
	}

	discoveryHelper := test.NewFakeDiscoveryHelper(false, resourcesMap)
	discoveryHelper.ResourceList = resourceList
	return discoveryHelper
}
