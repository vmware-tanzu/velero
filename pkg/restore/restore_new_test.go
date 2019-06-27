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
	"archive/tar"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"testing"
	"time"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1api "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/dynamic"
	kubetesting "k8s.io/client-go/testing"

	velerov1api "github.com/heptio/velero/pkg/apis/velero/v1"
	"github.com/heptio/velero/pkg/backup"
	"github.com/heptio/velero/pkg/client"
	"github.com/heptio/velero/pkg/discovery"
	"github.com/heptio/velero/pkg/kuberesource"
	"github.com/heptio/velero/pkg/plugin/velero"
	"github.com/heptio/velero/pkg/test"
	"github.com/heptio/velero/pkg/util/encode"
	kubeutil "github.com/heptio/velero/pkg/util/kube"
	testutil "github.com/heptio/velero/pkg/util/test"
)

// TestRestoreResourceFiltering runs restores with different combinations
// of resource filters (included/excluded resources, included/excluded
// namespaces, label selectors, "include cluster resources" flag), and
// verifies that the set of items created in the API are correct.
// Validation is done by looking at the namespaces/names of the items in
// the API; contents are not checked.
func TestRestoreResourceFiltering(t *testing.T) {
	tests := []struct {
		name         string
		restore      *velerov1api.Restore
		backup       *velerov1api.Backup
		apiResources []*test.APIResource
		tarball      io.Reader
		want         map[*test.APIResource][]string
	}{
		{
			name:    "no filters restores everything",
			restore: defaultRestore().Restore(),
			backup:  defaultBackup().Backup(),
			tarball: newTarWriter(t).
				addItems("pods",
					test.NewPod("ns-1", "pod-1"),
					test.NewPod("ns-2", "pod-2"),
				).
				addItems("persistentvolumes",
					test.NewPV("pv-1"),
					test.NewPV("pv-2"),
				).
				done(),
			apiResources: []*test.APIResource{
				test.Pods(),
				test.PVs(),
			},
			want: map[*test.APIResource][]string{
				test.Pods(): {"ns-1/pod-1", "ns-2/pod-2"},
				test.PVs():  {"/pv-1", "/pv-2"},
			},
		},
		{
			name:    "included resources filter only restores resources of those types",
			restore: defaultRestore().IncludedResources("pods").Restore(),
			backup:  defaultBackup().Backup(),
			tarball: newTarWriter(t).
				addItems("pods",
					test.NewPod("ns-1", "pod-1"),
					test.NewPod("ns-2", "pod-2"),
				).
				addItems("persistentvolumes",
					test.NewPV("pv-1"),
					test.NewPV("pv-2"),
				).
				done(),
			apiResources: []*test.APIResource{
				test.Pods(),
				test.PVs(),
			},
			want: map[*test.APIResource][]string{
				test.Pods(): {"ns-1/pod-1", "ns-2/pod-2"},
			},
		},
		{
			name:    "excluded resources filter only restores resources not of those types",
			restore: defaultRestore().ExcludedResources("pvs").Restore(),
			backup:  defaultBackup().Backup(),
			tarball: newTarWriter(t).
				addItems("pods",
					test.NewPod("ns-1", "pod-1"),
					test.NewPod("ns-2", "pod-2"),
				).
				addItems("persistentvolumes",
					test.NewPV("pv-1"),
					test.NewPV("pv-2"),
				).
				done(),
			apiResources: []*test.APIResource{
				test.Pods(),
				test.PVs(),
			},
			want: map[*test.APIResource][]string{
				test.Pods(): {"ns-1/pod-1", "ns-2/pod-2"},
			},
		},
		{
			name:    "included namespaces filter only restores resources in those namespaces",
			restore: defaultRestore().IncludedNamespaces("ns-1").Restore(),
			backup:  defaultBackup().Backup(),
			tarball: newTarWriter(t).
				addItems("pods",
					test.NewPod("ns-1", "pod-1"),
					test.NewPod("ns-2", "pod-2"),
				).
				addItems("deployments.apps",
					test.NewDeployment("ns-1", "deploy-1"),
					test.NewDeployment("ns-2", "deploy-2"),
				).
				addItems("persistentvolumes",
					test.NewPV("pv-1"),
					test.NewPV("pv-2"),
				).
				done(),
			apiResources: []*test.APIResource{
				test.Pods(),
				test.Deployments(),
				test.PVs(),
			},
			want: map[*test.APIResource][]string{
				test.Pods():        {"ns-1/pod-1"},
				test.Deployments(): {"ns-1/deploy-1"},
			},
		},
		{
			name:    "excluded namespaces filter only restores resources not in those namespaces",
			restore: defaultRestore().ExcludedNamespaces("ns-2").Restore(),
			backup:  defaultBackup().Backup(),
			tarball: newTarWriter(t).
				addItems("pods",
					test.NewPod("ns-1", "pod-1"),
					test.NewPod("ns-2", "pod-2"),
				).
				addItems("deployments.apps",
					test.NewDeployment("ns-1", "deploy-1"),
					test.NewDeployment("ns-2", "deploy-2"),
				).
				addItems("persistentvolumes",
					test.NewPV("pv-1"),
					test.NewPV("pv-2"),
				).
				done(),
			apiResources: []*test.APIResource{
				test.Pods(),
				test.Deployments(),
				test.PVs(),
			},
			want: map[*test.APIResource][]string{
				test.Pods():        {"ns-1/pod-1"},
				test.Deployments(): {"ns-1/deploy-1"},
			},
		},
		{
			name:    "IncludeClusterResources=false only restores namespaced resources",
			restore: defaultRestore().IncludeClusterResources(false).Restore(),
			backup:  defaultBackup().Backup(),
			tarball: newTarWriter(t).
				addItems("pods",
					test.NewPod("ns-1", "pod-1"),
					test.NewPod("ns-2", "pod-2"),
				).
				addItems("deployments.apps",
					test.NewDeployment("ns-1", "deploy-1"),
					test.NewDeployment("ns-2", "deploy-2"),
				).
				addItems("persistentvolumes",
					test.NewPV("pv-1"),
					test.NewPV("pv-2"),
				).
				done(),
			apiResources: []*test.APIResource{
				test.Pods(),
				test.Deployments(),
				test.PVs(),
			},
			want: map[*test.APIResource][]string{
				test.Pods():        {"ns-1/pod-1", "ns-2/pod-2"},
				test.Deployments(): {"ns-1/deploy-1", "ns-2/deploy-2"},
			},
		},
		{
			name:    "label selector only restores matching resources",
			restore: defaultRestore().LabelSelector(&metav1.LabelSelector{MatchLabels: map[string]string{"a": "b"}}).Restore(),
			backup:  defaultBackup().Backup(),
			tarball: newTarWriter(t).
				addItems("pods",
					test.NewPod("ns-1", "pod-1", test.WithLabels("a", "b")),
					test.NewPod("ns-2", "pod-2"),
				).
				addItems("deployments.apps",
					test.NewDeployment("ns-1", "deploy-1"),
					test.NewDeployment("ns-2", "deploy-2", test.WithLabels("a", "b")),
				).
				addItems("persistentvolumes",
					test.NewPV("pv-1", test.WithLabels("a", "b")),
					test.NewPV("pv-2", test.WithLabels("a", "c")),
				).
				done(),
			apiResources: []*test.APIResource{
				test.Pods(),
				test.Deployments(),
				test.PVs(),
			},
			want: map[*test.APIResource][]string{
				test.Pods():        {"ns-1/pod-1"},
				test.Deployments(): {"ns-2/deploy-2"},
				test.PVs():         {"/pv-1"},
			},
		},
		{
			name:    "should include cluster-scoped resources if restoring subset of namespaces and IncludeClusterResources=true",
			restore: defaultRestore().IncludedNamespaces("ns-1").IncludeClusterResources(true).Restore(),
			backup:  defaultBackup().Backup(),
			tarball: newTarWriter(t).
				addItems("pods",
					test.NewPod("ns-1", "pod-1"),
					test.NewPod("ns-2", "pod-2"),
				).
				addItems("deployments.apps",
					test.NewDeployment("ns-1", "deploy-1"),
					test.NewDeployment("ns-2", "deploy-2"),
				).
				addItems("persistentvolumes",
					test.NewPV("pv-1"),
					test.NewPV("pv-2"),
				).
				done(),
			apiResources: []*test.APIResource{
				test.Pods(),
				test.Deployments(),
				test.PVs(),
			},
			want: map[*test.APIResource][]string{
				test.Pods():        {"ns-1/pod-1"},
				test.Deployments(): {"ns-1/deploy-1"},
				test.PVs():         {"/pv-1", "/pv-2"},
			},
		},
		{
			name:    "should not include cluster-scoped resources if restoring subset of namespaces and IncludeClusterResources=false",
			restore: defaultRestore().IncludedNamespaces("ns-1").IncludeClusterResources(false).Restore(),
			backup:  defaultBackup().Backup(),
			tarball: newTarWriter(t).
				addItems("pods",
					test.NewPod("ns-1", "pod-1"),
					test.NewPod("ns-2", "pod-2"),
				).
				addItems("deployments.apps",
					test.NewDeployment("ns-1", "deploy-1"),
					test.NewDeployment("ns-2", "deploy-2"),
				).
				addItems("persistentvolumes",
					test.NewPV("pv-1"),
					test.NewPV("pv-2"),
				).
				done(),
			apiResources: []*test.APIResource{
				test.Pods(),
				test.Deployments(),
				test.PVs(),
			},
			want: map[*test.APIResource][]string{
				test.Pods():        {"ns-1/pod-1"},
				test.Deployments(): {"ns-1/deploy-1"},
			},
		},
		{
			name:    "should include cluster-scoped resources if restoring all namespaces and IncludeClusterResources=true",
			restore: defaultRestore().IncludeClusterResources(true).Restore(),
			backup:  defaultBackup().Backup(),
			tarball: newTarWriter(t).
				addItems("pods",
					test.NewPod("ns-1", "pod-1"),
					test.NewPod("ns-2", "pod-2"),
				).
				addItems("deployments.apps",
					test.NewDeployment("ns-1", "deploy-1"),
					test.NewDeployment("ns-2", "deploy-2"),
				).
				addItems("persistentvolumes",
					test.NewPV("pv-1"),
					test.NewPV("pv-2"),
				).
				done(),
			apiResources: []*test.APIResource{
				test.Pods(),
				test.Deployments(),
				test.PVs(),
			},
			want: map[*test.APIResource][]string{
				test.Pods():        {"ns-1/pod-1", "ns-2/pod-2"},
				test.Deployments(): {"ns-1/deploy-1", "ns-2/deploy-2"},
				test.PVs():         {"/pv-1", "/pv-2"},
			},
		},
		{
			name:    "should not include cluster-scoped resources if restoring all namespaces and IncludeClusterResources=false",
			restore: defaultRestore().IncludeClusterResources(false).Restore(),
			backup:  defaultBackup().Backup(),
			tarball: newTarWriter(t).
				addItems("pods",
					test.NewPod("ns-1", "pod-1"),
					test.NewPod("ns-2", "pod-2"),
				).
				addItems("deployments.apps",
					test.NewDeployment("ns-1", "deploy-1"),
					test.NewDeployment("ns-2", "deploy-2"),
				).
				addItems("persistentvolumes",
					test.NewPV("pv-1"),
					test.NewPV("pv-2"),
				).
				done(),
			apiResources: []*test.APIResource{
				test.Pods(),
				test.Deployments(),
				test.PVs(),
			},
			want: map[*test.APIResource][]string{
				test.Pods():        {"ns-1/pod-1", "ns-2/pod-2"},
				test.Deployments(): {"ns-1/deploy-1", "ns-2/deploy-2"},
			},
		},
		{
			name:    "when a wildcard and a specific resource are included, the wildcard takes precedence",
			restore: defaultRestore().IncludedResources("*", "pods").Restore(),
			backup:  defaultBackup().Backup(),
			tarball: newTarWriter(t).
				addItems("pods",
					test.NewPod("ns-1", "pod-1"),
					test.NewPod("ns-2", "pod-2"),
				).
				addItems("deployments.apps",
					test.NewDeployment("ns-1", "deploy-1"),
					test.NewDeployment("ns-2", "deploy-2"),
				).
				addItems("persistentvolumes",
					test.NewPV("pv-1"),
					test.NewPV("pv-2"),
				).
				done(),
			apiResources: []*test.APIResource{
				test.Pods(),
				test.Deployments(),
				test.PVs(),
			},
			want: map[*test.APIResource][]string{
				test.Pods():        {"ns-1/pod-1", "ns-2/pod-2"},
				test.Deployments(): {"ns-1/deploy-1", "ns-2/deploy-2"},
				test.PVs():         {"/pv-1", "/pv-2"},
			},
		},
		{
			name:    "wildcard excludes are ignored",
			restore: defaultRestore().ExcludedResources("*").Restore(),
			backup:  defaultBackup().Backup(),
			tarball: newTarWriter(t).
				addItems("pods",
					test.NewPod("ns-1", "pod-1"),
					test.NewPod("ns-2", "pod-2"),
				).
				addItems("deployments.apps",
					test.NewDeployment("ns-1", "deploy-1"),
					test.NewDeployment("ns-2", "deploy-2"),
				).
				addItems("persistentvolumes",
					test.NewPV("pv-1"),
					test.NewPV("pv-2"),
				).
				done(),
			apiResources: []*test.APIResource{
				test.Pods(),
				test.Deployments(),
				test.PVs(),
			},
			want: map[*test.APIResource][]string{
				test.Pods():        {"ns-1/pod-1", "ns-2/pod-2"},
				test.Deployments(): {"ns-1/deploy-1", "ns-2/deploy-2"},
				test.PVs():         {"/pv-1", "/pv-2"},
			},
		},
		{
			name:    "unresolvable included resources are ignored",
			restore: defaultRestore().IncludedResources("pods", "unresolvable").Restore(),
			backup:  defaultBackup().Backup(),
			tarball: newTarWriter(t).
				addItems("pods",
					test.NewPod("ns-1", "pod-1"),
					test.NewPod("ns-2", "pod-2"),
				).
				addItems("deployments.apps",
					test.NewDeployment("ns-1", "deploy-1"),
					test.NewDeployment("ns-2", "deploy-2"),
				).
				addItems("persistentvolumes",
					test.NewPV("pv-1"),
					test.NewPV("pv-2"),
				).
				done(),
			apiResources: []*test.APIResource{
				test.Pods(),
				test.Deployments(),
				test.PVs(),
			},
			want: map[*test.APIResource][]string{
				test.Pods(): {"ns-1/pod-1", "ns-2/pod-2"},
			},
		},
		{
			name:    "unresolvable excluded resources are ignored",
			restore: defaultRestore().ExcludedResources("deployments", "unresolvable").Restore(),
			backup:  defaultBackup().Backup(),
			tarball: newTarWriter(t).
				addItems("pods",
					test.NewPod("ns-1", "pod-1"),
					test.NewPod("ns-2", "pod-2"),
				).
				addItems("deployments.apps",
					test.NewDeployment("ns-1", "deploy-1"),
					test.NewDeployment("ns-2", "deploy-2"),
				).
				addItems("persistentvolumes",
					test.NewPV("pv-1"),
					test.NewPV("pv-2"),
				).
				done(),
			apiResources: []*test.APIResource{
				test.Pods(),
				test.Deployments(),
				test.PVs(),
			},
			want: map[*test.APIResource][]string{
				test.Pods(): {"ns-1/pod-1", "ns-2/pod-2"},
				test.PVs():  {"/pv-1", "/pv-2"},
			},
		},
		{
			name:         "mirror pods are not restored",
			restore:      defaultRestore().Restore(),
			backup:       defaultBackup().Backup(),
			tarball:      newTarWriter(t).addItems("pods", test.NewPod("ns-1", "pod-1", test.WithAnnotations(corev1api.MirrorPodAnnotationKey, "foo"))).done(),
			apiResources: []*test.APIResource{test.Pods()},
			want:         map[*test.APIResource][]string{test.Pods(): {}},
		},
		{
			name:         "service accounts are restored",
			restore:      defaultRestore().Restore(),
			backup:       defaultBackup().Backup(),
			tarball:      newTarWriter(t).addItems("serviceaccounts", test.NewServiceAccount("ns-1", "sa-1")).done(),
			apiResources: []*test.APIResource{test.ServiceAccounts()},
			want:         map[*test.APIResource][]string{test.ServiceAccounts(): {"ns-1/sa-1"}},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			h := newHarness(t)

			for _, r := range tc.apiResources {
				h.DiscoveryClient.WithAPIResource(r)
			}
			require.NoError(t, h.restorer.discoveryHelper.Refresh())

			warnings, errs := h.restorer.Restore(
				h.log,
				tc.restore,
				tc.backup,
				nil, // volume snapshots
				tc.tarball,
				nil, // actions
				nil, // snapshot location lister
				nil, // volume snapshotter getter
			)

			assertEmptyResults(t, warnings, errs)
			assertAPIContents(t, h, tc.want)
		})
	}
}

