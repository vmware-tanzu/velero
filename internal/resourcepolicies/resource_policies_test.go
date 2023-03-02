package resourcepolicies

import (
	"testing"

	"k8s.io/apimachinery/pkg/api/resource"
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
			_, err := LoadResourcePolicies(&tc.yamlData)

			if (err != nil) != tc.wantErr {
				t.Fatalf("Expected error %v, but got error %v", tc.wantErr, err)
			}
		})
	}
}

func TestGetVolumeMatchedAction(t *testing.T) {
	resPolicies := &ResourcePolicies{
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
			action := GetVolumeMatchedAction(resPolicies, tc.volume)
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
