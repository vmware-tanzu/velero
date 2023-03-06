package resourcepolicies

import (
	"context"
	"reflect"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestMGRGetVolumeMatchedAction(t *testing.T) {
	volume := &StructuredVolume{
		capacity: *resource.NewQuantity(5<<30, resource.BinarySI),
	}
	client := fake.NewClientBuilder().Build()
	testConfigmap := &v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-policies",
			Namespace: "velero",
			CreationTimestamp: metav1.Time{
				Time: time.Now().Add(-time.Hour),
			},
		},
		Data: map[string]string{
			"policies.yaml": "version: v1\nvolumePolicies:\n- conditions:\n    capacity: '1Gi,10Gi'\n  action:\n    type: skip\n",
		},
	}
	err := client.Create(context.Background(), testConfigmap)
	if err != nil {
		t.Errorf("Unexpected error: %v when create configmap", err)
	}
	ResPoliciesMgr.InitResPoliciesMgr("velero", client)

	// Test that the function returns the expected action for the test volume
	action, err := ResPoliciesMgr.GetVolumeMatchedAction("test-policies", "configmap", volume)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if action.Type != Skip {
		t.Errorf("Expected action type: %s, got: %s", Skip, action.Type)
	}

	// update configmap to check whether ResPoliciesMgr update resource policies
	testConfigmap.Data = map[string]string{
		"policies.yaml": "version: v1\nvolumePolicies:\n- conditions:\n    capacity: '1Gi,2Gi'\n  action:\n    type: skip\n",
	}
	testConfigmap.CreationTimestamp = metav1.Time{Time: time.Now()}
	err = client.Update(context.Background(), testConfigmap)
	if err != nil {
		t.Errorf("Unexpected error: %v when create configmap", err)
	}
	action, err = ResPoliciesMgr.GetVolumeMatchedAction("test-policies", "configmap", volume)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if action != nil {
		t.Errorf("Expected action dismatch volume but got: %s", action.Type)
	}

}

func TestGetResourcePolicies(t *testing.T) {
	testConfigmap := &v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-policies",
			Namespace: "velero",
			CreationTimestamp: metav1.Time{
				Time: time.Now().Add(-time.Hour),
			},
		},
		Data: map[string]string{
			"policies.yaml": "version: v1\nvolumePolicies:\n- conditions:\n    capacity: '1Gi,10Gi'\n  action:\n    type: skip\n",
		},
	}
	testPolicies := &ResourcePolicies{
		Version: "v1",
		VolumePolicies: []VolumePolicy{
			{
				Conditions: map[string]interface{}{
					"capacity": "1Gi,10Gi",
				},
				Action: Action{
					Type: Skip,
				},
			},
		},
	}
	client := fake.NewClientBuilder().Build()
	err := client.Create(context.Background(), testConfigmap)
	if err != nil {
		t.Errorf("Unexpected error: %v when create configmap", err)
	}
	ResPoliciesMgr.InitResPoliciesMgr("velero", client)
	// Call updateResourcePolicies to update the resource policies
	_, err = ResPoliciesMgr.getResourcePolicies("test-policies", "configmap")
	if err != nil {
		t.Fatalf("updateResourcePolicies failed: %v", err)
	}
	// Check that the policies were updated correctly
	key := resPoliciesKey{refName: "test-policies", refType: "configmap"}
	if meta, ok := ResPoliciesMgr.resPolicies[key]; !ok {
		t.Fatalf("expected resPolicies[%v] to exist, but it does not", key)
	} else if !reflect.DeepEqual(meta.policies, testPolicies) {
		t.Errorf("expected policies to be %v, but got %v", testPolicies, meta.policies)
	}
}

func TestGetStructredVolumeForPVC(t *testing.T) {
	pvc := &v1.PersistentVolumeClaim{
		Spec: v1.PersistentVolumeClaimSpec{
			VolumeName: "test-pv",
		},
		Status: v1.PersistentVolumeClaimStatus{
			Phase: v1.ClaimBound,
		},
	}
	pvc.Name = "test-pvc"
	pv := &v1.PersistentVolume{
		Spec: v1.PersistentVolumeSpec{
			PersistentVolumeSource: v1.PersistentVolumeSource{
				NFS: &v1.NFSVolumeSource{
					Server: "test-server",
					Path:   "/test-path",
				},
			},
		},
	}
	pv.Name = "test-pv"
	client := fake.NewClientBuilder().Build()
	err := client.Create(context.Background(), pv)
	if err != nil {
		t.Errorf("Unexpected error: %v when create pv", err)
	}
	err = client.Create(context.Background(), pvc)
	if err != nil {
		t.Errorf("Unexpected error: %v when create pvc", err)
	}
	ResPoliciesMgr.InitResPoliciesMgr("velero", client)
	expected := &StructuredVolume{
		nfs: &nFSVolumeSource{
			Server: "test-server",
			Path:   "/test-path",
		},
		csi: nil,
	}

	pvcObj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(pvc)
	assert.NoError(t, err)
	pvcUnstructured := &unstructured.Unstructured{Object: pvcObj}
	result, err := ResPoliciesMgr.GetStructredVolumeFromPVC(pvcUnstructured)

	assert.NoError(t, err)
	if !expected.capacity.Equal(result.capacity) {
		t.Errorf("the capacity is not as expected")
	}
	assert.Equal(t, expected.csi, result.csi)
	assert.Equal(t, expected.nfs, result.nfs)
	assert.Equal(t, expected.storageClass, result.storageClass)
}
