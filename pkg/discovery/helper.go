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

package discovery

import (
	"sort"
	"sync"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/restmapper"

	kcmdutil "github.com/heptio/ark/third_party/kubernetes/pkg/kubectl/cmd/util"
)

// Helper exposes functions for interacting with the Kubernetes discovery
// API.
type Helper interface {
	// Resources gets the current set of resources retrieved from discovery
	// that are backuppable by Ark.
	Resources() []*metav1.APIResourceList

	// ResourceFor gets a fully-resolved GroupVersionResource and an
	// APIResource for the provided partially-specified GroupVersionResource.
	ResourceFor(input schema.GroupVersionResource) (schema.GroupVersionResource, metav1.APIResource, error)

	// Refresh pulls an updated set of Ark-backuppable resources from the
	// discovery API.
	Refresh() error

	// APIGroups gets the current set of supported APIGroups
	// in the cluster.
	APIGroups() []metav1.APIGroup
}

type helper struct {
	discoveryClient discovery.DiscoveryInterface
	logger          logrus.FieldLogger

	// lock guards mapper, resources and resourcesMap
	lock         sync.RWMutex
	mapper       meta.RESTMapper
	resources    []*metav1.APIResourceList
	resourcesMap map[schema.GroupVersionResource]metav1.APIResource
	apiGroups    []metav1.APIGroup
}

var _ Helper = &helper{}

func NewHelper(discoveryClient discovery.DiscoveryInterface, logger logrus.FieldLogger) (Helper, error) {
	h := &helper{
		discoveryClient: discoveryClient,
	}
	if err := h.Refresh(); err != nil {
		return nil, err
	}
	return h, nil
}

func (h *helper) ResourceFor(input schema.GroupVersionResource) (schema.GroupVersionResource, metav1.APIResource, error) {
	h.lock.RLock()
	defer h.lock.RUnlock()

	gvr, err := h.mapper.ResourceFor(input)
	if err != nil {
		return schema.GroupVersionResource{}, metav1.APIResource{}, err
	}

	apiResource, found := h.resourcesMap[gvr]
	if !found {
		return schema.GroupVersionResource{}, metav1.APIResource{}, errors.Errorf("APIResource not found for GroupVersionResource %s", gvr)
	}

	return gvr, apiResource, nil
}

func (h *helper) Refresh() error {
	h.lock.Lock()
	defer h.lock.Unlock()

	groupResources, err := restmapper.GetAPIGroupResources(h.discoveryClient)
	if err != nil {
		return errors.WithStack(err)
	}
	mapper := restmapper.NewDiscoveryRESTMapper(groupResources)
	shortcutExpander, err := kcmdutil.NewShortcutExpander(mapper, h.discoveryClient, h.logger)
	if err != nil {
		return errors.WithStack(err)
	}
	h.mapper = shortcutExpander

	preferredResources, err := h.discoveryClient.ServerPreferredResources()
	if err != nil {
		return errors.WithStack(err)
	}

	h.resources = discovery.FilteredBy(
		discovery.ResourcePredicateFunc(filterByVerbs),
		preferredResources,
	)

	sortResources(h.resources)

	h.resourcesMap = make(map[schema.GroupVersionResource]metav1.APIResource)
	for _, resourceGroup := range h.resources {
		gv, err := schema.ParseGroupVersion(resourceGroup.GroupVersion)
		if err != nil {
			return errors.Wrapf(err, "unable to parse GroupVersion %s", resourceGroup.GroupVersion)
		}

		for _, resource := range resourceGroup.APIResources {
			gvr := gv.WithResource(resource.Name)
			h.resourcesMap[gvr] = resource
		}
	}

	apiGroupList, err := h.discoveryClient.ServerGroups()
	if err != nil {
		return errors.WithStack(err)
	}
	h.apiGroups = apiGroupList.Groups

	return nil
}

func filterByVerbs(groupVersion string, r *metav1.APIResource) bool {
	return discovery.SupportsAllVerbs{Verbs: []string{"list", "create", "get", "delete"}}.Match(groupVersion, r)
}

// sortResources sources resources by moving extensions to the end of the slice. The order of all
// the other resources is preserved.
func sortResources(resources []*metav1.APIResourceList) {
	sort.SliceStable(resources, func(i, j int) bool {
		left := resources[i]
		leftGV, _ := schema.ParseGroupVersion(left.GroupVersion)
		// not checking error because it should be impossible to fail to parse data coming from the
		// apiserver
		if leftGV.Group == "extensions" {
			// always sort extensions at the bottom by saying left is "greater"
			return false
		}

		right := resources[j]
		rightGV, _ := schema.ParseGroupVersion(right.GroupVersion)
		// not checking error because it should be impossible to fail to parse data coming from the
		// apiserver
		if rightGV.Group == "extensions" {
			// always sort extensions at the bottom by saying left is "less"
			return true
		}

		return i < j
	})
}

func (h *helper) Resources() []*metav1.APIResourceList {
	h.lock.RLock()
	defer h.lock.RUnlock()
	return h.resources
}

func (h *helper) APIGroups() []metav1.APIGroup {
	h.lock.RLock()
	defer h.lock.RUnlock()
	return h.apiGroups
}