// TestRestoreNamespaceMapping runs restores with namespace mappings specified,
// and verifies that the set of items created in the API are in the correct
// namespaces. Validation is done by looking at the namespaces/names of the items
// in the API; contents are not checked.
func TestRestoreNamespaceMapping(t *testing.T) {
	tests := []struct {
		name         string
		restore      *velerov1api.Restore
		backup       *velerov1api.Backup
		apiResources []*test.APIResource
		tarball      io.Reader
		want         map[*test.APIResource][]string
	}{
		{
			name:    "namespace mappings are applied",
			restore: defaultRestore().NamespaceMappings("ns-1", "mapped-ns-1", "ns-2", "mapped-ns-2").Restore(),
			backup:  defaultBackup().Backup(),
			apiResources: []*test.APIResource{
				test.Pods(),
			},
			tarball: newTarWriter(t).
				addItems("pods",
					test.NewPod("ns-1", "pod-1"),
					test.NewPod("ns-2", "pod-2"),
					test.NewPod("ns-3", "pod-3"),
				).
				done(),
			want: map[*test.APIResource][]string{
				test.Pods(): {"mapped-ns-1/pod-1", "mapped-ns-2/pod-2", "ns-3/pod-3"},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			h := newHarness(t)

			for _, r := range tc.apiResources {
				h.DiscoveryClient.WithAPIResource(r)
			}
			require.NoError(t, h.restorer.discoveryHelper.Refresh())

			warnings, errs := h.restorer.Restore(
				h.log,
				tc.restore,
				tc.backup,
				nil, // volume snapshots
				tc.tarball,
				nil, // actions
				nil, // snapshot location lister
				nil, // volume snapshotter getter
			)

			assertEmptyResults(t, warnings, errs)
			assertAPIContents(t, h, tc.want)
		})
	}
}

