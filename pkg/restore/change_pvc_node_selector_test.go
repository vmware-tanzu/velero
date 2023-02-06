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
	"bytes"
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1api "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/vmware-tanzu/velero/pkg/builder"
	"github.com/vmware-tanzu/velero/pkg/plugin/velero"
)

// TestChangePVCNodeSelectorActionExecute runs the ChangePVCNodeSelectorAction's Execute
// method and validates that the item's PVC is modified (or not) as expected.
// Validation is done by comparing the result of the Execute method to the test case's
// desired result.
func TestChangePVCNodeSelectorActionExecute(t *testing.T) {
	tests := []struct {
		name      string
		pvc       *corev1api.PersistentVolumeClaim
		configMap *corev1api.ConfigMap
		node      *corev1api.Node
		newNode   *corev1api.Node
		want      *corev1api.PersistentVolumeClaim
		wantErr   error
	}{
		{
			name: "a valid mapping for a persistent volume claim is applied correctly",
			pvc: builder.ForPersistentVolumeClaim("source-ns", "pvc-1").
				ObjectMeta(
					builder.WithAnnotations("volume.kubernetes.io/selected-node", "source-node"),
				).Result(),
			configMap: builder.ForConfigMap("velero", "change-pvc-node").
				ObjectMeta(builder.WithLabels("velero.io/plugin-config", "true", "velero.io/change-pvc-node-selector", "RestoreItemAction")).
				Data("source-node", "dest-node").
				Result(),
			newNode: builder.ForNode("dest-node").Result(),
			want: builder.ForPersistentVolumeClaim("source-ns", "pvc-1").
				ObjectMeta(
					builder.WithAnnotations("volume.kubernetes.io/selected-node", "dest-node"),
				).Result(),
		},
		{
			name: "when no config map exists for the plugin and node doesn't exist, the item is returned without node selector",
			pvc: builder.ForPersistentVolumeClaim("source-ns", "pvc-1").
				ObjectMeta(
					builder.WithAnnotations("volume.kubernetes.io/selected-node", "source-node"),
				).Result(),
			configMap: builder.ForConfigMap("velero", "change-pvc-node").
				ObjectMeta(builder.WithLabels("velero.io/plugin-config", "true", "velero.io/some-other-plugin", "RestoreItemAction")).
				Data("source-noed", "dest-node").
				Result(),
			want: builder.ForPersistentVolumeClaim("source-ns", "pvc-1").Result(),
		},
		{
			name: "when no node-mappings exist in the plugin config map and selected-node doesn't exist, the item is returned without node selector",
			pvc: builder.ForPersistentVolumeClaim("source-ns", "pvc-1").
				ObjectMeta(
					builder.WithAnnotations("volume.kubernetes.io/selected-node", "source-node"),
				).Result(),
			configMap: builder.ForConfigMap("velero", "change-pvc-node").
				ObjectMeta(builder.WithLabels("velero.io/plugin-config", "true", "velero.io/change-pvc-node-selector", "RestoreItemAction")).
				Result(),
			want: builder.ForPersistentVolumeClaim("source-ns", "pvc-1").Result(),
		},
		{
			name: "when no node-mappings exist in the plugin config map and selected-node exist, the item is returned as-is",
			pvc: builder.ForPersistentVolumeClaim("source-ns", "pvc-1").
				ObjectMeta(
					builder.WithAnnotations("volume.kubernetes.io/selected-node", "source-node"),
				).Result(),
			configMap: builder.ForConfigMap("velero", "change-pvc-node").
				ObjectMeta(builder.WithLabels("velero.io/plugin-config", "true", "velero.io/change-pvc-node-selector", "RestoreItemAction")).
				Result(),
			// MAYANK TODO
			node: builder.ForNode("source-node").Result(),
			want: builder.ForPersistentVolumeClaim("source-ns", "pvc-1").
				ObjectMeta(
					builder.WithAnnotations("volume.kubernetes.io/selected-node", "source-node"),
				).Result(),
		},
		{
			name: "when persistent volume claim has no node selector, the item is returned as-is",
			pvc:  builder.ForPersistentVolumeClaim("source-ns", "pvc-1").Result(),
			configMap: builder.ForConfigMap("velero", "change-pvc-node").
				ObjectMeta(builder.WithLabels("velero.io/plugin-config", "true", "velero.io/change-pvc-node-selector", "RestoreItemAction")).
				Data("source-node", "dest-node").
				Result(),
			want: builder.ForPersistentVolumeClaim("source-ns", "pvc-1").Result(),
		},
		{
			name: "when persistent volume claim's node-selector has no mapping in the config map, the item is returned without node selector",
			pvc: builder.ForPersistentVolumeClaim("source-ns", "pvc-1").
				ObjectMeta(
					builder.WithAnnotations("volume.kubernetes.io/selected-node", "source-node"),
				).Result(),
			configMap: builder.ForConfigMap("velero", "change-pvc-node").
				ObjectMeta(builder.WithLabels("velero.io/plugin-config", "true", "velero.io/change-pvc-node-selector", "RestoreItemAction")).
				Data("source-node-1", "dest-node").
				Result(),
			want: builder.ForPersistentVolumeClaim("source-ns", "pvc-1").Result(),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			clientset := fake.NewSimpleClientset()
			logger := logrus.StandardLogger()
			buf := bytes.Buffer{}
			logrus.SetOutput(&buf)
			a := NewChangePVCNodeSelectorAction(
				logger,
				clientset.CoreV1().ConfigMaps("velero"),
				clientset.CoreV1().Nodes(),
			)

			// set up test data
			if tc.configMap != nil {
				_, err := clientset.CoreV1().ConfigMaps(tc.configMap.Namespace).Create(context.TODO(), tc.configMap, metav1.CreateOptions{})
				require.NoError(t, err)
			}

			if tc.node != nil {
				_, err := clientset.CoreV1().Nodes().Create(context.TODO(), tc.node, metav1.CreateOptions{})
				require.NoError(t, err)
			}
			if tc.newNode != nil {
				_, err := clientset.CoreV1().Nodes().Create(context.TODO(), tc.newNode, metav1.CreateOptions{})
				require.NoError(t, err)
			}
			unstructuredMap, err := runtime.DefaultUnstructuredConverter.ToUnstructured(tc.pvc)
			require.NoError(t, err)

			input := &velero.RestoreItemActionExecuteInput{
				Item: &unstructured.Unstructured{
					Object: unstructuredMap,
				},
			}

			// execute method under test
			res, err := a.Execute(input)

			// Make sure mapped selected-node exists.
			log_output := buf.String()
			assert.Equal(t, strings.Contains(log_output, "Selected-node's mapped node doesn't exist"), false)

			// validate for both error and non-error cases
			switch {
			case tc.wantErr != nil:
				assert.EqualError(t, err, tc.wantErr.Error())
			default:
				fmt.Printf("got +%v\n", res.UpdatedItem)
				assert.NoError(t, err)

				wantUnstructured, err := runtime.DefaultUnstructuredConverter.ToUnstructured(tc.want)
				fmt.Printf("expected +%v\n", wantUnstructured)
				require.NoError(t, err)

				assert.Equal(t, &unstructured.Unstructured{Object: wantUnstructured}, res.UpdatedItem)
			}
		})
	}
}
