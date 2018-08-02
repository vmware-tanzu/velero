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

package azure

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestGetVolumeID(t *testing.T) {
	b := &blockStore{}

	pv := &unstructured.Unstructured{
		Object: map[string]interface{}{},
	}

	// missing spec.azureDisk -> no error
	volumeID, err := b.GetVolumeID(pv)
	require.NoError(t, err)
	assert.Equal(t, "", volumeID)

	// missing spec.azureDisk.diskName -> no error
	azure := map[string]interface{}{}
	pv.Object["spec"] = map[string]interface{}{
		"azureDisk": azure,
	}
	volumeID, err = b.GetVolumeID(pv)
	assert.NoError(t, err)
	assert.Equal(t, "", volumeID)

	// valid
	azure["diskName"] = "foo"
	volumeID, err = b.GetVolumeID(pv)
	assert.NoError(t, err)
	assert.Equal(t, "foo", volumeID)
}

func TestSetVolumeID(t *testing.T) {
	b := &blockStore{
		resourceGroup: "rg",
		subscription:  "sub",
	}

	pv := &unstructured.Unstructured{
		Object: map[string]interface{}{},
	}

	// missing spec.azureDisk -> error
	updatedPV, err := b.SetVolumeID(pv, "updated")
	require.Error(t, err)

	// happy path, no diskURI
	azure := map[string]interface{}{}
	pv.Object["spec"] = map[string]interface{}{
		"azureDisk": azure,
	}
	updatedPV, err = b.SetVolumeID(pv, "updated")
	require.NoError(t, err)
	actual, _, err := unstructured.NestedString(updatedPV.UnstructuredContent(), "spec", "azureDisk", "diskName")
	require.NoError(t, err)
	assert.Equal(t, "updated", actual)
	actual, _, err = unstructured.NestedString(updatedPV.UnstructuredContent(), "spec", "azureDisk", "diskURI")
	require.NoError(t, err)
	assert.Equal(t, "/subscriptions/sub/resourceGroups/rg/providers/Microsoft.Compute/disks/updated", actual)

	// with diskURI
	azure["diskURI"] = "/foo/bar/updated/blarg"
	updatedPV, err = b.SetVolumeID(pv, "revised")
	require.NoError(t, err)
	actual, _, err = unstructured.NestedString(updatedPV.UnstructuredContent(), "spec", "azureDisk", "diskName")
	require.NoError(t, err)
	assert.Equal(t, "revised", actual)
	actual, _, err = unstructured.NestedString(updatedPV.UnstructuredContent(), "spec", "azureDisk", "diskURI")
	require.NoError(t, err)
	assert.Equal(t, "/subscriptions/sub/resourceGroups/rg/providers/Microsoft.Compute/disks/revised", actual)
}

// TODO(1.0) rename to TestParseFullSnapshotName, switch to testing
// the `parseFullSnapshotName` function, and remove case for legacy
// format
func TestParseSnapshotName(t *testing.T) {
	b := &blockStore{
		subscription:  "default-sub",
		resourceGroup: "default-rg",
	}

	// invalid name
	fullName := "foo/bar"
	_, err := b.parseSnapshotName(fullName)
	assert.Error(t, err)

	// valid name (current format)
	fullName = "/subscriptions/sub-1/resourceGroups/rg-1/providers/Microsoft.Compute/snapshots/snap-1"
	snap, err := b.parseSnapshotName(fullName)
	require.NoError(t, err)

	assert.Equal(t, "sub-1", snap.subscription)
	assert.Equal(t, "rg-1", snap.resourceGroup)
	assert.Equal(t, "snap-1", snap.name)

	// valid name (legacy format)
	// TODO(1.0) remove this test case
	fullName = "foobar"
	snap, err = b.parseSnapshotName(fullName)
	require.NoError(t, err)
	assert.Equal(t, b.subscription, snap.subscription)
	assert.Equal(t, b.resourceGroup, snap.resourceGroup)
	assert.Equal(t, fullName, snap.name)

}

func TestGetComputeResourceName(t *testing.T) {
	assert.Equal(t, "/subscriptions/sub-1/resourceGroups/rg-1/providers/Microsoft.Compute/disks/disk-1", getComputeResourceName("sub-1", "rg-1", disksResource, "disk-1"))

	assert.Equal(t, "/subscriptions/sub-1/resourceGroups/rg-1/providers/Microsoft.Compute/snapshots/snap-1", getComputeResourceName("sub-1", "rg-1", snapshotsResource, "snap-1"))
}

func TestGetSnapshotTags(t *testing.T) {
	tests := []struct {
		name     string
		arkTags  map[string]string
		diskTags *map[string]*string
		expected *map[string]*string
	}{
		{
			name:     "degenerate case (no tags)",
			arkTags:  nil,
			diskTags: nil,
			expected: nil,
		},
		{
			name: "ark tags only get applied",
			arkTags: map[string]string{
				"ark-key1": "ark-val1",
				"ark-key2": "ark-val2",
			},
			diskTags: nil,
			expected: &map[string]*string{
				"ark-key1": stringPtr("ark-val1"),
				"ark-key2": stringPtr("ark-val2"),
			},
		},
		{
			name: "slashes in ark tag keys get replaces with dashes",
			arkTags: map[string]string{
				"ark/key1":  "ark-val1",
				"ark/key/2": "ark-val2",
			},
			diskTags: nil,
			expected: &map[string]*string{
				"ark-key1":  stringPtr("ark-val1"),
				"ark-key-2": stringPtr("ark-val2"),
			},
		},
		{
			name:    "volume tags only get applied",
			arkTags: nil,
			diskTags: &map[string]*string{
				"azure-key1": stringPtr("azure-val1"),
				"azure-key2": stringPtr("azure-val2"),
			},
			expected: &map[string]*string{
				"azure-key1": stringPtr("azure-val1"),
				"azure-key2": stringPtr("azure-val2"),
			},
		},
		{
			name:     "non-overlapping ark and volume tags both get applied",
			arkTags:  map[string]string{"ark-key": "ark-val"},
			diskTags: &map[string]*string{"azure-key": stringPtr("azure-val")},
			expected: &map[string]*string{
				"ark-key":   stringPtr("ark-val"),
				"azure-key": stringPtr("azure-val"),
			},
		},
		{
			name: "when tags overlap, ark tags take precedence",
			arkTags: map[string]string{
				"ark-key":         "ark-val",
				"overlapping-key": "ark-val",
			},
			diskTags: &map[string]*string{
				"azure-key":       stringPtr("azure-val"),
				"overlapping-key": stringPtr("azure-val"),
			},
			expected: &map[string]*string{
				"ark-key":         stringPtr("ark-val"),
				"azure-key":       stringPtr("azure-val"),
				"overlapping-key": stringPtr("ark-val"),
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			res := getSnapshotTags(test.arkTags, test.diskTags)

			if test.expected == nil {
				assert.Nil(t, res)
				return
			}

			assert.Equal(t, len(*test.expected), len(*res))
			for k, v := range *test.expected {
				assert.Equal(t, v, (*res)[k])
			}
		})
	}
}
