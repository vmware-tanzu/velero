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

package restore

import (
	"context"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	corev1api "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/vmware-tanzu/velero/pkg/builder"
	"github.com/vmware-tanzu/velero/pkg/plugin/velero"
)

// TestChangeImageNameActionExecute runs the ChangeImageNameAction's Execute
// method and validates that the item's image name is modified (or not) as expected.
// Validation is done by comparing the result of the Execute method to the test case's
// desired result.
func TestChangeImageRepositoryActionExecute(t *testing.T) {
	tests := []struct {
		name             string
		podOrObj         interface{}
		configMap        *corev1api.ConfigMap
		freshedImageName string
		imageNameSlice   []string
		want             interface{}
		wantErr          error
	}{
		{
			name: "a valid mapping with spaces for a new image repository is applied correctly",
			podOrObj: builder.ForPod("default", "pod1").ObjectMeta().
				Containers(&v1.Container{
					Name:  "container1",
					Image: "1.1.1.1:5000/abc:test",
				}).Result(),
			configMap: builder.ForConfigMap("velero", "change-image-name").
				ObjectMeta(builder.WithLabels("velero.io/plugin-config", "true", "velero.io/change-image-name", "RestoreItemAction")).
				Data("case1", "1.1.1.1:5000  ,  2.2.2.2:3000").
				Result(),
			freshedImageName: "2.2.2.2:3000/abc:test",
			want:             "2.2.2.2:3000/abc:test",
		},

		{
			name: "a valid mapping for a new image repository is applied correctly",
			podOrObj: builder.ForPod("default", "pod1").ObjectMeta().
				Containers(&v1.Container{
					Name:  "container2",
					Image: "1.1.1.1:5000/abc:test",
				}).Result(),
			configMap: builder.ForConfigMap("velero", "change-image-name").
				ObjectMeta(builder.WithLabels("velero.io/plugin-config", "true", "velero.io/change-image-name", "RestoreItemAction")).
				Data("specific", "1.1.1.1:5000,2.2.2.2:3000").
				Result(),
			freshedImageName: "2.2.2.2:3000/abc:test",
			want:             "2.2.2.2:3000/abc:test",
		},

		{
			name: "a valid mapping for a new image name is applied correctly",
			podOrObj: builder.ForPod("default", "pod1").ObjectMeta().
				Containers(&v1.Container{
					Name:  "container3",
					Image: "1.1.1.1:5000/abc:test",
				}).Result(),
			configMap: builder.ForConfigMap("velero", "change-image-name").
				ObjectMeta(builder.WithLabels("velero.io/plugin-config", "true", "velero.io/change-image-name", "RestoreItemAction")).
				Data("specific", "abc:test,myproject:latest").
				Result(),
			freshedImageName: "1.1.1.1:5000/myproject:latest",
			want:             "1.1.1.1:5000/myproject:latest",
		},

		{
			name: "a valid mapping for a new image repository port is applied correctly",
			podOrObj: builder.ForPod("default", "pod1").ObjectMeta().
				Containers(&v1.Container{
					Name:  "container4",
					Image: "1.1.1.1:5000/abc:test",
				}).Result(),
			configMap: builder.ForConfigMap("velero", "change-image-name").
				ObjectMeta(builder.WithLabels("velero.io/plugin-config", "true", "velero.io/change-image-name", "RestoreItemAction")).
				Data("specific", "5000,3333").
				Result(),
			freshedImageName: "1.1.1.1:5000/abc:test",
			want:             "1.1.1.1:3333/abc:test",
		},

		{
			name: "a valid mapping for a new image tag is applied correctly",
			podOrObj: builder.ForPod("default", "pod1").ObjectMeta().
				Containers(&v1.Container{
					Name:  "container5",
					Image: "1.1.1.1:5000/abc:test",
				}).Result(),
			configMap: builder.ForConfigMap("velero", "change-image-name").
				ObjectMeta(builder.WithLabels("velero.io/plugin-config", "true", "velero.io/change-image-name", "RestoreItemAction")).
				Data("specific", "test,latest").
				Result(),
			freshedImageName: "1.1.1.1:5000/abc:test",
			want:             "1.1.1.1:5000/abc:latest",
		},

		{
			name: "image name contains more than one part that matching the replacing words.",
			podOrObj: builder.ForPod("default", "pod1").ObjectMeta().
				Containers(&v1.Container{
					Name:  "container6",
					Image: "dev/image1:dev",
				}).Result(),
			configMap: builder.ForConfigMap("velero", "change-image-name").
				ObjectMeta(builder.WithLabels("velero.io/plugin-config", "true", "velero.io/change-image-name", "RestoreItemAction")).
				Data("specific", "dev/,test/").
				Result(),
			freshedImageName: "dev/image1:dev",
			want:             "test/image1:dev",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			clientset := fake.NewSimpleClientset()
			a := NewChangeImageNameAction(
				logrus.StandardLogger(),
				clientset.CoreV1().ConfigMaps("velero"),
			)

			// set up test data
			if tc.configMap != nil {
				_, err := clientset.CoreV1().ConfigMaps(tc.configMap.Namespace).Create(context.TODO(), tc.configMap, metav1.CreateOptions{})
				require.NoError(t, err)
			}

			unstructuredMap, err := runtime.DefaultUnstructuredConverter.ToUnstructured(tc.podOrObj)
			require.NoError(t, err)

			input := &velero.RestoreItemActionExecuteInput{
				Item: &unstructured.Unstructured{
					Object: unstructuredMap,
				},
			}

			// execute method under test
			res, err := a.Execute(input)

			// validate for both error and non-error cases
			switch {
			case tc.wantErr != nil:
				assert.EqualError(t, err, tc.wantErr.Error())
			default:
				assert.NoError(t, err)
				pod := new(corev1.Pod)
				err = runtime.DefaultUnstructuredConverter.FromUnstructured(res.UpdatedItem.UnstructuredContent(), pod)
				require.NoError(t, err)
				assert.Equal(t, pod.Spec.Containers[0].Image, tc.want)
			}
		})
	}
}
