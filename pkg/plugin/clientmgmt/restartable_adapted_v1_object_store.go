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

package clientmgmt

import (
	"context"
	"io"
	"time"

	"github.com/pkg/errors"
	"github.com/vmware-tanzu/velero/pkg/plugin/framework"
	objectstorev1 "github.com/vmware-tanzu/velero/pkg/plugin/velero/objectstore/v1"
	objectstorev2 "github.com/vmware-tanzu/velero/pkg/plugin/velero/objectstore/v2"
)

// restartableAdaptedV1ObjectStore is restartableAdaptedV1ObjectStore version 1 adaptive to version 2 plugin
type restartableAdaptedV1ObjectStore struct {
	restartableObjectStore
}

// newAdaptedV1ObjectStore returns a new restartableAdaptedV1ObjectStore.
func newAdaptedV1ObjectStore(name string, sharedPluginProcess RestartableProcess) objectstorev2.ObjectStore {
	key := kindAndName{kind: framework.PluginKindObjectStore, name: name}
	r := &restartableAdaptedV1ObjectStore{
		restartableObjectStore: restartableObjectStore{
			key:                 key,
			sharedPluginProcess: sharedPluginProcess,
		},
	}

	// Register our reinitializer so we can reinitialize after a restart with r.config.
	sharedPluginProcess.addReinitializer(key, r)
	return r
}

// reinitialize reinitializes a re-dispensed plugin using the initial data passed to Init().
func (r *restartableAdaptedV1ObjectStore) reinitialize(dispensed interface{}) error {
	objectStore, ok := dispensed.(objectstorev1.ObjectStore)
	if !ok {
		return errors.Errorf("%T is not a ObjectStore!", dispensed)
	}

	return r.init(objectStore, r.config)
}

// getObjectStore returns the object store for this restartableObjectStore. It does *not* restart the
// plugin process.
func (r *restartableAdaptedV1ObjectStore) getObjectStore() (objectstorev1.ObjectStore, error) {
	plugin, err := r.sharedPluginProcess.getByKindAndName(r.key)
	if err != nil {
		return nil, err
	}

	objectStore, ok := plugin.(objectstorev1.ObjectStore)
	if !ok {
		return nil, errors.Errorf("%T is not a ObjectStore!", plugin)
	}

	return objectStore, nil
}

// getDelegate restarts the plugin process (if needed) and returns the object store for this restartableObjectStore.
func (r *restartableAdaptedV1ObjectStore) getDelegate() (objectstorev1.ObjectStore, error) {
	if err := r.sharedPluginProcess.resetIfNeeded(); err != nil {
		return nil, err
	}

	return r.getObjectStore()
}

// Init initializes the object store instance using config. If this is the first invocation, r stores config for future
// reinitialization needs. Init does NOT restart the shared plugin process. Init may only be called once.
func (r *restartableAdaptedV1ObjectStore) Init(config map[string]string) error {
	if r.config != nil {
		return errors.Errorf("already initialized")
	}

	// Not using getDelegate() to avoid possible infinite recursion
	delegate, err := r.getObjectStore()
	if err != nil {
		return err
	}

	r.config = config

	return r.init(delegate, config)
}

func (r *restartableAdaptedV1ObjectStore) InitV2(ctx context.Context, config map[string]string) error {
	return r.Init(config)
}

// init calls Init on objectStore with config. This is split out from Init() so that both Init() and reinitialize() may
// call it using a specific ObjectStore.
func (r *restartableAdaptedV1ObjectStore) init(objectStore objectstorev1.ObjectStore, config map[string]string) error {
	return objectStore.Init(config)
}

// PutObject restarts the plugin's process if needed, then delegates the call.
func (r *restartableAdaptedV1ObjectStore) PutObject(bucket string, key string, body io.Reader) error {
	delegate, err := r.getDelegate()
	if err != nil {
		return err
	}
	return delegate.PutObject(bucket, key, body)
}

// ObjectExists restarts the plugin's process if needed, then delegates the call.
func (r *restartableAdaptedV1ObjectStore) ObjectExists(bucket, key string) (bool, error) {
	delegate, err := r.getDelegate()
	if err != nil {
		return false, err
	}
	return delegate.ObjectExists(bucket, key)
}

