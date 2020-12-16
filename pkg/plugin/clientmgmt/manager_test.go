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
	"fmt"
	"testing"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/vmware-tanzu/velero/pkg/plugin/framework"
	"github.com/vmware-tanzu/velero/pkg/test"
)

type mockRegistry struct {
	mock.Mock
}

func (r *mockRegistry) DiscoverPlugins() error {
	args := r.Called()
	return args.Error(0)
}

func (r *mockRegistry) List(kind framework.PluginKind) []framework.PluginIdentifier {
	args := r.Called(kind)
	return args.Get(0).([]framework.PluginIdentifier)
}

func (r *mockRegistry) Get(kind framework.PluginKind, name string) (framework.PluginIdentifier, error) {
	args := r.Called(kind, name)
	var id framework.PluginIdentifier
	if args.Get(0) != nil {
		id = args.Get(0).(framework.PluginIdentifier)
	}
	return id, args.Error(1)
}

func TestNewManager(t *testing.T) {
	logger := test.NewLogger()
	logLevel := logrus.InfoLevel

	registry := &mockRegistry{}
	defer registry.AssertExpectations(t)

	m := NewManager(logger, logLevel, registry).(*manager)
	assert.Equal(t, logger, m.logger)
	assert.Equal(t, logLevel, m.logLevel)
	assert.Equal(t, registry, m.registry)
	assert.NotNil(t, m.restartableProcesses)
	assert.Empty(t, m.restartableProcesses)
}

type mockRestartableProcessFactory struct {
	mock.Mock
}

func (f *mockRestartableProcessFactory) newRestartableProcess(command string, logger logrus.FieldLogger, logLevel logrus.Level) (RestartableProcess, error) {
	args := f.Called(command, logger, logLevel)
	var rp RestartableProcess
	if args.Get(0) != nil {
		rp = args.Get(0).(RestartableProcess)
	}
	return rp, args.Error(1)
}

type mockRestartableProcess struct {
	mock.Mock
}

func (rp *mockRestartableProcess) addReinitializer(key kindAndName, r reinitializer) {
	rp.Called(key, r)
}

func (rp *mockRestartableProcess) reset() error {
	args := rp.Called()
	return args.Error(0)
}

func (rp *mockRestartableProcess) resetIfNeeded() error {
	args := rp.Called()
	return args.Error(0)
}

func (rp *mockRestartableProcess) getByKindAndName(key kindAndName) (interface{}, error) {
	args := rp.Called(key)
	return args.Get(0), args.Error(1)
}

func (rp *mockRestartableProcess) stop() {
	rp.Called()
}

func TestGetRestartableProcess(t *testing.T) {
	logger := test.NewLogger()
	logLevel := logrus.InfoLevel

	registry := &mockRegistry{}
	defer registry.AssertExpectations(t)

	m := NewManager(logger, logLevel, registry).(*manager)
	factory := &mockRestartableProcessFactory{}
	defer factory.AssertExpectations(t)
	m.restartableProcessFactory = factory

	// Test 1: registry error
	pluginKind := framework.PluginKindBackupItemAction
	pluginName := "pod"
	registry.On("Get", pluginKind, pluginName).Return(nil, errors.Errorf("registry")).Once()
	rp, err := m.getRestartableProcess(pluginKind, pluginName)
	assert.Nil(t, rp)
	assert.EqualError(t, err, "registry")

	// Test 2: registry ok, factory error
	podID := framework.PluginIdentifier{
		Command: "/command",
		Kind:    pluginKind,
		Name:    pluginName,
	}
	registry.On("Get", pluginKind, pluginName).Return(podID, nil)
	factory.On("newRestartableProcess", podID.Command, logger, logLevel).Return(nil, errors.Errorf("factory")).Once()
	rp, err = m.getRestartableProcess(pluginKind, pluginName)
	assert.Nil(t, rp)
	assert.EqualError(t, err, "factory")

	// Test 3: registry ok, factory ok
	restartableProcess := &mockRestartableProcess{}
	defer restartableProcess.AssertExpectations(t)
	factory.On("newRestartableProcess", podID.Command, logger, logLevel).Return(restartableProcess, nil).Once()
	rp, err = m.getRestartableProcess(pluginKind, pluginName)
	require.NoError(t, err)
	assert.Equal(t, restartableProcess, rp)

	// Test 4: retrieve from cache
	rp, err = m.getRestartableProcess(pluginKind, pluginName)
	require.NoError(t, err)
	assert.Equal(t, restartableProcess, rp)
}

