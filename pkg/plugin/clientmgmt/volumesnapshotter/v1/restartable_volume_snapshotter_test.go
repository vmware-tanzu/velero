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

package v1

import (
	"testing"

	"github.com/Azure/go-autorest/autorest/to"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/vmware-tanzu/velero/internal/restartabletest"
	"github.com/vmware-tanzu/velero/pkg/plugin/clientmgmt/process"
	"github.com/vmware-tanzu/velero/pkg/plugin/framework/common"
	providermocks "github.com/vmware-tanzu/velero/pkg/plugin/velero/mocks/volumesnapshotter/v1"
)

func TestRestartableGetVolumeSnapshotter(t *testing.T) {
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
			expectedError: "int is not a VolumeSnapshotter!",
		},
		{
			name:   "happy path",
			plugin: new(providermocks.VolumeSnapshotter),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			p := new(restartabletest.MockRestartableProcess)
			p.Test(t)
			defer p.AssertExpectations(t)

			name := "aws"
			key := process.KindAndName{Kind: common.PluginKindVolumeSnapshotter, Name: name}
			p.On("GetByKindAndName", key).Return(tc.plugin, tc.getError)

			r := &RestartableVolumeSnapshotter{
				Key:                 key,
				SharedPluginProcess: p,
			}
			a, err := r.getVolumeSnapshotter()
			if tc.expectedError != "" {
				assert.EqualError(t, err, tc.expectedError)
				return
			}
			require.NoError(t, err)

			assert.Equal(t, tc.plugin, a)
		})
	}
}

func TestRestartableVolumeSnapshotterReinitialize(t *testing.T) {
	p := new(restartabletest.MockRestartableProcess)
	p.Test(t)
	defer p.AssertExpectations(t)

	name := "aws"
	key := process.KindAndName{Kind: common.PluginKindVolumeSnapshotter, Name: name}
	r := &RestartableVolumeSnapshotter{
		Key:                 key,
		SharedPluginProcess: p,
		config: map[string]string{
			"color": "blue",
		},
	}

	err := r.Reinitialize(3)
	assert.EqualError(t, err, "int is not a VolumeSnapshotter!")

	volumeSnapshotter := new(providermocks.VolumeSnapshotter)
	volumeSnapshotter.Test(t)
	defer volumeSnapshotter.AssertExpectations(t)

	volumeSnapshotter.On("Init", r.config).Return(errors.Errorf("init error")).Once()
	err = r.Reinitialize(volumeSnapshotter)
	assert.EqualError(t, err, "init error")

	volumeSnapshotter.On("Init", r.config).Return(nil)
	err = r.Reinitialize(volumeSnapshotter)
	assert.NoError(t, err)
}

func TestRestartableVolumeSnapshotterGetDelegate(t *testing.T) {
	p := new(restartabletest.MockRestartableProcess)
	p.Test(t)
	defer p.AssertExpectations(t)

	// Reset error
	p.On("ResetIfNeeded").Return(errors.Errorf("reset error")).Once()
	name := "aws"
	key := process.KindAndName{Kind: common.PluginKindVolumeSnapshotter, Name: name}
	r := &RestartableVolumeSnapshotter{
		Key:                 key,
		SharedPluginProcess: p,
	}
	a, err := r.getDelegate()
	assert.Nil(t, a)
	assert.EqualError(t, err, "reset error")

	// Happy path
	p.On("ResetIfNeeded").Return(nil)
	volumeSnapshotter := new(providermocks.VolumeSnapshotter)
	volumeSnapshotter.Test(t)
	defer volumeSnapshotter.AssertExpectations(t)
	p.On("GetByKindAndName", key).Return(volumeSnapshotter, nil)

	a, err = r.getDelegate()
	assert.NoError(t, err)
	assert.Equal(t, volumeSnapshotter, a)
}

