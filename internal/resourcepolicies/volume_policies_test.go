package resourcepolicies_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/vmware-tanzu/velero/internal/resourcepolicies"
	"github.com/zeebo/assert"
	"k8s.io/apimachinery/pkg/api/resource"
)

func TestParseCapacity(t *testing.T) {
	tests := []struct {
		input       string
		expected    *resourcepolicies.Capacity
		expectedErr error
	}{
		{"10Gi,20Gi", resourcepolicies.SetCapacity(*resource.NewQuantity(10<<30, resource.BinarySI), *resource.NewQuantity(20<<30, resource.BinarySI)), nil},
		{"10Gi,", resourcepolicies.SetCapacity(*resource.NewQuantity(10<<30, resource.BinarySI), *resource.NewQuantity(0, resource.DecimalSI)), nil},
		{"10Gi", nil, fmt.Errorf("wrong format of Capacity 10Gi")},
		{"", nil, fmt.Errorf("wrong format of Capacity ")},
	}

	for _, test := range tests {
		test := test // capture range variable
		t.Run(test.input, func(t *testing.T) {
			actual, actualErr := resourcepolicies.ParseCapacity(test.input)
			if test.expected != nil {
				assert.Equal(t, test.expected.GetLowerCapacity().Cmp(*actual.GetLowerCapacity()), 0)
				assert.Equal(t, test.expected.GetUpperCapacity().Cmp(*actual.GetUpperCapacity()), 0)
			}
			assert.Equal(t, test.expectedErr, actualErr)
		})
	}
}

func TestCapacity_IsInRange(t *testing.T) {
	t.Parallel()

	tests := []struct {
		capacity  *resourcepolicies.Capacity
		quantity  resource.Quantity
		isInRange bool
	}{
		{resourcepolicies.SetCapacity(*resource.NewQuantity(0, resource.BinarySI), *resource.NewQuantity(10<<30, resource.BinarySI)), *resource.NewQuantity(5<<30, resource.BinarySI), true},
		{resourcepolicies.SetCapacity(*resource.NewQuantity(0, resource.BinarySI), *resource.NewQuantity(10<<30, resource.BinarySI)), *resource.NewQuantity(15<<30, resource.BinarySI), false},
		{resourcepolicies.SetCapacity(*resource.NewQuantity(20<<30, resource.BinarySI), *resource.NewQuantity(0, resource.DecimalSI)), *resource.NewQuantity(25<<30, resource.BinarySI), true},
		{resourcepolicies.SetCapacity(*resource.NewQuantity(20<<30, resource.BinarySI), *resource.NewQuantity(0, resource.DecimalSI)), *resource.NewQuantity(15<<30, resource.BinarySI), false},
		{resourcepolicies.SetCapacity(*resource.NewQuantity(10<<30, resource.BinarySI), *resource.NewQuantity(20<<30, resource.BinarySI)), *resource.NewQuantity(15<<30, resource.BinarySI), true},
		{resourcepolicies.SetCapacity(*resource.NewQuantity(10<<30, resource.BinarySI), *resource.NewQuantity(20<<30, resource.BinarySI)), *resource.NewQuantity(5<<30, resource.BinarySI), false},
		{resourcepolicies.SetCapacity(*resource.NewQuantity(10<<30, resource.BinarySI), *resource.NewQuantity(20<<30, resource.BinarySI)), *resource.NewQuantity(25<<30, resource.BinarySI), false},
		{resourcepolicies.SetCapacity(*resource.NewQuantity(0, resource.BinarySI), *resource.NewQuantity(0, resource.BinarySI)), *resource.NewQuantity(5<<30, resource.BinarySI), true},
	}

	for _, test := range tests {
		test := test // capture range variable
		t.Run(fmt.Sprintf("%v with %v", test.capacity, test.quantity), func(t *testing.T) {
			t.Parallel()

			actual := test.capacity.IsInRange(test.quantity)

			assert.Equal(t, test.isInRange, actual)

		})
	}
}

func TestStorageClassConditionMatch(t *testing.T) {
	tests := []struct {
		name          string
		condition     *resourcepolicies.StorageClassCondition
		volume        *resourcepolicies.StructuredVolume
		expectedMatch bool
	}{
		{
			name:          "match single storage class",
			condition:     resourcepolicies.SetStorageClassCondition([]string{"gp2"}),
			volume:        resourcepolicies.SetStructuredVolume(*resource.NewQuantity(0, resource.BinarySI), "gp2", nil, nil),
			expectedMatch: true,
		},
		{
			name:          "match multiple storage classes",
			condition:     resourcepolicies.SetStorageClassCondition([]string{"gp2", "ebs-sc"}),
			volume:        resourcepolicies.SetStructuredVolume(*resource.NewQuantity(0, resource.BinarySI), "gp2", nil, nil),
			expectedMatch: true,
		},
		{
			name:          "mismatch storage class",
			condition:     resourcepolicies.SetStorageClassCondition([]string{"gp2"}),
			volume:        resourcepolicies.SetStructuredVolume(*resource.NewQuantity(0, resource.BinarySI), "ebs-sc", nil, nil),
			expectedMatch: false,
		},
		{
			name:          "empty storage class",
			condition:     resourcepolicies.SetStorageClassCondition([]string{}),
			volume:        resourcepolicies.SetStructuredVolume(*resource.NewQuantity(0, resource.BinarySI), "ebs-sc", nil, nil),
			expectedMatch: true,
		},
		{
			name:          "empty volume storage class",
			condition:     resourcepolicies.SetStorageClassCondition([]string{"gp2"}),
			volume:        resourcepolicies.SetStructuredVolume(*resource.NewQuantity(0, resource.BinarySI), "", nil, nil),
			expectedMatch: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			match := tt.condition.Match(tt.volume)
			if match != tt.expectedMatch {
				t.Errorf("expected %v, but got %v", tt.expectedMatch, match)
			}
		})
	}
}

