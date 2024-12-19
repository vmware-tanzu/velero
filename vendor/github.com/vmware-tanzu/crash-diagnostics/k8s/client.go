// Copyright (c) 2019 VMware, Inc. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package k8s

import (
	"context"
	"fmt"
	"strings"

	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/cli-runtime/pkg/printers"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/restmapper"
	"k8s.io/client-go/tools/clientcmd"
)

const (
	LegacyGroupName = "core"
)

// Client prepares and exposes a dynamic, discovery, and Rest clients
type Client struct {
	Client      dynamic.Interface
	Disco       discovery.DiscoveryInterface
	CoreRest    rest.Interface
	Mapper      meta.RESTMapper
	JsonPrinter printers.JSONPrinter
}

// New returns a *Client built with the kubecontext file path
// and an optional (at most one) K8s CLI context name.
func New(kubeconfig string, clusterContextOptions ...string) (*Client, error) {
	var clusterCtxName string
	if len(clusterContextOptions) > 0 {
		clusterCtxName = clusterContextOptions[0]
	}

	// creating cfg for each client type because each
	// setup needs its own cfg default which may not be compatible
	dynCfg, err := makeRESTConfig(kubeconfig, clusterCtxName)
	if err != nil {
		return nil, err
	}
	client, err := dynamic.NewForConfig(dynCfg)
	if err != nil {
		return nil, err
	}

	discoCfg, err := makeRESTConfig(kubeconfig, clusterCtxName)
	if err != nil {
		return nil, err
	}
	disco, err := discovery.NewDiscoveryClientForConfig(discoCfg)
	if err != nil {
		return nil, err
	}

	resources, err := restmapper.GetAPIGroupResources(disco)
	if err != nil {
		return nil, err
	}
	mapper := restmapper.NewDiscoveryRESTMapper(resources)

	restCfg, err := makeRESTConfig(kubeconfig, clusterCtxName)
	if err != nil {
		return nil, err
	}
	setCoreDefaultConfig(restCfg)
	restc, err := rest.RESTClientFor(restCfg)
	if err != nil {
		return nil, err
	}

	return &Client{Client: client, Disco: disco, CoreRest: restc, Mapper: mapper}, nil
}

// makeRESTConfig creates a new *rest.Config with a k8s context name if one is provided.
func makeRESTConfig(fileName, contextName string) (*rest.Config, error) {
	if fileName == "" {
		return nil, fmt.Errorf("kubeconfig file path required")
	}

	if contextName != "" {
		// create the config object from k8s config path and context
		return clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
			&clientcmd.ClientConfigLoadingRules{ExplicitPath: fileName},
			&clientcmd.ConfigOverrides{
				CurrentContext: contextName,
			}).ClientConfig()
	}

	return clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		&clientcmd.ClientConfigLoadingRules{ExplicitPath: fileName},
		&clientcmd.ConfigOverrides{},
	).ClientConfig()
}

func (k8sc *Client) Search(ctx context.Context, params SearchParams) ([]SearchResult, error) {
	return k8sc._search(ctx, strings.Join(params.Groups, " "),
		strings.Join(params.Categories, " "),
		strings.Join(params.Kinds, " "),
		strings.Join(params.Namespaces, " "),
		strings.Join(params.Versions, " "),
		strings.Join(params.Names, " "),
		strings.Join(params.Labels, " "),
		strings.Join(params.Containers, " "))
}

