/*
Copyright 2017 Heptio Inc.

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

package backup

import (
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"

	testutil "github.com/heptio/ark/pkg/util/test"
	testlogger "github.com/sirupsen/logrus/hooks/test"
	"github.com/stretchr/testify/assert"
)

func TestBackupPVAction(t *testing.T) {
	tests := []struct {
		name        string
		item        map[string]interface{}
		volumeName  string
		expectedErr bool
	}{
		{
			name: "execute PV backup in normal case",
			item: map[string]interface{}{
				"metadata": map[string]interface{}{"name": "pvc-1"},
				"spec":     map[string]interface{}{"volumeName": "pv-1"},
			},
			volumeName:  "pv-1",
			expectedErr: false,
		},
		{
			name: "error when PVC has no metadata.name",
			item: map[string]interface{}{
				"metadata": map[string]interface{}{},
				"spec":     map[string]interface{}{"volumeName": "pv-1"},
			},
			expectedErr: true,
		},
		{
			name: "error when PVC has no spec.volumeName",
			item: map[string]interface{}{
				"metadata": map[string]interface{}{"name": "pvc-1"},
				"spec":     map[string]interface{}{},
			},
			expectedErr: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var (
				discoveryHelper = testutil.NewFakeDiscoveryHelper(true, nil)
				dynamicFactory  = &testutil.FakeDynamicFactory{}
				dynamicClient   = &testutil.FakeDynamicClient{}
				testLogger, _   = testlogger.NewNullLogger()
				ctx             = &backupContext{discoveryHelper: discoveryHelper, dynamicFactory: dynamicFactory, logger: testLogger}
				backupper       = &fakeItemBackupper{}
				action          = NewBackupPVAction()
				pv              = &unstructured.Unstructured{}
				pvGVR           = schema.GroupVersionResource{Resource: "persistentvolumes"}
			)

			dynamicFactory.On("ClientForGroupVersionResource",
				pvGVR,
				metav1.APIResource{Name: "persistentvolumes"},
				"",
			).Return(dynamicClient, nil)

			dynamicClient.On("Get", test.volumeName, metav1.GetOptions{}).Return(pv, nil)

			backupper.On("backupItem", ctx, pv.UnstructuredContent(), pvGVR.GroupResource()).Return(nil)

			// method under test
			res := action.Execute(ctx, test.item, backupper)

			assert.Equal(t, test.expectedErr, res != nil)
		})
	}
}