// TestRestoreResourcePriorities runs restores with resource priorities specified,
// and verifies that the set of items created in the API are created in the expected
// order. Validation is done by adding a Reactor to the fake dynamic client that records
// resource identifiers as they're created, and comparing that to the expected order.
func TestRestoreResourcePriorities(t *testing.T) {
	tests := []struct {
		name               string
		restore            *velerov1api.Restore
		backup             *velerov1api.Backup
		apiResources       []*test.APIResource
		tarball            io.Reader
		resourcePriorities []string
	}{
		{
			name:    "resources are restored according to the specified resource priorities",
			restore: defaultRestore().Restore(),
			backup:  defaultBackup().Backup(),
			tarball: newTarWriter(t).
				addItems("pods",
					test.NewPod("ns-1", "pod-1"),
					test.NewPod("ns-2", "pod-2"),
				).
				addItems("persistentvolumes",
					test.NewPV("pv-1"),
					test.NewPV("pv-2"),
				).
				addItems("deployments.apps",
					test.NewDeployment("ns-1", "deploy-1"),
					test.NewDeployment("ns-2", "deploy-2"),
				).
				addItems("serviceaccounts",
					test.NewServiceAccount("ns-1", "sa-1"),
					test.NewServiceAccount("ns-2", "sa-2"),
				).
				addItems("persistentvolumeclaims",
					test.NewPVC("ns-1", "pvc-1"),
					test.NewPVC("ns-2", "pvc-2"),
				).
				done(),
			apiResources: []*test.APIResource{
				test.Pods(),
				test.PVs(),
				test.Deployments(),
				test.ServiceAccounts(),
			},
			resourcePriorities: []string{"persistentvolumes", "serviceaccounts", "pods", "deployments.apps"},
		},
	}

	for _, tc := range tests {
		h := newHarness(t)
		h.restorer.resourcePriorities = tc.resourcePriorities

		recorder := &createRecorder{t: t}
		h.DynamicClient.PrependReactor("create", "*", recorder.reactor())

		for _, r := range tc.apiResources {
			h.DiscoveryClient.WithAPIResource(r)
		}
		require.NoError(t, h.restorer.discoveryHelper.Refresh())

		warnings, errs := h.restorer.Restore(
			h.log,
			tc.restore,
			tc.backup,
			nil, // volume snapshots
			tc.tarball,
			nil, // actions
			nil, // snapshot location lister
			nil, // volume snapshotter getter
		)

		assertEmptyResults(t, warnings, errs)
		assertResourceCreationOrder(t, tc.resourcePriorities, recorder.resources)
	}
}

// TestInvalidTarballContents runs restores for tarballs that are invalid in some way, and
// verifies that the set of items created in the API and the errors returned are correct.
// Validation is done by looking at the namespaces/names of the items in the API and the
// Result objects returned from the restorer.
func TestInvalidTarballContents(t *testing.T) {
	tests := []struct {
		name         string
		restore      *velerov1api.Restore
		backup       *velerov1api.Backup
		apiResources []*test.APIResource
		tarball      io.Reader
		want         map[*test.APIResource][]string
		wantErrs     Result
	}{
		{
			name:    "empty tarball returns an error",
			restore: defaultRestore().Restore(),
			backup:  defaultBackup().Backup(),
			tarball: newTarWriter(t).
				done(),
			wantErrs: Result{
				Velero: []string{"backup does not contain top level resources directory"},
			},
		},
		{
			name:    "invalid JSON is reported as an error and restore continues",
			restore: defaultRestore().Restore(),
			backup:  defaultBackup().Backup(),
			tarball: newTarWriter(t).
				add("resources/pods/namespaces/ns-1/pod-1.json", []byte("invalid JSON")).
				addItems("pods",
					test.NewPod("ns-1", "pod-2"),
				).
				done(),
			apiResources: []*test.APIResource{
				test.Pods(),
			},
			want: map[*test.APIResource][]string{
				test.Pods(): {"ns-1/pod-2"},
			},
			wantErrs: Result{
				Namespaces: map[string][]string{
					"ns-1": {"error decoding \"resources/pods/namespaces/ns-1/pod-1.json\": invalid character 'i' looking for beginning of value"},
				},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			h := newHarness(t)

			for _, r := range tc.apiResources {
				h.DiscoveryClient.WithAPIResource(r)
			}
			require.NoError(t, h.restorer.discoveryHelper.Refresh())

			warnings, errs := h.restorer.Restore(
				h.log,
				tc.restore,
				tc.backup,
				nil, // volume snapshots
				tc.tarball,
				nil, // actions
				nil, // snapshot location lister
				nil, // volume snapshotter getter
			)

			assertEmptyResults(t, warnings)
			assert.Equal(t, tc.wantErrs, errs)
			assertAPIContents(t, h, tc.want)
		})
	}
}

