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

package kube

import (
	"context"
	"fmt"
	"time"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	corev1api "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/util/wait"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	corev1client "k8s.io/client-go/kubernetes/typed/core/v1"
)

const (
	waitInternal = 2 * time.Second
)

// DeletePVCIfAny deletes a PVC by name if it exists, and log an error when the deletion fails
func DeletePVCIfAny(ctx context.Context, pvcGetter corev1client.PersistentVolumeClaimsGetter, pvcName string, pvcNamespace string, log logrus.FieldLogger) {
	err := pvcGetter.PersistentVolumeClaims(pvcNamespace).Delete(ctx, pvcName, metav1.DeleteOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			log.WithError(err).Debugf("Abort deleting PVC, it doesn't exist, %s/%s", pvcNamespace, pvcName)
		} else {
			log.WithError(err).Errorf("Failed to delete pvc %s/%s", pvcNamespace, pvcName)
		}
	}
}

// WaitPVCBound wait for binding of a PVC specified by name and returns the bound PV object
func WaitPVCBound(ctx context.Context, pvcGetter corev1client.PersistentVolumeClaimsGetter,
	pvGetter corev1client.PersistentVolumesGetter, pvc string, namespace string, timeout time.Duration) (*corev1api.PersistentVolume, error) {

	var updated *corev1api.PersistentVolumeClaim
	err := wait.PollImmediate(waitInternal, timeout, func() (bool, error) {
		tmpPVC, err := pvcGetter.PersistentVolumeClaims(namespace).Get(ctx, pvc, metav1.GetOptions{})
		if err != nil {
			return false, errors.Wrapf(err, fmt.Sprintf("failed to get pvc %s/%s", namespace, pvc))
		}

		if tmpPVC.Spec.VolumeName == "" {
			return false, nil
		}

		updated = tmpPVC

		return true, nil
	})

	if err != nil {
		return nil, errors.Wrap(err, "error to wait for rediness of PVC")
	}

	pv, err := pvGetter.PersistentVolumes().Get(ctx, updated.Spec.VolumeName, metav1.GetOptions{})
	if err != nil {
		return nil, errors.Wrap(err, "error to get PV")
	}

	return pv, err
}
