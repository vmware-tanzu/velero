/*
Copyright 2019 the Heptio Ark contributors.

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

package openstack

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/heptio/velero/pkg/util/collections"
)

func TestGetVolumeID(t *testing.T) {
	b := &blockStore{}

	pv := &unstructured.Unstructured{
		Object: map[string]interface{}{},
	}

	// missing spec.cinder -> no error
	volumeID, err := b.GetVolumeID(pv)
	require.NoError(t, err)
	assert.Equal(t, "", volumeID)

	// missing spec.cinder.volumeID -> error
	cinder := map[string]interface{}{}
	pv.Object["spec"] = map[string]interface{}{
		"cinder": cinder,
	}
	volumeID, err = b.GetVolumeID(pv)
	assert.Error(t, err)
	assert.Equal(t, "", volumeID)

	// valid
	cinder["volumeID"] = "abc123"
	volumeID, err = b.GetVolumeID(pv)
	assert.NoError(t, err)
	assert.Equal(t, "abc123", volumeID)
}

func TestSetVolumeID(t *testing.T) {
	b := &blockStore{}

	pv := &unstructured.Unstructured{
		Object: map[string]interface{}{},
	}

	// missing spec.cinder -> error
	updatedPV, err := b.SetVolumeID(pv, "abc123")
	require.Error(t, err)

	// happy path
	cinder := map[string]interface{}{}
	pv.Object["spec"] = map[string]interface{}{
		"cinder": cinder,
	}
	updatedPV, err = b.SetVolumeID(pv, "123abc")
	require.NoError(t, err)
	actual, err := collections.GetString(updatedPV.UnstructuredContent(), "spec.cinder.volumeID")
	require.NoError(t, err)
	assert.Equal(t, "123abc", actual)
}
