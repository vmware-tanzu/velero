/*
Copyright 2018 the Heptio Ark contributors.

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

package pluginmanagement

import (
	"testing"

	"github.com/Azure/go-autorest/autorest/to"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/heptio/velero/pkg/cloudprovider/mocks"
	"github.com/heptio/ark/pkg/plugin"
)

func TestRestartableGetBlockStore(t *testing.T) {
	tests := []struct {
		name          string
		plugin        interface{}
		getError      error
		expectedError string
	}{
		{
			name:          "error getting by kind and name",
			getError:      errors.Errorf("get error"),
			expectedError: "get error",
		},
		{
			name:          "wrong type",
			plugin:        3,
			expectedError: "int is not a BlockStore!",
		},
		{
			name:   "happy path",
			plugin: new(mocks.BlockStore),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			p := new(mockRestartableProcess)
			p.Test(t)
			defer p.AssertExpectations(t)

			name := "aws"
			key := kindAndName{kind: plugin.PluginKindBlockStore, name: name}
			p.On("getByKindAndName", key).Return(tc.plugin, tc.getError)

			r := &restartableBlockStore{
				key:                 key,
				sharedPluginProcess: p,
			}
			a, err := r.getBlockStore()
			if tc.expectedError != "" {
				assert.EqualError(t, err, tc.expectedError)
				return
			}
			require.NoError(t, err)

			assert.Equal(t, tc.plugin, a)
		})
	}
}

func TestRestartableBlockStoreReinitialize(t *testing.T) {
	p := new(mockRestartableProcess)
	p.Test(t)
	defer p.AssertExpectations(t)

	name := "aws"
	key := kindAndName{kind: plugin.PluginKindBlockStore, name: name}
	r := &restartableBlockStore{
		key:                 key,
		sharedPluginProcess: p,
		config: map[string]string{
			"color": "blue",
		},
	}

	err := r.reinitialize(3)
	assert.EqualError(t, err, "int is not a BlockStore!")

	blockStore := new(mocks.BlockStore)
	blockStore.Test(t)
	defer blockStore.AssertExpectations(t)

	blockStore.On("Init", r.config).Return(errors.Errorf("init error")).Once()
	err = r.reinitialize(blockStore)
	assert.EqualError(t, err, "init error")

	blockStore.On("Init", r.config).Return(nil)
	err = r.reinitialize(blockStore)
	assert.NoError(t, err)
}

func TestRestartableBlockStoreGetDelegate(t *testing.T) {
	p := new(mockRestartableProcess)
	p.Test(t)
	defer p.AssertExpectations(t)

	// Reset error
	p.On("resetIfNeeded").Return(errors.Errorf("reset error")).Once()
	name := "aws"
	key := kindAndName{kind: plugin.PluginKindBlockStore, name: name}
	r := &restartableBlockStore{
		key:                 key,
		sharedPluginProcess: p,
	}
	a, err := r.getDelegate()
	assert.Nil(t, a)
	assert.EqualError(t, err, "reset error")

	// Happy path
	p.On("resetIfNeeded").Return(nil)
	blockStore := new(mocks.BlockStore)
	blockStore.Test(t)
	defer blockStore.AssertExpectations(t)
	p.On("getByKindAndName", key).Return(blockStore, nil)

	a, err = r.getDelegate()
	assert.NoError(t, err)
	assert.Equal(t, blockStore, a)
}

func TestRestartableBlockStoreInit(t *testing.T) {
	p := new(mockRestartableProcess)
	p.Test(t)
	defer p.AssertExpectations(t)

	// getBlockStore error
	name := "aws"
	key := kindAndName{kind: plugin.PluginKindBlockStore, name: name}
	r := &restartableBlockStore{
		key:                 key,
		sharedPluginProcess: p,
	}
	p.On("getByKindAndName", key).Return(nil, errors.Errorf("getByKindAndName error")).Once()

	config := map[string]string{
		"color": "blue",
	}
	err := r.Init(config)
	assert.EqualError(t, err, "getByKindAndName error")

	// Delegate returns error
	blockStore := new(mocks.BlockStore)
	blockStore.Test(t)
	defer blockStore.AssertExpectations(t)
	p.On("getByKindAndName", key).Return(blockStore, nil)
	blockStore.On("Init", config).Return(errors.Errorf("Init error")).Once()

	err = r.Init(config)
	assert.EqualError(t, err, "Init error")

	// wipe this out because the previous failed Init call set it
	r.config = nil

	// Happy path
	blockStore.On("Init", config).Return(nil)
	err = r.Init(config)
	assert.NoError(t, err)
	assert.Equal(t, config, r.config)

	// Calling Init twice is forbidden
	err = r.Init(config)
	assert.EqualError(t, err, "already initialized")
}

func TestRestartableBlockStoreDelegatedFunctions(t *testing.T) {
	pv := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"color": "blue",
		},
	}

	pvToReturn := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"color": "green",
		},
	}

	runRestartableDelegateTests(
		t,
		plugin.PluginKindBlockStore,
		func(key kindAndName, p RestartableProcess) interface{} {
			return &restartableBlockStore{
				key:                 key,
				sharedPluginProcess: p,
			}
		},
		func() mockable {
			return new(mocks.BlockStore)
		},
		restartableDelegateTest{
			function:                "CreateVolumeFromSnapshot",
			inputs:                  []interface{}{"snapshotID", "volumeID", "volumeAZ", to.Int64Ptr(10000)},
			expectedErrorOutputs:    []interface{}{"", errors.Errorf("reset error")},
			expectedDelegateOutputs: []interface{}{"volumeID", errors.Errorf("delegate error")},
		},
		restartableDelegateTest{
			function:                "GetVolumeID",
			inputs:                  []interface{}{pv},
			expectedErrorOutputs:    []interface{}{"", errors.Errorf("reset error")},
			expectedDelegateOutputs: []interface{}{"volumeID", errors.Errorf("delegate error")},
		},
		restartableDelegateTest{
			function:                "SetVolumeID",
			inputs:                  []interface{}{pv, "volumeID"},
			expectedErrorOutputs:    []interface{}{nil, errors.Errorf("reset error")},
			expectedDelegateOutputs: []interface{}{pvToReturn, errors.Errorf("delegate error")},
		},
		restartableDelegateTest{
			function:                "GetVolumeInfo",
			inputs:                  []interface{}{"volumeID", "volumeAZ"},
			expectedErrorOutputs:    []interface{}{"", (*int64)(nil), errors.Errorf("reset error")},
			expectedDelegateOutputs: []interface{}{"volumeType", to.Int64Ptr(10000), errors.Errorf("delegate error")},
		},
		restartableDelegateTest{
			function:                "CreateSnapshot",
			inputs:                  []interface{}{"volumeID", "volumeAZ", map[string]string{"a": "b"}},
			expectedErrorOutputs:    []interface{}{"", errors.Errorf("reset error")},
			expectedDelegateOutputs: []interface{}{"snapshotID", errors.Errorf("delegate error")},
		},
		restartableDelegateTest{
			function:                "DeleteSnapshot",
			inputs:                  []interface{}{"snapshotID"},
			expectedErrorOutputs:    []interface{}{errors.Errorf("reset error")},
			expectedDelegateOutputs: []interface{}{errors.Errorf("delegate error")},
		},
	)
}
