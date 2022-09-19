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
		volumes     map[string]volumeBackupInfo
		expected    string
		expectedErr string
		prefixOnly  bool
	}{
		{
			name:        "empty volume",
			expectedErr: "empty volume list",
		},
		{
			name: "empty repository type, first one",
			volumes: map[string]volumeBackupInfo{
				"volume1": {"fake-snapshot-id-1", "fake-uploader-1", ""},
				"volume2": {"", "", "fake-type"},
			},
			expectedErr: "empty repository type found among volume snapshots, snapshot ID fake-snapshot-id-1, uploader fake-uploader-1",
		},
		{
			name: "empty repository type, last one",
			volumes: map[string]volumeBackupInfo{
				"volume1": {"", "", "fake-type"},
				"volume2": {"", "", "fake-type"},
				"volume3": {"fake-snapshot-id-3", "fake-uploader-3", ""},
			},
			expectedErr: "empty repository type found among volume snapshots, snapshot ID fake-snapshot-id-3, uploader fake-uploader-3",
		},
		{
			name: "empty repository type, middle one",
			volumes: map[string]volumeBackupInfo{
				"volume1": {"", "", "fake-type"},
				"volume2": {"fake-snapshot-id-2", "fake-uploader-2", ""},
				"volume3": {"", "", "fake-type"},
			},
			expectedErr: "empty repository type found among volume snapshots, snapshot ID fake-snapshot-id-2, uploader fake-uploader-2",
		},
		{
			name: "mismatch repository type",
			volumes: map[string]volumeBackupInfo{
				"volume1": {"", "", "fake-type1"},
				"volume2": {"fake-snapshot-id-2", "fake-uploader-2", "fake-type2"},
			},
			prefixOnly:  true,
			expectedErr: "multiple repository type in one backup",
		},
		{
			name: "success",
			volumes: map[string]volumeBackupInfo{
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
				if tc.prefixOnly {
					errMsg := err.Error()
					if len(errMsg) >= len(tc.expectedErr) {
						errMsg = errMsg[0:len(tc.expectedErr)]
					}

					assert.Equal(t, tc.expectedErr, errMsg)
				} else {
					assert.EqualError(t, err, tc.expectedErr)
				}
			}
		})
	}
}
