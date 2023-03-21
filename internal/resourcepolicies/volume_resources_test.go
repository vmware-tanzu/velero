package resourcepolicies

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	corev1api "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

func setStructuredVolume(capacity resource.Quantity, sc string, nfs *nFSVolumeSource, csi *csiVolumeSource) *structuredVolume {
	return &structuredVolume{
		capacity:     capacity,
		storageClass: sc,
		nfs:          nfs,
		csi:          csi,
	}
}

func TestParseCapacity(t *testing.T) {
	var emptyCapacity capacity
	tests := []struct {
		input       string
		expected    capacity
		expectedErr error
	}{
		{"10Gi,20Gi", capacity{lower: *resource.NewQuantity(10<<30, resource.BinarySI), upper: *resource.NewQuantity(20<<30, resource.BinarySI)}, nil},
		{"10Gi,", capacity{lower: *resource.NewQuantity(10<<30, resource.BinarySI), upper: *resource.NewQuantity(0, resource.DecimalSI)}, nil},
		{"10Gi", emptyCapacity, fmt.Errorf("wrong format of Capacity 10Gi")},
		{"", emptyCapacity, nil},
	}

	for _, test := range tests {
		test := test // capture range variable
		t.Run(test.input, func(t *testing.T) {
			actual, actualErr := parseCapacity(test.input)
			if test.expected != emptyCapacity {
				assert.Equal(t, test.expected.lower.Cmp(actual.lower), 0)
				assert.Equal(t, test.expected.upper.Cmp(actual.upper), 0)
			}
			assert.Equal(t, test.expectedErr, actualErr)
		})
	}
}

func TestCapacityIsInRange(t *testing.T) {
	t.Parallel()

	tests := []struct {
		capacity  *capacity
		quantity  resource.Quantity
		isInRange bool
	}{
		{&capacity{*resource.NewQuantity(0, resource.BinarySI), *resource.NewQuantity(10<<30, resource.BinarySI)}, *resource.NewQuantity(5<<30, resource.BinarySI), true},
		{&capacity{*resource.NewQuantity(0, resource.BinarySI), *resource.NewQuantity(10<<30, resource.BinarySI)}, *resource.NewQuantity(15<<30, resource.BinarySI), false},
		{&capacity{*resource.NewQuantity(20<<30, resource.BinarySI), *resource.NewQuantity(0, resource.DecimalSI)}, *resource.NewQuantity(25<<30, resource.BinarySI), true},
		{&capacity{*resource.NewQuantity(20<<30, resource.BinarySI), *resource.NewQuantity(0, resource.DecimalSI)}, *resource.NewQuantity(15<<30, resource.BinarySI), false},
		{&capacity{*resource.NewQuantity(10<<30, resource.BinarySI), *resource.NewQuantity(20<<30, resource.BinarySI)}, *resource.NewQuantity(15<<30, resource.BinarySI), true},
		{&capacity{*resource.NewQuantity(10<<30, resource.BinarySI), *resource.NewQuantity(20<<30, resource.BinarySI)}, *resource.NewQuantity(5<<30, resource.BinarySI), false},
		{&capacity{*resource.NewQuantity(10<<30, resource.BinarySI), *resource.NewQuantity(20<<30, resource.BinarySI)}, *resource.NewQuantity(25<<30, resource.BinarySI), false},
		{&capacity{*resource.NewQuantity(0, resource.BinarySI), *resource.NewQuantity(0, resource.BinarySI)}, *resource.NewQuantity(5<<30, resource.BinarySI), true},
	}

	for _, test := range tests {
		test := test // capture range variable
		t.Run(fmt.Sprintf("%v with %v", test.capacity, test.quantity), func(t *testing.T) {
			t.Parallel()

			actual := test.capacity.isInRange(test.quantity)

			assert.Equal(t, test.isInRange, actual)

		})
	}
}

