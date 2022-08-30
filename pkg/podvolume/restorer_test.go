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

package podvolume

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetVolumesRepositoryType(t *testing.T) {
	testCases := []struct {
		name        string
		volumes     map[string]VolumeBackupInfo
		expected    string
		expectedErr string
	}{
		{
			name:        "empty volume",
			expectedErr: "empty volume list",
		},
		{
			name: "empty repository type, first one",
			volumes: map[string]VolumeBackupInfo{
				"volume1": {"", "", ""},
				"volume2": {"", "", "fake-type"},
			},
			expectedErr: "invalid repository type among volumes",
		},
		{
			name: "empty repository type, last one",
			volumes: map[string]VolumeBackupInfo{
				"volume1": {"", "", "fake-type"},
				"volume2": {"", "", "fake-type"},
				"volume3": {"", "", ""},
			},
			expectedErr: "invalid repository type among volumes",
		},
		{
			name: "empty repository type, middle one",
			volumes: map[string]VolumeBackupInfo{
				"volume1": {"", "", "fake-type"},
				"volume2": {"", "", ""},
				"volume3": {"", "", "fake-type"},
			},
			expectedErr: "invalid repository type among volumes",
		},
		{
			name: "mismatch repository type",
			volumes: map[string]VolumeBackupInfo{
				"volume1": {"", "", "fake-type1"},
				"volume2": {"", "", "fake-type2"},
			},
			expectedErr: "multiple repository type in one backup",
		},
		{
			name: "success",
			volumes: map[string]VolumeBackupInfo{
				"volume1": {"", "", "fake-type"},
				"volume2": {"", "", "fake-type"},
				"volume3": {"", "", "fake-type"},
			},
			expected: "fake-type",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actual, err := getVolumesRepositoryType(tc.volumes)
			assert.Equal(t, tc.expected, actual)

			if err != nil {
				assert.EqualError(t, err, tc.expectedErr)
			}
		})

	}
}
