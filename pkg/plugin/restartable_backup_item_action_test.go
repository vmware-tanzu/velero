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

package plugin

import (
	"testing"

	"github.com/heptio/ark/pkg/apis/ark/v1"
	"github.com/heptio/ark/pkg/backup"
	"github.com/heptio/ark/pkg/backup/mocks"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
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
			expectedError: "int is not a backup.ItemAction!",
		},
		{
			name:   "happy path",
			plugin: new(mocks.ItemAction),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			p := new(mockRestartableProcess)
			defer p.AssertExpectations(t)

			name := "pod"
			key := kindAndName{kind: PluginKindBackupItemAction, name: name}
			p.On("getByKindAndName", key).Return(tc.plugin, tc.getError)

			r := newRestartableBackupItemAction(name, p)
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
	p := new(mockRestartableProcess)
	defer p.AssertExpectations(t)

	// Reset error
	p.On("resetIfNeeded").Return(errors.Errorf("reset error")).Once()
	name := "pod"
	r := newRestartableBackupItemAction(name, p)
	a, err := r.getDelegate()
	assert.Nil(t, a)
	assert.EqualError(t, err, "reset error")

	// Happy path
	p.On("resetIfNeeded").Return(nil)
	expected := new(mocks.ItemAction)
	key := kindAndName{kind: PluginKindBackupItemAction, name: name}
	p.On("getByKindAndName", key).Return(expected, nil)

	a, err = r.getDelegate()
	assert.NoError(t, err)
	assert.Equal(t, expected, a)
}

func TestRestartableBackupItemActionAppliesTo(t *testing.T) {
	p := new(mockRestartableProcess)
	defer p.AssertExpectations(t)

	// getDelegate error
	p.On("resetIfNeeded").Return(errors.Errorf("reset error")).Once()
	name := "pod"
	r := newRestartableBackupItemAction(name, p)
	a, err := r.AppliesTo()
	assert.Equal(t, backup.ResourceSelector{}, a)
	assert.EqualError(t, err, "reset error")

	// Delegate returns error
	p.On("resetIfNeeded").Return(nil)
	delegate := new(mocks.ItemAction)
	key := kindAndName{kind: PluginKindBackupItemAction, name: name}
	p.On("getByKindAndName", key).Return(delegate, nil)
	delegate.On("AppliesTo").Return(backup.ResourceSelector{}, errors.Errorf("applies to error")).Once()

	a, err = r.AppliesTo()
	assert.EqualError(t, err, "applies to error")
	assert.Equal(t, backup.ResourceSelector{}, a)

	// Happy path
	resourceSelector := backup.ResourceSelector{
		IncludedNamespaces: []string{"ns1"},
	}
	delegate.On("AppliesTo").Return(resourceSelector, nil)

	a, err = r.AppliesTo()
	assert.NoError(t, err)
	assert.Equal(t, resourceSelector, a)
}

func TestRestartableBackupItemActionExecute(t *testing.T) {
	p := new(mockRestartableProcess)
	defer p.AssertExpectations(t)

	item := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"foo": "bar",
		},
	}
	b := new(v1.Backup)
	updatedItem := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"color": "blue",
		},
	}

	// getDelegate error
	p.On("resetIfNeeded").Return(errors.Errorf("reset error")).Once()
	name := "pod"
	r := newRestartableBackupItemAction(name, p)

	actualUpdatedItem, actualAdditionalItems, err := r.Execute(item, b)
	assert.Nil(t, actualUpdatedItem)
	assert.Nil(t, actualAdditionalItems)
	assert.EqualError(t, err, "reset error")

	// Delegate returns error
	p.On("resetIfNeeded").Return(nil)
	delegate := new(mocks.ItemAction)
	key := kindAndName{kind: PluginKindBackupItemAction, name: name}
	p.On("getByKindAndName", key).Return(delegate, nil)
	delegate.On("Execute", item, b).Return(nil, nil, errors.Errorf("execute error")).Once()

	actualUpdatedItem, actualAdditionalItems, err = r.Execute(item, b)
	assert.Nil(t, actualUpdatedItem)
	assert.Nil(t, actualAdditionalItems)
	assert.EqualError(t, err, "execute error")

	// Happy path
	additionalItems := []backup.ResourceIdentifier{
		{
			GroupResource: schema.GroupResource{Group: "ark.heptio.com", Resource: "backups"},
		},
	}
	delegate.On("Execute", item, b).Return(updatedItem, additionalItems, nil)

	actualUpdatedItem, actualAdditionalItems, err = r.Execute(item, b)
	assert.Equal(t, updatedItem, actualUpdatedItem)
	assert.Equal(t, additionalItems, actualAdditionalItems)
	assert.NoError(t, err)
}