func TestStorageClassConditionMatch(t *testing.T) {
	tests := []struct {
		name          string
		condition     *storageClassCondition
		volume        *structuredVolume
		expectedMatch bool
	}{
		{
			name:          "match single storage class",
			condition:     &storageClassCondition{[]string{"gp2"}},
			volume:        setStructuredVolume(*resource.NewQuantity(0, resource.BinarySI), "gp2", nil, nil),
			expectedMatch: true,
		},
		{
			name:          "match multiple storage classes",
			condition:     &storageClassCondition{[]string{"gp2", "ebs-sc"}},
			volume:        setStructuredVolume(*resource.NewQuantity(0, resource.BinarySI), "gp2", nil, nil),
			expectedMatch: true,
		},
		{
			name:          "mismatch storage class",
			condition:     &storageClassCondition{[]string{"gp2"}},
			volume:        setStructuredVolume(*resource.NewQuantity(0, resource.BinarySI), "ebs-sc", nil, nil),
			expectedMatch: false,
		},
		{
			name:          "empty storage class",
			condition:     &storageClassCondition{[]string{}},
			volume:        setStructuredVolume(*resource.NewQuantity(0, resource.BinarySI), "ebs-sc", nil, nil),
			expectedMatch: true,
		},
		{
			name:          "empty volume storage class",
			condition:     &storageClassCondition{[]string{"gp2"}},
			volume:        setStructuredVolume(*resource.NewQuantity(0, resource.BinarySI), "", nil, nil),
			expectedMatch: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			match := tt.condition.match(tt.volume)
			if match != tt.expectedMatch {
				t.Errorf("expected %v, but got %v", tt.expectedMatch, match)
			}
		})
	}
}

func TestNFSConditionMatch(t *testing.T) {

	tests := []struct {
		name          string
		condition     *nfsCondition
		volume        *structuredVolume
		expectedMatch bool
	}{
		{
			name:          "match nfs condition",
			condition:     &nfsCondition{&nFSVolumeSource{Server: "192.168.10.20"}},
			volume:        setStructuredVolume(*resource.NewQuantity(0, resource.BinarySI), "", &nFSVolumeSource{Server: "192.168.10.20"}, nil),
			expectedMatch: true,
		},
		{
			name:          "empty nfs condition",
			condition:     &nfsCondition{nil},
			volume:        setStructuredVolume(*resource.NewQuantity(0, resource.BinarySI), "", &nFSVolumeSource{Server: "192.168.10.20"}, nil),
			expectedMatch: true,
		},
		{
			name:          "empty nfs server and path condition",
			condition:     &nfsCondition{&nFSVolumeSource{Server: "", Path: ""}},
			volume:        setStructuredVolume(*resource.NewQuantity(0, resource.BinarySI), "", &nFSVolumeSource{Server: "192.168.10.20"}, nil),
			expectedMatch: true,
		},
		{
			name:          "server dismatch",
			condition:     &nfsCondition{&nFSVolumeSource{Server: "192.168.10.20", Path: ""}},
			volume:        setStructuredVolume(*resource.NewQuantity(0, resource.BinarySI), "", &nFSVolumeSource{Server: ""}, nil),
			expectedMatch: false,
		},
		{
			name:          "empty nfs server condition",
			condition:     &nfsCondition{&nFSVolumeSource{Path: "/mnt/data"}},
			volume:        setStructuredVolume(*resource.NewQuantity(0, resource.BinarySI), "", &nFSVolumeSource{Server: "192.168.10.20", Path: "/mnt/data"}, nil),
			expectedMatch: true,
		},
		{
			name:          "empty nfs volume",
			condition:     &nfsCondition{&nFSVolumeSource{Server: "192.168.10.20"}},
			volume:        setStructuredVolume(*resource.NewQuantity(0, resource.BinarySI), "", nil, nil),
			expectedMatch: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			match := tt.condition.match(tt.volume)
			if match != tt.expectedMatch {
				t.Errorf("expected %v, but got %v", tt.expectedMatch, match)
			}
		})
	}
}

