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
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	kuberrs "k8s.io/apimachinery/pkg/util/errors"

	"github.com/heptio/ark/pkg/client"
	"github.com/heptio/ark/pkg/discovery"
	"github.com/heptio/ark/pkg/kuberesource"
	"github.com/heptio/ark/pkg/podexec"
	"github.com/heptio/ark/pkg/restic"
	"github.com/heptio/ark/pkg/util/collections"
)

type resourceBackupperFactory interface {
	newResourceBackupper(
		log logrus.FieldLogger,
		backupRequest *Request,
		dynamicFactory client.DynamicFactory,
		discoveryHelper discovery.Helper,
		backedUpItems map[itemKey]struct{},
		cohabitatingResources map[string]*cohabitatingResource,
		podCommandExecutor podexec.PodCommandExecutor,
		tarWriter tarWriter,
		resticBackupper restic.Backupper,
		resticSnapshotTracker *pvcSnapshotTracker,
		blockStoreGetter BlockStoreGetter,
	) resourceBackupper
}

type defaultResourceBackupperFactory struct{}

func (f *defaultResourceBackupperFactory) newResourceBackupper(
	log logrus.FieldLogger,
	backupRequest *Request,
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
	return &defaultResourceBackupper{
		log:                   log,
		backupRequest:         backupRequest,
		dynamicFactory:        dynamicFactory,
		discoveryHelper:       discoveryHelper,
		backedUpItems:         backedUpItems,
		cohabitatingResources: cohabitatingResources,
		podCommandExecutor:    podCommandExecutor,
		tarWriter:             tarWriter,
		resticBackupper:       resticBackupper,
		resticSnapshotTracker: resticSnapshotTracker,
		blockStoreGetter:      blockStoreGetter,

		itemBackupperFactory: &defaultItemBackupperFactory{},
	}
}

type resourceBackupper interface {
	backupResource(group *metav1.APIResourceList, resource metav1.APIResource) error
}

type defaultResourceBackupper struct {
	log                   logrus.FieldLogger
	backupRequest         *Request
	dynamicFactory        client.DynamicFactory
	discoveryHelper       discovery.Helper
	backedUpItems         map[itemKey]struct{}
	cohabitatingResources map[string]*cohabitatingResource
	podCommandExecutor    podexec.PodCommandExecutor
	tarWriter             tarWriter
	resticBackupper       restic.Backupper
	resticSnapshotTracker *pvcSnapshotTracker
	itemBackupperFactory  itemBackupperFactory
	blockStoreGetter      BlockStoreGetter
}

// backupResource backs up all the objects for a given group-version-resource.
func (rb *defaultResourceBackupper) backupResource(
	group *metav1.APIResourceList,
	resource metav1.APIResource,
) error {
	var errs []error

	gv, err := schema.ParseGroupVersion(group.GroupVersion)
	if err != nil {
		return errors.Wrapf(err, "error parsing GroupVersion %s", group.GroupVersion)
	}
	gr := schema.GroupResource{Group: gv.Group, Resource: resource.Name}
	grString := gr.String()

	log := rb.log.WithField("groupResource", grString)

	log.Info("Evaluating resource")

	clusterScoped := !resource.Namespaced

	// If the resource we are backing up is NOT namespaces, and it is cluster-scoped, check to see if
	// we should include it based on the IncludeClusterResources setting.
	if gr != kuberesource.Namespaces && clusterScoped {
		if rb.backupRequest.Spec.IncludeClusterResources == nil {
			if !rb.backupRequest.NamespaceIncludesExcludes.IncludeEverything() {
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
		} else if !*rb.backupRequest.Spec.IncludeClusterResources {
			log.Info("Skipping resource because it's cluster-scoped")
			return nil
		}
	}

	if !rb.backupRequest.ResourceIncludesExcludes.ShouldInclude(grString) {
		log.Infof("Resource is excluded")
		return nil
	}

	if cohabitator, found := rb.cohabitatingResources[resource.Name]; found {
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

	itemBackupper := rb.itemBackupperFactory.newItemBackupper(
		rb.backupRequest,
		rb.backedUpItems,
		rb.podCommandExecutor,
		rb.tarWriter,
		rb.dynamicFactory,
		rb.discoveryHelper,
		rb.resticBackupper,
		rb.resticSnapshotTracker,
		rb.blockStoreGetter,
	)

	namespacesToList := getNamespacesToList(rb.backupRequest.NamespaceIncludesExcludes)

	// Check if we're backing up namespaces, and only certain ones
	if gr == kuberesource.Namespaces && namespacesToList[0] != "" {
		resourceClient, err := rb.dynamicFactory.ClientForGroupVersionResource(gv, resource, "")
		if err != nil {
			return err
		}

		var labelSelector labels.Selector
		if rb.backupRequest.Spec.LabelSelector != nil {
			labelSelector, err = metav1.LabelSelectorAsSelector(rb.backupRequest.Spec.LabelSelector)
			if err != nil {
				// This should never happen...
				return errors.Wrap(err, "invalid label selector")
			}
		}

		for _, ns := range namespacesToList {
			log.WithField("namespace", ns).Info("Getting namespace")
			unstructured, err := resourceClient.Get(ns, metav1.GetOptions{})
			if err != nil {
				errs = append(errs, errors.Wrap(err, "error getting namespace"))
				continue
			}

			labels := labels.Set(unstructured.GetLabels())
			if labelSelector != nil && !labelSelector.Matches(labels) {
				log.WithField("name", unstructured.GetName()).Info("skipping item because it does not match the backup's label selector")
				continue
			}

			if err := itemBackupper.backupItem(log, unstructured, gr); err != nil {
				errs = append(errs, err)
			}
		}

		return kuberrs.NewAggregate(errs)
	}

	// If we get here, we're backing up something other than namespaces
	if clusterScoped {
		namespacesToList = []string{""}
	}

	for _, namespace := range namespacesToList {
		resourceClient, err := rb.dynamicFactory.ClientForGroupVersionResource(gv, resource, namespace)
		if err != nil {
			return err
		}

		var labelSelector string
		if selector := rb.backupRequest.Spec.LabelSelector; selector != nil {
			labelSelector = metav1.FormatLabelSelector(selector)
		}

		log.WithField("namespace", namespace).Info("Listing items")
		unstructuredList, err := resourceClient.List(metav1.ListOptions{LabelSelector: labelSelector})
		if err != nil {
			return errors.WithStack(err)
		}

		// do the backup
		items, err := meta.ExtractList(unstructuredList)
		if err != nil {
			return errors.WithStack(err)
		}

		log.WithField("namespace", namespace).Infof("Retrieved %d items", len(items))
		for _, item := range items {
			unstructured, ok := item.(runtime.Unstructured)
			if !ok {
				errs = append(errs, errors.Errorf("unexpected type %T", item))
				continue
			}

			metadata, err := meta.Accessor(unstructured)
			if err != nil {
				errs = append(errs, errors.Wrapf(err, "unable to get a metadata accessor"))
				continue
			}

			if gr == kuberesource.Namespaces && !rb.backupRequest.NamespaceIncludesExcludes.ShouldInclude(metadata.GetName()) {
				log.WithField("name", metadata.GetName()).Info("skipping namespace because it is excluded")
				continue
			}

			if err := itemBackupper.backupItem(log, unstructured, gr); err != nil {
				errs = append(errs, err)
			}
		}
	}

	return kuberrs.NewAggregate(errs)
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