// TestRestoreItems runs restores of specific items and validates that they are created
// with the expected metadata/spec/status in the API.
func TestRestoreItems(t *testing.T) {
	tests := []struct {
		name         string
		restore      *velerov1api.Restore
		backup       *velerov1api.Backup
		apiResources []*test.APIResource
		tarball      io.Reader
		want         []*test.APIResource
	}{
		{
			name:    "metadata other than namespace/name/labels/annotations gets removed",
			restore: defaultRestore().Restore(),
			backup:  defaultBackup().Backup(),
			tarball: newTarWriter(t).
				addItems("pods",
					test.NewPod("ns-1", "pod-1",
						test.WithLabels("key-1", "val-1"),
						test.WithAnnotations("key-1", "val-1"),
						test.WithClusterName("cluster-1"),
						test.WithFinalizers("finalizer-1")),
				).
				done(),
			apiResources: []*test.APIResource{
				test.Pods(),
			},
			want: []*test.APIResource{
				test.Pods(
					test.NewPod("ns-1", "pod-1",
						test.WithLabels("key-1", "val-1", "velero.io/backup-name", "backup-1", "velero.io/restore-name", "restore-1"),
						test.WithAnnotations("key-1", "val-1"),
					),
				),
			},
		},
		{
			name:    "status gets removed",
			restore: defaultRestore().Restore(),
			backup:  defaultBackup().Backup(),
			tarball: newTarWriter(t).
				addItems("pods",
					&corev1api.Pod{
						TypeMeta: metav1.TypeMeta{
							APIVersion: "v1",
							Kind:       "Pod",
						},
						ObjectMeta: metav1.ObjectMeta{
							Namespace: "ns-1",
							Name:      "pod-1",
						},
						Status: corev1api.PodStatus{
							Message: "a non-empty status",
						},
					},
				).
				done(),
			apiResources: []*test.APIResource{
				test.Pods(),
			},
			want: []*test.APIResource{
				test.Pods(
					test.NewPod("ns-1", "pod-1", test.WithLabels("velero.io/backup-name", "backup-1", "velero.io/restore-name", "restore-1")),
				),
			},
		},
		{
			name:    "object gets labeled with full backup and restore names when they're both shorter than 63 characters",
			restore: defaultRestore().Restore(),
			backup:  defaultBackup().Backup(),
			tarball: newTarWriter(t).
				addItems("pods", test.NewPod("ns-1", "pod-1")).
				done(),
			apiResources: []*test.APIResource{
				test.Pods(),
			},
			want: []*test.APIResource{
				test.Pods(test.NewPod("ns-1", "pod-1", test.WithLabels("velero.io/backup-name", "backup-1", "velero.io/restore-name", "restore-1"))),
			},
		},
		{
			name: "object gets labeled with full backup and restore names when they're both equal to 63 characters",
			restore: NewNamedBuilder(velerov1api.DefaultNamespace, "the-really-long-kube-service-name-that-is-exactly-63-characters").
				Backup("the-really-long-kube-service-name-that-is-exactly-63-characters").
				Restore(),
			backup: backup.NewNamedBuilder(velerov1api.DefaultNamespace, "the-really-long-kube-service-name-that-is-exactly-63-characters").Backup(),
			tarball: newTarWriter(t).
				addItems("pods", test.NewPod("ns-1", "pod-1")).
				done(),
			apiResources: []*test.APIResource{
				test.Pods(),
			},
			want: []*test.APIResource{
				test.Pods(test.NewPod("ns-1", "pod-1", test.WithLabels(
					"velero.io/backup-name", "the-really-long-kube-service-name-that-is-exactly-63-characters",
					"velero.io/restore-name", "the-really-long-kube-service-name-that-is-exactly-63-characters",
				))),
			},
		},
		{
			name: "object gets labeled with shortened backup and restore names when they're both longer than 63 characters",
			restore: NewNamedBuilder(velerov1api.DefaultNamespace, "the-really-long-kube-service-name-that-is-much-greater-than-63-characters").
				Backup("the-really-long-kube-service-name-that-is-much-greater-than-63-characters").
				Restore(),
			backup: backup.NewNamedBuilder(velerov1api.DefaultNamespace, "the-really-long-kube-service-name-that-is-much-greater-than-63-characters").Backup(),
			tarball: newTarWriter(t).
				addItems("pods", test.NewPod("ns-1", "pod-1")).
				done(),
			apiResources: []*test.APIResource{
				test.Pods(),
			},
			want: []*test.APIResource{
				test.Pods(test.NewPod("ns-1", "pod-1", test.WithLabels(
					"velero.io/backup-name", "the-really-long-kube-service-name-that-is-much-greater-th8a11b3",
					"velero.io/restore-name", "the-really-long-kube-service-name-that-is-much-greater-th8a11b3",
				))),
			},
		},
		{
			name:    "no error when service account already exists in cluster and is identical to the backed up one",
			restore: defaultRestore().Restore(),
			backup:  defaultBackup().Backup(),
			tarball: newTarWriter(t).
				addItems("serviceaccounts", test.NewServiceAccount("ns-1", "sa-1")).
				done(),
			apiResources: []*test.APIResource{
				test.ServiceAccounts(test.NewServiceAccount("ns-1", "sa-1")),
			},
			want: []*test.APIResource{
				test.ServiceAccounts(test.NewServiceAccount("ns-1", "sa-1")),
			},
		},
		{
			name:    "service account secrets and image pull secrets are restored when service account already exists in cluster",
			restore: defaultRestore().Restore(),
			backup:  defaultBackup().Backup(),
			tarball: newTarWriter(t).
				addItems("serviceaccounts", &corev1api.ServiceAccount{
					TypeMeta: metav1.TypeMeta{
						APIVersion: "v1",
						Kind:       "ServiceAccount",
					},
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "ns-1",
						Name:      "sa-1",
					},
					Secrets:          []corev1api.ObjectReference{{Name: "secret-1"}},
					ImagePullSecrets: []corev1api.LocalObjectReference{{Name: "pull-secret-1"}},
				}).
				done(),
			apiResources: []*test.APIResource{
				test.ServiceAccounts(test.NewServiceAccount("ns-1", "sa-1")),
			},
			want: []*test.APIResource{
				test.ServiceAccounts(&corev1api.ServiceAccount{
					TypeMeta: metav1.TypeMeta{
						APIVersion: "v1",
						Kind:       "ServiceAccount",
					},
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "ns-1",
						Name:      "sa-1",
					},
					Secrets:          []corev1api.ObjectReference{{Name: "secret-1"}},
					ImagePullSecrets: []corev1api.LocalObjectReference{{Name: "pull-secret-1"}},
				}),
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			h := newHarness(t)

			for _, r := range tc.apiResources {
				h.addItems(t, r)
			}

			warnings, errs := h.restorer.Restore(
				h.log,
				tc.restore,
				tc.backup,
				nil, // volume snapshots
				tc.tarball,
				nil, // actions
				nil, // snapshot location lister
				nil, // volume snapshotter getter
			)

			assertEmptyResults(t, warnings, errs)
			assertRestoredItems(t, h, tc.want)
		})
	}
}

// recordResourcesAction is a restore item action that can be configured
// to run for specific resources/namespaces and simply records the items
// that it is executed for.
type recordResourcesAction struct {
	selector        velero.ResourceSelector
	ids             []string
	additionalItems []velero.ResourceIdentifier
}

func (a *recordResourcesAction) AppliesTo() (velero.ResourceSelector, error) {
	return a.selector, nil
}

func (a *recordResourcesAction) Execute(input *velero.RestoreItemActionExecuteInput) (*velero.RestoreItemActionExecuteOutput, error) {
	metadata, err := meta.Accessor(input.Item)
	if err != nil {
		return &velero.RestoreItemActionExecuteOutput{
			UpdatedItem:     input.Item,
			AdditionalItems: a.additionalItems,
		}, err
	}
	a.ids = append(a.ids, kubeutil.NamespaceAndName(metadata))

	return &velero.RestoreItemActionExecuteOutput{
		UpdatedItem:     input.Item,
		AdditionalItems: a.additionalItems,
	}, nil
}

func (a *recordResourcesAction) ForResource(resource string) *recordResourcesAction {
	a.selector.IncludedResources = append(a.selector.IncludedResources, resource)
	return a
}

func (a *recordResourcesAction) ForNamespace(namespace string) *recordResourcesAction {
	a.selector.IncludedNamespaces = append(a.selector.IncludedNamespaces, namespace)
	return a
}

func (a *recordResourcesAction) ForLabelSelector(selector string) *recordResourcesAction {
	a.selector.LabelSelector = selector
	return a
}

func (a *recordResourcesAction) WithAdditionalItems(items []velero.ResourceIdentifier) *recordResourcesAction {
	a.additionalItems = items
	return a
}

