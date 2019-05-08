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

package backup

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"io"
	"sort"
	"strings"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	discoveryfake "k8s.io/client-go/discovery/fake"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	kubefake "k8s.io/client-go/kubernetes/fake"

	velerov1 "github.com/heptio/velero/pkg/apis/velero/v1"
	"github.com/heptio/velero/pkg/client"
	"github.com/heptio/velero/pkg/discovery"
	"github.com/heptio/velero/pkg/generated/clientset/versioned/fake"
	"github.com/heptio/velero/pkg/test"
)

func TestBackupResourceFiltering(t *testing.T) {
	tests := []struct {
		name         string
		backup       *velerov1.Backup
		apiResources []*apiResource
		want         []string
	}{
		{
			name:   "no filters backs up everything",
			backup: defaultBackup().Build(),
			apiResources: []*apiResource{
				pods(
					newPod("foo", "bar"),
					newPod("zoo", "raz"),
				),
				deployments(
					newDeployment("foo", "bar"),
					newDeployment("zoo", "raz"),
				),
			},
			want: []string{
				"resources/pods/namespaces/foo/bar.json",
				"resources/pods/namespaces/zoo/raz.json",
				"resources/deployments.apps/namespaces/foo/bar.json",
				"resources/deployments.apps/namespaces/zoo/raz.json",
			},
		},
		{
			name: "included resources filter only backs up resources of those type",
			backup: defaultBackup().
				IncludedResources("pods").
				Build(),
			apiResources: []*apiResource{
				pods(
					newPod("foo", "bar"),
					newPod("zoo", "raz"),
				),
				deployments(
					newDeployment("foo", "bar"),
					newDeployment("zoo", "raz"),
				),
			},
			want: []string{
				"resources/pods/namespaces/foo/bar.json",
				"resources/pods/namespaces/zoo/raz.json",
			},
		},
		{
			name: "excluded resources filter only backs up resources not of those type",
			backup: defaultBackup().
				ExcludedResources("deployments").
				Build(),
			apiResources: []*apiResource{
				pods(
					newPod("foo", "bar"),
					newPod("zoo", "raz"),
				),
				deployments(
					newDeployment("foo", "bar"),
					newDeployment("zoo", "raz"),
				),
			},
			want: []string{
				"resources/pods/namespaces/foo/bar.json",
				"resources/pods/namespaces/zoo/raz.json",
			},
		},
		{
			name: "included namespaces filter only backs up resources in those namespaces",
			backup: defaultBackup().
				IncludedNamespaces("foo").
				Build(),
			apiResources: []*apiResource{
				pods(
					newPod("foo", "bar"),
					newPod("zoo", "raz"),
				),
				deployments(
					newDeployment("foo", "bar"),
					newDeployment("zoo", "raz"),
				),
			},
			want: []string{
				"resources/pods/namespaces/foo/bar.json",
				"resources/deployments.apps/namespaces/foo/bar.json",
			},
		},
		{
			name: "excluded namespaces filter only backs up resources not in those namespaces",
			backup: defaultBackup().
				ExcludedNamespaces("zoo").
				Build(),
			apiResources: []*apiResource{
				pods(
					newPod("foo", "bar"),
					newPod("zoo", "raz"),
				),
				deployments(
					newDeployment("foo", "bar"),
					newDeployment("zoo", "raz"),
				),
			},
			want: []string{
				"resources/pods/namespaces/foo/bar.json",
				"resources/deployments.apps/namespaces/foo/bar.json",
			},
		},
		{
			name: "IncludeClusterResources=false only backs up namespaced resources",
			backup: defaultBackup().
				IncludeClusterResources(false).
				Build(),
			apiResources: []*apiResource{
				pods(
					newPod("foo", "bar"),
					newPod("zoo", "raz"),
				),
				deployments(
					newDeployment("foo", "bar"),
					newDeployment("zoo", "raz"),
				),
				pvs(
					newPV("bar"),
					newPV("baz"),
				),
			},
			want: []string{
				"resources/pods/namespaces/foo/bar.json",
				"resources/pods/namespaces/zoo/raz.json",
				"resources/deployments.apps/namespaces/foo/bar.json",
				"resources/deployments.apps/namespaces/zoo/raz.json",
			},
		},
		{
			name: "label selector only backs up matching resources",
			backup: defaultBackup().
				LabelSelector(&metav1.LabelSelector{MatchLabels: map[string]string{"a": "b"}}).
				Build(),
			apiResources: []*apiResource{
				pods(
					withLabel(newPod("foo", "bar"), "a", "b"),
					newPod("zoo", "raz"),
				),
				deployments(
					newDeployment("foo", "bar"),
					withLabel(newDeployment("zoo", "raz"), "a", "b"),
				),
				pvs(
					withLabel(newPV("bar"), "a", "b"),
					withLabel(newPV("baz"), "a", "c"),
				),
			},
			want: []string{
				"resources/pods/namespaces/foo/bar.json",
				"resources/deployments.apps/namespaces/zoo/raz.json",
				"resources/persistentvolumes/cluster/bar.json",
			},
		},
		{
			name: "should include cluster-scoped resources if backing up subset of namespaces and IncludeClusterResources=true",
			backup: defaultBackup().
				IncludedNamespaces("ns-1", "ns-2").
				IncludeClusterResources(true).
				Build(),
			apiResources: []*apiResource{
				pods(
					newPod("ns-1", "pod-1"),
					newPod("ns-2", "pod-1"),
					newPod("ns-3", "pod-1"),
				),
				pvs(
					newPV("pv-1"),
					newPV("pv-2"),
				),
			},
			want: []string{
				"resources/pods/namespaces/ns-1/pod-1.json",
				"resources/pods/namespaces/ns-2/pod-1.json",
				"resources/persistentvolumes/cluster/pv-1.json",
				"resources/persistentvolumes/cluster/pv-2.json",
			},
		},
		{
			name: "should not include cluster-scoped resource if backing up subset of namespaces and IncludeClusterResources=false",
			backup: defaultBackup().
				IncludedNamespaces("ns-1", "ns-2").
				IncludeClusterResources(false).
				Build(),
			apiResources: []*apiResource{
				pods(
					newPod("ns-1", "pod-1"),
					newPod("ns-2", "pod-1"),
					newPod("ns-3", "pod-1"),
				),
				pvs(
					newPV("pv-1"),
					newPV("pv-2"),
				),
			},
			want: []string{
				"resources/pods/namespaces/ns-1/pod-1.json",
				"resources/pods/namespaces/ns-2/pod-1.json",
			},
		},
		{
			name: "should not include cluster-scoped resource if backing up subset of namespaces and IncludeClusterResources=nil",
			backup: defaultBackup().
				IncludedNamespaces("ns-1", "ns-2").
				Build(),
			apiResources: []*apiResource{
				pods(
					newPod("ns-1", "pod-1"),
					newPod("ns-2", "pod-1"),
					newPod("ns-3", "pod-1"),
				),
				pvs(
					newPV("pv-1"),
					newPV("pv-2"),
				),
			},
			want: []string{
				"resources/pods/namespaces/ns-1/pod-1.json",
				"resources/pods/namespaces/ns-2/pod-1.json",
			},
		},
		{
			name: "should include cluster-scoped resources if backing up all namespaces and IncludeClusterResources=true",
			backup: defaultBackup().
				IncludeClusterResources(true).
				Build(),
			apiResources: []*apiResource{
				pods(
					newPod("ns-1", "pod-1"),
					newPod("ns-2", "pod-1"),
					newPod("ns-3", "pod-1"),
				),
				pvs(
					newPV("pv-1"),
					newPV("pv-2"),
				),
			},
			want: []string{
				"resources/pods/namespaces/ns-1/pod-1.json",
				"resources/pods/namespaces/ns-2/pod-1.json",
				"resources/pods/namespaces/ns-3/pod-1.json",
				"resources/persistentvolumes/cluster/pv-1.json",
				"resources/persistentvolumes/cluster/pv-2.json",
			},
		},
		{
			name: "should not include cluster-scoped resources if backing up all namespaces and IncludeClusterResources=false",
			backup: defaultBackup().
				IncludeClusterResources(false).
				Build(),
			apiResources: []*apiResource{
				pods(
					newPod("ns-1", "pod-1"),
					newPod("ns-2", "pod-1"),
					newPod("ns-3", "pod-1"),
				),
				pvs(
					newPV("pv-1"),
					newPV("pv-2"),
				),
			},
			want: []string{
				"resources/pods/namespaces/ns-1/pod-1.json",
				"resources/pods/namespaces/ns-2/pod-1.json",
				"resources/pods/namespaces/ns-3/pod-1.json",
			},
		},
		{
			name: "should include cluster-scoped resources if backing up all namespaces and IncludeClusterResources=nil",
			backup: defaultBackup().
				Build(),
			apiResources: []*apiResource{
				pods(
					newPod("ns-1", "pod-1"),
					newPod("ns-2", "pod-1"),
					newPod("ns-3", "pod-1"),
				),
				pvs(
					newPV("pv-1"),
					newPV("pv-2"),
				),
			},
			want: []string{
				"resources/pods/namespaces/ns-1/pod-1.json",
				"resources/pods/namespaces/ns-2/pod-1.json",
				"resources/pods/namespaces/ns-3/pod-1.json",
				"resources/persistentvolumes/cluster/pv-1.json",
				"resources/persistentvolumes/cluster/pv-2.json",
			},
		},
		{
			name: "when a wildcard and a specific resource are included, the wildcard takes precedence",
			backup: defaultBackup().
				IncludedResources("*", "pods").
				Build(),
			apiResources: []*apiResource{
				pods(
					newPod("foo", "bar"),
					newPod("zoo", "raz"),
				),
				deployments(
					newDeployment("foo", "bar"),
					newDeployment("zoo", "raz"),
				),
			},
			want: []string{
				"resources/pods/namespaces/foo/bar.json",
				"resources/pods/namespaces/zoo/raz.json",
				"resources/deployments.apps/namespaces/foo/bar.json",
				"resources/deployments.apps/namespaces/zoo/raz.json",
			},
		},
		{
			name: "wildcard excludes are ignored",
			backup: defaultBackup().
				ExcludedResources("*").
				Build(),
			apiResources: []*apiResource{
				pods(
					newPod("foo", "bar"),
					newPod("zoo", "raz"),
				),
				deployments(
					newDeployment("foo", "bar"),
					newDeployment("zoo", "raz"),
				),
			},
			want: []string{
				"resources/pods/namespaces/foo/bar.json",
				"resources/pods/namespaces/zoo/raz.json",
				"resources/deployments.apps/namespaces/foo/bar.json",
				"resources/deployments.apps/namespaces/zoo/raz.json",
			},
		},
		{
			name: "unresolvable included resources are ignored",
			backup: defaultBackup().
				IncludedResources("pods", "unresolvable").
				Build(),
			apiResources: []*apiResource{
				pods(
					newPod("foo", "bar"),
					newPod("zoo", "raz"),
				),
				deployments(
					newDeployment("foo", "bar"),
					newDeployment("zoo", "raz"),
				),
			},
			want: []string{
				"resources/pods/namespaces/foo/bar.json",
				"resources/pods/namespaces/zoo/raz.json",
			},
		},
		{
			name: "unresolvable excluded resources are ignored",
			backup: defaultBackup().
				ExcludedResources("deployments", "unresolvable").
				Build(),
			apiResources: []*apiResource{
				pods(
					newPod("foo", "bar"),
					newPod("zoo", "raz"),
				),
				deployments(
					newDeployment("foo", "bar"),
					newDeployment("zoo", "raz"),
				),
			},
			want: []string{
				"resources/pods/namespaces/foo/bar.json",
				"resources/pods/namespaces/zoo/raz.json",
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var (
				h          = newHarness(t)
				req        = &Request{Backup: tc.backup}
				backupFile = bytes.NewBuffer([]byte{})
			)

			for _, resource := range tc.apiResources {
				h.addItems(t, resource.group, resource.version, resource.name, resource.shortName, resource.namespaced, resource.items...)
			}

			h.backupper.Backup(h.log, req, backupFile, nil, nil)

			assertTarballContents(t, backupFile, append(tc.want, "metadata/version")...)
		})
	}
}