func TestCleanupClients(t *testing.T) {
	logger := test.NewLogger()
	logLevel := logrus.InfoLevel

	registry := &mockRegistry{}
	defer registry.AssertExpectations(t)

	m := NewManager(logger, logLevel, registry).(*manager)

	for i := 0; i < 5; i++ {
		rp := &mockRestartableProcess{}
		defer rp.AssertExpectations(t)
		rp.On("stop")
		m.restartableProcesses[fmt.Sprintf("rp%d", i)] = rp
	}

	m.CleanupClients()
}

func TestGetObjectStore(t *testing.T) {
	getPluginTest(t,
		framework.PluginKindObjectStore,
		"velero.io/aws",
		func(m Manager, name string) (interface{}, error) {
			return m.GetObjectStore(name)
		},
		func(name string, sharedPluginProcess RestartableProcess) interface{} {
			return &restartableObjectStore{
				key:                 kindAndName{kind: framework.PluginKindObjectStore, name: name},
				sharedPluginProcess: sharedPluginProcess,
			}
		},
		true,
	)
}

func TestGetVolumeSnapshotter(t *testing.T) {
	getPluginTest(t,
		framework.PluginKindVolumeSnapshotter,
		"velero.io/aws",
		func(m Manager, name string) (interface{}, error) {
			return m.GetVolumeSnapshotter(name)
		},
		func(name string, sharedPluginProcess RestartableProcess) interface{} {
			return &restartableVolumeSnapshotter{
				key:                 kindAndName{kind: framework.PluginKindVolumeSnapshotter, name: name},
				sharedPluginProcess: sharedPluginProcess,
			}
		},
		true,
	)
}

func TestGetBackupItemAction(t *testing.T) {
	getPluginTest(t,
		framework.PluginKindBackupItemAction,
		"velero.io/pod",
		func(m Manager, name string) (interface{}, error) {
			return m.GetBackupItemAction(name)
		},
		func(name string, sharedPluginProcess RestartableProcess) interface{} {
			return &restartableBackupItemAction{
				key:                 kindAndName{kind: framework.PluginKindBackupItemAction, name: name},
				sharedPluginProcess: sharedPluginProcess,
			}
		},
		false,
	)
}

func TestGetRestoreItemAction(t *testing.T) {
	getPluginTest(t,
		framework.PluginKindRestoreItemAction,
		"velero.io/pod",
		func(m Manager, name string) (interface{}, error) {
			return m.GetRestoreItemAction(name)
		},
		func(name string, sharedPluginProcess RestartableProcess) interface{} {
			return &restartableRestoreItemAction{
				key:                 kindAndName{kind: framework.PluginKindRestoreItemAction, name: name},
				sharedPluginProcess: sharedPluginProcess,
			}
		},
		false,
	)
}

func getPluginTest(
	t *testing.T,
	kind framework.PluginKind,
	name string,
	getPluginFunc func(m Manager, name string) (interface{}, error),
	expectedResultFunc func(name string, sharedPluginProcess RestartableProcess) interface{},
	reinitializable bool,
) {
	logger := test.NewLogger()
	logLevel := logrus.InfoLevel

	registry := &mockRegistry{}
	defer registry.AssertExpectations(t)

	m := NewManager(logger, logLevel, registry).(*manager)
	factory := &mockRestartableProcessFactory{}
	defer factory.AssertExpectations(t)
	m.restartableProcessFactory = factory

	pluginKind := kind
	pluginName := name
	pluginID := framework.PluginIdentifier{
		Command: "/command",
		Kind:    pluginKind,
		Name:    pluginName,
	}
	registry.On("Get", pluginKind, pluginName).Return(pluginID, nil)

	restartableProcess := &mockRestartableProcess{}
	defer restartableProcess.AssertExpectations(t)

	// Test 1: error getting restartable process
	factory.On("newRestartableProcess", pluginID.Command, logger, logLevel).Return(nil, errors.Errorf("newRestartableProcess")).Once()
	actual, err := getPluginFunc(m, pluginName)
	assert.Nil(t, actual)
	assert.EqualError(t, err, "newRestartableProcess")

	// Test 2: happy path
	factory.On("newRestartableProcess", pluginID.Command, logger, logLevel).Return(restartableProcess, nil).Once()

	expected := expectedResultFunc(name, restartableProcess)
	if reinitializable {
		key := kindAndName{kind: pluginID.Kind, name: pluginID.Name}
		restartableProcess.On("addReinitializer", key, expected)
	}

	actual, err = getPluginFunc(m, pluginName)
	require.NoError(t, err)
	assert.Equal(t, expected, actual)
}

