/*
Copyright 2017 the Heptio Ark contributors.

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

	"github.com/heptio/ark/pkg/util/kube"
	arktest "github.com/heptio/ark/pkg/util/test"
	"github.com/stretchr/testify/assert"

	corev1api "k8s.io/api/core/v1"
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
		obj         kube.UnstructuredObject
		expectedErr bool
		expectedRes kube.UnstructuredObject
	}{
		{
			name:        "no spec should error",
			obj:         NewTestUnstructured().WithName("svc-1").Unstructured,
			expectedErr: true,
		},
		{
			name:        "no spec ports should error",
			obj:         NewTestUnstructured().WithName("svc-1").WithSpec().Unstructured,
			expectedErr: true,
		},
		{
			name:        "clusterIP (only) should be deleted from spec",
			obj:         NewTestUnstructured().WithName("svc-1").WithSpec("clusterIP", "foo").WithSpecField("ports", []interface{}{}).Unstructured,
			expectedErr: false,
			expectedRes: NewTestUnstructured().WithName("svc-1").WithSpec("foo").WithSpecField("ports", []interface{}{}).Unstructured,
		},
		{
			name:        "headless clusterIP should not be deleted from spec",
			obj:         NewTestUnstructured().WithName("svc-1").WithSpecField("clusterIP", "None").WithSpecField("ports", []interface{}{}).Unstructured,
			expectedErr: false,
			expectedRes: NewTestUnstructured().WithName("svc-1").WithSpecField("clusterIP", "None").WithSpecField("ports", []interface{}{}).Unstructured,
		},
		{
			name: "nodePort (only) should be deleted from all spec.ports",
			obj: NewTestUnstructured().WithName("svc-1").
				WithSpecField("ports", []interface{}{
					map[string]interface{}{"nodePort": ""},
					map[string]interface{}{"nodePort": "", "foo": "bar"},
				}).Unstructured,
			expectedErr: false,
			expectedRes: NewTestUnstructured().WithName("svc-1").
				WithSpecField("ports", []interface{}{
					map[string]interface{}{},
					map[string]interface{}{"foo": "bar"},
				}).Unstructured,
		},
		{
			name: "unnamed nodePort should be deleted when missing in annotation",
			obj: NewTestUnstructured().WithName("svc-1").
				WithAnnotationValues(map[string]string{
					annotationLastAppliedConfig: svcJSON(),
				}).
				WithSpecField("ports", []interface{}{
					map[string]interface{}{"nodePort": 8080},
				}).Unstructured,
			expectedErr: false,
			expectedRes: NewTestUnstructured().WithName("svc-1").
				WithAnnotationValues(map[string]string{
					annotationLastAppliedConfig: svcJSON(),
				}).
				WithSpecField("ports", []interface{}{
					map[string]interface{}{},
				}).Unstructured,
		},
		{
			name: "unnamed nodePort should be preserved when specified in annotation",
			obj: NewTestUnstructured().WithName("svc-1").
				WithAnnotationValues(map[string]string{
					annotationLastAppliedConfig: svcJSON(corev1api.ServicePort{NodePort: 8080}),
				}).
				WithSpecField("ports", []interface{}{
					map[string]interface{}{
						"nodePort": 8080,
					},
				}).Unstructured,
			expectedErr: false,
			expectedRes: NewTestUnstructured().WithName("svc-1").
				WithAnnotationValues(map[string]string{
					annotationLastAppliedConfig: svcJSON(corev1api.ServicePort{NodePort: 8080}),
				}).
				WithSpecField("ports", []interface{}{
					map[string]interface{}{
						"nodePort": 8080,
					},
				}).Unstructured,
		},
		{
			name: "unnamed nodePort should be deleted when named nodePort specified in annotation",
			obj: NewTestUnstructured().WithName("svc-1").
				WithAnnotationValues(map[string]string{
					annotationLastAppliedConfig: svcJSON(corev1api.ServicePort{Name: "http", NodePort: 8080}),
				}).
				WithSpecField("ports", []interface{}{
					map[string]interface{}{
						"nodePort": 8080,
					},
				}).Unstructured,
			expectedErr: false,
			expectedRes: NewTestUnstructured().WithName("svc-1").
				WithAnnotationValues(map[string]string{
					annotationLastAppliedConfig: svcJSON(corev1api.ServicePort{Name: "http", NodePort: 8080}),
				}).
				WithSpecField("ports", []interface{}{
					map[string]interface{}{},
				}).Unstructured,
		},
		{
			name: "named nodePort should be preserved when specified in annotation",
			obj: NewTestUnstructured().WithName("svc-1").
				WithAnnotationValues(map[string]string{
					annotationLastAppliedConfig: svcJSON(corev1api.ServicePort{Name: "http", NodePort: 8080}),
				}).
				WithSpecField("ports", []interface{}{
					map[string]interface{}{
						"name":     "http",
						"nodePort": 8080,
					},
					map[string]interface{}{
						"name":     "admin",
						"nodePort": 9090,
					},
				}).Unstructured,
			expectedErr: false,
			expectedRes: NewTestUnstructured().WithName("svc-1").
				WithAnnotationValues(map[string]string{
					annotationLastAppliedConfig: svcJSON(corev1api.ServicePort{Name: "http", NodePort: 8080}),
				}).
				WithSpecField("ports", []interface{}{
					map[string]interface{}{
						"name":     "http",
						"nodePort": 8080,
					},
					map[string]interface{}{
						"name": "admin",
					},
				}).Unstructured,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			action := NewServiceAction(arktest.NewLogger())

			res, _, err := action.Execute(test.obj, nil)

			if assert.Equal(t, test.expectedErr, err != nil) {
				assert.Equal(t, test.expectedRes, res)
			}
		})
	}
}
