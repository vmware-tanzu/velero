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

package install

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestResources(t *testing.T) {
	bsl := BackupStorageLocation(DefaultVeleroNamespace, "test", "test", "", make(map[string]string), []byte("test"))

	assert.Equal(t, "velero", bsl.ObjectMeta.Namespace)
	assert.Equal(t, "test", bsl.Spec.Provider)
	assert.Equal(t, "test", bsl.Spec.StorageType.ObjectStorage.Bucket)
	assert.Equal(t, make(map[string]string), bsl.Spec.Config)
	assert.Equal(t, []byte("test"), bsl.Spec.ObjectStorage.CACert)

	vsl := VolumeSnapshotLocation(DefaultVeleroNamespace, "test", make(map[string]string))

	assert.Equal(t, "velero", vsl.ObjectMeta.Namespace)
	assert.Equal(t, "test", vsl.Spec.Provider)
	assert.Equal(t, make(map[string]string), vsl.Spec.Config)

	ns := Namespace("velero")

	assert.Equal(t, "velero", ns.Name)
	// For k8s version v1.25 and later, need to add the following labels to make
	// velero installation namespace has privileged version to work with
	// PSA(Pod Security Admission) and PSS(Pod Security Standards).
	assert.Equal(t, ns.Labels["pod-security.kubernetes.io/enforce"], "privileged")
	assert.Equal(t, ns.Labels["pod-security.kubernetes.io/enforce-version"], "latest")

	crb := ClusterRoleBinding(DefaultVeleroNamespace)
	// The CRB is a cluster-scoped resource
	assert.Equal(t, "", crb.ObjectMeta.Namespace)
	assert.Equal(t, "velero", crb.ObjectMeta.Name)
	assert.Equal(t, "velero", crb.Subjects[0].Namespace)

	customNamespaceCRB := ClusterRoleBinding("foo")
	// The CRB is a cluster-scoped resource
	assert.Equal(t, "", customNamespaceCRB.ObjectMeta.Namespace)
	assert.Equal(t, "velero-foo", customNamespaceCRB.ObjectMeta.Name)
	assert.Equal(t, "foo", customNamespaceCRB.Subjects[0].Namespace)

	sa := ServiceAccount(DefaultVeleroNamespace, map[string]string{"abcd": "cbd"})
	assert.Equal(t, "velero", sa.ObjectMeta.Namespace)
	assert.Equal(t, "cbd", sa.ObjectMeta.Annotations["abcd"])
}
