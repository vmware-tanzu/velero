/*
Copyright 2017 the Heptio Ark contributors.

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

	"github.com/stretchr/testify/assert"
)

func TestGetString(t *testing.T) {
	var testCases = []struct {
		root      map[string]interface{}
		path      string
		expectErr bool
		result    string
	}{
		{map[string]interface{}{"path": "value"}, "path", false, "value"},
		{map[string]interface{}{"path": "value"}, "path2", true, ""},
		{map[string]interface{}{"path1": map[string]interface{}{"path2": "value"}}, "path1.path2", false, "value"},
		{map[string]interface{}{"path1": map[string]interface{}{"path2": "value"}}, "path1.path1", true, ""},
	}

	for _, tc := range testCases {
		res, err := GetString(tc.root, tc.path)

		if (err != nil) != tc.expectErr {
			t.Error("err")
		}
		if res != tc.result {
			t.Error("res")
		}
	}
}

func TestMergeMaps(t *testing.T) {
	var testCases = []struct {
		name        string
		source      map[string]string
		destination map[string]string
		expected    map[string]string
	}{
		{
			name:        "nil destination should result in source being copied",
			destination: nil,
			source: map[string]string{
				"k1": "v1",
			},
			expected: map[string]string{
				"k1": "v1",
			},
		},
		{
			name: "keys missing from destination should be copied from source",
			destination: map[string]string{
				"k2": "v2",
			},
			source: map[string]string{
				"k1": "v1",
			},
			expected: map[string]string{
				"k1": "v1",
				"k2": "v2",
			},
		},
		{
			name: "matching key should not have value copied from source",
			destination: map[string]string{
				"k1": "v1",
			},
			source: map[string]string{
				"k1": "v2",
			},
			expected: map[string]string{
				"k1": "v1",
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {

			result := MergeMaps(tc.destination, tc.source)

			assert.Equal(t, tc.expected, result)
		})
	}
}
