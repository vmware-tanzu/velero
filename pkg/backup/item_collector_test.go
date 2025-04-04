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
	"os"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	corev1api "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"

	velerov1api "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	"github.com/vmware-tanzu/velero/pkg/builder"
	"github.com/vmware-tanzu/velero/pkg/kuberesource"
	"github.com/vmware-tanzu/velero/pkg/test"
	"github.com/vmware-tanzu/velero/pkg/util/collections"
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
		{namespace: "ns1", name: "pod3"},
		{namespace: "ns1", name: "pod1"},
		{namespace: "ns1", name: "pod2"},
	}
	order := []string{"ns1/pod2", "ns1/pod1"}
	expectedResources := []*kubernetesResource{
		{namespace: "ns1", name: "pod2", orderedResource: true},
		{namespace: "ns1", name: "pod1", orderedResource: true},
		{namespace: "ns1", name: "pod3"},
	}
	sortedResources := sortResourcesByOrder(log, podResources, order)
	assert.Equal(t, expectedResources, sortedResources)

	// Test cluster resources
	pvResources := []*kubernetesResource{
		{name: "pv1"},
		{name: "pv2"},
		{name: "pv3"},
	}
	pvOrder := []string{"pv5", "pv2", "pv1"}
	expectedPvResources := []*kubernetesResource{
		{name: "pv2", orderedResource: true},
		{name: "pv1", orderedResource: true},
		{name: "pv3"},
	}
	sortedPvResources := sortResourcesByOrder(log, pvResources, pvOrder)
	assert.Equal(t, expectedPvResources, sortedPvResources)
}

