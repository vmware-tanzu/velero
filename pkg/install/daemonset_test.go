/*
Copyright 2019, 2020 the Velero contributors.

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

package install

import (
	"testing"

	"github.com/stretchr/testify/assert"
	corev1api "k8s.io/api/core/v1"
)

func TestDaemonSet(t *testing.T) {
	userID := int64(0)
	boolFalse := false
	boolTrue := true

	ds := DaemonSet("velero")

	assert.Equal(t, "node-agent", ds.Spec.Template.Spec.Containers[0].Name)
	assert.Equal(t, "velero", ds.ObjectMeta.Namespace)
	assert.Equal(t, "node-agent", ds.Spec.Template.ObjectMeta.Labels["name"])
	assert.Equal(t, "node-agent", ds.Spec.Template.ObjectMeta.Labels["role"])
	assert.Equal(t, "linux", ds.Spec.Template.Spec.NodeSelector["kubernetes.io/os"])
	assert.Equal(t, "linux", string(ds.Spec.Template.Spec.OS.Name))
	assert.Equal(t, corev1api.PodSecurityContext{RunAsUser: &userID}, *ds.Spec.Template.Spec.SecurityContext)
	assert.Equal(t, corev1api.SecurityContext{Privileged: &boolFalse}, *ds.Spec.Template.Spec.Containers[0].SecurityContext)

	ds = DaemonSet("velero", WithPrivilegedNodeAgent(true))
	assert.Equal(t, corev1api.SecurityContext{Privileged: &boolTrue}, *ds.Spec.Template.Spec.Containers[0].SecurityContext)

	ds = DaemonSet("velero", WithImage("velero/velero:v0.11"))
	assert.Equal(t, "velero/velero:v0.11", ds.Spec.Template.Spec.Containers[0].Image)
	assert.Equal(t, corev1api.PullIfNotPresent, ds.Spec.Template.Spec.Containers[0].ImagePullPolicy)

	ds = DaemonSet("velero", WithSecret(true))
	assert.Len(t, ds.Spec.Template.Spec.Containers[0].Env, 7)
	assert.Len(t, ds.Spec.Template.Spec.Volumes, 4)

	ds = DaemonSet("velero", WithFeatures([]string{"foo,bar,baz"}))
	assert.Len(t, ds.Spec.Template.Spec.Containers[0].Args, 3)
	assert.Equal(t, "--features=foo,bar,baz", ds.Spec.Template.Spec.Containers[0].Args[2])

	ds = DaemonSet("velero", WithNodeAgentConfigMap("node-agent-config-map"))
	assert.Len(t, ds.Spec.Template.Spec.Containers[0].Args, 3)
	assert.Equal(t, "--node-agent-configmap=node-agent-config-map", ds.Spec.Template.Spec.Containers[0].Args[2])

	ds = DaemonSet("velero", WithServiceAccountName("test-sa"))
	assert.Equal(t, "test-sa", ds.Spec.Template.Spec.ServiceAccountName)

	ds = DaemonSet("velero", WithForWindows())
	assert.Equal(t, "node-agent-windows", ds.Spec.Template.Spec.Containers[0].Name)
	assert.Equal(t, "velero", ds.ObjectMeta.Namespace)
	assert.Equal(t, "node-agent-windows", ds.Spec.Template.ObjectMeta.Labels["name"])
	assert.Equal(t, "node-agent", ds.Spec.Template.ObjectMeta.Labels["role"])
	assert.Equal(t, "windows", ds.Spec.Template.Spec.NodeSelector["kubernetes.io/os"])
	assert.Equal(t, "windows", string(ds.Spec.Template.Spec.OS.Name))
	assert.Equal(t, (*corev1api.PodSecurityContext)(nil), ds.Spec.Template.Spec.SecurityContext)
	assert.Equal(t, (*corev1api.SecurityContext)(nil), ds.Spec.Template.Spec.Containers[0].SecurityContext)
}
