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

package kube

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/fake"

	corev1 "k8s.io/api/core/v1"
)

func TestEvent(t *testing.T) {
	client := fake.NewSimpleClientset()

	scheme := runtime.NewScheme()
	err := corev1.AddToScheme(scheme)
	require.NoError(t, err)

	recorder := NewEventRecorder(client, scheme, "source-1", "fake-node")

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "fake-ns",
			Name:      "fake-pod",
			UID:       types.UID("fake-uid"),
		},
		Spec: corev1.PodSpec{
			NodeName: "fake-node",
		},
	}

	recorder.Event(pod, false, "Progress", "progress-1")
	recorder.Event(pod, false, "Progress", "progress-2")

	recorder.Shutdown()

	items, err := client.CoreV1().Events("fake-ns").List(context.Background(), metav1.ListOptions{})
	require.NoError(t, err)

	assert.Len(t, items.Items, 1)
}