func TestBackupResourceCohabitation(t *testing.T) {
	tests := []struct {
		name         string
		backup       *velerov1.Backup
		apiResources []*apiResource
		want         []string
	}{
		{
			name:   "when deployments exist only in extensions, they're backed up",
			backup: defaultBackup().Build(),
			apiResources: []*apiResource{
				extensionsDeployments(
					newDeployment("foo", "bar"),
					newDeployment("zoo", "raz"),
				),
			},
			want: []string{
				"resources/deployments.extensions/namespaces/foo/bar.json",
				"resources/deployments.extensions/namespaces/zoo/raz.json",
			},
		},
		{
			name:   "when deployments exist in both apps and extensions, only apps/deployments are backed up",
			backup: defaultBackup().Build(),
			apiResources: []*apiResource{
				extensionsDeployments(
					newDeployment("foo", "bar"),
					newDeployment("zoo", "raz"),
				),
				deployments(
					newDeployment("foo", "bar"),
					newDeployment("zoo", "raz"),
				),
			},
			want: []string{
				"resources/deployments.apps/namespaces/foo/bar.json",
				"resources/deployments.apps/namespaces/zoo/raz.json",
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var (
				h          = newHarness(t)
				req        = &Request{Backup: tc.backup}
				backupFile = bytes.NewBuffer([]byte{})
			)

			for _, resource := range tc.apiResources {
				h.addItems(t, resource.group, resource.version, resource.name, resource.shortName, resource.namespaced, resource.items...)
			}

			h.backupper.Backup(h.log, req, backupFile, nil, nil)

			assertTarballContents(t, backupFile, append(tc.want, "metadata/version")...)
		})
	}
}