func TestGetBackupItemActions(t *testing.T) {
	tests := []struct {
		name                       string
		names                      []string
		newRestartableProcessError error
		expectedError              string
	}{
		{
			name:  "No items",
			names: []string{},
		},
		{
			name:                       "Error getting restartable process",
			names:                      []string{"velero.io/a", "velero.io/b", "velero.io/c"},
			newRestartableProcessError: errors.Errorf("newRestartableProcess"),
			expectedError:              "newRestartableProcess",
		},
		{
			name:  "Happy path",
			names: []string{"velero.io/a", "velero.io/b", "velero.io/c"},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			logger := test.NewLogger()
			logLevel := logrus.InfoLevel

			registry := &mockRegistry{}
			defer registry.AssertExpectations(t)

			m := NewManager(logger, logLevel, registry).(*manager)
			factory := &mockRestartableProcessFactory{}
			defer factory.AssertExpectations(t)
			m.restartableProcessFactory = factory

			pluginKind := framework.PluginKindBackupItemAction
			var pluginIDs []framework.PluginIdentifier
			for i := range tc.names {
				pluginID := framework.PluginIdentifier{
					Command: "/command",
					Kind:    pluginKind,
					Name:    tc.names[i],
				}
				pluginIDs = append(pluginIDs, pluginID)
			}
			registry.On("List", pluginKind).Return(pluginIDs)

			var expectedActions []interface{}
			for i := range pluginIDs {
				pluginID := pluginIDs[i]
				pluginName := pluginID.Name

				registry.On("Get", pluginKind, pluginName).Return(pluginID, nil)

				restartableProcess := &mockRestartableProcess{}
				defer restartableProcess.AssertExpectations(t)

				expected := &restartableBackupItemAction{
					key:                 kindAndName{kind: pluginKind, name: pluginName},
					sharedPluginProcess: restartableProcess,
				}

				if tc.newRestartableProcessError != nil {
					// Test 1: error getting restartable process
					factory.On("newRestartableProcess", pluginID.Command, logger, logLevel).Return(nil, errors.Errorf("newRestartableProcess")).Once()
					break
				}

				// Test 2: happy path
				if i == 0 {
					factory.On("newRestartableProcess", pluginID.Command, logger, logLevel).Return(restartableProcess, nil).Once()
				}

				expectedActions = append(expectedActions, expected)
			}

			backupItemActions, err := m.GetBackupItemActions()
			if tc.newRestartableProcessError != nil {
				assert.Nil(t, backupItemActions)
				assert.EqualError(t, err, "newRestartableProcess")
			} else {
				require.NoError(t, err)
				var actual []interface{}
				for i := range backupItemActions {
					actual = append(actual, backupItemActions[i])
				}
				assert.Equal(t, expectedActions, actual)
			}
		})
	}
}

func TestGetRestoreItemActions(t *testing.T) {
	tests := []struct {
		name                       string
		names                      []string
		newRestartableProcessError error
		expectedError              string
	}{
		{
			name:  "No items",
			names: []string{},
		},
		{
			name:                       "Error getting restartable process",
			names:                      []string{"velero.io/a", "velero.io/b", "velero.io/c"},
			newRestartableProcessError: errors.Errorf("newRestartableProcess"),
			expectedError:              "newRestartableProcess",
		},
		{
			name:  "Happy path",
			names: []string{"velero.io/a", "velero.io/b", "velero.io/c"},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			logger := test.NewLogger()
			logLevel := logrus.InfoLevel

			registry := &mockRegistry{}
			defer registry.AssertExpectations(t)

			m := NewManager(logger, logLevel, registry).(*manager)
			factory := &mockRestartableProcessFactory{}
			defer factory.AssertExpectations(t)
			m.restartableProcessFactory = factory

			pluginKind := framework.PluginKindRestoreItemAction
			var pluginIDs []framework.PluginIdentifier
			for i := range tc.names {
				pluginID := framework.PluginIdentifier{
					Command: "/command",
					Kind:    pluginKind,
					Name:    tc.names[i],
				}
				pluginIDs = append(pluginIDs, pluginID)
			}
			registry.On("List", pluginKind).Return(pluginIDs)

			var expectedActions []interface{}
			for i := range pluginIDs {
				pluginID := pluginIDs[i]
				pluginName := pluginID.Name

				registry.On("Get", pluginKind, pluginName).Return(pluginID, nil)

				restartableProcess := &mockRestartableProcess{}
				defer restartableProcess.AssertExpectations(t)

				expected := &restartableRestoreItemAction{
					key:                 kindAndName{kind: pluginKind, name: pluginName},
					sharedPluginProcess: restartableProcess,
				}

				if tc.newRestartableProcessError != nil {
					// Test 1: error getting restartable process
					factory.On("newRestartableProcess", pluginID.Command, logger, logLevel).Return(nil, errors.Errorf("newRestartableProcess")).Once()
					break
				}

				// Test 2: happy path
				if i == 0 {
					factory.On("newRestartableProcess", pluginID.Command, logger, logLevel).Return(restartableProcess, nil).Once()
				}

				expectedActions = append(expectedActions, expected)
			}

			restoreItemActions, err := m.GetRestoreItemActions()
			if tc.newRestartableProcessError != nil {
				assert.Nil(t, restoreItemActions)
				assert.EqualError(t, err, "newRestartableProcess")
			} else {
				require.NoError(t, err)
				var actual []interface{}
				for i := range restoreItemActions {
					actual = append(actual, restoreItemActions[i])
				}
				assert.Equal(t, expectedActions, actual)
			}
		})
	}
}

