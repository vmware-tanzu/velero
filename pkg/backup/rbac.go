/*
Copyright 2018 the Velero contributors.

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
	"context"

	"github.com/pkg/errors"
	rbac "k8s.io/api/rbac/v1"
	rbacbeta "k8s.io/api/rbac/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	rbacclient "k8s.io/client-go/kubernetes/typed/rbac/v1"
	rbacbetaclient "k8s.io/client-go/kubernetes/typed/rbac/v1beta1"
)

// ClusterRoleBindingLister allows for listing ClusterRoleBindings in a version-independent way.
type ClusterRoleBindingLister interface {
	// List returns a slice of ClusterRoleBindings which can represent either v1 or v1beta1 ClusterRoleBindings.
	List() ([]ClusterRoleBinding, error)
}

// noopClusterRoleBindingLister exists to handle clusters where RBAC is disabled.
type noopClusterRoleBindingLister struct {
}

func (noop noopClusterRoleBindingLister) List() ([]ClusterRoleBinding, error) {
	return []ClusterRoleBinding{}, nil
}

type v1ClusterRoleBindingLister struct {
	client rbacclient.ClusterRoleBindingInterface
}

func (v1 v1ClusterRoleBindingLister) List() ([]ClusterRoleBinding, error) {
	crbList, err := v1.client.List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return nil, errors.WithStack(err)
	}
	var crbs []ClusterRoleBinding
	for _, crb := range crbList.Items {
		crbs = append(crbs, v1ClusterRoleBinding{crb: crb})
	}

	return crbs, nil
}

type v1beta1ClusterRoleBindingLister struct {
	client rbacbetaclient.ClusterRoleBindingInterface
}

func (v1beta1 v1beta1ClusterRoleBindingLister) List() ([]ClusterRoleBinding, error) {
	crbList, err := v1beta1.client.List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return nil, errors.WithStack(err)
	}
	var crbs []ClusterRoleBinding
	for _, crb := range crbList.Items {
		crbs = append(crbs, v1beta1ClusterRoleBinding{crb: crb})
	}

	return crbs, nil
}

// NewClusterRoleBindingListerMap creates a map of RBAC version strings to their associated
// ClusterRoleBindingLister structs.
// Necessary so that callers to the ClusterRoleBindingLister interfaces don't need the kubernetes.Interface.
func NewClusterRoleBindingListerMap(clientset kubernetes.Interface) map[string]ClusterRoleBindingLister {
	return map[string]ClusterRoleBindingLister{
		rbac.SchemeGroupVersion.Version:     v1ClusterRoleBindingLister{client: clientset.RbacV1().ClusterRoleBindings()},
		rbacbeta.SchemeGroupVersion.Version: v1beta1ClusterRoleBindingLister{client: clientset.RbacV1beta1().ClusterRoleBindings()},
		"":                                  noopClusterRoleBindingLister{},
	}
}

// ClusterRoleBinding abstracts access to ClusterRoleBindings whether they're v1 or v1beta1.
type ClusterRoleBinding interface {
	// Name returns the name of a ClusterRoleBinding.
	Name() string
	// ServiceAccountSubjects returns the names of subjects that are service accounts in the given namespace.
	ServiceAccountSubjects(namespace string) []string
	// RoleRefName returns the name of a ClusterRoleBinding's RoleRef.
	RoleRefName() string
}

type v1ClusterRoleBinding struct {
	crb rbac.ClusterRoleBinding
}

func (c v1ClusterRoleBinding) Name() string {
	return c.crb.Name
}

func (c v1ClusterRoleBinding) RoleRefName() string {
	return c.crb.RoleRef.Name
}

func (c v1ClusterRoleBinding) ServiceAccountSubjects(namespace string) []string {
	var saSubjects []string
	for _, s := range c.crb.Subjects {
		if s.Kind == rbac.ServiceAccountKind && s.Namespace == namespace {
			saSubjects = append(saSubjects, s.Name)
		}
	}
	return saSubjects
}

type v1beta1ClusterRoleBinding struct {
	crb rbacbeta.ClusterRoleBinding
}

func (c v1beta1ClusterRoleBinding) Name() string {
	return c.crb.Name
}

func (c v1beta1ClusterRoleBinding) RoleRefName() string {
	return c.crb.RoleRef.Name
}

func (c v1beta1ClusterRoleBinding) ServiceAccountSubjects(namespace string) []string {
	var saSubjects []string
	for _, s := range c.crb.Subjects {
		if s.Kind == rbac.ServiceAccountKind && s.Namespace == namespace {
			saSubjects = append(saSubjects, s.Name)
		}
	}
	return saSubjects
}