// Search does a drill-down search from group, version, resourceList, to resources.  The following rules are applied
// 1) Legacy core group can be specified as "core" instead of empty string.
// 2) All specified search params will use AND operator for match (i.e. groups=core AND kinds=pods AND versions=v1 AND ... etc)
// 3) kinds (resources) will match resource.Kind or resource.Name
// 4) All search params are passed as comma- or space-separated sets that are matched using OR (i.e. kinds=pods services
//    will match resouces of type pods or services)
func (k8sc *Client) _search(ctx context.Context, groups, categories, kinds, namespaces, versions, names, labels, containers string) ([]SearchResult, error) {

	// normalize params
	groups = strings.ToLower(groups)
	categories = strings.ToLower(categories)
	kinds = strings.ToLower(kinds) // means resources in K8s API term (i.e. pods, services, etc)
	namespaces = strings.ToLower(namespaces)
	versions = strings.ToLower(versions)
	labels = strings.ToLower(labels)
	containers = strings.ToLower(containers)

	logrus.Debugf(
		"Search filters groups:[%v]; categories:[%v]; kinds:[%v]; namespaces:[%v]; versions:[%v]; names:[%v]; labels:[%v] containers:[%s]",
		groups, categories, kinds, namespaces, versions, names, labels, containers,
	)

	// Build a groups-resource Map that maps each
	// selected group to its associated resources.
	groupResMap := make(map[schema.GroupVersion]*metav1.APIResourceList)
	switch {
	case groups == "" && kinds == "" && versions == "" && categories == "":
		// no groups, no kinds (resources), no versions, no categories provided
		return nil, fmt.Errorf("search: at least one of {groups, kinds, versions, or categories} is required")
	case groups == "" && kinds == "" && versions != "" && categories == "":
		// only versions provided
		return nil, fmt.Errorf("search: versions must be provided with at least one of {groups, kinds, or categories}")
	default:
		// build a group-to-resources map, based on the passed parameters.
		// first, extract groups needed to build the map
		var groupList *metav1.APIGroupList
		if groups != "" {
			groupList = &metav1.APIGroupList{}
			groupSlice := splitParamList(groups)

			// adjust for legacy group name "core" -> "" empty
			for i := 0; i < len(groupSlice); i++ {
				groupSlice[i] = toLegacyGrpName(groupSlice[i])
			}

			serverGroups, err := k8sc.Disco.ServerGroups()
			if err != nil {
				return nil, fmt.Errorf("search: failed to get server groups: %w", err)
			}
			// for each server group, match specified group name from param
			for _, grp := range serverGroups.Groups {
				if sliceContains(groupSlice, grp.Name) {
					groupList.Groups = append(groupList.Groups, grp)
				}
			}
		} else {
			serverGroups, err := k8sc.Disco.ServerGroups()
			if err != nil {
				return nil, fmt.Errorf("search: failed to get server groups: %w", err)
			}
			groupList = serverGroups
		}

		// extract resources names (kinds param) and versions params
		verSlice := splitParamList(versions)
		resSlice := splitParamList(kinds)
		catSlice := splitParamList(categories)

		// next, for each groupVersion pair
		// retrieve a set of resources associated with it
		for _, grp := range groupList.Groups {
			for _, ver := range grp.Versions {
				// only select ver if it can be matched, otherwise continue to next ver
				if versions != "" && !sliceContains(verSlice, ver.Version) {
					continue
				}

				// grab all available resources for group/ver
				groupVersion := schema.GroupVersion{Group: grp.Name, Version: ver.Version}
				resList, err := k8sc.Disco.ServerResourcesForGroupVersion(groupVersion.String())
				if err != nil {
					return nil, fmt.Errorf("search: failed to get resources for groups: %w", err)
				}

				// for each resource in group/ver
				// attempt to match it with provided resources name (kinds)
				resultList := &metav1.APIResourceList{GroupVersion: groupVersion.String()}
				for _, resource := range resList.APIResources {
					// filter resources on names if provided (kinds param)
					if kinds != "" && !sliceContains(resSlice, resource.Kind) && !sliceContains(resSlice, resource.Name) {
						continue
					}
					// filter resources on categories if specified
					if categories != "" && !sliceContains(catSlice, resource.Categories...) {
						continue
					}
					resultList.APIResources = append(resultList.APIResources, resource)
				}
				groupResMap[groupVersion] = resultList
			}
		}
	}

	// prepare namespaces
	var nsList []string
	if namespaces != "" {
		nsList = splitParamList(namespaces)
	} else {
		nsNames, err := getNamespaces(ctx, k8sc)
		if err != nil {
			return nil, err
		}
		nsList = nsNames
	}

	// Collect resource objects using the grou-to-resources map
	var finalResults []SearchResult
	logrus.Debugf("searching through %d groups", len(groupResMap))
	for groupVer, resourceList := range groupResMap {
		for _, resource := range resourceList.APIResources {
			listOptions := metav1.ListOptions{
				LabelSelector: labels,
			}
			gvr := schema.GroupVersionResource{Group: groupVer.Group, Version: groupVer.Version, Resource: resource.Name}
			// gather found resources
			var results []SearchResult
			if resource.Namespaced {
				for _, ns := range nsList {
					logrus.Debugf("searching for %s objects in [group=%s; namespace=%s; labels=%v]",
						resource.Name, groupVer, ns, listOptions.LabelSelector,
					)
					list, err := k8sc.Client.Resource(gvr).Namespace(ns).List(ctx, listOptions)
					if err != nil {
						logrus.Debugf(
							"WARN: failed to get %s objects in [group=%s; namespace=%s; labels=%v]: %s",
							resource.Name, groupVer, ns, listOptions.LabelSelector, err,
						)
						continue
					}
					if len(list.Items) == 0 {
						logrus.Debugf(
							"WARN: found 0 %s in [group=%s; namespace=%s; labels=%v]",
							resource.Name, groupVer, ns, listOptions.LabelSelector,
						)
						continue
					}

					logrus.Debugf("found %d %s in [group=%s; namespace=%s; labels=%v]",
						len(list.Items), resource.Name, groupVer, ns, listOptions.LabelSelector,
					)
					result := SearchResult{
						ListKind:             list.GetKind(),
						ResourceName:         resource.Name,
						ResourceKind:         resource.Kind,
						Namespaced:           resource.Namespaced,
						Namespace:            ns,
						GroupVersionResource: gvr,
						List:                 list,
					}
					results = append(results, result)
				}
			} else {
				logrus.Debugf("searching for %s objects in [group=%s; non-namespced; labels=%v]",
					resource.Name, groupVer, listOptions.LabelSelector,
				)

				list, err := k8sc.Client.Resource(gvr).List(ctx, listOptions)
				if err != nil {
					logrus.Debugf(
						"WARN: failed to get %s objects in [group=%s; non-namespaced; labels=%v]: %s",
						resource.Name, groupVer, listOptions.LabelSelector, err,
					)
					continue
				}
				if len(list.Items) == 0 {
					logrus.Debugf(
						"WARN: found 0 %s in [group=%s; non-namespaced; labels=%v]",
						resource.Name, groupVer, listOptions.LabelSelector,
					)
					continue
				}

				logrus.Debugf("found %d %s in [group=%s; non-namespaced; labels=%v]",
					len(list.Items), resource.Name, groupVer, listOptions.LabelSelector,
				)

				result := SearchResult{
					ListKind:             list.GetKind(),
					ResourceKind:         resource.Kind,
					ResourceName:         resource.Name,
					Namespaced:           resource.Namespaced,
					GroupVersionResource: gvr,
					List:                 list,
				}
				results = append(results, result)
			}

			// apply name filters
			logrus.Debugf("applying filters on %d results", len(results))
			for _, result := range results {
				filteredResult := result
				if len(containers) > 0 && result.ListKind == "PodList" {
					filteredResult = filterPodsByContainers(result, containers)
					logrus.Debugf("found %d %s with container filter [%s]", len(filteredResult.List.Items), filteredResult.ResourceName, containers)
				}
				if len(names) > 0 {
					filteredResult = filterByNames(result, names)
					logrus.Debugf("found %d %s with name filter [%s]", len(filteredResult.List.Items), filteredResult.ResourceName, names)
				}
				finalResults = append(finalResults, filteredResult)
			}
		}
	}

	return finalResults, nil
}

