/*
Copyright 2022 the Velero contributors.

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
	corev1api "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/vmware-tanzu/velero/pkg/builder"
	"github.com/vmware-tanzu/velero/pkg/plugin/velero"
)

func TestChaneImageTagActionExecute(t *testing.T) {
	const defaultNamespace = "default"
	tests := []struct {
		name                 string
		configMap            *corev1api.ConfigMap
		podOrDeployOrStsOrDs interface{}
		want                 interface{}
		wantErr              error
	}{
		{
			name: "a valid registry mapping for a pod is applied correctly",
			configMap: builder.ForConfigMap("velero", "change-image-registry").
				ObjectMeta(builder.WithLabels("velero.io/plugin-config", "true", "velero.io/change-image-registry", "RestoreItemAction")).
				ObjectMeta(builder.WithAnnotations("velero.io/public", "new-registry.com/new-project")).
				Result(),
			podOrDeployOrStsOrDs: builder.ForPodWithImage(defaultNamespace, "my-pod", "velero.io/public/nginx:stable", "velero.io/public/tomcat:stable").Result(),
			want:                 builder.ForPodWithImage(defaultNamespace, "my-pod", "new-registry.com/new-project/nginx:stable", "new-registry.com/new-project/tomcat:stable").Result(),
		},
		{
			name: "a valid registry mapping for a pod is applied correctly, but last character of Registry is entered with an ’/’ ",
			configMap: builder.ForConfigMap("velero", "change-image-registry").
				ObjectMeta(builder.WithLabels("velero.io/plugin-config", "true", "velero.io/change-image-registry", "RestoreItemAction")).
				ObjectMeta(builder.WithAnnotations("velero.io/public/", "new-registry.com/new-project/")).
				Result(),
			podOrDeployOrStsOrDs: builder.ForPodWithImage(defaultNamespace, "my-pod", "velero.io/public/nginx:stable").Result(),
			want:                 builder.ForPodWithImage(defaultNamespace, "my-pod", "new-registry.com/new-project/nginx:stable").Result(),
		},
		{
			name: "when pod's image registry has no mapping in the config map, the item is returned as-is",
			configMap: builder.ForConfigMap("velero", "change-image-registry").
				ObjectMeta(builder.WithLabels("velero.io/plugin-config", "true", "velero.io/change-image-registry", "RestoreItemAction")).
				ObjectMeta(builder.WithAnnotations("none-registry.com/public", "velero.io/new-project")).
				Result(),
			podOrDeployOrStsOrDs: builder.ForPodWithImage(defaultNamespace, "my-pod", "velero.io/public/nginx:stable").Result(),
			want:                 builder.ForPodWithImage(defaultNamespace, "my-pod", "velero.io/public/nginx:stable").Result(),
		},
		{
			name: "when no config map exists for the plugin, the pod item is returned as-is",
			configMap: builder.ForConfigMap("velero", "change-image-registry").
				ObjectMeta(builder.WithLabels("velero.io/plugin-config", "true", "velero.io/some-other-plugin", "RestoreItemAction")).
				ObjectMeta(builder.WithAnnotations("velero.io/public", "velero.io/new-project")).
				Result(),
			podOrDeployOrStsOrDs: builder.ForPodWithImage(defaultNamespace, "my-pod", "velero.io/public/nginx:stable").Result(),
			want:                 builder.ForPodWithImage(defaultNamespace, "my-pod", "velero.io/public/nginx:stable").Result(),
		},
		{
			name: "a valid registry mapping for a pod is applied correctly when original registry is 'none'",
			configMap: builder.ForConfigMap("velero", "change-image-registry").
				ObjectMeta(builder.WithLabels("velero.io/plugin-config", "true", "velero.io/change-image-registry", "RestoreItemAction")).
				ObjectMeta(builder.WithAnnotations("none", "new-registry.com/new-project")).
				Result(),
			podOrDeployOrStsOrDs: builder.ForPodWithImage(defaultNamespace, "my-pod", "nginx:stable").Result(),
			want:                 builder.ForPodWithImage(defaultNamespace, "my-pod", "new-registry.com/new-project/nginx:stable").Result(),
		},
		{
			name: "set 'none' original registry mapping for a pod, but pod's original registry not 'none', the item is returned as-is",
			configMap: builder.ForConfigMap("velero", "change-image-registry").
				ObjectMeta(builder.WithLabels("velero.io/plugin-config", "true", "velero.io/change-image-registry", "RestoreItemAction")).
				ObjectMeta(builder.WithAnnotations("none", "new-registry.com/new-project")).
				Result(),
			podOrDeployOrStsOrDs: builder.ForPodWithImage(defaultNamespace, "my-pod", "velero.io/public/nginx:stable").Result(),
			want:                 builder.ForPodWithImage(defaultNamespace, "my-pod", "velero.io/public/nginx:stable").Result(),
		},
		{
			name: "a valid registry mapping for a deploy is applied correctly",
			configMap: builder.ForConfigMap("velero", "change-image-registry").
				ObjectMeta(builder.WithLabels("velero.io/plugin-config", "true", "velero.io/change-image-registry", "RestoreItemAction")).
				ObjectMeta(builder.WithAnnotations("velero.io/public", "new-registry.com/new-project", "velero.io/web", "new-registry.com/web")).
				Result(),
			podOrDeployOrStsOrDs: builder.ForDeploymentWithImage(defaultNamespace, "my-deploy", "velero.io/public/nginx:stable", "velero.io/web/tomcat:stable").Result(),
			want:                 builder.ForDeploymentWithImage(defaultNamespace, "my-deploy", "new-registry.com/new-project/nginx:stable", "new-registry.com/web/tomcat:stable").Result(),
		},
		{
			name: "when no config map exists for the plugin, the deployment item is returned as-is",
			configMap: builder.ForConfigMap("velero", "change-image-registry").
				ObjectMeta(builder.WithLabels("velero.io/plugin-config", "true", "velero.io/some-other-plugin", "RestoreItemAction")).
				ObjectMeta(builder.WithAnnotations("velero.io/public", "new-registry.com/new-project/")).
				Result(),
			podOrDeployOrStsOrDs: builder.ForDeploymentWithImage(defaultNamespace, "my-deploy", "velero.io/public/nginx:stable").Result(),
			want:                 builder.ForDeploymentWithImage(defaultNamespace, "my-deploy", "velero.io/public/nginx:stable").Result(),
		},
		{
			name: "when deployment's image registry has no mapping in the config map, the item is returned as-is",
			configMap: builder.ForConfigMap("velero", "change-image-registry").
				ObjectMeta(builder.WithLabels("velero.io/plugin-config", "true", "velero.io/change-image-registry", "RestoreItemAction")).
				ObjectMeta(builder.WithAnnotations("none-registry.com/public", "new-registry.com/new-project")).
				Result(),
			podOrDeployOrStsOrDs: builder.ForDeploymentWithImage(defaultNamespace, "my-deploy", "velero.io/public/nginx:stable").Result(),
			want:                 builder.ForDeploymentWithImage(defaultNamespace, "my-deploy", "velero.io/public/nginx:stable").Result(),
		},
		{
			name: "a valid registry mapping for a deployment is applied correctly when original registry is 'none'",
			configMap: builder.ForConfigMap("velero", "change-image-registry").
				ObjectMeta(builder.WithLabels("velero.io/plugin-config", "true", "velero.io/change-image-registry", "RestoreItemAction")).
				ObjectMeta(builder.WithAnnotations("none", "new-registry.com/new-project")).
				Result(),
			podOrDeployOrStsOrDs: builder.ForDeploymentWithImage(defaultNamespace, "my-deploy", "nginx:stable").Result(),
			want:                 builder.ForDeploymentWithImage(defaultNamespace, "my-deploy", "new-registry.com/new-project/nginx:stable").Result(),
		},
		{
			name: "set 'none' original registry mapping for a deployment, but deployment's original registry not 'none', the item is returned as-is",
			configMap: builder.ForConfigMap("velero", "change-image-registry").
				ObjectMeta(builder.WithLabels("velero.io/plugin-config", "true", "velero.io/change-image-registry", "RestoreItemAction")).
				ObjectMeta(builder.WithAnnotations("none", "new-registry.com/new-project")).
				Result(),
			podOrDeployOrStsOrDs: builder.ForDeploymentWithImage(defaultNamespace, "my-deploy", "velero.io/public/nginx:stable").Result(),
			want:                 builder.ForDeploymentWithImage(defaultNamespace, "my-deploy", "velero.io/public/nginx:stable").Result(),
		},
		{
			name: "a valid registry mapping for a replicaset is applied correctly",
			configMap: builder.ForConfigMap("velero", "change-image-registry").
				ObjectMeta(builder.WithLabels("velero.io/plugin-config", "true", "velero.io/change-image-registry", "RestoreItemAction")).
				ObjectMeta(builder.WithAnnotations("velero.io/public", "new-registry.com/new-project", "velero.io/web", "new-registry.com/web")).
				Result(),
			podOrDeployOrStsOrDs: builder.ForReplicaSetWithImage(defaultNamespace, "my-replicaset", "velero.io/public/nginx:stable", "velero.io/web/tomcat:stable").Result(),
			want:                 builder.ForReplicaSetWithImage(defaultNamespace, "my-replicaset", "new-registry.com/new-project/nginx:stable", "new-registry.com/web/tomcat:stable").Result(),
		},
		{
			name: "when no config map exists for the plugin, the replicaset item is returned as-is",
			configMap: builder.ForConfigMap("velero", "change-image-registry").
				ObjectMeta(builder.WithLabels("velero.io/plugin-config", "true", "velero.io/some-other-plugin", "RestoreItemAction")).
				ObjectMeta(builder.WithAnnotations("velero.io/public", "new-registry.com/new-project")).
				Result(),
			podOrDeployOrStsOrDs: builder.ForReplicaSetWithImage(defaultNamespace, "my-replicaset", "velero.io/public/nginx:stable", "velero.io/web/tomcat:stable").Result(),
			want:                 builder.ForReplicaSetWithImage(defaultNamespace, "my-replicaset", "velero.io/public/nginx:stable", "velero.io/web/tomcat:stable").Result(),
		},
		{
			name: "when replicaset's image registry has no mapping in the config map, the item is returned as-is",
			configMap: builder.ForConfigMap("velero", "change-image-registry").
				ObjectMeta(builder.WithLabels("velero.io/plugin-config", "true", "velero.io/change-image-registry", "RestoreItemAction")).
				ObjectMeta(builder.WithAnnotations("none-registry.com/public", "new-registry.com/new-project")).
				Result(),
			podOrDeployOrStsOrDs: builder.ForReplicaSetWithImage(defaultNamespace, "my-replicaset", "velero.io/public/nginx:stable", "velero.io/web/tomcat:stable").Result(),
			want:                 builder.ForReplicaSetWithImage(defaultNamespace, "my-replicaset", "velero.io/public/nginx:stable", "velero.io/web/tomcat:stable").Result(),
		},
		{
			name: "a valid registry mapping for a replicaset is applied correctly when original registry is 'none'",
			configMap: builder.ForConfigMap("velero", "change-image-registry").
				ObjectMeta(builder.WithLabels("velero.io/plugin-config", "true", "velero.io/change-image-registry", "RestoreItemAction")).
				ObjectMeta(builder.WithAnnotations("none", "new-registry.com/new-project")).
				Result(),
			podOrDeployOrStsOrDs: builder.ForReplicaSetWithImage(defaultNamespace, "my-replicaset", "nginx:stable", "tomcat:stable").Result(),
			want:                 builder.ForReplicaSetWithImage(defaultNamespace, "my-replicaset", "new-registry.com/new-project/nginx:stable", "new-registry.com/new-project/tomcat:stable").Result(),
		},
		{
			name: "set 'none' original registry mapping for a replicaset, but replicaset's original registry not 'none', the item is returned as-is",
			configMap: builder.ForConfigMap("velero", "change-image-registry").
				ObjectMeta(builder.WithLabels("velero.io/plugin-config", "true", "velero.io/change-image-registry", "RestoreItemAction")).
				ObjectMeta(builder.WithAnnotations("none", "new-registry.com/new-project")).
				Result(),
			podOrDeployOrStsOrDs: builder.ForReplicaSetWithImage(defaultNamespace, "my-replicaset", "velero.io/public/nginx:stable", "velero.io/public/tomcat:stable").Result(),
			want:                 builder.ForReplicaSetWithImage(defaultNamespace, "my-replicaset", "velero.io/public/nginx:stable", "velero.io/public/tomcat:stable").Result(),
		},
		{
			name: "a valid registry mapping for a statefulset is applied correctly",
			configMap: builder.ForConfigMap("velero", "change-image-registry").
				ObjectMeta(builder.WithLabels("velero.io/plugin-config", "true", "velero.io/change-image-registry", "RestoreItemAction")).
				ObjectMeta(builder.WithAnnotations("velero.io/public", "new-registry.com/new-project", "velero.io/web", "new-registry.com/web")).
				Result(),
			podOrDeployOrStsOrDs: builder.ForStatefulSetWithImage(defaultNamespace, "my-sts", "velero.io/public/nginx:stable", "velero.io/web/tomcat:stable").Result(),
			want:                 builder.ForStatefulSetWithImage(defaultNamespace, "my-sts", "new-registry.com/new-project/nginx:stable", "new-registry.com/web/tomcat:stable").Result(),
		},
		{
			name: "when no config map exists for the plugin, the statefulset item is returned as-is",
			configMap: builder.ForConfigMap("velero", "change-image-registry").
				ObjectMeta(builder.WithLabels("velero.io/plugin-config", "true", "velero.io/some-other-plugin", "RestoreItemAction")).
				ObjectMeta(builder.WithAnnotations("velero.io/public", "new-registry.com/new-project")).
				Result(),
			podOrDeployOrStsOrDs: builder.ForStatefulSetWithImage(defaultNamespace, "my-sts", "velero.io/public/nginx:stable").Result(),
			want:                 builder.ForStatefulSetWithImage(defaultNamespace, "my-sts", "velero.io/public/nginx:stable").Result(),
		},
		{
			name: "when statefulset's image registry has no mapping in the config map, the item is returned as-is",
			configMap: builder.ForConfigMap("velero", "change-image-registry").
				ObjectMeta(builder.WithLabels("velero.io/plugin-config", "true", "velero.io/change-image-registry", "RestoreItemAction")).
				ObjectMeta(builder.WithAnnotations("none-registry.com/public", "velero.io/new-project")).
				Result(),
			podOrDeployOrStsOrDs: builder.ForStatefulSetWithImage(defaultNamespace, "my-sts", "velero.io/public/nginx:stable").Result(),
			want:                 builder.ForStatefulSetWithImage(defaultNamespace, "my-sts", "velero.io/public/nginx:stable").Result(),
		},
		{
			name: "a valid registry mapping for a statefulset is applied correctly when original registry is 'none'",
			configMap: builder.ForConfigMap("velero", "change-image-registry").
				ObjectMeta(builder.WithLabels("velero.io/plugin-config", "true", "velero.io/change-image-registry", "RestoreItemAction")).
				ObjectMeta(builder.WithAnnotations("none", "new-registry.com/new-project")).
				Result(),
			podOrDeployOrStsOrDs: builder.ForStatefulSetWithImage(defaultNamespace, "my-sts", "nginx:stable").Result(),
			want:                 builder.ForStatefulSetWithImage(defaultNamespace, "my-sts", "new-registry.com/new-project/nginx:stable").Result(),
		},
		{
			name: "set 'none' original registry mapping for a statefulset, but statefulset's original registry not 'none', the item is returned as-is",
			configMap: builder.ForConfigMap("velero", "change-image-registry").
				ObjectMeta(builder.WithLabels("velero.io/plugin-config", "true", "velero.io/change-image-registry", "RestoreItemAction")).
				ObjectMeta(builder.WithAnnotations("none", "new-registry.com/new-project")).
				Result(),
			podOrDeployOrStsOrDs: builder.ForStatefulSetWithImage(defaultNamespace, "my-sts", "velero.io/public/nginx:stable").Result(),
			want:                 builder.ForStatefulSetWithImage(defaultNamespace, "my-sts", "velero.io/public/nginx:stable").Result(),
		},
		{
			name: "a valid registry mapping for a daemonset is applied correctly",
			configMap: builder.ForConfigMap("velero", "change-image-registry").
				ObjectMeta(builder.WithLabels("velero.io/plugin-config", "true", "velero.io/change-image-registry", "RestoreItemAction")).
				ObjectMeta(builder.WithAnnotations("velero.io/public", "new-registry.com/new-project")).
				Result(),
			podOrDeployOrStsOrDs: builder.ForDaemonsetWithImage(defaultNamespace, "my-ds", "velero.io/public/nginx:stable").Result(),
			want:                 builder.ForDaemonsetWithImage(defaultNamespace, "my-ds", "new-registry.com/new-project/nginx:stable").Result(),
		},
		{
			name: "when no config map exists for the plugin, the daemonset item is returned as-is",
			configMap: builder.ForConfigMap("velero", "change-image-registry").
				ObjectMeta(builder.WithLabels("velero.io/plugin-config", "true", "velero.io/some-other-plugin", "RestoreItemAction")).
				ObjectMeta(builder.WithAnnotations("velero.io/public", "new-registry.com/new-project")).
				Result(),
			podOrDeployOrStsOrDs: builder.ForDaemonsetWithImage(defaultNamespace, "my-ds", "velero.io/public/nginx:stable").Result(),
			want:                 builder.ForDaemonsetWithImage(defaultNamespace, "my-ds", "velero.io/public/nginx:stable").Result(),
		},
		{
			name: "when daemonset's image registry has no mapping in the config map, the item is returned as-is",
			configMap: builder.ForConfigMap("velero", "change-image-registry").
				ObjectMeta(builder.WithLabels("velero.io/plugin-config", "true", "velero.io/change-image-registry", "RestoreItemAction")).
				ObjectMeta(builder.WithAnnotations("none-registry.com/public", "velero.io/new-project")).
				Result(),
			podOrDeployOrStsOrDs: builder.ForDaemonsetWithImage(defaultNamespace, "my-ds", "velero.io/public/nginx:stable").Result(),
			want:                 builder.ForDaemonsetWithImage(defaultNamespace, "my-ds", "velero.io/public/nginx:stable").Result(),
		},
		{
			name: "a valid registry mapping for a daemonset is applied correctly when original registry is 'none'",
			configMap: builder.ForConfigMap("velero", "change-image-registry").
				ObjectMeta(builder.WithLabels("velero.io/plugin-config", "true", "velero.io/change-image-registry", "RestoreItemAction")).
				ObjectMeta(builder.WithAnnotations("none", "new-registry.com/new-project")).
				Result(),
			podOrDeployOrStsOrDs: builder.ForDaemonsetWithImage(defaultNamespace, "my-ds", "nginx:stable").Result(),
			want:                 builder.ForDaemonsetWithImage(defaultNamespace, "my-ds", "new-registry.com/new-project/nginx:stable").Result(),
		},
		{
			name: "set 'none' original registry mapping for a daemonset, but daemonset's original registry not 'none', the item is returned as-is",
			configMap: builder.ForConfigMap("velero", "change-image-registry").
				ObjectMeta(builder.WithLabels("velero.io/plugin-config", "true", "velero.io/change-image-registry", "RestoreItemAction")).
				ObjectMeta(builder.WithAnnotations("none", "new-registry.com/new-project")).
				Result(),
			podOrDeployOrStsOrDs: builder.ForDaemonsetWithImage(defaultNamespace, "my-ds", "velero.io/public/nginx:stable").Result(),
			want:                 builder.ForDaemonsetWithImage(defaultNamespace, "my-ds", "velero.io/public/nginx:stable").Result(),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			clientset := fake.NewSimpleClientset()
			a := NewChangeImageRegistryAction(
				logrus.StandardLogger(),
				clientset.CoreV1().ConfigMaps("velero"),
			)

			// set up test data
			if tc.configMap != nil {
				_, err := clientset.CoreV1().ConfigMaps(tc.configMap.Namespace).Create(context.TODO(), tc.configMap, metav1.CreateOptions{})
				require.NoError(t, err)
			}

			unstructuredMap, err := runtime.DefaultUnstructuredConverter.ToUnstructured(tc.podOrDeployOrStsOrDs)
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

				wantUnstructured, err := runtime.DefaultUnstructuredConverter.ToUnstructured(tc.want)
				require.NoError(t, err)

				assert.Equal(t, &unstructured.Unstructured{Object: wantUnstructured}, res.UpdatedItem)
			}
		})
	}
}
