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
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/vmware-tanzu/velero/pkg/kuberesource"

	"github.com/stretchr/testify/assert"
	corev1api "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/vmware-tanzu/velero/pkg/builder"
)

func Test_resourceKey(t *testing.T) {
	tests := []struct {
		resource metav1.Object
		want     string
	}{
		{resource: builder.ForPod("default", "test").Result(), want: "v1/Pod"},
		{resource: builder.ForDeployment("default", "test").Result(), want: "apps/v1/Deployment"},
		{resource: builder.ForPersistentVolume("test").Result(), want: "v1/PersistentVolume"},
		{resource: builder.ForRole("default", "test").Result(), want: "rbac.authorization.k8s.io/v1/Role"},
	}
	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			content, _ := runtime.DefaultUnstructuredConverter.ToUnstructured(tt.resource)
			unstructured := &unstructured.Unstructured{Object: content}
			assert.Equal(t, tt.want, resourceKey(unstructured))
		})
	}
}

func Test_zoneFromPVNodeAffinity(t *testing.T) {
	keys := []string{
		awsEbsCsiZoneKey,
		azureCsiZoneKey,
		gkeCsiZoneKey,
		zoneLabel,
		zoneLabelDeprecated,
	}
	tests := []struct {
		name      string
		pv        *corev1api.PersistentVolume
		wantKey   string
		wantValue string
	}{
		{
			name: "AWS CSI Volume",
			pv: builder.ForPersistentVolume("awscsi").NodeAffinityRequired(
				builder.ForNodeSelector(
					*builder.NewNodeSelectorTermBuilder().WithMatchExpression("topology.ebs.csi.aws.com/zone",
						"In", "us-east-2c").Result(),
				).Result(),
			).Result(),
			wantKey:   "topology.ebs.csi.aws.com/zone",
			wantValue: "us-east-2c",
		},
		{
			name: "Azure CSI Volume",
			pv: builder.ForPersistentVolume("azurecsi").NodeAffinityRequired(
				builder.ForNodeSelector(
					*builder.NewNodeSelectorTermBuilder().WithMatchExpression("topology.disk.csi.azure.com/zone",
						"In", "us-central").Result(),
				).Result(),
			).Result(),
			wantKey:   "topology.disk.csi.azure.com/zone",
			wantValue: "us-central",
		},
		{
			name: "GCP CSI Volume",
			pv: builder.ForPersistentVolume("gcpcsi").NodeAffinityRequired(
				builder.ForNodeSelector(
					*builder.NewNodeSelectorTermBuilder().WithMatchExpression("topology.gke.io/zone",
						"In", "us-west1-a").Result(),
				).Result(),
			).Result(),
			wantKey:   "topology.gke.io/zone",
			wantValue: "us-west1-a",
		},
		{
			name: "AWS CSI Volume with multiple zone value, returns the first",
			pv: builder.ForPersistentVolume("awscsi").NodeAffinityRequired(
				builder.ForNodeSelector(
					*builder.NewNodeSelectorTermBuilder().WithMatchExpression("topology.ebs.csi.aws.com/zone",
						"In", "us-east-2c", "us-west").Result(),
				).Result(),
			).Result(),
			wantKey:   "topology.ebs.csi.aws.com/zone",
			wantValue: "us-east-2c",
		},
		{
			name: "Volume with no matching key",
			pv: builder.ForPersistentVolume("no-matching-pv").NodeAffinityRequired(
				builder.ForNodeSelector(
					*builder.NewNodeSelectorTermBuilder().WithMatchExpression("some-key",
						"In", "us-west").Result(),
				).Result(),
			).Result(),
			wantKey:   "",
			wantValue: "",
		},
		{
			name: "Volume with multiple valid keys, returns the first match", // it should never happen
			pv: builder.ForPersistentVolume("multi-matching-pv").NodeAffinityRequired(
				builder.ForNodeSelector(
					*builder.NewNodeSelectorTermBuilder().WithMatchExpression("topology.disk.csi.azure.com/zone",
						"In", "us-central").Result(),
					*builder.NewNodeSelectorTermBuilder().WithMatchExpression("topology.ebs.csi.aws.com/zone",
						"In", "us-east-2c", "us-west").Result(),
					*builder.NewNodeSelectorTermBuilder().WithMatchExpression("topology.ebs.csi.aws.com/zone",
						"In", "unknown").Result(),
				).Result(),
			).Result(),
			wantKey:   "topology.disk.csi.azure.com/zone",
			wantValue: "us-central",
		},
		{
			/* an valid example of node affinity in a GKE's regional PV
			nodeAffinity:
			  required:
			    nodeSelectorTerms:
			    - matchExpressions:
			      - key: topology.gke.io/zone
			        operator: In
			        values:
			        - us-central1-a
			    - matchExpressions:
			      - key: topology.gke.io/zone
			        operator: In
			        values:
			        - us-central1-c
			*/
			name: "Volume with multiple valid keys, and provider is gke, returns all valid entries's first zone value",
			pv: builder.ForPersistentVolume("multi-matching-pv").NodeAffinityRequired(
				builder.ForNodeSelector(
					*builder.NewNodeSelectorTermBuilder().WithMatchExpression("topology.gke.io/zone",
						"In", "us-central1-c").Result(),
					*builder.NewNodeSelectorTermBuilder().WithMatchExpression("topology.gke.io/zone",
						"In", "us-east-2c", "us-east-2b").Result(),
					*builder.NewNodeSelectorTermBuilder().WithMatchExpression("topology.gke.io/zone",
						"In", "europe-north1-a").Result(),
				).Result(),
			).Result(),
			wantKey:   "topology.gke.io/zone",
			wantValue: "us-central1-c__us-east-2c__europe-north1-a",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			k, v := zoneFromPVNodeAffinity(tt.pv, keys...)
			assert.Equal(t, tt.wantKey, k)
			assert.Equal(t, tt.wantValue, v)
		})
	}
}

