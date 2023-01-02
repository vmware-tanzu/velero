/*
Copyright 2018 the Velero contributors.

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

package clientmgmt

import (
	"io"
	"strings"
	"testing"
	"time"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/vmware-tanzu/velero/internal/restartabletest"
	"github.com/vmware-tanzu/velero/pkg/plugin/clientmgmt/process"
	"github.com/vmware-tanzu/velero/pkg/plugin/framework/common"
	providermocks "github.com/vmware-tanzu/velero/pkg/plugin/velero/mocks"
)

func TestRestartableGetObjectStore(t *testing.T) {
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
			expectedError: "int is not a ObjectStore!",
		},
		{
			name:   "happy path",
			plugin: new(providermocks.ObjectStore),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			p := new(restartabletest.MockRestartableProcess)
			p.Test(t)
			defer p.AssertExpectations(t)

			name := "aws"
			key := process.KindAndName{Kind: common.PluginKindObjectStore, Name: name}
			p.On("GetByKindAndName", key).Return(tc.plugin, tc.getError)

			r := &restartableObjectStore{
				key:                 key,
				sharedPluginProcess: p,
			}
			a, err := r.getObjectStore()
			if tc.expectedError != "" {
				assert.EqualError(t, err, tc.expectedError)
				return
			}
			require.NoError(t, err)

			assert.Equal(t, tc.plugin, a)
		})
	}
}

func TestRestartableObjectStoreReinitialize(t *testing.T) {
	p := new(restartabletest.MockRestartableProcess)
	p.Test(t)
	defer p.AssertExpectations(t)

	name := "aws"
	key := process.KindAndName{Kind: common.PluginKindObjectStore, Name: name}
	r := &restartableObjectStore{
		key:                 key,
		sharedPluginProcess: p,
		config: map[string]string{
			"color": "blue",
		},
	}

	err := r.Reinitialize(3)
	assert.EqualError(t, err, "int is not a ObjectStore!")

	objectStore := new(providermocks.ObjectStore)
	objectStore.Test(t)
	defer objectStore.AssertExpectations(t)

	objectStore.On("Init", r.config).Return(errors.Errorf("init error")).Once()
	err = r.Reinitialize(objectStore)
	assert.EqualError(t, err, "init error")

	objectStore.On("Init", r.config).Return(nil)
	err = r.Reinitialize(objectStore)
	assert.NoError(t, err)
}

func TestRestartableObjectStoreGetDelegate(t *testing.T) {
	p := new(restartabletest.MockRestartableProcess)
	p.Test(t)
	defer p.AssertExpectations(t)

	// Reset error
	p.On("ResetIfNeeded").Return(errors.Errorf("reset error")).Once()
	name := "aws"
	key := process.KindAndName{Kind: common.PluginKindObjectStore, Name: name}
	r := &restartableObjectStore{
		key:                 key,
		sharedPluginProcess: p,
	}
	a, err := r.getDelegate()
	assert.Nil(t, a)
	assert.EqualError(t, err, "reset error")

	// Happy path
	p.On("ResetIfNeeded").Return(nil)
	objectStore := new(providermocks.ObjectStore)
	objectStore.Test(t)
	defer objectStore.AssertExpectations(t)
	p.On("GetByKindAndName", key).Return(objectStore, nil)

	a, err = r.getDelegate()
	assert.NoError(t, err)
	assert.Equal(t, objectStore, a)
}

func TestRestartableObjectStoreInit(t *testing.T) {
	p := new(restartabletest.MockRestartableProcess)
	p.Test(t)
	defer p.AssertExpectations(t)

	// getObjectStore error
	name := "aws"
	key := process.KindAndName{Kind: common.PluginKindObjectStore, Name: name}
	r := &restartableObjectStore{
		key:                 key,
		sharedPluginProcess: p,
	}
	p.On("GetByKindAndName", key).Return(nil, errors.Errorf("GetByKindAndName error")).Once()

	config := map[string]string{
		"color": "blue",
	}
	err := r.Init(config)
	assert.EqualError(t, err, "GetByKindAndName error")

	// Delegate returns error
	objectStore := new(providermocks.ObjectStore)
	objectStore.Test(t)
	defer objectStore.AssertExpectations(t)
	p.On("GetByKindAndName", key).Return(objectStore, nil)
	objectStore.On("Init", config).Return(errors.Errorf("Init error")).Once()

	err = r.Init(config)
	assert.EqualError(t, err, "Init error")

	// wipe this out because the previous failed Init call set it
	r.config = nil

	// Happy path
	objectStore.On("Init", config).Return(nil)
	err = r.Init(config)
	assert.NoError(t, err)
	assert.Equal(t, config, r.config)

	// Calling Init twice is forbidden
	err = r.Init(config)
	assert.EqualError(t, err, "already initialized")
}

func TestRestartableObjectStoreDelegatedFunctions(t *testing.T) {
	restartabletest.RunRestartableDelegateTests(
		t,
		common.PluginKindObjectStore,
		func(key process.KindAndName, p process.RestartableProcess) interface{} {
			return &restartableObjectStore{
				key:                 key,
				sharedPluginProcess: p,
			}
		},
		func() restartabletest.Mockable {
			return new(providermocks.ObjectStore)
		},
		restartabletest.RestartableDelegateTest{
			Function:                "PutObject",
			Inputs:                  []interface{}{"bucket", "key", strings.NewReader("body")},
			ExpectedErrorOutputs:    []interface{}{errors.Errorf("reset error")},
			ExpectedDelegateOutputs: []interface{}{errors.Errorf("delegate error")},
		},
		restartabletest.RestartableDelegateTest{
			Function:                "GetObject",
			Inputs:                  []interface{}{"bucket", "key"},
			ExpectedErrorOutputs:    []interface{}{nil, errors.Errorf("reset error")},
			ExpectedDelegateOutputs: []interface{}{io.NopCloser(strings.NewReader("object")), errors.Errorf("delegate error")},
		},
		restartabletest.RestartableDelegateTest{
			Function:                "ListCommonPrefixes",
			Inputs:                  []interface{}{"bucket", "prefix", "delimiter"},
			ExpectedErrorOutputs:    []interface{}{([]string)(nil), errors.Errorf("reset error")},
			ExpectedDelegateOutputs: []interface{}{[]string{"a", "b"}, errors.Errorf("delegate error")},
		},
		restartabletest.RestartableDelegateTest{
			Function:                "ListObjects",
			Inputs:                  []interface{}{"bucket", "prefix"},
			ExpectedErrorOutputs:    []interface{}{([]string)(nil), errors.Errorf("reset error")},
			ExpectedDelegateOutputs: []interface{}{[]string{"a", "b"}, errors.Errorf("delegate error")},
		},
		restartabletest.RestartableDelegateTest{
			Function:                "DeleteObject",
			Inputs:                  []interface{}{"bucket", "key"},
			ExpectedErrorOutputs:    []interface{}{errors.Errorf("reset error")},
			ExpectedDelegateOutputs: []interface{}{errors.Errorf("delegate error")},
		},
		restartabletest.RestartableDelegateTest{
			Function:                "CreateSignedURL",
			Inputs:                  []interface{}{"bucket", "key", 30 * time.Minute},
			ExpectedErrorOutputs:    []interface{}{"", errors.Errorf("reset error")},
			ExpectedDelegateOutputs: []interface{}{"signedURL", errors.Errorf("delegate error")},
		},
	)
}
