/*
Copyright 2017, 2019, 2020 the Velero contributors.

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

package backup

import (
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestSortCoreGroup(t *testing.T) {
	group := &metav1.APIResourceList{
		GroupVersion: "v1",
		APIResources: []metav1.APIResource{
			{Name: "persistentvolumes"},
			{Name: "configmaps"},
			{Name: "antelopes"},
			{Name: "persistentvolumeclaims"},
			{Name: "pods"},
		},
	}

	sortCoreGroup(group)

	expected := []string{
		"pods",
		"persistentvolumeclaims",
		"persistentvolumes",
		"configmaps",
		"antelopes",
	}
	for i, r := range group.APIResources {
		assert.Equal(t, expected[i], r.Name)
	}
}

func TestSortOrderedResource(t *testing.T) {
	log := logrus.StandardLogger()
	podResources := []*kubernetesResource{
		{namespace: "ns1", name: "pod1"},
		{namespace: "ns1", name: "pod2"},
	}
	order := []string{"ns1/pod2", "ns1/pod1"}
	expectedResources := []*kubernetesResource{
		{namespace: "ns1", name: "pod2"},
		{namespace: "ns1", name: "pod1"},
	}
	sortedResources := sortResourcesByOrder(log, podResources, order)
	assert.Equal(t, sortedResources, expectedResources)

	// Test cluster resources
	pvResources := []*kubernetesResource{
		{name: "pv1"},
		{name: "pv2"},
	}
	pvOrder := []string{"pv5", "pv2", "pv1"}
	expectedPvResources := []*kubernetesResource{
		{name: "pv2"},
		{name: "pv1"},
	}
	sortedPvResources := sortResourcesByOrder(log, pvResources, pvOrder)
	assert.Equal(t, sortedPvResources, expectedPvResources)

}
