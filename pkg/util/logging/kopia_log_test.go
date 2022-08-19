/*
Copyright the Velero contributors.

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

package logging

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGetLogFields(t *testing.T) {
	testCases := []struct {
		name     string
		pairs    []interface{}
		expected map[string]interface{}
	}{
		{
			name: "normal",
			pairs: []interface{}{
				"fake-key1",
				"fake-value1",
				"fake-key2",
				10,
				"fake-key3",
				struct{ v int }{v: 10},
			},
			expected: map[string]interface{}{
				"fake-key1": "fake-value1",
				"fake-key2": 10,
				"fake-key3": struct{ v int }{v: 10},
			},
		},
		{
			name: "non string key",
			pairs: []interface{}{
				"fake-key1",
				"fake-value1",
				10,
				10,
				"fake-key3",
				struct{ v int }{v: 10},
			},
			expected: map[string]interface{}{
				"fake-key1":      "fake-value1",
				"non-string-key": 10,
				"fake-key3":      struct{ v int }{v: 10},
			},
		},
		{
			name: "missing value",
			pairs: []interface{}{
				"fake-key1",
				"fake-value1",
				"fake-key2",
				10,
				"fake-key3",
			},
			expected: map[string]interface{}{
				"fake-key1": "fake-value1",
				"fake-key2": 10,
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			m := getLogFields(tc.pairs...)

			require.Equal(t, tc.expected, m)
		})
	}
}
