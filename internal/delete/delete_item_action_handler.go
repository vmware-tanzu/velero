/*
Copyright 2020 the Velero contributors.

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

package delete

import (
	"io"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/sets"

	velerov1api "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	"github.com/vmware-tanzu/velero/pkg/archive"
	"github.com/vmware-tanzu/velero/pkg/discovery"
	"github.com/vmware-tanzu/velero/pkg/plugin/velero"
	"github.com/vmware-tanzu/velero/pkg/util/collections"
	"github.com/vmware-tanzu/velero/pkg/util/filesystem"
)

// Context provides the necessary environment to run DeleteItemAction plugins
type Context struct {
	Backup          *velerov1api.Backup
	BackupReader    io.Reader
	Actions         []velero.DeleteItemAction
	Filesystem      filesystem.Interface
	Log             logrus.FieldLogger
	DiscoveryHelper discovery.Helper

	resolvedActions []resolvedAction
}

func InvokeDeleteActions(ctx *Context) error {
	var err error
	ctx.resolvedActions, err = resolveActions(ctx.Actions, ctx.DiscoveryHelper)

	// No actions installed and no error means we don't have to continue;
	// just do the backup deletion without worrying about plugins.
	if len(ctx.resolvedActions) == 0 && err == nil {
		ctx.Log.Debug("No delete item actions present, proceeding with rest of backup deletion process")
		return nil
	} else if err != nil {
		return errors.Wrapf(err, "error resolving actions")
	}

	// get items out of backup tarball into a temp directory
	dir, err := archive.NewExtractor(ctx.Log, ctx.Filesystem).UnzipAndExtractBackup(ctx.BackupReader)
	if err != nil {
		return errors.Wrapf(err, "error extracting backup")

	}
	defer ctx.Filesystem.RemoveAll(dir)
	ctx.Log.Debugf("Downloaded and extracted the backup file to: %s", dir)

	backupResources, err := archive.NewParser(ctx.Log, ctx.Filesystem).Parse(dir)
	processdResources := sets.NewString()

	ctx.Log.Debugf("Trying to reconcile resource names with Kube API server.")
	// Transform resource names based on what's canonical in the API server.
	for resource := range backupResources {
		gvr, _, err := ctx.DiscoveryHelper.ResourceFor(schema.ParseGroupResource(resource).WithVersion(""))
		if err != nil {
			return errors.Wrapf(err, "failed to resolve resource into complete group/version/resource: %v", resource)
		}

		groupResource := gvr.GroupResource()

		// We've already seen this group/resource, so don't process it again.
		if processdResources.Has(groupResource.String()) {
			continue
		}

		// Get a list of all items that exist for this resource
		resourceList := backupResources[groupResource.String()]
		if resourceList == nil {
			// After canonicalization from the API server, the resources may not exist in the tarball
			// Skip them if that's the case.
			continue
		}

		// Iterate over all items, grouped by namespace.
		for namespace, items := range resourceList.ItemsByNamespace {
			nsLog := ctx.Log.WithField("namespace", namespace)
			nsLog.Info("Starting to check for items in namespace")

			// Filter applicable actions based on namespace only once per namespace.
			actions := ctx.getApplicableActions(groupResource, namespace)

			// Process individual items from the backup
			for _, item := range items {
				itemPath := archive.GetItemFilePath(dir, resource, namespace, item)

				// obj is the Unstructured item from the backup
				obj, err := archive.Unmarshal(ctx.Filesystem, itemPath)
				if err != nil {
					return errors.Wrapf(err, "Could not unmarshal item: %v", item)
				}

				itemLog := nsLog.WithField("item", obj.GetName())
				itemLog.Infof("invoking DeleteItemAction plugins")

				for _, action := range actions {
					if !action.selector.Matches(labels.Set(obj.GetLabels())) {
						continue
					}
					err = action.Execute(&velero.DeleteItemActionExecuteInput{
						Item:   obj,
						Backup: ctx.Backup,
					})
					// Since we want to keep looping even on errors, log them instead of just returning.
					if err != nil {
						itemLog.WithError(err).Error("plugin error")

					}
				}
			}
		}
	}
	return nil
}

// getApplicableActions takes resolved DeleteItemActions and filters them for a given group/resource and namespace.
func (ctx *Context) getApplicableActions(groupResource schema.GroupResource, namespace string) []resolvedAction {
	var actions []resolvedAction

	for _, action := range ctx.resolvedActions {
		if !action.resourceIncludesExcludes.ShouldInclude(groupResource.String()) {
			continue
		}

		if namespace != "" && !action.namespaceIncludesExcludes.ShouldInclude(namespace) {
			continue
		}

		if namespace == "" && !action.namespaceIncludesExcludes.IncludeEverything() {
			continue
		}

		actions = append(actions, action)
	}

	return actions
}

// resolvedActions are DeleteItemActions decorated with resource/namespace include/exclude collections, as well as label selectors for easy comparison.
type resolvedAction struct {
	velero.DeleteItemAction

	resourceIncludesExcludes  *collections.IncludesExcludes
	namespaceIncludesExcludes *collections.IncludesExcludes
	selector                  labels.Selector
}

// resolveActions resolves the AppliesTo ResourceSelectors of DeleteItemActions plugins against the Kubernetes discovery API for fully-qualified names.
func resolveActions(actions []velero.DeleteItemAction, helper discovery.Helper) ([]resolvedAction, error) {
	var resolved []resolvedAction

	for _, action := range actions {
		resourceSelector, err := action.AppliesTo()
		if err != nil {
			return nil, err
		}

		resources := collections.GetResourceIncludesExcludes(helper, resourceSelector.IncludedResources, resourceSelector.ExcludedResources)
		namespaces := collections.NewIncludesExcludes().Includes(resourceSelector.IncludedNamespaces...).Excludes(resourceSelector.ExcludedNamespaces...)

		selector := labels.Everything()
		if resourceSelector.LabelSelector != "" {
			if selector, err = labels.Parse(resourceSelector.LabelSelector); err != nil {
				return nil, err
			}
		}

		res := resolvedAction{
			DeleteItemAction:          action,
			resourceIncludesExcludes:  resources,
			namespaceIncludesExcludes: namespaces,
			selector:                  selector,
		}
		resolved = append(resolved, res)
	}

	return resolved, nil
}
