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
package fake

import (
	ark_v1 "github.com/heptio/ark/pkg/apis/ark/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

// Get takes name of the schedule, and returns the corresponding schedule object, and an error if there is any.
func (c *FakeSchedules) Get(name string, options v1.GetOptions) (result *ark_v1.Schedule, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewGetAction(schedulesResource, c.ns, name), &ark_v1.Schedule{})

	if obj == nil {
		return nil, err
	}
	return obj.(*ark_v1.Schedule), err
}

// List takes label and field selectors, and returns the list of Schedules that match those selectors.
func (c *FakeSchedules) List(opts v1.ListOptions) (result *ark_v1.ScheduleList, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewListAction(schedulesResource, schedulesKind, c.ns, opts), &ark_v1.ScheduleList{})

	if obj == nil {
		return nil, err
	}

	label, _, _ := testing.ExtractFromListOptions(opts)
	if label == nil {
		label = labels.Everything()
	}
	list := &ark_v1.ScheduleList{}
	for _, item := range obj.(*ark_v1.ScheduleList).Items {
		if label.Matches(labels.Set(item.Labels)) {
			list.Items = append(list.Items, item)
		}
	}
	return list, err
}

// Watch returns a watch.Interface that watches the requested schedules.
func (c *FakeSchedules) Watch(opts v1.ListOptions) (watch.Interface, error) {
	return c.Fake.
		InvokesWatch(testing.NewWatchAction(schedulesResource, c.ns, opts))

}

// Create takes the representation of a schedule and creates it.  Returns the server's representation of the schedule, and an error, if there is any.
func (c *FakeSchedules) Create(schedule *ark_v1.Schedule) (result *ark_v1.Schedule, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewCreateAction(schedulesResource, c.ns, schedule), &ark_v1.Schedule{})

	if obj == nil {
		return nil, err
	}
	return obj.(*ark_v1.Schedule), err
}

// Update takes the representation of a schedule and updates it. Returns the server's representation of the schedule, and an error, if there is any.
func (c *FakeSchedules) Update(schedule *ark_v1.Schedule) (result *ark_v1.Schedule, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewUpdateAction(schedulesResource, c.ns, schedule), &ark_v1.Schedule{})

	if obj == nil {
		return nil, err
	}
	return obj.(*ark_v1.Schedule), err
}

// UpdateStatus was generated because the type contains a Status member.
// Add a +genclient:noStatus comment above the type to avoid generating UpdateStatus().
func (c *FakeSchedules) UpdateStatus(schedule *ark_v1.Schedule) (*ark_v1.Schedule, error) {
	obj, err := c.Fake.
		Invokes(testing.NewUpdateSubresourceAction(schedulesResource, "status", c.ns, schedule), &ark_v1.Schedule{})

	if obj == nil {
		return nil, err
	}
	return obj.(*ark_v1.Schedule), err
}

// Delete takes name of the schedule and deletes it. Returns an error if one occurs.
func (c *FakeSchedules) Delete(name string, options *v1.DeleteOptions) error {
	_, err := c.Fake.
		Invokes(testing.NewDeleteAction(schedulesResource, c.ns, name), &ark_v1.Schedule{})

	return err
}

// DeleteCollection deletes a collection of objects.
func (c *FakeSchedules) DeleteCollection(options *v1.DeleteOptions, listOptions v1.ListOptions) error {
	action := testing.NewDeleteCollectionAction(schedulesResource, c.ns, listOptions)

	_, err := c.Fake.Invokes(action, &ark_v1.ScheduleList{})
	return err
}

// Patch applies the patch and returns the patched schedule.
func (c *FakeSchedules) Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *ark_v1.Schedule, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewPatchSubresourceAction(schedulesResource, c.ns, name, data, subresources...), &ark_v1.Schedule{})

	if obj == nil {
		return nil, err
	}
	return obj.(*ark_v1.Schedule), err
}
