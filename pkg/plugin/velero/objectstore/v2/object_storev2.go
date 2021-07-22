/*
Copyright 2021 the Velero contributors.

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

package v2

import (
	v1 "github.com/vmware-tanzu/velero/pkg/plugin/velero/objectstore/v1"

	"context"
	"io"
	"time"
)

type ObjectStore interface {
	v1.ObjectStore

	// InitV2 prepares the ObjectStore for usage using the provided map of
	// configuration key-value pairs. It returns an error if the ObjectStore
	// cannot be initialized from the provided config.
	InitV2(ctx context.Context, config map[string]string) error

	// PutObjectV2 creates a new object using the data in body within the specified
	// object storage bucket with the given key.
	PutObjectV2(ctx context.Context, bucket, key string, body io.Reader) error

	// ObjectExistsV2 checks if there is an object with the given key in the object storage bucket.
	ObjectExistsV2(ctx context.Context, bucket, key string) (bool, error)

	// GetObjectV2 retrieves the object with the given key from the specified
	// bucket in object storage.
	GetObjectV2(ctx context.Context, bucket, key string) (io.ReadCloser, error)

	// ListCommonPrefixesV2 gets a list of all object key prefixes that start with
	// the specified prefix and stop at the next instance of the provided delimiter.
	//
	// For example, if the bucket contains the following keys:
	//		a-prefix/foo-1/bar
	// 		a-prefix/foo-1/baz
	//		a-prefix/foo-2/baz
	// 		some-other-prefix/foo-3/bar
	// and the provided prefix arg is "a-prefix/", and the delimiter is "/",
	// this will return the slice {"a-prefix/foo-1/", "a-prefix/foo-2/"}.
	ListCommonPrefixesV2(ctx context.Context, bucket, prefix, delimiter string) ([]string, error)

	// ListObjectsV2 gets a list of all keys in the specified bucket
	// that have the given prefix.
	ListObjectsV2(ctx context.Context, bucket, prefix string) ([]string, error)

	// DeleteObjectV2 removes the object with the specified key from the given
	// bucket.
	DeleteObjectV2(ctx context.Context, bucket, key string) error

	// CreateSignedURLV2 creates a pre-signed URL for the given bucket and key that expires after ttl.
	CreateSignedURLV2(ctx context.Context, bucket, key string, ttl time.Duration) (string, error)
}
