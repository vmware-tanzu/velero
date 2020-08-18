/*
Copyright 2020 the Velero contributors.

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
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	api "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	"github.com/vmware-tanzu/velero/pkg/plugin/framework"
	"github.com/vmware-tanzu/velero/pkg/plugin/velero"
	"github.com/vmware-tanzu/velero/pkg/plugin/velero/mocks"
)

func TestRestartableGetDeleteItemAction(t *testing.T) {
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
			expectedError: "int is not a DeleteItemAction!",
		},
		{
			name:   "happy path",
			plugin: new(mocks.DeleteItemAction),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			p := new(mockRestartableProcess)
			defer p.AssertExpectations(t)

			name := "pod"
			key := kindAndName{kind: framework.PluginKindDeleteItemAction, name: name}
			p.On("getByKindAndName", key).Return(tc.plugin, tc.getError)

			r := newRestartableDeleteItemAction(name, p)
			a, err := r.getDeleteItemAction()
			if tc.expectedError != "" {
				assert.EqualError(t, err, tc.expectedError)
				return
			}
			require.NoError(t, err)

			assert.Equal(t, tc.plugin, a)
		})
	}
}

func TestRestartableDeleteItemActionGetDelegate(t *testing.T) {
	p := new(mockRestartableProcess)
	defer p.AssertExpectations(t)

	// Reset error
	p.On("resetIfNeeded").Return(errors.Errorf("reset error")).Once()
	name := "pod"
	r := newRestartableDeleteItemAction(name, p)
	a, err := r.getDelegate()
	assert.Nil(t, a)
	assert.EqualError(t, err, "reset error")

	// Happy path
	// Currently broken since this mocks out the restore item action interface
	p.On("resetIfNeeded").Return(nil)
	expected := new(mocks.DeleteItemAction)
	key := kindAndName{kind: framework.PluginKindDeleteItemAction, name: name}
	p.On("getByKindAndName", key).Return(expected, nil)

	a, err = r.getDelegate()
	assert.NoError(t, err)
	assert.Equal(t, expected, a)
}

func TestRestartableDeleteItemActionDelegatedFunctions(t *testing.T) {
	pv := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"color": "blue",
		},
	}

	backup := &api.Backup{}

	input := &velero.DeleteItemActionExecuteInput{
		Item:   pv,
		Backup: backup,
	}

	runRestartableDelegateTests(
		t,
		framework.PluginKindDeleteItemAction,
		func(key kindAndName, p RestartableProcess) interface{} {
			return &restartableDeleteItemAction{
				key:                 key,
				sharedPluginProcess: p,
			}
		},
		func() mockable {
			// Currently broken because this mocks the restore item action interface
			return new(mocks.DeleteItemAction)
		},
		restartableDelegateTest{
			function:                "AppliesTo",
			inputs:                  []interface{}{},
			expectedErrorOutputs:    []interface{}{velero.ResourceSelector{}, errors.Errorf("reset error")},
			expectedDelegateOutputs: []interface{}{velero.ResourceSelector{IncludedNamespaces: []string{"a"}}, errors.Errorf("delegate error")},
		},
		restartableDelegateTest{
			function:                "Execute",
			inputs:                  []interface{}{input},
			expectedErrorOutputs:    []interface{}{errors.Errorf("reset error")},
			expectedDelegateOutputs: []interface{}{errors.Errorf("delegate error")},
		},
	)
}