func TestBackupUsesNewCohabitatingResourcesForEachBackup(t *testing.T) {
	h := newHarness(t)

	// run and verify backup 1
	backup1 := &Request{
		Backup: defaultBackup().Build(),
	}
	backup1File := bytes.NewBuffer([]byte{})

	h.addItems(t, "apps", "v1", "deployments", "deploys", true, newDeployment("ns-1", "deploy-1"))
	h.addItems(t, "extensions", "v1", "deployments", "deploys", true, newDeployment("ns-1", "deploy-1"))

	h.backupper.Backup(h.log, backup1, backup1File, nil, nil)

	assertTarballContents(t, backup1File, "metadata/version", "resources/deployments.apps/namespaces/ns-1/deploy-1.json")

	// run and verify backup 2
	backup2 := &Request{
		Backup: defaultBackup().Build(),
	}
	backup2File := bytes.NewBuffer([]byte{})

	h.backupper.Backup(h.log, backup2, backup2File, nil, nil)

	assertTarballContents(t, backup2File, "metadata/version", "resources/deployments.apps/namespaces/ns-1/deploy-1.json")
}

func TestBackupResourceOrdering(t *testing.T) {
	tests := []struct {
		name         string
		backup       *velerov1.Backup
		apiResources []*apiResource
	}{
		{
			name: "core API group: pods come before pvcs, pvcs come before pvs, pvs come before anything else",
			backup: defaultBackup().
				SnapshotVolumes(false).
				Build(),
			apiResources: []*apiResource{
				pods(
					newPod("foo", "bar"),
					newPod("zoo", "raz"),
				),
				pvcs(
					newPVC("foo", "bar"),
					newPVC("zoo", "raz"),
				),
				pvs(
					newPV("bar"),
					newPV("baz"),
				),
				secrets(
					newSecret("foo", "bar"),
					newSecret("zoo", "raz"),
				),
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var (
				h          = newHarness(t)
				req        = &Request{Backup: tc.backup}
				backupFile = bytes.NewBuffer([]byte{})
			)

			for _, resource := range tc.apiResources {
				h.addItems(t, resource.group, resource.version, resource.name, resource.shortName, resource.namespaced, resource.items...)
			}

			h.backupper.Backup(h.log, req, backupFile, nil, nil)

			assertTarballOrdering(t, backupFile, "pods", "persistentvolumeclaims", "persistentvolumes")
		})
	}
}

