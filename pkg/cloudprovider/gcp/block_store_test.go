/*
Copyright 2017 the Velero contributors.

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

	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"

	velerotest "github.com/heptio/velero/pkg/util/test"
)

func TestGetVolumeID(t *testing.T) {
	b := &BlockStore{}

	pv := &unstructured.Unstructured{
		Object: map[string]interface{}{},
	}

	// missing spec.gcePersistentDisk -> no error
	volumeID, err := b.GetVolumeID(pv)
	require.NoError(t, err)
	assert.Equal(t, "", volumeID)

	// missing spec.gcePersistentDisk.pdName -> error
	gce := map[string]interface{}{}
	pv.Object["spec"] = map[string]interface{}{
		"gcePersistentDisk": gce,
	}
	volumeID, err = b.GetVolumeID(pv)
	assert.Error(t, err)
	assert.Equal(t, "", volumeID)

	// valid
	gce["pdName"] = "abc123"
	volumeID, err = b.GetVolumeID(pv)
	assert.NoError(t, err)
	assert.Equal(t, "abc123", volumeID)
}

func TestSetVolumeID(t *testing.T) {
	b := &BlockStore{}

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

	res := new(v1.PersistentVolume)
	require.NoError(t, runtime.DefaultUnstructuredConverter.FromUnstructured(updatedPV.UnstructuredContent(), res))
	require.NotNil(t, res.Spec.GCEPersistentDisk)
	assert.Equal(t, "123abc", res.Spec.GCEPersistentDisk.PDName)
}

func TestGetSnapshotTags(t *testing.T) {
	tests := []struct {
		name            string
		veleroTags      map[string]string
		diskDescription string
		expected        string
	}{
		{
			name:            "degenerate case (no tags)",
			veleroTags:      nil,
			diskDescription: "",
			expected:        "",
		},
		{
			name: "velero tags only get applied",
			veleroTags: map[string]string{
				"velero-key1": "velero-val1",
				"velero-key2": "velero-val2",
			},
			diskDescription: "",
			expected:        `{"velero-key1":"velero-val1","velero-key2":"velero-val2"}`,
		},
		{
			name:            "disk tags only get applied",
			veleroTags:      nil,
			diskDescription: `{"aws-key1":"aws-val1","aws-key2":"aws-val2"}`,
			expected:        `{"aws-key1":"aws-val1","aws-key2":"aws-val2"}`,
		},
		{
			name:            "non-overlapping velero and disk tags both get applied",
			veleroTags:      map[string]string{"velero-key": "velero-val"},
			diskDescription: `{"aws-key":"aws-val"}`,
			expected:        `{"velero-key":"velero-val","aws-key":"aws-val"}`,
		},
		{
			name: "when tags overlap, velero tags take precedence",
			veleroTags: map[string]string{
				"velero-key":      "velero-val",
				"overlapping-key": "velero-val",
			},
			diskDescription: `{"aws-key":"aws-val","overlapping-key":"aws-val"}`,
			expected:        `{"velero-key":"velero-val","aws-key":"aws-val","overlapping-key":"velero-val"}`,
		},
		{
			name:            "if disk description is invalid JSON, apply just velero tags",
			veleroTags:      map[string]string{"velero-key": "velero-val"},
			diskDescription: `THIS IS INVALID JSON`,
			expected:        `{"velero-key":"velero-val"}`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			res := getSnapshotTags(test.veleroTags, test.diskDescription, velerotest.NewLogger())

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

func TestRegionHelpers(t *testing.T) {
	tests := []struct {
		name                string
		volumeAZ            string
		expectedRegion      string
		expectedIsMultiZone bool
		expectedError       error
	}{
		{
			name:                "valid multizone(2) tag",
			volumeAZ:            "us-central1-a__us-central1-b",
			expectedRegion:      "us-central1",
			expectedIsMultiZone: true,
			expectedError:       nil,
		},
		{
			name:                "valid multizone(4) tag",
			volumeAZ:            "us-central1-a__us-central1-b__us-central1-f__us-central1-e",
			expectedRegion:      "us-central1",
			expectedIsMultiZone: true,
			expectedError:       nil,
		},
		{
			name:                "valid single zone tag",
			volumeAZ:            "us-central1-a",
			expectedRegion:      "us-central1",
			expectedIsMultiZone: false,
			expectedError:       nil,
		},
		{
			name:                "invalid single zone tag",
			volumeAZ:            "us^central1^a",
			expectedRegion:      "",
			expectedIsMultiZone: false,
			expectedError:       errors.Errorf("failed to parse region from zone: %q", "us^central1^a"),
		},
		{
			name:                "invalid multizone tag",
			volumeAZ:            "us^central1^a__us^central1^b",
			expectedRegion:      "",
			expectedIsMultiZone: true,
			expectedError:       errors.Errorf("failed to parse region from zone: %q", "us^central1^a__us^central1^b"),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.expectedIsMultiZone, isMultiZone(test.volumeAZ))
			region, err := parseRegion(test.volumeAZ)
			if test.expectedError == nil {
				assert.NoError(t, err)
			} else {
				assert.Equal(t, test.expectedError.Error(), err.Error())
			}
			assert.Equal(t, test.expectedRegion, region)
		})
	}
}
