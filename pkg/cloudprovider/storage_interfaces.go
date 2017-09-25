/*
Copyright 2017 Heptio Inc.

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

package cloudprovider

import (
	"io"
	"time"
)

// ObjectStorageAdapter exposes basic object-storage operations required
// by Ark.
type ObjectStorageAdapter interface {
	// PutObject creates a new object using the data in body within the specified
	// object storage bucket with the given key.
	PutObject(bucket string, key string, body io.ReadSeeker) error

	// GetObject retrieves the object with the given key from the specified
	// bucket in object storage.
	GetObject(bucket string, key string) (io.ReadCloser, error)

	// ListCommonPrefixes gets a list of all object key prefixes that come
	// before the provided delimiter (this is often used to simulate a directory
	// hierarchy in object storage).
	ListCommonPrefixes(bucket string, delimiter string) ([]string, error)

	// ListObjects gets a list of all objects in bucket that have the same prefix.
	ListObjects(bucket, prefix string) ([]string, error)

	// DeleteObject removes object with the specified key from the given
	// bucket.
	DeleteObject(bucket string, key string) error

	// CreateSignedURL creates a pre-signed URL for the given bucket and key that expires after ttl.
	CreateSignedURL(bucket, key string, ttl time.Duration) (string, error)
}

// BlockStorageAdapter exposes basic block-storage operations required
// by Ark.
type BlockStorageAdapter interface {
	// CreateVolumeFromSnapshot creates a new block volume, initialized from the provided snapshot,
	// and with the specified type and IOPS (if using provisioned IOPS).
	CreateVolumeFromSnapshot(snapshotID, volumeType, volumeAZ string, iops *int64) (volumeID string, err error)

	// GetVolumeInfo returns the type and IOPS (if using provisioned IOPS) for a specified block
	// volume.
	GetVolumeInfo(volumeID, volumeAZ string) (string, *int64, error)

	// IsVolumeReady returns whether the specified volume is ready to be used.
	IsVolumeReady(volumeID, volumeAZ string) (ready bool, err error)

	// ListSnapshots returns a list of all snapshots matching the specified set of tag key/values.
	ListSnapshots(tagFilters map[string]string) ([]string, error)

	// CreateSnapshot creates a snapshot of the specified block volume, and applies the provided
	// set of tags to the snapshot.
	CreateSnapshot(volumeID, volumeAZ string, tags map[string]string) (snapshotID string, err error)

	// DeleteSnapshot deletes the specified volume snapshot.
	DeleteSnapshot(snapshotID string) error
}