type apiResource struct {
	group      string
	version    string
	name       string
	shortName  string
	namespaced bool
	items      []metav1.Object
}

func pods(items ...metav1.Object) *apiResource {
	return &apiResource{
		group:      "",
		version:    "v1",
		name:       "pods",
		shortName:  "po",
		namespaced: true,
		items:      items,
	}
}

func pvcs(items ...metav1.Object) *apiResource {
	return &apiResource{
		group:      "",
		version:    "v1",
		name:       "persistentvolumeclaims",
		shortName:  "pvc",
		namespaced: true,
		items:      items,
	}
}

func secrets(items ...metav1.Object) *apiResource {
	return &apiResource{
		group:      "",
		version:    "v1",
		name:       "secrets",
		shortName:  "secrets",
		namespaced: true,
		items:      items,
	}
}

func deployments(items ...metav1.Object) *apiResource {
	return &apiResource{
		group:      "apps",
		version:    "v1",
		name:       "deployments",
		shortName:  "deploy",
		namespaced: true,
		items:      items,
	}
}

func extensionsDeployments(items ...metav1.Object) *apiResource {
	return &apiResource{
		group:      "extensions",
		version:    "v1",
		name:       "deployments",
		shortName:  "deploy",
		namespaced: true,
		items:      items,
	}
}

func pvs(items ...metav1.Object) *apiResource {
	return &apiResource{
		group:      "",
		version:    "v1",
		name:       "persistentvolumes",
		shortName:  "pv",
		namespaced: false,
		items:      items,
	}
}

type harness struct {
	veleroClient    *fake.Clientset
	kubeClient      *kubefake.Clientset
	dynamicClient   *dynamicfake.FakeDynamicClient
	discoveryClient *test.DiscoveryClient
	backupper       *kubernetesBackupper
	log             logrus.FieldLogger
}

