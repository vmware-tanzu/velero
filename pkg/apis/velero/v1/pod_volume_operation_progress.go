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

package v1

import (
	"context"

	"github.com/sirupsen/logrus"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// PodVolumeOperationProgress represents the progress of a
// PodVolumeBackup/Restore (restic) operation
type PodVolumeOperationProgress struct {
	// +optional
	TotalBytes int64 `json:"totalBytes,omitempty"`

	// +optional
	BytesDone int64 `json:"bytesDone,omitempty"`
}

type BackupProgressUpdater struct {
	pvb *PodVolumeBackup
	log logrus.FieldLogger
	ctx context.Context
	cli client.Client
}

type RestoreProgressUpdater struct {
	pvr *PodVolumeRestore
	log logrus.FieldLogger
	ctx context.Context
	cli client.Client
}

func NewBackupProgressUpdater(pvb *PodVolumeBackup, log logrus.FieldLogger, ctx context.Context, cli client.Client) *BackupProgressUpdater {
	return &BackupProgressUpdater{pvb, log, ctx, cli}
}

//UpdateProgress which implement ProgressUpdater to update pvb progress status
func (b *BackupProgressUpdater) UpdateProgress(p *UploaderProgress) {
	original := b.pvb.DeepCopy()
	b.pvb.Status.Progress = PodVolumeOperationProgress{TotalBytes: p.TotalBytes, BytesDone: p.BytesDone}
	if b.cli == nil {
		b.log.Errorf("failed to update backup pod %s volume %s progress with uninitailize client", b.pvb.Spec.Pod.Name, b.pvb.Spec.Volume)
		return
	}
	if err := b.cli.Patch(b.ctx, b.pvb, client.MergeFrom(original)); err != nil {
		b.log.Errorf("update backup pod %s volume %s progress with %v", b.pvb.Spec.Pod.Name, b.pvb.Spec.Volume, err)
	}
}

func NewRestoreProgressUpdater(pvr *PodVolumeRestore, log logrus.FieldLogger, ctx context.Context, cli client.Client) *RestoreProgressUpdater {
	return &RestoreProgressUpdater{pvr, log, ctx, cli}
}

//UpdateProgress which implement ProgressUpdater to update update pvb progress status
func (r *RestoreProgressUpdater) UpdateProgress(p *UploaderProgress) {
	original := r.pvr.DeepCopy()
	r.pvr.Status.Progress = PodVolumeOperationProgress{TotalBytes: p.TotalBytes, BytesDone: p.BytesDone}
	if r.cli == nil {
		r.log.Errorf("failed to update restore pod %s volume %s progress with uninitailize client", r.pvr.Spec.Pod.Name, r.pvr.Spec.Volume)
		return
	}
	if err := r.cli.Patch(r.ctx, r.pvr, client.MergeFrom(original)); err != nil {
		r.log.Errorf("update restore pod %s volume %s progress with %v", r.pvr.Spec.Pod.Name, r.pvr.Spec.Volume, err)
	}
}
