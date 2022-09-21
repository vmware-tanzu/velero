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

package v1

import (
	"testing"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/vmware-tanzu/velero/internal/restartabletest"
	v1 "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	"github.com/vmware-tanzu/velero/pkg/plugin/clientmgmt/process"
	"github.com/vmware-tanzu/velero/pkg/plugin/framework/common"
	"github.com/vmware-tanzu/velero/pkg/plugin/velero"
	mocks "github.com/vmware-tanzu/velero/pkg/plugin/velero/mocks/backupitemaction/v1"
)

func TestRestartableGetBackupItemAction(t *testing.T) {
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
			expectedError: "int is not a BackupItemAction!",
		},
		{
			name:   "happy path",
			plugin: new(mocks.BackupItemAction),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			p := new(restartabletest.MockRestartableProcess)
			defer p.AssertExpectations(t)

			name := "pod"
			key := process.KindAndName{Kind: common.PluginKindBackupItemAction, Name: name}
			p.On("GetByKindAndName", key).Return(tc.plugin, tc.getError)

			r := NewRestartableBackupItemAction(name, p)
			a, err := r.getBackupItemAction()
			if tc.expectedError != "" {
				assert.EqualError(t, err, tc.expectedError)
				return
			}
			require.NoError(t, err)

			assert.Equal(t, tc.plugin, a)
		})
	}
}

func TestRestartableBackupItemActionGetDelegate(t *testing.T) {
	p := new(restartabletest.MockRestartableProcess)
	defer p.AssertExpectations(t)

	// Reset error
	p.On("ResetIfNeeded").Return(errors.Errorf("reset error")).Once()
	name := "pod"
	r := NewRestartableBackupItemAction(name, p)
	a, err := r.getDelegate()
	assert.Nil(t, a)
	assert.EqualError(t, err, "reset error")

	// Happy path
	p.On("ResetIfNeeded").Return(nil)
	expected := new(mocks.BackupItemAction)
	key := process.KindAndName{Kind: common.PluginKindBackupItemAction, Name: name}
	p.On("GetByKindAndName", key).Return(expected, nil)

	a, err = r.getDelegate()
	assert.NoError(t, err)
	assert.Equal(t, expected, a)
}

func TestRestartableBackupItemActionDelegatedFunctions(t *testing.T) {
	b := new(v1.Backup)

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

	additionalItems := []velero.ResourceIdentifier{
		{
			GroupResource: schema.GroupResource{Group: "velero.io", Resource: "backups"},
		},
	}

	restartabletest.RunRestartableDelegateTests(
		t,
		common.PluginKindBackupItemAction,
		func(key process.KindAndName, p process.RestartableProcess) interface{} {
			return &RestartableBackupItemAction{
				Key:                 key,
				SharedPluginProcess: p,
			}
		},
		func() restartabletest.Mockable {
			return new(mocks.BackupItemAction)
		},
		restartabletest.RestartableDelegateTest{
			Function:                "AppliesTo",
			Inputs:                  []interface{}{},
			ExpectedErrorOutputs:    []interface{}{velero.ResourceSelector{}, errors.Errorf("reset error")},
			ExpectedDelegateOutputs: []interface{}{velero.ResourceSelector{IncludedNamespaces: []string{"a"}}, errors.Errorf("delegate error")},
		},
		restartabletest.RestartableDelegateTest{
			Function:                "Execute",
			Inputs:                  []interface{}{pv, b},
			ExpectedErrorOutputs:    []interface{}{nil, ([]velero.ResourceIdentifier)(nil), errors.Errorf("reset error")},
			ExpectedDelegateOutputs: []interface{}{pvToReturn, additionalItems, errors.Errorf("delegate error")},
		},
	)
}
