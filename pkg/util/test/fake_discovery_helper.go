/*
Copyright 2017 Heptio Inc.

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
