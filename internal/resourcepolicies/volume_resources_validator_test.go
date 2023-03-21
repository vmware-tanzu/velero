package resourcepolicies

import (
	"testing"

	"k8s.io/apimachinery/pkg/api/resource"
)

func TestCapacityConditionValidate(t *testing.T) {
	testCases := []struct {
		name     string
		capacity *capacity
		wantErr  bool
	}{
		{
			name:     "lower and upper are both zero",
			capacity: &capacity{lower: *resource.NewQuantity(0, resource.DecimalSI), upper: *resource.NewQuantity(0, resource.DecimalSI)},
			wantErr:  false,
		},
		{
			name:     "lower is zero and upper is greater than zero",
			capacity: &capacity{lower: *resource.NewQuantity(0, resource.DecimalSI), upper: *resource.NewQuantity(100, resource.DecimalSI)},
			wantErr:  false,
		},
		{
			name:     "lower is greater than upper",
			capacity: &capacity{lower: *resource.NewQuantity(100, resource.DecimalSI), upper: *resource.NewQuantity(50, resource.DecimalSI)},
			wantErr:  true,
		},
		{
			name:     "lower and upper are equal",
			capacity: &capacity{lower: *resource.NewQuantity(100, resource.DecimalSI), upper: *resource.NewQuantity(100, resource.DecimalSI)},
			wantErr:  false,
		},
		{
			name:     "lower is greater than zero and upper is zero",
			capacity: &capacity{lower: *resource.NewQuantity(100, resource.DecimalSI), upper: *resource.NewQuantity(0, resource.DecimalSI)},
			wantErr:  false,
		},
		{
			name:     "lower and upper are both not zero and lower is less than upper",
			capacity: &capacity{lower: *resource.NewQuantity(100, resource.DecimalSI), upper: *resource.NewQuantity(200, resource.DecimalSI)},
			wantErr:  false,
		},
		{
			name:     "lower and upper are both not zero and lower is equal to upper",
			capacity: &capacity{lower: *resource.NewQuantity(100, resource.DecimalSI), upper: *resource.NewQuantity(100, resource.DecimalSI)},
			wantErr:  false,
		},
		{
			name:     "lower and upper are both not zero and lower is greater than upper",
			capacity: &capacity{lower: *resource.NewQuantity(200, resource.DecimalSI), upper: *resource.NewQuantity(100, resource.DecimalSI)},
			wantErr:  true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			c := &capacityCondition{capacity: *tc.capacity}
			err := c.validate()

			if (err != nil) != tc.wantErr {
				t.Fatalf("Expected error %v, but got error %v", tc.wantErr, err)
			}
		})
	}
}

func TestValidate(t *testing.T) {
	testCases := []struct {
		name    string
		res     *resourcePolicies
		wantErr bool
	}{
		{
			name: "unknown key in yaml",
			res: &resourcePolicies{
				Version: "v1",
				VolumePolicies: []volumePolicy{
					{
						Action: Action{Type: "skip"},
						Conditions: map[string]interface{}{
							"capacity":     "0,10Gi",
							"unknown":      "",
							"storageClass": []string{"gp2", "ebs-sc"},
							"csi": interface{}(
								map[string]interface{}{
									"driver": "aws.efs.csi.driver",
								}),
						},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "error format of capacity",
			res: &resourcePolicies{
				Version: "v1",
				VolumePolicies: []volumePolicy{
					{
						Action: Action{Type: "skip"},
						Conditions: map[string]interface{}{
							"capacity":     "10Gi",
							"storageClass": []string{"gp2", "ebs-sc"},
							"csi": interface{}(
								map[string]interface{}{
									"driver": "aws.efs.csi.driver",
								}),
						},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "error format of storageClass",
			res: &resourcePolicies{
				Version: "v1",
				VolumePolicies: []volumePolicy{
					{
						Action: Action{Type: "skip"},
						Conditions: map[string]interface{}{
							"capacity":     "0,10Gi",
							"storageClass": "ebs-sc",
							"csi": interface{}(
								map[string]interface{}{
									"driver": "aws.efs.csi.driver",
								}),
						},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "error format of csi",
			res: &resourcePolicies{
				Version: "v1",
				VolumePolicies: []volumePolicy{
					{
						Action: Action{Type: "skip"},
						Conditions: map[string]interface{}{
							"capacity":     "0,10Gi",
							"storageClass": []string{"gp2", "ebs-sc"},
							"csi":          "aws.efs.csi.driver",
						},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "unsupported version",
			res: &resourcePolicies{
				Version: "v2",
				VolumePolicies: []volumePolicy{
					{
						Action: Action{Type: "skip"},
						Conditions: map[string]interface{}{
							"capacity": "0,10Gi",
							"csi": interface{}(
								map[string]interface{}{
									"driver": "aws.efs.csi.driver",
								}),
						},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "unsupported action",
			res: &resourcePolicies{
				Version: "v1",
				VolumePolicies: []volumePolicy{
					{
						Action: Action{Type: "unsupported"},
						Conditions: map[string]interface{}{
							"capacity": "0,10Gi",
							"csi": interface{}(
								map[string]interface{}{
									"driver": "aws.efs.csi.driver",
								}),
						},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "error format of nfs",
			res: &resourcePolicies{
				Version: "v1",
				VolumePolicies: []volumePolicy{
					{
						Action: Action{Type: "skip"},
						Conditions: map[string]interface{}{
							"capacity":     "0,10Gi",
							"storageClass": []string{"gp2", "ebs-sc"},
							"nfs":          "aws.efs.csi.driver",
						},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "supported formart volume policies",
			res: &resourcePolicies{
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
							"nfs": interface{}(
								map[string]interface{}{
									"server": "192.168.20.90",
									"path":   "/mnt/data/",
								}),
						},
					},
				},
			},
			wantErr: false,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			policies := &Policies{}
			err1 := policies.buildPolicy(tc.res)
			err2 := policies.Validate()

			if tc.wantErr {
				if err1 == nil && err2 == nil {
					t.Fatalf("Expected error %v, but not get error", tc.wantErr)
				}
			} else {
				if err1 != nil || err2 != nil {
					t.Fatalf("Expected error %v, but got error %v %v", tc.wantErr, err1, err2)
				}
			}
		})
	}
}
