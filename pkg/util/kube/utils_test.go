/*
Copyright the Velero contributors.

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
	"encoding/json"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	storagev1api "k8s.io/api/storage/v1"
	apiextv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apiextv1beta1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/vmware-tanzu/velero/pkg/builder"
	velerotest "github.com/vmware-tanzu/velero/pkg/test"
	"github.com/vmware-tanzu/velero/pkg/uploader"
)

func TestNamespaceAndName(t *testing.T) {
	//TODO
}

// TestGetVolumeDirectorySuccess tests that the GetVolumeDirectory function
// returns a volume's name or a volume's name plus '/mount' when a PVC is present.
func TestGetVolumeDirectorySuccess(t *testing.T) {
	tests := []struct {
		name string
		pod  *corev1.Pod
		pvc  *corev1.PersistentVolumeClaim
		pv   *corev1.PersistentVolume
		want string
	}{
		{
			name: "Non-CSI volume with a PVC/PV returns the volume's name",
			pod:  builder.ForPod("ns-1", "my-pod").Volumes(builder.ForVolume("my-vol").PersistentVolumeClaimSource("my-pvc").Result()).Result(),
			pvc:  builder.ForPersistentVolumeClaim("ns-1", "my-pvc").VolumeName("a-pv").Result(),
			pv:   builder.ForPersistentVolume("a-pv").Result(),
			want: "a-pv",
		},
		{
			name: "CSI volume with a PVC/PV appends '/mount' to the volume name",
			pod:  builder.ForPod("ns-1", "my-pod").Volumes(builder.ForVolume("my-vol").PersistentVolumeClaimSource("my-pvc").Result()).Result(),
			pvc:  builder.ForPersistentVolumeClaim("ns-1", "my-pvc").VolumeName("a-pv").Result(),
			pv:   builder.ForPersistentVolume("a-pv").CSI("csi.test.com", "provider-volume-id").Result(),
			want: "a-pv/mount",
		},
		{
			name: "Block CSI volume with a PVC/PV does not append '/mount' to the volume name",
			pod:  builder.ForPod("ns-1", "my-pod").Volumes(builder.ForVolume("my-vol").PersistentVolumeClaimSource("my-pvc").Result()).Result(),
			pvc:  builder.ForPersistentVolumeClaim("ns-1", "my-pvc").VolumeName("a-pv").Result(),
			pv:   builder.ForPersistentVolume("a-pv").CSI("csi.test.com", "provider-volume-id").VolumeMode(corev1.PersistentVolumeBlock).Result(),
			want: "a-pv",
		},
		{
			name: "CSI volume mounted without a PVC appends '/mount' to the volume name",
			pod:  builder.ForPod("ns-1", "my-pod").Volumes(builder.ForVolume("my-vol").CSISource("csi.test.com").Result()).Result(),
			want: "my-vol/mount",
		},
		{
			name: "Non-CSI volume without a PVC returns the volume name",
			pod:  builder.ForPod("ns-1", "my-pod").Volumes(builder.ForVolume("my-vol").Result()).Result(),
			want: "my-vol",
		},
		{
			name: "Volume with CSI annotation appends '/mount' to the volume name",
			pod:  builder.ForPod("ns-1", "my-pod").Volumes(builder.ForVolume("my-vol").PersistentVolumeClaimSource("my-pvc").Result()).Result(),
			pvc:  builder.ForPersistentVolumeClaim("ns-1", "my-pvc").VolumeName("a-pv").Result(),
			pv:   builder.ForPersistentVolume("a-pv").ObjectMeta(builder.WithAnnotations(KubeAnnDynamicallyProvisioned, "csi.test.com")).Result(),
			want: "a-pv/mount",
		},
		{
			name: "Volume with CSI annotation 'pv.kubernetes.io/migrated-to' appends '/mount' to the volume name",
			pod:  builder.ForPod("ns-1", "my-pod").Volumes(builder.ForVolume("my-vol").PersistentVolumeClaimSource("my-pvc").Result()).Result(),
			pvc:  builder.ForPersistentVolumeClaim("ns-1", "my-pvc").VolumeName("a-pv").Result(),
			pv:   builder.ForPersistentVolume("a-pv").ObjectMeta(builder.WithAnnotations(KubeAnnMigratedTo, "csi.test.com")).Result(),
			want: "a-pv/mount",
		},
	}

	csiDriver := storagev1api.CSIDriver{
		ObjectMeta: metav1.ObjectMeta{Name: "csi.test.com"},
	}
	for _, tc := range tests {
		clientBuilder := fake.NewClientBuilder().WithLists(&storagev1api.CSIDriverList{Items: []storagev1api.CSIDriver{csiDriver}})

		if tc.pvc != nil {
			clientBuilder = clientBuilder.WithObjects(tc.pvc)
		}
		if tc.pv != nil {
			clientBuilder = clientBuilder.WithObjects(tc.pv)
		}

		// Function under test
		dir, err := GetVolumeDirectory(context.Background(), logrus.StandardLogger(), tc.pod, tc.pod.Spec.Volumes[0].Name, clientBuilder.Build())

		require.NoError(t, err)
		assert.Equal(t, tc.want, dir)
	}
}

// TestGetVolumeModeSuccess tests the GetVolumeMode function
func TestGetVolumeModeSuccess(t *testing.T) {
	tests := []struct {
		name string
		pod  *corev1.Pod
		pvc  *corev1.PersistentVolumeClaim
		pv   *corev1.PersistentVolume
		want uploader.PersistentVolumeMode
	}{
		{
			name: "Filesystem PVC volume",
			pod:  builder.ForPod("ns-1", "my-pod").Volumes(builder.ForVolume("my-vol").PersistentVolumeClaimSource("my-pvc").Result()).Result(),
			pvc:  builder.ForPersistentVolumeClaim("ns-1", "my-pvc").VolumeName("a-pv").Result(),
			pv:   builder.ForPersistentVolume("a-pv").VolumeMode(corev1.PersistentVolumeFilesystem).Result(),
			want: uploader.PersistentVolumeFilesystem,
		},
		{
			name: "Block PVC volume",
			pod:  builder.ForPod("ns-1", "my-pod").Volumes(builder.ForVolume("my-vol").PersistentVolumeClaimSource("my-pvc").Result()).Result(),
			pvc:  builder.ForPersistentVolumeClaim("ns-1", "my-pvc").VolumeName("a-pv").Result(),
			pv:   builder.ForPersistentVolume("a-pv").VolumeMode(corev1.PersistentVolumeBlock).Result(),
			want: uploader.PersistentVolumeBlock,
		},
		{
			name: "Pod volume without a PVC",
			pod:  builder.ForPod("ns-1", "my-pod").Volumes(builder.ForVolume("my-vol").Result()).Result(),
			want: uploader.PersistentVolumeFilesystem,
		},
	}

	for _, tc := range tests {
		clientBuilder := fake.NewClientBuilder()

		if tc.pvc != nil {
			clientBuilder = clientBuilder.WithObjects(tc.pvc)
		}
		if tc.pv != nil {
			clientBuilder = clientBuilder.WithObjects(tc.pv)
		}

		// Function under test
		mode, err := GetVolumeMode(context.Background(), logrus.StandardLogger(), tc.pod, tc.pod.Spec.Volumes[0].Name, clientBuilder.Build())

		require.NoError(t, err)
		assert.Equal(t, tc.want, mode)
	}
}

func TestIsV1Beta1CRDReady(t *testing.T) {
	tests := []struct {
		name string
		crd  *apiextv1beta1.CustomResourceDefinition
		want bool
	}{
		{
			name: "CRD is not established & not accepting names - not ready",
			crd:  builder.ForCustomResourceDefinitionV1Beta1("MyCRD").Result(),
			want: false,
		},
		{
			name: "CRD is established & not accepting names - not ready",
			crd: builder.ForCustomResourceDefinitionV1Beta1("MyCRD").
				Condition(builder.ForCustomResourceDefinitionV1Beta1Condition().Type(apiextv1beta1.Established).Status(apiextv1beta1.ConditionTrue).Result()).Result(),
			want: false,
		},
		{
			name: "CRD is not established & accepting names - not ready",
			crd: builder.ForCustomResourceDefinitionV1Beta1("MyCRD").
				Condition(builder.ForCustomResourceDefinitionV1Beta1Condition().Type(apiextv1beta1.NamesAccepted).Status(apiextv1beta1.ConditionTrue).Result()).Result(),
			want: false,
		},
		{
			name: "CRD is established & accepting names - ready",
			crd: builder.ForCustomResourceDefinitionV1Beta1("MyCRD").
				Condition(builder.ForCustomResourceDefinitionV1Beta1Condition().Type(apiextv1beta1.Established).Status(apiextv1beta1.ConditionTrue).Result()).
				Condition(builder.ForCustomResourceDefinitionV1Beta1Condition().Type(apiextv1beta1.NamesAccepted).Status(apiextv1beta1.ConditionTrue).Result()).
				Result(),
			want: true,
		},
	}

	for _, tc := range tests {
		result := IsV1Beta1CRDReady(tc.crd)
		assert.Equal(t, tc.want, result)
	}
}

func TestIsV1CRDReady(t *testing.T) {
	tests := []struct {
		name string
		crd  *apiextv1.CustomResourceDefinition
		want bool
	}{
		{
			name: "CRD is not established & not accepting names - not ready",
			crd:  builder.ForV1CustomResourceDefinition("MyCRD").Result(),
			want: false,
		},
		{
			name: "CRD is established & not accepting names - not ready",
			crd: builder.ForV1CustomResourceDefinition("MyCRD").
				Condition(builder.ForV1CustomResourceDefinitionCondition().Type(apiextv1.Established).Status(apiextv1.ConditionTrue).Result()).Result(),
			want: false,
		},
		{
			name: "CRD is not established & accepting names - not ready",
			crd: builder.ForV1CustomResourceDefinition("MyCRD").
				Condition(builder.ForV1CustomResourceDefinitionCondition().Type(apiextv1.NamesAccepted).Status(apiextv1.ConditionTrue).Result()).Result(),
			want: false,
		},
		{
			name: "CRD is established & accepting names - ready",
			crd: builder.ForV1CustomResourceDefinition("MyCRD").
				Condition(builder.ForV1CustomResourceDefinitionCondition().Type(apiextv1.Established).Status(apiextv1.ConditionTrue).Result()).
				Condition(builder.ForV1CustomResourceDefinitionCondition().Type(apiextv1.NamesAccepted).Status(apiextv1.ConditionTrue).Result()).
				Result(),
			want: true,
		},
	}

	for _, tc := range tests {
		result := IsV1CRDReady(tc.crd)
		assert.Equal(t, tc.want, result)
	}
}

func TestIsCRDReady(t *testing.T) {
	v1beta1tests := []struct {
		name string
		crd  *apiextv1beta1.CustomResourceDefinition
		want bool
	}{
		{
			name: "v1beta1CRD is not established & not accepting names - not ready",
			crd:  builder.ForCustomResourceDefinitionV1Beta1("MyCRD").Result(),
			want: false,
		},
		{
			name: "v1beta1CRD is established & not accepting names - not ready",
			crd: builder.ForCustomResourceDefinitionV1Beta1("MyCRD").
				Condition(builder.ForCustomResourceDefinitionV1Beta1Condition().Type(apiextv1beta1.Established).Status(apiextv1beta1.ConditionTrue).Result()).Result(),
			want: false,
		},
		{
			name: "v1beta1CRD is not established & accepting names - not ready",
			crd: builder.ForCustomResourceDefinitionV1Beta1("MyCRD").
				Condition(builder.ForCustomResourceDefinitionV1Beta1Condition().Type(apiextv1beta1.NamesAccepted).Status(apiextv1beta1.ConditionTrue).Result()).Result(),
			want: false,
		},
		{
			name: "v1beta1CRD is established & accepting names - ready",
			crd: builder.ForCustomResourceDefinitionV1Beta1("MyCRD").
				Condition(builder.ForCustomResourceDefinitionV1Beta1Condition().Type(apiextv1beta1.Established).Status(apiextv1beta1.ConditionTrue).Result()).
				Condition(builder.ForCustomResourceDefinitionV1Beta1Condition().Type(apiextv1beta1.NamesAccepted).Status(apiextv1beta1.ConditionTrue).Result()).
				Result(),
			want: true,
		},
	}

	for _, tc := range v1beta1tests {
		m, err := runtime.DefaultUnstructuredConverter.ToUnstructured(tc.crd)
		require.NoError(t, err)
		result, err := IsCRDReady(&unstructured.Unstructured{Object: m})
		require.NoError(t, err)
		assert.Equal(t, tc.want, result)
	}

	v1tests := []struct {
		name string
		crd  *apiextv1.CustomResourceDefinition
		want bool
	}{
		{
			name: "v1CRD is not established & not accepting names - not ready",
			crd:  builder.ForV1CustomResourceDefinition("MyCRD").Result(),
			want: false,
		},
		{
			name: "v1CRD is established & not accepting names - not ready",
			crd: builder.ForV1CustomResourceDefinition("MyCRD").
				Condition(builder.ForV1CustomResourceDefinitionCondition().Type(apiextv1.Established).Status(apiextv1.ConditionTrue).Result()).Result(),
			want: false,
		},
		{
			name: "v1CRD is not established & accepting names - not ready",
			crd: builder.ForV1CustomResourceDefinition("MyCRD").
				Condition(builder.ForV1CustomResourceDefinitionCondition().Type(apiextv1.NamesAccepted).Status(apiextv1.ConditionTrue).Result()).Result(),
			want: false,
		},
		{
			name: "v1CRD is established & accepting names - ready",
			crd: builder.ForV1CustomResourceDefinition("MyCRD").
				Condition(builder.ForV1CustomResourceDefinitionCondition().Type(apiextv1.Established).Status(apiextv1.ConditionTrue).Result()).
				Condition(builder.ForV1CustomResourceDefinitionCondition().Type(apiextv1.NamesAccepted).Status(apiextv1.ConditionTrue).Result()).
				Result(),
			want: true,
		},
	}

	for _, tc := range v1tests {
		m, err := runtime.DefaultUnstructuredConverter.ToUnstructured(tc.crd)
		require.NoError(t, err)
		result, err := IsCRDReady(&unstructured.Unstructured{Object: m})
		require.NoError(t, err)
		assert.Equal(t, tc.want, result)
	}

	// input param is unrecognized
	resBytes := []byte(`
{
	"apiVersion": "apiextensions.k8s.io/v9",
	"kind": "CustomResourceDefinition",
	"metadata": {
		"name": "foos.example.foo.com"
	},
	"spec": {
		"group": "example.foo.com",
		"version": "v1alpha1",
		"scope": "Namespaced",
		"names": {
			"plural": "foos",
			"singular": "foo",
			"kind": "Foo"
		},
		"validation": {
			"openAPIV3Schema": {
				"required": [
					"spec"
				],
				"properties": {
					"spec": {
						"required": [
							"bar"
						],
						"properties": {
							"bar": {
								"type": "integer",
								"minimum": 1
							}
						}
					}
				}
			}
		}
	}
}
`)
	obj := &unstructured.Unstructured{}
	err := json.Unmarshal(resBytes, obj)
	require.NoError(t, err)
	_, err = IsCRDReady(obj)
	assert.NotNil(t, err)
}

func TestSinglePathMatch(t *testing.T) {
	fakeFS := velerotest.NewFakeFileSystem()
	fakeFS.MkdirAll("testDir1/subpath", 0755)
	fakeFS.MkdirAll("testDir2/subpath", 0755)

	_, err := SinglePathMatch("./*/subpath", fakeFS, logrus.StandardLogger())
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "expected one matching path")
}
