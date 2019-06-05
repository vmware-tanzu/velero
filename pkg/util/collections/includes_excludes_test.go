/*
Copyright 2017 the Velero contributors.

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
		check    string
		should   bool
	}{
		{
			name:   "empty - include everything",
			check:  "foo",
			should: true,
		},
		{
			name:     "include *",
			includes: []string{"*"},
			check:    "foo",
			should:   true,
		},
		{
			name:     "include specific - found",
			includes: []string{"foo", "bar", "baz"},
			check:    "foo",
			should:   true,
		},
		{
			name:     "include specific - not found",
			includes: []string{"foo", "baz"},
			check:    "bar",
			should:   false,
		},
		{
			name:     "include *, exclude foo",
			includes: []string{"*"},
			excludes: []string{"foo"},
			check:    "foo",
			should:   false,
		},
		{
			name:     "include *, exclude foo, check bar",
			includes: []string{"*"},
			excludes: []string{"foo"},
			check:    "bar",
			should:   true,
		},
		{
			name:     "both include and exclude foo",
			includes: []string{"foo"},
			excludes: []string{"foo"},
			check:    "foo",
			should:   false,
		},
		{
			name:     "wildcard include",
			includes: []string{"*.bar"},
			check:    "foo.bar",
			should:   true,
		},
		{
			name:     "wildcard include fail",
			includes: []string{"*.bar"},
			check:    "bar.foo",
			should:   false,
		},
		{
			name:     "wildcard exclude",
			includes: []string{"*"},
			excludes: []string{"*.bar"},
			check:    "foo.bar",
			should:   false,
		},
		{
			name:     "wildcard exclude fail",
			includes: []string{"*"},
			excludes: []string{"*.bar"},
			check:    "bar.foo",
			should:   true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			i := NewIncludesExcludes().Includes(test.includes...).Excludes(test.excludes...)
			if e, a := test.should, i.ShouldInclude(test.check); e != a {
				t.Errorf("expected %t, got %t", e, a)
			}
		})
	}
}

func TestValidateIncludesExcludes(t *testing.T) {
	tests := []struct {
		name     string
		includes []string
		excludes []string
		expected []error
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
			expected: []error{errors.New("includes list must either contain '*' only, or a non-empty list of items")},
		},
		{
			name:     "exclude everything not allowed",
			includes: []string{"foo"},
			excludes: []string{"*"},
			expected: []error{errors.New("excludes list cannot contain '*'")},
		},
		{
			name:     "excludes cannot contain items in includes",
			includes: []string{"foo", "bar"},
			excludes: []string{"bar"},
			expected: []error{errors.New("excludes list cannot contain an item in the includes list: bar")},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			res := ValidateIncludesExcludes(test.includes, test.excludes)

			require.Equal(t, len(test.expected), len(res))

			for i := 0; i < len(test.expected); i++ {
				assert.Equal(t, test.expected[i].Error(), res[i].Error())
			}
		})
	}
}

func TestIncludeExcludeString(t *testing.T) {
	tests := []struct {
		name             string
		includes         []string
		excludes         []string
		expectedIncludes string
		expectedExcludes string
	}{
		{
			name:             "unspecified includes/excludes should return '*'/'<none>'",
			includes:         nil,
			excludes:         nil,
			expectedIncludes: "*",
			expectedExcludes: "<none>",
		},
		{
			name:             "specific resources should result in sorted joined string",
			includes:         []string{"foo", "bar"},
			excludes:         []string{"baz", "xyz"},
			expectedIncludes: "bar, foo",
			expectedExcludes: "baz, xyz",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ie := NewIncludesExcludes().Includes(test.includes...).Excludes(test.excludes...)
			assert.Equal(t, test.expectedIncludes, ie.IncludesString())
			assert.Equal(t, test.expectedExcludes, ie.ExcludesString())
		})
	}
}
