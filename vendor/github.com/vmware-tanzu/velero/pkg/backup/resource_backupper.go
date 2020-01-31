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

package backup

import (
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	apiextv1beta1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
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
	"github.com/vmware-tanzu/velero/pkg/podexec"
	"github.com/vmware-tanzu/velero/pkg/restic"
	"github.com/vmware-tanzu/velero/pkg/util/collections"
)

type resourceBackupperFactory interface {
	newResourceBackupper(
		log logrus.FieldLogger,
		backupRequest *Request,
		dynamicFactory client.DynamicFactory,
		discoveryHelper discovery.Helper,
		cohabitatingResources map[string]*cohabitatingResource,
		podCommandExecutor podexec.PodCommandExecutor,
		tarWriter tarWriter,
		resticBackupper restic.Backupper,
		resticSnapshotTracker *pvcSnapshotTracker,
		volumeSnapshotterGetter VolumeSnapshotterGetter,
	) resourceBackupper
}

type defaultResourceBackupperFactory struct{}

func (f *defaultResourceBackupperFactory) newResourceBackupper(
	log logrus.FieldLogger,
	backupRequest *Request,
	dynamicFactory client.DynamicFactory,
	discoveryHelper discovery.Helper,
	cohabitatingResources map[string]*cohabitatingResource,
	podCommandExecutor podexec.PodCommandExecutor,
	tarWriter tarWriter,
	resticBackupper restic.Backupper,
	resticSnapshotTracker *pvcSnapshotTracker,
	volumeSnapshotterGetter VolumeSnapshotterGetter,
) resourceBackupper {
	return &defaultResourceBackupper{
		log:                     log,
		backupRequest:           backupRequest,
		dynamicFactory:          dynamicFactory,
		discoveryHelper:         discoveryHelper,
		cohabitatingResources:   cohabitatingResources,
		podCommandExecutor:      podCommandExecutor,
		tarWriter:               tarWriter,
		resticBackupper:         resticBackupper,
		resticSnapshotTracker:   resticSnapshotTracker,
		volumeSnapshotterGetter: volumeSnapshotterGetter,

		itemBackupperFactory: &defaultItemBackupperFactory{},
	}
}

type resourceBackupper interface {
	backupResource(group *metav1.APIResourceList, resource metav1.APIResource) error
}

type defaultResourceBackupper struct {
	log                     logrus.FieldLogger
	backupRequest           *Request
	dynamicFactory          client.DynamicFactory
	discoveryHelper         discovery.Helper
	cohabitatingResources   map[string]*cohabitatingResource
	podCommandExecutor      podexec.PodCommandExecutor
	tarWriter               tarWriter
	resticBackupper         restic.Backupper
	resticSnapshotTracker   *pvcSnapshotTracker
	itemBackupperFactory    itemBackupperFactory
	volumeSnapshotterGetter VolumeSnapshotterGetter
}

