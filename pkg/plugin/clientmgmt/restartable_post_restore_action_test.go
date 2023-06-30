/*
Copyright the Velero contributors.

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

	"github.com/vmware-tanzu/velero/internal/restartabletest"
	api "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	"github.com/vmware-tanzu/velero/pkg/plugin/clientmgmt/process"
	"github.com/vmware-tanzu/velero/pkg/plugin/framework/common"
	"github.com/vmware-tanzu/velero/pkg/plugin/velero/mocks"
)

func TestRestartableGetPostRestoreAction(t *testing.T) {
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
			expectedError: "int is not a PostRestoreAction",
		},
		{
			name:   "happy path",
			plugin: new(mocks.PostRestoreAction),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			p := new(restartabletest.MockRestartableProcess)
			defer p.AssertExpectations(t)

			name := "pod"
			key := process.KindAndName{Kind: common.PluginKindPostRestoreAction, Name: name}
			p.On("GetByKindAndName", key).Return(tc.plugin, tc.getError)

			r := newRestartablePostRestoreAction(name, p)
			a, err := r.getPostRestoreAction()
			if tc.expectedError != "" {
				assert.EqualError(t, err, tc.expectedError)
				return
			}
			require.NoError(t, err)

			assert.Equal(t, tc.plugin, a)
		})
	}
}

func TestRestartablePostRestoreActionGetDelegate(t *testing.T) {
	p := new(restartabletest.MockRestartableProcess)
	defer p.AssertExpectations(t)

	// Reset error
	p.On("ResetIfNeeded").Return(errors.Errorf("reset error")).Once()
	name := "pod"
	r := newRestartablePostRestoreAction(name, p)
	a, err := r.getDelegate()
	assert.Nil(t, a)
	assert.EqualError(t, err, "reset error")

	// Happy path
	p.On("ResetIfNeeded").Return(nil)
	expected := new(mocks.PostRestoreAction)
	key := process.KindAndName{Kind: common.PluginKindPostRestoreAction, Name: name}
	p.On("GetByKindAndName", key).Return(expected, nil)

	a, err = r.getDelegate()
	assert.NoError(t, err)
	assert.Equal(t, expected, a)
}

func TestRestartablePostRestoreActionDelegatedFunctions(t *testing.T) {
	restore := &api.Restore{}

	restartabletest.RunRestartableDelegateTests(
		t,
		common.PluginKindPostRestoreAction,
		func(key process.KindAndName, p process.RestartableProcess) interface{} {
			return &restartablePostRestoreAction{
				key:                 key,
				sharedPluginProcess: p,
			}
		},
		func() restartabletest.Mockable {
			return new(mocks.PostRestoreAction)
		},
		restartabletest.RestartableDelegateTest{
			Function:                "Execute",
			Inputs:                  []interface{}{restore},
			ExpectedErrorOutputs:    []interface{}{errors.Errorf("reset error")},
			ExpectedDelegateOutputs: []interface{}{errors.Errorf("delegate error")},
		},
	)
}
