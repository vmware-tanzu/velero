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

package kube

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"

	velerotest "github.com/heptio/velero/pkg/util/test"
)

func TestNamespaceAndName(t *testing.T) {
	//TODO
}

func TestEnsureNamespaceExistsAndIsReady(t *testing.T) {
	tests := []struct {
		name           string
		expectNSFound  bool
		nsPhase        v1.NamespacePhase
		nsDeleting     bool
		expectCreate   bool
		alreadyExists  bool
		expectedResult bool
	}{
		{
			name:           "namespace found, not deleting",
			expectNSFound:  true,
			expectedResult: true,
		},
		{
			name:           "namespace found, terminating phase",
			expectNSFound:  true,
			nsPhase:        v1.NamespaceTerminating,
			expectedResult: false,
		},
		{
			name:           "namespace found, deletiontimestamp set",
			expectNSFound:  true,
			nsDeleting:     true,
			expectedResult: false,
		},
		{
			name:           "namespace not found, successfully created",
			expectCreate:   true,
			expectedResult: true,
		},
		{
			name:           "namespace not found initially, create returns already exists error, returned namespace is ready",
			alreadyExists:  true,
			expectedResult: true,
		},
		{
			name:           "namespace not found initially, create returns already exists error, returned namespace is terminating",
			alreadyExists:  true,
			nsPhase:        v1.NamespaceTerminating,
			expectedResult: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			namespace := &v1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test",
				},
			}

			if test.nsPhase != "" {
				namespace.Status.Phase = test.nsPhase
			}

			if test.nsDeleting {
				namespace.SetDeletionTimestamp(&metav1.Time{Time: time.Now()})
			}

			timeout := time.Millisecond

			nsClient := &velerotest.FakeNamespaceClient{}
			defer nsClient.AssertExpectations(t)

			if test.expectNSFound {
				nsClient.On("Get", "test", metav1.GetOptions{}).Return(namespace, nil)
			} else {
				nsClient.On("Get", "test", metav1.GetOptions{}).Return(&v1.Namespace{}, k8serrors.NewNotFound(schema.GroupResource{Resource: "namespaces"}, "test"))
			}

			if test.alreadyExists {
				nsClient.On("Create", namespace).Return(namespace, k8serrors.NewAlreadyExists(schema.GroupResource{Resource: "namespaces"}, "test"))
			}

			if test.expectCreate {
				nsClient.On("Create", namespace).Return(namespace, nil)
			}

			result, _ := EnsureNamespaceExistsAndIsReady(namespace, nsClient, timeout)

			assert.Equal(t, test.expectedResult, result)
		})
	}

}