func (h *harness) addItems(t *testing.T, group, version, resource, shortName string, namespaced bool, items ...metav1.Object) {
	t.Helper()

	h.discoveryClient.WithResource(group, version, resource, namespaced, shortName)
	require.NoError(t, h.backupper.discoveryHelper.Refresh())

	gvr := schema.GroupVersionResource{
		Group:    group,
		Version:  version,
		Resource: resource,
	}

	for _, item := range items {
		obj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(item)
		require.NoError(t, err)

		unstructuredObj := &unstructured.Unstructured{Object: obj}

		if namespaced {
			_, err = h.dynamicClient.Resource(gvr).Namespace(item.GetNamespace()).Create(unstructuredObj, metav1.CreateOptions{})
		} else {
			_, err = h.dynamicClient.Resource(gvr).Create(unstructuredObj, metav1.CreateOptions{})
		}
		require.NoError(t, err)
	}
}

func newHarness(t *testing.T) *harness {
	t.Helper()

	// API server fakes
	var (
		veleroClient    = fake.NewSimpleClientset()
		kubeClient      = kubefake.NewSimpleClientset()
		dynamicClient   = dynamicfake.NewSimpleDynamicClient(runtime.NewScheme())
		discoveryClient = &test.DiscoveryClient{FakeDiscovery: kubeClient.Discovery().(*discoveryfake.FakeDiscovery)}
	)

	log := logrus.StandardLogger()

	discoveryHelper, err := discovery.NewHelper(discoveryClient, log)
	require.NoError(t, err)

	return &harness{
		veleroClient:    veleroClient,
		kubeClient:      kubeClient,
		dynamicClient:   dynamicClient,
		discoveryClient: discoveryClient,
		backupper: &kubernetesBackupper{
			dynamicFactory:        client.NewDynamicFactory(dynamicClient),
			discoveryHelper:       discoveryHelper,
			groupBackupperFactory: new(defaultGroupBackupperFactory),

			// unsupported
			podCommandExecutor:     nil,
			resticBackupperFactory: nil,
			resticTimeout:          0,
		},
		log: log,
	}
}

func withLabel(obj metav1.Object, key, val string) metav1.Object {
	labels := obj.GetLabels()
	if labels == nil {
		labels = make(map[string]string)
	}
	labels[key] = val
	obj.SetLabels(labels)

	return obj
}

func newPod(ns, name string) *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: ns,
			Name:      name,
		},
	}
}

func newPVC(ns, name string) *corev1.PersistentVolumeClaim {
	return &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: ns,
			Name:      name,
		},
	}
}

func newSecret(ns, name string) *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: ns,
			Name:      name,
		},
	}
}

func newDeployment(ns, name string) *appsv1.Deployment {
	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: ns,
			Name:      name,
		},
	}
}

func newPV(name string) *corev1.PersistentVolume {
	return &corev1.PersistentVolume{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
	}
}

func defaultBackup() *Builder {
	return NewNamedBuilder(velerov1.DefaultNamespace, "backup-1")
}

func assertTarballContents(t *testing.T, backupFile io.Reader, items ...string) {
	t.Helper()

	gzr, err := gzip.NewReader(backupFile)
	require.NoError(t, err)

	r := tar.NewReader(gzr)

	var files []string
	for {
		hdr, err := r.Next()
		if err == io.EOF {
			break
		}
		require.NoError(t, err)

		files = append(files, hdr.Name)
	}

	sort.Strings(files)
	sort.Strings(items)
	assert.Equal(t, items, files)
}

func assertTarballOrdering(t *testing.T, backupFile io.Reader, orderedResources ...string) {
	t.Helper()

	gzr, err := gzip.NewReader(backupFile)
	require.NoError(t, err)

	r := tar.NewReader(gzr)
	lastSeen := 0

	for {
		hdr, err := r.Next()
		if err == io.EOF {
			break
		}
		require.NoError(t, err)

		if !strings.HasPrefix(hdr.Name, "resources/") {
			continue
		}

		// get the resource name
		parts := strings.Split(hdr.Name, "/")
		require.True(t, len(parts) >= 2)
		resourceName := parts[1]

		// find the index of resourceName in the expected ordering
		current := len(orderedResources)
		for i, item := range orderedResources {
			if item == resourceName {
				current = i
				break
			}
		}

		assert.True(t, current >= lastSeen, "%s was backed up out of order", resourceName)
		lastSeen = current
	}
}
