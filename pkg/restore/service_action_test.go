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
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1api "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"

	api "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	"github.com/vmware-tanzu/velero/pkg/builder"
	"github.com/vmware-tanzu/velero/pkg/plugin/velero"
	velerotest "github.com/vmware-tanzu/velero/pkg/test"
)

func svcJSON(ports ...corev1api.ServicePort) string {
	svc := corev1api.Service{
		Spec: corev1api.ServiceSpec{
			Ports: ports,
		},
	}

	data, err := json.Marshal(svc)
	if err != nil {
		panic(err)
	}

	return string(data)
}

func TestServiceActionExecute(t *testing.T) {

	tests := []struct {
		name        string
		obj         corev1api.Service
		restore     *api.Restore
		expectedErr bool
		expectedRes corev1api.Service
	}{
		{
			name: "clusterIP (only) should be deleted from spec",
			obj: corev1api.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name: "svc-1",
				},
				Spec: corev1api.ServiceSpec{
					ClusterIP:      "should-be-removed",
					LoadBalancerIP: "should-be-kept",
				},
			},
			restore:     builder.ForRestore(api.DefaultNamespace, "").Result(),
			expectedErr: false,
			expectedRes: corev1api.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name: "svc-1",
				},
				Spec: corev1api.ServiceSpec{
					LoadBalancerIP: "should-be-kept",
				},
			},
		},
		{
			name: "headless clusterIP should not be deleted from spec",
			obj: corev1api.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name: "svc-1",
				},
				Spec: corev1api.ServiceSpec{
					ClusterIP: "None",
				},
			},
			restore: builder.ForRestore(api.DefaultNamespace, "").Result(),
			expectedRes: corev1api.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name: "svc-1",
				},
				Spec: corev1api.ServiceSpec{
					ClusterIP: "None",
				},
			},
		},
		{
			name: "nodePort (only) should be deleted from all spec.ports",
			obj: corev1api.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name: "svc-1",
				},
				Spec: corev1api.ServiceSpec{
					Ports: []corev1api.ServicePort{
						{
							Port:     32000,
							NodePort: 32000,
						},
						{
							Port:     32001,
							NodePort: 32001,
						},
					},
				},
			},
			restore: builder.ForRestore(api.DefaultNamespace, "").Result(),
			expectedRes: corev1api.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name: "svc-1",
				},
				Spec: corev1api.ServiceSpec{
					Ports: []corev1api.ServicePort{
						{
							Port: 32000,
						},
						{
							Port: 32001,
						},
					},
				},
			},
		},
		{
			name: "unnamed nodePort should be deleted when missing in annotation",
			obj: corev1api.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name: "svc-1",
					Annotations: map[string]string{
						annotationLastAppliedConfig: svcJSON(),
					},
				},
				Spec: corev1api.ServiceSpec{
					Ports: []corev1api.ServicePort{
						{
							NodePort: 8080,
						},
					},
				},
			},
			restore: builder.ForRestore(api.DefaultNamespace, "").Result(),
			expectedRes: corev1api.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name: "svc-1",
					Annotations: map[string]string{
						annotationLastAppliedConfig: svcJSON(),
					},
				},
				Spec: corev1api.ServiceSpec{
					Ports: []corev1api.ServicePort{
						{},
					},
				},
			},
		},
		{
			name: "unnamed nodePort should be preserved when specified in annotation",
			obj: corev1api.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name: "svc-1",
					Annotations: map[string]string{
						annotationLastAppliedConfig: svcJSON(corev1api.ServicePort{NodePort: 8080}),
					},
				},
				Spec: corev1api.ServiceSpec{
					Ports: []corev1api.ServicePort{
						{
							NodePort: 8080,
						},
					},
				},
			},
			restore: builder.ForRestore(api.DefaultNamespace, "").Result(),
			expectedRes: corev1api.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name: "svc-1",
					Annotations: map[string]string{
						annotationLastAppliedConfig: svcJSON(corev1api.ServicePort{NodePort: 8080}),
					},
				},
				Spec: corev1api.ServiceSpec{
					Ports: []corev1api.ServicePort{
						{
							NodePort: 8080,
						},
					},
				},
			},
		},
		{
			name: "unnamed nodePort should be deleted when named nodePort specified in annotation",
			obj: corev1api.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name: "svc-1",
					Annotations: map[string]string{
						annotationLastAppliedConfig: svcJSON(corev1api.ServicePort{Name: "http", NodePort: 8080}),
					},
				},
				Spec: corev1api.ServiceSpec{
					Ports: []corev1api.ServicePort{
						{
							NodePort: 8080,
						},
					},
				},
			},
			restore: builder.ForRestore(api.DefaultNamespace, "").Result(),
			expectedRes: corev1api.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name: "svc-1",
					Annotations: map[string]string{
						annotationLastAppliedConfig: svcJSON(corev1api.ServicePort{Name: "http", NodePort: 8080}),
					},
				},
				Spec: corev1api.ServiceSpec{
					Ports: []corev1api.ServicePort{
						{},
					},
				},
			},
		},
		{
			name: "named nodePort should be preserved when specified in annotation",
			obj: corev1api.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name: "svc-1",
					Annotations: map[string]string{
						annotationLastAppliedConfig: svcJSON(corev1api.ServicePort{Name: "http", NodePort: 8080}),
					},
				},
				Spec: corev1api.ServiceSpec{
					Ports: []corev1api.ServicePort{
						{
							Name:     "http",
							NodePort: 8080,
						},
						{
							Name:     "admin",
							NodePort: 9090,
						},
					},
				},
			},
			restore: builder.ForRestore(api.DefaultNamespace, "").Result(),
			expectedRes: corev1api.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name: "svc-1",
					Annotations: map[string]string{
						annotationLastAppliedConfig: svcJSON(corev1api.ServicePort{Name: "http", NodePort: 8080}),
					},
				},
				Spec: corev1api.ServiceSpec{
					Ports: []corev1api.ServicePort{
						{
							Name:     "http",
							NodePort: 8080,
						},
						{
							Name: "admin",
						},
					},
				},
			},
		},
		{
			name: "If PreserveNodePorts is True in restore spec then nodePort always preserved.",
			obj: corev1api.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name: "svc-1",
				},
				Spec: corev1api.ServiceSpec{
					Ports: []corev1api.ServicePort{
						{
							Name:     "http",
							Port:     80,
							NodePort: 8080,
						},
						{
							Name:     "hepsiburada",
							NodePort: 9025,
						},
					},
				},
			},
			restore: builder.ForRestore(api.DefaultNamespace, "").PreserveNodePorts(true).Result(),
			expectedRes: corev1api.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name: "svc-1",
				},
				Spec: corev1api.ServiceSpec{
					Ports: []corev1api.ServicePort{
						{
							Name:     "http",
							Port:     80,
							NodePort: 8080,
						},
						{
							Name:     "hepsiburada",
							NodePort: 9025,
						},
					},
				},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			action := NewServiceAction(velerotest.NewLogger())

			unstructuredSvc, err := runtime.DefaultUnstructuredConverter.ToUnstructured(&test.obj)
			require.NoError(t, err)

			res, err := action.Execute(&velero.RestoreItemActionExecuteInput{
				Item:           &unstructured.Unstructured{Object: unstructuredSvc},
				ItemFromBackup: &unstructured.Unstructured{Object: unstructuredSvc},
				Restore:        test.restore,
			})

			if assert.Equal(t, test.expectedErr, err != nil) && !test.expectedErr {
				var svc corev1api.Service
				require.NoError(t, runtime.DefaultUnstructuredConverter.FromUnstructured(res.UpdatedItem.UnstructuredContent(), &svc))

				assert.Equal(t, test.expectedRes, svc)
			}
		})
	}
}
