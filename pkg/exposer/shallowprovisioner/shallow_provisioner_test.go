/*
Copyright The Velero Contributors.

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

package shallowprovisioner

import (
	"context"
	"testing"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	
	corev1api "k8s.io/api/core/v1"
	storagev1api "k8s.io/api/storage/v1"
	clientTesting "k8s.io/client-go/testing"
)

type reactor struct {
	verb        string
	resource    string
	reactorFunc clientTesting.ReactionFunc
}

func TestShallowProvisioner(t *testing.T) {

	cephStorageClass := &storagev1api.StorageClass{
		ObjectMeta: metav1.ObjectMeta{
			Name: "cephfs",
		},
		Provisioner: "cephfs.csi.ceph.com",
		Parameters: map[string]string{},
	}

	cephPVCObj := &corev1api.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "fake-ns",
			Name:      "fake-target-pvc",
		},
		Spec: corev1api.PersistentVolumeClaimSpec{
			StorageClassName: &cephStorageClass.Name,
			AccessModes: []corev1api.PersistentVolumeAccessMode{corev1api.ReadWriteMany},
		},
	}

	scaleStorageClass := &storagev1api.StorageClass{
		ObjectMeta: metav1.ObjectMeta{
			Name: "scale",
		},
		Provisioner: "spectrumscale.csi.ibm.com",
		Parameters: map[string]string{},
	}

	scalePVCObj := &corev1api.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "fake-ns",
			Name:      "fake-target-pvc",
		},
		Spec: corev1api.PersistentVolumeClaimSpec{
			StorageClassName: &scaleStorageClass.Name,
			AccessModes: []corev1api.PersistentVolumeAccessMode{corev1api.ReadWriteOnce, corev1api.ReadWriteMany},
		},
	}
	
	nfsStorageClass := &storagev1api.StorageClass{
		ObjectMeta: metav1.ObjectMeta{
			Name: "nfs",
		},
		Provisioner: "nfs.csi.k8s.io",
		Parameters: map[string]string{},
	}

	nfsPVCObj := &corev1api.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "fake-ns",
			Name:      "fake-target-pvc",
		},
		Spec: corev1api.PersistentVolumeClaimSpec{
			StorageClassName: &nfsStorageClass.Name,
			AccessModes: []corev1api.PersistentVolumeAccessMode{corev1api.ReadWriteOnce},
		},
	}

	tests := []struct {
		name               string
		kubeClientObj      []runtime.Object
		targetPVC          *corev1api.PersistentVolumeClaim
		targetStorageClass *storagev1api.StorageClass
		accessModes        []corev1api.PersistentVolumeAccessMode
		kubeReactors       []reactor
		err                error
	}{
		{
			name:               "test cephfs transform",
			kubeClientObj: []runtime.Object{
				cephStorageClass,
			},
			targetPVC: cephPVCObj,
			targetStorageClass: cephStorageClass,
			accessModes: []corev1api.PersistentVolumeAccessMode{corev1api.ReadOnlyMany,},
			kubeReactors: []reactor{
				{
					verb: "create",
					resource: "storageclasses",
					reactorFunc: func(action clientTesting.Action) (handled bool, ret runtime.Object, err error) {
						return true, nil, errors.New("fake-create-storageclass-error")
					},
				},
			},
			err: nil,
		},
		{
			name:               "test scale transform",
			kubeClientObj: []runtime.Object{
				cephStorageClass,
			},
			targetPVC: scalePVCObj,
			targetStorageClass: scaleStorageClass,
			accessModes: []corev1api.PersistentVolumeAccessMode{corev1api.ReadOnlyMany,},
			kubeReactors: []reactor{
				{
					verb: "create",
					resource: "storageclasses",
					reactorFunc: func(action clientTesting.Action) (handled bool, ret runtime.Object, err error) {
						return true, nil, errors.New("fake-create-storageclass-error")
					},
				},
			},
			err: nil,
		},
		{
			name:               "test nfs transform",
			kubeClientObj: []runtime.Object{
				cephStorageClass,
			},
			targetPVC: nfsPVCObj,
			targetStorageClass: nfsStorageClass,
			accessModes: []corev1api.PersistentVolumeAccessMode{corev1api.ReadWriteOnce,},
			kubeReactors: []reactor{
				{
					verb: "create",
					resource: "storageclasses",
					reactorFunc: func(action clientTesting.Action) (handled bool, ret runtime.Object, err error) {
						return true, nil, errors.New("fake-create-storageclass-error")
					},
				},
			},
			err: nil,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fakeKubeClient := fake.NewSimpleClientset(test.kubeClientObj...)

			for _, reactor := range test.kubeReactors {
				fakeKubeClient.Fake.PrependReactor(reactor.verb, reactor.resource, reactor.reactorFunc)
			}


			pvc, err := ShallowCopyTransform(context.Background(), fakeKubeClient.StorageV1(), test.targetPVC)
			assert.Equal(t, pvc.Spec.AccessModes, []corev1api.PersistentVolumeAccessMode{corev1api.ReadOnlyMany,})
			assert.ErrorIs(t, err, test.err)
		})
	}
}