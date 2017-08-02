package fake

import (
	v1 "github.com/heptio/ark/pkg/apis/ark/v1"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	labels "k8s.io/apimachinery/pkg/labels"
	schema "k8s.io/apimachinery/pkg/runtime/schema"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	testing "k8s.io/client-go/testing"
)

// FakeBackups implements BackupInterface
type FakeBackups struct {
	Fake *FakeArkV1
	ns   string
}

var backupsResource = schema.GroupVersionResource{Group: "ark.heptio.com", Version: "v1", Resource: "backups"}

var backupsKind = schema.GroupVersionKind{Group: "ark.heptio.com", Version: "v1", Kind: "Backup"}

func (c *FakeBackups) Create(backup *v1.Backup) (result *v1.Backup, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewCreateAction(backupsResource, c.ns, backup), &v1.Backup{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1.Backup), err
}

func (c *FakeBackups) Update(backup *v1.Backup) (result *v1.Backup, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewUpdateAction(backupsResource, c.ns, backup), &v1.Backup{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1.Backup), err
}

func (c *FakeBackups) UpdateStatus(backup *v1.Backup) (*v1.Backup, error) {
	obj, err := c.Fake.
		Invokes(testing.NewUpdateSubresourceAction(backupsResource, "status", c.ns, backup), &v1.Backup{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1.Backup), err
}

func (c *FakeBackups) Delete(name string, options *meta_v1.DeleteOptions) error {
	_, err := c.Fake.
		Invokes(testing.NewDeleteAction(backupsResource, c.ns, name), &v1.Backup{})

	return err
}

func (c *FakeBackups) DeleteCollection(options *meta_v1.DeleteOptions, listOptions meta_v1.ListOptions) error {
	action := testing.NewDeleteCollectionAction(backupsResource, c.ns, listOptions)

	_, err := c.Fake.Invokes(action, &v1.BackupList{})
	return err
}

func (c *FakeBackups) Get(name string, options meta_v1.GetOptions) (result *v1.Backup, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewGetAction(backupsResource, c.ns, name), &v1.Backup{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1.Backup), err
}

func (c *FakeBackups) List(opts meta_v1.ListOptions) (result *v1.BackupList, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewListAction(backupsResource, backupsKind, c.ns, opts), &v1.BackupList{})

	if obj == nil {
		return nil, err
	}

	label, _, _ := testing.ExtractFromListOptions(opts)
	if label == nil {
		label = labels.Everything()
	}
	list := &v1.BackupList{}
	for _, item := range obj.(*v1.BackupList).Items {
		if label.Matches(labels.Set(item.Labels)) {
			list.Items = append(list.Items, item)
		}
	}
	return list, err
}

// Watch returns a watch.Interface that watches the requested backups.
func (c *FakeBackups) Watch(opts meta_v1.ListOptions) (watch.Interface, error) {
	return c.Fake.
		InvokesWatch(testing.NewWatchAction(backupsResource, c.ns, opts))

}

// Patch applies the patch and returns the patched backup.
func (c *FakeBackups) Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *v1.Backup, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewPatchSubresourceAction(backupsResource, c.ns, name, data, subresources...), &v1.Backup{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1.Backup), err
}
