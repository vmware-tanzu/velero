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

	"github.com/heptio/ark/pkg/client"
	"github.com/heptio/ark/pkg/discovery"
	"github.com/heptio/ark/pkg/podexec"
	"github.com/heptio/ark/pkg/restic"
	arktest "github.com/heptio/ark/pkg/util/test"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestBackupGroupBacksUpCorrectResourcesInCorrectOrder(t *testing.T) {
	resourceBackupperFactory := new(mockResourceBackupperFactory)
	resourceBackupper := new(mockResourceBackupper)

	defer resourceBackupperFactory.AssertExpectations(t)
	defer resourceBackupper.AssertExpectations(t)

	resourceBackupperFactory.On("newResourceBackupper",
		mock.Anything,
		mock.Anything,
		mock.Anything,
		mock.Anything,
		mock.Anything,
		mock.Anything,
		mock.Anything,
		mock.Anything,
		mock.Anything,
		mock.Anything,
		mock.Anything,
	).Return(resourceBackupper)

	gb := &defaultGroupBackupper{
		log: arktest.NewLogger(),
		resourceBackupperFactory: resourceBackupperFactory,
	}

	group := &metav1.APIResourceList{
		GroupVersion: "v1",
		APIResources: []metav1.APIResource{
			{Name: "persistentvolumes"},
			{Name: "pods"},
			{Name: "persistentvolumeclaims"},
		},
	}

	var actualOrder []string
	runFunc := func(args mock.Arguments) {
		actualOrder = append(actualOrder, args.Get(1).(metav1.APIResource).Name)
	}

	resourceBackupper.On("backupResource", group, metav1.APIResource{Name: "pods"}).Return(nil).Run(runFunc)
	resourceBackupper.On("backupResource", group, metav1.APIResource{Name: "persistentvolumeclaims"}).Return(nil).Run(runFunc)
	resourceBackupper.On("backupResource", group, metav1.APIResource{Name: "persistentvolumes"}).Return(nil).Run(runFunc)

	require.NoError(t, gb.backupGroup(group))

	// make sure PVs were last
	assert.Equal(t, []string{"pods", "persistentvolumeclaims", "persistentvolumes"}, actualOrder)
}

type mockResourceBackupperFactory struct {
	mock.Mock
}

func (rbf *mockResourceBackupperFactory) newResourceBackupper(
	log logrus.FieldLogger,
	backup *Request,
	dynamicFactory client.DynamicFactory,
	discoveryHelper discovery.Helper,
	backedUpItems map[itemKey]struct{},
	cohabitatingResources map[string]*cohabitatingResource,
	podCommandExecutor podexec.PodCommandExecutor,
	tarWriter tarWriter,
	resticBackupper restic.Backupper,
	resticSnapshotTracker *pvcSnapshotTracker,
	blockStoreGetter BlockStoreGetter,
) resourceBackupper {
	args := rbf.Called(
		log,
		backup,
		dynamicFactory,
		discoveryHelper,
		backedUpItems,
		cohabitatingResources,
		podCommandExecutor,
		tarWriter,
		resticBackupper,
		resticSnapshotTracker,
		blockStoreGetter,
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

func TestSortCoreGroup(t *testing.T) {
	group := &metav1.APIResourceList{
		GroupVersion: "v1",
		APIResources: []metav1.APIResource{
			{Name: "persistentvolumes"},
			{Name: "configmaps"},
			{Name: "antelopes"},
			{Name: "persistentvolumeclaims"},
			{Name: "pods"},
		},
	}

	sortCoreGroup(group)

	expected := []string{
		"pods",
		"persistentvolumeclaims",
		"persistentvolumes",
		"configmaps",
		"antelopes",
	}
	for i, r := range group.APIResources {
		assert.Equal(t, expected[i], r.Name)
	}
}
