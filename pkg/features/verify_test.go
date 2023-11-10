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

package features

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	findermocks "github.com/vmware-tanzu/velero/pkg/features/mocks"
)

func TestVerify(t *testing.T) {
	NewFeatureFlagSet()
	verifier := verifier{}

	finder := new(findermocks.PluginFinder)
	finder.On("Find", mock.Anything, mock.Anything).Return(false)
	verifier.finder = finder
	ready, err := verifier.Verify("EnableCSI")
	assert.Equal(t, false, ready)
	assert.Nil(t, err)

	finder = new(findermocks.PluginFinder)
	finder.On("Find", mock.Anything, mock.Anything).Return(true)
	verifier.finder = finder
	ready, err = verifier.Verify("EnableCSI")
	assert.Equal(t, false, ready)
	assert.EqualError(t, err, "CSI plugins are registered, but the EnableCSI feature is not enabled")

	Enable("EnableCSI")
	finder = new(findermocks.PluginFinder)
	finder.On("Find", mock.Anything, mock.Anything).Return(false)
	verifier.finder = finder
	ready, err = verifier.Verify("EnableCSI")
	assert.Equal(t, false, ready)
	assert.EqualError(t, err, "CSI feature is enabled, but CSI plugins are not registered")

	Enable("EnableCSI")
	finder = new(findermocks.PluginFinder)
	finder.On("Find", mock.Anything, mock.Anything).Return(true)
	verifier.finder = finder
	ready, err = verifier.Verify("EnableCSI")
	assert.Equal(t, true, ready)
	assert.Nil(t, err)
}
