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

import "testing"

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
