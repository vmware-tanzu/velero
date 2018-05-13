package plugin

import (
	"github.com/heptio/ark/pkg/cloudprovider"
	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/runtime"
)

type resumableBlockStore struct {
	key     kindAndName
	wrapper *wrapper
	config  map[string]string
}

func newResumableBlockStore(name string, wrapper *wrapper) *resumableBlockStore {
	key := kindAndName{kind: PluginKindBlockStore, name: name}
	r := &resumableBlockStore{
		key:     key,
		wrapper: wrapper,
	}
	wrapper.addReinitializer(key, r)
	return r
}

func (r *resumableBlockStore) reinitialize(dispensed interface{}) error {
	blockStore, ok := dispensed.(cloudprovider.BlockStore)
	if !ok {
		return errors.Errorf("%T is not an BlockStore!", dispensed)
	}
	return r.init(blockStore, r.config)
}

func (r *resumableBlockStore) getBlockStore() (cloudprovider.BlockStore, error) {
	plugin, err := r.wrapper.getByKindAndName(r.key)
	if err != nil {
		return nil, err
	}

	blockStore, ok := plugin.(cloudprovider.BlockStore)
	if !ok {
		return nil, errors.Errorf("%T is not an BlockStore!", plugin)
	}

	return blockStore, nil
}

func (r *resumableBlockStore) getDelegate() (cloudprovider.BlockStore, error) {
	if _, err := r.wrapper.resetIfNeeded(); err != nil {
		return nil, err
	}

	return r.getBlockStore()
}

func (r *resumableBlockStore) Init(config map[string]string) error {
	// Not using getDelegate() to avoid possible infinite recursion
	delegate, err := r.getBlockStore()
	if err != nil {
		return err
	}
	if r.config == nil {
		r.config = config
	}
	return r.init(delegate, config)
}

func (r *resumableBlockStore) init(blockStore cloudprovider.BlockStore, config map[string]string) error {
	return blockStore.Init(config)
}

func (r *resumableBlockStore) CreateVolumeFromSnapshot(snapshotID string, volumeType string, volumeAZ string, iops *int64) (volumeID string, err error) {
	delegate, err := r.getDelegate()
	if err != nil {
		return "", err
	}
	return delegate.CreateVolumeFromSnapshot(snapshotID, volumeType, volumeAZ, iops)
}

func (r *resumableBlockStore) GetVolumeID(pv runtime.Unstructured) (string, error) {
	delegate, err := r.getDelegate()
	if err != nil {
		return "", err
	}
	return delegate.GetVolumeID(pv)
}

func (r *resumableBlockStore) SetVolumeID(pv runtime.Unstructured, volumeID string) (runtime.Unstructured, error) {
	delegate, err := r.getDelegate()
	if err != nil {
		return nil, err
	}
	return delegate.SetVolumeID(pv, volumeID)
}

func (r *resumableBlockStore) GetVolumeInfo(volumeID string, volumeAZ string) (string, *int64, error) {
	delegate, err := r.getDelegate()
	if err != nil {
		return "", nil, err
	}
	return delegate.GetVolumeInfo(volumeID, volumeAZ)
}

func (r *resumableBlockStore) IsVolumeReady(volumeID string, volumeAZ string) (ready bool, err error) {
	delegate, err := r.getDelegate()
	if err != nil {
		return false, err
	}
	return delegate.IsVolumeReady(volumeID, volumeAZ)
}

func (r *resumableBlockStore) CreateSnapshot(volumeID string, volumeAZ string, tags map[string]string) (snapshotID string, err error) {
	delegate, err := r.getDelegate()
	if err != nil {
		return "", err
	}
	return delegate.CreateSnapshot(volumeID, volumeAZ, tags)
}

func (r *resumableBlockStore) DeleteSnapshot(snapshotID string) error {
	delegate, err := r.getDelegate()
	if err != nil {
		return err
	}
	return delegate.DeleteSnapshot(snapshotID)
}