func TestFilterNamespaces(t *testing.T) {
	tests := []struct {
		name              string
		resources         []*kubernetesResource
		needToTrack       string
		expectedResources []*kubernetesResource
	}{
		{
			name: "Namespace include by the filter but not in namespacesContainResource",
			resources: []*kubernetesResource{
				{
					groupResource: kuberesource.Namespaces,
					preferredGVR:  kuberesource.Namespaces.WithVersion("v1"),
					name:          "ns1",
				},
				{
					groupResource: kuberesource.Namespaces,
					preferredGVR:  kuberesource.Namespaces.WithVersion("v1"),
					name:          "ns2",
				},
				{
					groupResource: kuberesource.Pods,
					preferredGVR:  kuberesource.Namespaces.WithVersion("v1"),
					name:          "pod1",
				},
			},
			needToTrack: "ns1",
			expectedResources: []*kubernetesResource{
				{
					groupResource: kuberesource.Namespaces,
					preferredGVR:  kuberesource.Namespaces.WithVersion("v1"),
					name:          "ns1",
				},
				{
					groupResource: kuberesource.Pods,
					preferredGVR:  kuberesource.Namespaces.WithVersion("v1"),
					name:          "pod1",
				},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(*testing.T) {
			r := itemCollector{
				backupRequest: &Request{},
			}

			if tc.needToTrack != "" {
				r.nsTracker.track(tc.needToTrack)
			}

			require.Equal(t, tc.expectedResources, r.nsTracker.filterNamespaces(tc.resources))
		})
	}
}

func TestItemCollectorBackupNamespaces(t *testing.T) {
	tests := []struct {
		name              string
		ie                *collections.IncludesExcludes
		namespaces        []*corev1api.Namespace
		backup            *velerov1api.Backup
		expectedTrackedNS []string
	}{
		{
			name:   "ns filter by namespace IE filter",
			backup: builder.ForBackup("velero", "backup").Result(),
			ie:     collections.NewIncludesExcludes().Includes("ns1"),
			namespaces: []*corev1api.Namespace{
				builder.ForNamespace("ns1").Result(),
				builder.ForNamespace("ns2").Result(),
			},
			expectedTrackedNS: []string{"ns1"},
		},
		{
			name: "ns filter by backup labelSelector",
			backup: builder.ForBackup("velero", "backup").LabelSelector(&metav1.LabelSelector{
				MatchLabels: map[string]string{"name": "ns1"},
			}).Result(),
			ie: collections.NewIncludesExcludes().Includes("*"),
			namespaces: []*corev1api.Namespace{
				builder.ForNamespace("ns1").ObjectMeta(builder.WithLabels("name", "ns1")).Result(),
				builder.ForNamespace("ns2").Result(),
			},
			expectedTrackedNS: []string{"ns1"},
		},
		{
			name: "ns filter by backup orLabelSelector",
			backup: builder.ForBackup("velero", "backup").OrLabelSelector([]*metav1.LabelSelector{
				{MatchLabels: map[string]string{"name": "ns1"}},
			}).Result(),
			ie: collections.NewIncludesExcludes().Includes("*"),
			namespaces: []*corev1api.Namespace{
				builder.ForNamespace("ns1").ObjectMeta(builder.WithLabels("name", "ns1")).Result(),
				builder.ForNamespace("ns2").Result(),
			},
			expectedTrackedNS: []string{"ns1"},
		},
		{
			name: "ns not included by IE filter, but included by labelSelector",
			backup: builder.ForBackup("velero", "backup").LabelSelector(&metav1.LabelSelector{
				MatchLabels: map[string]string{"name": "ns1"},
			}).Result(),
			ie: collections.NewIncludesExcludes().Excludes("ns1"),
			namespaces: []*corev1api.Namespace{
				builder.ForNamespace("ns1").ObjectMeta(builder.WithLabels("name", "ns1")).Result(),
				builder.ForNamespace("ns2").Result(),
			},
			expectedTrackedNS: []string{"ns1"},
		},
		{
			name: "ns not included by IE filter, but included by orLabelSelector",
			backup: builder.ForBackup("velero", "backup").OrLabelSelector([]*metav1.LabelSelector{
				{MatchLabels: map[string]string{"name": "ns1"}},
			}).Result(),
			ie: collections.NewIncludesExcludes().Excludes("ns1", "ns2"),
			namespaces: []*corev1api.Namespace{
				builder.ForNamespace("ns1").ObjectMeta(builder.WithLabels("name", "ns1")).Result(),
				builder.ForNamespace("ns2").Result(),
				builder.ForNamespace("ns3").Result(),
			},
			expectedTrackedNS: []string{"ns1", "ns3"},
		},
		{
			name:   "No ns filters",
			backup: builder.ForBackup("velero", "backup").Result(),
			ie:     collections.NewIncludesExcludes().Includes("*"),
			namespaces: []*corev1api.Namespace{
				builder.ForNamespace("ns1").ObjectMeta(builder.WithLabels("name", "ns1")).Result(),
				builder.ForNamespace("ns2").Result(),
			},
			expectedTrackedNS: []string{"ns1", "ns2"},
		},
		{
			name:   "ns specified by the IncludeNamespaces cannot be found",
			backup: builder.ForBackup("velero", "backup").IncludedNamespaces("ns1", "invalid", "*").Result(),
			ie:     collections.NewIncludesExcludes().Includes("ns1", "invalid", "*"),
			namespaces: []*corev1api.Namespace{
				builder.ForNamespace("ns1").ObjectMeta(builder.WithLabels("name", "ns1")).Result(),
				builder.ForNamespace("ns2").Result(),
				builder.ForNamespace("ns3").Result(),
			},
			expectedTrackedNS: []string{"ns1"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(*testing.T) {
			tempDir, err := os.MkdirTemp("", "")
			require.NoError(t, err)

			var unstructuredNSList unstructured.UnstructuredList
			for _, ns := range tc.namespaces {
				unstructuredNS, err := runtime.DefaultUnstructuredConverter.ToUnstructured(ns)
				require.NoError(t, err)
				unstructuredNSList.Items = append(unstructuredNSList.Items,
					unstructured.Unstructured{Object: unstructuredNS})
			}

			dc := &test.FakeDynamicClient{}
			dc.On("List", mock.Anything).Return(&unstructuredNSList, nil)

			factory := &test.FakeDynamicFactory{}
			factory.On(
				"ClientForGroupVersionResource",
				mock.Anything,
				mock.Anything,
				mock.Anything,
			).Return(dc, nil)

			r := itemCollector{
				backupRequest: &Request{
					Backup:                    tc.backup,
					NamespaceIncludesExcludes: tc.ie,
				},
				dynamicFactory: factory,
				dir:            tempDir,
			}

			r.collectNamespaces(
				metav1.APIResource{
					Name:       "Namespace",
					Kind:       "Namespace",
					Namespaced: false,
				},
				kuberesource.Namespaces.WithVersion("").GroupVersion(),
				kuberesource.Namespaces,
				kuberesource.Namespaces.WithVersion(""),
				logrus.StandardLogger(),
			)

			for _, ns := range tc.expectedTrackedNS {
				require.True(t, r.nsTracker.isTracked(ns))
			}
		})
	}
}