func TestCSIConditionMatch(t *testing.T) {

	tests := []struct {
		name          string
		condition     *csiCondition
		volume        *structuredVolume
		expectedMatch bool
	}{
		{
			name:          "match csi condition",
			condition:     &csiCondition{&csiVolumeSource{Driver: "test"}},
			volume:        setStructuredVolume(*resource.NewQuantity(0, resource.BinarySI), "", nil, &csiVolumeSource{Driver: "test"}),
			expectedMatch: true,
		},
		{
			name:          "empty csi condition",
			condition:     &csiCondition{nil},
			volume:        setStructuredVolume(*resource.NewQuantity(0, resource.BinarySI), "", nil, &csiVolumeSource{Driver: "test"}),
			expectedMatch: true,
		},
		{
			name:          "empty csi volume",
			condition:     &csiCondition{&csiVolumeSource{Driver: "test"}},
			volume:        setStructuredVolume(*resource.NewQuantity(0, resource.BinarySI), "", nil, &csiVolumeSource{}),
			expectedMatch: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			match := tt.condition.match(tt.volume)
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
				"csi": &csiVolumeSource{
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
			expectedError: "str `csi.driver` into resourcepolicies.csiVolumeSource",
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
			_, err := unmarshalVolConditions(tc.input)
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

func TestParsePodVolume(t *testing.T) {
	// Mock data
	nfsVolume := corev1api.Volume{}
	nfsVolume.NFS = &corev1api.NFSVolumeSource{
		Server: "nfs.example.com",
		Path:   "/exports/data",
	}
	csiVolume := corev1api.Volume{}
	csiVolume.CSI = &corev1api.CSIVolumeSource{
		Driver: "csi.example.com",
	}
	emptyVolume := corev1api.Volume{}

	// Test cases
	testCases := []struct {
		name        string
		inputVolume *corev1api.Volume
		expectedNFS *nFSVolumeSource
		expectedCSI *csiVolumeSource
	}{
		{
			name:        "NFS volume",
			inputVolume: &nfsVolume,
			expectedNFS: &nFSVolumeSource{Server: "nfs.example.com", Path: "/exports/data"},
		},
		{
			name:        "CSI volume",
			inputVolume: &csiVolume,
			expectedCSI: &csiVolumeSource{Driver: "csi.example.com"},
		},
		{
			name:        "Empty volume",
			inputVolume: &emptyVolume,
			expectedNFS: nil,
			expectedCSI: nil,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Call the function
			structuredVolume := &structuredVolume{}
			structuredVolume.parsePodVolume(tc.inputVolume)

			// Check the results
			if tc.expectedNFS != nil {
				if structuredVolume.nfs == nil {
					t.Errorf("Expected a non-nil NFS volume source")
				} else if *tc.expectedNFS != *structuredVolume.nfs {
					t.Errorf("NFS volume source does not match expected value")
				}
			}
			if tc.expectedCSI != nil {
				if structuredVolume.csi == nil {
					t.Errorf("Expected a non-nil CSI volume source")
				} else if *tc.expectedCSI != *structuredVolume.csi {
					t.Errorf("CSI volume source does not match expected value")
				}
			}
		})
	}
}

func TestParsePV(t *testing.T) {
	// Mock data
	nfsVolume := corev1api.PersistentVolume{}
	nfsVolume.Spec.Capacity = corev1api.ResourceList{corev1api.ResourceStorage: resource.MustParse("1Gi")}
	nfsVolume.Spec.NFS = &corev1api.NFSVolumeSource{Server: "nfs.example.com", Path: "/exports/data"}
	csiVolume := corev1api.PersistentVolume{}
	csiVolume.Spec.Capacity = corev1api.ResourceList{corev1api.ResourceStorage: resource.MustParse("2Gi")}
	csiVolume.Spec.CSI = &corev1api.CSIPersistentVolumeSource{Driver: "csi.example.com"}
	emptyVolume := corev1api.PersistentVolume{}

	// Test cases
	testCases := []struct {
		name        string
		inputVolume *corev1api.PersistentVolume
		expectedNFS *nFSVolumeSource
		expectedCSI *csiVolumeSource
	}{
		{
			name:        "NFS volume",
			inputVolume: &nfsVolume,
			expectedNFS: &nFSVolumeSource{Server: "nfs.example.com", Path: "/exports/data"},
			expectedCSI: nil,
		},
		{
			name:        "CSI volume",
			inputVolume: &csiVolume,
			expectedNFS: nil,
			expectedCSI: &csiVolumeSource{Driver: "csi.example.com"},
		},
		{
			name:        "Empty volume",
			inputVolume: &emptyVolume,
			expectedNFS: nil,
			expectedCSI: nil,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Call the function
			structuredVolume := &structuredVolume{}
			structuredVolume.parsePV(tc.inputVolume)
			// Check the results
			if structuredVolume.capacity != *tc.inputVolume.Spec.Capacity.Storage() {
				t.Errorf("capacity does not match expected value")
			}
			if structuredVolume.storageClass != tc.inputVolume.Spec.StorageClassName {
				t.Errorf("Storage class does not match expected value")
			}
			if tc.expectedNFS != nil {
				if structuredVolume.nfs == nil {
					t.Errorf("Expected a non-nil NFS volume source")
				} else if *tc.expectedNFS != *structuredVolume.nfs {
					t.Errorf("NFS volume source does not match expected value")
				}
			}
			if tc.expectedCSI != nil {
				if structuredVolume.csi == nil {
					t.Errorf("Expected a non-nil CSI volume source")
				} else if *tc.expectedCSI != *structuredVolume.csi {
					t.Errorf("CSI volume source does not match expected value")
				}
			}
		})
	}
}
