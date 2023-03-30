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
		VolumePolicies: []volumePolicy{
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
		volume         *structuredVolume
		expectedAction *Action
	}{
		{
			name: "match policy",
			volume: &structuredVolume{
				capacity:     *resource.NewQuantity(5<<30, resource.BinarySI),
				storageClass: "ebs-sc",
				csi:          &csiVolumeSource{Driver: "aws.efs.csi.driver"},
			},
			expectedAction: &Action{Type: "skip"},
		},
		{
			name: "both matches return the first policy",
			volume: &structuredVolume{
				capacity:     *resource.NewQuantity(50<<30, resource.BinarySI),
				storageClass: "ebs-sc",
				csi:          &csiVolumeSource{Driver: "aws.efs.csi.driver"},
			},
			expectedAction: &Action{Type: "volume-snapshot"},
		},
		{
			name: "dismatch all policies",
			volume: &structuredVolume{
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

			action := policies.match(tc.volume)
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
	assert.Equal(t, "v1", resPolicies.version)
	assert.Len(t, resPolicies.volumePolicies, 1)
	policies := resourcePolicies{
		Version: "v1",
		VolumePolicies: []volumePolicy{
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
	assert.Equal(t, p, resPolicies)
}

func TestGetMatchAction(t *testing.T) {
	testCases := []struct {
		name     string
		yamlData string
		vol      *v1.PersistentVolume
		skip     bool
	}{
		{
			name: "empty csi",
			yamlData: `version: v1
volumePolicies:
- conditions:
   csi: {}
  action:
    type: skip`,
			vol: &v1.PersistentVolume{
				Spec: v1.PersistentVolumeSpec{
					PersistentVolumeSource: v1.PersistentVolumeSource{
						CSI: &v1.CSIPersistentVolumeSource{Driver: "aws.ebs.csi.driver"},
					}},
			},
			skip: true,
		},
		{
			name: "empty csi with pv no csi driver",
			yamlData: `version: v1
volumePolicies:
- conditions:
   csi: {}
  action:
    type: skip`,
			vol: &v1.PersistentVolume{
				Spec: v1.PersistentVolumeSpec{
					Capacity: v1.ResourceList{
						v1.ResourceStorage: resource.MustParse("1Gi"),
					}},
			},
			skip: false,
		},
		{
			name: "csi not configured",
			yamlData: `version: v1
volumePolicies:
- conditions:
    capacity: "0,100Gi"
  action:
    type: skip`,
			vol: &v1.PersistentVolume{
				Spec: v1.PersistentVolumeSpec{
					Capacity: v1.ResourceList{
						v1.ResourceStorage: resource.MustParse("1Gi"),
					},
					PersistentVolumeSource: v1.PersistentVolumeSource{
						CSI: &v1.CSIPersistentVolumeSource{Driver: "aws.ebs.csi.driver"},
					}},
			},
			skip: true,
		},
		{
			name: "empty nfs",
			yamlData: `version: v1
volumePolicies:
- conditions:
    nfs: {}
  action:
    type: skip`,
			vol: &v1.PersistentVolume{
				Spec: v1.PersistentVolumeSpec{
					PersistentVolumeSource: v1.PersistentVolumeSource{
						NFS: &v1.NFSVolumeSource{Server: "192.168.1.20"},
					}},
			},
			skip: true,
		},
		{
			name: "nfs not configured",
			yamlData: `version: v1
volumePolicies:
- conditions:
    capacity: "0,100Gi"
  action:
    type: skip`,
			vol: &v1.PersistentVolume{
				Spec: v1.PersistentVolumeSpec{
					Capacity: v1.ResourceList{
						v1.ResourceStorage: resource.MustParse("1Gi"),
					},
					PersistentVolumeSource: v1.PersistentVolumeSource{
						NFS: &v1.NFSVolumeSource{Server: "192.168.1.20"},
					},
				},
			},
			skip: true,
		},
		{
			name: "empty nfs with pv no nfs volume source",
			yamlData: `version: v1
volumePolicies:
- conditions:
    capacity: "0,100Gi"
    nfs: {}
  action:
    type: skip`,
			vol: &v1.PersistentVolume{
				Spec: v1.PersistentVolumeSpec{
					Capacity: v1.ResourceList{
						v1.ResourceStorage: resource.MustParse("1Gi"),
					},
				},
			},
			skip: false,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			resPolicies, err := unmarshalResourcePolicies(&tc.yamlData)
			if err != nil {
				t.Fatalf("got error when get match action %v", err)
			}
			assert.Nil(t, err)
			policies := &Policies{}
			err = policies.buildPolicy(resPolicies)
			assert.Nil(t, err)
			action, err := policies.GetMatchAction(tc.vol)
			assert.Nil(t, err)

			if tc.skip {
				if action.Type != Skip {
					t.Fatalf("Expected action skip but is %v", action.Type)
				}
			} else if action != nil && action.Type == Skip {
				t.Fatalf("Expected action not skip but is %v", action.Type)
			}
		})
	}
}
