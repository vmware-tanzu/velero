/*
Copyright 2020 the Velero contributors.

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

package main

import (
	"fmt"
	"testing"
)

func TestRegexMatching(t *testing.T) {
	tests := []struct {
		version     string
		expectMatch bool
	}{
		{
			version:     "v1.4.0",
			expectMatch: true,
		},
		{
			version:     "v2.0.0",
			expectMatch: true,
		},
		{
			version:     "v1.5.0-alpha.1",
			expectMatch: true,
		},
		{
			version:     "v1.16.1320-beta.14",
			expectMatch: true,
		},
		{
			version:     "1.0.0",
			expectMatch: false,
		},
		{
			// this is true because while the "--" is invalid, v1.0.0 is a valid part of the regex
			version:     "v1.0.0--beta.1",
			expectMatch: true,
		},
	}

	for _, test := range tests {
		name := fmt.Sprintf("Testing version string %s", test.version)
		t.Run(name, func(t *testing.T) {
			results := reSubMatchMap(release_regex, test.version)

			if len(results) == 0 && test.expectMatch {
				t.Fail()
			}

			if len(results) > 0 && !test.expectMatch {
				fmt.Printf("%v", results)
				t.Fail()
			}
		})
	}
}
