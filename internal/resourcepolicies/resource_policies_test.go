package resourcepolicies

import (
	"testing"

	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestLoadResourcePolicies(t *testing.T) {
	testCases := []struct {
		name     string
		yamlData string
		wantErr  bool
	}{
		{
			name: "unknown key in yaml",
			yamlData: `version: v1
	volumePolicies:
	- conditions:
		capacity: "0,100Gi"
		unknown: {}
		storageClass:
		- gp2
		- ebs-sc
	  action:
		type: skip`,
			wantErr: true,
		},
		{
			name: "reduplicated key in yaml",
			yamlData: `version: v1
	volumePolicies:
	- conditions:
		capacity: "0,100Gi"
		capacity: "0,100Gi"
		storageClass:
		- gp2
		- ebs-sc
	  action:
		type: skip`,
			wantErr: true,
		},
		{
			name: "error format of storageClass",
			yamlData: `version: v1
	volumePolicies:
	- conditions:
		capacity: "0,100Gi"
		storageClass: gp2
	  action:
		type: skip`,
			wantErr: true,
		},
		{
			name: "error format of csi",
			yamlData: `version: v1
	volumePolicies:
	- conditions:
		capacity: "0,100Gi"
		csi: gp2
	  action:
		type: skip`,
			wantErr: true,
		},
		{
			name: "error format of nfs",
			yamlData: `version: v1
	volumePolicies:
	- conditions:
		capacity: "0,100Gi"
		csi: {}
		nfs: abc
	  action:
		type: skip`,
			wantErr: true,
		},
		{
			name: "supported formart volume policies",
			yamlData: `version: v1
	volumePolicies:
	- conditions:
		capacity: "0,100Gi"
		csi:
		  driver: aws.efs.csi.driver
		nfs: {}
		storageClass:
		- gp2
		- ebs-sc
	  action:
		type: skip`,
			wantErr: true,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := unmarshalResourcePolicies(&tc.yamlData)

			if (err != nil) != tc.wantErr {
				t.Fatalf("Expected error %v, but got error %v", tc.wantErr, err)
			}
		})
	}
}

func TestGetResourceMatchedAction(t *testing.T) {
	resPolicies := &resourcePolicies{
		Version: "v1",
		VolumePolicies: []VolumePolicy{
			{
				Action: Action{Type: "skip"},
				Conditions: map[string]interface{}{
					"capacity":     "0,10Gi",
					"storageClass": []string{"gp2", "ebs-sc"},
					"csi": interface{}(
						map[string]interface{}{
							"driver": "aws.efs.csi.driver",
						}),
				},
			},
			{
				Action: Action{Type: "volume-snapshot"},
				Conditions: map[string]interface{}{
					"capacity":     "10,100Gi",
					"storageClass": []string{"gp2", "ebs-sc"},
					"csi": interface{}(
						map[string]interface{}{
							"driver": "aws.efs.csi.driver",
						}),
				},
			},
			{
				Action: Action{Type: "file-system-backup"},
				Conditions: map[string]interface{}{
					"storageClass": []string{"gp2", "ebs-sc"},
					"csi": interface{}(
						map[string]interface{}{
							"driver": "aws.efs.csi.driver",
						}),
				},
			},
		},
	}
	testCases := []struct {
		name           string
		volume         *StructuredVolume
		expectedAction *Action
	}{
		{
			name: "match policy",
			volume: &StructuredVolume{
				capacity:     *resource.NewQuantity(5<<30, resource.BinarySI),
				storageClass: "ebs-sc",
				csi:          &csiVolumeSource{Driver: "aws.efs.csi.driver"},
			},
			expectedAction: &Action{Type: "skip"},
		},
		{
			name: "both matches return the first policy",
			volume: &StructuredVolume{
				capacity:     *resource.NewQuantity(50<<30, resource.BinarySI),
				storageClass: "ebs-sc",
				csi:          &csiVolumeSource{Driver: "aws.efs.csi.driver"},
			},
			expectedAction: &Action{Type: "volume-snapshot"},
		},
		{
			name: "dismatch all policies",
			volume: &StructuredVolume{
				capacity:     *resource.NewQuantity(50<<30, resource.BinarySI),
				storageClass: "ebs-sc",
				nfs:          &nFSVolumeSource{},
			},
			expectedAction: nil,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			policies := &Policies{}
			err := policies.buildPolicy(resPolicies)
			if err != nil {
				t.Errorf("Failed to build policy with error %v", err)
			}

			action := policies.Match(tc.volume)
			if action == nil {
				if tc.expectedAction != nil {
					t.Errorf("Expected action %v, but got result nil", tc.expectedAction.Type)
				}
			} else {
				if tc.expectedAction != nil {
					if action.Type != tc.expectedAction.Type {
						t.Errorf("Expected action %v, but got result %v", tc.expectedAction.Type, action.Type)
					}
				} else {
					t.Errorf("Expected action nil, but got result %v", action.Type)
				}
			}
		})
	}
}

func TestGetResourcePoliciesFromConfig(t *testing.T) {
	// Create a test ConfigMap
	cm := &v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-configmap",
			Namespace: "test-namespace",
		},
		Data: map[string]string{
			"test-data": "version: v1\nvolumePolicies:\n- conditions:\n    capacity: '0,10Gi'\n  action:\n    type: skip",
		},
	}

	// Call the function and check for errors
	resPolicies, err := GetResourcePoliciesFromConfig(cm)
	assert.Nil(t, err)

	// Check that the returned resourcePolicies object contains the expected data
	assert.Equal(t, "v1", resPolicies.Version)
	assert.Len(t, resPolicies.VolumePolicies, 1)
	policies := resourcePolicies{
		Version: "v1",
		VolumePolicies: []VolumePolicy{
			{
				Conditions: map[string]interface{}{
					"capacity": "0,10Gi",
				},
				Action: Action{
					Type: Skip,
				},
			},
		},
	}
	p := &Policies{}
	err = p.buildPolicy(&policies)
	if err != nil {
		t.Fatalf("failed to build policy with error %v", err)
	}

	for k := range resPolicies.VolumePolicies[0].conditions {
		t.Logf("vae %v", resPolicies.VolumePolicies[0].conditions[k])
	}

	assert.Equal(t, p, resPolicies)
}
