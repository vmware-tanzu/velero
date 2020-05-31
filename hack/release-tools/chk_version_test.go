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