// TestRestoreActionsRunsForCorrectItems runs restores with restore item actions, and
// verifies that each restore item action is run for the correct set of resources based on its
// AppliesTo() resource selector. Verification is done by using the recordResourcesAction struct,
// which records which resources it's executed for.
func TestRestoreActionsRunForCorrectItems(t *testing.T) {
	tests := []struct {
		name         string
		restore      *velerov1api.Restore
		backup       *velerov1api.Backup
		apiResources []*test.APIResource
		tarball      io.Reader
		actions      map[*recordResourcesAction][]string
	}{
		{
			name:    "single action with no selector runs for all items",
			restore: defaultRestore().Restore(),
			backup:  defaultBackup().Backup(),
			tarball: newTarWriter(t).
				addItems("pods", test.NewPod("ns-1", "pod-1"), test.NewPod("ns-2", "pod-2")).
				addItems("persistentvolumes", test.NewPV("pv-1"), test.NewPV("pv-2")).
				done(),
			apiResources: []*test.APIResource{test.Pods(), test.PVs()},
			actions: map[*recordResourcesAction][]string{
				new(recordResourcesAction): {"ns-1/pod-1", "ns-2/pod-2", "pv-1", "pv-2"},
			},
		},
		{
			name:    "single action with a resource selector for namespaced resources runs only for matching resources",
			restore: defaultRestore().Restore(),
			backup:  defaultBackup().Backup(),
			tarball: newTarWriter(t).
				addItems("pods", test.NewPod("ns-1", "pod-1"), test.NewPod("ns-2", "pod-2")).
				addItems("persistentvolumes", test.NewPV("pv-1"), test.NewPV("pv-2")).
				done(),
			apiResources: []*test.APIResource{test.Pods(), test.PVs()},
			actions: map[*recordResourcesAction][]string{
				new(recordResourcesAction).ForResource("pods"): {"ns-1/pod-1", "ns-2/pod-2"},
			},
		},
		{
			name:    "single action with a resource selector for cluster-scoped resources runs only for matching resources",
			restore: defaultRestore().Restore(),
			backup:  defaultBackup().Backup(),
			tarball: newTarWriter(t).
				addItems("pods", test.NewPod("ns-1", "pod-1"), test.NewPod("ns-2", "pod-2")).
				addItems("persistentvolumes", test.NewPV("pv-1"), test.NewPV("pv-2")).
				done(),
			apiResources: []*test.APIResource{test.Pods(), test.PVs()},
			actions: map[*recordResourcesAction][]string{
				new(recordResourcesAction).ForResource("persistentvolumes"): {"pv-1", "pv-2"},
			},
		},
		{
			name:    "single action with a namespace selector runs only for resources in that namespace",
			restore: defaultRestore().Restore(),
			backup:  defaultBackup().Backup(),
			tarball: newTarWriter(t).
				addItems("pods", test.NewPod("ns-1", "pod-1"), test.NewPod("ns-2", "pod-2")).
				addItems("persistentvolumeclaims", test.NewPVC("ns-1", "pvc-1"), test.NewPVC("ns-2", "pvc-2")).
				addItems("persistentvolumes", test.NewPV("pv-1"), test.NewPV("pv-2")).
				done(),
			apiResources: []*test.APIResource{test.Pods(), test.PVCs(), test.PVs()},
			actions: map[*recordResourcesAction][]string{
				new(recordResourcesAction).ForNamespace("ns-1"): {"ns-1/pod-1", "ns-1/pvc-1"},
			},
		},
		{
			name:    "single action with a resource and namespace selector runs only for matching resources in that namespace",
			restore: defaultRestore().Restore(),
			backup:  defaultBackup().Backup(),
			tarball: newTarWriter(t).
				addItems("pods", test.NewPod("ns-1", "pod-1"), test.NewPod("ns-2", "pod-2")).
				addItems("persistentvolumeclaims", test.NewPVC("ns-1", "pvc-1"), test.NewPVC("ns-2", "pvc-2")).
				addItems("persistentvolumes", test.NewPV("pv-1"), test.NewPV("pv-2")).
				done(),
			apiResources: []*test.APIResource{test.Pods(), test.PVCs(), test.PVs()},
			actions: map[*recordResourcesAction][]string{
				new(recordResourcesAction).ForNamespace("ns-1").ForResource("pods"): {"ns-1/pod-1"},
			},
		},
		{
			name:    "multiple actions, each with a different resource selector using short name, run for matching resources",
			restore: defaultRestore().Restore(),
			backup:  defaultBackup().Backup(),
			tarball: newTarWriter(t).
				addItems("pods", test.NewPod("ns-1", "pod-1"), test.NewPod("ns-2", "pod-2")).
				addItems("persistentvolumeclaims", test.NewPVC("ns-1", "pvc-1"), test.NewPVC("ns-2", "pvc-2")).
				addItems("persistentvolumes", test.NewPV("pv-1"), test.NewPV("pv-2")).
				done(),
			apiResources: []*test.APIResource{test.Pods(), test.PVCs(), test.PVs()},
			actions: map[*recordResourcesAction][]string{
				new(recordResourcesAction).ForResource("po"): {"ns-1/pod-1", "ns-2/pod-2"},
				new(recordResourcesAction).ForResource("pv"): {"pv-1", "pv-2"},
			},
		},
		{
			name:    "actions with selectors that don't match anything don't run for any resources",
			restore: defaultRestore().Restore(),
			backup:  defaultBackup().Backup(),
			tarball: newTarWriter(t).
				addItems("pods", test.NewPod("ns-1", "pod-1")).
				addItems("persistentvolumeclaims", test.NewPVC("ns-2", "pvc-2")).
				done(),
			apiResources: []*test.APIResource{test.Pods(), test.PVCs(), test.PVs()},
			actions: map[*recordResourcesAction][]string{
				new(recordResourcesAction).ForNamespace("ns-1").ForResource("persistentvolumeclaims"): nil,
				new(recordResourcesAction).ForNamespace("ns-2").ForResource("pods"):                   nil,
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			h := newHarness(t)

			for _, r := range tc.apiResources {
				h.addItems(t, r)
			}

			actions := []velero.RestoreItemAction{}
			for action := range tc.actions {
				actions = append(actions, action)
			}

			warnings, errs := h.restorer.Restore(
				h.log,
				tc.restore,
				tc.backup,
				nil, // volume snapshots
				tc.tarball,
				actions,
				nil, // snapshot location lister
				nil, // volume snapshotter getter
			)

			assertEmptyResults(t, warnings, errs)

			for action, want := range tc.actions {
				sort.Strings(want)
				sort.Strings(action.ids)
				assert.Equal(t, want, action.ids)
			}
		})
	}
}

// pluggableAction is a restore item action that can be plugged with an Execute
// function body at runtime.
type pluggableAction struct {
	selector    velero.ResourceSelector
	executeFunc func(*velero.RestoreItemActionExecuteInput) (*velero.RestoreItemActionExecuteOutput, error)
}

func (a *pluggableAction) Execute(input *velero.RestoreItemActionExecuteInput) (*velero.RestoreItemActionExecuteOutput, error) {
	if a.executeFunc == nil {
		return &velero.RestoreItemActionExecuteOutput{
			UpdatedItem: input.Item,
		}, nil
	}

	return a.executeFunc(input)
}

func (a *pluggableAction) AppliesTo() (velero.ResourceSelector, error) {
	return a.selector, nil
}