func TestRestartableVolumeSnapshotterInit(t *testing.T) {
	p := new(restartabletest.MockRestartableProcess)
	p.Test(t)
	defer p.AssertExpectations(t)

	// getVolumeSnapshottererror
	name := "aws"
	key := process.KindAndName{Kind: common.PluginKindVolumeSnapshotter, Name: name}
	r := &RestartableVolumeSnapshotter{
		Key:                 key,
		SharedPluginProcess: p,
	}
	p.On("GetByKindAndName", key).Return(nil, errors.Errorf("GetByKindAndName error")).Once()

	config := map[string]string{
		"color": "blue",
	}
	err := r.Init(config)
	assert.EqualError(t, err, "GetByKindAndName error")

	// Delegate returns error
	volumeSnapshotter := new(providermocks.VolumeSnapshotter)
	volumeSnapshotter.Test(t)
	defer volumeSnapshotter.AssertExpectations(t)
	p.On("GetByKindAndName", key).Return(volumeSnapshotter, nil)
	volumeSnapshotter.On("Init", config).Return(errors.Errorf("Init error")).Once()

	err = r.Init(config)
	assert.EqualError(t, err, "Init error")

	// wipe this out because the previous failed Init call set it
	r.config = nil

	// Happy path
	volumeSnapshotter.On("Init", config).Return(nil)
	err = r.Init(config)
	assert.NoError(t, err)
	assert.Equal(t, config, r.config)

	// Calling Init twice is forbidden
	err = r.Init(config)
	assert.EqualError(t, err, "already initialized")
}

func TestRestartableVolumeSnapshotterDelegatedFunctions(t *testing.T) {
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

	restartabletest.RunRestartableDelegateTests(
		t,
		common.PluginKindVolumeSnapshotter,
		func(key process.KindAndName, p process.RestartableProcess) interface{} {
			return &RestartableVolumeSnapshotter{
				Key:                 key,
				SharedPluginProcess: p,
			}
		},
		func() restartabletest.Mockable {
			return new(providermocks.VolumeSnapshotter)
		},
		restartabletest.RestartableDelegateTest{
			Function:                "CreateVolumeFromSnapshot",
			Inputs:                  []interface{}{"snapshotID", "volumeID", "volumeAZ", to.Int64Ptr(10000)},
			ExpectedErrorOutputs:    []interface{}{"", errors.Errorf("reset error")},
			ExpectedDelegateOutputs: []interface{}{"volumeID", errors.Errorf("delegate error")},
		},
		restartabletest.RestartableDelegateTest{
			Function:                "GetVolumeID",
			Inputs:                  []interface{}{pv},
			ExpectedErrorOutputs:    []interface{}{"", errors.Errorf("reset error")},
			ExpectedDelegateOutputs: []interface{}{"volumeID", errors.Errorf("delegate error")},
		},
		restartabletest.RestartableDelegateTest{
			Function:                "SetVolumeID",
			Inputs:                  []interface{}{pv, "volumeID"},
			ExpectedErrorOutputs:    []interface{}{nil, errors.Errorf("reset error")},
			ExpectedDelegateOutputs: []interface{}{pvToReturn, errors.Errorf("delegate error")},
		},
		restartabletest.RestartableDelegateTest{
			Function:                "GetVolumeInfo",
			Inputs:                  []interface{}{"volumeID", "volumeAZ"},
			ExpectedErrorOutputs:    []interface{}{"", (*int64)(nil), errors.Errorf("reset error")},
			ExpectedDelegateOutputs: []interface{}{"volumeType", to.Int64Ptr(10000), errors.Errorf("delegate error")},
		},
		restartabletest.RestartableDelegateTest{
			Function:                "CreateSnapshot",
			Inputs:                  []interface{}{"volumeID", "volumeAZ", map[string]string{"a": "b"}},
			ExpectedErrorOutputs:    []interface{}{"", errors.Errorf("reset error")},
			ExpectedDelegateOutputs: []interface{}{"snapshotID", errors.Errorf("delegate error")},
		},
		restartabletest.RestartableDelegateTest{
			Function:                "DeleteSnapshot",
			Inputs:                  []interface{}{"snapshotID"},
			ExpectedErrorOutputs:    []interface{}{errors.Errorf("reset error")},
			ExpectedDelegateOutputs: []interface{}{errors.Errorf("delegate error")},
		},
	)
}
