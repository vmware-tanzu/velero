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

package discovery

import (
	"errors"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"

	velerotest "github.com/vmware-tanzu/velero/pkg/test"
	"github.com/vmware-tanzu/velero/pkg/util/logging"
)

func TestSortResources(t *testing.T) {
	tests := []struct {
		name      string
		resources []*metav1.APIResourceList
		expected  []*metav1.APIResourceList
	}{
		{
			name: "no resources",
		},
		{
			name: "no extensions, order is preserved",
			resources: []*metav1.APIResourceList{
				{GroupVersion: "v1"},
				{GroupVersion: "groupC/v1"},
				{GroupVersion: "groupA/v1"},
				{GroupVersion: "groupB/v1"},
			},
			expected: []*metav1.APIResourceList{
				{GroupVersion: "v1"},
				{GroupVersion: "groupC/v1"},
				{GroupVersion: "groupA/v1"},
				{GroupVersion: "groupB/v1"},
			},
		},
		{
			name: "extensions moves to end, order is preserved",
			resources: []*metav1.APIResourceList{
				{GroupVersion: "extensions/v1beta1"},
				{GroupVersion: "v1"},
				{GroupVersion: "groupC/v1"},
				{GroupVersion: "groupA/v1"},
				{GroupVersion: "groupB/v1"},
				{GroupVersion: "apps/v1beta1"},
			},
			expected: []*metav1.APIResourceList{
				{GroupVersion: "v1"},
				{GroupVersion: "groupC/v1"},
				{GroupVersion: "groupA/v1"},
				{GroupVersion: "groupB/v1"},
				{GroupVersion: "apps/v1beta1"},
				{GroupVersion: "extensions/v1beta1"},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Logf("before")
			for _, r := range test.resources {
				t.Logf(r.GroupVersion)
			}
			sortResources(test.resources)
			t.Logf("after")
			for _, r := range test.resources {
				t.Logf(r.GroupVersion)
			}
			assert.Equal(t, test.expected, test.resources)
		})
	}
}

func TestFilteringByVerbs(t *testing.T) {
	tests := []struct {
		name         string
		groupVersion string
		res          *metav1.APIResource
		expected     bool
	}{
		{
			name:         "resource that supports list, create, get, delete",
			groupVersion: "v1",
			res: &metav1.APIResource{
				Verbs: metav1.Verbs{"list", "create", "get", "delete"},
			},
			expected: true,
		},
		{
			name:         "resource that supports list, create, get, delete in a different order",
			groupVersion: "v1",
			res: &metav1.APIResource{
				Verbs: metav1.Verbs{"delete", "get", "create", "list"},
			},
			expected: true,
		},
		{
			name:         "resource that supports list, create, get, delete, and more",
			groupVersion: "v1",
			res: &metav1.APIResource{
				Verbs: metav1.Verbs{"list", "create", "get", "delete", "update", "patch", "deletecollection"},
			},
			expected: true,
		},
		{
			name:         "resource that supports only list and create",
			groupVersion: "v1",
			res: &metav1.APIResource{
				Verbs: metav1.Verbs{"list", "create"},
			},
			expected: false,
		},
		{
			name:         "resource that supports only get and delete",
			groupVersion: "v1",
			res: &metav1.APIResource{
				Verbs: metav1.Verbs{"get", "delete"},
			},
			expected: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			out := filterByVerbs(test.groupVersion, test.res)
			assert.Equal(t, test.expected, out)
		})
	}
}

func TestRefreshServerPreferredResources(t *testing.T) {
	tests := []struct {
		name         string
		resourceList []*metav1.APIResourceList
		apiGroup     []*metav1.APIGroup
		failedGroups map[schema.GroupVersion]error
		returnError  error
	}{
		{
			name: "all groups discovered, no error is returned",
			resourceList: []*metav1.APIResourceList{
				{GroupVersion: "groupB/v1"},
				{GroupVersion: "apps/v1beta1"},
				{GroupVersion: "extensions/v1beta1"},
			},
		},
		{
			name: "failed to discover some groups, no error is returned",
			resourceList: []*metav1.APIResourceList{
				{GroupVersion: "groupB/v1"},
				{GroupVersion: "apps/v1beta1"},
				{GroupVersion: "extensions/v1beta1"},
			},
			failedGroups: map[schema.GroupVersion]error{
				{Group: "groupA", Version: "v1"}: errors.New("Fake error"),
				{Group: "groupC", Version: "v2"}: errors.New("Fake error"),
			},
		},
		{
			name:        "non ErrGroupDiscoveryFailed error, returns error",
			returnError: errors.New("Generic error"),
		},
	}

	formatFlag := logging.FormatText

	for _, test := range tests {
		fakeServer := velerotest.NewFakeServerResourcesInterface(test.resourceList, test.apiGroup, test.failedGroups, test.returnError)
		t.Run(test.name, func(t *testing.T) {
			resources, err := refreshServerPreferredResources(fakeServer, logging.DefaultLogger(logrus.DebugLevel, formatFlag))
			if test.returnError != nil {
				assert.NotNil(t, err)
			} else {
				assert.Nil(t, err)
				assert.Equal(t, test.returnError, err)
			}

			assert.Equal(t, test.resourceList, resources)
		})
	}

}
