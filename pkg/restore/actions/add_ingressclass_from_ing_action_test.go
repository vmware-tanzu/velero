/*
Copyright 2019 the Velero contributors.

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

package actions

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	networkapi "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/pointer"

	"github.com/vmware-tanzu/velero/pkg/kuberesource"
	"github.com/vmware-tanzu/velero/pkg/plugin/velero"
	velerotest "github.com/vmware-tanzu/velero/pkg/test"
)

func TestAddIngressClassFromIngActionExecute(t *testing.T) {
	tests := []struct {
		name string
		item *networkapi.Ingress
		want []velero.ResourceIdentifier
	}{
		{
			name: "ingress with no related ingressClass",
			item: &networkapi.Ingress{},
			want: nil,
		},
		{
			name: "ingress with related ingressClass returns ingressClass as additional item",
			item: &networkapi.Ingress{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "ns-1",
					Name:      "ing",
				},
				Spec: networkapi.IngressSpec{
					IngressClassName: pointer.StringPtr("ingressclass"),
				},
			},
			want: []velero.ResourceIdentifier{
				{GroupResource: kuberesource.IngressClasses, Name: "ingressclass"},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			itemData, err := runtime.DefaultUnstructuredConverter.ToUnstructured(test.item)
			require.NoError(t, err)

			action := &AddIngressClassFromIngAction{logger: velerotest.NewLogger()}

			input := &velero.RestoreItemActionExecuteInput{
				Item: &unstructured.Unstructured{Object: itemData},
			}

			res, err := action.Execute(input)
			require.NoError(t, err)

			assert.Equal(t, test.want, res.AdditionalItems)
		})
	}
}
