/*
Copyright 2017 the Heptio Ark contributors.

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

	"k8s.io/apimachinery/pkg/runtime"
)

// ObjectStore exposes basic object-storage operations required
// by Ark.
type ObjectStore interface {
	// Init prepares the ObjectStore for usage using the provided map of
	// configuration key-value pairs. It returns an error if the ObjectStore
	// cannot be initialized from the provided config.
	Init(config map[string]string) error

	// PutObject creates a new object using the data in body within the specified
	// object storage bucket with the given key.
	PutObject(bucket string, key string, body io.Reader) error

	// GetObject retrieves the object with the given key from the specified
	// bucket in object storage.
	GetObject(bucket string, key string) (io.ReadCloser, error)

	// ListCommonPrefixes gets a list of all object key prefixes that come
	// before the provided delimiter. For example, if the bucket contains
	// the keys "foo-1/bar", "foo-1/baz", and "foo-2/baz", and the delimiter
	// is "/", this will return the slice {"foo-1", "foo-2"}.
	ListCommonPrefixes(bucket string, delimiter string) ([]string, error)

	// ListObjects gets a list of all keys in the specified bucket
	// that have the given prefix.
	ListObjects(bucket, prefix string) ([]string, error)

	// DeleteObject removes the object with the specified key from the given
	// bucket.
	DeleteObject(bucket string, key string) error

	// CreateSignedURL creates a pre-signed URL for the given bucket and key that expires after ttl.
	CreateSignedURL(bucket, key string, ttl time.Duration) (string, error)
}

// BlockStore exposes basic block-storage operations required
// by Ark.
type BlockStore interface {
	// Init prepares the BlockStore for usage using the provided map of
	// configuration key-value pairs. It returns an error if the BlockStore
	// cannot be initialized from the provided config.
	Init(config map[string]string) error

	// CreateVolumeFromSnapshot creates a new block volume in the specified
	// availability zone, initialized from the provided snapshot,
	// and with the specified type and IOPS (if using provisioned IOPS).
	CreateVolumeFromSnapshot(snapshotID, volumeType, volumeAZ string, iops *int64) (volumeID string, err error)

	// GetVolumeID returns the cloud provider specific identifier for the PersistentVolume.
	GetVolumeID(pv runtime.Unstructured) (string, error)

	// SetVolumeID sets the cloud provider specific identifier for the PersistentVolume.
	SetVolumeID(pv runtime.Unstructured, volumeID string) (runtime.Unstructured, error)

	// GetVolumeInfo returns the type and IOPS (if using provisioned IOPS) for
	// the specified block volume in the given availability zone.
	GetVolumeInfo(volumeID, volumeAZ string) (string, *int64, error)

	// IsVolumeReady returns whether the specified volume is ready to be used.
	IsVolumeReady(volumeID, volumeAZ string) (ready bool, err error)

	// CreateSnapshot creates a snapshot of the specified block volume, and applies the provided
	// set of tags to the snapshot.
	CreateSnapshot(volumeID, volumeAZ string, tags map[string]string) (snapshotID string, err error)

	// DeleteSnapshot deletes the specified volume snapshot.
	DeleteSnapshot(snapshotID string) error
}
