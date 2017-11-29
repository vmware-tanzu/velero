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
	"testing"

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
