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
	corev1 "k8s.io/api/core/v1"
)

func TestDaemonSet(t *testing.T) {
	ds := DaemonSet("velero")

	assert.Equal(t, "restic", ds.Spec.Template.Spec.Containers[0].Name)
	assert.Equal(t, "velero", ds.ObjectMeta.Namespace)

	ds = DaemonSet("velero", WithoutCredentialsVolume())
	assert.Equal(t, 2, len(ds.Spec.Template.Spec.Volumes))

	ds = DaemonSet("velero", WithImage("gcr.io/heptio-images/velero:v0.11"))
	assert.Equal(t, "gcr.io/heptio-images/velero:v0.11", ds.Spec.Template.Spec.Containers[0].Image)
	assert.Equal(t, corev1.PullIfNotPresent, ds.Spec.Template.Spec.Containers[0].ImagePullPolicy)
}
