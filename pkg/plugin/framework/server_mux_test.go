/*
Copyright 2019 the Velero contributors.

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

package framework

import (
	"strings"
	"testing"
)

func TestValidPluginName(t *testing.T) {
	successCases := []struct {
		pluginName    string
		existingNames []string
	}{
		{"example.io/azure", []string{"velero.io/aws"}},
		{"with-dashes/name", []string{"velero.io/aws"}},
		{"prefix/Uppercase_Is_OK_123", []string{"velero.io/aws"}},
		{"example-with-dash.io/azure", []string{"velero.io/aws"}},
		{"1.2.3.4/5678", []string{"velero.io/aws"}},
		{"example.io/azure", []string{"velero.io/aws"}},
		{"example.io/azure", []string{""}},
		{"example.io/azure", nil},
		{strings.Repeat("a", 253) + "/name", []string{"velero.io/aws"}},
	}
	for i, tt := range successCases {
		t.Run(tt.pluginName, func(t *testing.T) {
			if err := ValidatePluginName(tt.pluginName, tt.existingNames); err != nil {
				t.Errorf("case[%d]: %q: expected success: %v", i, successCases[i], err)
			}
		})
	}

	errorCases := []struct {
		pluginName    string
		existingNames []string
	}{
		{"", []string{"velero.io/aws"}},
		{"single", []string{"velero.io/aws"}},
		{"/", []string{"velero.io/aws"}},
		{"//", []string{"velero.io/aws"}},
		{"///", []string{"velero.io/aws"}},
		{"a/", []string{"velero.io/aws"}},
		{"/a", []string{"velero.io/aws"}},
		{"velero.io/aws", []string{"velero.io/aws"}},
		{"Uppercase_Is_OK_123/name", []string{"velero.io/aws"}},
		{strings.Repeat("a", 254) + "/name", []string{"velero.io/aws"}},
		{"ospecialchars%^=@", []string{"velero.io/aws"}},
	}

	for i, tt := range errorCases {
		t.Run(tt.pluginName, func(t *testing.T) {
			if err := ValidatePluginName(tt.pluginName, tt.existingNames); err == nil {
				t.Errorf("case[%d]: %q: expected failure.", i, errorCases[i])
			}
		})
	}
}
