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

package alibabacloud

import (
	"os"
	"sort"
	"testing"

	"github.com/aliyun/alibaba-cloud-sdk-go/services/ecs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/heptio/velero/pkg/util/test"
)

func TestGetJSONArrayString(t *testing.T) {
	result := getJSONArrayString("d-test")
	assert.Equal(t, "[\"d-test\"]", result)
}

func TestGetVolumeIDFlexVolume(t *testing.T) {
	b := NewVolumeSnapshotter(test.NewLogger())

	pv := &unstructured.Unstructured{
		Object: map[string]interface{}{},
	}

	// missing spec.CSI and spec.FlexVolume -> no error
	volumeID, err := b.GetVolumeID(pv)
	require.NoError(t, err)
	assert.Equal(t, "", volumeID)

	// missing spec.flexvolume.options.volumeID -> error
	options := map[string]interface{}{}

	flexVolume := map[string]interface{}{
		"driver":  "alicloud/disk",
		"options": options,
	}
	pv.Object["spec"] = map[string]interface{}{
		"flexVolume": flexVolume,
	}
	volumeID, err = b.GetVolumeID(pv)
	assert.Error(t, err)
	assert.Equal(t, "", volumeID)

	// regex miss
	options["volumeId"] = "foo"
	volumeID, err = b.GetVolumeID(pv)
	assert.NoError(t, err)
	assert.Equal(t, "foo", volumeID)

	// regex match 1
	options["volumeId"] = "vol-abc123"
	volumeID, err = b.GetVolumeID(pv)
	assert.NoError(t, err)
	assert.Equal(t, "vol-abc123", volumeID)

	// regex match 2
	options["volumeId"] = "vol-abc123"
	volumeID, err = b.GetVolumeID(pv)
	assert.NoError(t, err)
	assert.Equal(t, "vol-abc123", volumeID)
}

func TestSetVolumeIDFlexVolume(t *testing.T) {
	b := &VolumeSnapshotter{}

	pv := &unstructured.Unstructured{
		Object: map[string]interface{}{},
	}

	// missing spec.FlexVolume -> no error
	updatedPV, err := b.SetVolumeID(pv, "vol-updated")
	require.Error(t, err)

	// happy path
	flexVolume := map[string]interface{}{
		"driver": "alicloud/disk",
	}

	pv.Object["spec"] = map[string]interface{}{
		"flexVolume": flexVolume,
	}

	labels := map[string]interface{}{
		"failure-domain.beta.kubernetes.io/zone": "cn-hangzhou-c",
	}

	pv.Object["metadata"] = map[string]interface{}{
		"labels": labels,
	}

	updatedPV, err = b.SetVolumeID(pv, "vol-updated")

	require.NoError(t, err)

	res := new(v1.PersistentVolume)
	require.NoError(t, runtime.DefaultUnstructuredConverter.FromUnstructured(updatedPV.UnstructuredContent(), res))
	require.NotNil(t, res.Spec.FlexVolume)
	diskId, err := getEBSDiskID(res)
	require.NoError(t, err)
	assert.Equal(t, "vol-updated", diskId)
}

func TestGetVolumeID(t *testing.T) {
	b := NewVolumeSnapshotter(test.NewLogger())

	pv := &unstructured.Unstructured{
		Object: map[string]interface{}{},
	}

	// missing spec.CSI and spec.FlexVolume -> no error
	volumeID, err := b.GetVolumeID(pv)
	require.NoError(t, err)
	assert.Equal(t, "", volumeID)

	// missing spec.csi.volumeAttributes.volumeID -> error
	csi := map[string]interface{}{
		"driver": "diskplugin.csi.alibabacloud.com",
	}
	pv.Object["spec"] = map[string]interface{}{
		"csi": csi,
	}
	volumeID, err = b.GetVolumeID(pv)
	assert.Error(t, err)
	assert.Equal(t, "", volumeID)

	// regex miss
	csi["volumeHandle"] = "foo"
	volumeID, err = b.GetVolumeID(pv)
	assert.NoError(t, err)
	assert.Equal(t, "foo", volumeID)

	// regex match 1
	csi["volumeHandle"] = "vol-abc123"
	volumeID, err = b.GetVolumeID(pv)
	assert.NoError(t, err)
	assert.Equal(t, "vol-abc123", volumeID)

	// regex match 2
	csi["volumeHandle"] = "vol-abc123"
	volumeID, err = b.GetVolumeID(pv)
	assert.NoError(t, err)
	assert.Equal(t, "vol-abc123", volumeID)
}

