/*
Copyright the Velero contributors.

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

// Code generated by lister-gen. DO NOT EDIT.

package v1

import (
	v1 "github.com/reynencourt/velero/pkg/apis/velero/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/tools/cache"
)

// DeleteBackupRequestLister helps list DeleteBackupRequests.
type DeleteBackupRequestLister interface {
	// List lists all DeleteBackupRequests in the indexer.
	List(selector labels.Selector) (ret []*v1.DeleteBackupRequest, err error)
	// DeleteBackupRequests returns an object that can list and get DeleteBackupRequests.
	DeleteBackupRequests(namespace string) DeleteBackupRequestNamespaceLister
	DeleteBackupRequestListerExpansion
}

// deleteBackupRequestLister implements the DeleteBackupRequestLister interface.
type deleteBackupRequestLister struct {
	indexer cache.Indexer
}

// NewDeleteBackupRequestLister returns a new DeleteBackupRequestLister.
func NewDeleteBackupRequestLister(indexer cache.Indexer) DeleteBackupRequestLister {
	return &deleteBackupRequestLister{indexer: indexer}
}

// List lists all DeleteBackupRequests in the indexer.
func (s *deleteBackupRequestLister) List(selector labels.Selector) (ret []*v1.DeleteBackupRequest, err error) {
	err = cache.ListAll(s.indexer, selector, func(m interface{}) {
		ret = append(ret, m.(*v1.DeleteBackupRequest))
	})
	return ret, err
}

// DeleteBackupRequests returns an object that can list and get DeleteBackupRequests.
func (s *deleteBackupRequestLister) DeleteBackupRequests(namespace string) DeleteBackupRequestNamespaceLister {
	return deleteBackupRequestNamespaceLister{indexer: s.indexer, namespace: namespace}
}

// DeleteBackupRequestNamespaceLister helps list and get DeleteBackupRequests.
type DeleteBackupRequestNamespaceLister interface {
	// List lists all DeleteBackupRequests in the indexer for a given namespace.
	List(selector labels.Selector) (ret []*v1.DeleteBackupRequest, err error)
	// Get retrieves the DeleteBackupRequest from the indexer for a given namespace and name.
	Get(name string) (*v1.DeleteBackupRequest, error)
	DeleteBackupRequestNamespaceListerExpansion
}

// deleteBackupRequestNamespaceLister implements the DeleteBackupRequestNamespaceLister
// interface.
type deleteBackupRequestNamespaceLister struct {
	indexer   cache.Indexer
	namespace string
}

// List lists all DeleteBackupRequests in the indexer for a given namespace.
func (s deleteBackupRequestNamespaceLister) List(selector labels.Selector) (ret []*v1.DeleteBackupRequest, err error) {
	err = cache.ListAllByNamespace(s.indexer, s.namespace, selector, func(m interface{}) {
		ret = append(ret, m.(*v1.DeleteBackupRequest))
	})
	return ret, err
}

// Get retrieves the DeleteBackupRequest from the indexer for a given namespace and name.
func (s deleteBackupRequestNamespaceLister) Get(name string) (*v1.DeleteBackupRequest, error) {
	obj, exists, err := s.indexer.GetByKey(s.namespace + "/" + name)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, errors.NewNotFound(v1.Resource("deletebackuprequest"), name)
	}
	return obj.(*v1.DeleteBackupRequest), nil
}