func TestGetDeleteItemAction(t *testing.T) {
	getPluginTest(t,
		framework.PluginKindDeleteItemAction,
		"velero.io/deleter",
		func(m Manager, name string) (interface{}, error) {
			return m.GetDeleteItemAction(name)
		},
		func(name string, sharedPluginProcess RestartableProcess) interface{} {
			return &restartableDeleteItemAction{
				key:                 kindAndName{kind: framework.PluginKindDeleteItemAction, name: name},
				sharedPluginProcess: sharedPluginProcess,
			}
		},
		false,
	)
}

func TestGetDeleteItemActions(t *testing.T) {
	tests := []struct {
		name                       string
		names                      []string
		newRestartableProcessError error
		expectedError              string
	}{
		{
			name:  "No items",
			names: []string{},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			logger := test.NewLogger()
			logLevel := logrus.InfoLevel

			registry := &mockRegistry{}
			defer registry.AssertExpectations(t)

			m := NewManager(logger, logLevel, registry).(*manager)
			factory := &mockRestartableProcessFactory{}
			defer factory.AssertExpectations(t)
			m.restartableProcessFactory = factory

			pluginKind := framework.PluginKindDeleteItemAction
			var pluginIDs []framework.PluginIdentifier
			for i := range tc.names {
				pluginID := framework.PluginIdentifier{
					Command: "/command",
					Kind:    pluginKind,
					Name:    tc.names[i],
				}
				pluginIDs = append(pluginIDs, pluginID)
			}
			registry.On("List", pluginKind).Return(pluginIDs)

			var expectedActions []interface{}
			for i := range pluginIDs {
				pluginID := pluginIDs[i]
				pluginName := pluginID.Name

				registry.On("Get", pluginKind, pluginName).Return(pluginID, nil)

				restartableProcess := &mockRestartableProcess{}
				defer restartableProcess.AssertExpectations(t)

				expected := &restartableRestoreItemAction{
					key:                 kindAndName{kind: pluginKind, name: pluginName},
					sharedPluginProcess: restartableProcess,
				}

				if tc.newRestartableProcessError != nil {
					// Test 1: error getting restartable process
					factory.On("newRestartableProcess", pluginID.Command, logger, logLevel).Return(nil, errors.Errorf("newRestartableProcess")).Once()
					break
				}

				// Test 2: happy path
				if i == 0 {
					factory.On("newRestartableProcess", pluginID.Command, logger, logLevel).Return(restartableProcess, nil).Once()
				}

				expectedActions = append(expectedActions, expected)
			}

			deleteItemActions, err := m.GetDeleteItemActions()
			if tc.newRestartableProcessError != nil {
				assert.Nil(t, deleteItemActions)
				assert.EqualError(t, err, "newRestartableProcess")
			} else {
				require.NoError(t, err)
				var actual []interface{}
				for i := range deleteItemActions {
					actual = append(actual, deleteItemActions[i])
				}
				assert.Equal(t, expectedActions, actual)
			}
		})
	}
}

func TestSanitizeName(t *testing.T) {
	tests := []struct {
		name, pluginName, expectedName string
	}{
		{
			name:         "Legacy, non-namespaced plugin",
			pluginName:   "aws",
			expectedName: "velero.io/aws",
		},
		{
			name:         "A Velero plugin",
			pluginName:   "velero.io/aws",
			expectedName: "velero.io/aws",
		},
		{
			name:         "A non-Velero plugin with a Velero namespace",
			pluginName:   "velero.io/plugin-for-velero",
			expectedName: "velero.io/plugin-for-velero",
		},
		{
			name:         "A non-Velero plugin with a non-Velero namespace",
			pluginName:   "digitalocean.com/velero",
			expectedName: "digitalocean.com/velero",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			sanitizedName := sanitizeName(tc.pluginName)
			assert.Equal(t, sanitizedName, tc.expectedName)
		})
	}
}
