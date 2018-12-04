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
	"fmt"
	"io"
	"time"

	"github.com/pkg/errors"
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
	PutObject(bucket, key string, body io.Reader) error

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

// NotFoundError indicates an object could not be found in object storage.
type NotFoundError struct {
	bucket string
	key    string
}

// NewNotFoundError creates a new NotFoundError.
func NewNotFoundError(bucket, key string) error {
	return &NotFoundError{
		bucket: bucket,
		key:    key,
	}
}

func (e *NotFoundError) Error() string {
	return fmt.Sprintf("%s/%s not found", e.bucket, e.key)
}

// Bucket returns the bucket.
func (e *NotFoundError) Bucket() string {
	return e.bucket
}

// Key returns the key.
func (e *NotFoundError) Key() string {
	return e.key
}

// ToNotFoundError returns a *NotFoundError if err or its cause is a *NotFoundError. Otherwise, ToNotFoundError returns
// nil.
func ToNotFoundError(err error) *NotFoundError {
	if e, ok := errors.Cause(err).(*NotFoundError); ok {
		return e
	}

	return nil
}
