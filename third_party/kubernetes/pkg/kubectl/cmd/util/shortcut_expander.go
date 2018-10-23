/*
Copyright 2016 The Kubernetes Authors.

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

package util

import (
	"errors"
	"strings"

	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/discovery"
)

// ResourceShortcuts represents a structure that holds the information how to
// transition from resource's shortcut to its full name.
type ResourceShortcuts struct {
	ShortForm schema.GroupResource
	LongForm  schema.GroupResource
}

// shortcutExpander is a RESTMapper that can be used for Kubernetes resources.   It expands the resource first, then invokes the wrapped
type shortcutExpander struct {
	RESTMapper meta.RESTMapper

	discoveryClient discovery.DiscoveryInterface
	logger          logrus.FieldLogger
}

var _ meta.RESTMapper = &shortcutExpander{}

func NewShortcutExpander(delegate meta.RESTMapper, client discovery.DiscoveryInterface, logger logrus.FieldLogger) (shortcutExpander, error) {
	if client == nil {
		return shortcutExpander{}, errors.New("Please provide discovery client to shortcut expander")
	}
	return shortcutExpander{RESTMapper: delegate, discoveryClient: client}, nil
}

func (e shortcutExpander) KindFor(resource schema.GroupVersionResource) (schema.GroupVersionKind, error) {
	return e.RESTMapper.KindFor(e.expandResourceShortcut(resource))
}

func (e shortcutExpander) KindsFor(resource schema.GroupVersionResource) ([]schema.GroupVersionKind, error) {
	return e.RESTMapper.KindsFor(e.expandResourceShortcut(resource))
}

func (e shortcutExpander) ResourcesFor(resource schema.GroupVersionResource) ([]schema.GroupVersionResource, error) {
	return e.RESTMapper.ResourcesFor(e.expandResourceShortcut(resource))
}

func (e shortcutExpander) ResourceFor(resource schema.GroupVersionResource) (schema.GroupVersionResource, error) {
	return e.RESTMapper.ResourceFor(e.expandResourceShortcut(resource))
}

func (e shortcutExpander) ResourceSingularizer(resource string) (string, error) {
	return e.RESTMapper.ResourceSingularizer(e.expandResourceShortcut(schema.GroupVersionResource{Resource: resource}).Resource)
}

func (e shortcutExpander) RESTMapping(gk schema.GroupKind, versions ...string) (*meta.RESTMapping, error) {
	return e.RESTMapper.RESTMapping(gk, versions...)
}

func (e shortcutExpander) RESTMappings(gk schema.GroupKind, versions ...string) ([]*meta.RESTMapping, error) {
	return e.RESTMapper.RESTMappings(gk, versions...)
}

// getShortcutMappings returns a set of tuples which holds short names for resources.
// First the list of potential resources will be taken from the API server.
// Next we will append the hardcoded list of resources - to be backward compatible with old servers.
// NOTE that the list is ordered by group priority.
func (e shortcutExpander) getShortcutMappings() ([]ResourceShortcuts, error) {
	// TODO delete this once discovery is working for svc and netpol
	haveSvc := false
	haveNetpol := false

	res := []ResourceShortcuts{}
	// get server resources
	apiResList, err := e.discoveryClient.ServerResources()
	if err == nil {
		for _, apiResources := range apiResList {
			for _, apiRes := range apiResources.APIResources {
				for _, shortName := range apiRes.ShortNames {
					gv, err := schema.ParseGroupVersion(apiResources.GroupVersion)
					if err != nil {
						e.logger.WithError(err).WithField("groupVersion", apiResources.GroupVersion).Error("Unable to parse groupversion")
						continue
					}
					rs := ResourceShortcuts{
						ShortForm: schema.GroupResource{Group: gv.Group, Resource: shortName},
						LongForm:  schema.GroupResource{Group: gv.Group, Resource: apiRes.Name},
					}
					res = append(res, rs)
					if shortName == "svc" {
						haveSvc = true
					}
					if shortName == "netpol" {
						haveNetpol = true
					}
				}
			}
		}
	}

	// 'svc' is not exposed due to the way the services rest storage handler is installed.
	// 'netpol' is not currently in discovery.
	// add these 2 shortcuts until they're actually in discovery.
	if !haveSvc {
		res = append(res,
			ResourceShortcuts{
				ShortForm: schema.GroupResource{Group: "", Resource: "svc"},
				LongForm:  schema.GroupResource{Group: "", Resource: "services"},
			},
		)
	}
	if !haveNetpol {
		res = append(res,
			ResourceShortcuts{
				ShortForm: schema.GroupResource{Group: "extensions", Resource: "netpol"},
				LongForm:  schema.GroupResource{Group: "extensions", Resource: "networkpolicies"},
			},
		)
	}

	return res, nil
}

// expandResourceShortcut will return the expanded version of resource
// (something that a pkg/api/meta.RESTMapper can understand), if it is
// indeed a shortcut. If no match has been found, we will match on group prefixing.
// Lastly we will return resource unmodified.
func (e shortcutExpander) expandResourceShortcut(resource schema.GroupVersionResource) schema.GroupVersionResource {
	// get the shortcut mappings and return on first match.
	if resources, err := e.getShortcutMappings(); err == nil {
		for _, item := range resources {
			if len(resource.Group) != 0 && resource.Group != item.ShortForm.Group {
				continue
			}
			if resource.Resource == item.ShortForm.Resource {
				resource.Resource = item.LongForm.Resource
				return resource
			}
		}

		// we didn't find exact match so match on group prefixing. This allows autoscal to match autoscaling
		if len(resource.Group) == 0 {
			return resource
		}
		for _, item := range resources {
			if !strings.HasPrefix(item.ShortForm.Group, resource.Group) {
				continue
			}
			if resource.Resource == item.ShortForm.Resource {
				resource.Resource = item.LongForm.Resource
				return resource
			}
		}
	}

	return resource
}