// TestRestoreActionModifications runs restores with restore item actions that modify resources, and
// verifies that that the modified item is correctly created in the API. Verification is done by looking
// at the full object in the API.
func TestRestoreActionModifications(t *testing.T) {
	// modifyingActionGetter is a helper function that returns a *pluggableAction, whose Execute(...)
	// method modifies the item being passed in by calling the 'modify' function on it.
	modifyingActionGetter := func(modify func(*unstructured.Unstructured)) *pluggableAction {
		return &pluggableAction{
			executeFunc: func(input *velero.RestoreItemActionExecuteInput) (*velero.RestoreItemActionExecuteOutput, error) {
				obj, ok := input.Item.(*unstructured.Unstructured)
				if !ok {
					return nil, errors.Errorf("unexpected type %T", input.Item)
				}

				res := obj.DeepCopy()
				modify(res)

				return &velero.RestoreItemActionExecuteOutput{
					UpdatedItem: res,
				}, nil
			},
		}
	}

	tests := []struct {
		name         string
		restore      *velerov1api.Restore
		backup       *velerov1api.Backup
		apiResources []*test.APIResource
		tarball      io.Reader
		actions      []velero.RestoreItemAction
		want         []*test.APIResource
	}{
		{
			name:         "action that adds a label to item gets restored",
			restore:      defaultRestore().Restore(),
			backup:       defaultBackup().Backup(),
			tarball:      newTarWriter(t).addItems("pods", test.NewPod("ns-1", "pod-1")).done(),
			apiResources: []*test.APIResource{test.Pods()},
			actions: []velero.RestoreItemAction{
				modifyingActionGetter(func(item *unstructured.Unstructured) {
					item.SetLabels(map[string]string{"updated": "true"})
				}),
			},
			want: []*test.APIResource{
				test.Pods(
					test.NewPod("ns-1", "pod-1", test.WithLabels("updated", "true"))),
			},
		},
		{
			name:         "action that removes a label to item gets restored",
			restore:      defaultRestore().Restore(),
			backup:       defaultBackup().Backup(),
			tarball:      newTarWriter(t).addItems("pods", test.NewPod("ns-1", "pod-1", test.WithLabels("should-be-removed", "true"))).done(),
			apiResources: []*test.APIResource{test.Pods()},
			actions: []velero.RestoreItemAction{
				modifyingActionGetter(func(item *unstructured.Unstructured) {
					item.SetLabels(nil)
				}),
			},
			want: []*test.APIResource{
				test.Pods(
					test.NewPod("ns-1", "pod-1")),
			},
		},
		// TODO action that modifies namespace/name - what's the expected behavior?
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			h := newHarness(t)

			for _, r := range tc.apiResources {
				h.addItems(t, r)
			}

			// every restored item should have the restore and backup name labels, set
			// them here so we don't have to do it in every test case definition above.
			for _, resource := range tc.want {
				for _, item := range resource.Items {
					labels := item.GetLabels()
					if labels == nil {
						labels = make(map[string]string)
					}

					labels["velero.io/restore-name"] = tc.restore.Name
					labels["velero.io/backup-name"] = tc.restore.Spec.BackupName

					item.SetLabels(labels)
				}
			}

			warnings, errs := h.restorer.Restore(
				h.log,
				tc.restore,
				tc.backup,
				nil, // volume snapshots
				tc.tarball,
				tc.actions,
				nil, // snapshot location lister
				nil, // volume snapshotter getter
			)

			assertEmptyResults(t, warnings, errs)
			assertRestoredItems(t, h, tc.want)
		})
	}
}

// TestRestoreActionAdditionalItems runs restores with restore item actions that return additional items
// to be restored, and verifies that that the correct set of items is created in the API. Verification is
// done by looking at the namespaces/names of the items in the API; contents are not checked.
func TestRestoreActionAdditionalItems(t *testing.T) {
	tests := []struct {
		name         string
		restore      *velerov1api.Restore
		backup       *velerov1api.Backup
		tarball      io.Reader
		apiResources []*test.APIResource
		actions      []velero.RestoreItemAction
		want         map[*test.APIResource][]string
	}{
		{
			name:         "additional items that are already being restored are not restored twice",
			restore:      defaultRestore().Restore(),
			backup:       defaultBackup().Backup(),
			tarball:      newTarWriter(t).addItems("pods", test.NewPod("ns-1", "pod-1"), test.NewPod("ns-2", "pod-2")).done(),
			apiResources: []*test.APIResource{test.Pods()},
			actions: []velero.RestoreItemAction{
				&pluggableAction{
					selector: velero.ResourceSelector{IncludedNamespaces: []string{"ns-1"}},
					executeFunc: func(input *velero.RestoreItemActionExecuteInput) (*velero.RestoreItemActionExecuteOutput, error) {
						return &velero.RestoreItemActionExecuteOutput{
							UpdatedItem: input.Item,
							AdditionalItems: []velero.ResourceIdentifier{
								{GroupResource: kuberesource.Pods, Namespace: "ns-2", Name: "pod-2"},
							},
						}, nil
					},
				},
			},
			want: map[*test.APIResource][]string{
				test.Pods(): {"ns-1/pod-1", "ns-2/pod-2"},
			},
		},
		// TODO the below test case fails, which seems like a bug
		// {
		// 	name:         "when using a restore namespace filter, additional items that are in a non-included namespace are not restored",
		// 	restore:      defaultRestore().IncludedNamespaces("ns-1").Restore(),
		// 	backup:       defaultBackup().Backup(),
		// 	tarball:      newTarWriter(t).addItems("pods", test.NewPod("ns-1", "pod-1"), test.NewPod("ns-2", "pod-2")).done(),
		// 	apiResources: []*test.APIResource{test.Pods()},
		// 	actions: []velero.RestoreItemAction{
		// 		&pluggableAction{
		// 			executeFunc: func(input *velero.RestoreItemActionExecuteInput) (*velero.RestoreItemActionExecuteOutput, error) {
		// 				return &velero.RestoreItemActionExecuteOutput{
		// 					UpdatedItem: input.Item,
		// 					AdditionalItems: []velero.ResourceIdentifier{
		// 						{GroupResource: kuberesource.Pods, Namespace: "ns-2", Name: "pod-2"},
		// 					},
		// 				}, nil
		// 			},
		// 		},
		// 	},
		// 	want: map[*test.APIResource][]string{
		// 		test.Pods(): {"ns-1/pod-1"},
		// 	},
		// },
		{
			name:    "when using a restore namespace filter, additional items that are cluster-scoped are restored",
			restore: defaultRestore().IncludedNamespaces("ns-1").Restore(),
			backup:  defaultBackup().Backup(),
			tarball: newTarWriter(t).
				addItems("pods", test.NewPod("ns-1", "pod-1")).
				addItems("persistentvolumes", test.NewPV("pv-1")).
				done(),
			apiResources: []*test.APIResource{test.Pods(), test.PVs()},
			actions: []velero.RestoreItemAction{
				&pluggableAction{
					executeFunc: func(input *velero.RestoreItemActionExecuteInput) (*velero.RestoreItemActionExecuteOutput, error) {
						return &velero.RestoreItemActionExecuteOutput{
							UpdatedItem: input.Item,
							AdditionalItems: []velero.ResourceIdentifier{
								{GroupResource: kuberesource.PersistentVolumes, Name: "pv-1"},
							},
						}, nil
					},
				},
			},
			want: map[*test.APIResource][]string{
				test.Pods(): {"ns-1/pod-1"},
				test.PVs():  {"/pv-1"},
			},
		},
		// TODO the below test case fails, which seems like a bug
		// {
		// 	name:    "when using a restore resource filter, additional items that are non-included resources are not restored",
		// 	restore: defaultRestore().IncludedResources("pods").Restore(),
		// 	backup:  defaultBackup().Backup(),
		// 	tarball: newTarWriter(t).
		// 		addItems("pods", test.NewPod("ns-1", "pod-1")).
		// 		addItems("persistentvolumes", test.NewPV("pv-1")).
		// 		done(),
		// 	apiResources: []*test.APIResource{test.Pods(), test.PVs()},
		// 	actions: []velero.RestoreItemAction{
		// 		&pluggableAction{
		// 			executeFunc: func(input *velero.RestoreItemActionExecuteInput) (*velero.RestoreItemActionExecuteOutput, error) {
		// 				return &velero.RestoreItemActionExecuteOutput{
		// 					UpdatedItem: input.Item,
		// 					AdditionalItems: []velero.ResourceIdentifier{
		// 						{GroupResource: kuberesource.PersistentVolumes, Name: "pv-1"},
		// 					},
		// 				}, nil
		// 			},
		// 		},
		// 	},
		// 	want: map[*test.APIResource][]string{
		// 		test.Pods(): {"ns-1/pod-1"},
		// 		test.PVs():  nil,
		// 	},
		// },
		// TODO the below test case fails, which seems like a bug
		// {
		// 	name:    "when IncludeClusterResources=false, additional items that are cluster-scoped are not restored",
		// 	restore: defaultRestore().IncludeClusterResources(false).Restore(),
		// 	backup:  defaultBackup().Backup(),
		// 	tarball: newTarWriter(t).
		// 		addItems("pods", test.NewPod("ns-1", "pod-1")).
		// 		addItems("persistentvolumes", test.NewPV("pv-1")).
		// 		done(),
		// 	apiResources: []*test.APIResource{test.Pods(), test.PVs()},
		// 	actions: []velero.RestoreItemAction{
		// 		&pluggableAction{
		// 			executeFunc: func(input *velero.RestoreItemActionExecuteInput) (*velero.RestoreItemActionExecuteOutput, error) {
		// 				return &velero.RestoreItemActionExecuteOutput{
		// 					UpdatedItem: input.Item,
		// 					AdditionalItems: []velero.ResourceIdentifier{
		// 						{GroupResource: kuberesource.PersistentVolumes, Name: "pv-1"},
		// 					},
		// 				}, nil
		// 			},
		// 		},
		// 	},
		// 	want: map[*test.APIResource][]string{
		// 		test.Pods(): {"ns-1/pod-1"},
		// 		test.PVs():  nil,
		// 	},
		// },
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			h := newHarness(t)

			for _, r := range tc.apiResources {
				h.addItems(t, r)
			}

			warnings, errs := h.restorer.Restore(
				h.log,
				tc.restore,
				tc.backup,
				nil, // volume snapshots
				tc.tarball,
				tc.actions,
				nil, // snapshot location lister
				nil, // volume snapshotter getter
			)

			assertEmptyResults(t, warnings, errs)
			assertAPIContents(t, h, tc.want)
		})
	}
}

