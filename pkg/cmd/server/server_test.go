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

package server

import (
	"testing"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/heptio/ark/pkg/apis/ark/v1"
	arktest "github.com/heptio/ark/pkg/util/test"
)

func TestArkResourcesExist(t *testing.T) {
	var (
		fakeDiscoveryHelper = &arktest.FakeDiscoveryHelper{}
		server              = &server{
			logger:          arktest.NewLogger(),
			discoveryHelper: fakeDiscoveryHelper,
		}
	)

	// Ark API group doesn't exist in discovery: should error
	fakeDiscoveryHelper.ResourceList = []*metav1.APIResourceList{
		{
			GroupVersion: "foo/v1",
			APIResources: []metav1.APIResource{
				{
					Name: "Backups",
					Kind: "Backup",
				},
			},
		},
	}
	assert.Error(t, server.arkResourcesExist())

	// Ark API group doesn't contain any custom resources: should error
	arkAPIResourceList := &metav1.APIResourceList{
		GroupVersion: v1.SchemeGroupVersion.String(),
	}

	fakeDiscoveryHelper.ResourceList = append(fakeDiscoveryHelper.ResourceList, arkAPIResourceList)
	assert.Error(t, server.arkResourcesExist())

	// Ark API group contains all custom resources: should not error
	for kind := range v1.CustomResources() {
		arkAPIResourceList.APIResources = append(arkAPIResourceList.APIResources, metav1.APIResource{
			Kind: kind,
		})
	}
	assert.NoError(t, server.arkResourcesExist())

	// Ark API group contains some but not all custom resources: should error
	arkAPIResourceList.APIResources = arkAPIResourceList.APIResources[:3]
	assert.Error(t, server.arkResourcesExist())
}
