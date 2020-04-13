/*
Copyright 2017, 2020 the Velero contributors.

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
	"sort"
	"strings"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	kubeerrs "k8s.io/apimachinery/pkg/util/errors"

	"github.com/vmware-tanzu/velero/pkg/client"
	"github.com/vmware-tanzu/velero/pkg/discovery"
	"github.com/vmware-tanzu/velero/pkg/kuberesource"
	"github.com/vmware-tanzu/velero/pkg/util/collections"
)

// resourceBackupper collects resources from the Kubernetes API according to
// the backup spec and passes them to an itemBackupper to be backed up.
type resourceBackupper struct {
	log                   logrus.FieldLogger
	backupRequest         *Request
	discoveryHelper       discovery.Helper
	dynamicFactory        client.DynamicFactory
	cohabitatingResources map[string]*cohabitatingResource
	newItemBackupper      func() ItemBackupper
}

// collect backs up all API groups.
func (r *resourceBackupper) backupAllGroups() {
	for _, group := range r.discoveryHelper.Resources() {
		if err := r.backupGroup(r.log, group); err != nil {
			r.log.WithError(err).WithField("apiGroup", group.String()).Error("Error backing up API group")
		}
	}
}

// backupGroup backs up a single API group.
func (r *resourceBackupper) backupGroup(log logrus.FieldLogger, group *metav1.APIResourceList) error {
	log = log.WithField("group", group.GroupVersion)

	log.Infof("Backing up group")

	// Parse so we can check if this is the core group
	gv, err := schema.ParseGroupVersion(group.GroupVersion)
	if err != nil {
		return errors.Wrapf(err, "error parsing GroupVersion %q", group.GroupVersion)
	}
	if gv.Group == "" {
		// This is the core group, so make sure we process in the following order: pods, pvcs, pvs,
		// everything else.
		sortCoreGroup(group)
	}

	for _, resource := range group.APIResources {
		if err := r.backupResource(log, group, resource); err != nil {
			log.WithError(err).WithField("resource", resource.String()).Error("Error backing up API resource")
		}
	}

	return nil
}

// backupResource backs up all the objects for a given group-version-resource.
func (r *resourceBackupper) backupResource(log logrus.FieldLogger, group *metav1.APIResourceList, resource metav1.APIResource) error {
	log = log.WithField("resource", resource.Name)

	log.Info("Backing up resource")

	gv, err := schema.ParseGroupVersion(group.GroupVersion)
	if err != nil {
		return errors.Wrapf(err, "error parsing GroupVersion %s", group.GroupVersion)
	}
	gr := schema.GroupResource{Group: gv.Group, Resource: resource.Name}

	// Getting the preferred group version of this resource
	preferredGVR, _, err := r.discoveryHelper.ResourceFor(gr.WithVersion(""))
	if err != nil {
		return errors.WithStack(err)
	}

	clusterScoped := !resource.Namespaced

	// If the resource we are backing up is NOT namespaces, and it is cluster-scoped, check to see if
	// we should include it based on the IncludeClusterResources setting.
	if gr != kuberesource.Namespaces && clusterScoped {
		if r.backupRequest.Spec.IncludeClusterResources == nil {
			if !r.backupRequest.NamespaceIncludesExcludes.IncludeEverything() {
				// when IncludeClusterResources == nil (auto), only directly
				// back up cluster-scoped resources if we're doing a full-cluster
				// (all namespaces) backup. Note that in the case of a subset of
				// namespaces being backed up, some related cluster-scoped resources
				// may still be backed up if triggered by a custom action (e.g. PVC->PV).
				// If we're processing namespaces themselves, we will not skip here, they may be
				// filtered out later.
				log.Info("Skipping resource because it's cluster-scoped and only specific namespaces are included in the backup")
				return nil
			}
		} else if !*r.backupRequest.Spec.IncludeClusterResources {
			log.Info("Skipping resource because it's cluster-scoped")
			return nil
		}
	}

	if !r.backupRequest.ResourceIncludesExcludes.ShouldInclude(gr.String()) {
		log.Infof("Skipping resource because it's excluded")
		return nil
	}

	if cohabitator, found := r.cohabitatingResources[resource.Name]; found {
		if cohabitator.seen {
			log.WithFields(
				logrus.Fields{
					"cohabitatingResource1": cohabitator.groupResource1.String(),
					"cohabitatingResource2": cohabitator.groupResource2.String(),
				},
			).Infof("Skipping resource because it cohabitates and we've already processed it")
			return nil
		}
		cohabitator.seen = true
	}

	itemBackupper := r.newItemBackupper()

	namespacesToList := getNamespacesToList(r.backupRequest.NamespaceIncludesExcludes)

	// Check if we're backing up namespaces, and only certain ones
	if gr == kuberesource.Namespaces && namespacesToList[0] != "" {
		resourceClient, err := r.dynamicFactory.ClientForGroupVersionResource(gv, resource, "")
		if err != nil {
			log.WithError(err).Error("Error getting dynamic client")
		} else {
			var labelSelector labels.Selector
			if r.backupRequest.Spec.LabelSelector != nil {
				labelSelector, err = metav1.LabelSelectorAsSelector(r.backupRequest.Spec.LabelSelector)
				if err != nil {
					// This should never happen...
					return errors.Wrap(err, "invalid label selector")
				}
			}

			for _, ns := range namespacesToList {
				log = log.WithField("namespace", ns)
				log.Info("Getting namespace")
				unstructured, err := resourceClient.Get(ns, metav1.GetOptions{})
				if err != nil {
					log.WithError(errors.WithStack(err)).Error("Error getting namespace")
					continue
				}

				labels := labels.Set(unstructured.GetLabels())
				if labelSelector != nil && !labelSelector.Matches(labels) {
					log.Info("Skipping namespace because it does not match the backup's label selector")
					continue
				}

				if _, err := itemBackupper.backupItem(log, unstructured, gr, preferredGVR); err != nil {
					log.WithError(errors.WithStack(err)).Error("Error backing up namespace")
				}
			}

			return nil
		}
	}

	// If we get here, we're backing up something other than namespaces
	if clusterScoped {
		namespacesToList = []string{""}
	}

	backedUpItem := false
	for _, namespace := range namespacesToList {
		log = log.WithField("namespace", namespace)

		resourceClient, err := r.dynamicFactory.ClientForGroupVersionResource(gv, resource, namespace)
		if err != nil {
			log.WithError(err).Error("Error getting dynamic client")
			continue
		}

		var labelSelector string
		if selector := r.backupRequest.Spec.LabelSelector; selector != nil {
			labelSelector = metav1.FormatLabelSelector(selector)
		}

		log.Info("Listing items")
		unstructuredList, err := resourceClient.List(metav1.ListOptions{LabelSelector: labelSelector})
		if err != nil {
			log.WithError(errors.WithStack(err)).Error("Error listing items")
			continue
		}
		log.Infof("Retrieved %d items", len(unstructuredList.Items))

		// do the backup
		for _, item := range unstructuredList.Items {
			if r.backupItem(log, gr, itemBackupper, &item, preferredGVR) {
				backedUpItem = true
			}
		}
	}

	// back up CRD for resource if found. We should only need to do this if we've backed up at least
	// one item and IncludeClusterResources is nil. If IncludeClusterResources is false
	// we don't want to back it up, and if it's true it will already be included.
	if backedUpItem && r.backupRequest.Spec.IncludeClusterResources == nil {
		r.backupCRD(log, gr, itemBackupper)
	}

	return nil
}

func (r *resourceBackupper) backupItem(
	log logrus.FieldLogger,
	gr schema.GroupResource,
	itemBackupper ItemBackupper,
	unstructured runtime.Unstructured,
	preferredGVR schema.GroupVersionResource,
) bool {
	metadata, err := meta.Accessor(unstructured)
	if err != nil {
		log.WithError(errors.WithStack(err)).Error("Error getting a metadata accessor")
		return false
	}

	log = log.WithFields(map[string]interface{}{
		"namespace": metadata.GetNamespace(),
		"name":      metadata.GetName(),
	})

	if gr == kuberesource.Namespaces && !r.backupRequest.NamespaceIncludesExcludes.ShouldInclude(metadata.GetName()) {
		log.Info("Skipping namespace because it's excluded")
		return false
	}

	backedUpItem, err := itemBackupper.backupItem(log, unstructured, gr, preferredGVR)
	if aggregate, ok := err.(kubeerrs.Aggregate); ok {
		log.Infof("%d errors encountered backup up item", len(aggregate.Errors()))
		// log each error separately so we get error location info in the log, and an
		// accurate count of errors
		for _, err = range aggregate.Errors() {
			log.WithError(err).Error("Error backing up item")
		}

		return false
	}
	if err != nil {
		log.WithError(err).Error("Error backing up item")
		return false
	}
	return backedUpItem
}

// backupCRD checks if the resource is a custom resource, and if so, backs up the custom resource definition
// associated with it.
func (r *resourceBackupper) backupCRD(log logrus.FieldLogger, gr schema.GroupResource, itemBackupper ItemBackupper) {
	crdGroupResource := kuberesource.CustomResourceDefinitions

	log.Debugf("Getting server preferred API version for %s", crdGroupResource)
	gvr, apiResource, err := r.discoveryHelper.ResourceFor(crdGroupResource.WithVersion(""))
	if err != nil {
		log.WithError(errors.WithStack(err)).Errorf("Error getting resolved resource for %s", crdGroupResource)
		return
	}
	log.Debugf("Got server preferred API version %s for %s", gvr.Version, crdGroupResource)

	log.Debugf("Getting dynamic client for %s", gvr.String())
	crdClient, err := r.dynamicFactory.ClientForGroupVersionResource(gvr.GroupVersion(), apiResource, "")
	if err != nil {
		log.WithError(errors.WithStack(err)).Errorf("Error getting dynamic client for %s", crdGroupResource)
		return
	}
	log.Debugf("Got dynamic client for %s", gvr.String())

	// try to get a CRD whose name matches the provided GroupResource
	unstructured, err := crdClient.Get(gr.String(), metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		// not found: this means the GroupResource provided was not a
		// custom resource, so there's no CRD to back up.
		log.Debugf("No CRD found for GroupResource %s", gr.String())
		return
	}
	if err != nil {
		log.WithError(errors.WithStack(err)).Errorf("Error getting CRD %s", gr.String())
		return
	}
	log.Infof("Found associated CRD %s to add to backup", gr.String())

	r.backupItem(log, gvr.GroupResource(), itemBackupper, unstructured, gvr)
}

// sortCoreGroup sorts the core API group.
func sortCoreGroup(group *metav1.APIResourceList) {
	sort.SliceStable(group.APIResources, func(i, j int) bool {
		return coreGroupResourcePriority(group.APIResources[i].Name) < coreGroupResourcePriority(group.APIResources[j].Name)
	})
}

// These constants represent the relative priorities for resources in the core API group. We want to
// ensure that we process pods, then pvcs, then pvs, then anything else. This ensures that when a
// pod is backed up, we can perform a pre hook, then process pvcs and pvs (including taking a
// snapshot), then perform a post hook on the pod.
const (
	pod = iota
	pvc
	pv
	other
)

// coreGroupResourcePriority returns the relative priority of the resource, in the following order:
// pods, pvcs, pvs, everything else.
func coreGroupResourcePriority(resource string) int {
	switch strings.ToLower(resource) {
	case "pods":
		return pod
	case "persistentvolumeclaims":
		return pvc
	case "persistentvolumes":
		return pv
	}

	return other
}

// getNamespacesToList examines ie and resolves the includes and excludes to a full list of
// namespaces to list. If ie is nil or it includes *, the result is just "" (list across all
// namespaces). Otherwise, the result is a list of every included namespace minus all excluded ones.
func getNamespacesToList(ie *collections.IncludesExcludes) []string {
	if ie == nil {
		return []string{""}
	}

	if ie.ShouldInclude("*") {
		// "" means all namespaces
		return []string{""}
	}

	var list []string
	for _, i := range ie.GetIncludes() {
		if ie.ShouldInclude(i) {
			list = append(list, i)
		}
	}

	return list
}

type cohabitatingResource struct {
	resource       string
	groupResource1 schema.GroupResource
	groupResource2 schema.GroupResource
	seen           bool
}

func newCohabitatingResource(resource, group1, group2 string) *cohabitatingResource {
	return &cohabitatingResource{
		resource:       resource,
		groupResource1: schema.GroupResource{Group: group1, Resource: resource},
		groupResource2: schema.GroupResource{Group: group2, Resource: resource},
		seen:           false,
	}
}
