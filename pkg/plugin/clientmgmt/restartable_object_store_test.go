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
	"io/ioutil"
	"strings"
	"testing"
	"time"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/vmware-tanzu/velero/pkg/plugin/framework"
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
			p := new(mockRestartableProcess)
			p.Test(t)
			defer p.AssertExpectations(t)

			name := "aws"
			key := kindAndName{kind: framework.PluginKindObjectStore, name: name}
			p.On("getByKindAndName", key).Return(tc.plugin, tc.getError)

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
	p := new(mockRestartableProcess)
	p.Test(t)
	defer p.AssertExpectations(t)

	name := "aws"
	key := kindAndName{kind: framework.PluginKindObjectStore, name: name}
	r := &restartableObjectStore{
		key:                 key,
		sharedPluginProcess: p,
		config: map[string]string{
			"color": "blue",
		},
	}

	err := r.reinitialize(3)
	assert.EqualError(t, err, "int is not a ObjectStore!")

	objectStore := new(providermocks.ObjectStore)
	objectStore.Test(t)
	defer objectStore.AssertExpectations(t)

	objectStore.On("Init", r.config).Return(errors.Errorf("init error")).Once()
	err = r.reinitialize(objectStore)
	assert.EqualError(t, err, "init error")

	objectStore.On("Init", r.config).Return(nil)
	err = r.reinitialize(objectStore)
	assert.NoError(t, err)
}

func TestRestartableObjectStoreGetDelegate(t *testing.T) {
	p := new(mockRestartableProcess)
	p.Test(t)
	defer p.AssertExpectations(t)

	// Reset error
	p.On("resetIfNeeded").Return(errors.Errorf("reset error")).Once()
	name := "aws"
	key := kindAndName{kind: framework.PluginKindObjectStore, name: name}
	r := &restartableObjectStore{
		key:                 key,
		sharedPluginProcess: p,
	}
	a, err := r.getDelegate()
	assert.Nil(t, a)
	assert.EqualError(t, err, "reset error")

	// Happy path
	p.On("resetIfNeeded").Return(nil)
	objectStore := new(providermocks.ObjectStore)
	objectStore.Test(t)
	defer objectStore.AssertExpectations(t)
	p.On("getByKindAndName", key).Return(objectStore, nil)

	a, err = r.getDelegate()
	assert.NoError(t, err)
	assert.Equal(t, objectStore, a)
}

func TestRestartableObjectStoreInit(t *testing.T) {
	p := new(mockRestartableProcess)
	p.Test(t)
	defer p.AssertExpectations(t)

	// getObjectStore error
	name := "aws"
	key := kindAndName{kind: framework.PluginKindObjectStore, name: name}
	r := &restartableObjectStore{
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
	objectStore := new(providermocks.ObjectStore)
	objectStore.Test(t)
	defer objectStore.AssertExpectations(t)
	p.On("getByKindAndName", key).Return(objectStore, nil)
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
	runRestartableDelegateTests(
		t,
		framework.PluginKindObjectStore,
		func(key kindAndName, p RestartableProcess) interface{} {
			return &restartableObjectStore{
				key:                 key,
				sharedPluginProcess: p,
			}
		},
		func() mockable {
			return new(providermocks.ObjectStore)
		},
		restartableDelegateTest{
			function:                "PutObject",
			inputs:                  []interface{}{"bucket", "key", strings.NewReader("body")},
			expectedErrorOutputs:    []interface{}{errors.Errorf("reset error")},
			expectedDelegateOutputs: []interface{}{errors.Errorf("delegate error")},
		},
		restartableDelegateTest{
			function:                "GetObject",
			inputs:                  []interface{}{"bucket", "key"},
			expectedErrorOutputs:    []interface{}{nil, errors.Errorf("reset error")},
			expectedDelegateOutputs: []interface{}{ioutil.NopCloser(strings.NewReader("object")), errors.Errorf("delegate error")},
		},
		restartableDelegateTest{
			function:                "ListCommonPrefixes",
			inputs:                  []interface{}{"bucket", "prefix", "delimiter"},
			expectedErrorOutputs:    []interface{}{([]string)(nil), errors.Errorf("reset error")},
			expectedDelegateOutputs: []interface{}{[]string{"a", "b"}, errors.Errorf("delegate error")},
		},
		restartableDelegateTest{
			function:                "ListObjects",
			inputs:                  []interface{}{"bucket", "prefix"},
			expectedErrorOutputs:    []interface{}{([]string)(nil), errors.Errorf("reset error")},
			expectedDelegateOutputs: []interface{}{[]string{"a", "b"}, errors.Errorf("delegate error")},
		},
		restartableDelegateTest{
			function:                "DeleteObject",
			inputs:                  []interface{}{"bucket", "key"},
			expectedErrorOutputs:    []interface{}{errors.Errorf("reset error")},
			expectedDelegateOutputs: []interface{}{errors.Errorf("delegate error")},
		},
		restartableDelegateTest{
			function:                "CreateSignedURL",
			inputs:                  []interface{}{"bucket", "key", 30 * time.Minute},
			expectedErrorOutputs:    []interface{}{"", errors.Errorf("reset error")},
			expectedDelegateOutputs: []interface{}{"signedURL", errors.Errorf("delegate error")},
		},
	)
}
