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

	"github.com/heptio/ark/pkg/util/collections"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestGetVolumeID(t *testing.T) {
	b := &blockStore{}

	pv := &unstructured.Unstructured{}

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
	b := &blockStore{}

	pv := &unstructured.Unstructured{}

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
	actual, err := collections.GetString(updatedPV.UnstructuredContent(), "spec.azureDisk.diskName")
	require.NoError(t, err)
	assert.Equal(t, "updated", actual)
	assert.NotContains(t, azure, "diskURI")

	// with diskURI
	azure["diskURI"] = "/foo/bar/updated/blarg"
	updatedPV, err = b.SetVolumeID(pv, "revised")
	require.NoError(t, err)
	actual, err = collections.GetString(updatedPV.UnstructuredContent(), "spec.azureDisk.diskName")
	require.NoError(t, err)
	assert.Equal(t, "revised", actual)
	actual, err = collections.GetString(updatedPV.UnstructuredContent(), "spec.azureDisk.diskURI")
	require.NoError(t, err)
	assert.Equal(t, "/foo/bar/revised/blarg", actual)
}
