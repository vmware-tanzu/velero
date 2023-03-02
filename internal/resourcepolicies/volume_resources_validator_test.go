package resourcepolicies

import (
	"testing"

	"k8s.io/apimachinery/pkg/api/resource"
)

func TestCapacityConditionValidate(t *testing.T) {
	testCases := []struct {
		name     string
		capacity *Capacity
		want     bool
		wantErr  bool
	}{
		{
			name:     "lower and upper are both zero",
			capacity: &Capacity{lower: *resource.NewQuantity(0, resource.DecimalSI), upper: *resource.NewQuantity(0, resource.DecimalSI)},
			want:     true,
			wantErr:  false,
		},
		{
			name:     "lower is zero and upper is greater than zero",
			capacity: &Capacity{lower: *resource.NewQuantity(0, resource.DecimalSI), upper: *resource.NewQuantity(100, resource.DecimalSI)},
			want:     true,
			wantErr:  false,
		},
		{
			name:     "lower is greater than upper",
			capacity: &Capacity{lower: *resource.NewQuantity(100, resource.DecimalSI), upper: *resource.NewQuantity(50, resource.DecimalSI)},
			want:     false,
			wantErr:  true,
		},
		{
			name:     "lower and upper are equal",
			capacity: &Capacity{lower: *resource.NewQuantity(100, resource.DecimalSI), upper: *resource.NewQuantity(100, resource.DecimalSI)},
			want:     true,
			wantErr:  false,
		},
		{
			name:     "lower is greater than zero and upper is zero",
			capacity: &Capacity{lower: *resource.NewQuantity(100, resource.DecimalSI), upper: *resource.NewQuantity(0, resource.DecimalSI)},
			want:     true,
			wantErr:  false,
		},
		{
			name:     "lower and upper are both not zero and lower is less than upper",
			capacity: &Capacity{lower: *resource.NewQuantity(100, resource.DecimalSI), upper: *resource.NewQuantity(200, resource.DecimalSI)},
			want:     true,
			wantErr:  false,
		},
		{
			name:     "lower and upper are both not zero and lower is equal to upper",
			capacity: &Capacity{lower: *resource.NewQuantity(100, resource.DecimalSI), upper: *resource.NewQuantity(100, resource.DecimalSI)},
			want:     true,
			wantErr:  false,
		},
		{
			name:     "lower and upper are both not zero and lower is greater than upper",
			capacity: &Capacity{lower: *resource.NewQuantity(200, resource.DecimalSI), upper: *resource.NewQuantity(100, resource.DecimalSI)},
			want:     false,
			wantErr:  true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			c := &capacityCondition{capacity: *tc.capacity}
			got, err := c.Validate()

			if (err != nil) != tc.wantErr {
				t.Fatalf("Expected error %v, but got error %v", tc.wantErr, err)
			}

			if got != tc.want {
				t.Errorf("Expected result %v, but got result %v", tc.want, got)
			}
		})
	}
}

func TestValidate(t *testing.T) {
	testCases := []struct {
		name     string
		yamlData string
		want     bool
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
			want:    false,
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
			want:    false,
			wantErr: true,
		},
		{
			name: "error format of capacity",
			yamlData: `version: v1
volumePolicies:
- conditions:
    capacity: "100Gi"
    storageClass:
    - gp2
    - ebs-sc
  action:
    type: skip`,
			want:    false,
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
			want:    false,
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
			want:    false,
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
			want:    false,
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
    nfs:
	  server: 192.168.20.90
	  path: /mnt/data/
    storageClass:
    - gp2
    - ebs-sc
  action:
    type: skip`,
			want:    false,
			wantErr: true,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := Validate(&tc.yamlData)

			if (err != nil) != tc.wantErr {
				t.Fatalf("Expected error %v, but got error %v", tc.wantErr, err)
			}

			if got != tc.want {
				t.Errorf("Expected result %v, but got result %v", tc.want, got)
			}
		})
	}
}