// backupResource backs up all the objects for a given group-version-resource.
func (rb *defaultResourceBackupper) backupResource(group *metav1.APIResourceList, resource metav1.APIResource) error {
	log := rb.log.WithField("resource", resource.Name)

	log.Info("Backing up resource")

	gv, err := schema.ParseGroupVersion(group.GroupVersion)
	if err != nil {
		return errors.Wrapf(err, "error parsing GroupVersion %s", group.GroupVersion)
	}
	gr := schema.GroupResource{Group: gv.Group, Resource: resource.Name}

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

	if !rb.backupRequest.ResourceIncludesExcludes.ShouldInclude(gr.String()) {
		log.Infof("Skipping resource because it's excluded")
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
		rb.podCommandExecutor,
		rb.tarWriter,
		rb.dynamicFactory,
		rb.discoveryHelper,
		rb.resticBackupper,
		rb.resticSnapshotTracker,
		rb.volumeSnapshotterGetter,
	)

	namespacesToList := getNamespacesToList(rb.backupRequest.NamespaceIncludesExcludes)

	// Check if we're backing up namespaces, and only certain ones
	if gr == kuberesource.Namespaces && namespacesToList[0] != "" {
		resourceClient, err := rb.dynamicFactory.ClientForGroupVersionResource(gv, resource, "")
		if err != nil {
			log.WithError(err).Error("Error getting dynamic client")
		} else {
			var labelSelector labels.Selector
			if rb.backupRequest.Spec.LabelSelector != nil {
				labelSelector, err = metav1.LabelSelectorAsSelector(rb.backupRequest.Spec.LabelSelector)
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

				if _, err := itemBackupper.backupItem(log, unstructured, gr); err != nil {
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

		resourceClient, err := rb.dynamicFactory.ClientForGroupVersionResource(gv, resource, namespace)
		if err != nil {
			log.WithError(err).Error("Error getting dynamic client")
			continue
		}

		var labelSelector string
		if selector := rb.backupRequest.Spec.LabelSelector; selector != nil {
			labelSelector = metav1.FormatLabelSelector(selector)
		}

		log.Info("Listing items")
		unstructuredList, err := resourceClient.List(metav1.ListOptions{LabelSelector: labelSelector})
		if err != nil {
			log.WithError(errors.WithStack(err)).Error("Error listing items")
			continue
		}

		// do the backup
		items, err := meta.ExtractList(unstructuredList)
		if err != nil {
			log.WithError(errors.WithStack(err)).Error("Error extracting list")
			continue
		}

		log.Infof("Retrieved %d items", len(items))

		for _, item := range items {
			unstructured, ok := item.(runtime.Unstructured)
			if !ok {
				log.Errorf("Unexpected type %T", item)
				continue
			}
			if rb.backupItem(log, gr, itemBackupper, unstructured) {
				backedUpItem = true
			}
		}
	}

	// back up CRD for resource if found. We should only need to do this if we've backed up at least
	// one item and IncludeClusterResources is nil. If IncludeClusterResources is false
	// we don't want to back it up, and if it's true it will already be included.
	if backedUpItem && rb.backupRequest.Spec.IncludeClusterResources == nil {
		rb.backupCRD(log, gr, itemBackupper)
	}

	return nil
}

func (rb *defaultResourceBackupper) backupItem(
	log logrus.FieldLogger,
	gr schema.GroupResource,
	itemBackupper ItemBackupper,
	unstructured runtime.Unstructured,
) bool {
	metadata, err := meta.Accessor(unstructured)
	if err != nil {
		log.WithError(errors.WithStack(err)).Error("Error getting a metadata accessor")
		return false
	}

	if gr == kuberesource.Namespaces && !rb.backupRequest.NamespaceIncludesExcludes.ShouldInclude(metadata.GetName()) {
		log.WithField("name", metadata.GetName()).Info("Skipping namespace because it's excluded")
		return false
	}

	backedUpItem, err := itemBackupper.backupItem(log, unstructured, gr)
	if aggregate, ok := err.(kubeerrs.Aggregate); ok {
		log.WithField("name", metadata.GetName()).Infof("%d errors encountered backup up item", len(aggregate.Errors()))
		// log each error separately so we get error location info in the log, and an
		// accurate count of errors
		for _, err = range aggregate.Errors() {
			log.WithError(err).WithField("name", metadata.GetName()).Error("Error backing up item")
		}

		return false
	}
	if err != nil {
		log.WithError(err).WithField("name", metadata.GetName()).Error("Error backing up item")
		return false
	}
	return backedUpItem
}

// Adds CRD to the backup if one is found corresponding to this resource
func (rb *defaultResourceBackupper) backupCRD(
	log logrus.FieldLogger,
	gr schema.GroupResource,
	itemBackupper ItemBackupper,
) {
	crdGr := schema.GroupResource{Group: apiextv1beta1.GroupName, Resource: "customresourcedefinitions"}
	crdClient, err := rb.dynamicFactory.ClientForGroupVersionResource(apiextv1beta1.SchemeGroupVersion,
		metav1.APIResource{
			Name:       "customresourcedefinitions",
			Namespaced: false,
		},
		"",
	)
	if err != nil {
		log.WithError(err).Error("Error getting dynamic client for CRDs")
		return
	}

	unstructured, err := crdClient.Get(gr.String(), metav1.GetOptions{})
	if err != nil {
		if !apierrors.IsNotFound(err) {
			log.WithError(errors.WithStack(err)).Error("Error getting CRD")
		}
		return
	}
	log.Infof("Found associated CRD to add to backup %s", gr.String())
	_ = rb.backupItem(log, crdGr, itemBackupper, unstructured)
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
