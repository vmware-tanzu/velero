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

package restore

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"

	velerov1 "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	velerov2alpha1 "github.com/vmware-tanzu/velero/pkg/apis/velero/v2alpha1"
	"github.com/vmware-tanzu/velero/pkg/builder"
	"github.com/vmware-tanzu/velero/pkg/plugin/velero"
	velerotest "github.com/vmware-tanzu/velero/pkg/test"
)

func TestDataUploadRetrieveActionExectue(t *testing.T) {
	tests := []struct {
		name                     string
		dataUpload               *velerov2alpha1.DataUpload
		restore                  *velerov1.Restore
		expectedDataUploadResult *corev1.ConfigMap
		expectedErr              string
	}{
		{
			name:                     "DataUploadRetrieve Action test",
			dataUpload:               builder.ForDataUpload("velero", "testDU").SourceNamespace("testNamespace").SourcePVC("testPVC").Result(),
			restore:                  builder.ForRestore("velero", "testRestore").ObjectMeta(builder.WithUID("testingUID")).Result(),
			expectedDataUploadResult: builder.ForConfigMap("velero", "").ObjectMeta(builder.WithGenerateName("testDU-"), builder.WithLabels(velerov1.PVCNamespaceNameLabel, "testNamespace.testPVC", velerov1.RestoreUIDLabel, "testingUID", velerov1.ResourceUsageLabel, string(velerov1.VeleroResourceUsageDataUploadResult))).Data("testingUID", `{"backupStorageLocation":"","sourceNamespace":"testNamespace"}`).Result(),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			logger := velerotest.NewLogger()
			cmClient := fake.NewSimpleClientset()

			var unstructuredDataUpload map[string]interface{}
			if tc.dataUpload != nil {
				var err error
				unstructuredDataUpload, err = runtime.DefaultUnstructuredConverter.ToUnstructured(tc.dataUpload)
				require.NoError(t, err)
			}
			input := velero.RestoreItemActionExecuteInput{
				Restore:        tc.restore,
				ItemFromBackup: &unstructured.Unstructured{Object: unstructuredDataUpload},
			}

			action := NewDataUploadRetrieveAction(logger, cmClient.CoreV1().ConfigMaps("velero"))
			_, err := action.Execute(&input)
			if tc.expectedErr != "" {
				require.Equal(t, tc.expectedErr, err.Error())
			}
			require.NoError(t, err)

			if tc.expectedDataUploadResult != nil {
				cmList, err := cmClient.CoreV1().ConfigMaps("velero").List(context.Background(), metav1.ListOptions{
					LabelSelector: fmt.Sprintf("%s=%s,%s=%s", velerov1.RestoreUIDLabel, "testingUID", velerov1.PVCNamespaceNameLabel, tc.dataUpload.Spec.SourceNamespace+"."+tc.dataUpload.Spec.SourcePVC),
				})
				require.NoError(t, err)
				// debug
				fmt.Printf("CM: %s\n", &cmList.Items[0])
				require.Equal(t, *tc.expectedDataUploadResult, cmList.Items[0])
			}
		})
	}
}
