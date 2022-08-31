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
	"context"
	"testing"
	"time"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"

	v1 "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	"github.com/vmware-tanzu/velero/pkg/plugin/velero/item_snapshotter/v1/mocks"

	"github.com/vmware-tanzu/velero/pkg/plugin/clientmgmt/process"
	"github.com/vmware-tanzu/velero/pkg/plugin/framework"
	"github.com/vmware-tanzu/velero/pkg/plugin/velero"
	isv1 "github.com/vmware-tanzu/velero/pkg/plugin/velero/item_snapshotter/v1"
)

func TestRestartableGetItemSnapshotter(t *testing.T) {
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
			expectedError: "int is not an ItemSnapshotter!",
		},
		{
			name:   "happy path",
			plugin: new(mocks.ItemSnapshotter),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			p := new(mockRestartableProcess)
			defer p.AssertExpectations(t)

			name := "pvc"
			key := process.KindAndName{Kind: framework.PluginKindItemSnapshotter, Name: name}
			p.On("GetByKindAndName", key).Return(tc.plugin, tc.getError)

			r := NewRestartableItemSnapshotter(name, p)
			a, err := r.getItemSnapshotter()
			if tc.expectedError != "" {
				assert.EqualError(t, err, tc.expectedError)
				return
			}
			require.NoError(t, err)

			assert.Equal(t, tc.plugin, a)
		})
	}
}

func TestRestartableItemSnapshotterGetDelegate(t *testing.T) {
	p := new(mockRestartableProcess)
	defer p.AssertExpectations(t)

	// Reset error
	p.On("ResetIfNeeded").Return(errors.Errorf("reset error")).Once()
	name := "pvc"
	r := NewRestartableItemSnapshotter(name, p)
	a, err := r.getDelegate()
	assert.Nil(t, a)
	assert.EqualError(t, err, "reset error")

	// Happy path
	p.On("ResetIfNeeded").Return(nil)
	expected := new(mocks.ItemSnapshotter)
	key := process.KindAndName{Kind: framework.PluginKindItemSnapshotter, Name: name}
	p.On("GetByKindAndName", key).Return(expected, nil)

	a, err = r.getDelegate()
	assert.NoError(t, err)
	assert.Equal(t, expected, a)
}

func TestRestartableItemSnasphotterDelegatedFunctions(t *testing.T) {
	b := new(v1.Backup)

	pv := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"color": "blue",
		},
	}

	sii := &isv1.SnapshotItemInput{
		Item:   pv,
		Params: nil,
		Backup: b,
	}

	ctx := context.Background()

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

	sio := &isv1.SnapshotItemOutput{
		UpdatedItem:      pvToReturn,
		SnapshotID:       "",
		SnapshotMetadata: nil,
		AdditionalItems:  additionalItems,
		HandledItems:     nil,
	}

	cii := &isv1.CreateItemInput{
		SnapshottedItem:  nil,
		SnapshotID:       "",
		ItemFromBackup:   nil,
		SnapshotMetadata: nil,
		Params:           nil,
		Restore:          nil,
	}

	cio := &isv1.CreateItemOutput{
		UpdatedItem:     nil,
		AdditionalItems: nil,
		SkipRestore:     false,
	}

	pi := &isv1.ProgressInput{
		ItemID:     velero.ResourceIdentifier{},
		SnapshotID: "",
		Backup:     nil,
	}
	po := &isv1.ProgressOutput{
		Phase:           isv1.SnapshotPhaseInProgress,
		Err:             "",
		ItemsCompleted:  0,
		ItemsToComplete: 0,
		Started:         time.Time{},
		Updated:         time.Time{},
	}
	dsi := &isv1.DeleteSnapshotInput{
		SnapshotID:       "",
		ItemFromBackup:   nil,
		SnapshotMetadata: nil,
		Params:           nil,
	}
	runRestartableDelegateTests(
		t,
		framework.PluginKindItemSnapshotter,
		func(key process.KindAndName, p process.RestartableProcess) interface{} {
			return &restartableItemSnapshotter{
				key:                 key,
				sharedPluginProcess: p,
			}
		},
		func() mockable {
			return new(mocks.ItemSnapshotter)
		},
		restartableDelegateTest{
			function:                "Init",
			inputs:                  []interface{}{map[string]string{}},
			expectedErrorOutputs:    []interface{}{errors.Errorf("reset error")},
			expectedDelegateOutputs: []interface{}{errors.Errorf("delegate error")},
		},
		restartableDelegateTest{
			function:                "AppliesTo",
			inputs:                  []interface{}{},
			expectedErrorOutputs:    []interface{}{velero.ResourceSelector{}, errors.Errorf("reset error")},
			expectedDelegateOutputs: []interface{}{velero.ResourceSelector{IncludedNamespaces: []string{"a"}}, errors.Errorf("delegate error")},
		},
		restartableDelegateTest{
			function:                "AlsoHandles",
			inputs:                  []interface{}{&isv1.AlsoHandlesInput{}},
			expectedErrorOutputs:    []interface{}{[]velero.ResourceIdentifier([]velero.ResourceIdentifier(nil)), errors.Errorf("reset error")},
			expectedDelegateOutputs: []interface{}{[]velero.ResourceIdentifier([]velero.ResourceIdentifier(nil)), errors.Errorf("delegate error")},
		},
		restartableDelegateTest{
			function:                "SnapshotItem",
			inputs:                  []interface{}{ctx, sii},
			expectedErrorOutputs:    []interface{}{nil, errors.Errorf("reset error")},
			expectedDelegateOutputs: []interface{}{sio, errors.Errorf("delegate error")},
		},
		restartableDelegateTest{
			function:                "CreateItemFromSnapshot",
			inputs:                  []interface{}{ctx, cii},
			expectedErrorOutputs:    []interface{}{nil, errors.Errorf("reset error")},
			expectedDelegateOutputs: []interface{}{cio, errors.Errorf("delegate error")},
		},
		restartableDelegateTest{
			function:                "Progress",
			inputs:                  []interface{}{pi},
			expectedErrorOutputs:    []interface{}{nil, errors.Errorf("reset error")},
			expectedDelegateOutputs: []interface{}{po, errors.Errorf("delegate error")},
		},
		restartableDelegateTest{
			function:                "DeleteSnapshot",
			inputs:                  []interface{}{ctx, dsi},
			expectedErrorOutputs:    []interface{}{errors.Errorf("reset error")},
			expectedDelegateOutputs: []interface{}{errors.Errorf("delegate error")},
		},
	)
}