func TestGetPVName(t *testing.T) {
	testcases := []struct {
		name          string
		obj           metav1.Object
		groupResource schema.GroupResource
		pvName        string
		hasErr        bool
	}{
		{
			name:          "pv should return pv name",
			obj:           builder.ForPersistentVolume("test-pv").Result(),
			groupResource: kuberesource.PersistentVolumes,
			pvName:        "test-pv",
			hasErr:        false,
		},
		{
			name:          "pvc without volumeName should return error",
			obj:           builder.ForPersistentVolumeClaim("ns", "pvc-1").Result(),
			groupResource: kuberesource.PersistentVolumeClaims,
			pvName:        "",
			hasErr:        true,
		},
		{
			name:          "pvc with volumeName should return pv name",
			obj:           builder.ForPersistentVolumeClaim("ns", "pvc-1").VolumeName("test-pv-2").Result(),
			groupResource: kuberesource.PersistentVolumeClaims,
			pvName:        "test-pv-2",
			hasErr:        false,
		},
		{
			name:          "unsupported group resource should return empty pv name",
			obj:           builder.ForPod("ns", "pod1").Result(),
			groupResource: kuberesource.Pods,
			pvName:        "",
			hasErr:        false,
		},
	}
	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			o := &unstructured.Unstructured{Object: nil}
			if tc.obj != nil {
				data, err := runtime.DefaultUnstructuredConverter.ToUnstructured(tc.obj)
				o = &unstructured.Unstructured{Object: data}
				require.NoError(t, err)
			}
			name, err2 := getPVName(o, tc.groupResource)
			assert.Equal(t, tc.pvName, name)
			assert.Equal(t, tc.hasErr, err2 != nil)
		})
	}
}

func TestRandom(t *testing.T) {
	pv := new(corev1api.PersistentVolume)
	pvc := new(corev1api.PersistentVolumeClaim)
	obj := builder.ForPod("ns1", "pod1").ServiceAccount("sa").Result()
	o, _ := runtime.DefaultUnstructuredConverter.ToUnstructured(obj)
	err1 := runtime.DefaultUnstructuredConverter.FromUnstructured(o, pv)
	err2 := runtime.DefaultUnstructuredConverter.FromUnstructured(o, pvc)
	t.Logf("err1: %v, err2: %v", err1, err2)
}

func TestAddVolumeInfo(t *testing.T) {
	tests := []struct {
		name string
		pv   *corev1api.PersistentVolume
	}{
		{
			name: "PV has ClaimRef",
			pv:   builder.ForPersistentVolume("testPV").ClaimRef("testNS", "testPVC").Result(),
		},
		{
			name: "PV has no ClaimRef",
			pv:   builder.ForPersistentVolume("testPV").Result(),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ib := itemBackupper{}
			ib.backupRequest = new(Request)
			ib.backupRequest.VolumesInformation.Init()

			pvObj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(tc.pv)
			require.NoError(t, err)
			logger := logrus.StandardLogger()

			err = ib.addVolumeInfo(&unstructured.Unstructured{Object: pvObj}, logger)
			require.NoError(t, err)
		})
	}
}
