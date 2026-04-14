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

package velero

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func Test_VersionNoOlderThan(t *testing.T) {
	type versionTest struct {
		caseName      string
		version       string
		targetVersion string
		result        bool
		err           error
	}
	tests := []versionTest{
		{
			caseName:      "branch version compare",
			version:       "release-1.18",
			targetVersion: "v1.16",
			result:        true,
			err:           nil,
		},
		{
			caseName:      "tag version compare",
			version:       "v1.18.0",
			targetVersion: "v1.16",
			result:        true,
			err:           nil,
		},
		{
			caseName:      "main version compare",
			version:       "main",
			targetVersion: "v1.15",
			result:        true,
			err:           nil,
		},
	}

	for _, test := range tests {
		t.Run(test.caseName, func(t *testing.T) {
			res, err := VersionNoOlderThan(test.version, test.targetVersion)

			require.Equal(t, test.err, err)
			require.Equal(t, test.result, res)
		})
	}
}
