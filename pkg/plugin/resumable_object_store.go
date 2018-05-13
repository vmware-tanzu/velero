package plugin

import (
	"io"
	"time"

	"github.com/heptio/ark/pkg/cloudprovider"
	"github.com/pkg/errors"
)

type resumableObjectStore struct {
	key     kindAndName
	wrapper *wrapper
	config  map[string]string
}

func newResumableObjectStore(name string, wrapper *wrapper) *resumableObjectStore {
	key := kindAndName{kind: PluginKindObjectStore, name: name}
	r := &resumableObjectStore{
		key:     key,
		wrapper: wrapper,
	}
	wrapper.addReinitializer(key, r)
	return r
}

func (r *resumableObjectStore) reinitialize(dispensed interface{}) error {
	objectStore, ok := dispensed.(cloudprovider.ObjectStore)
	if !ok {
		return errors.Errorf("%T is not an ObjectStore!", dispensed)
	}
	return r.init(objectStore, r.config)
}

func (r *resumableObjectStore) getObjectStore() (cloudprovider.ObjectStore, error) {
	plugin, err := r.wrapper.getByKindAndName(r.key)
	if err != nil {
		return nil, err
	}

	objectStore, ok := plugin.(cloudprovider.ObjectStore)
	if !ok {
		return nil, errors.Errorf("%T is not an ObjectStore!", plugin)
	}

	return objectStore, nil
}

func (r *resumableObjectStore) getDelegate() (cloudprovider.ObjectStore, error) {
	if _, err := r.wrapper.resetIfNeeded(); err != nil {
		return nil, err
	}

	return r.getObjectStore()
}

func (r *resumableObjectStore) Init(config map[string]string) error {
	// Not using getDelegate() to avoid possible infinite recursion
	delegate, err := r.getObjectStore()
	if err != nil {
		return err
	}
	if r.config == nil {
		r.config = config
	}
	return r.init(delegate, config)
}

func (r *resumableObjectStore) init(objectStore cloudprovider.ObjectStore, config map[string]string) error {
	return objectStore.Init(config)
}

func (r *resumableObjectStore) PutObject(bucket string, key string, body io.Reader) error {
	delegate, err := r.getDelegate()
	if err != nil {
		return err
	}
	return delegate.PutObject(bucket, key, body)
}

func (r *resumableObjectStore) GetObject(bucket string, key string) (io.ReadCloser, error) {
	delegate, err := r.getDelegate()
	if err != nil {
		return nil, err
	}
	return delegate.GetObject(bucket, key)
}

func (r *resumableObjectStore) ListCommonPrefixes(bucket string, delimiter string) ([]string, error) {
	delegate, err := r.getDelegate()
	if err != nil {
		return nil, err
	}
	return delegate.ListCommonPrefixes(bucket, delimiter)
}

func (r *resumableObjectStore) ListObjects(bucket string, prefix string) ([]string, error) {
	delegate, err := r.getDelegate()
	if err != nil {
		return nil, err
	}
	return delegate.ListObjects(bucket, prefix)
}

func (r *resumableObjectStore) DeleteObject(bucket string, key string) error {
	delegate, err := r.getDelegate()
	if err != nil {
		return err
	}
	return delegate.DeleteObject(bucket, key)
}

func (r *resumableObjectStore) CreateSignedURL(bucket string, key string, ttl time.Duration) (string, error) {
	delegate, err := r.getDelegate()
	if err != nil {
		return "", err
	}
	return delegate.CreateSignedURL(bucket, key, ttl)
}
