/*
Copyright 2020 the Velero contributors.

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
	"strings"

	"github.com/pkg/errors"

	corev1api "k8s.io/api/core/v1"
	storagev1api "k8s.io/api/storage/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	storagev1 "k8s.io/client-go/kubernetes/typed/storage/v1"
)

/*
evaluates whether the storageclass has enabled shallow-copy restore from snapshot methadology

	a shallow-copy refers to restore from VolumeSnapshot of a network-file system that otherwise does copy on restore
	to setup a reference to the original volume in ROX (ReadOnlyMany)

	provisioners with copy on restore are prone to excessive large snapshot to volume restore times

	this is internal to the CSI driver, there is no current CSI spec to check this feature other than maintaining
	a compatible provisioner list or by plugin interface implementation
*/
type ShallowCopyProvisioner interface {
	// evaluates whether the storageclass matches one of the required provisioner that support shallow-copy
	Evaluate(storageClass *storagev1api.StorageClass) bool
	//transforms the PersistentVolumeClaim into one that activates the csi-driver shallow-copy implementation
	Transform(*corev1api.PersistentVolumeClaim) *corev1api.PersistentVolumeClaim
}

type storageClassProvisioner struct {
	Provisioner string
}

type cephFSProvisioner struct {
	storageClassProvisioner
}

type scaleProvisioner struct {
	storageClassProvisioner
}

func ShallowCopyTransform(ctx context.Context, storageClient storagev1.StorageV1Interface, pvc *corev1api.PersistentVolumeClaim) (*corev1api.PersistentVolumeClaim, error) {
	provisioners := [...]ShallowCopyProvisioner{NewCephFSProvisioner(), NewScaleProvisioner()}

	if pvc.Spec.StorageClassName == nil {
		return pvc, nil
	}

	storageClass, err := GetStorageClass(ctx, storageClient, pvc.Spec.StorageClassName)
	if err != nil {
		return pvc, err
	}

	for _, provisioner := range provisioners {
		isShallowCopyEnabled := provisioner.Evaluate(storageClass)

		if isShallowCopyEnabled {
			return provisioner.Transform(pvc), nil
		}
	}

	return pvc, nil
}

func GetStorageClass(ctx context.Context, storageClient storagev1.StorageV1Interface, storageClassName *string) (*storagev1api.StorageClass, error) {
	// retrieve the StorageClass and compare the provisioner and parameters
	var storageClass *storagev1api.StorageClass
	storageClass, err := storageClient.StorageClasses().Get(ctx, *storageClassName, metav1.GetOptions{})
	if err != nil {
		return nil, errors.Wrapf(err, "unable to retrieve storageclass (storageclass=%s)", storageClass)
	}
	return storageClass, nil
}

/*
provisioners are sometimes repackaged so check if the value is included in the storageclass provisioner
known variants:
- cephfs.csi.ceph.com: Generic CSI Driver https://github.com/ceph/ceph-csi
- openshift-storage.cephfs.csi.ceph.com Red Hat Openshift Data Foundation
- rook-ceph.cephfs.csi.ceph.com Rook.io https://github.com/rook
*/
func NewCephFSProvisioner() cephFSProvisioner {
	return cephFSProvisioner{
		storageClassProvisioner{
			Provisioner: "cephfs.csi.ceph.com",
		},
	}
}

/*
	  	cephfs requirements
	  	storage class parameter backingSnapshot is not false, by default this value is true since v3.8.0,
			introduced in v3.7.0 but the default was false
*/
func (p cephFSProvisioner) Evaluate(storageClass *storagev1api.StorageClass) bool {
	// retrieve the StorageClass and compare the provisioner and parameters

	if strings.Contains(storageClass.Provisioner, p.Provisioner) {
		backingSnapshot, keyExists := storageClass.Parameters["backingSnapshot"]
		// the default in v3.8.0 ceph-csi-cephfs is true
		if !keyExists || strings.ToLower(backingSnapshot) == "true" {
			return true
		}
	}

	return false
}

// cephfs only requires that the accessmode is set to readwriteonly
func (p cephFSProvisioner) Transform(pvc *corev1api.PersistentVolumeClaim) *corev1api.PersistentVolumeClaim {
	transform_pvc := pvc.DeepCopy()

	transform_pvc.Spec.AccessModes = []corev1api.PersistentVolumeAccessMode{corev1api.ReadOnlyMany}

	return transform_pvc
}

/*
provisioners are sometimes repackaged so check if the value is included in the storageclass provisioner
known variants:
- IBM Spectrum Scale Provisioner - spectrumscale.csi.ibm.com https://www.ibm.com/docs/en/scalecsi?topic=configurations-storage-class
*/
func NewScaleProvisioner() scaleProvisioner {
	return scaleProvisioner{
		storageClassProvisioner{
			Provisioner: "spectrumscale.csi.ibm.com",
		},
	}
}

/*
	   	scale requirements
		required with version v5.2.0 and above to trigger shallow provisioner behavior
*/
func (p scaleProvisioner) Evaluate(storageClass *storagev1api.StorageClass) bool {

	return strings.Contains(storageClass.Provisioner, p.Provisioner)
}

// scale only requires that the accessmode is set to readwriteonly
func (p scaleProvisioner) Transform(pvc *corev1api.PersistentVolumeClaim) *corev1api.PersistentVolumeClaim {
	transform_pvc := pvc.DeepCopy()

	transform_pvc.Spec.AccessModes = []corev1api.PersistentVolumeAccessMode{corev1api.ReadOnlyMany}

	return transform_pvc
}
