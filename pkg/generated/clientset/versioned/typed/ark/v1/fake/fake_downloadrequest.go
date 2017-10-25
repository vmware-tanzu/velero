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

// FakeDownloadRequests implements DownloadRequestInterface
type FakeDownloadRequests struct {
	Fake *FakeArkV1
	ns   string
}

var downloadrequestsResource = schema.GroupVersionResource{Group: "ark.heptio.com", Version: "v1", Resource: "downloadrequests"}

var downloadrequestsKind = schema.GroupVersionKind{Group: "ark.heptio.com", Version: "v1", Kind: "DownloadRequest"}

// Get takes name of the downloadRequest, and returns the corresponding downloadRequest object, and an error if there is any.
func (c *FakeDownloadRequests) Get(name string, options v1.GetOptions) (result *ark_v1.DownloadRequest, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewGetAction(downloadrequestsResource, c.ns, name), &ark_v1.DownloadRequest{})

	if obj == nil {
		return nil, err
	}
	return obj.(*ark_v1.DownloadRequest), err
}

// List takes label and field selectors, and returns the list of DownloadRequests that match those selectors.
func (c *FakeDownloadRequests) List(opts v1.ListOptions) (result *ark_v1.DownloadRequestList, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewListAction(downloadrequestsResource, downloadrequestsKind, c.ns, opts), &ark_v1.DownloadRequestList{})

	if obj == nil {
		return nil, err
	}

	label, _, _ := testing.ExtractFromListOptions(opts)
	if label == nil {
		label = labels.Everything()
	}
	list := &ark_v1.DownloadRequestList{}
	for _, item := range obj.(*ark_v1.DownloadRequestList).Items {
		if label.Matches(labels.Set(item.Labels)) {
			list.Items = append(list.Items, item)
		}
	}
	return list, err
}

// Watch returns a watch.Interface that watches the requested downloadRequests.
func (c *FakeDownloadRequests) Watch(opts v1.ListOptions) (watch.Interface, error) {
	return c.Fake.
		InvokesWatch(testing.NewWatchAction(downloadrequestsResource, c.ns, opts))

}

// Create takes the representation of a downloadRequest and creates it.  Returns the server's representation of the downloadRequest, and an error, if there is any.
func (c *FakeDownloadRequests) Create(downloadRequest *ark_v1.DownloadRequest) (result *ark_v1.DownloadRequest, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewCreateAction(downloadrequestsResource, c.ns, downloadRequest), &ark_v1.DownloadRequest{})

	if obj == nil {
		return nil, err
	}
	return obj.(*ark_v1.DownloadRequest), err
}

// Update takes the representation of a downloadRequest and updates it. Returns the server's representation of the downloadRequest, and an error, if there is any.
func (c *FakeDownloadRequests) Update(downloadRequest *ark_v1.DownloadRequest) (result *ark_v1.DownloadRequest, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewUpdateAction(downloadrequestsResource, c.ns, downloadRequest), &ark_v1.DownloadRequest{})

	if obj == nil {
		return nil, err
	}
	return obj.(*ark_v1.DownloadRequest), err
}

// UpdateStatus was generated because the type contains a Status member.
// Add a +genclient:noStatus comment above the type to avoid generating UpdateStatus().
func (c *FakeDownloadRequests) UpdateStatus(downloadRequest *ark_v1.DownloadRequest) (*ark_v1.DownloadRequest, error) {
	obj, err := c.Fake.
		Invokes(testing.NewUpdateSubresourceAction(downloadrequestsResource, "status", c.ns, downloadRequest), &ark_v1.DownloadRequest{})

	if obj == nil {
		return nil, err
	}
	return obj.(*ark_v1.DownloadRequest), err
}

// Delete takes name of the downloadRequest and deletes it. Returns an error if one occurs.
func (c *FakeDownloadRequests) Delete(name string, options *v1.DeleteOptions) error {
	_, err := c.Fake.
		Invokes(testing.NewDeleteAction(downloadrequestsResource, c.ns, name), &ark_v1.DownloadRequest{})

	return err
}

// DeleteCollection deletes a collection of objects.
func (c *FakeDownloadRequests) DeleteCollection(options *v1.DeleteOptions, listOptions v1.ListOptions) error {
	action := testing.NewDeleteCollectionAction(downloadrequestsResource, c.ns, listOptions)

	_, err := c.Fake.Invokes(action, &ark_v1.DownloadRequestList{})
	return err
}

// Patch applies the patch and returns the patched downloadRequest.
func (c *FakeDownloadRequests) Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *ark_v1.DownloadRequest, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewPatchSubresourceAction(downloadrequestsResource, c.ns, name, data, subresources...), &ark_v1.DownloadRequest{})

	if obj == nil {
		return nil, err
	}
	return obj.(*ark_v1.DownloadRequest), err
}
