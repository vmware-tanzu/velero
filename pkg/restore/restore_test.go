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
	ctx "context"
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
	discoveryfake "k8s.io/client-go/discovery/fake"
	"k8s.io/client-go/dynamic"
	kubefake "k8s.io/client-go/kubernetes/fake"
	kubetesting "k8s.io/client-go/testing"

	velerov1api "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	"github.com/vmware-tanzu/velero/pkg/builder"
	"github.com/vmware-tanzu/velero/pkg/client"
	"github.com/vmware-tanzu/velero/pkg/discovery"
	velerov1informers "github.com/vmware-tanzu/velero/pkg/generated/informers/externalversions"
	"github.com/vmware-tanzu/velero/pkg/kuberesource"
	"github.com/vmware-tanzu/velero/pkg/plugin/velero"
	"github.com/vmware-tanzu/velero/pkg/restic"
	resticmocks "github.com/vmware-tanzu/velero/pkg/restic/mocks"
	"github.com/vmware-tanzu/velero/pkg/test"
	testutil "github.com/vmware-tanzu/velero/pkg/test"
	"github.com/vmware-tanzu/velero/pkg/util/collections"
	"github.com/vmware-tanzu/velero/pkg/util/encode"
	kubeutil "github.com/vmware-tanzu/velero/pkg/util/kube"
	"github.com/vmware-tanzu/velero/pkg/volume"
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
			restore: defaultRestore().Result(),
			backup:  defaultBackup().Result(),
			tarball: newTarWriter(t).
				addItems("pods",
					builder.ForPod("ns-1", "pod-1").Result(),
					builder.ForPod("ns-2", "pod-2").Result(),
				).
				addItems("persistentvolumes",
					builder.ForPersistentVolume("pv-1").Result(),
					builder.ForPersistentVolume("pv-2").Result(),
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
			restore: defaultRestore().IncludedResources("pods").Result(),
			backup:  defaultBackup().Result(),
			tarball: newTarWriter(t).
				addItems("pods",
					builder.ForPod("ns-1", "pod-1").Result(),
					builder.ForPod("ns-2", "pod-2").Result(),
				).
				addItems("persistentvolumes",
					builder.ForPersistentVolume("pv-1").Result(),
					builder.ForPersistentVolume("pv-2").Result(),
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
			restore: defaultRestore().ExcludedResources("pvs").Result(),
			backup:  defaultBackup().Result(),
			tarball: newTarWriter(t).
				addItems("pods",
					builder.ForPod("ns-1", "pod-1").Result(),
					builder.ForPod("ns-2", "pod-2").Result(),
				).
				addItems("persistentvolumes",
					builder.ForPersistentVolume("pv-1").Result(),
					builder.ForPersistentVolume("pv-2").Result(),
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
			restore: defaultRestore().IncludedNamespaces("ns-1").Result(),
			backup:  defaultBackup().Result(),
			tarball: newTarWriter(t).
				addItems("pods",
					builder.ForPod("ns-1", "pod-1").Result(),
					builder.ForPod("ns-2", "pod-2").Result(),
				).
				addItems("deployments.apps",
					builder.ForDeployment("ns-1", "deploy-1").Result(),
					builder.ForDeployment("ns-2", "deploy-2").Result(),
				).
				addItems("persistentvolumes",
					builder.ForPersistentVolume("pv-1").Result(),
					builder.ForPersistentVolume("pv-2").Result(),
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
			restore: defaultRestore().ExcludedNamespaces("ns-2").Result(),
			backup:  defaultBackup().Result(),
			tarball: newTarWriter(t).
				addItems("pods",
					builder.ForPod("ns-1", "pod-1").Result(),
					builder.ForPod("ns-2", "pod-2").Result(),
				).
				addItems("deployments.apps",
					builder.ForDeployment("ns-1", "deploy-1").Result(),
					builder.ForDeployment("ns-2", "deploy-2").Result(),
				).
				addItems("persistentvolumes",
					builder.ForPersistentVolume("pv-1").Result(),
					builder.ForPersistentVolume("pv-2").Result(),
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
			restore: defaultRestore().IncludeClusterResources(false).Result(),
			backup:  defaultBackup().Result(),
			tarball: newTarWriter(t).
				addItems("pods",
					builder.ForPod("ns-1", "pod-1").Result(),
					builder.ForPod("ns-2", "pod-2").Result(),
				).
				addItems("deployments.apps",
					builder.ForDeployment("ns-1", "deploy-1").Result(),
					builder.ForDeployment("ns-2", "deploy-2").Result(),
				).
				addItems("persistentvolumes",
					builder.ForPersistentVolume("pv-1").Result(),
					builder.ForPersistentVolume("pv-2").Result(),
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
			restore: defaultRestore().LabelSelector(&metav1.LabelSelector{MatchLabels: map[string]string{"a": "b"}}).Result(),
			backup:  defaultBackup().Result(),
			tarball: newTarWriter(t).
				addItems("pods",
					builder.ForPod("ns-1", "pod-1").ObjectMeta(builder.WithLabels("a", "b")).Result(),
					builder.ForPod("ns-2", "pod-2").Result(),
				).
				addItems("deployments.apps",
					builder.ForDeployment("ns-1", "deploy-1").Result(),
					builder.ForDeployment("ns-2", "deploy-2").ObjectMeta(builder.WithLabels("a", "b")).Result(),
				).
				addItems("persistentvolumes",
					builder.ForPersistentVolume("pv-1").ObjectMeta(builder.WithLabels("a", "b")).Result(),
					builder.ForPersistentVolume("pv-2").ObjectMeta(builder.WithLabels("a", "c")).Result(),
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
			restore: defaultRestore().IncludedNamespaces("ns-1").IncludeClusterResources(true).Result(),
			backup:  defaultBackup().Result(),
			tarball: newTarWriter(t).
				addItems("pods",
					builder.ForPod("ns-1", "pod-1").Result(),
					builder.ForPod("ns-2", "pod-2").Result(),
				).
				addItems("deployments.apps",
					builder.ForDeployment("ns-1", "deploy-1").Result(),
					builder.ForDeployment("ns-2", "deploy-2").Result(),
				).
				addItems("persistentvolumes",
					builder.ForPersistentVolume("pv-1").Result(),
					builder.ForPersistentVolume("pv-2").Result(),
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
			restore: defaultRestore().IncludedNamespaces("ns-1").IncludeClusterResources(false).Result(),
			backup:  defaultBackup().Result(),
			tarball: newTarWriter(t).
				addItems("pods",
					builder.ForPod("ns-1", "pod-1").Result(),
					builder.ForPod("ns-2", "pod-2").Result(),
				).
				addItems("deployments.apps",
					builder.ForDeployment("ns-1", "deploy-1").Result(),
					builder.ForDeployment("ns-2", "deploy-2").Result(),
				).
				addItems("persistentvolumes",
					builder.ForPersistentVolume("pv-1").Result(),
					builder.ForPersistentVolume("pv-2").Result(),
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
				test.PVs():         {},
			},
		},
		{
			name:    "should not include cluster-scoped resources if restoring subset of namespaces and IncludeClusterResources=nil",
			restore: defaultRestore().IncludedNamespaces("ns-1").Result(),
			backup:  defaultBackup().Result(),
			tarball: newTarWriter(t).
				addItems("pods",
					builder.ForPod("ns-1", "pod-1").Result(),
					builder.ForPod("ns-2", "pod-2").Result(),
				).
				addItems("deployments.apps",
					builder.ForDeployment("ns-1", "deploy-1").Result(),
					builder.ForDeployment("ns-2", "deploy-2").Result(),
				).
				addItems("persistentvolumes",
					builder.ForPersistentVolume("pv-1").Result(),
					builder.ForPersistentVolume("pv-2").Result(),
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
				test.PVs():         {},
			},
		},
		{
			name:    "should include cluster-scoped resources if restoring all namespaces and IncludeClusterResources=true",
			restore: defaultRestore().IncludeClusterResources(true).Result(),
			backup:  defaultBackup().Result(),
			tarball: newTarWriter(t).
				addItems("pods",
					builder.ForPod("ns-1", "pod-1").Result(),
					builder.ForPod("ns-2", "pod-2").Result(),
				).
				addItems("deployments.apps",
					builder.ForDeployment("ns-1", "deploy-1").Result(),
					builder.ForDeployment("ns-2", "deploy-2").Result(),
				).
				addItems("persistentvolumes",
					builder.ForPersistentVolume("pv-1").Result(),
					builder.ForPersistentVolume("pv-2").Result(),
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
			restore: defaultRestore().IncludeClusterResources(false).Result(),
			backup:  defaultBackup().Result(),
			tarball: newTarWriter(t).
				addItems("pods",
					builder.ForPod("ns-1", "pod-1").Result(),
					builder.ForPod("ns-2", "pod-2").Result(),
				).
				addItems("deployments.apps",
					builder.ForDeployment("ns-1", "deploy-1").Result(),
					builder.ForDeployment("ns-2", "deploy-2").Result(),
				).
				addItems("persistentvolumes",
					builder.ForPersistentVolume("pv-1").Result(),
					builder.ForPersistentVolume("pv-2").Result(),
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
			restore: defaultRestore().IncludedResources("*", "pods").Result(),
			backup:  defaultBackup().Result(),
			tarball: newTarWriter(t).
				addItems("pods",
					builder.ForPod("ns-1", "pod-1").Result(),
					builder.ForPod("ns-2", "pod-2").Result(),
				).
				addItems("deployments.apps",
					builder.ForDeployment("ns-1", "deploy-1").Result(),
					builder.ForDeployment("ns-2", "deploy-2").Result(),
				).
				addItems("persistentvolumes",
					builder.ForPersistentVolume("pv-1").Result(),
					builder.ForPersistentVolume("pv-2").Result(),
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
			restore: defaultRestore().ExcludedResources("*").Result(),
			backup:  defaultBackup().Result(),
			tarball: newTarWriter(t).
				addItems("pods",
					builder.ForPod("ns-1", "pod-1").Result(),
					builder.ForPod("ns-2", "pod-2").Result(),
				).
				addItems("deployments.apps",
					builder.ForDeployment("ns-1", "deploy-1").Result(),
					builder.ForDeployment("ns-2", "deploy-2").Result(),
				).
				addItems("persistentvolumes",
					builder.ForPersistentVolume("pv-1").Result(),
					builder.ForPersistentVolume("pv-2").Result(),
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
			restore: defaultRestore().IncludedResources("pods", "unresolvable").Result(),
			backup:  defaultBackup().Result(),
			tarball: newTarWriter(t).
				addItems("pods",
					builder.ForPod("ns-1", "pod-1").Result(),
					builder.ForPod("ns-2", "pod-2").Result(),
				).
				addItems("deployments.apps",
					builder.ForDeployment("ns-1", "deploy-1").Result(),
					builder.ForDeployment("ns-2", "deploy-2").Result(),
				).
				addItems("persistentvolumes",
					builder.ForPersistentVolume("pv-1").Result(),
					builder.ForPersistentVolume("pv-2").Result(),
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
			restore: defaultRestore().ExcludedResources("deployments", "unresolvable").Result(),
			backup:  defaultBackup().Result(),
			tarball: newTarWriter(t).
				addItems("pods",
					builder.ForPod("ns-1", "pod-1").Result(),
					builder.ForPod("ns-2", "pod-2").Result(),
				).
				addItems("deployments.apps",
					builder.ForDeployment("ns-1", "deploy-1").Result(),
					builder.ForDeployment("ns-2", "deploy-2").Result(),
				).
				addItems("persistentvolumes",
					builder.ForPersistentVolume("pv-1").Result(),
					builder.ForPersistentVolume("pv-2").Result(),
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
			restore:      defaultRestore().Result(),
			backup:       defaultBackup().Result(),
			tarball:      newTarWriter(t).addItems("pods", builder.ForPod("ns-1", "pod-1").ObjectMeta(builder.WithAnnotations(corev1api.MirrorPodAnnotationKey, "foo")).Result()).done(),
			apiResources: []*test.APIResource{test.Pods()},
			want:         map[*test.APIResource][]string{test.Pods(): {}},
		},
		{
			name:         "service accounts are restored",
			restore:      defaultRestore().Result(),
			backup:       defaultBackup().Result(),
			tarball:      newTarWriter(t).addItems("serviceaccounts", builder.ForServiceAccount("ns-1", "sa-1").Result()).done(),
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

			data := Request{
				Log:              h.log,
				Restore:          tc.restore,
				Backup:           tc.backup,
				PodVolumeBackups: nil,
				VolumeSnapshots:  nil,
				BackupReader:     tc.tarball,
			}
			warnings, errs := h.restorer.Restore(
				data,
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
			restore: defaultRestore().NamespaceMappings("ns-1", "mapped-ns-1", "ns-2", "mapped-ns-2").Result(),
			backup:  defaultBackup().Result(),
			apiResources: []*test.APIResource{
				test.Pods(),
			},
			tarball: newTarWriter(t).
				addItems("pods",
					builder.ForPod("ns-1", "pod-1").Result(),
					builder.ForPod("ns-2", "pod-2").Result(),
					builder.ForPod("ns-3", "pod-3").Result(),
				).
				done(),
			want: map[*test.APIResource][]string{
				test.Pods(): {"mapped-ns-1/pod-1", "mapped-ns-2/pod-2", "ns-3/pod-3"},
			},
		},
		{
			name:    "namespace mappings are applied when IncludedNamespaces are specified",
			restore: defaultRestore().IncludedNamespaces("ns-1", "ns-2").NamespaceMappings("ns-1", "mapped-ns-1", "ns-2", "mapped-ns-2").Result(),
			backup:  defaultBackup().Result(),
			apiResources: []*test.APIResource{
				test.Pods(),
			},
			tarball: newTarWriter(t).
				addItems("pods",
					builder.ForPod("ns-1", "pod-1").Result(),
					builder.ForPod("ns-2", "pod-2").Result(),
					builder.ForPod("ns-3", "pod-3").Result(),
				).
				done(),
			want: map[*test.APIResource][]string{
				test.Pods(): {"mapped-ns-1/pod-1", "mapped-ns-2/pod-2"},
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

			data := Request{
				Log:              h.log,
				Restore:          tc.restore,
				Backup:           tc.backup,
				PodVolumeBackups: nil,
				VolumeSnapshots:  nil,
				BackupReader:     tc.tarball,
			}
			warnings, errs := h.restorer.Restore(
				data,
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
			restore: defaultRestore().Result(),
			backup:  defaultBackup().Result(),
			tarball: newTarWriter(t).
				addItems("pods",
					builder.ForPod("ns-1", "pod-1").Result(),
					builder.ForPod("ns-2", "pod-2").Result(),
				).
				addItems("persistentvolumes",
					builder.ForPersistentVolume("pv-1").Result(),
					builder.ForPersistentVolume("pv-2").Result(),
				).
				addItems("deployments.apps",
					builder.ForDeployment("ns-1", "deploy-1").Result(),
					builder.ForDeployment("ns-2", "deploy-2").Result(),
				).
				addItems("serviceaccounts",
					builder.ForServiceAccount("ns-1", "sa-1").Result(),
					builder.ForServiceAccount("ns-2", "sa-2").Result(),
				).
				addItems("persistentvolumeclaims",
					builder.ForPersistentVolumeClaim("ns-1", "pvc-1").Result(),
					builder.ForPersistentVolumeClaim("ns-2", "pvc-2").Result(),
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

		data := Request{
			Log:              h.log,
			Restore:          tc.restore,
			Backup:           tc.backup,
			PodVolumeBackups: nil,
			VolumeSnapshots:  nil,
			BackupReader:     tc.tarball,
		}
		warnings, errs := h.restorer.Restore(
			data,
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
			restore: defaultRestore().Result(),
			backup:  defaultBackup().Result(),
			tarball: newTarWriter(t).
				done(),
			wantErrs: Result{
				Velero: []string{"error parsing backup contents: directory \"resources\" does not exist"},
			},
		},
		{
			name:    "invalid JSON is reported as an error and restore continues",
			restore: defaultRestore().Result(),
			backup:  defaultBackup().Result(),
			tarball: newTarWriter(t).
				add("resources/pods/namespaces/ns-1/pod-1.json", []byte("invalid JSON")).
				addItems("pods",
					builder.ForPod("ns-1", "pod-2").Result(),
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

			data := Request{
				Log:              h.log,
				Restore:          tc.restore,
				Backup:           tc.backup,
				PodVolumeBackups: nil,
				VolumeSnapshots:  nil,
				BackupReader:     tc.tarball,
			}
			warnings, errs := h.restorer.Restore(
				data,
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
			restore: defaultRestore().Result(),
			backup:  defaultBackup().Result(),
			tarball: newTarWriter(t).
				addItems("pods",
					builder.ForPod("ns-1", "pod-1").
						ObjectMeta(
							builder.WithLabels("key-1", "val-1"),
							builder.WithAnnotations("key-1", "val-1"),
							builder.WithClusterName("cluster-1"),
							builder.WithFinalizers("finalizer-1"),
						).
						Result(),
				).
				done(),
			apiResources: []*test.APIResource{
				test.Pods(),
			},
			want: []*test.APIResource{
				test.Pods(
					builder.ForPod("ns-1", "pod-1").
						ObjectMeta(
							builder.WithLabels("key-1", "val-1", "velero.io/backup-name", "backup-1", "velero.io/restore-name", "restore-1"),
							builder.WithAnnotations("key-1", "val-1"),
						).
						Result(),
				),
			},
		},
		{
			name:    "status gets removed",
			restore: defaultRestore().Result(),
			backup:  defaultBackup().Result(),
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
					builder.ForPod("ns-1", "pod-1").ObjectMeta(builder.WithLabels("velero.io/backup-name", "backup-1", "velero.io/restore-name", "restore-1")).Result(),
				),
			},
		},
		{
			name:    "object gets labeled with full backup and restore names when they're both shorter than 63 characters",
			restore: defaultRestore().Result(),
			backup:  defaultBackup().Result(),
			tarball: newTarWriter(t).
				addItems("pods", builder.ForPod("ns-1", "pod-1").Result()).
				done(),
			apiResources: []*test.APIResource{
				test.Pods(),
			},
			want: []*test.APIResource{
				test.Pods(builder.ForPod("ns-1", "pod-1").ObjectMeta(builder.WithLabels("velero.io/backup-name", "backup-1", "velero.io/restore-name", "restore-1")).Result()),
			},
		},
		{
			name: "object gets labeled with full backup and restore names when they're both equal to 63 characters",
			restore: builder.ForRestore(velerov1api.DefaultNamespace, "the-really-long-kube-service-name-that-is-exactly-63-characters").
				Backup("the-really-long-kube-service-name-that-is-exactly-63-characters").
				Result(),
			backup: builder.ForBackup(velerov1api.DefaultNamespace, "the-really-long-kube-service-name-that-is-exactly-63-characters").Result(),
			tarball: newTarWriter(t).
				addItems("pods", builder.ForPod("ns-1", "pod-1").Result()).
				done(),
			apiResources: []*test.APIResource{
				test.Pods(),
			},
			want: []*test.APIResource{
				test.Pods(
					builder.ForPod("ns-1", "pod-1").
						ObjectMeta(
							builder.WithLabels(
								"velero.io/backup-name", "the-really-long-kube-service-name-that-is-exactly-63-characters",
								"velero.io/restore-name", "the-really-long-kube-service-name-that-is-exactly-63-characters",
							),
						).Result(),
				),
			},
		},
		{
			name: "object gets labeled with shortened backup and restore names when they're both longer than 63 characters",
			restore: builder.ForRestore(velerov1api.DefaultNamespace, "the-really-long-kube-service-name-that-is-much-greater-than-63-characters").
				Backup("the-really-long-kube-service-name-that-is-much-greater-than-63-characters").
				Result(),
			backup: builder.ForBackup(velerov1api.DefaultNamespace, "the-really-long-kube-service-name-that-is-much-greater-than-63-characters").Result(),
			tarball: newTarWriter(t).
				addItems("pods", builder.ForPod("ns-1", "pod-1").Result()).
				done(),
			apiResources: []*test.APIResource{
				test.Pods(),
			},
			want: []*test.APIResource{
				test.Pods(builder.ForPod("ns-1", "pod-1").
					ObjectMeta(
						builder.WithLabels(
							"velero.io/backup-name", "the-really-long-kube-service-name-that-is-much-greater-th8a11b3",
							"velero.io/restore-name", "the-really-long-kube-service-name-that-is-much-greater-th8a11b3",
						),
					).
					Result(),
				),
			},
		},
		{
			name:    "no error when service account already exists in cluster and is identical to the backed up one",
			restore: defaultRestore().Result(),
			backup:  defaultBackup().Result(),
			tarball: newTarWriter(t).
				addItems("serviceaccounts", builder.ForServiceAccount("ns-1", "sa-1").Result()).
				done(),
			apiResources: []*test.APIResource{
				test.ServiceAccounts(builder.ForServiceAccount("ns-1", "sa-1").Result()),
			},
			want: []*test.APIResource{
				test.ServiceAccounts(builder.ForServiceAccount("ns-1", "sa-1").Result()),
			},
		},
		{
			name:    "service account secrets and image pull secrets are restored when service account already exists in cluster",
			restore: defaultRestore().Result(),
			backup:  defaultBackup().Result(),
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
				test.ServiceAccounts(builder.ForServiceAccount("ns-1", "sa-1").Result()),
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

			data := Request{
				Log:              h.log,
				Restore:          tc.restore,
				Backup:           tc.backup,
				PodVolumeBackups: nil,
				VolumeSnapshots:  nil,
				BackupReader:     tc.tarball,
			}
			warnings, errs := h.restorer.Restore(
				data,
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
			restore: defaultRestore().Result(),
			backup:  defaultBackup().Result(),
			tarball: newTarWriter(t).
				addItems("pods", builder.ForPod("ns-1", "pod-1").Result(), builder.ForPod("ns-2", "pod-2").Result()).
				addItems("persistentvolumes", builder.ForPersistentVolume("pv-1").Result(), builder.ForPersistentVolume("pv-2").Result()).
				done(),
			apiResources: []*test.APIResource{test.Pods(), test.PVs()},
			actions: map[*recordResourcesAction][]string{
				new(recordResourcesAction): {"ns-1/pod-1", "ns-2/pod-2", "pv-1", "pv-2"},
			},
		},
		{
			name:    "single action with a resource selector for namespaced resources runs only for matching resources",
			restore: defaultRestore().Result(),
			backup:  defaultBackup().Result(),
			tarball: newTarWriter(t).
				addItems("pods", builder.ForPod("ns-1", "pod-1").Result(), builder.ForPod("ns-2", "pod-2").Result()).
				addItems("persistentvolumes", builder.ForPersistentVolume("pv-1").Result(), builder.ForPersistentVolume("pv-2").Result()).
				done(),
			apiResources: []*test.APIResource{test.Pods(), test.PVs()},
			actions: map[*recordResourcesAction][]string{
				new(recordResourcesAction).ForResource("pods"): {"ns-1/pod-1", "ns-2/pod-2"},
			},
		},
		{
			name:    "single action with a resource selector for cluster-scoped resources runs only for matching resources",
			restore: defaultRestore().Result(),
			backup:  defaultBackup().Result(),
			tarball: newTarWriter(t).
				addItems("pods", builder.ForPod("ns-1", "pod-1").Result(), builder.ForPod("ns-2", "pod-2").Result()).
				addItems("persistentvolumes", builder.ForPersistentVolume("pv-1").Result(), builder.ForPersistentVolume("pv-2").Result()).
				done(),
			apiResources: []*test.APIResource{test.Pods(), test.PVs()},
			actions: map[*recordResourcesAction][]string{
				new(recordResourcesAction).ForResource("persistentvolumes"): {"pv-1", "pv-2"},
			},
		},
		{
			name:    "single action with a namespace selector runs only for resources in that namespace",
			restore: defaultRestore().Result(),
			backup:  defaultBackup().Result(),
			tarball: newTarWriter(t).
				addItems("pods", builder.ForPod("ns-1", "pod-1").Result(), builder.ForPod("ns-2", "pod-2").Result()).
				addItems("persistentvolumeclaims", builder.ForPersistentVolumeClaim("ns-1", "pvc-1").Result(), builder.ForPersistentVolumeClaim("ns-2", "pvc-2").Result()).
				addItems("persistentvolumes", builder.ForPersistentVolume("pv-1").Result(), builder.ForPersistentVolume("pv-2").Result()).
				done(),
			apiResources: []*test.APIResource{test.Pods(), test.PVCs(), test.PVs()},
			actions: map[*recordResourcesAction][]string{
				new(recordResourcesAction).ForNamespace("ns-1"): {"ns-1/pod-1", "ns-1/pvc-1"},
			},
		},
		{
			name:    "single action with a resource and namespace selector runs only for matching resources in that namespace",
			restore: defaultRestore().Result(),
			backup:  defaultBackup().Result(),
			tarball: newTarWriter(t).
				addItems("pods", builder.ForPod("ns-1", "pod-1").Result(), builder.ForPod("ns-2", "pod-2").Result()).
				addItems("persistentvolumeclaims", builder.ForPersistentVolumeClaim("ns-1", "pvc-1").Result(), builder.ForPersistentVolumeClaim("ns-2", "pvc-2").Result()).
				addItems("persistentvolumes", builder.ForPersistentVolume("pv-1").Result(), builder.ForPersistentVolume("pv-2").Result()).
				done(),
			apiResources: []*test.APIResource{test.Pods(), test.PVCs(), test.PVs()},
			actions: map[*recordResourcesAction][]string{
				new(recordResourcesAction).ForNamespace("ns-1").ForResource("pods"): {"ns-1/pod-1"},
			},
		},
		{
			name:    "multiple actions, each with a different resource selector using short name, run for matching resources",
			restore: defaultRestore().Result(),
			backup:  defaultBackup().Result(),
			tarball: newTarWriter(t).
				addItems("pods", builder.ForPod("ns-1", "pod-1").Result(), builder.ForPod("ns-2", "pod-2").Result()).
				addItems("persistentvolumeclaims", builder.ForPersistentVolumeClaim("ns-1", "pvc-1").Result(), builder.ForPersistentVolumeClaim("ns-2", "pvc-2").Result()).
				addItems("persistentvolumes", builder.ForPersistentVolume("pv-1").Result(), builder.ForPersistentVolume("pv-2").Result()).
				done(),
			apiResources: []*test.APIResource{test.Pods(), test.PVCs(), test.PVs()},
			actions: map[*recordResourcesAction][]string{
				new(recordResourcesAction).ForResource("po"): {"ns-1/pod-1", "ns-2/pod-2"},
				new(recordResourcesAction).ForResource("pv"): {"pv-1", "pv-2"},
			},
		},
		{
			name:    "actions with selectors that don't match anything don't run for any resources",
			restore: defaultRestore().Result(),
			backup:  defaultBackup().Result(),
			tarball: newTarWriter(t).
				addItems("pods", builder.ForPod("ns-1", "pod-1").Result()).
				addItems("persistentvolumeclaims", builder.ForPersistentVolumeClaim("ns-2", "pvc-2").Result()).
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

			data := Request{
				Log:              h.log,
				Restore:          tc.restore,
				Backup:           tc.backup,
				PodVolumeBackups: nil,
				VolumeSnapshots:  nil,
				BackupReader:     tc.tarball,
			}
			warnings, errs := h.restorer.Restore(
				data,
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
			restore:      defaultRestore().Result(),
			backup:       defaultBackup().Result(),
			tarball:      newTarWriter(t).addItems("pods", builder.ForPod("ns-1", "pod-1").Result()).done(),
			apiResources: []*test.APIResource{test.Pods()},
			actions: []velero.RestoreItemAction{
				modifyingActionGetter(func(item *unstructured.Unstructured) {
					item.SetLabels(map[string]string{"updated": "true"})
				}),
			},
			want: []*test.APIResource{
				test.Pods(
					builder.ForPod("ns-1", "pod-1").ObjectMeta(builder.WithLabels("updated", "true")).Result(),
				),
			},
		},
		{
			name:         "action that removes a label to item gets restored",
			restore:      defaultRestore().Result(),
			backup:       defaultBackup().Result(),
			tarball:      newTarWriter(t).addItems("pods", builder.ForPod("ns-1", "pod-1").ObjectMeta(builder.WithLabels("should-be-removed", "true")).Result()).done(),
			apiResources: []*test.APIResource{test.Pods()},
			actions: []velero.RestoreItemAction{
				modifyingActionGetter(func(item *unstructured.Unstructured) {
					item.SetLabels(nil)
				}),
			},
			want: []*test.APIResource{
				test.Pods(builder.ForPod("ns-1", "pod-1").Result()),
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

			data := Request{
				Log:              h.log,
				Restore:          tc.restore,
				Backup:           tc.backup,
				PodVolumeBackups: nil,
				VolumeSnapshots:  nil,
				BackupReader:     tc.tarball,
			}
			warnings, errs := h.restorer.Restore(
				data,
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
			restore:      defaultRestore().Result(),
			backup:       defaultBackup().Result(),
			tarball:      newTarWriter(t).addItems("pods", builder.ForPod("ns-1", "pod-1").Result(), builder.ForPod("ns-2", "pod-2").Result()).done(),
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
		{
			name:         "when using a restore namespace filter, additional items that are in a non-included namespace are not restored",
			restore:      defaultRestore().IncludedNamespaces("ns-1").Result(),
			backup:       defaultBackup().Result(),
			tarball:      newTarWriter(t).addItems("pods", builder.ForPod("ns-1", "pod-1").Result(), builder.ForPod("ns-2", "pod-2").Result()).done(),
			apiResources: []*test.APIResource{test.Pods()},
			actions: []velero.RestoreItemAction{
				&pluggableAction{
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
				test.Pods(): {"ns-1/pod-1"},
			},
		},
		{
			name:    "when using a restore namespace filter, additional items that are cluster-scoped are restored when IncludeClusterResources=nil",
			restore: defaultRestore().IncludedNamespaces("ns-1").Result(),
			backup:  defaultBackup().Result(),
			tarball: newTarWriter(t).
				addItems("pods", builder.ForPod("ns-1", "pod-1").Result()).
				addItems("persistentvolumes", builder.ForPersistentVolume("pv-1").Result()).
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
		{
			name:    "additional items that are cluster-scoped are not restored when IncludeClusterResources=false",
			restore: defaultRestore().IncludeClusterResources(false).Result(),
			backup:  defaultBackup().Result(),
			tarball: newTarWriter(t).
				addItems("pods", builder.ForPod("ns-1", "pod-1").Result()).
				addItems("persistentvolumes", builder.ForPersistentVolume("pv-1").Result()).
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
				test.PVs():  nil,
			},
		},
		{
			name:    "when using a restore resource filter, additional items that are non-included resources are not restored",
			restore: defaultRestore().IncludedResources("pods").Result(),
			backup:  defaultBackup().Result(),
			tarball: newTarWriter(t).
				addItems("pods", builder.ForPod("ns-1", "pod-1").Result()).
				addItems("persistentvolumes", builder.ForPersistentVolume("pv-1").Result()).
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
				test.PVs():  nil,
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			h := newHarness(t)

			for _, r := range tc.apiResources {
				h.addItems(t, r)
			}

			data := Request{
				Log:              h.log,
				Restore:          tc.restore,
				Backup:           tc.backup,
				PodVolumeBackups: nil,
				VolumeSnapshots:  nil,
				BackupReader:     tc.tarball,
			}
			warnings, errs := h.restorer.Restore(
				data,
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
				test.PVs(
					builder.ForPersistentVolume("pv-1").ClaimRef("ns-1", "pvc-1").Result(),
				),
				test.PVCs(builder.ForPersistentVolumeClaim("ns-1", "pvc-1").Result()),
			},
			namespaces: []*corev1api.Namespace{builder.ForNamespace("ns-1").Result()},
			want:       false,
		},
		{
			name:   "when PV is found and has associated PVC that is deleting, result is false + timeout error",
			pvName: "pv-1",
			apiResources: []*test.APIResource{
				test.PVs(
					builder.ForPersistentVolume("pv-1").ClaimRef("ns-1", "pvc-1").Result(),
				),
				test.PVCs(
					builder.ForPersistentVolumeClaim("ns-1", "pvc-1").ObjectMeta(builder.WithDeletionTimestamp(time.Now())).Result(),
				),
			},
			want:    false,
			wantErr: errors.New("timed out waiting for the condition"),
		},
		{
			name:   "when PV is found, has associated PVC that's not deleting, has associated NS that is terminating, result is false + timeout error",
			pvName: "pv-1",
			apiResources: []*test.APIResource{
				test.PVs(
					builder.ForPersistentVolume("pv-1").ClaimRef("ns-1", "pvc-1").Result(),
				),
				test.PVCs(builder.ForPersistentVolumeClaim("ns-1", "pvc-1").Result()),
			},
			namespaces: []*corev1api.Namespace{
				builder.ForNamespace("ns-1").Phase(corev1api.NamespaceTerminating).Result(),
			},
			want:    false,
			wantErr: errors.New("timed out waiting for the condition"),
		},
		{
			name:   "when PV is found, has associated PVC that's not deleting, has associated NS that has deletion timestamp, result is false + timeout error",
			pvName: "pv-1",
			apiResources: []*test.APIResource{
				test.PVs(
					builder.ForPersistentVolume("pv-1").ClaimRef("ns-1", "pvc-1").Result(),
				),
				test.PVCs(builder.ForPersistentVolumeClaim("ns-1", "pvc-1").Result()),
			},
			namespaces: []*corev1api.Namespace{
				builder.ForNamespace("ns-1").ObjectMeta(builder.WithDeletionTimestamp(time.Now())).Result(),
			},
			want:    false,
			wantErr: errors.New("timed out waiting for the condition"),
		},
		{
			name:   "when PV is found, associated PVC is not found, result is false + timeout error",
			pvName: "pv-1",
			apiResources: []*test.APIResource{
				test.PVs(
					builder.ForPersistentVolume("pv-1").ClaimRef("ns-1", "pvc-1").Result(),
				),
			},
			want:    false,
			wantErr: errors.New("timed out waiting for the condition"),
		},
		{
			name:   "when PV is found, has associated PVC, associated namespace not found, result is false + timeout error",
			pvName: "pv-1",
			apiResources: []*test.APIResource{
				test.PVs(
					builder.ForPersistentVolume("pv-1").ClaimRef("ns-1", "pvc-1").Result(),
				),
				test.PVCs(builder.ForPersistentVolumeClaim("ns-1", "pvc-1").Result()),
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

// volumeSnapshotterGetter is a simple implementation of the VolumeSnapshotterGetter
// interface that returns velero.VolumeSnapshotters from a map if they exist.
type volumeSnapshotterGetter map[string]velero.VolumeSnapshotter

func (vsg volumeSnapshotterGetter) GetVolumeSnapshotter(name string) (velero.VolumeSnapshotter, error) {
	snapshotter, ok := vsg[name]
	if !ok {
		return nil, errors.New("volume snapshotter not found")
	}

	return snapshotter, nil
}

// volumeSnapshotter is a test fake for the velero.VolumeSnapshotter interface
type volumeSnapshotter struct {
	// a map from snapshotID to volumeID
	snapshotVolumes map[string]string
}

// Init is a no-op.
func (vs *volumeSnapshotter) Init(config map[string]string) error {
	return nil
}

// CreateVolumeFromSnapshot looks up the specified snapshotID in the snapshotVolumes
// map and returns the corresponding volumeID if it exists, or an error otherwise.
func (vs *volumeSnapshotter) CreateVolumeFromSnapshot(snapshotID, volumeType, volumeAZ string, iops *int64) (volumeID string, err error) {
	volumeID, ok := vs.snapshotVolumes[snapshotID]
	if !ok {
		return "", errors.New("snapshot not found")
	}

	return volumeID, nil
}

// SetVolumeID sets the persistent volume's spec.awsElasticBlockStore.volumeID field
// with the provided volumeID.
func (*volumeSnapshotter) SetVolumeID(pv runtime.Unstructured, volumeID string) (runtime.Unstructured, error) {
	unstructured.SetNestedField(pv.UnstructuredContent(), volumeID, "spec", "awsElasticBlockStore", "volumeID")
	return pv, nil
}

// GetVolumeID panics because it's not expected to be used for restores.
func (*volumeSnapshotter) GetVolumeID(pv runtime.Unstructured) (string, error) {
	panic("GetVolumeID should not be used for restores")
}

// CreateSnapshot panics because it's not expected to be used for restores.
func (*volumeSnapshotter) CreateSnapshot(volumeID, volumeAZ string, tags map[string]string) (snapshotID string, err error) {
	panic("CreateSnapshot should not be used for restores")
}

// GetVolumeInfo panics because it's not expected to be used for restores.
func (*volumeSnapshotter) GetVolumeInfo(volumeID, volumeAZ string) (string, *int64, error) {
	panic("GetVolumeInfo should not be used for restores")
}

// DeleteSnapshot panics because it's not expected to be used for restores.
func (*volumeSnapshotter) DeleteSnapshot(snapshotID string) error {
	panic("DeleteSnapshot should not be used for backups")
}

// TestRestorePersistentVolumes runs restores for persistent volumes and verifies that
// they are restored as expected, including restoring volumes from snapshots when expected.
// Verification is done by looking at the contents of the API and the metadata/spec/status of
// the items in the API.
func TestRestorePersistentVolumes(t *testing.T) {
	tests := []struct {
		name                    string
		restore                 *velerov1api.Restore
		backup                  *velerov1api.Backup
		tarball                 io.Reader
		apiResources            []*test.APIResource
		volumeSnapshots         []*volume.Snapshot
		volumeSnapshotLocations []*velerov1api.VolumeSnapshotLocation
		volumeSnapshotterGetter volumeSnapshotterGetter
		want                    []*test.APIResource
	}{
		{
			name:    "when a PV with a reclaim policy of delete has no snapshot and does not exist in-cluster, it does not get restored, and its PVC gets reset for dynamic provisioning",
			restore: defaultRestore().Result(),
			backup:  defaultBackup().Result(),
			tarball: newTarWriter(t).
				addItems("persistentvolumes",
					builder.ForPersistentVolume("pv-1").ReclaimPolicy(corev1api.PersistentVolumeReclaimDelete).ClaimRef("ns-1", "pvc-1").Result(),
				).
				addItems("persistentvolumeclaims",
					builder.ForPersistentVolumeClaim("ns-1", "pvc-1").
						VolumeName("pv-1").
						ObjectMeta(
							builder.WithAnnotations("pv.kubernetes.io/bind-completed", "true", "pv.kubernetes.io/bound-by-controller", "true", "foo", "bar"),
						).
						Result(),
				).
				done(),
			apiResources: []*test.APIResource{
				test.PVs(),
				test.PVCs(),
			},
			want: []*test.APIResource{
				test.PVs(),
				test.PVCs(
					builder.ForPersistentVolumeClaim("ns-1", "pvc-1").
						ObjectMeta(
							builder.WithAnnotations("foo", "bar"),
							builder.WithLabels("velero.io/backup-name", "backup-1", "velero.io/restore-name", "restore-1"),
						).
						Result(),
				),
			},
		},
		{
			name:    "when a PV with a reclaim policy of retain has no snapshot and does not exist in-cluster, it gets restored, without its claim ref",
			restore: defaultRestore().Result(),
			backup:  defaultBackup().Result(),
			tarball: newTarWriter(t).
				addItems("persistentvolumes",
					builder.ForPersistentVolume("pv-1").ReclaimPolicy(corev1api.PersistentVolumeReclaimRetain).ClaimRef("ns-1", "pvc-1").Result(),
				).
				done(),
			apiResources: []*test.APIResource{
				test.PVs(),
				test.PVCs(),
			},
			want: []*test.APIResource{
				test.PVs(
					builder.ForPersistentVolume("pv-1").
						ReclaimPolicy(corev1api.PersistentVolumeReclaimRetain).
						ObjectMeta(
							builder.WithLabels("velero.io/backup-name", "backup-1", "velero.io/restore-name", "restore-1"),
						).
						Result(),
				),
			},
		},
		{
			name:    "when a PV with a reclaim policy of delete has a snapshot and does not exist in-cluster, the snapshot and PV are restored",
			restore: defaultRestore().Result(),
			backup:  defaultBackup().Result(),
			tarball: newTarWriter(t).
				addItems("persistentvolumes",
					builder.ForPersistentVolume("pv-1").ReclaimPolicy(corev1api.PersistentVolumeReclaimDelete).AWSEBSVolumeID("old-volume").Result(),
				).
				done(),
			apiResources: []*test.APIResource{
				test.PVs(),
				test.PVCs(),
			},
			volumeSnapshots: []*volume.Snapshot{
				{
					Spec: volume.SnapshotSpec{
						BackupName:           "backup-1",
						Location:             "default",
						PersistentVolumeName: "pv-1",
					},
					Status: volume.SnapshotStatus{
						Phase:              volume.SnapshotPhaseCompleted,
						ProviderSnapshotID: "snapshot-1",
					},
				},
			},
			volumeSnapshotLocations: []*velerov1api.VolumeSnapshotLocation{
				builder.ForVolumeSnapshotLocation(velerov1api.DefaultNamespace, "default").Provider("provider-1").Result(),
			},
			volumeSnapshotterGetter: map[string]velero.VolumeSnapshotter{
				"provider-1": &volumeSnapshotter{
					snapshotVolumes: map[string]string{"snapshot-1": "new-volume"},
				},
			},
			want: []*test.APIResource{
				test.PVs(
					builder.ForPersistentVolume("pv-1").
						ReclaimPolicy(corev1api.PersistentVolumeReclaimDelete).
						AWSEBSVolumeID("new-volume").
						ObjectMeta(
							builder.WithLabels("velero.io/backup-name", "backup-1", "velero.io/restore-name", "restore-1"),
						).
						Result(),
				),
			},
		},
		{
			name:    "when a PV with a reclaim policy of retain has a snapshot and does not exist in-cluster, the snapshot and PV are restored",
			restore: defaultRestore().Result(),
			backup:  defaultBackup().Result(),
			tarball: newTarWriter(t).
				addItems("persistentvolumes",
					builder.ForPersistentVolume("pv-1").
						ReclaimPolicy(corev1api.PersistentVolumeReclaimRetain).
						AWSEBSVolumeID("old-volume").
						Result(),
				).
				done(),
			apiResources: []*test.APIResource{
				test.PVs(),
				test.PVCs(),
			},
			volumeSnapshots: []*volume.Snapshot{
				{
					Spec: volume.SnapshotSpec{
						BackupName:           "backup-1",
						Location:             "default",
						PersistentVolumeName: "pv-1",
					},
					Status: volume.SnapshotStatus{
						Phase:              volume.SnapshotPhaseCompleted,
						ProviderSnapshotID: "snapshot-1",
					},
				},
			},
			volumeSnapshotLocations: []*velerov1api.VolumeSnapshotLocation{
				builder.ForVolumeSnapshotLocation(velerov1api.DefaultNamespace, "default").Provider("provider-1").Result(),
			},
			volumeSnapshotterGetter: map[string]velero.VolumeSnapshotter{
				"provider-1": &volumeSnapshotter{
					snapshotVolumes: map[string]string{"snapshot-1": "new-volume"},
				},
			},
			want: []*test.APIResource{
				test.PVs(
					builder.ForPersistentVolume("pv-1").
						ReclaimPolicy(corev1api.PersistentVolumeReclaimRetain).
						AWSEBSVolumeID("new-volume").
						ObjectMeta(
							builder.WithLabels("velero.io/backup-name", "backup-1", "velero.io/restore-name", "restore-1"),
						).
						Result(),
				),
			},
		},
		{
			name:    "when a PV with a reclaim policy of delete has a snapshot and exists in-cluster, neither the snapshot nor the PV are restored",
			restore: defaultRestore().Result(),
			backup:  defaultBackup().Result(),
			tarball: newTarWriter(t).
				addItems("persistentvolumes",
					builder.ForPersistentVolume("pv-1").
						ReclaimPolicy(corev1api.PersistentVolumeReclaimDelete).
						AWSEBSVolumeID("old-volume").
						Result(),
				).
				done(),
			apiResources: []*test.APIResource{
				test.PVs(
					builder.ForPersistentVolume("pv-1").
						ReclaimPolicy(corev1api.PersistentVolumeReclaimDelete).
						AWSEBSVolumeID("old-volume").
						Result(),
				),
				test.PVCs(),
			},
			volumeSnapshots: []*volume.Snapshot{
				{
					Spec: volume.SnapshotSpec{
						BackupName:           "backup-1",
						Location:             "default",
						PersistentVolumeName: "pv-1",
					},
					Status: volume.SnapshotStatus{
						Phase:              volume.SnapshotPhaseCompleted,
						ProviderSnapshotID: "snapshot-1",
					},
				},
			},
			volumeSnapshotLocations: []*velerov1api.VolumeSnapshotLocation{
				builder.ForVolumeSnapshotLocation(velerov1api.DefaultNamespace, "default").Provider("provider-1").Result(),
			},
			volumeSnapshotterGetter: map[string]velero.VolumeSnapshotter{
				// the volume snapshotter fake is not configured with any snapshotID -> volumeID
				// mappings as a way to verify that the snapshot is not restored, since if it were
				// restored, we'd get an error of "snapshot not found".
				"provider-1": &volumeSnapshotter{},
			},
			want: []*test.APIResource{
				test.PVs(
					builder.ForPersistentVolume("pv-1").
						ReclaimPolicy(corev1api.PersistentVolumeReclaimDelete).
						AWSEBSVolumeID("old-volume").
						Result(),
				),
			},
		},
		{
			name:    "when a PV with a reclaim policy of retain has a snapshot and exists in-cluster, neither the snapshot nor the PV are restored",
			restore: defaultRestore().Result(),
			backup:  defaultBackup().Result(),
			tarball: newTarWriter(t).
				addItems("persistentvolumes",
					builder.ForPersistentVolume("pv-1").
						ReclaimPolicy(corev1api.PersistentVolumeReclaimRetain).
						AWSEBSVolumeID("old-volume").
						Result(),
				).
				done(),
			apiResources: []*test.APIResource{
				test.PVs(
					builder.ForPersistentVolume("pv-1").
						ReclaimPolicy(corev1api.PersistentVolumeReclaimRetain).
						AWSEBSVolumeID("old-volume").
						Result(),
				),
				test.PVCs(),
			},
			volumeSnapshots: []*volume.Snapshot{
				{
					Spec: volume.SnapshotSpec{
						BackupName:           "backup-1",
						Location:             "default",
						PersistentVolumeName: "pv-1",
					},
					Status: volume.SnapshotStatus{
						Phase:              volume.SnapshotPhaseCompleted,
						ProviderSnapshotID: "snapshot-1",
					},
				},
			},
			volumeSnapshotLocations: []*velerov1api.VolumeSnapshotLocation{
				builder.ForVolumeSnapshotLocation(velerov1api.DefaultNamespace, "default").Provider("provider-1").Result(),
			},
			volumeSnapshotterGetter: map[string]velero.VolumeSnapshotter{
				// the volume snapshotter fake is not configured with any snapshotID -> volumeID
				// mappings as a way to verify that the snapshot is not restored, since if it were
				// restored, we'd get an error of "snapshot not found".
				"provider-1": &volumeSnapshotter{},
			},
			want: []*test.APIResource{
				test.PVs(
					builder.ForPersistentVolume("pv-1").
						ReclaimPolicy(corev1api.PersistentVolumeReclaimRetain).
						AWSEBSVolumeID("old-volume").
						Result(),
				),
			},
		},
		{
			name:    "when a PV with a snapshot is used by a PVC in a namespace that's being remapped, and the original PV exists in-cluster, the PV is renamed",
			restore: defaultRestore().NamespaceMappings("source-ns", "target-ns").Result(),
			backup:  defaultBackup().Result(),
			tarball: newTarWriter(t).
				addItems(
					"persistentvolumes",
					builder.ForPersistentVolume("source-pv").AWSEBSVolumeID("source-volume").ClaimRef("source-ns", "pvc-1").Result(),
				).
				addItems(
					"persistentvolumeclaims",
					builder.ForPersistentVolumeClaim("source-ns", "pvc-1").VolumeName("source-pv").Result(),
				).
				done(),
			apiResources: []*test.APIResource{
				test.PVs(
					builder.ForPersistentVolume("source-pv").AWSEBSVolumeID("source-volume").ClaimRef("source-ns", "pvc-1").Result(),
				),
				test.PVCs(),
			},
			volumeSnapshots: []*volume.Snapshot{
				{
					Spec: volume.SnapshotSpec{
						BackupName:           "backup-1",
						Location:             "default",
						PersistentVolumeName: "source-pv",
					},
					Status: volume.SnapshotStatus{
						Phase:              volume.SnapshotPhaseCompleted,
						ProviderSnapshotID: "snapshot-1",
					},
				},
			},
			volumeSnapshotLocations: []*velerov1api.VolumeSnapshotLocation{
				builder.ForVolumeSnapshotLocation(velerov1api.DefaultNamespace, "default").Provider("provider-1").Result(),
			},
			volumeSnapshotterGetter: map[string]velero.VolumeSnapshotter{
				"provider-1": &volumeSnapshotter{
					snapshotVolumes: map[string]string{"snapshot-1": "new-volume"},
				},
			},
			want: []*test.APIResource{
				test.PVs(
					builder.ForPersistentVolume("source-pv").AWSEBSVolumeID("source-volume").ClaimRef("source-ns", "pvc-1").Result(),
					// note that the renamed PV is not expected to have a claimRef in this test; that would be
					// added after creation by the Kubernetes PV/PVC controller when it does a bind.
					builder.ForPersistentVolume("renamed-source-pv").
						ObjectMeta(
							builder.WithAnnotations("velero.io/original-pv-name", "source-pv"),
							builder.WithLabels("velero.io/backup-name", "backup-1", "velero.io/restore-name", "restore-1"),
						).
						AWSEBSVolumeID("new-volume").
						Result(),
				),
				test.PVCs(
					builder.ForPersistentVolumeClaim("target-ns", "pvc-1").
						ObjectMeta(
							builder.WithLabels("velero.io/backup-name", "backup-1", "velero.io/restore-name", "restore-1"),
						).
						VolumeName("renamed-source-pv").
						Result(),
				),
			},
		},
		{
			name:    "when a PV with a snapshot is used by a PVC in a namespace that's being remapped, and the original PV does not exist in-cluster, the PV is not renamed",
			restore: defaultRestore().NamespaceMappings("source-ns", "target-ns").Result(),
			backup:  defaultBackup().Result(),
			tarball: newTarWriter(t).
				addItems(
					"persistentvolumes",
					builder.ForPersistentVolume("source-pv").AWSEBSVolumeID("source-volume").ClaimRef("source-ns", "pvc-1").Result(),
				).
				addItems(
					"persistentvolumeclaims",
					builder.ForPersistentVolumeClaim("source-ns", "pvc-1").VolumeName("source-pv").Result(),
				).
				done(),
			apiResources: []*test.APIResource{
				test.PVs(),
				test.PVCs(),
			},
			volumeSnapshots: []*volume.Snapshot{
				{
					Spec: volume.SnapshotSpec{
						BackupName:           "backup-1",
						Location:             "default",
						PersistentVolumeName: "source-pv",
					},
					Status: volume.SnapshotStatus{
						Phase:              volume.SnapshotPhaseCompleted,
						ProviderSnapshotID: "snapshot-1",
					},
				},
			},
			volumeSnapshotLocations: []*velerov1api.VolumeSnapshotLocation{
				builder.ForVolumeSnapshotLocation(velerov1api.DefaultNamespace, "default").Provider("provider-1").Result(),
			},
			volumeSnapshotterGetter: map[string]velero.VolumeSnapshotter{
				"provider-1": &volumeSnapshotter{
					snapshotVolumes: map[string]string{"snapshot-1": "new-volume"},
				},
			},
			want: []*test.APIResource{
				test.PVs(
					builder.ForPersistentVolume("source-pv").
						ObjectMeta(
							builder.WithLabels("velero.io/backup-name", "backup-1", "velero.io/restore-name", "restore-1"),
						).
						AWSEBSVolumeID("new-volume").
						Result(),
				),
				test.PVCs(
					builder.ForPersistentVolumeClaim("target-ns", "pvc-1").
						ObjectMeta(
							builder.WithLabels("velero.io/backup-name", "backup-1", "velero.io/restore-name", "restore-1"),
						).
						VolumeName("source-pv").
						Result(),
				),
			},
		},
		{
			name:    "when a PV with a reclaim policy of retain has a snapshot and exists in-cluster, neither the snapshot nor the PV are restored",
			restore: defaultRestore().Result(),
			backup:  defaultBackup().Result(),
			tarball: newTarWriter(t).
				addItems("persistentvolumes",
					builder.ForPersistentVolume("pv-1").
						ReclaimPolicy(corev1api.PersistentVolumeReclaimRetain).
						AWSEBSVolumeID("old-volume").
						Result(),
				).
				done(),
			apiResources: []*test.APIResource{
				test.PVs(
					builder.ForPersistentVolume("pv-1").
						ReclaimPolicy(corev1api.PersistentVolumeReclaimRetain).
						AWSEBSVolumeID("old-volume").
						Result(),
				),
				test.PVCs(),
			},
			volumeSnapshots: []*volume.Snapshot{
				{
					Spec: volume.SnapshotSpec{
						BackupName:           "backup-1",
						Location:             "default",
						PersistentVolumeName: "pv-1",
					},
					Status: volume.SnapshotStatus{
						Phase:              volume.SnapshotPhaseCompleted,
						ProviderSnapshotID: "snapshot-1",
					},
				},
			},
			volumeSnapshotLocations: []*velerov1api.VolumeSnapshotLocation{
				{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: velerov1api.DefaultNamespace,
						Name:      "default",
					},
					Spec: velerov1api.VolumeSnapshotLocationSpec{
						Provider: "provider-1",
					},
				},
			},
			volumeSnapshotterGetter: map[string]velero.VolumeSnapshotter{
				// the volume snapshotter fake is not configured with any snapshotID -> volumeID
				// mappings as a way to verify that the snapshot is not restored, since if it were
				// restored, we'd get an error of "snapshot not found".
				"provider-1": &volumeSnapshotter{},
			},

			want: []*test.APIResource{
				test.PVs(
					builder.ForPersistentVolume("pv-1").
						ReclaimPolicy(corev1api.PersistentVolumeReclaimRetain).
						AWSEBSVolumeID("old-volume").
						Result(),
				),
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			h := newHarness(t)
			h.restorer.resourcePriorities = []string{"persistentvolumes", "persistentvolumeclaims"}
			h.restorer.pvRenamer = func(oldName string) (string, error) {
				renamed := "renamed-" + oldName
				return renamed, nil
			}

			// set up the VolumeSnapshotLocation informer/lister and add test data to it
			vslInformer := velerov1informers.NewSharedInformerFactory(h.VeleroClient, 0).Velero().V1().VolumeSnapshotLocations()
			for _, vsl := range tc.volumeSnapshotLocations {
				require.NoError(t, vslInformer.Informer().GetStore().Add(vsl))
			}

			for _, r := range tc.apiResources {
				h.addItems(t, r)
			}

			// Collect the IDs of all of the wanted resources so we can ensure the
			// exact set exists in the API after restore.
			wantIDs := make(map[*test.APIResource][]string)
			for i, resource := range tc.want {
				wantIDs[tc.want[i]] = []string{}

				for _, item := range resource.Items {
					wantIDs[tc.want[i]] = append(wantIDs[tc.want[i]], fmt.Sprintf("%s/%s", item.GetNamespace(), item.GetName()))
				}
			}

			data := Request{
				Log:             h.log,
				Restore:         tc.restore,
				Backup:          tc.backup,
				VolumeSnapshots: tc.volumeSnapshots,
				BackupReader:    tc.tarball,
			}
			warnings, errs := h.restorer.Restore(
				data,
				nil, // actions
				vslInformer.Lister(),
				tc.volumeSnapshotterGetter,
			)

			assertEmptyResults(t, warnings, errs)
			assertAPIContents(t, h, wantIDs)
			assertRestoredItems(t, h, tc.want)
		})
	}
}

type fakeResticRestorerFactory struct {
	restorer *resticmocks.Restorer
}

func (f *fakeResticRestorerFactory) NewRestorer(ctx.Context, *velerov1api.Restore) (restic.Restorer, error) {
	return f.restorer, nil
}

// TestRestoreWithRestic verifies that a call to RestorePodVolumes was made as and when
// expected for the given pods by using a mock for the restic restorer.
func TestRestoreWithRestic(t *testing.T) {
	tests := []struct {
		name                        string
		restore                     *velerov1api.Restore
		backup                      *velerov1api.Backup
		apiResources                []*test.APIResource
		podVolumeBackups            []*velerov1api.PodVolumeBackup
		podWithPVBs, podWithoutPVBs []*corev1api.Pod
		want                        map[*test.APIResource][]string
	}{
		{
			name:         "a pod that exists in given backup and contains associated PVBs should have should have RestorePodVolumes called",
			restore:      defaultRestore().Result(),
			backup:       defaultBackup().Result(),
			apiResources: []*test.APIResource{test.Pods()},
			podVolumeBackups: []*velerov1api.PodVolumeBackup{
				builder.ForPodVolumeBackup("velero", "pvb-1").PodName("pod-1").SnapshotID("foo").Result(),
				builder.ForPodVolumeBackup("velero", "pvb-2").PodName("pod-2").SnapshotID("foo").Result(),
				builder.ForPodVolumeBackup("velero", "pvb-3").PodName("pod-4").SnapshotID("foo").Result(),
			},
			podWithPVBs: []*corev1api.Pod{
				builder.ForPod("ns-1", "pod-2").
					Result(),
				builder.ForPod("ns-2", "pod-4").
					Result(),
			},
			podWithoutPVBs: []*corev1api.Pod{
				builder.ForPod("ns-2", "pod-3").
					Result(),
			},
			want: map[*test.APIResource][]string{
				test.Pods(): {"ns-1/pod-2", "ns-2/pod-3", "ns-2/pod-4"},
			},
		},
		{
			name:         "a pod that exists in given backup but does not contain associated PVBs should not have should have RestorePodVolumes called",
			restore:      defaultRestore().Result(),
			backup:       defaultBackup().Result(),
			apiResources: []*test.APIResource{test.Pods()},
			podVolumeBackups: []*velerov1api.PodVolumeBackup{
				builder.ForPodVolumeBackup("velero", "pvb-1").PodName("pod-1").Result(),
				builder.ForPodVolumeBackup("velero", "pvb-2").PodName("pod-2").Result(),
			},
			podWithPVBs: []*corev1api.Pod{},
			podWithoutPVBs: []*corev1api.Pod{
				builder.ForPod("ns-1", "pod-3").
					Result(),
				builder.ForPod("ns-2", "pod-4").
					Result(),
			},
			want: map[*test.APIResource][]string{
				test.Pods(): {"ns-1/pod-3", "ns-2/pod-4"},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			h := newHarness(t)
			restorer := new(resticmocks.Restorer)
			defer restorer.AssertExpectations(t)
			h.restorer.resticRestorerFactory = &fakeResticRestorerFactory{
				restorer: restorer,
			}

			// needed only to indicate resource types that can be restored, in this case, pods
			for _, resource := range tc.apiResources {
				h.addItems(t, resource)
			}

			tarball := newTarWriter(t)

			// these backed up pods don't have any PVBs associated with them, so a call to RestorePodVolumes is not expected to be made for them
			for _, pod := range tc.podWithoutPVBs {
				tarball.addItems("pods", pod)
			}

			// these backed up pods have PVBs associated with them, so a call to RestorePodVolumes will be made for each of them
			for _, pod := range tc.podWithPVBs {
				tarball.addItems("pods", pod)

				// the restore process adds these labels before restoring, so we must add them here too otherwise they won't match
				pod.Labels = map[string]string{"velero.io/backup-name": tc.backup.Name, "velero.io/restore-name": tc.restore.Name}
				expectedArgs := restic.RestoreData{
					Restore:          tc.restore,
					Pod:              pod,
					PodVolumeBackups: tc.podVolumeBackups,
					SourceNamespace:  pod.Namespace,
					BackupLocation:   "",
				}
				restorer.
					On("RestorePodVolumes", expectedArgs).
					Return(nil)
			}

			data := Request{
				Log:              h.log,
				Restore:          tc.restore,
				Backup:           tc.backup,
				PodVolumeBackups: tc.podVolumeBackups,
				BackupReader:     tarball.done(),
			}

			warnings, errs := h.restorer.Restore(
				data,
				nil, // actions
				nil, // snapshot location lister
				nil, // volume snapshotter getter
			)

			assertEmptyResults(t, warnings, errs)
			assertAPIContents(t, h, tc.want)
		})
	}
}

func TestPrioritizeResources(t *testing.T) {
	tests := []struct {
		name         string
		apiResources map[string][]string
		priorities   []string
		includes     []string
		excludes     []string
		expected     []string
	}{
		{
			name: "priorities & ordering are correctly applied",
			apiResources: map[string][]string{
				"v1": {"aaa", "bbb", "configmaps", "ddd", "namespaces", "ooo", "pods", "sss"},
			},
			priorities: []string{"namespaces", "configmaps", "pods"},
			includes:   []string{"*"},
			expected:   []string{"namespaces", "configmaps", "pods", "aaa", "bbb", "ddd", "ooo", "sss"},
		},
		{
			name: "includes are correctly applied",
			apiResources: map[string][]string{
				"v1": {"aaa", "bbb", "configmaps", "ddd", "namespaces", "ooo", "pods", "sss"},
			},
			priorities: []string{"namespaces", "configmaps", "pods"},
			includes:   []string{"namespaces", "aaa", "sss"},
			expected:   []string{"namespaces", "aaa", "sss"},
		},
		{
			name: "excludes are correctly applied",
			apiResources: map[string][]string{
				"v1": {"aaa", "bbb", "configmaps", "ddd", "namespaces", "ooo", "pods", "sss"},
			},
			priorities: []string{"namespaces", "configmaps", "pods"},
			includes:   []string{"*"},
			excludes:   []string{"ooo", "pods"},
			expected:   []string{"namespaces", "configmaps", "aaa", "bbb", "ddd", "sss"},
		},
	}

	logger := testutil.NewLogger()

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			discoveryClient := &test.DiscoveryClient{
				FakeDiscovery: kubefake.NewSimpleClientset().Discovery().(*discoveryfake.FakeDiscovery),
			}

			helper, err := discovery.NewHelper(discoveryClient, logger)
			require.NoError(t, err)

			// add all the test case's API resources to the discovery client
			for gvString, resources := range tc.apiResources {
				gv, err := schema.ParseGroupVersion(gvString)
				require.NoError(t, err)

				for _, resource := range resources {
					discoveryClient.WithAPIResource(&test.APIResource{
						Group:   gv.Group,
						Version: gv.Version,
						Name:    resource,
					})
				}
			}

			require.NoError(t, helper.Refresh())

			includesExcludes := collections.NewIncludesExcludes().Includes(tc.includes...).Excludes(tc.excludes...)

			result, err := prioritizeResources(helper, tc.priorities, includesExcludes, logger)
			require.NoError(t, err)

			require.Equal(t, len(tc.expected), len(result))

			for i := range result {
				if e, a := tc.expected[i], result[i].Resource; e != a {
					t.Errorf("index %d, expected %s, got %s", i, e, a)
				}
			}
		})
	}
}

func TestResetMetadataAndStatus(t *testing.T) {
	tests := []struct {
		name        string
		obj         *unstructured.Unstructured
		expectedErr bool
		expectedRes *unstructured.Unstructured
	}{
		{
			name:        "no metadata causes error",
			obj:         &unstructured.Unstructured{},
			expectedErr: true,
		},
		{
			name:        "keep name, namespace, labels, annotations only",
			obj:         NewTestUnstructured().WithMetadata("name", "blah", "namespace", "labels", "annotations", "foo").Unstructured,
			expectedErr: false,
			expectedRes: NewTestUnstructured().WithMetadata("name", "namespace", "labels", "annotations").Unstructured,
		},
		{
			name:        "don't keep status",
			obj:         NewTestUnstructured().WithMetadata().WithStatus().Unstructured,
			expectedErr: false,
			expectedRes: NewTestUnstructured().WithMetadata().Unstructured,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			res, err := resetMetadataAndStatus(test.obj)

			if assert.Equal(t, test.expectedErr, err != nil) {
				assert.Equal(t, test.expectedRes, res)
			}
		})
	}
}

func TestIsCompleted(t *testing.T) {
	tests := []struct {
		name          string
		expected      bool
		content       string
		groupResource schema.GroupResource
		expectedErr   bool
	}{
		{
			name:          "Failed pods are complete",
			expected:      true,
			content:       `{"apiVersion":"v1","kind":"Pod","metadata":{"namespace":"ns","name":"pod1"}, "status": {"phase": "Failed"}}`,
			groupResource: schema.GroupResource{Group: "", Resource: "pods"},
		},
		{
			name:          "Succeeded pods are complete",
			expected:      true,
			content:       `{"apiVersion":"v1","kind":"Pod","metadata":{"namespace":"ns","name":"pod1"}, "status": {"phase": "Succeeded"}}`,
			groupResource: schema.GroupResource{Group: "", Resource: "pods"},
		},
		{
			name:          "Pending pods aren't complete",
			expected:      false,
			content:       `{"apiVersion":"v1","kind":"Pod","metadata":{"namespace":"ns","name":"pod1"}, "status": {"phase": "Pending"}}`,
			groupResource: schema.GroupResource{Group: "", Resource: "pods"},
		},
		{
			name:          "Running pods aren't complete",
			expected:      false,
			content:       `{"apiVersion":"v1","kind":"Pod","metadata":{"namespace":"ns","name":"pod1"}, "status": {"phase": "Running"}}`,
			groupResource: schema.GroupResource{Group: "", Resource: "pods"},
		},
		{
			name:          "Jobs without a completion time aren't complete",
			expected:      false,
			content:       `{"apiVersion":"v1","kind":"Pod","metadata":{"namespace":"ns","name":"pod1"}}`,
			groupResource: schema.GroupResource{Group: "batch", Resource: "jobs"},
		},
		{
			name:          "Jobs with a completion time are completed",
			expected:      true,
			content:       `{"apiVersion":"v1","kind":"Pod","metadata":{"namespace":"ns","name":"pod1"}, "status": {"completionTime": "bar"}}`,
			groupResource: schema.GroupResource{Group: "batch", Resource: "jobs"},
		},
		{
			name:          "Jobs with an empty completion time are not completed",
			expected:      false,
			content:       `{"apiVersion":"v1","kind":"Pod","metadata":{"namespace":"ns","name":"pod1"}, "status": {"completionTime": ""}}`,
			groupResource: schema.GroupResource{Group: "batch", Resource: "jobs"},
		},
		{
			name:          "Something not a pod or a job may actually be complete, but we're not concerned with that",
			expected:      false,
			content:       `{"apiVersion": "v1", "kind": "Namespace", "metadata": {"name": "ns"}, "status": {"completionTime": "bar", "phase":"Completed"}}`,
			groupResource: schema.GroupResource{Group: "", Resource: "namespaces"},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			u := testutil.UnstructuredOrDie(test.content)
			backup, err := isCompleted(u, test.groupResource)

			if assert.Equal(t, test.expectedErr, err != nil) {
				assert.Equal(t, test.expected, backup)
			}
		})
	}
}

func TestGetItemFilePath(t *testing.T) {
	res := getItemFilePath("root", "resource", "", "item")
	assert.Equal(t, "root/resources/resource/cluster/item.json", res)

	res = getItemFilePath("root", "resource", "namespace", "item")
	assert.Equal(t, "root/resources/resource/namespaces/namespace/item.json", res)
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

func defaultRestore() *builder.RestoreBuilder {
	return builder.ForRestore(velerov1api.DefaultNamespace, "restore-1").Backup("backup-1")
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
