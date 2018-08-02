/*
Copyright 2017 the Heptio Ark contributors.

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

package gcp

import (
	"encoding/json"
	"testing"

	arktest "github.com/heptio/ark/pkg/util/test"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestGetVolumeID(t *testing.T) {
	b := &blockStore{}

	pv := &unstructured.Unstructured{
		Object: map[string]interface{}{},
	}

	// missing spec.gcePersistentDisk -> no error
	volumeID, err := b.GetVolumeID(pv)
	require.NoError(t, err)
	assert.Equal(t, "", volumeID)

	// missing spec.gcePersistentDisk.pdName -> no error
	gce := map[string]interface{}{}
	pv.Object["spec"] = map[string]interface{}{
		"gcePersistentDisk": gce,
	}
	volumeID, err = b.GetVolumeID(pv)
	assert.NoError(t, err)
	assert.Equal(t, "", volumeID)

	// valid
	gce["pdName"] = "abc123"
	volumeID, err = b.GetVolumeID(pv)
	assert.NoError(t, err)
	assert.Equal(t, "abc123", volumeID)
}

func TestSetVolumeID(t *testing.T) {
	b := &blockStore{}

	pv := &unstructured.Unstructured{
		Object: map[string]interface{}{},
	}

	// missing spec.gcePersistentDisk -> error
	updatedPV, err := b.SetVolumeID(pv, "abc123")
	require.Error(t, err)

	// happy path
	gce := map[string]interface{}{}
	pv.Object["spec"] = map[string]interface{}{
		"gcePersistentDisk": gce,
	}
	updatedPV, err = b.SetVolumeID(pv, "123abc")
	require.NoError(t, err)
	actual, _, err := unstructured.NestedString(updatedPV.UnstructuredContent(), "spec", "gcePersistentDisk", "pdName")
	require.NoError(t, err)
	assert.Equal(t, "123abc", actual)
}

func TestGetSnapshotTags(t *testing.T) {
	tests := []struct {
		name            string
		arkTags         map[string]string
		diskDescription string
		expected        string
	}{
		{
			name:            "degenerate case (no tags)",
			arkTags:         nil,
			diskDescription: "",
			expected:        "",
		},
		{
			name: "ark tags only get applied",
			arkTags: map[string]string{
				"ark-key1": "ark-val1",
				"ark-key2": "ark-val2",
			},
			diskDescription: "",
			expected:        `{"ark-key1":"ark-val1","ark-key2":"ark-val2"}`,
		},
		{
			name:            "disk tags only get applied",
			arkTags:         nil,
			diskDescription: `{"aws-key1":"aws-val1","aws-key2":"aws-val2"}`,
			expected:        `{"aws-key1":"aws-val1","aws-key2":"aws-val2"}`,
		},
		{
			name:            "non-overlapping ark and disk tags both get applied",
			arkTags:         map[string]string{"ark-key": "ark-val"},
			diskDescription: `{"aws-key":"aws-val"}`,
			expected:        `{"ark-key":"ark-val","aws-key":"aws-val"}`,
		},
		{
			name: "when tags overlap, ark tags take precedence",
			arkTags: map[string]string{
				"ark-key":         "ark-val",
				"overlapping-key": "ark-val",
			},
			diskDescription: `{"aws-key":"aws-val","overlapping-key":"aws-val"}`,
			expected:        `{"ark-key":"ark-val","aws-key":"aws-val","overlapping-key":"ark-val"}`,
		},
		{
			name:            "if disk description is invalid JSON, apply just ark tags",
			arkTags:         map[string]string{"ark-key": "ark-val"},
			diskDescription: `THIS IS INVALID JSON`,
			expected:        `{"ark-key":"ark-val"}`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			res := getSnapshotTags(test.arkTags, test.diskDescription, arktest.NewLogger())

			if test.expected == "" {
				assert.Equal(t, test.expected, res)
				return
			}

			var actualMap map[string]interface{}
			require.NoError(t, json.Unmarshal([]byte(res), &actualMap))

			var expectedMap map[string]interface{}
			require.NoError(t, json.Unmarshal([]byte(test.expected), &expectedMap))

			assert.Equal(t, len(expectedMap), len(actualMap))
			for k, v := range expectedMap {
				assert.Equal(t, v, actualMap[k])
			}
		})
	}
}
