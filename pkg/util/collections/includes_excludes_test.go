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
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