func TestSetVolumeID(t *testing.T) {
	b := &VolumeSnapshotter{}

	pv := &unstructured.Unstructured{
		Object: map[string]interface{}{},
	}

	// missing spec.CSI -> error
	updatedPV, err := b.SetVolumeID(pv, "vol-updated")
	require.Error(t, err)

	// happy path
	csi := map[string]interface{}{
		"driver": "diskplugin.csi.alibabacloud.com",
	}
	pv.Object["spec"] = map[string]interface{}{
		"csi": csi,
	}

	labels := map[string]interface{}{
		"failure-domain.beta.kubernetes.io/zone": "cn-hangzhou-c",
	}

	pv.Object["metadata"] = map[string]interface{}{
		"labels": labels,
	}

	updatedPV, err = b.SetVolumeID(pv, "vol-updated")

	require.NoError(t, err)

	res := new(v1.PersistentVolume)
	require.NoError(t, runtime.DefaultUnstructuredConverter.FromUnstructured(updatedPV.UnstructuredContent(), res))
	require.NotNil(t, res.Spec.CSI)
	diskId, err := getEBSDiskID(res)
	require.NoError(t, err)
	assert.Equal(t, "vol-updated", diskId)
}

func TestSetVolumeIDNoZone(t *testing.T) {
	b := &VolumeSnapshotter{}

	pv := &unstructured.Unstructured{
		Object: map[string]interface{}{},
	}

	// missing spec.CSI -> error
	updatedPV, err := b.SetVolumeID(pv, "vol-updated")
	require.Error(t, err)

	// happy path
	csi := map[string]interface{}{
		"driver": "diskplugin.csi.alibabacloud.com",
	}
	pv.Object["spec"] = map[string]interface{}{
		"csi": csi,
	}

	updatedPV, err = b.SetVolumeID(pv, "vol-updated")

	require.NoError(t, err)

	res := new(v1.PersistentVolume)
	require.NoError(t, runtime.DefaultUnstructuredConverter.FromUnstructured(updatedPV.UnstructuredContent(), res))
	require.NotNil(t, res.Spec.CSI)
	diskId, err := getEBSDiskID(res)
	require.NoError(t, err)
	assert.Equal(t, "vol-updated", diskId)
}

func TestGetTagsForCluster(t *testing.T) {
	tests := []struct {
		name         string
		isNameSet    bool
		snapshotTags []ecs.Tag
		expected     []ecs.CreateDiskTag
	}{
		{
			name:         "degenerate case (no tags)",
			isNameSet:    false,
			snapshotTags: nil,
			expected:     nil,
		},
		{
			name:      "cluster tags exist and remain set",
			isNameSet: false,
			snapshotTags: []ecs.Tag{
				{TagKey: "KubernetesCluster", TagValue: "old-cluster"},
				{TagKey: "kubernetes.io/cluster/old-cluster", TagValue: "owned"},
				{TagKey: "alibaba-cloud-key", TagValue: "alibaba-cloud-val"},
			},
			expected: []ecs.CreateDiskTag{
				{Key: "KubernetesCluster", Value: "old-cluster"},
				{Key: "kubernetes.io/cluster/old-cluster", Value: "owned"},
				{Key: "alibaba-cloud-key", Value: "alibaba-cloud-val"},
			},
		},
		{
			name:         "cluster tags only get applied",
			isNameSet:    true,
			snapshotTags: nil,
			expected: []ecs.CreateDiskTag{
				{Key: "KubernetesCluster", Value: "current-cluster"},
				{Key: "kubernetes.io/cluster/current-cluster", Value: "owned"},
			},
		},
		{
			name:      "non-overlaping cluster and snapshot tags both get applied",
			isNameSet: true,
			snapshotTags: []ecs.Tag{
				{TagKey: "alibaba-cloud-key", TagValue: "alibaba-cloud-val"},
			},
			expected: []ecs.CreateDiskTag{
				{Key: "KubernetesCluster", Value: "current-cluster"},
				{Key: "kubernetes.io/cluster/current-cluster", Value: "owned"},
				{Key: "alibaba-cloud-key", Value: "alibaba-cloud-val"},
			},
		},
		{name: "overlaping cluster tags, current cluster tags take precedence",
			isNameSet: true,
			snapshotTags: []ecs.Tag{
				{TagKey: "KubernetesCluster", TagValue: "old-name"},
				{TagKey: "kubernetes.io/cluster/old-name", TagValue: "owned"},
				{TagKey: "alibaba-cloud-key", TagValue: "alibaba-cloud-val"},
			},
			expected: []ecs.CreateDiskTag{
				{Key: "KubernetesCluster", Value: "current-cluster"},
				{Key: "kubernetes.io/cluster/current-cluster", Value: "owned"},
				{Key: "alibaba-cloud-key", Value: "alibaba-cloud-val"},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if test.isNameSet {
				os.Setenv(ackClusterNameKey, "current-cluster")
			}
			res := getTagsForCluster(test.snapshotTags)

			sort.Slice(res, func(i, j int) bool {
				return res[i].Key < res[j].Key
			})

			sort.Slice(test.expected, func(i, j int) bool {
				return test.expected[i].Key < test.expected[j].Key
			})

			assert.Equal(t, test.expected, res)

			if test.isNameSet {
				os.Unsetenv(ackClusterNameKey)
			}
		})
	}
}

