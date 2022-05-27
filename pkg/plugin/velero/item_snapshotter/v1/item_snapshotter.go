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

package v1

import (
	"context"
	"fmt"
	"time"

	"k8s.io/apimachinery/pkg/runtime"

	api "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	"github.com/vmware-tanzu/velero/pkg/plugin/velero"
)

type AlsoHandlesInput struct {
	// Item is the item that will be snapshotted
	Item runtime.Unstructured
	// Backup is the representation of the backup resource being processed by Velero.
	Backup *api.Backup
}

type SnapshotItemInput struct {
	// Item is the item to snapshot
	Item runtime.Unstructured
	// Params are parameters to the snapshot
	Params map[string]string
	// Backup is the representation of the backup resource being processed by Velero.
	Backup *api.Backup
}

type SnapshotItemOutput struct {
	// UpdatedItem is the Item that should be included in the backup.  It can optionally be modified during the Snapshot
	UpdatedItem runtime.Unstructured
	// SnapshotID identifies the snapshot that was taken
	SnapshotID string
	// SnapshotMetadata is information in addition to the SnapshotID that the
	// plugin wants to store in the backup.  SnapshotMetadata will be passed
	// back in for DeleteSnapshot and CreateItemFromSnapshot
	SnapshotMetadata map[string]string
	// AdditionalItems are resources that need to be included in the backup to support this snapshhot
	AdditionalItems []velero.ResourceIdentifier
	// Items that were handled by this snapshot that should be excluded from the backup
	HandledItems []velero.ResourceIdentifier
}

// ProgressInput contains the input parameters for the ItemSnapshotter's Progress function.
type ProgressInput struct {
	// ItemID is the id of item that was stored in the backup
	ItemID velero.ResourceIdentifier
	// SnapshotID is the snapshot ID returned by ItemSnapshotter
	SnapshotID string
	// Backup is the representation of the backup resource being processed by Velero.
	Backup *api.Backup
}

// SnapshotPhase is the lifecycle phase of a Velero item snapshot.
type SnapshotPhase string

const (
	// SnapshotPhaseInProgress means the snapshot of the item has been taken and the point-in-time has been preserved,
	// but the snapshot is not ready for use yet
	SnapshotPhaseInProgress = SnapshotPhase("InProgress")

	// SnapshotPhaseCompleted means the item snapshot was successfully created and can be restored from
	SnapshotPhaseCompleted = SnapshotPhase("Completed")

	// SnapshotPhaseFailed means the item snapshot was unable to be completed
	SnapshotPhaseFailed = SnapshotPhase("Failed")
)

func SnapshotPhaseFromString(phase string) (SnapshotPhase, error) {
	switch phase {
	case string(SnapshotPhaseInProgress):
		return SnapshotPhaseInProgress, nil
	case string(SnapshotPhaseCompleted):
		return SnapshotPhaseCompleted, nil
	case string(SnapshotPhaseFailed):
		return SnapshotPhaseFailed, nil
	default:
		return SnapshotPhase(""), fmt.Errorf("%s is not a valid SnapshotPhase", phase)
	}
}

type ProgressOutput struct {
	// Phase of the snapshot.  If the phase is SnapshotPhaseFailed, the error will be in the Err string
	Phase SnapshotPhase
	// Err is a message about the error(s) that occurred during the processing of the snapshot
	Err string
	// ItemsCompleted is the number of items that have been completed in processing of the snapshot
	// This is simply to show progress, when Phase goes to SnapshotPhaseCompleted ItemsCompleted and ItemsToComplete
	// should be the same.  This could be blocks to copy, files to copy, or anything else.
	ItemsCompleted int64
	// ItemsToComplete is the number of items that need to be completed
	ItemsToComplete int64
	// Started indicates when processing on the snapshot began (usually the time the snapshot was taken)
	Started time.Time
	// Updated indicates the time the status was last updated.  Time 0 (time.Unix(0, 0)) is returned if unknown.
	Updated time.Time
}

