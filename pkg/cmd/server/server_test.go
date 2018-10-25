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
	fakeclientset "github.com/heptio/ark/pkg/generated/clientset/versioned/fake"
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

func TestDefaultVolumeSnapshotLocations(t *testing.T) {
	namespace := "heptio-ark"
	arkClient := fakeclientset.NewSimpleClientset()

	location := &v1.VolumeSnapshotLocation{ObjectMeta: metav1.ObjectMeta{Name: "location1"}, Spec: v1.VolumeSnapshotLocationSpec{Provider: "provider1"}}
	arkClient.ArkV1().VolumeSnapshotLocations(namespace).Create(location)

	defaultVolumeSnapshotLocations := make(map[string]string)

	// No default: a default VSL does not need to be specified if there’s only one vsl for a given provider
	volumeSnapshotLocations, err := getVolumeSnapshotLocations(arkClient, namespace, defaultVolumeSnapshotLocations)
	assert.Equal(t, 1, len(volumeSnapshotLocations))
	assert.NoError(t, err)

	// 1 default
	defaultVolumeSnapshotLocations["provider1"] = "location1"
	volumeSnapshotLocations, err = getVolumeSnapshotLocations(arkClient, namespace, defaultVolumeSnapshotLocations)
	assert.Equal(t, 1, len(volumeSnapshotLocations))
	assert.NoError(t, err)

	// No default fails when there’s more than one vsl for a that provider
	defaultVolumeSnapshotLocations = make(map[string]string)
	location3 := &v1.VolumeSnapshotLocation{ObjectMeta: metav1.ObjectMeta{Name: "location3"}, Spec: v1.VolumeSnapshotLocationSpec{Provider: "provider1"}}
	arkClient.ArkV1().VolumeSnapshotLocations(namespace).Create(location3)
	volumeSnapshotLocations, err = getVolumeSnapshotLocations(arkClient, namespace, defaultVolumeSnapshotLocations)
	assert.Equal(t, 0, len(volumeSnapshotLocations))
	assert.Error(t, err)

	// Non-existing provider given in a < 2 vsl scenario
	defaultVolumeSnapshotLocations["provider1"] = "badlocation"
	volumeSnapshotLocations, err = getVolumeSnapshotLocations(arkClient, namespace, defaultVolumeSnapshotLocations)
	assert.Equal(t, 1, len(volumeSnapshotLocations))
	assert.NoError(t, err)

	// Non-existing provider given in a < 2 vsls scenario
	defaultVolumeSnapshotLocations["provider2"] = "badlocation"
	volumeSnapshotLocations, err = getVolumeSnapshotLocations(arkClient, namespace, defaultVolumeSnapshotLocations)
	assert.Equal(t, 1, len(volumeSnapshotLocations))
	assert.NoError(t, err)

	// Good provider, good location
	delete(defaultVolumeSnapshotLocations, "provider2")
	defaultVolumeSnapshotLocations["provider1"] = "location1"
	volumeSnapshotLocations, err = getVolumeSnapshotLocations(arkClient, namespace, defaultVolumeSnapshotLocations)
	assert.Equal(t, 1, len(volumeSnapshotLocations))
	assert.NoError(t, err)

	location2 := &v1.VolumeSnapshotLocation{ObjectMeta: metav1.ObjectMeta{Name: "location2"}, Spec: v1.VolumeSnapshotLocationSpec{Provider: "provider2"}}
	arkClient.ArkV1().VolumeSnapshotLocations(namespace).Create(location2)

	// Mutliple Provider/Location 1 good, 1 bad, < 2 vsls
	defaultVolumeSnapshotLocations["provider2"] = "badlocation"
	volumeSnapshotLocations, err = getVolumeSnapshotLocations(arkClient, namespace, defaultVolumeSnapshotLocations)
	assert.NoError(t, err)

	location21 := &v1.VolumeSnapshotLocation{ObjectMeta: metav1.ObjectMeta{Name: "location2-1"}, Spec: v1.VolumeSnapshotLocationSpec{Provider: "provider2"}}
	arkClient.ArkV1().VolumeSnapshotLocations(namespace).Create(location21)

	location11 := &v1.VolumeSnapshotLocation{ObjectMeta: metav1.ObjectMeta{Name: "location1-1"}, Spec: v1.VolumeSnapshotLocationSpec{Provider: "provider1"}}
	arkClient.ArkV1().VolumeSnapshotLocations(namespace).Create(location11)

	// Mutliple Provider/Location all good
	defaultVolumeSnapshotLocations["provider2"] = "location2"
	volumeSnapshotLocations, err = getVolumeSnapshotLocations(arkClient, namespace, defaultVolumeSnapshotLocations)
	assert.Equal(t, 2, len(volumeSnapshotLocations))
	assert.NoError(t, err)
	assert.Equal(t, "location1", volumeSnapshotLocations["provider1"].ObjectMeta.Name)
	assert.Equal(t, "location2", volumeSnapshotLocations["provider2"].ObjectMeta.Name)
}
