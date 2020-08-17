/*
Copyright 2017 the Velero contributors.

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
	"errors"
	"fmt"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/version"
	"k8s.io/client-go/discovery"
)

type FakeDiscoveryHelper struct {
	ResourceList       []*metav1.APIResourceList
	Mapper             meta.RESTMapper
	AutoReturnResource bool
	APIGroupsList      []metav1.APIGroup
	ServerVersionData  *version.Info
}

func (dh *FakeDiscoveryHelper) KindFor(input schema.GroupVersionKind) (schema.GroupVersionResource, metav1.APIResource, error) {
	panic("implement me")
}

func NewFakeDiscoveryHelper(autoReturnResource bool, resources map[schema.GroupVersionResource]schema.GroupVersionResource) *FakeDiscoveryHelper {
	helper := &FakeDiscoveryHelper{
		AutoReturnResource: autoReturnResource,
		Mapper: &FakeMapper{
			Resources: resources,
		},
	}

	if resources == nil {
		return helper
	}

	apiResourceMap := make(map[string][]metav1.APIResource)

	for _, gvr := range resources {
		var gvString string
		if gvr.Version != "" && gvr.Group != "" {
			gvString = fmt.Sprintf("%s/%s", gvr.Group, gvr.Version)
		} else {
			gvString = fmt.Sprintf("%s%s", gvr.Group, gvr.Version)
		}

		apiResourceMap[gvString] = append(apiResourceMap[gvString], metav1.APIResource{Name: gvr.Resource})
		helper.APIGroupsList = append(helper.APIGroupsList,
			metav1.APIGroup{
				Name: gvr.Group,
				PreferredVersion: metav1.GroupVersionForDiscovery{
					GroupVersion: gvString,
					Version:      gvr.Version,
				},
			})
	}

	for group, resources := range apiResourceMap {
		helper.ResourceList = append(helper.ResourceList, &metav1.APIResourceList{GroupVersion: group, APIResources: resources})
	}

	// FakeTest of version.Info

	serverVersion := &version.Info{
		Major:        "1",
		Minor:        "16",
		GitVersion:   "v1.16.4",
		GitCommit:    "FakeTest",
		GitTreeState: "",
		BuildDate:    "",
		GoVersion:    "",
		Compiler:     "",
		Platform:     "",
	}

	helper.ServerVersionData = serverVersion

	return helper
}

func (dh *FakeDiscoveryHelper) Resources() []*metav1.APIResourceList {
	return dh.ResourceList
}

func (dh *FakeDiscoveryHelper) Refresh() error {
	return nil
}

func (dh *FakeDiscoveryHelper) ResourceFor(input schema.GroupVersionResource) (schema.GroupVersionResource, metav1.APIResource, error) {
	if dh.AutoReturnResource {
		return schema.GroupVersionResource{
				Group:    input.Group,
				Version:  input.Version,
				Resource: input.Resource,
			},
			metav1.APIResource{
				Name: input.Resource,
			},
			nil
	}

	gvr, err := dh.Mapper.ResourceFor(input)
	if err != nil {
		return schema.GroupVersionResource{}, metav1.APIResource{}, err
	}

	var gvString string
	if gvr.Version != "" && gvr.Group != "" {
		gvString = fmt.Sprintf("%s/%s", gvr.Group, gvr.Version)
	} else {
		gvString = gvr.Version + gvr.Group
	}

	for _, gr := range dh.ResourceList {
		if gr.GroupVersion != gvString {
			continue
		}

		for _, resource := range gr.APIResources {
			if resource.Name == gvr.Resource {
				return gvr, resource, nil
			}
		}
	}

	return schema.GroupVersionResource{}, metav1.APIResource{}, errors.New("APIResource not found")
}

func (dh *FakeDiscoveryHelper) APIGroups() []metav1.APIGroup {
	return dh.APIGroupsList
}

type FakeServerResourcesInterface struct {
	ResourceList []*metav1.APIResourceList
	ApiGroup     []*metav1.APIGroup
	FailedGroups map[schema.GroupVersion]error
	ReturnError  error
}

func (di *FakeServerResourcesInterface) ServerPreferredResources() ([]*metav1.APIResourceList, error) {
	if di.ReturnError != nil {
		return di.ResourceList, di.ReturnError
	}
	if di.FailedGroups == nil || len(di.FailedGroups) == 0 {
		return di.ResourceList, nil
	}
	return di.ResourceList, &discovery.ErrGroupDiscoveryFailed{Groups: di.FailedGroups}
}

func (di *FakeServerResourcesInterface) ServerGroupsAndResources() ([]*metav1.APIGroup, []*metav1.APIResourceList, error) {
	if di.ReturnError != nil {
		return di.ApiGroup, di.ResourceList, di.ReturnError
	}
	if di.FailedGroups == nil || len(di.FailedGroups) == 0 {
		return di.ApiGroup, di.ResourceList, nil
	}
	return di.ApiGroup, di.ResourceList, &discovery.ErrGroupDiscoveryFailed{Groups: di.FailedGroups}
}

func NewFakeServerResourcesInterface(resourceList []*metav1.APIResourceList, apiGroup []*metav1.APIGroup, failedGroups map[schema.GroupVersion]error, returnError error) *FakeServerResourcesInterface {
	helper := &FakeServerResourcesInterface{
		ResourceList: resourceList,
		ApiGroup:     apiGroup,
		FailedGroups: failedGroups,
		ReturnError:  returnError,
	}
	return helper
}

func (dh *FakeDiscoveryHelper) ServerVersion() *version.Info {
	return dh.ServerVersionData
}