func setCoreDefaultConfig(config *rest.Config) {
	config.GroupVersion = &corev1.SchemeGroupVersion
	config.APIPath = "/api"
	config.NegotiatedSerializer = scheme.Codecs.WithoutConversion()

	if config.UserAgent == "" {
		config.UserAgent = rest.DefaultKubernetesUserAgent()
	}
}

func toLegacyGrpName(str string) string {
	if strings.EqualFold(str, "core") {
		return ""
	}
	return str
}

func splitParamList(nses string) []string {
	if nses == "" {
		return []string{}
	}
	if strings.Contains(nses, ",") {
		return strings.Split(nses, ",")
	}
	return strings.Split(nses, " ")
}

func filterByNames(result SearchResult, names string) SearchResult {
	if len(names) == 0 {
		return result
	}
	var filteredItems []unstructured.Unstructured
	for _, item := range result.List.Items {
		if len(names) > 0 && !strings.Contains(names, item.GetName()) {
			continue
		}
		filteredItems = append(filteredItems, item)
	}
	result.List.Items = filteredItems
	return result
}

func filterPodsByContainers(result SearchResult, containers string) SearchResult {
	if result.ListKind != "PodList" {
		return result
	}
	var filteredItems []unstructured.Unstructured
	for _, podItem := range result.List.Items {
		containerItems := getPodContainers(podItem)
		for _, containerItem := range containerItems {
			containerObj, ok := containerItem.(map[string]interface{})
			if !ok {
				logrus.Errorf("Failed to assert ustructured item (type %T) as Container", containerItem)
				continue
			}
			name, ok, err := unstructured.NestedString(containerObj, "name")
			if !ok || err != nil {
				logrus.Errorf("Failed to get container object name: %s", err)
				continue
			}
			if len(containers) > 0 && !strings.Contains(containers, name) {
				logrus.Debugf("Container %s not found in filter list %s", name, containers)
				continue
			}
			filteredItems = append(filteredItems, podItem)
		}

	}

	result.List.Items = filteredItems
	return result
}

// getPodContainers collect and return init-containers, containers, ephemeral containers from pod item
func getPodContainers(podItem unstructured.Unstructured) []interface{} {
	var result []interface{}

	initContainers, ok, err := unstructured.NestedSlice(podItem.Object, "spec", "initContainers")
	if err != nil {
		logrus.Errorf("Failed to get init-containers for pod %s: %s", podItem.GetName(), err)
	}
	if ok {
		result = append(result, initContainers...)
	}

	containers, ok, err := unstructured.NestedSlice(podItem.Object, "spec", "containers")
	if err != nil {
		logrus.Errorf("Failed to get containers for pod %s: %s", podItem.GetName(), err)
	}
	if ok {
		result = append(result, containers...)
	}

	return result
}

// getNamespaces collect all available namespaces in cluster
func getNamespaces(ctx context.Context, k8sc *Client) ([]string, error) {
	gvr := schema.GroupVersionResource{
		Group:    "",
		Version:  "v1",
		Resource: "namespaces",
	}
	objList, err := k8sc.Client.Resource(gvr).List(ctx, metav1.ListOptions{})

	if err != nil {
		return nil, err
	}

	names := make([]string, len(objList.Items))
	for i, obj := range objList.Items {
		names[i] = obj.GetName()
	}

	return names, nil
}

// sliceContains check if atleast one of the values is present in the slice
func sliceContains(slice []string, values ...string) bool {
	for _, s := range slice {
		for _, v := range values {
			if strings.EqualFold(strings.TrimSpace(s), strings.TrimSpace(v)) {
				return true
			}
		}
	}

	return false
}
