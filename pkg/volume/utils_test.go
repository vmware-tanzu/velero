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

package volume

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetRestoredPVCFromRestoredResourceList(t *testing.T) {
	// test empty list
	restoredResourceList := map[string][]string{}
	actual := RestoredPVCFromRestoredResourceList(restoredResourceList)
	assert.Empty(t, actual)

	// test no match
	restoredResourceList = map[string][]string{
		"v1/PersistentVolumeClaim": {
			"namespace1/pvc1(updated)",
		},
		"v1/PersistentVolume": {
			"namespace1/pv(created)",
		},
	}
	actual = RestoredPVCFromRestoredResourceList(restoredResourceList)
	assert.Empty(t, actual)

	// test matches
	restoredResourceList = map[string][]string{
		"v1/PersistentVolumeClaim": {
			"namespace1/pvc1(created)",
			"namespace2/pvc2(updated)",
			"namespace3/pvc(3)(created)",
		},
	}
	expected := map[string]struct{}{
		"namespace1/pvc1":   {},
		"namespace3/pvc(3)": {},
	}
	actual = RestoredPVCFromRestoredResourceList(restoredResourceList)
	assert.Equal(t, expected, actual)
}