// TestShouldRestore runs the ShouldRestore function for various permutations of
// existing/nonexisting/being-deleted PVs, PVCs, and namespaces, and verifies the
// result/error matches expectations.
func TestShouldRestore(t *testing.T) {
	tests := []struct {
		name         string
		pvName       string
		apiResources []*test.APIResource
		namespaces   []*corev1api.Namespace
		want         bool
		wantErr      error
	}{
		{
			name:   "when PV is not found, result is true",
			pvName: "pv-1",
			want:   true,
		},
		{
			name:   "when PV is found and has Phase=Released, result is false",
			pvName: "pv-1",
			apiResources: []*test.APIResource{
				test.PVs(&corev1api.PersistentVolume{
					TypeMeta: metav1.TypeMeta{
						APIVersion: "v1",
						Kind:       "PersistentVolume",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name: "pv-1",
					},
					Status: corev1api.PersistentVolumeStatus{
						Phase: corev1api.VolumeReleased,
					},
				}),
			},
			want: false,
		},
		{
			name:   "when PV is found and has associated PVC and namespace that aren't deleting, result is false",
			pvName: "pv-1",
			apiResources: []*test.APIResource{
				test.PVs(&corev1api.PersistentVolume{
					TypeMeta:   test.NewPV("").TypeMeta,
					ObjectMeta: test.NewPV("pv-1").ObjectMeta,
					Spec: corev1api.PersistentVolumeSpec{
						ClaimRef: &corev1api.ObjectReference{
							Namespace: "ns-1",
							Name:      "pvc-1",
						},
					},
				}),
				test.PVCs(test.NewPVC("ns-1", "pvc-1")),
			},
			namespaces: []*corev1api.Namespace{test.NewNamespace("ns-1")},
			want:       false,
		},
		{
			name:   "when PV is found and has associated PVC that is deleting, result is false + timeout error",
			pvName: "pv-1",
			apiResources: []*test.APIResource{
				test.PVs(&corev1api.PersistentVolume{
					TypeMeta:   test.NewPV("").TypeMeta,
					ObjectMeta: test.NewPV("pv-1").ObjectMeta,
					Spec: corev1api.PersistentVolumeSpec{
						ClaimRef: &corev1api.ObjectReference{
							Namespace: "ns-1",
							Name:      "pvc-1",
						},
					},
				}),
				test.PVCs(
					test.NewPVC("ns-1", "pvc-1", test.WithDeletionTimestamp(time.Now())),
				),
			},
			want:    false,
			wantErr: errors.New("timed out waiting for the condition"),
		},
		{
			name:   "when PV is found, has associated PVC that's not deleting, has associated NS that is terminating, result is false + timeout error",
			pvName: "pv-1",
			apiResources: []*test.APIResource{
				test.PVs(&corev1api.PersistentVolume{
					TypeMeta:   test.NewPV("").TypeMeta,
					ObjectMeta: test.NewPV("pv-1").ObjectMeta,
					Spec: corev1api.PersistentVolumeSpec{
						ClaimRef: &corev1api.ObjectReference{
							Namespace: "ns-1",
							Name:      "pvc-1",
						},
					},
				}),
				test.PVCs(test.NewPVC("ns-1", "pvc-1")),
			},
			namespaces: []*corev1api.Namespace{
				{
					TypeMeta:   test.NewNamespace("").TypeMeta,
					ObjectMeta: test.NewNamespace("ns-1").ObjectMeta,
					Status: corev1api.NamespaceStatus{
						Phase: corev1api.NamespaceTerminating,
					},
				},
			},
			want:    false,
			wantErr: errors.New("timed out waiting for the condition"),
		},
		{
			name:   "when PV is found, has associated PVC that's not deleting, has associated NS that has deletion timestamp, result is false + timeout error",
			pvName: "pv-1",
			apiResources: []*test.APIResource{
				test.PVs(&corev1api.PersistentVolume{
					TypeMeta:   test.NewPV("").TypeMeta,
					ObjectMeta: test.NewPV("pv-1").ObjectMeta,
					Spec: corev1api.PersistentVolumeSpec{
						ClaimRef: &corev1api.ObjectReference{
							Namespace: "ns-1",
							Name:      "pvc-1",
						},
					},
				}),
				test.PVCs(test.NewPVC("ns-1", "pvc-1")),
			},
			namespaces: []*corev1api.Namespace{
				test.NewNamespace("ns-1", test.WithDeletionTimestamp(time.Now())),
			},
			want:    false,
			wantErr: errors.New("timed out waiting for the condition"),
		},
		{
			name:   "when PV is found, associated PVC is not found, result is false + timeout error",
			pvName: "pv-1",
			apiResources: []*test.APIResource{
				test.PVs(&corev1api.PersistentVolume{
					TypeMeta:   test.NewPV("").TypeMeta,
					ObjectMeta: test.NewPV("pv-1").ObjectMeta,
					Spec: corev1api.PersistentVolumeSpec{
						ClaimRef: &corev1api.ObjectReference{
							Namespace: "ns-1",
							Name:      "pvc-1",
						},
					},
				}),
			},
			want:    false,
			wantErr: errors.New("timed out waiting for the condition"),
		},
		{
			name:   "when PV is found, has associated PVC, associated namespace not found, result is false + timeout error",
			pvName: "pv-1",
			apiResources: []*test.APIResource{
				test.PVs(&corev1api.PersistentVolume{
					TypeMeta:   test.NewPV("").TypeMeta,
					ObjectMeta: test.NewPV("pv-1").ObjectMeta,
					Spec: corev1api.PersistentVolumeSpec{
						ClaimRef: &corev1api.ObjectReference{
							Namespace: "ns-1",
							Name:      "pvc-1",
						},
					},
				}),
				test.PVCs(test.NewPVC("ns-1", "pvc-1")),
			},
			want:    false,
			wantErr: errors.New("timed out waiting for the condition"),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			h := newHarness(t)

			ctx := &context{
				log:                        h.log,
				dynamicFactory:             client.NewDynamicFactory(h.DynamicClient),
				namespaceClient:            h.KubeClient.CoreV1().Namespaces(),
				resourceTerminatingTimeout: time.Millisecond,
			}

			for _, resource := range tc.apiResources {
				h.addItems(t, resource)
			}

			for _, ns := range tc.namespaces {
				_, err := ctx.namespaceClient.Create(ns)
				require.NoError(t, err)
			}

			pvClient, err := ctx.dynamicFactory.ClientForGroupVersionResource(
				schema.GroupVersion{Group: "", Version: "v1"},
				metav1.APIResource{Name: "persistentvolumes"},
				"",
			)
			require.NoError(t, err)

			res, err := ctx.shouldRestore(tc.pvName, pvClient)
			assert.Equal(t, tc.want, res)
			if tc.wantErr != nil {
				if assert.NotNil(t, err, "expected a non-nil error") {
					assert.EqualError(t, err, tc.wantErr.Error())
				}
			} else {
				assert.Nil(t, err)
			}
		})
	}
}

