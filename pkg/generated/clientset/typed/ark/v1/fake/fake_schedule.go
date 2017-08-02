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

// FakeSchedules implements ScheduleInterface
type FakeSchedules struct {
	Fake *FakeArkV1
	ns   string
}

var schedulesResource = schema.GroupVersionResource{Group: "ark.heptio.com", Version: "v1", Resource: "schedules"}

var schedulesKind = schema.GroupVersionKind{Group: "ark.heptio.com", Version: "v1", Kind: "Schedule"}

func (c *FakeSchedules) Create(schedule *v1.Schedule) (result *v1.Schedule, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewCreateAction(schedulesResource, c.ns, schedule), &v1.Schedule{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1.Schedule), err
}

func (c *FakeSchedules) Update(schedule *v1.Schedule) (result *v1.Schedule, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewUpdateAction(schedulesResource, c.ns, schedule), &v1.Schedule{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1.Schedule), err
}

func (c *FakeSchedules) UpdateStatus(schedule *v1.Schedule) (*v1.Schedule, error) {
	obj, err := c.Fake.
		Invokes(testing.NewUpdateSubresourceAction(schedulesResource, "status", c.ns, schedule), &v1.Schedule{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1.Schedule), err
}

func (c *FakeSchedules) Delete(name string, options *meta_v1.DeleteOptions) error {
	_, err := c.Fake.
		Invokes(testing.NewDeleteAction(schedulesResource, c.ns, name), &v1.Schedule{})

	return err
}

func (c *FakeSchedules) DeleteCollection(options *meta_v1.DeleteOptions, listOptions meta_v1.ListOptions) error {
	action := testing.NewDeleteCollectionAction(schedulesResource, c.ns, listOptions)

	_, err := c.Fake.Invokes(action, &v1.ScheduleList{})
	return err
}

func (c *FakeSchedules) Get(name string, options meta_v1.GetOptions) (result *v1.Schedule, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewGetAction(schedulesResource, c.ns, name), &v1.Schedule{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1.Schedule), err
}

func (c *FakeSchedules) List(opts meta_v1.ListOptions) (result *v1.ScheduleList, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewListAction(schedulesResource, schedulesKind, c.ns, opts), &v1.ScheduleList{})

	if obj == nil {
		return nil, err
	}

	label, _, _ := testing.ExtractFromListOptions(opts)
	if label == nil {
		label = labels.Everything()
	}
	list := &v1.ScheduleList{}
	for _, item := range obj.(*v1.ScheduleList).Items {
		if label.Matches(labels.Set(item.Labels)) {
			list.Items = append(list.Items, item)
		}
	}
	return list, err
}

// Watch returns a watch.Interface that watches the requested schedules.
func (c *FakeSchedules) Watch(opts meta_v1.ListOptions) (watch.Interface, error) {
	return c.Fake.
		InvokesWatch(testing.NewWatchAction(schedulesResource, c.ns, opts))

}

// Patch applies the patch and returns the patched schedule.
func (c *FakeSchedules) Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *v1.Schedule, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewPatchSubresourceAction(schedulesResource, c.ns, name, data, subresources...), &v1.Schedule{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1.Schedule), err
}
