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

package csi

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	jsonpatch "github.com/evanphx/json-patch"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"

	"github.com/vmware-tanzu/velero/pkg/util/boolptr"
	"github.com/vmware-tanzu/velero/pkg/util/stringptr"

	snapshotv1api "github.com/kubernetes-csi/external-snapshotter/client/v4/apis/volumesnapshot/v1"
	snapshotter "github.com/kubernetes-csi/external-snapshotter/client/v4/clientset/versioned/typed/volumesnapshot/v1"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
)

const (
	waitInternal = 2 * time.Second
)

// WaitVolumeSnapshotReady waits a VS to become ready to use until the timeout reaches
func WaitVolumeSnapshotReady(ctx context.Context, snapshotClient snapshotter.SnapshotV1Interface,
	volumeSnapshot string, volumeSnapshotNS string, timeout time.Duration) (*snapshotv1api.VolumeSnapshot, error) {
	var updated *snapshotv1api.VolumeSnapshot

	err := wait.PollImmediate(waitInternal, timeout, func() (bool, error) {
		tmpVS, err := snapshotClient.VolumeSnapshots(volumeSnapshotNS).Get(ctx, volumeSnapshot, metav1.GetOptions{})
		if err != nil {
			return false, errors.Wrapf(err, fmt.Sprintf("error to get volumesnapshot %s/%s", volumeSnapshotNS, volumeSnapshot))
		}

		if tmpVS.Status == nil {
			return false, nil
		}

		if tmpVS.Status.Error != nil {
			return false, errors.Errorf("volume snapshot creation error %s", stringptr.GetString(tmpVS.Status.Error.Message))
		}

		if !boolptr.IsSetToTrue(tmpVS.Status.ReadyToUse) {
			return false, nil
		}

		updated = tmpVS
		return true, nil
	})

	return updated, err
}

// GetVolumeSnapshotContentForVolumeSnapshot returns the volumesnapshotcontent object associated with the volumesnapshot
func GetVolumeSnapshotContentForVolumeSnapshot(volSnap *snapshotv1api.VolumeSnapshot, snapshotClient snapshotter.SnapshotV1Interface) (*snapshotv1api.VolumeSnapshotContent, error) {
	if volSnap.Status == nil || volSnap.Status.BoundVolumeSnapshotContentName == nil {
		return nil, errors.Errorf("invalid snapshot info in volume snapshot %s", volSnap.Name)
	}

	vsc, err := snapshotClient.VolumeSnapshotContents().Get(context.TODO(), *volSnap.Status.BoundVolumeSnapshotContentName, metav1.GetOptions{})
	if err != nil {
		return nil, errors.Wrap(err, "error getting volume snapshot content from API")
	}

	return vsc, nil
}

// RetainVSC updates the VSC's deletion policy to Retain and return the update VSC
func RetainVSC(ctx context.Context, snapshotClient snapshotter.SnapshotV1Interface,
	vsc *snapshotv1api.VolumeSnapshotContent) (*snapshotv1api.VolumeSnapshotContent, error) {
	if vsc.Spec.DeletionPolicy == snapshotv1api.VolumeSnapshotContentRetain {
		return nil, nil
	}
	origBytes, err := json.Marshal(vsc)
	if err != nil {
		return nil, errors.Wrap(err, "error marshaling original VSC")
	}

	updated := vsc.DeepCopy()
	updated.Spec.DeletionPolicy = snapshotv1api.VolumeSnapshotContentRetain

	updatedBytes, err := json.Marshal(updated)
	if err != nil {
		return nil, errors.Wrap(err, "error marshaling updated VSC")
	}

	patchBytes, err := jsonpatch.CreateMergePatch(origBytes, updatedBytes)
	if err != nil {
		return nil, errors.Wrap(err, "error creating json merge patch for VSC")
	}

	retained, err := snapshotClient.VolumeSnapshotContents().Patch(ctx, vsc.Name, types.MergePatchType, patchBytes, metav1.PatchOptions{})
	if err != nil {
		return nil, errors.Wrap(err, "error patching VSC")
	}

	return retained, nil
}

// DeleteVolumeSnapshotContentIfAny deletes a VSC by name if it exists, and log an error when the deletion fails
func DeleteVolumeSnapshotContentIfAny(ctx context.Context, snapshotClient snapshotter.SnapshotV1Interface, vscName string, log logrus.FieldLogger) {
	err := snapshotClient.VolumeSnapshotContents().Delete(ctx, vscName, metav1.DeleteOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			log.WithError(err).Debugf("Abort deleting VSC, it doesn't exist %s", vscName)
		} else {
			log.WithError(err).Errorf("Failed to delete volume snapshot content %s", vscName)
		}
	}
}

// EnsureDeleteVS asserts the existence of a VS by name, deletes it and waits for its disappearance and returns errors on any failure
func EnsureDeleteVS(ctx context.Context, snapshotClient snapshotter.SnapshotV1Interface,
	vsName string, vsNamespace string, timeout time.Duration) error {
	err := snapshotClient.VolumeSnapshots(vsNamespace).Delete(ctx, vsName, metav1.DeleteOptions{})
	if err != nil {
		return errors.Wrap(err, "error to delete volume snapshot")
	}

	err = wait.PollImmediate(waitInternal, timeout, func() (bool, error) {
		_, err := snapshotClient.VolumeSnapshots(vsNamespace).Get(ctx, vsName, metav1.GetOptions{})
		if err != nil {
			if apierrors.IsNotFound(err) {
				return true, nil
			}

			return false, errors.Wrapf(err, fmt.Sprintf("error to get VolumeSnapshot %s", vsName))
		}

		return false, nil
	})

	if err != nil {
		return errors.Wrapf(err, "error to assure VolumeSnapshot is deleted, %s", vsName)
	}

	return nil
}

// EnsureDeleteVSC asserts the existence of a VSC by name, deletes it and waits for its disappearance and returns errors on any failure
func EnsureDeleteVSC(ctx context.Context, snapshotClient snapshotter.SnapshotV1Interface,
	vscName string, timeout time.Duration) error {
	err := snapshotClient.VolumeSnapshotContents().Delete(ctx, vscName, metav1.DeleteOptions{})
	if err != nil {
		return errors.Wrap(err, "error to delete volume snapshot content")
	}

	err = wait.PollImmediate(waitInternal, timeout, func() (bool, error) {
		_, err := snapshotClient.VolumeSnapshotContents().Get(ctx, vscName, metav1.GetOptions{})
		if err != nil {
			if apierrors.IsNotFound(err) {
				return true, nil
			}

			return false, errors.Wrapf(err, fmt.Sprintf("error to get VolumeSnapshotContent %s", vscName))
		}

		return false, nil
	})

	if err != nil {
		return errors.Wrapf(err, "error to assure VolumeSnapshotContent is deleted, %s", vscName)
	}

	return nil
}

// DeleteVolumeSnapshotIfAny deletes a VS by name if it exists, and log an error when the deletion fails
func DeleteVolumeSnapshotIfAny(ctx context.Context, snapshotClient snapshotter.SnapshotV1Interface, vsName string, vsNamespace string, log logrus.FieldLogger) {
	err := snapshotClient.VolumeSnapshots(vsNamespace).Delete(ctx, vsName, metav1.DeleteOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			log.WithError(err).Debugf("Abort deleting volume snapshot, it doesn't exist %s/%s", vsNamespace, vsName)
		} else {
			log.WithError(err).Errorf("Failed to delete volume snapshot %s/%s", vsNamespace, vsName)
		}
	}
}
