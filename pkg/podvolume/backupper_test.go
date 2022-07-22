/*
Copyright 2018 the Velero contributors.

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

package podvolume

import (
	"context"
	"testing"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	corev1api "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestIsHostPathVolume(t *testing.T) {
	// hostPath pod volume
	vol := &corev1api.Volume{
		VolumeSource: corev1api.VolumeSource{
			HostPath: &corev1api.HostPathVolumeSource{},
		},
	}
	isHostPath, err := isHostPathVolume(vol, nil, nil)
	assert.Nil(t, err)
	assert.True(t, isHostPath)

	// non-hostPath pod volume
	vol = &corev1api.Volume{
		VolumeSource: corev1api.VolumeSource{
			EmptyDir: &corev1api.EmptyDirVolumeSource{},
		},
	}
	isHostPath, err = isHostPathVolume(vol, nil, nil)
	assert.Nil(t, err)
	assert.False(t, isHostPath)

	// PVC that doesn't have a PV
	vol = &corev1api.Volume{
		VolumeSource: corev1api.VolumeSource{
			PersistentVolumeClaim: &corev1api.PersistentVolumeClaimVolumeSource{
				ClaimName: "pvc-1",
			},
		},
	}
	pvc := &corev1api.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "ns-1",
			Name:      "pvc-1",
		},
	}
	isHostPath, err = isHostPathVolume(vol, pvc, nil)
	assert.Nil(t, err)
	assert.False(t, isHostPath)

	// PVC that claims a non-hostPath PV
	vol = &corev1api.Volume{
		VolumeSource: corev1api.VolumeSource{
			PersistentVolumeClaim: &corev1api.PersistentVolumeClaimVolumeSource{
				ClaimName: "pvc-1",
			},
		},
	}
	pvc = &corev1api.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "ns-1",
			Name:      "pvc-1",
		},
		Spec: corev1api.PersistentVolumeClaimSpec{
			VolumeName: "pv-1",
		},
	}
	pvGetter := &fakePVGetter{
		pv: &corev1api.PersistentVolume{
			ObjectMeta: metav1.ObjectMeta{
				Name: "pv-1",
			},
			Spec: corev1api.PersistentVolumeSpec{},
		},
	}
	isHostPath, err = isHostPathVolume(vol, pvc, pvGetter)
	assert.Nil(t, err)
	assert.False(t, isHostPath)

	// PVC that claims a hostPath PV
	vol = &corev1api.Volume{
		VolumeSource: corev1api.VolumeSource{
			PersistentVolumeClaim: &corev1api.PersistentVolumeClaimVolumeSource{
				ClaimName: "pvc-1",
			},
		},
	}
	pvc = &corev1api.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "ns-1",
			Name:      "pvc-1",
		},
		Spec: corev1api.PersistentVolumeClaimSpec{
			VolumeName: "pv-1",
		},
	}
	pvGetter = &fakePVGetter{
		pv: &corev1api.PersistentVolume{
			ObjectMeta: metav1.ObjectMeta{
				Name: "pv-1",
			},
			Spec: corev1api.PersistentVolumeSpec{
				PersistentVolumeSource: corev1api.PersistentVolumeSource{
					HostPath: &corev1api.HostPathVolumeSource{},
				},
			},
		},
	}
	isHostPath, err = isHostPathVolume(vol, pvc, pvGetter)
	assert.Nil(t, err)
	assert.True(t, isHostPath)
}

type fakePVGetter struct {
	pv *corev1api.PersistentVolume
}

func (g *fakePVGetter) Get(ctx context.Context, name string, opts metav1.GetOptions) (*corev1api.PersistentVolume, error) {
	if g.pv != nil {
		return g.pv, nil
	}

	return nil, errors.New("item not found")
}
