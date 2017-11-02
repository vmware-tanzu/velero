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
	api "github.com/heptio/ark/pkg/apis/ark/v1"
	"github.com/heptio/ark/pkg/client"
	"github.com/heptio/ark/pkg/discovery"
	"github.com/heptio/ark/pkg/util/collections"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	kuberrs "k8s.io/apimachinery/pkg/util/errors"
)

type resourceBackupperFactory interface {
	newResourceBackupper(
		log *logrus.Entry,
		backup *api.Backup,
		namespaces *collections.IncludesExcludes,
		resources *collections.IncludesExcludes,
		labelSelector string,
		dynamicFactory client.DynamicFactory,
		discoveryHelper discovery.Helper,
		backedUpItems map[itemKey]struct{},
		cohabitatingResources map[string]*cohabitatingResource,
		actions map[schema.GroupResource]Action,
		podCommandExecutor podCommandExecutor,
		tarWriter tarWriter,
		resourceHooks []resourceHook,
	) resourceBackupper
}

type defaultResourceBackupperFactory struct{}

func (f *defaultResourceBackupperFactory) newResourceBackupper(
	log *logrus.Entry,
	backup *api.Backup,
	namespaces *collections.IncludesExcludes,
	resources *collections.IncludesExcludes,
	labelSelector string,
	dynamicFactory client.DynamicFactory,
	discoveryHelper discovery.Helper,
	backedUpItems map[itemKey]struct{},
	cohabitatingResources map[string]*cohabitatingResource,
	actions map[schema.GroupResource]Action,
	podCommandExecutor podCommandExecutor,
	tarWriter tarWriter,
	resourceHooks []resourceHook,
) resourceBackupper {
	return &defaultResourceBackupper{
		log:                   log,
		backup:                backup,
		namespaces:            namespaces,
		resources:             resources,
		labelSelector:         labelSelector,
		dynamicFactory:        dynamicFactory,
		discoveryHelper:       discoveryHelper,
		backedUpItems:         backedUpItems,
		actions:               actions,
		cohabitatingResources: cohabitatingResources,
		podCommandExecutor:    podCommandExecutor,
		tarWriter:             tarWriter,
		resourceHooks:         resourceHooks,

		itemBackupperFactory: &defaultItemBackupperFactory{},
	}
}

type resourceBackupper interface {
	backupResource(group *metav1.APIResourceList, resource metav1.APIResource) error
}

type defaultResourceBackupper struct {
	log                   *logrus.Entry
	backup                *api.Backup
	namespaces            *collections.IncludesExcludes
	resources             *collections.IncludesExcludes
	labelSelector         string
	dynamicFactory        client.DynamicFactory
	discoveryHelper       discovery.Helper
	backedUpItems         map[itemKey]struct{}
	cohabitatingResources map[string]*cohabitatingResource
	actions               map[schema.GroupResource]Action
	podCommandExecutor    podCommandExecutor
	tarWriter             tarWriter
	resourceHooks         []resourceHook

	itemBackupperFactory itemBackupperFactory
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

	switch {
	case rb.backup.Spec.IncludeClusterResources == nil:
		// when IncludeClusterResources == nil (auto), only directly
		// back up cluster-scoped resources if we're doing a full-cluster
		// (all namespaces) backup. Note that in the case of a subset of
		// namespaces being backed up, some related cluster-scoped resources
		// may still be backed up if triggered by a custom action (e.g. PVC->PV).
		// If we're processing namespaces themselves, we will not skip here, they may be
		// filtered out later.
		if !resource.Namespaced && resource.Kind != "Namespace" && !rb.namespaces.IncludeEverything() {
			log.Info("Skipping resource because it's cluster-scoped and only specific namespaces are included in the backup")
			return nil
		}
	case *rb.backup.Spec.IncludeClusterResources == false:
		if !resource.Namespaced {
			log.Info("Skipping resource because it's cluster-scoped")
			return nil
		}
	case *rb.backup.Spec.IncludeClusterResources == true:
		// include the resource, no action required
	}

	if !rb.resources.ShouldInclude(grString) {
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
		rb.backup,
		rb.namespaces,
		rb.resources,
		rb.backedUpItems,
		rb.actions,
		rb.podCommandExecutor,
		rb.tarWriter,
		rb.resourceHooks,
		rb.dynamicFactory,
		rb.discoveryHelper,
	)

	// TODO: when processing namespaces, and only including certain namespaces, we still list
	// them all here. Could optimize to get specifics, but watch out for label selector.
	var namespacesToList []string
	if resource.Namespaced {
		namespacesToList = getNamespacesToList(rb.namespaces)
	} else {
		namespacesToList = []string{""}
	}
	for _, namespace := range namespacesToList {
		resourceClient, err := rb.dynamicFactory.ClientForGroupVersionResource(gv, resource, namespace)
		if err != nil {
			return err
		}

		unstructuredList, err := resourceClient.List(metav1.ListOptions{LabelSelector: rb.labelSelector})
		if err != nil {
			return errors.WithStack(err)
		}

		// do the backup
		items, err := meta.ExtractList(unstructuredList)
		if err != nil {
			return errors.WithStack(err)
		}

		for _, item := range items {
			unstructured, ok := item.(runtime.Unstructured)
			if !ok {
				errs = append(errs, errors.Errorf("unexpected type %T", item))
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
