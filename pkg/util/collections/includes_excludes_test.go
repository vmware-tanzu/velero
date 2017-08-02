/*
Copyright 2017 Heptio Inc.

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
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
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
			name:   "empty - don't include anything",
			check:  "foo",
			should: false,
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
			name:     "include nothing not allowed",
			includes: []string{},
			expected: []error{errors.New("includes list cannot be empty")},
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

			assert.Equal(t, test.expected, res)
		})
	}
}
