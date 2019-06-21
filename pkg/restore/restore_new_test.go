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
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1api "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/sets"

	velerov1api "github.com/heptio/velero/pkg/apis/velero/v1"
	"github.com/heptio/velero/pkg/client"
	"github.com/heptio/velero/pkg/discovery"
	"github.com/heptio/velero/pkg/test"
	"github.com/heptio/velero/pkg/util/encode"
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

func defaultRestore() *Builder {
	return NewNamedBuilder(velerov1api.DefaultNamespace, "restore-1").Backup("backup-1")
}

// assertAPIContents asserts that the dynamic client on the provided harness contains
// all of the items specified in 'want' (a map from an APIResource definition to a slice
// of resource identifiers, formatted as <namespace>/<name>).
func assertAPIContents(t *testing.T, h *harness, want map[*test.APIResource][]string) {
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

func (tw *tarWriter) addItems(groupVersion string, items ...metav1.Object) *tarWriter {
	tw.t.Helper()

	for _, obj := range items {

		var path string
		if obj.GetNamespace() == "" {
			path = fmt.Sprintf("resources/%s/cluster/%s.json", groupVersion, obj.GetName())
		} else {
			path = fmt.Sprintf("resources/%s/namespaces/%s/%s.json", groupVersion, obj.GetNamespace(), obj.GetName())
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
