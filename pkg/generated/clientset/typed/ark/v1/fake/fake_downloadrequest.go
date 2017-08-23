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

// FakeDownloadRequests implements DownloadRequestInterface
type FakeDownloadRequests struct {
	Fake *FakeArkV1
	ns   string
}

var downloadrequestsResource = schema.GroupVersionResource{Group: "ark.heptio.com", Version: "v1", Resource: "downloadrequests"}

var downloadrequestsKind = schema.GroupVersionKind{Group: "ark.heptio.com", Version: "v1", Kind: "DownloadRequest"}

func (c *FakeDownloadRequests) Create(downloadRequest *v1.DownloadRequest) (result *v1.DownloadRequest, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewCreateAction(downloadrequestsResource, c.ns, downloadRequest), &v1.DownloadRequest{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1.DownloadRequest), err
}

func (c *FakeDownloadRequests) Update(downloadRequest *v1.DownloadRequest) (result *v1.DownloadRequest, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewUpdateAction(downloadrequestsResource, c.ns, downloadRequest), &v1.DownloadRequest{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1.DownloadRequest), err
}

func (c *FakeDownloadRequests) UpdateStatus(downloadRequest *v1.DownloadRequest) (*v1.DownloadRequest, error) {
	obj, err := c.Fake.
		Invokes(testing.NewUpdateSubresourceAction(downloadrequestsResource, "status", c.ns, downloadRequest), &v1.DownloadRequest{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1.DownloadRequest), err
}

func (c *FakeDownloadRequests) Delete(name string, options *meta_v1.DeleteOptions) error {
	_, err := c.Fake.
		Invokes(testing.NewDeleteAction(downloadrequestsResource, c.ns, name), &v1.DownloadRequest{})

	return err
}

func (c *FakeDownloadRequests) DeleteCollection(options *meta_v1.DeleteOptions, listOptions meta_v1.ListOptions) error {
	action := testing.NewDeleteCollectionAction(downloadrequestsResource, c.ns, listOptions)

	_, err := c.Fake.Invokes(action, &v1.DownloadRequestList{})
	return err
}

func (c *FakeDownloadRequests) Get(name string, options meta_v1.GetOptions) (result *v1.DownloadRequest, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewGetAction(downloadrequestsResource, c.ns, name), &v1.DownloadRequest{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1.DownloadRequest), err
}

func (c *FakeDownloadRequests) List(opts meta_v1.ListOptions) (result *v1.DownloadRequestList, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewListAction(downloadrequestsResource, downloadrequestsKind, c.ns, opts), &v1.DownloadRequestList{})

	if obj == nil {
		return nil, err
	}

	label, _, _ := testing.ExtractFromListOptions(opts)
	if label == nil {
		label = labels.Everything()
	}
	list := &v1.DownloadRequestList{}
	for _, item := range obj.(*v1.DownloadRequestList).Items {
		if label.Matches(labels.Set(item.Labels)) {
			list.Items = append(list.Items, item)
		}
	}
	return list, err
}

// Watch returns a watch.Interface that watches the requested downloadRequests.
func (c *FakeDownloadRequests) Watch(opts meta_v1.ListOptions) (watch.Interface, error) {
	return c.Fake.
		InvokesWatch(testing.NewWatchAction(downloadrequestsResource, c.ns, opts))

}

// Patch applies the patch and returns the patched downloadRequest.
func (c *FakeDownloadRequests) Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *v1.DownloadRequest, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewPatchSubresourceAction(downloadrequestsResource, c.ns, name, data, subresources...), &v1.DownloadRequest{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1.DownloadRequest), err
}
