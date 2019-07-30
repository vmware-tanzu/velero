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
	"fmt"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"

	api "github.com/heptio/velero/pkg/apis/velero/v1"
	"github.com/heptio/velero/pkg/buildinfo"
	"github.com/heptio/velero/pkg/plugin/velero"
	"github.com/heptio/velero/pkg/test"
	"github.com/heptio/velero/pkg/util/kube"
	velerotest "github.com/heptio/velero/pkg/util/test"
)

func TestGetImage(t *testing.T) {
	configMapWithData := func(key, val string) *corev1.ConfigMap {
		return &corev1.ConfigMap{
			Data: map[string]string{
				key: val,
			},
		}
	}

	originalVersion := buildinfo.Version
	buildinfo.Version = "buildinfo-version"
	defer func() {
		buildinfo.Version = originalVersion
	}()

	tests := []struct {
		name      string
		configMap *corev1.ConfigMap
		want      string
	}{
		{
			name:      "nil config map returns default image with buildinfo.Version as tag",
			configMap: nil,
			want:      fmt.Sprintf("%s:%s", defaultImageBase, buildinfo.Version),
		},
		{
			name:      "config map without 'image' key returns default image with buildinfo.Version as tag",
			configMap: configMapWithData("non-matching-key", "val"),
			want:      fmt.Sprintf("%s:%s", defaultImageBase, buildinfo.Version),
		},
		{
			name:      "config map with invalid data in 'image' key returns default image with buildinfo.Version as tag",
			configMap: configMapWithData("image", "not:valid:image"),
			want:      fmt.Sprintf("%s:%s", defaultImageBase, buildinfo.Version),
		},
		{
			name:      "config map with untagged image returns image with buildinfo.Version as tag",
			configMap: configMapWithData("image", "myregistry.io/my-image"),
			want:      fmt.Sprintf("%s:%s", "myregistry.io/my-image", buildinfo.Version),
		},
		{
			name:      "config map with tagged image returns tagged image",
			configMap: configMapWithData("image", "myregistry.io/my-image:my-tag"),
			want:      "myregistry.io/my-image:my-tag",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.want, getImage(velerotest.NewLogger(), test.configMap))
		})
	}
}

func defaultResticInitContainer() *corev1.Container {
	container := newResticInitContainer(initContainerImage(defaultImageBase), "")
	resourceReqs, _ := kube.ParseResourceRequirements(
		defaultCPURequestLimit, defaultMemRequestLimit, // requests
		defaultCPURequestLimit, defaultMemRequestLimit, // limits
	)

	container.Resources = resourceReqs
	return container
}

// TestResticRestoreActionExecute tests the restic restore item action plugin's Execute method.
func TestResticRestoreActionExecute(t *testing.T) {
	// Need to add the ConfigMap testing
	tests := []struct {
		name string
		pod  *corev1.Pod
		want *corev1.Pod
	}{
		{
			name: "Restoring pod with no other initContainers adds the restic initContainer",
			pod: test.NewPod("ns-1", "pod",
				test.WithAnnotations("snapshot.velero.io/myvol", ""),
			),
			want: test.NewPod("ns-1", "pod",
				test.WithAnnotations("snapshot.velero.io/myvol", ""),
				test.WithInitContainer(defaultResticInitContainer(),
					test.WithVolumeMounts(test.NewVolumeMount("myvol"))),
			),
		},
		{
			name: "Restoring pod with other initContainers adds the restic initContainer as the first one",
			pod: test.NewPod("ns-1", "pod",
				test.WithAnnotations("snapshot.velero.io/myvol", ""),
				test.WithInitContainer(test.NewContainer("first-container")),
			),
			want: test.NewPod("ns-1", "pod",
				test.WithAnnotations("snapshot.velero.io/myvol", ""),
				test.WithInitContainer(defaultResticInitContainer(),
					test.WithVolumeMounts(test.NewVolumeMount("myvol"))),
				test.WithInitContainer(test.NewContainer("first-container")),
			),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			unstructuredMap, err := runtime.DefaultUnstructuredConverter.ToUnstructured(tc.pod)
			require.NoError(t, err)

			input := &velero.RestoreItemActionExecuteInput{
				Item: &unstructured.Unstructured{
					Object: unstructuredMap,
				},
				Restore: velerotest.NewTestRestore("my-restore", "velero", api.RestorePhaseInProgress).Restore,
			}

			clientset := fake.NewSimpleClientset()
			a := NewResticRestoreAction(
				logrus.StandardLogger(),
				clientset.CoreV1().ConfigMaps("velero"),
			)

			// method under test
			res, err := a.Execute(input)

			assert.NoError(t, err)

			wantUnstructured, err := runtime.DefaultUnstructuredConverter.ToUnstructured(tc.want)
			require.NoError(t, err)

			assert.Equal(t, &unstructured.Unstructured{Object: wantUnstructured}, res.UpdatedItem)
		})
	}

}
