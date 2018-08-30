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

package aws

import (
	"os"
	"sort"
	"testing"

	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/heptio/ark/pkg/util/collections"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestGetVolumeID(t *testing.T) {
	b := &blockStore{}

	pv := &unstructured.Unstructured{}

	// missing spec.awsElasticBlockStore -> no error
	volumeID, err := b.GetVolumeID(pv)
	require.NoError(t, err)
	assert.Equal(t, "", volumeID)

	// missing spec.awsElasticBlockStore.volumeID -> error
	aws := map[string]interface{}{}
	pv.Object["spec"] = map[string]interface{}{
		"awsElasticBlockStore": aws,
	}
	volumeID, err = b.GetVolumeID(pv)
	assert.Error(t, err)
	assert.Equal(t, "", volumeID)

	// regex miss
	aws["volumeID"] = "foo"
	volumeID, err = b.GetVolumeID(pv)
	assert.NoError(t, err)
	assert.Equal(t, "", volumeID)

	// regex match 1
	aws["volumeID"] = "aws://us-east-1c/vol-abc123"
	volumeID, err = b.GetVolumeID(pv)
	assert.NoError(t, err)
	assert.Equal(t, "vol-abc123", volumeID)

	// regex match 2
	aws["volumeID"] = "vol-abc123"
	volumeID, err = b.GetVolumeID(pv)
	assert.NoError(t, err)
	assert.Equal(t, "vol-abc123", volumeID)
}

func TestSetVolumeID(t *testing.T) {
	b := &blockStore{}

	pv := &unstructured.Unstructured{}

	// missing spec.awsElasticBlockStore -> error
	updatedPV, err := b.SetVolumeID(pv, "vol-updated")
	require.Error(t, err)

	// happy path
	aws := map[string]interface{}{}
	pv.Object["spec"] = map[string]interface{}{
		"awsElasticBlockStore": aws,
	}
	updatedPV, err = b.SetVolumeID(pv, "vol-updated")
	require.NoError(t, err)
	actual, err := collections.GetString(updatedPV.UnstructuredContent(), "spec.awsElasticBlockStore.volumeID")
	require.NoError(t, err)
	assert.Equal(t, "vol-updated", actual)
}

func TestGetTagsForCluster(t *testing.T) {
	tests := []struct {
		name         string
		isNameSet    bool
		snapshotTags []*ec2.Tag
		expected     []*ec2.Tag
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
			snapshotTags: []*ec2.Tag{
				ec2Tag("KubernetesCluster", "old-cluster"),
				ec2Tag("kubernetes.io/cluster/old-cluster", "owned"),
				ec2Tag("aws-key", "aws-val"),
			},
			expected: []*ec2.Tag{
				ec2Tag("KubernetesCluster", "old-cluster"),
				ec2Tag("kubernetes.io/cluster/old-cluster", "owned"),
				ec2Tag("aws-key", "aws-val"),
			},
		},
		{
			name:         "cluster tags only get applied",
			isNameSet:    true,
			snapshotTags: nil,
			expected: []*ec2.Tag{
				ec2Tag("KubernetesCluster", "current-cluster"),
				ec2Tag("kubernetes.io/cluster/current-cluster", "owned"),
			},
		},
		{
			name:         "non-overlaping cluster and snapshot tags both get applied",
			isNameSet:    true,
			snapshotTags: []*ec2.Tag{ec2Tag("aws-key", "aws-val")},
			expected: []*ec2.Tag{
				ec2Tag("KubernetesCluster", "current-cluster"),
				ec2Tag("kubernetes.io/cluster/current-cluster", "owned"),
				ec2Tag("aws-key", "aws-val"),
			},
		},
		{name: "overlaping cluster tags, current cluster tags take precedence",
			isNameSet: true,
			snapshotTags: []*ec2.Tag{
				ec2Tag("KubernetesCluster", "old-name"),
				ec2Tag("kubernetes.io/cluster/old-name", "owned"),
				ec2Tag("aws-key", "aws-val"),
			},
			expected: []*ec2.Tag{
				ec2Tag("KubernetesCluster", "current-cluster"),
				ec2Tag("kubernetes.io/cluster/current-cluster", "owned"),
				ec2Tag("aws-key", "aws-val"),
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if test.isNameSet {
				os.Setenv("AWS_CLUSTER_NAME", "current-cluster")
			}
			res := getTagsForCluster(test.snapshotTags)

			sort.Slice(res, func(i, j int) bool {
				return *res[i].Key < *res[j].Key
			})

			sort.Slice(test.expected, func(i, j int) bool {
				return *test.expected[i].Key < *test.expected[j].Key
			})

			assert.Equal(t, test.expected, res)

			if test.isNameSet {
				os.Unsetenv("AWS_CLUSTER_NAME")
			}
		})
	}
}

func TestGetTags(t *testing.T) {
	tests := []struct {
		name       string
		arkTags    map[string]string
		volumeTags []*ec2.Tag
		expected   []*ec2.Tag
	}{
		{
			name:       "degenerate case (no tags)",
			arkTags:    nil,
			volumeTags: nil,
			expected:   nil,
		},
		{
			name: "ark tags only get applied",
			arkTags: map[string]string{
				"ark-key1": "ark-val1",
				"ark-key2": "ark-val2",
			},
			volumeTags: nil,
			expected: []*ec2.Tag{
				ec2Tag("ark-key1", "ark-val1"),
				ec2Tag("ark-key2", "ark-val2"),
			},
		},
		{
			name:    "volume tags only get applied",
			arkTags: nil,
			volumeTags: []*ec2.Tag{
				ec2Tag("aws-key1", "aws-val1"),
				ec2Tag("aws-key2", "aws-val2"),
			},
			expected: []*ec2.Tag{
				ec2Tag("aws-key1", "aws-val1"),
				ec2Tag("aws-key2", "aws-val2"),
			},
		},
		{
			name:       "non-overlapping ark and volume tags both get applied",
			arkTags:    map[string]string{"ark-key": "ark-val"},
			volumeTags: []*ec2.Tag{ec2Tag("aws-key", "aws-val")},
			expected: []*ec2.Tag{
				ec2Tag("ark-key", "ark-val"),
				ec2Tag("aws-key", "aws-val"),
			},
		},
		{
			name: "when tags overlap, ark tags take precedence",
			arkTags: map[string]string{
				"ark-key":         "ark-val",
				"overlapping-key": "ark-val",
			},
			volumeTags: []*ec2.Tag{
				ec2Tag("aws-key", "aws-val"),
				ec2Tag("overlapping-key", "aws-val"),
			},
			expected: []*ec2.Tag{
				ec2Tag("ark-key", "ark-val"),
				ec2Tag("overlapping-key", "ark-val"),
				ec2Tag("aws-key", "aws-val"),
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			res := getTags(test.arkTags, test.volumeTags)

			sort.Slice(res, func(i, j int) bool {
				return *res[i].Key < *res[j].Key
			})

			sort.Slice(test.expected, func(i, j int) bool {
				return *test.expected[i].Key < *test.expected[j].Key
			})

			assert.Equal(t, test.expected, res)
		})
	}
}
