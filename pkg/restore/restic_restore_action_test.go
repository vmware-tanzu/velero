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

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"

	"github.com/heptio/velero/pkg/buildinfo"
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