// GetObject restarts the plugin's process if needed, then delegates the call.
func (r *restartableAdaptedV1ObjectStore) GetObject(bucket string, key string) (io.ReadCloser, error) {
	delegate, err := r.getDelegate()
	if err != nil {
		return nil, err
	}
	return delegate.GetObject(bucket, key)
}

// ListCommonPrefixes restarts the plugin's process if needed, then delegates the call.
func (r *restartableAdaptedV1ObjectStore) ListCommonPrefixes(bucket string, prefix string, delimiter string) ([]string, error) {
	delegate, err := r.getDelegate()
	if err != nil {
		return nil, err
	}
	return delegate.ListCommonPrefixes(bucket, prefix, delimiter)
}

// ListObjects restarts the plugin's process if needed, then delegates the call.
func (r *restartableAdaptedV1ObjectStore) ListObjects(bucket string, prefix string) ([]string, error) {
	delegate, err := r.getDelegate()
	if err != nil {
		return nil, err
	}
	return delegate.ListObjects(bucket, prefix)
}

// DeleteObject restarts the plugin's process if needed, then delegates the call.
func (r *restartableAdaptedV1ObjectStore) DeleteObject(bucket string, key string) error {
	delegate, err := r.getDelegate()
	if err != nil {
		return err
	}
	return delegate.DeleteObject(bucket, key)
}

// CreateSignedURL restarts the plugin's process if needed, then delegates the call.
func (r *restartableAdaptedV1ObjectStore) CreateSignedURL(bucket string, key string, ttl time.Duration) (string, error) {
	delegate, err := r.getDelegate()
	if err != nil {
		return "", err
	}
	return delegate.CreateSignedURL(bucket, key, ttl)
}

// Version 2.  Simply discard ctx.
// PutObjectV2 restarts the plugin's process if needed, then delegates the call.
func (r *restartableAdaptedV1ObjectStore) PutObjectV2(ctx context.Context, bucket string, key string, body io.Reader) error {
	delegate, err := r.getDelegate()
	if err != nil {
		return err
	}
	return delegate.PutObject(bucket, key, body)
}

// ObjectExistsV2 restarts the plugin's process if needed, then delegates the call.
func (r *restartableAdaptedV1ObjectStore) ObjectExistsV2(ctx context.Context, bucket, key string) (bool, error) {
	delegate, err := r.getDelegate()
	if err != nil {
		return false, err
	}
	return delegate.ObjectExists(bucket, key)
}

// GetObjectV2 restarts the plugin's process if needed, then delegates the call.
func (r *restartableAdaptedV1ObjectStore) GetObjectV2(ctx context.Context, bucket string, key string) (io.ReadCloser, error) {
	delegate, err := r.getDelegate()
	if err != nil {
		return nil, err
	}
	return delegate.GetObject(bucket, key)
}

// ListCommonPrefixesV2 restarts the plugin's process if needed, then delegates the call.
func (r *restartableAdaptedV1ObjectStore) ListCommonPrefixesV2(
	ctx context.Context, bucket string, prefix string, delimiter string) ([]string, error) {
	delegate, err := r.getDelegate()
	if err != nil {
		return nil, err
	}
	return delegate.ListCommonPrefixes(bucket, prefix, delimiter)
}

// ListObjectsV2 restarts the plugin's process if needed, then delegates the call.
func (r *restartableAdaptedV1ObjectStore) ListObjectsV2(ctx context.Context, bucket string, prefix string) ([]string, error) {
	delegate, err := r.getDelegate()
	if err != nil {
		return nil, err
	}
	return delegate.ListObjects(bucket, prefix)
}

// DeleteObjectV2 restarts the plugin's process if needed, then delegates the call.
func (r *restartableAdaptedV1ObjectStore) DeleteObjectV2(ctx context.Context, bucket string, key string) error {
	delegate, err := r.getDelegate()
	if err != nil {
		return err
	}
	return delegate.DeleteObject(bucket, key)
}

// CreateSignedURLV2 restarts the plugin's process if needed, then delegates the call.
func (r *restartableAdaptedV1ObjectStore) CreateSignedURLV2(ctx context.Context, bucket string, key string, ttl time.Duration) (string, error) {
	delegate, err := r.getDelegate()
	if err != nil {
		return "", err
	}
	return delegate.CreateSignedURL(bucket, key, ttl)
}
