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

// FakeConfigs implements ConfigInterface
type FakeConfigs struct {
	Fake *FakeArkV1
	ns   string
}

var configsResource = schema.GroupVersionResource{Group: "ark.heptio.com", Version: "v1", Resource: "configs"}

var configsKind = schema.GroupVersionKind{Group: "ark.heptio.com", Version: "v1", Kind: "Config"}

func (c *FakeConfigs) Create(config *v1.Config) (result *v1.Config, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewCreateAction(configsResource, c.ns, config), &v1.Config{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1.Config), err
}

func (c *FakeConfigs) Update(config *v1.Config) (result *v1.Config, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewUpdateAction(configsResource, c.ns, config), &v1.Config{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1.Config), err
}

func (c *FakeConfigs) Delete(name string, options *meta_v1.DeleteOptions) error {
	_, err := c.Fake.
		Invokes(testing.NewDeleteAction(configsResource, c.ns, name), &v1.Config{})

	return err
}

func (c *FakeConfigs) DeleteCollection(options *meta_v1.DeleteOptions, listOptions meta_v1.ListOptions) error {
	action := testing.NewDeleteCollectionAction(configsResource, c.ns, listOptions)

	_, err := c.Fake.Invokes(action, &v1.ConfigList{})
	return err
}

func (c *FakeConfigs) Get(name string, options meta_v1.GetOptions) (result *v1.Config, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewGetAction(configsResource, c.ns, name), &v1.Config{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1.Config), err
}

func (c *FakeConfigs) List(opts meta_v1.ListOptions) (result *v1.ConfigList, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewListAction(configsResource, configsKind, c.ns, opts), &v1.ConfigList{})

	if obj == nil {
		return nil, err
	}

	label, _, _ := testing.ExtractFromListOptions(opts)
	if label == nil {
		label = labels.Everything()
	}
	list := &v1.ConfigList{}
	for _, item := range obj.(*v1.ConfigList).Items {
		if label.Matches(labels.Set(item.Labels)) {
			list.Items = append(list.Items, item)
		}
	}
	return list, err
}

// Watch returns a watch.Interface that watches the requested configs.
func (c *FakeConfigs) Watch(opts meta_v1.ListOptions) (watch.Interface, error) {
	return c.Fake.
		InvokesWatch(testing.NewWatchAction(configsResource, c.ns, opts))

}

// Patch applies the patch and returns the patched config.
func (c *FakeConfigs) Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *v1.Config, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewPatchSubresourceAction(configsResource, c.ns, name, data, subresources...), &v1.Config{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1.Config), err
}