func TestNFSCondition_Match(t *testing.T) {

	tests := []struct {
		name          string
		condition     *resourcepolicies.NFSCondition
		volume        *resourcepolicies.StructuredVolume
		expectedMatch bool
	}{
		{
			name:          "match nfs conditon",
			condition:     resourcepolicies.SetNFSCondition(&struct{}{}),
			volume:        resourcepolicies.SetStructuredVolume(*resource.NewQuantity(0, resource.BinarySI), "", &struct{}{}, nil),
			expectedMatch: true,
		},
		{
			name:          "emtpy nfs condition",
			condition:     resourcepolicies.SetNFSCondition(nil),
			volume:        resourcepolicies.SetStructuredVolume(*resource.NewQuantity(0, resource.BinarySI), "", &struct{}{}, nil),
			expectedMatch: true,
		},
		{
			name:          "emtpy nfs volume",
			condition:     resourcepolicies.SetNFSCondition(&struct{}{}),
			volume:        resourcepolicies.SetStructuredVolume(*resource.NewQuantity(0, resource.BinarySI), "", nil, nil),
			expectedMatch: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			match := tt.condition.Match(tt.volume)
			if match != tt.expectedMatch {
				t.Errorf("expected %v, but got %v", tt.expectedMatch, match)
			}
		})
	}
}

func TestCSICondition_Match(t *testing.T) {

	tests := []struct {
		name          string
		condition     *resourcepolicies.CSICondition
		volume        *resourcepolicies.StructuredVolume
		expectedMatch bool
	}{
		{
			name:          "match csi conditon",
			condition:     resourcepolicies.SetCSICondition(&resourcepolicies.CSIVolumeSource{Driver: "test"}),
			volume:        resourcepolicies.SetStructuredVolume(*resource.NewQuantity(0, resource.BinarySI), "", nil, &resourcepolicies.CSIVolumeSource{Driver: "test"}),
			expectedMatch: true,
		},
		{
			name:          "empty csi conditon",
			condition:     resourcepolicies.SetCSICondition(nil),
			volume:        resourcepolicies.SetStructuredVolume(*resource.NewQuantity(0, resource.BinarySI), "", nil, &resourcepolicies.CSIVolumeSource{Driver: "test"}),
			expectedMatch: true,
		},
		{
			name:          "empty csi volume",
			condition:     resourcepolicies.SetCSICondition(&resourcepolicies.CSIVolumeSource{Driver: "test"}),
			volume:        resourcepolicies.SetStructuredVolume(*resource.NewQuantity(0, resource.BinarySI), "", nil, &resourcepolicies.CSIVolumeSource{}),
			expectedMatch: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			match := tt.condition.Match(tt.volume)
			if match != tt.expectedMatch {
				t.Errorf("expected %v, but got %v", tt.expectedMatch, match)
			}
		})
	}
}

func TestUnmarshalVolumeConditions(t *testing.T) {
	testCases := []struct {
		name          string
		input         map[string]interface{}
		expectedError string
	}{
		{
			name: "Valid input",
			input: map[string]interface{}{
				"capacity": "1Gi,10Gi",
				"storageClass": []string{
					"gp2",
					"ebs-sc",
				},
				"csi": &resourcepolicies.CSIVolumeSource{
					Driver: "aws.efs.csi.driver",
				},
			},
			expectedError: "",
		},
		{
			name: "Invalid input: invalid capacity filed name",
			input: map[string]interface{}{
				"Capacity": "1Gi,10Gi",
			},
			expectedError: "field Capacity not found",
		},
		{
			name: "Invalid input: invalid storage class format",
			input: map[string]interface{}{
				"storageClass": "ebs-sc",
			},
			expectedError: "str `ebs-sc` into []string",
		},
		{
			name: "Invalid input: invalid csi format",
			input: map[string]interface{}{
				"csi": "csi.driver",
			},
			expectedError: "str `csi.driver` into resourcepolicies.CSIVolumeSource",
		},
		{
			name: "Invalid input: unknown field",
			input: map[string]interface{}{
				"unknown": "foo",
			},
			expectedError: "field unknown not found in type",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := resourcepolicies.UnmarshalVolConditions(tc.input)
			if tc.expectedError != "" {
				if err == nil {
					t.Errorf("Expected error '%s', but got nil", tc.expectedError)
				} else if !strings.Contains(err.Error(), tc.expectedError) {
					t.Errorf("Expected error '%s', but got '%v'", tc.expectedError, err)
				}
			}
		})
	}
}
