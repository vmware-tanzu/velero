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

func Test_getVersionWithoutPatch(t *testing.T) {
	versionTests := []struct {
		caseName string
		version  string
		result   string
	}{
		{
			caseName: "main version",
			version:  "main",
			result:   "main",
		},
		{
			caseName: "release version",
			version:  "release-1.18-dev",
			result:   "v1.18",
		},
		{
			caseName: "tag version",
			version:  "v1.17.2",
			result:   "v1.17",
		},
	}

	for _, test := range versionTests {
		t.Run(test.caseName, func(t *testing.T) {
			res := getVersionWithoutPatch(test.version)
			require.Equal(t, test.result, res)
		})
	}
}
