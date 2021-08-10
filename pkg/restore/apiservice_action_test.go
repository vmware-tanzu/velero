/*
Copyright 2017 the Velero contributors.

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
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	apiregistrationv1 "k8s.io/kube-aggregator/pkg/apis/apiregistration/v1"
	"k8s.io/kube-aggregator/pkg/controllers/autoregister"

	"github.com/vmware-tanzu/velero/pkg/plugin/velero"
	velerotest "github.com/vmware-tanzu/velero/pkg/test"
)

func TestAPIServiceActionExecute(t *testing.T) {
	tests := []struct {
		name        string
		obj         apiregistrationv1.APIService
		skipRestore bool
	}{
		{
			name: "APIService with no labels should be restored without modification",
			obj: apiregistrationv1.APIService{
				ObjectMeta: metav1.ObjectMeta{
					Name: "v1.foo.velero.io",
				},
			},
			skipRestore: false,
		},
		{
			name: "Non-Local APIService without Kubernetes managed label should be restored without modification",
			obj: apiregistrationv1.APIService{
				ObjectMeta: metav1.ObjectMeta{
					Name: "v1.foo.velero.io",
					Labels: map[string]string{
						"component": "velero",
					},
				},
				Spec: apiregistrationv1.APIServiceSpec{
					Group:   "velero.io",
					Version: "v1",
					Service: &apiregistrationv1.ServiceReference{
						Namespace: "velero",
						Name:      "velero-aggregated-api-server",
					},
				},
			},
			skipRestore: false,
		},
		{
			name: "APIService with Kubernetes managed label with 'true' value should not be restored",
			obj: apiregistrationv1.APIService{
				ObjectMeta: metav1.ObjectMeta{
					Name: "v1.foo.velero.io",
					Labels: map[string]string{
						autoregister.AutoRegisterManagedLabel: "true",
					},
				},
			},
			skipRestore: true,
		},
		{
			name: "APIService with Kubernetes managed label with 'onstart' value should not be restored",
			obj: apiregistrationv1.APIService{
				ObjectMeta: metav1.ObjectMeta{
					Name: "v1.foo.velero.io",
					Labels: map[string]string{
						autoregister.AutoRegisterManagedLabel: "onstart",
					},
				},
			},
			skipRestore: true,
		},
		{
			name: "APIService with Kubernetes managed label with any value should not be restored",
			obj: apiregistrationv1.APIService{
				ObjectMeta: metav1.ObjectMeta{
					Name: "v1.foo.velero.io",
					Labels: map[string]string{
						autoregister.AutoRegisterManagedLabel: "randomvalue",
					},
				},
			},
			skipRestore: true,
		},
		{
			name: "Non-Local APIService with Kubernetes managed label should not be restored",
			obj: apiregistrationv1.APIService{
				ObjectMeta: metav1.ObjectMeta{
					Name: "v1.foo.velero.io",
					Labels: map[string]string{
						autoregister.AutoRegisterManagedLabel: "onstart",
					},
				},
				Spec: apiregistrationv1.APIServiceSpec{
					Group:   "velero.io",
					Version: "v1",
					Service: &apiregistrationv1.ServiceReference{
						Namespace: "velero",
						Name:      "velero-aggregated-api-server",
					},
				},
			},
			skipRestore: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			action := NewAPIServiceAction(velerotest.NewLogger())

			unstructuredAPIService, err := runtime.DefaultUnstructuredConverter.ToUnstructured(&test.obj)
			require.NoError(t, err)

			res, err := action.Execute(&velero.RestoreItemActionExecuteInput{
				Item:           &unstructured.Unstructured{Object: unstructuredAPIService},
				ItemFromBackup: &unstructured.Unstructured{Object: unstructuredAPIService},
			})

			require.NoError(t, err)

			var apiService apiregistrationv1.APIService
			require.NoError(t, runtime.DefaultUnstructuredConverter.FromUnstructured(res.UpdatedItem.UnstructuredContent(), &apiService))
			assert.Equal(t, test.obj, apiService)
			assert.Equal(t, test.skipRestore, res.SkipRestore)
		})
	}
}
