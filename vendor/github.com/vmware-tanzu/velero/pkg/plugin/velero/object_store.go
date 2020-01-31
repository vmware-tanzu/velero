/*
Copyright 2017 the Velero contributors.

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

package velero

import (
	"io"
	"time"
)

// ObjectStore exposes basic object-storage operations required
// by Velero.
type ObjectStore interface {
	// Init prepares the ObjectStore for usage using the provided map of
	// configuration key-value pairs. It returns an error if the ObjectStore
	// cannot be initialized from the provided config.
	Init(config map[string]string) error

	// PutObject creates a new object using the data in body within the specified
	// object storage bucket with the given key.
	PutObject(bucket, key string, body io.Reader) error

	// ObjectExists checks if there is an object with the given key in the object storage bucket.
	ObjectExists(bucket, key string) (bool, error)

	// GetObject retrieves the object with the given key from the specified
	// bucket in object storage.
	GetObject(bucket, key string) (io.ReadCloser, error)

	// ListCommonPrefixes gets a list of all object key prefixes that start with
	// the specified prefix and stop at the next instance of the provided delimiter.
	//
	// For example, if the bucket contains the following keys:
	//		a-prefix/foo-1/bar
	// 		a-prefix/foo-1/baz
	//		a-prefix/foo-2/baz
	// 		some-other-prefix/foo-3/bar
	// and the provided prefix arg is "a-prefix/", and the delimiter is "/",
	// this will return the slice {"a-prefix/foo-1/", "a-prefix/foo-2/"}.
	ListCommonPrefixes(bucket, prefix, delimiter string) ([]string, error)

	// ListObjects gets a list of all keys in the specified bucket
	// that have the given prefix.
	ListObjects(bucket, prefix string) ([]string, error)

	// DeleteObject removes the object with the specified key from the given
	// bucket.
	DeleteObject(bucket, key string) error

	// CreateSignedURL creates a pre-signed URL for the given bucket and key that expires after ttl.
	CreateSignedURL(bucket, key string, ttl time.Duration) (string, error)
}
