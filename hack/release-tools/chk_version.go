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
	"os"
	"regexp"
)

// This regex should match both our GA format (example: v1.4.3) and pre-release formats (v1.2.4-beta.2, v1.5.0-rc.1)
// The following sub-capture groups are defined:
//
//	major
//	minor
//	patch
//	prerelease (this will be alpha/beta/rc followed by a ".", followed by 1 or more digits (alpha.5)
var release_regex *regexp.Regexp = regexp.MustCompile(`^v(?P<major>[[:digit:]]+)\.(?P<minor>[[:digit:]]+)\.(?P<patch>[[:digit:]]+)(-{1}(?P<prerelease>(alpha|beta|rc)\.[[:digit:]]+))*`)

// This small program exists because checking the VELERO_VERSION rules in bash is difficult, and difficult to test for correctness.
// Calling it with --verify will verify whether or not the VELERO_VERSION environment variable is a valid version string, without parsing for its components.
// Calling it without --verify will try to parse the version into its component pieces.
func main() {

	velero_version := os.Getenv("VELERO_VERSION")

	submatches := reSubMatchMap(release_regex, velero_version)

	// Didn't match the regex, exit.
	if len(submatches) == 0 {
		fmt.Printf("VELERO_VERSION of %s was not valid. Please correct the value and retry.", velero_version)
		os.Exit(1)
	}

	if len(os.Args) > 1 && os.Args[1] == "--verify" {
		os.Exit(0)
	}

	// Send these in a bash variable format to stdout, so that they can be consumed by bash scripts that call the go program.
	fmt.Printf("VELERO_MAJOR=%s\n", submatches["major"])
	fmt.Printf("VELERO_MINOR=%s\n", submatches["minor"])
	fmt.Printf("VELERO_PATCH=%s\n", submatches["patch"])
	fmt.Printf("VELERO_PRERELEASE=%s\n", submatches["prerelease"])
}

// reSubMatchMap returns a map with the named submatches within a regular expression populated as keys, and their matched values within a given string as values.
// If no matches are found, a nil map is returned
func reSubMatchMap(r *regexp.Regexp, s string) map[string]string {
	match := r.FindStringSubmatch(s)
	submatches := make(map[string]string)
	if len(match) == 0 {
		return submatches
	}
	for i, name := range r.SubexpNames() {
		// 0 will always be empty from the return values of SubexpNames's documentation, so skip it.
		if i != 0 {
			submatches[name] = match[i]
		}
	}

	return submatches
}
