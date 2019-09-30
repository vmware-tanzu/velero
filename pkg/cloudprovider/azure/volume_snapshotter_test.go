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

package azure

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
)

func TestGetVolumeID(t *testing.T) {
	b := &VolumeSnapshotter{}

	pv := &unstructured.Unstructured{
		Object: map[string]interface{}{},
	}

	// missing spec.azureDisk -> no error
	volumeID, err := b.GetVolumeID(pv)
	require.NoError(t, err)
	assert.Equal(t, "", volumeID)

	// missing spec.azureDisk.diskName -> error
	azure := map[string]interface{}{}
	pv.Object["spec"] = map[string]interface{}{
		"azureDisk": azure,
	}
	volumeID, err = b.GetVolumeID(pv)
	assert.Error(t, err)
	assert.Equal(t, "", volumeID)

	// valid
	azure["diskName"] = "foo"
	volumeID, err = b.GetVolumeID(pv)
	assert.NoError(t, err)
	assert.Equal(t, "foo", volumeID)
}

func TestSetVolumeID(t *testing.T) {
	b := &VolumeSnapshotter{
		disksResourceGroup: "rg",
		disksSubscription:  "sub",
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

	res := new(v1.PersistentVolume)
	require.NoError(t, runtime.DefaultUnstructuredConverter.FromUnstructured(updatedPV.UnstructuredContent(), res))
	require.NotNil(t, res.Spec.AzureDisk)
	assert.Equal(t, "updated", res.Spec.AzureDisk.DiskName)
	assert.Equal(t, "/subscriptions/sub/resourceGroups/rg/providers/Microsoft.Compute/disks/updated", res.Spec.AzureDisk.DataDiskURI)

	// with diskURI
	azure["diskURI"] = "/foo/bar/updated/blarg"
	updatedPV, err = b.SetVolumeID(pv, "revised")
	require.NoError(t, err)

	res = new(v1.PersistentVolume)
	require.NoError(t, runtime.DefaultUnstructuredConverter.FromUnstructured(updatedPV.UnstructuredContent(), res))
	require.NotNil(t, res.Spec.AzureDisk)
	assert.Equal(t, "revised", res.Spec.AzureDisk.DiskName)
	assert.Equal(t, "/subscriptions/sub/resourceGroups/rg/providers/Microsoft.Compute/disks/revised", res.Spec.AzureDisk.DataDiskURI)
}

func TestParseFullSnapshotName(t *testing.T) {
	// invalid name
	fullName := "foo/bar"
	_, err := parseFullSnapshotName(fullName)
	assert.Error(t, err)

	// valid name (current format)
	fullName = "/subscriptions/sub-1/resourceGroups/rg-1/providers/Microsoft.Compute/snapshots/snap-1"
	snap, err := parseFullSnapshotName(fullName)
	require.NoError(t, err)

	assert.Equal(t, "sub-1", snap.subscription)
	assert.Equal(t, "rg-1", snap.resourceGroup)
	assert.Equal(t, "snap-1", snap.name)
}

func TestGetComputeResourceName(t *testing.T) {
	assert.Equal(t, "/subscriptions/sub-1/resourceGroups/rg-1/providers/Microsoft.Compute/disks/disk-1", getComputeResourceName("sub-1", "rg-1", disksResource, "disk-1"))

	assert.Equal(t, "/subscriptions/sub-1/resourceGroups/rg-1/providers/Microsoft.Compute/snapshots/snap-1", getComputeResourceName("sub-1", "rg-1", snapshotsResource, "snap-1"))
}

func TestGetSnapshotTags(t *testing.T) {
	tests := []struct {
		name       string
		veleroTags map[string]string
		diskTags   map[string]*string
		expected   map[string]*string
	}{
		{
			name:       "degenerate case (no tags)",
			veleroTags: nil,
			diskTags:   nil,
			expected:   nil,
		},
		{
			name: "velero tags only get applied",
			veleroTags: map[string]string{
				"velero-key1": "velero-val1",
				"velero-key2": "velero-val2",
			},
			diskTags: nil,
			expected: map[string]*string{
				"velero-key1": stringPtr("velero-val1"),
				"velero-key2": stringPtr("velero-val2"),
			},
		},
		{
			name: "slashes in velero tag keys get replaces with dashes",
			veleroTags: map[string]string{
				"velero/key1":  "velero-val1",
				"velero/key/2": "velero-val2",
			},
			diskTags: nil,
			expected: map[string]*string{
				"velero-key1":  stringPtr("velero-val1"),
				"velero-key-2": stringPtr("velero-val2"),
			},
		},
		{
			name:       "volume tags only get applied",
			veleroTags: nil,
			diskTags: map[string]*string{
				"azure-key1": stringPtr("azure-val1"),
				"azure-key2": stringPtr("azure-val2"),
			},
			expected: map[string]*string{
				"azure-key1": stringPtr("azure-val1"),
				"azure-key2": stringPtr("azure-val2"),
			},
		},
		{
			name:       "non-overlapping velero and volume tags both get applied",
			veleroTags: map[string]string{"velero-key": "velero-val"},
			diskTags:   map[string]*string{"azure-key": stringPtr("azure-val")},
			expected: map[string]*string{
				"velero-key": stringPtr("velero-val"),
				"azure-key":  stringPtr("azure-val"),
			},
		},
		{
			name: "when tags overlap, velero tags take precedence",
			veleroTags: map[string]string{
				"velero-key":      "velero-val",
				"overlapping-key": "velero-val",
			},
			diskTags: map[string]*string{
				"azure-key":       stringPtr("azure-val"),
				"overlapping-key": stringPtr("azure-val"),
			},
			expected: map[string]*string{
				"velero-key":      stringPtr("velero-val"),
				"azure-key":       stringPtr("azure-val"),
				"overlapping-key": stringPtr("velero-val"),
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			res := getSnapshotTags(test.veleroTags, test.diskTags)

			if test.expected == nil {
				assert.Nil(t, res)
				return
			}

			assert.Equal(t, len(test.expected), len(res))
			for k, v := range test.expected {
				assert.Equal(t, v, res[k])
			}
		})
	}
}
