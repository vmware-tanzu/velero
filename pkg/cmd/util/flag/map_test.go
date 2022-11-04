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

package flag

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSetOfMap(t *testing.T) {
	cases := []struct {
		name     string
		input    string
		error    bool
		expected map[string]string
	}{
		{
			name:  "invalid_input_missing_quote",
			input: `"k=v`,
			error: true,
		},
		{
			name:  "invalid_input_contains_no_key_value_delimiter",
			input: `k`,
			error: true,
		},
		{
			name:  "valid input",
			input: `k1=v1,k2=v2`,
			error: false,
			expected: map[string]string{
				"k1": "v1",
				"k2": "v2",
			},
		},
		{
			name:  "valid input whose value contains entry delimiter",
			input: `k1=v1,"k2=a=b,c=d"`,
			error: false,
			expected: map[string]string{
				"k1": "v1",
				"k2": "a=b,c=d",
			},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			m := NewMap()
			err := m.Set(c.input)
			if c.error {
				require.NotNil(t, err)
				return
			}
			assert.EqualValues(t, c.expected, m.data)
		})
	}

}