func assertRestoredItems(t *testing.T, h *harness, want []*test.APIResource) {
	t.Helper()

	for _, resource := range want {
		resourceClient := h.DynamicClient.Resource(resource.GVR())
		for _, item := range resource.Items {
			var client dynamic.ResourceInterface
			if item.GetNamespace() != "" {
				client = resourceClient.Namespace(item.GetNamespace())
			} else {
				client = resourceClient
			}

			res, err := client.Get(item.GetName(), metav1.GetOptions{})
			if !assert.NoError(t, err) {
				continue
			}

			itemJSON, err := json.Marshal(item)
			if !assert.NoError(t, err) {
				continue
			}

			t.Logf("%v", string(itemJSON))

			u := make(map[string]interface{})
			if !assert.NoError(t, json.Unmarshal(itemJSON, &u)) {
				continue
			}
			want := &unstructured.Unstructured{Object: u}

			// These fields get non-nil zero values in the unstructured objects if they're
			// empty in the structured objects. Remove them to make comparison easier.
			unstructured.RemoveNestedField(want.Object, "metadata", "creationTimestamp")
			unstructured.RemoveNestedField(want.Object, "status")

			assert.Equal(t, want, res)
		}
	}
}

// assertResourceCreationOrder ensures that resources were created in the expected
// order. Any resources *not* in resourcePriorities are required to come *after* all
// resources in any order.
func assertResourceCreationOrder(t *testing.T, resourcePriorities []string, createdResources []resourceID) {
	// lastSeen tracks the index in 'resourcePriorities' of the last resource type
	// we saw created. Once we've seen a resource in 'resourcePriorities', we should
	// never see another instance of a prior resource.
	lastSeen := 0

	// Find the index in 'resourcePriorities' of the resource type for
	// the current item, if it exists. This index ('current') *must*
	// be greater than or equal to 'lastSeen', which was the last resource
	// we saw, since otherwise the current resource would be out of order. By
	// initializing current to len(ordered), we're saying that if the resource
	// is not explicitly in orderedResources, then it must come *after*
	// all orderedResources.
	for _, r := range createdResources {
		current := len(resourcePriorities)
		for i, item := range resourcePriorities {
			if item == r.groupResource {
				current = i
				break
			}
		}

		// the index of the current resource must be the same as or greater than the index of
		// the last resource we saw for the restored order to be correct.
		assert.True(t, current >= lastSeen, "%s was restored out of order", r.groupResource)
		lastSeen = current
	}
}

type resourceID struct {
	groupResource string
	nsAndName     string
}

// createRecorder provides a Reactor that can be used to capture
// resources created in a fake client.
type createRecorder struct {
	t         *testing.T
	resources []resourceID
}

func (cr *createRecorder) reactor() func(kubetesting.Action) (bool, runtime.Object, error) {
	return func(action kubetesting.Action) (bool, runtime.Object, error) {
		createAction, ok := action.(kubetesting.CreateAction)
		if !ok {
			return false, nil, nil
		}

		accessor, err := meta.Accessor(createAction.GetObject())
		assert.NoError(cr.t, err)

		cr.resources = append(cr.resources, resourceID{
			groupResource: action.GetResource().GroupResource().String(),
			nsAndName:     fmt.Sprintf("%s/%s", action.GetNamespace(), accessor.GetName()),
		})

		return false, nil, nil
	}
}

func defaultRestore() *Builder {
	return NewNamedBuilder(velerov1api.DefaultNamespace, "restore-1").Backup("backup-1")
}

// assertAPIContents asserts that the dynamic client on the provided harness contains
// all of the items specified in 'want' (a map from an APIResource definition to a slice
// of resource identifiers, formatted as <namespace>/<name>).
func assertAPIContents(t *testing.T, h *harness, want map[*test.APIResource][]string) {
	t.Helper()

	for r, want := range want {
		res, err := h.DynamicClient.Resource(r.GVR()).List(metav1.ListOptions{})
		assert.NoError(t, err)
		if err != nil {
			continue
		}

		got := sets.NewString()
		for _, item := range res.Items {
			got.Insert(fmt.Sprintf("%s/%s", item.GetNamespace(), item.GetName()))
		}

		assert.Equal(t, sets.NewString(want...), got)
	}
}

func assertEmptyResults(t *testing.T, res ...Result) {
	t.Helper()

	for _, r := range res {
		assert.Empty(t, r.Cluster)
		assert.Empty(t, r.Namespaces)
		assert.Empty(t, r.Velero)
	}
}

type tarWriter struct {
	t   *testing.T
	buf *bytes.Buffer
	gzw *gzip.Writer
	tw  *tar.Writer
}

func newTarWriter(t *testing.T) *tarWriter {
	tw := new(tarWriter)
	tw.t = t
	tw.buf = new(bytes.Buffer)
	tw.gzw = gzip.NewWriter(tw.buf)
	tw.tw = tar.NewWriter(tw.gzw)

	return tw
}

func (tw *tarWriter) addItems(groupResource string, items ...metav1.Object) *tarWriter {
	tw.t.Helper()

	for _, obj := range items {

		var path string
		if obj.GetNamespace() == "" {
			path = fmt.Sprintf("resources/%s/cluster/%s.json", groupResource, obj.GetName())
		} else {
			path = fmt.Sprintf("resources/%s/namespaces/%s/%s.json", groupResource, obj.GetNamespace(), obj.GetName())
		}

		tw.add(path, obj)
	}

	return tw
}

func (tw *tarWriter) add(name string, obj interface{}) *tarWriter {
	tw.t.Helper()

	var data []byte
	var err error

	switch obj.(type) {
	case runtime.Object:
		data, err = encode.Encode(obj.(runtime.Object), "json")
	case []byte:
		data = obj.([]byte)
	default:
		data, err = json.Marshal(obj)
	}
	require.NoError(tw.t, err)

	require.NoError(tw.t, tw.tw.WriteHeader(&tar.Header{
		Name:     name,
		Size:     int64(len(data)),
		Typeflag: tar.TypeReg,
		Mode:     0755,
		ModTime:  time.Now(),
	}))

	_, err = tw.tw.Write(data)
	require.NoError(tw.t, err)

	return tw
}

func (tw *tarWriter) done() *bytes.Buffer {
	require.NoError(tw.t, tw.tw.Close())
	require.NoError(tw.t, tw.gzw.Close())

	return tw.buf
}

type harness struct {
	*test.APIServer

	restorer *kubernetesRestorer
	log      logrus.FieldLogger
}

func newHarness(t *testing.T) *harness {
	t.Helper()

	apiServer := test.NewAPIServer(t)
	log := logrus.StandardLogger()

	discoveryHelper, err := discovery.NewHelper(apiServer.DiscoveryClient, log)
	require.NoError(t, err)

	return &harness{
		APIServer: apiServer,
		restorer: &kubernetesRestorer{
			discoveryHelper:            discoveryHelper,
			dynamicFactory:             client.NewDynamicFactory(apiServer.DynamicClient),
			namespaceClient:            apiServer.KubeClient.CoreV1().Namespaces(),
			resourceTerminatingTimeout: time.Minute,
			logger:                     log,
			fileSystem:                 testutil.NewFakeFileSystem(),

			// unsupported
			resticRestorerFactory: nil,
			resticTimeout:         0,
		},
		log: log,
	}
}

func (h *harness) addItems(t *testing.T, resource *test.APIResource) {
	t.Helper()

	h.DiscoveryClient.WithAPIResource(resource)
	require.NoError(t, h.restorer.discoveryHelper.Refresh())

	for _, item := range resource.Items {
		obj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(item)
		require.NoError(t, err)

		unstructuredObj := &unstructured.Unstructured{Object: obj}

		// These fields have non-nil zero values in the unstructured objects. We remove
		// them to make comparison easier in our tests.
		unstructured.RemoveNestedField(unstructuredObj.Object, "metadata", "creationTimestamp")
		unstructured.RemoveNestedField(unstructuredObj.Object, "status")

		if resource.Namespaced {
			_, err = h.DynamicClient.Resource(resource.GVR()).Namespace(item.GetNamespace()).Create(unstructuredObj, metav1.CreateOptions{})
		} else {
			_, err = h.DynamicClient.Resource(resource.GVR()).Create(unstructuredObj, metav1.CreateOptions{})
		}
		require.NoError(t, err)
	}
}
