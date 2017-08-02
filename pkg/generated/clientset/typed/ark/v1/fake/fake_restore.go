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

// FakeRestores implements RestoreInterface
type FakeRestores struct {
	Fake *FakeArkV1
	ns   string
}

var restoresResource = schema.GroupVersionResource{Group: "ark.heptio.com", Version: "v1", Resource: "restores"}

var restoresKind = schema.GroupVersionKind{Group: "ark.heptio.com", Version: "v1", Kind: "Restore"}

func (c *FakeRestores) Create(restore *v1.Restore) (result *v1.Restore, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewCreateAction(restoresResource, c.ns, restore), &v1.Restore{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1.Restore), err
}

func (c *FakeRestores) Update(restore *v1.Restore) (result *v1.Restore, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewUpdateAction(restoresResource, c.ns, restore), &v1.Restore{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1.Restore), err
}

func (c *FakeRestores) UpdateStatus(restore *v1.Restore) (*v1.Restore, error) {
	obj, err := c.Fake.
		Invokes(testing.NewUpdateSubresourceAction(restoresResource, "status", c.ns, restore), &v1.Restore{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1.Restore), err
}

func (c *FakeRestores) Delete(name string, options *meta_v1.DeleteOptions) error {
	_, err := c.Fake.
		Invokes(testing.NewDeleteAction(restoresResource, c.ns, name), &v1.Restore{})

	return err
}

func (c *FakeRestores) DeleteCollection(options *meta_v1.DeleteOptions, listOptions meta_v1.ListOptions) error {
	action := testing.NewDeleteCollectionAction(restoresResource, c.ns, listOptions)

	_, err := c.Fake.Invokes(action, &v1.RestoreList{})
	return err
}

func (c *FakeRestores) Get(name string, options meta_v1.GetOptions) (result *v1.Restore, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewGetAction(restoresResource, c.ns, name), &v1.Restore{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1.Restore), err
}

func (c *FakeRestores) List(opts meta_v1.ListOptions) (result *v1.RestoreList, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewListAction(restoresResource, restoresKind, c.ns, opts), &v1.RestoreList{})

	if obj == nil {
		return nil, err
	}

	label, _, _ := testing.ExtractFromListOptions(opts)
	if label == nil {
		label = labels.Everything()
	}
	list := &v1.RestoreList{}
	for _, item := range obj.(*v1.RestoreList).Items {
		if label.Matches(labels.Set(item.Labels)) {
			list.Items = append(list.Items, item)
		}
	}
	return list, err
}

// Watch returns a watch.Interface that watches the requested restores.
func (c *FakeRestores) Watch(opts meta_v1.ListOptions) (watch.Interface, error) {
	return c.Fake.
		InvokesWatch(testing.NewWatchAction(restoresResource, c.ns, opts))

}

// Patch applies the patch and returns the patched restore.
func (c *FakeRestores) Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *v1.Restore, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewPatchSubresourceAction(restoresResource, c.ns, name, data, subresources...), &v1.Restore{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1.Restore), err
}
