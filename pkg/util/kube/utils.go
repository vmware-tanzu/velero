/*
Copyright the Velero contributors.

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

package kube

import (
	"context"
	"fmt"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	corev1api "k8s.io/api/core/v1"
	storagev1api "k8s.io/api/storage/v1"
	apiextv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apiextv1beta1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/vmware-tanzu/velero/pkg/uploader"
	"github.com/vmware-tanzu/velero/pkg/util/filesystem"
)

// These annotations are taken from the Kubernetes persistent volume/persistent volume claim controller.
// They cannot be directly importing because they are part of the kubernetes/kubernetes package, and importing that package is unsupported.
// Their values are well-known and slow changing. They're duplicated here as constants to provide compile-time checking.
// Originals can be found in kubernetes/kubernetes/pkg/controller/volume/persistentvolume/util/util.go.
const (
	KubeAnnBindCompleted          = "pv.kubernetes.io/bind-completed"
	KubeAnnBoundByController      = "pv.kubernetes.io/bound-by-controller"
	KubeAnnDynamicallyProvisioned = "pv.kubernetes.io/provisioned-by"
	KubeAnnMigratedTo             = "pv.kubernetes.io/migrated-to"
	KubeAnnSelectedNode           = "volume.kubernetes.io/selected-node"
)

var ErrorPodVolumeIsNotPVC = errors.New("pod volume is not a PVC")

// NamespaceAndName returns a string in the format <namespace>/<name>
func NamespaceAndName(objMeta metav1.Object) string {
	if objMeta.GetNamespace() == "" {
		return objMeta.GetName()
	}
	return fmt.Sprintf("%s/%s", objMeta.GetNamespace(), objMeta.GetName())
}

// GetVolumeDirectory gets the name of the directory on the host, under /var/lib/kubelet/pods/<podUID>/volumes/,
// where the specified volume lives.
// For volumes with a CSIVolumeSource, append "/mount" to the directory name.
func GetVolumeDirectory(ctx context.Context, log logrus.FieldLogger, pod *corev1api.Pod, volumeName string, cli client.Client) (string, error) {
	pvc, pv, volume, err := GetPodPVCVolume(ctx, log, pod, volumeName, cli)
	if err != nil {
		// This case implies the administrator created the PV and attached it directly, without PVC.
		// Note that only one VolumeSource can be populated per Volume on a pod
		if err == ErrorPodVolumeIsNotPVC {
			if volume.VolumeSource.CSI != nil {
				return volume.Name + "/mount", nil
			}
			return volume.Name, nil
		}
		return "", errors.WithStack(err)
	}

	// Most common case is that we have a PVC VolumeSource, and we need to check the PV it points to for a CSI source.
	// PV's been created with a CSI source.
	isProvisionedByCSI, err := isProvisionedByCSI(log, pv, cli)
	if err != nil {
		return "", errors.WithStack(err)
	}
	if isProvisionedByCSI {
		if pv.Spec.VolumeMode != nil && *pv.Spec.VolumeMode == corev1api.PersistentVolumeBlock {
			return pvc.Spec.VolumeName, nil
		}
		return pvc.Spec.VolumeName + "/mount", nil
	}

	return pvc.Spec.VolumeName, nil
}

// GetVolumeMode gets the uploader.PersistentVolumeMode of the volume.
func GetVolumeMode(ctx context.Context, log logrus.FieldLogger, pod *corev1api.Pod, volumeName string, cli client.Client) (
	uploader.PersistentVolumeMode, error) {
	_, pv, _, err := GetPodPVCVolume(ctx, log, pod, volumeName, cli)

	if err != nil {
		if err == ErrorPodVolumeIsNotPVC {
			return uploader.PersistentVolumeFilesystem, nil
		}
		return "", errors.WithStack(err)
	}

	if pv.Spec.VolumeMode != nil && *pv.Spec.VolumeMode == corev1api.PersistentVolumeBlock {
		return uploader.PersistentVolumeBlock, nil
	}
	return uploader.PersistentVolumeFilesystem, nil
}

// GetPodPVCVolume gets the PVC, PV and volume for a pod volume name.
// Returns pod volume in case of ErrorPodVolumeIsNotPVC error
func GetPodPVCVolume(ctx context.Context, log logrus.FieldLogger, pod *corev1api.Pod, volumeName string, cli client.Client) (
	*corev1api.PersistentVolumeClaim, *corev1api.PersistentVolume, *corev1api.Volume, error) {
	var volume *corev1api.Volume

	for i := range pod.Spec.Volumes {
		if pod.Spec.Volumes[i].Name == volumeName {
			volume = &pod.Spec.Volumes[i]
			break
		}
	}

	if volume == nil {
		return nil, nil, nil, errors.New("volume not found in pod")
	}

	if volume.VolumeSource.PersistentVolumeClaim == nil {
		return nil, nil, volume, ErrorPodVolumeIsNotPVC // There is a pod volume but it is not a PVC
	}

	pvc := &corev1api.PersistentVolumeClaim{}
	err := cli.Get(ctx, client.ObjectKey{Namespace: pod.Namespace, Name: volume.VolumeSource.PersistentVolumeClaim.ClaimName}, pvc)
	if err != nil {
		return nil, nil, nil, errors.WithStack(err)
	}

	pv := &corev1api.PersistentVolume{}
	err = cli.Get(ctx, client.ObjectKey{Name: pvc.Spec.VolumeName}, pv)
	if err != nil {
		return nil, nil, nil, errors.WithStack(err)
	}

	return pvc, pv, volume, nil
}

// isProvisionedByCSI function checks whether this is a CSI PV by annotation.
// Either "pv.kubernetes.io/provisioned-by" or "pv.kubernetes.io/migrated-to" indicates
// PV is provisioned by CSI.
func isProvisionedByCSI(log logrus.FieldLogger, pv *corev1api.PersistentVolume, kbClient client.Client) (bool, error) {
	if pv.Spec.CSI != nil {
		return true, nil
	}
	// Although the pv.Spec.CSI is nil, the volume could be provisioned by a CSI driver when enabling the CSI migration
	// Refer to https://github.com/vmware-tanzu/velero/issues/4496 for more details
	if pv.Annotations != nil {
		driverName := pv.Annotations[KubeAnnDynamicallyProvisioned]
		migratedDriver := pv.Annotations[KubeAnnMigratedTo]
		if len(driverName) > 0 || len(migratedDriver) > 0 {
			list := &storagev1api.CSIDriverList{}
			if err := kbClient.List(context.TODO(), list); err != nil {
				return false, err
			}
			for _, driver := range list.Items {
				if driverName == driver.Name || migratedDriver == driver.Name {
					log.Debugf("the annotation %s or %s equals to %s indicates the volume is provisioned by a CSI driver", KubeAnnDynamicallyProvisioned, KubeAnnMigratedTo, driver.Name)
					return true, nil
				}
			}
		}
	}
	return false, nil
}

// SinglePathMatch checks whether pass-in volume path is valid.
// Check whether there is only one match by the path's pattern.
func SinglePathMatch(path string, fs filesystem.Interface, log logrus.FieldLogger) (string, error) {
	matches, err := fs.Glob(path)
	if err != nil {
		return "", errors.WithStack(err)
	}
	if len(matches) != 1 {
		return "", errors.Errorf("expected one matching path: %s, got %d", path, len(matches))
	}

	log.Debugf("This is a valid volume path: %s.", matches[0])
	return matches[0], nil
}

// IsV1CRDReady checks a v1 CRD to see if it's ready, with both the Established and NamesAccepted conditions.
func IsV1CRDReady(crd *apiextv1.CustomResourceDefinition) bool {
	var isEstablished, namesAccepted bool
	for _, cond := range crd.Status.Conditions {
		if cond.Type == apiextv1.Established && cond.Status == apiextv1.ConditionTrue {
			isEstablished = true
		}
		if cond.Type == apiextv1.NamesAccepted && cond.Status == apiextv1.ConditionTrue {
			namesAccepted = true
		}
	}

	return (isEstablished && namesAccepted)
}

// IsV1Beta1CRDReady checks a v1beta1 CRD to see if it's ready, with both the Established and NamesAccepted conditions.
func IsV1Beta1CRDReady(crd *apiextv1beta1.CustomResourceDefinition) bool {
	var isEstablished, namesAccepted bool
	for _, cond := range crd.Status.Conditions {
		if cond.Type == apiextv1beta1.Established && cond.Status == apiextv1beta1.ConditionTrue {
			isEstablished = true
		}
		if cond.Type == apiextv1beta1.NamesAccepted && cond.Status == apiextv1beta1.ConditionTrue {
			namesAccepted = true
		}
	}

	return (isEstablished && namesAccepted)
}

// IsCRDReady triggers IsV1Beta1CRDReady/IsV1CRDReady according to the version of the input param
func IsCRDReady(crd *unstructured.Unstructured) (bool, error) {
	ver := crd.GroupVersionKind().Version
	switch ver {
	case "v1beta1":
		v1beta1crd := &apiextv1beta1.CustomResourceDefinition{}
		err := runtime.DefaultUnstructuredConverter.FromUnstructured(crd.Object, v1beta1crd)
		if err != nil {
			return false, err
		}
		return IsV1Beta1CRDReady(v1beta1crd), nil
	case "v1":
		v1crd := &apiextv1.CustomResourceDefinition{}
		err := runtime.DefaultUnstructuredConverter.FromUnstructured(crd.Object, v1crd)
		if err != nil {
			return false, err
		}
		return IsV1CRDReady(v1crd), nil
	default:
		return false, fmt.Errorf("unable to handle CRD with version %s", ver)
	}
}
