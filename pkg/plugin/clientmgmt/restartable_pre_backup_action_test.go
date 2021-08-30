/*
Copyright The Velero Contributors.

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
	"testing"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	v1 "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	"github.com/vmware-tanzu/velero/pkg/plugin/framework"
	"github.com/vmware-tanzu/velero/pkg/plugin/velero/mocks"
)

func TestRestartableGetPreBackupAction(t *testing.T) {
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
			expectedError: "int is not a PreBackupAction!",
		},
		{
			name:   "happy path",
			plugin: new(mocks.PreBackupAction),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			p := new(mockRestartableProcess)
			defer p.AssertExpectations(t)

			name := "pod"
			key := kindAndName{kind: framework.PluginKindPreBackupAction, name: name}
			p.On("getByKindAndName", key).Return(tc.plugin, tc.getError)

			r := newRestartablePreBackupAction(name, p)
			a, err := r.getPreBackupAction()
			if tc.expectedError != "" {
				assert.EqualError(t, err, tc.expectedError)
				return
			}

			require.NoError(t, err)

			assert.Equal(t, tc.plugin, a)
		})
	}
}

func TestRestartablePreBackupActionGetDelegate(t *testing.T) {
	p := new(mockRestartableProcess)
	defer p.AssertExpectations(t)

	// Reset error
	p.On("resetIfNeeded").Return(errors.Errorf("reset error")).Once()
	name := "pod"
	r := newRestartablePreBackupAction(name, p)
	a, err := r.getDelegate()
	assert.Nil(t, a)
	assert.EqualError(t, err, "reset error")

	// Happy path
	p.On("resetIfNeeded").Return(nil)
	expected := new(mocks.PreBackupAction)
	key := kindAndName{kind: framework.PluginKindPreBackupAction, name: name}
	p.On("getByKindAndName", key).Return(expected, nil)

	a, err = r.getDelegate()
	assert.NoError(t, err)
	assert.Equal(t, expected, a)
}

func TestRestartablePreBackupActionDelegatedFunctions(t *testing.T) {
	b := new(v1.Backup)

	runRestartableDelegateTests(
		t,
		framework.PluginKindPreBackupAction,
		func(key kindAndName, p RestartableProcess) interface{} {
			return &restartablePreBackupAction{
				key:                 key,
				sharedPluginProcess: p,
			}
		},
		func() mockable {
			return new(mocks.PreBackupAction)
		},
		restartableDelegateTest{
			function:                "Execute",
			inputs:                  []interface{}{b},
			expectedErrorOutputs:    []interface{}{errors.Errorf("reset error")},
			expectedDelegateOutputs: []interface{}{errors.Errorf("delegate error")},
		},
	)
}
