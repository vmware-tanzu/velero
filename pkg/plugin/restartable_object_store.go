/*
Copyright 2018 the Heptio Ark contributors.

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
package plugin

import (
	"io"
	"time"

	"github.com/pkg/errors"

	"github.com/heptio/ark/pkg/cloudprovider"
)

// restartableObjectStore is an object store for a given implementation (such as "aws"). It is associated with
// a restartableProcess, which may be shared and used to run multiple plugins. At the beginning of each method
// call, the restartableObjectStore asks its restartableProcess to restart itself if needed (e.g. if the
// process terminated for any reason), then it proceeds with the actual call.
type restartableObjectStore struct {
	key                 kindAndName
	sharedPluginProcess RestartableProcess
	// config contains the data used to initialize the plugin. It is used to reinitialize the plugin in the event its
	// sharedPluginProcess gets restarted.
	config map[string]string
}

// newRestartableObjectStore returns a new restartableObjectStore.
func newRestartableObjectStore(name string, sharedPluginProcess RestartableProcess) *restartableObjectStore {
	key := kindAndName{kind: PluginKindObjectStore, name: name}
	r := &restartableObjectStore{
		key:                 key,
		sharedPluginProcess: sharedPluginProcess,
	}

	// Register our reinitializer so we can reinitialize after a restart with r.config.
	sharedPluginProcess.addReinitializer(key, r)

	return r
}

// reinitialize reinitializes a re-dispensed plugin using the initial data passed to Init().
func (r *restartableObjectStore) reinitialize(dispensed interface{}) error {
	objectStore, ok := dispensed.(cloudprovider.ObjectStore)
	if !ok {
		return errors.Errorf("%T is not a cloudprovider.ObjectStore!", dispensed)
	}

	return r.init(objectStore, r.config)
}

// getObjectStore returns the object store for this restartableObjectStore. It does *not* restart the
// plugin process.
func (r *restartableObjectStore) getObjectStore() (cloudprovider.ObjectStore, error) {
	plugin, err := r.sharedPluginProcess.getByKindAndName(r.key)
	if err != nil {
		return nil, err
	}

	objectStore, ok := plugin.(cloudprovider.ObjectStore)
	if !ok {
		return nil, errors.Errorf("%T is not a cloudprovider.ObjectStore!", plugin)
	}

	return objectStore, nil
}

// getDelegate restarts the plugin process (if needed) and returns the object store for this restartableObjectStore.
func (r *restartableObjectStore) getDelegate() (cloudprovider.ObjectStore, error) {
	if err := r.sharedPluginProcess.resetIfNeeded(); err != nil {
		return nil, err
	}

	return r.getObjectStore()
}

// Init initializes the object store instance using config. If this is the first invocation, r stores config for future
// reinitialization needs. Init does NOT restart the shared plugin process. Init may only be called once.
func (r *restartableObjectStore) Init(config map[string]string) error {
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

// init calls Init on objectStore with config. This is split out from Init() so that both Init() and reinitialize() may
// call it using a specific ObjectStore.
func (r *restartableObjectStore) init(objectStore cloudprovider.ObjectStore, config map[string]string) error {
	return objectStore.Init(config)
}

// PutObject restarts the plugin's process if needed, then delegates the call.
func (r *restartableObjectStore) PutObject(bucket string, key string, body io.Reader) error {
	delegate, err := r.getDelegate()
	if err != nil {
		return err
	}
	return delegate.PutObject(bucket, key, body)
}

// GetObject restarts the plugin's process if needed, then delegates the call.
func (r *restartableObjectStore) GetObject(bucket string, key string) (io.ReadCloser, error) {
	delegate, err := r.getDelegate()
	if err != nil {
		return nil, err
	}
	return delegate.GetObject(bucket, key)
}

// ListCommonPrefixes restarts the plugin's process if needed, then delegates the call.
func (r *restartableObjectStore) ListCommonPrefixes(bucket string, prefix string, delimiter string) ([]string, error) {
	delegate, err := r.getDelegate()
	if err != nil {
		return nil, err
	}
	return delegate.ListCommonPrefixes(bucket, prefix, delimiter)
}

// ListObjects restarts the plugin's process if needed, then delegates the call.
func (r *restartableObjectStore) ListObjects(bucket string, prefix string) ([]string, error) {
	delegate, err := r.getDelegate()
	if err != nil {
		return nil, err
	}
	return delegate.ListObjects(bucket, prefix)
}

// DeleteObject restarts the plugin's process if needed, then delegates the call.
func (r *restartableObjectStore) DeleteObject(bucket string, key string) error {
	delegate, err := r.getDelegate()
	if err != nil {
		return err
	}
	return delegate.DeleteObject(bucket, key)
}

// CreateSignedURL restarts the plugin's process if needed, then delegates the call.
func (r *restartableObjectStore) CreateSignedURL(bucket string, key string, ttl time.Duration) (string, error) {
	delegate, err := r.getDelegate()
	if err != nil {
		return "", err
	}
	return delegate.CreateSignedURL(bucket, key, ttl)
}
