package resourcepolicies

import (
	"testing"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestNewVolumeFilterData(t *testing.T) {
	testCases := []struct {
		name            string
		pv              *corev1.PersistentVolume
		podVol          *corev1.Volume
		pvc             *corev1.PersistentVolumeClaim
		expectedPVName  string
		expectedPodName string
		expectedPVCName string
	}{
		{
			name: "all provided",
			pv: &corev1.PersistentVolume{
				ObjectMeta: metav1.ObjectMeta{
					Name: "pv-test",
				},
			},
			podVol: &corev1.Volume{
				Name: "pod-vol-test",
			},
			pvc: &corev1.PersistentVolumeClaim{
				ObjectMeta: metav1.ObjectMeta{
					Name: "pvc-test",
				},
			},
			expectedPVName:  "pv-test",
			expectedPodName: "pod-vol-test",
			expectedPVCName: "pvc-test",
		},
		{
			name: "only PV provided",
			pv: &corev1.PersistentVolume{
				ObjectMeta: metav1.ObjectMeta{
					Name: "pv-only",
				},
			},
			podVol:          nil,
			pvc:             nil,
			expectedPVName:  "pv-only",
			expectedPodName: "",
			expectedPVCName: "",
		},
		{
			name: "only PodVolume provided",
			pv:   nil,
			podVol: &corev1.Volume{
				Name: "pod-only",
			},
			pvc:             nil,
			expectedPVName:  "",
			expectedPodName: "pod-only",
			expectedPVCName: "",
		},
		{
			name:   "only PVC provided",
			pv:     nil,
			podVol: nil,
			pvc: &corev1.PersistentVolumeClaim{
				ObjectMeta: metav1.ObjectMeta{
					Name: "pvc-only",
				},
			},
			expectedPVName:  "",
			expectedPodName: "",
			expectedPVCName: "pvc-only",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			vfd := NewVolumeFilterData(tc.pv, tc.podVol, tc.pvc)
			if tc.expectedPVName != "" {
				assert.NotNil(t, vfd.PersistentVolume)
				assert.Equal(t, tc.expectedPVName, vfd.PersistentVolume.Name)
			} else {
				assert.Nil(t, vfd.PersistentVolume)
			}
			if tc.expectedPodName != "" {
				assert.NotNil(t, vfd.PodVolume)
				assert.Equal(t, tc.expectedPodName, vfd.PodVolume.Name)
			} else {
				assert.Nil(t, vfd.PodVolume)
			}
			if tc.expectedPVCName != "" {
				assert.NotNil(t, vfd.PVC)
				assert.Equal(t, tc.expectedPVCName, vfd.PVC.Name)
			} else {
				assert.Nil(t, vfd.PVC)
			}
		})
	}
}
