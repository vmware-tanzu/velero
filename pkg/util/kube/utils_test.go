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

package kube

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/heptio/ark/pkg/util/collections"
)

func TestSetVolumeID(t *testing.T) {
	tests := []struct {
		name                  string
		spec                  map[string]interface{}
		volumeID              string
		expectedErr           error
		specFieldExpectations map[string]string
	}{
		{
			name: "awsElasticBlockStore normal case",
			spec: map[string]interface{}{
				"awsElasticBlockStore": map[string]interface{}{
					"volumeID": "vol-old",
				},
			},
			volumeID:    "vol-new",
			expectedErr: nil,
		},
		{
			name: "gcePersistentDisk normal case",
			spec: map[string]interface{}{
				"gcePersistentDisk": map[string]interface{}{
					"pdName": "old-pd",
				},
			},
			volumeID:    "new-pd",
			expectedErr: nil,
		},
		{
			name: "azureDisk normal case",
			spec: map[string]interface{}{
				"azureDisk": map[string]interface{}{
					"diskName": "old-disk",
					"diskURI":  "some-nonsense/old-disk",
				},
			},
			volumeID:    "new-disk",
			expectedErr: nil,
			specFieldExpectations: map[string]string{
				"azureDisk.diskURI": "some-nonsense/new-disk",
			},
		},
		{
			name: "azureDisk with no diskURI",
			spec: map[string]interface{}{
				"azureDisk": map[string]interface{}{
					"diskName": "old-disk",
				},
			},
			volumeID:    "new-disk",
			expectedErr: nil,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := SetVolumeID(test.spec, test.volumeID)

			require.Equal(t, test.expectedErr, err)

			if test.expectedErr != nil {
				return
			}

			pv := map[string]interface{}{
				"spec": test.spec,
			}

			volumeID, err := GetVolumeID(pv)
			require.Nil(t, err)

			assert.Equal(t, test.volumeID, volumeID)

			for path, expected := range test.specFieldExpectations {
				actual, err := collections.GetString(test.spec, path)
				assert.Nil(t, err)
				assert.Equal(t, expected, actual)
			}
		})
	}
}
