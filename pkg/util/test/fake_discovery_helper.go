package test

import (
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

type FakeDiscoveryHelper struct {
	ResourceList []*metav1.APIResourceList
	RESTMapper   meta.RESTMapper
}

func (dh *FakeDiscoveryHelper) Mapper() meta.RESTMapper {
	return dh.RESTMapper
}

func (dh *FakeDiscoveryHelper) Resources() []*metav1.APIResourceList {
	return dh.ResourceList
}
func (dh *FakeDiscoveryHelper) Refresh() error {
	return nil
}

func (dh *FakeDiscoveryHelper) ResolveGroupResource(resource string) (schema.GroupResource, error) {
	gvr, err := dh.RESTMapper.ResourceFor(schema.ParseGroupResource(resource).WithVersion(""))
	if err != nil {
		return schema.GroupResource{}, err
	}
	return gvr.GroupResource(), nil
}