func TestGetTags(t *testing.T) {
	tests := []struct {
		name       string
		veleroTags map[string]string
		volumeTags []ecs.Tag
		expected   []ecs.CreateSnapshotTag
	}{
		{
			name:       "degenerate case (no tags)",
			veleroTags: nil,
			volumeTags: nil,
			expected:   nil,
		},
		{
			name: "velero tags only get applied",
			veleroTags: map[string]string{
				"velero-key1": "velero-val1",
				"velero-key2": "velero-val2",
			},
			volumeTags: nil,
			expected: []ecs.CreateSnapshotTag{
				{Key: "velero-key1", Value: "velero-val1"},
				{Key: "velero-key2", Value: "velero-val2"},
			},
		},
		{
			name:       "volume tags only get applied",
			veleroTags: nil,
			volumeTags: []ecs.Tag{
				{TagKey: "alibaba-cloud-key1", TagValue: "alibaba-cloud-val1"},
				{TagKey: "alibaba-cloud-key2", TagValue: "alibaba-cloud-val2"},
			},
			expected: []ecs.CreateSnapshotTag{
				{Key: "alibaba-cloud-key1", Value: "alibaba-cloud-val1"},
				{Key: "alibaba-cloud-key2", Value: "alibaba-cloud-val2"},
			},
		},
		{
			name:       "non-overlapping velero and volume tags both get applied",
			veleroTags: map[string]string{"velero-key": "velero-val"},
			volumeTags: []ecs.Tag{
				{TagKey: "alibaba-cloud-key", TagValue: "alibaba-cloud-val"},
			},
			expected: []ecs.CreateSnapshotTag{
				{Key: "velero-key", Value: "velero-val"},
				{Key: "alibaba-cloud-key", Value: "alibaba-cloud-val"},
			},
		},
		{
			name: "when tags overlap, velero tags take precedence",
			veleroTags: map[string]string{
				"velero-key":      "velero-val",
				"overlapping-key": "velero-val",
			},
			volumeTags: []ecs.Tag{
				{TagKey: "alibaba-cloud-key", TagValue: "alibaba-cloud-val"},
				{TagKey: "overlapping-key", TagValue: "alibaba-cloud-val"},
			},
			expected: []ecs.CreateSnapshotTag{
				{Key: "velero-key", Value: "velero-val"},
				{Key: "overlapping-key", Value: "velero-val"},
				{Key: "alibaba-cloud-key", Value: "alibaba-cloud-val"},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			res := getTags(test.veleroTags, test.volumeTags)

			sort.Slice(res, func(i, j int) bool {
				return res[i].Key < res[j].Key
			})

			sort.Slice(test.expected, func(i, j int) bool {
				return test.expected[i].Key < test.expected[j].Key
			})

			assert.Equal(t, test.expected, res)
		})
	}
}
