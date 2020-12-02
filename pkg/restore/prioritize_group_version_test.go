/*
Copyright 2020 the Velero contributors.

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
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	velerov1api "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	"github.com/vmware-tanzu/velero/pkg/builder"
	"github.com/vmware-tanzu/velero/pkg/client"
	"github.com/vmware-tanzu/velero/pkg/features"
	"github.com/vmware-tanzu/velero/pkg/test"
)

func TestMeetsAPIGVRestoreReqs(t *testing.T) {
	tests := []struct {
		name   string
		flag   string
		fmtVer string
		want   bool
	}{
		{
			name:   "returns false if feature flag not enabled and Format Version 1.1.0 not used",
			flag:   "",
			fmtVer: "",
			want:   false,
		},
		{
			name:   "returns false if feature flag not enabled and Format Version 1.1.0 used",
			flag:   "",
			fmtVer: "1.1.0",
			want:   false,
		},
		{
			name:   "returns false if feature flag enabled and Format Version 1.1.0 not used",
			flag:   velerov1api.APIGroupVersionsFeatureFlag,
			fmtVer: "",
			want:   false,
		},
		{
			name:   "returns true if feature flag enabled and Format Version 1.1.0 used",
			flag:   velerov1api.APIGroupVersionsFeatureFlag,
			fmtVer: "1.1.0",
			want:   true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			fakeCtx := &restoreContext{
				backup: &velerov1api.Backup{
					Status: velerov1api.BackupStatus{
						FormatVersion: tc.fmtVer,
					},
				},
			}

			features.NewFeatureFlagSet(tc.flag)

			met := fakeCtx.meetsAPIGVRestoreReqs()

			assert.Equal(t, tc.want, met)
		})

		// Test clean up
		features.NewFeatureFlagSet()
	}
}

func TestChooseAPIVersionsToRestore(t *testing.T) {
	tests := []struct {
		name   string
		files  []string
		fakeDH *test.FakeDiscoveryHelper
		want   map[string]ChosenGroupVersion
	}{
		{
			name: "target cluster preferred version matches source cluster preferred version (Priority 1, Case A)",
			files: []string{
				"/var/s7t9v/resources/priority.example/namespaces/myexample/obj.json",
				"/var/s7t9v/resources/priority.example/v1-preferredversion/namespaces/myexample/obj.json",
				"/var/s7t9v/resources/priority.example/v1alpha1/namespaces/myexample/obj.json",
			},
			fakeDH: &test.FakeDiscoveryHelper{
				APIGroupsList: []metav1.APIGroup{
					{
						Name: "example",
						PreferredVersion: metav1.GroupVersionForDiscovery{
							Version: "v1",
						},
						Versions: []metav1.GroupVersionForDiscovery{
							{Version: "v1"},
							{Version: "v2beta1"},
						},
					},
				},
			},
			want: map[string]ChosenGroupVersion{
				"priority.example": {
					Group:   "example",
					Version: "v1",
					Dir:     "v1-preferredversion",
				},
			},
		},
		{
			name: "target cluster preferred version matches source cluster supported version (Priority 1, Case B)",
			files: []string{
				"/var/s7t9v/resources/priority.example/namespaces/myexample/obj.json",
				"/var/s7t9v/resources/priority.example/v1-preferredversion/namespaces/myexample/obj.json",
				"/var/s7t9v/resources/priority.example/v2beta2/namespaces/myexample/obj.json",
				"/var/s7t9v/resources/priority.example/v2beta1/namespaces/myexample/obj.json",
			},
			fakeDH: &test.FakeDiscoveryHelper{
				APIGroupsList: []metav1.APIGroup{
					{
						Name: "example",
						PreferredVersion: metav1.GroupVersionForDiscovery{
							Version: "v2beta2",
						},
						Versions: []metav1.GroupVersionForDiscovery{
							{Version: "v2beta1"},
						},
					},
				},
			},
			want: map[string]ChosenGroupVersion{
				"priority.example": {
					Group:   "example",
					Version: "v2beta2",
					Dir:     "v2beta2",
				},
			},
		},
		{
			name: "source cluster preferred version matches target supported version (Priority 2, Case C)",
			files: []string{
				"/var/s7t9v/resources/priority.example/namespaces/myexample/obj.json",
				"/var/s7t9v/resources/priority.example/v1-preferredversion/namespaces/myexample/obj.json",
				"/var/s7t9v/resources/priority.example/v1alpha1/namespaces/myexample/obj.json",
			},
			fakeDH: &test.FakeDiscoveryHelper{
				APIGroupsList: []metav1.APIGroup{
					{
						Name: "example",
						PreferredVersion: metav1.GroupVersionForDiscovery{
							Version: "v2",
						},
						Versions: []metav1.GroupVersionForDiscovery{
							{Version: "v2"},
							{Version: "v1"},
						},
					},
				},
			},
			want: map[string]ChosenGroupVersion{
				"priority.example": {
					Group:   "example",
					Version: "v1",
					Dir:     "v1-preferredversion",
				},
			},
		},
		{
			name: "latest common supported version (Priority 3, Case D)",
			files: []string{
				"/var/s7t9v/resources/priority.example/namespaces/myexample/obj.json",
				"/var/s7t9v/resources/priority.example/v1-preferredversion/namespaces/myexample/obj.json",
				"/var/s7t9v/resources/priority.example/v2beta2/namespaces/myexample/obj.json",
				"/var/s7t9v/resources/priority.example/v2beta1/namespaces/myexample/obj.json",
			},
			fakeDH: &test.FakeDiscoveryHelper{
				APIGroupsList: []metav1.APIGroup{
					{
						Name: "example",
						PreferredVersion: metav1.GroupVersionForDiscovery{
							Version: "v2",
						},
						Versions: []metav1.GroupVersionForDiscovery{
							{Version: "v2"},
							{Version: "v2beta1"},
							{Version: "v2beta2"},
						},
					},
				},
			},
			want: map[string]ChosenGroupVersion{
				"priority.example": {
					Group:   "example",
					Version: "v2beta2",
					Dir:     "v2beta2",
				},
			},
		},
		{
			name: "no common supported versions defaults to source preferred version",
			files: []string{
				"/var/s7t9v/resources/priority.example/namespaces/myexample/obj.json",
				"/var/s7t9v/resources/priority.example/v1-preferredversion/namespaces/myexample/obj.json",
				"/var/s7t9v/resources/priority.example/v1beta1/namespaces/myexample/obj.json",
				"/var/s7t9v/resources/priority.example/v1alpha1/namespaces/myexample/obj.json",
			},
			fakeDH: &test.FakeDiscoveryHelper{
				APIGroupsList: []metav1.APIGroup{
					{
						Name: "example",
						PreferredVersion: metav1.GroupVersionForDiscovery{
							Version: "v2",
						},
						Versions: []metav1.GroupVersionForDiscovery{
							{Version: "v2"},
							{Version: "v2alpha1"},
						},
					},
				},
			},
			want: map[string]ChosenGroupVersion{
				"priority.example": {
					Group:   "example",
					Version: "v1",
					Dir:     "v1-preferredversion",
				},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			fakeCtx := &restoreContext{
				log:                    test.NewLogger(),
				fileSystem:             test.NewFakeFileSystem(),
				restoreDir:             "/var/s7t9v",
				discoveryHelper:        tc.fakeDH,
				chosenGrpVersToRestore: make(map[string]ChosenGroupVersion),
			}

			for _, file := range tc.files {
				require.NoError(t, fakeCtx.fileSystem.MkdirAll(file, 0755))

				if !strings.HasSuffix(file, "/") {
					res, err := fakeCtx.fileSystem.Create(file)
					require.NoError(t, err)
					require.NoError(t, res.Close())
				}
			}

			require.NoError(t, fakeCtx.chooseAPIVersionsToRestore())

			assert.Equal(t, tc.want, fakeCtx.chosenGrpVersToRestore)
		})
	}
}

func TestK8sPrioritySort(t *testing.T) {
	tests := []struct {
		name string
		orig []metav1.GroupVersionForDiscovery
		want []metav1.GroupVersionForDiscovery
	}{
		{
			name: "sorts Kubernetes API group versions per k8s priority",
			orig: []metav1.GroupVersionForDiscovery{
				{Version: "v2"},
				{Version: "v11alpha2"},
				{Version: "foo10"},
				{Version: "v10"},
				{Version: "v12alpha1"},
				{Version: "v3beta1"},
				{Version: "foo1"},
				{Version: "v1"},
				{Version: "v10beta3"},
				{Version: "v11beta2"},
			},
			want: []metav1.GroupVersionForDiscovery{
				{Version: "v10"},
				{Version: "v2"},
				{Version: "v1"},
				{Version: "v11beta2"},
				{Version: "v10beta3"},
				{Version: "v3beta1"},
				{Version: "v12alpha1"},
				{Version: "v11alpha2"},
				{Version: "foo1"},
				{Version: "foo10"},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			k8sPrioritySort(tc.orig)

			assert.Equal(t, tc.want, tc.orig)
		})
	}
}

func TestUserResourceGroupVersionPriorities(t *testing.T) {
	tests := []struct {
		name string
		cm   *corev1.ConfigMap
		want map[string]metav1.APIGroup
	}{
		{
			name: "retrieve version priority data from config map",
			cm: builder.
				ForConfigMap("velero", "enableapigroupversions").
				Data(
					"restoreResourcesVersionPriority",
					`rockbands.music.example.io=v2beta1,v2beta2
orchestras.music.example.io=v2,v3alpha1
subscriptions.operators.coreos.com=v2,v1`,
				).
				Result(),
			want: map[string]metav1.APIGroup{
				"rockbands.music.example.io": {Versions: []metav1.GroupVersionForDiscovery{
					{Version: "v2beta1"},
					{Version: "v2beta2"},
				}},
				"orchestras.music.example.io": {Versions: []metav1.GroupVersionForDiscovery{
					{Version: "v2"},
					{Version: "v3alpha1"},
				}},
				"subscriptions.operators.coreos.com": {Versions: []metav1.GroupVersionForDiscovery{
					{Version: "v2"},
					{Version: "v1"},
				}},
			},
		},
		{
			name: "expect empty map if config map is in wrong namespace",
			cm: builder.
				ForConfigMap("default", "enableapigroupversions").
				Data(
					"restoreResourcesVersionPriority",
					`rockbands.music.example.io=v2beta1,v2beta2`,
				).
				Result(),
			want: nil,
		},
		{
			name: "expect empty map if config map has wrong name",
			cm: builder.
				ForConfigMap("velero", "disableapigroupversions").
				Data(
					"restoreResourcesVersionPriority",
					`rockbands.music.example.io=v2beta1,v2beta2`,
				).
				Result(),
			want: nil,
		},
		{
			name: "incorrect data format returns no user version priorities",
			cm: builder.
				ForConfigMap("velero", "enableapigroupversions").
				Data(
					"restoreResourcesVersionPriority",
					`rockbands.music.example.io=v2beta1,v2beta2\n orchestras.music.example.io=v2,v3alpha1`,
				).
				Result(),
			want: nil,
		},
		{
			name: "incorrect and correct data format returns some user version priorities",
			cm: builder.
				ForConfigMap("velero", "enableapigroupversions").
				Data(
					"restoreResourcesVersionPriority",
					`rb=foo,foo2
o=v2beta2
\n`,
				).
				Result(),
			want: map[string]metav1.APIGroup{
				"rb": {Versions: []metav1.GroupVersionForDiscovery{
					{Version: "foo"},
					{Version: "foo2"},
				}},
				"o": {Versions: []metav1.GroupVersionForDiscovery{
					{Version: "v2beta2"},
				}},
			},
		},
	}

	cfg, err := client.LoadConfig()
	require.NoError(t, err)

	fc := client.NewFactory("APIGroupVersionsRestore_UnitTest", cfg)

	kc, err := fc.KubeClient()
	require.NoError(t, err)

	fakeCtx := &restoreContext{
		log: test.NewLogger(),
	}

	for _, tc := range tests {
		// Test preparation
		_, _ = kc.CoreV1().ConfigMaps(tc.cm.Namespace).Create(
			context.Background(),
			tc.cm,
			metav1.CreateOptions{},
		)

		// Run tests
		t.Log(tc.name)
		m, err := userResourceGroupVersionPriorities(fakeCtx)
		require.NoError(t, err)
		assert.Equal(t, tc.want, m)

		// Test clean up
		err = kc.CoreV1().ConfigMaps(tc.cm.Namespace).Delete(
			context.Background(),
			tc.cm.Name,
			metav1.DeleteOptions{},
		)
		require.NoError(t, err)
	}
}

func TestFindTargetGroup(t *testing.T) {
	tests := []struct {
		name    string
		tGrps   []metav1.APIGroup
		grpName string
		want    metav1.APIGroup
	}{
		{
			name: "return the API Group in target list matching group string",
			tGrps: []metav1.APIGroup{
				{
					Name: "rbac.authorization.k8s.io",
					Versions: []metav1.GroupVersionForDiscovery{
						{Version: "v2"},
					},
					PreferredVersion: metav1.GroupVersionForDiscovery{Version: "v2"},
				},
				{
					Name: "",
					Versions: []metav1.GroupVersionForDiscovery{
						{Version: "v1"},
					},
					PreferredVersion: metav1.GroupVersionForDiscovery{Version: "v1"},
				},
				{
					Name: "velero.vmware.com",
					Versions: []metav1.GroupVersionForDiscovery{
						{Version: "v2beta1"},
						{Version: "v2beta2"},
						{Version: "v2"},
					},
					PreferredVersion: metav1.GroupVersionForDiscovery{Version: "v2"},
				},
			},
			grpName: "velero.vmware.com",
			want: metav1.APIGroup{
				Name: "velero.vmware.com",
				Versions: []metav1.GroupVersionForDiscovery{
					{Version: "v2beta1"},
					{Version: "v2beta2"},
					{Version: "v2"},
				},
				PreferredVersion: metav1.GroupVersionForDiscovery{Version: "v2"},
			},
		},
		{
			name: "return empty API Group if no match in target list",
			tGrps: []metav1.APIGroup{
				{
					Name: "rbac.authorization.k8s.io",
					Versions: []metav1.GroupVersionForDiscovery{
						{Version: "v2"},
					},
					PreferredVersion: metav1.GroupVersionForDiscovery{Version: "v2"},
				},
				{
					Name: "",
					Versions: []metav1.GroupVersionForDiscovery{
						{Version: "v1"},
					},
					PreferredVersion: metav1.GroupVersionForDiscovery{Version: "v1"},
				},
				{
					Name: "velero.vmware.com",
					Versions: []metav1.GroupVersionForDiscovery{
						{Version: "v2beta1"},
						{Version: "v2beta2"},
						{Version: "v2"},
					},
					PreferredVersion: metav1.GroupVersionForDiscovery{Version: "v2"},
				},
			},
			grpName: "autoscaling",
			want:    metav1.APIGroup{},
		},
	}

	for _, tc := range tests {
		grp := findTargetGroup(tc.tGrps, tc.grpName)

		assert.Equal(t, tc.want, grp)
	}
}

func TestFindSupportedUserVersion(t *testing.T) {
	tests := []struct {
		name string
		ugvs []metav1.GroupVersionForDiscovery
		tgvs []metav1.GroupVersionForDiscovery
		sgvs []metav1.GroupVersionForDiscovery
		want string
	}{
		{
			name: "return the single user group version that has a match in both source and target clusters",
			ugvs: []metav1.GroupVersionForDiscovery{
				{Version: "foo"},
				{Version: "v10alpha2"},
				{Version: "v3"},
			},
			tgvs: []metav1.GroupVersionForDiscovery{
				{Version: "v9"},
				{Version: "v10beta1"},
				{Version: "v10alpha2"},
				{Version: "v10alpha3"},
			},
			sgvs: []metav1.GroupVersionForDiscovery{
				{Version: "v10alpha2"},
				{Version: "v9beta1"},
			},
			want: "v10alpha2",
		},
		{
			name: "return the first user group version that has a match in both source and target clusters",
			ugvs: []metav1.GroupVersionForDiscovery{
				{Version: "v2beta1"},
				{Version: "v2beta2"},
			},
			tgvs: []metav1.GroupVersionForDiscovery{
				{Version: "v2beta2"},
				{Version: "v2beta1"},
			},
			sgvs: []metav1.GroupVersionForDiscovery{
				{Version: "v1"},
				{Version: "v2beta2"},
				{Version: "v2beta1"},
			},
			want: "v2beta1",
		},
		{
			name: "return empty string if there's only matches in the source cluster, but not target",
			ugvs: []metav1.GroupVersionForDiscovery{
				{Version: "v1"},
			},
			tgvs: []metav1.GroupVersionForDiscovery{
				{Version: "v2"},
			},
			sgvs: []metav1.GroupVersionForDiscovery{
				{Version: "v1"},
			},
			want: "",
		},
		{
			name: "return empty string if there's only matches in the target cluster, but not source",
			ugvs: []metav1.GroupVersionForDiscovery{
				{Version: "v3"},
				{Version: "v1"},
			},
			tgvs: []metav1.GroupVersionForDiscovery{
				{Version: "v3"},
				{Version: "v3beta2"},
			},
			sgvs: []metav1.GroupVersionForDiscovery{
				{Version: "v2"},
				{Version: "v2beta1"},
			},
			want: "",
		},
		{
			name: "return empty string if there is no match with either target and source clusters",
			ugvs: []metav1.GroupVersionForDiscovery{
				{Version: "v2beta2"},
				{Version: "v2beta1"},
				{Version: "v2beta3"},
			},
			tgvs: []metav1.GroupVersionForDiscovery{
				{Version: "v2"},
				{Version: "v1"},
				{Version: "v2alpha1"},
			},
			sgvs: []metav1.GroupVersionForDiscovery{
				{Version: "v1"},
				{Version: "v2alpha1"},
			},
			want: "",
		},
	}

	for _, tc := range tests {
		uv := findSupportedUserVersion(tc.ugvs, tc.tgvs, tc.sgvs)

		assert.Equal(t, tc.want, uv)
	}
}

func TestVersionsContain(t *testing.T) {
	tests := []struct {
		name string
		GVs  []metav1.GroupVersionForDiscovery
		ver  string
		want bool
	}{
		{
			name: "version is not in list",
			GVs: []metav1.GroupVersionForDiscovery{
				{Version: "v1"},
				{Version: "v2alpha1"},
				{Version: "v2beta1"},
			},
			ver:  "v2",
			want: false,
		},
		{
			name: "version is in list",
			GVs: []metav1.GroupVersionForDiscovery{
				{Version: "v2"},
				{Version: "v2alpha1"},
				{Version: "v2beta1"},
			},
			ver:  "v2",
			want: true,
		},
	}

	for _, tc := range tests {
		assert.Equal(t, tc.want, versionsContain(tc.GVs, tc.ver))
	}
}

func TestLatestCommon(t *testing.T) {
	tests := []struct {
		name       string
		tGVs, sGVs []metav1.GroupVersionForDiscovery
		want       string
	}{
		{
			name: "choose the latest when there are three versions in common",
			tGVs: []metav1.GroupVersionForDiscovery{
				{Version: "v2"},
				{Version: "v2alpha1"},
				{Version: "v2beta1"},
			},
			sGVs: []metav1.GroupVersionForDiscovery{
				{Version: "v2beta1"},
				{Version: "v2"},
				{Version: "v2alpha1"},
			},
			want: "v2beta1",
		},
		{
			name: "return empty string when no version is in common",
			tGVs: []metav1.GroupVersionForDiscovery{
				{Version: "v2"},
				{Version: "v2alpha1"},
				{Version: "v2beta1"},
			},
			sGVs: []metav1.GroupVersionForDiscovery{
				{Version: "v1"},
				{Version: "v1alpha1"},
				{Version: "v1beta1"},
			},
			want: "",
		},
	}

	for _, tc := range tests {
		assert.Equal(t, tc.want, latestCommon(tc.sGVs, tc.tGVs))
	}
}
