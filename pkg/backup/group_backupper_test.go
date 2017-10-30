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

package backup

import (
	"testing"

	"github.com/heptio/ark/pkg/apis/ark/v1"
	"github.com/heptio/ark/pkg/client"
	"github.com/heptio/ark/pkg/discovery"
	"github.com/heptio/ark/pkg/util/collections"
	arktest "github.com/heptio/ark/pkg/util/test"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func TestBackupGroup(t *testing.T) {
	backup := &v1.Backup{}

	namespaces := collections.NewIncludesExcludes().Includes("a")
	resources := collections.NewIncludesExcludes().Includes("b")
	labelSelector := "foo=bar"

	dynamicFactory := &arktest.FakeDynamicFactory{}
	defer dynamicFactory.AssertExpectations(t)

	discoveryHelper := arktest.NewFakeDiscoveryHelper(true, nil)

	backedUpItems := map[itemKey]struct{}{
		{resource: "a", namespace: "b", name: "c"}: {},
	}

	cohabitatingResources := map[string]*cohabitatingResource{
		"a": {
			resource:       "a",
			groupResource1: schema.GroupResource{Group: "g1", Resource: "a"},
			groupResource2: schema.GroupResource{Group: "g2", Resource: "a"},
		},
	}

	actions := map[schema.GroupResource]Action{
		{Group: "", Resource: "pods"}: &fakeAction{},
	}

	podCommandExecutor := &mockPodCommandExecutor{}
	defer podCommandExecutor.AssertExpectations(t)

	tarWriter := &fakeTarWriter{}

	resourceHooks := []resourceHook{
		{name: "myhook"},
	}

	gb := (&defaultGroupBackupperFactory{}).newGroupBackupper(
		arktest.NewLogger(),
		backup,
		namespaces,
		resources,
		labelSelector,
		dynamicFactory,
		discoveryHelper,
		backedUpItems,
		cohabitatingResources,
		actions,
		podCommandExecutor,
		tarWriter,
		resourceHooks,
	).(*defaultGroupBackupper)

	resourceBackupperFactory := &mockResourceBackupperFactory{}
	defer resourceBackupperFactory.AssertExpectations(t)
	gb.resourceBackupperFactory = resourceBackupperFactory

	resourceBackupper := &mockResourceBackupper{}
	defer resourceBackupper.AssertExpectations(t)

	resourceBackupperFactory.On("newResourceBackupper",
		mock.Anything,
		backup,
		namespaces,
		resources,
		labelSelector,
		dynamicFactory,
		discoveryHelper,
		backedUpItems,
		cohabitatingResources,
		actions,
		podCommandExecutor,
		tarWriter,
		resourceHooks,
	).Return(resourceBackupper)

	group := &metav1.APIResourceList{
		GroupVersion: "v1",
		APIResources: []metav1.APIResource{
			{Name: "persistentvolumes"},
			{Name: "pods"},
			{Name: "persistentvolumeclaims"},
		},
	}

	expectedOrder := []string{"pods", "persistentvolumeclaims", "persistentvolumes"}
	var actualOrder []string

	runFunc := func(args mock.Arguments) {
		actualOrder = append(actualOrder, args.Get(1).(metav1.APIResource).Name)
	}

	resourceBackupper.On("backupResource", group, metav1.APIResource{Name: "pods"}).Return(nil).Run(runFunc)
	resourceBackupper.On("backupResource", group, metav1.APIResource{Name: "persistentvolumeclaims"}).Return(nil).Run(runFunc)
	resourceBackupper.On("backupResource", group, metav1.APIResource{Name: "persistentvolumes"}).Return(nil).Run(runFunc)

	err := gb.backupGroup(group)
	require.NoError(t, err)

	// make sure PVs were last
	assert.Equal(t, expectedOrder, actualOrder)
}

type mockResourceBackupperFactory struct {
	mock.Mock
}

func (rbf *mockResourceBackupperFactory) newResourceBackupper(
	log *logrus.Entry,
	backup *v1.Backup,
	namespaces *collections.IncludesExcludes,
	resources *collections.IncludesExcludes,
	labelSelector string,
	dynamicFactory client.DynamicFactory,
	discoveryHelper discovery.Helper,
	backedUpItems map[itemKey]struct{},
	cohabitatingResources map[string]*cohabitatingResource,
	actions map[schema.GroupResource]Action,
	podCommandExecutor podCommandExecutor,
	tarWriter tarWriter,
	resourceHooks []resourceHook,
) resourceBackupper {
	args := rbf.Called(
		log,
		backup,
		namespaces,
		resources,
		labelSelector,
		dynamicFactory,
		discoveryHelper,
		backedUpItems,
		cohabitatingResources,
		actions,
		podCommandExecutor,
		tarWriter,
		resourceHooks,
	)
	return args.Get(0).(resourceBackupper)
}

type mockResourceBackupper struct {
	mock.Mock
}

func (rb *mockResourceBackupper) backupResource(group *metav1.APIResourceList, resource metav1.APIResource) error {
	args := rb.Called(group, resource)
	return args.Error(0)
}
