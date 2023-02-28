/*
Copyright The Velero Contributors.

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
	"testing"

	"github.com/stretchr/testify/assert"
	coreV1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestResourceKey(t *testing.T) {
	namespace := &coreV1.Namespace{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Namespace",
		},
	}
	assert.Equal(t, "v1/Namespace", resourceKey(namespace))

	cr := &coreV1.Namespace{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "customized/v1",
			Kind:       "Cron",
		},
	}
	assert.Equal(t, "customized/v1/Cron", resourceKey(cr))
}

func TestRestoredResourceList(t *testing.T) {
	request := &Request{
		RestoredItems: map[itemKey]restoredItemStatus{
			{
				resource:  "v1/Namespace",
				namespace: "",
				name:      "default",
			}: {action: "created"},
			{
				resource:  "v1/ConfigMap",
				namespace: "default",
				name:      "cm",
			}: {action: "skipped"},
		},
	}

	expected := map[string][]string{
		"v1/ConfigMap": {"default/cm(skipped)"},
		"v1/Namespace": {"default(created)"},
	}

	assert.EqualValues(t, expected, request.RestoredResourceList())
}