type DeleteSnapshotInput struct {
	// SnapshotID is the snapshot to delete
	SnapshotID string
	// ItemFromBackup is the resource that was included in the backup
	ItemFromBackup runtime.Unstructured
	// SnapshotMetadata is the metadata that was returned when the snapshot was originally taken
	SnapshotMetadata map[string]string
	// Params are parameters to the deletion
	Params map[string]string
}

type CreateItemInput struct {
	// The snapshotted item at this stage of the restore (RestoreItemActions may
	// have modified the item prior to CreateItemFromSnapshot being called)
	SnapshottedItem runtime.Unstructured
	// SnapshotID is the snapshot to create the item from
	SnapshotID string
	// ItemFromBackup is the snapshotted item that was stored in the backup
	ItemFromBackup runtime.Unstructured
	// SnapshotMetadata is the metadata that was returned when the snapshot was originally taken
	SnapshotMetadata map[string]string
	// Params are parameters to the deletion
	Params map[string]string
	// Restore is the representation of the restore resource being processed by Velero.
	Restore *api.Restore
}

type CreateItemOutput struct {
	// UpdatedItem is the item being restored mutated by ItemAction.
	UpdatedItem runtime.Unstructured

	// AdditionalItems is a list of additional related items that should
	// be restored.
	AdditionalItems []velero.ResourceIdentifier

	// SkipRestore tells velero to stop executing further actions
	// on this item, and skip the restore step. When this field's
	// value is true, AdditionalItems will be ignored.
	SkipRestore bool
}

// ItemSnapshotter handles snapshots on an individual item being backed up.
type ItemSnapshotter interface {

	// Init prepares the ItemSnapshotter for usage using the provided map of
	// configuration key-value pairs. It returns an error if the ItemSnapshotter
	// cannot be initialized from the provided config.
	Init(config map[string]string) error

	// AppliesTo returns information about which resources this action should be invoked for.
	// An ItemSnapshotter's SnapshotItem method will only be invoked on items that match the returned
	// selector. A zero-valued ResourceSelector matches all resources.
	AppliesTo() (velero.ResourceSelector, error)

	// AlsoHandles is called for each item this ItemSnapshotter should handle and returns any items
	// which will be handled by this plugin when snapshotting the item.  These items will be excluded from the
	// items being backed up.  AlsoHandles will be called before SnapshotItem is called.  For example, a database may expose
	// a database resource that can be snapshotted. If the database uses a PVC that will be snapshotted/backed up as
	// part of the database snapshot, that PVC should be returned when AlsoHandles is invoked.  This is different from
	// AdditionalItems (returned in SnapshotItemOutput and CreateItemOutput) which are specifying additional resources
	// that Velero should store in the backup or create.
	AlsoHandles(input *AlsoHandlesInput) ([]velero.ResourceIdentifier, error)

	// SnapshotItem causes the ItemSnapshotter to snapshot the specified item.  It may also
	// perform arbitrary logic with the item being backed up, including mutating the item itself prior to backup.
	// The item (unmodified or modified) should be returned, along with an optional slice of ResourceIdentifiers specifying
	// additional related items that should be backed up.
	// A caller can pass a context that includes a timeout.  If the time to take the snapshot exceeds the
	// time in the context, the plugin may abort the snapshot.  The context timeout does not apply to upload
	// time that occurs after SnapshotItem returns
	SnapshotItem(ctx context.Context, input *SnapshotItemInput) (*SnapshotItemOutput, error)

	// Progress will return the progress of a snapshot that is being uploaded
	Progress(input *ProgressInput) (*ProgressOutput, error)

	// DeleteSnapshot removes a snapshot
	DeleteSnapshot(ctx context.Context, input *DeleteSnapshotInput) error

	// CreateItemFromSnapshot creates a new item from the snapshot
	CreateItemFromSnapshot(ctx context.Context, input *CreateItemInput) (*CreateItemOutput, error)
}
