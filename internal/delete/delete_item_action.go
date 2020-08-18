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
	"github.com/vmware-tanzu/velero/pkg/archive"
	"github.com/vmware-tanzu/velero/pkg/discovery"
	"github.com/vmware-tanzu/velero/pkg/plugin/velero"
	"github.com/vmware-tanzu/velero/pkg/util/collections"
	"github.com/vmware-tanzu/velero/pkg/util/filesystem"
	"k8s.io/apimachinery/pkg/labels"
)

// Context provides the necessary environment to run DeleteItemAction plugins
type Context struct {
	BackupReader    io.Reader
	Actions         []velero.DeleteItemAction
	Filesystem      filesystem.Interface
	Log             logrus.FieldLogger
	DiscoveryHelper discovery.Helper
}

func InvokeDeleteActions(ctx *Context) error {
	// helper will need to be passed in to the controller.
	resolvedActions, err := resolveActions(ctx.Actions, ctx.DiscoveryHelper)

	// No actions installed and no error means we don't have to continue;
	// just do the backup deletion without worrying about plugins.
	if resolvedActions == nil && err == nil {
		ctx.Log.Debug("No delete item actions present, proceeding with rest of backup deletion process")
		return nil
	}

	// // get items out of backup tarball into a temp directory
	dir, err := archive.NewExtractor(ctx.Log, ctx.Filesystem).UnzipAndExtractBackup(ctx.BackupReader)
	if err != nil {
		return errors.Wrapf(err, "error extracting backup")

	}
	defer ctx.Filesystem.RemoveAll(dir)
	ctx.Log.Debugf("Downloaded and extracted the backup file to: %s", dir)

	backupResources, err := archive.NewParser(ctx.Log, ctx.Filesystem).Parse(dir)
	if err != nil {
		return errors.Wrapf(err, "error parsing backup contents")
	}
	for _, r := range backupResources {
		ctx.Log.Debugf("Have this resource: %#v", r)
	}
	// // iterate through resources. Need to transform these from a map into a string, probably with some ordering like in restore.
	// for _, resource := range backupResources {
	// 	gvr, _, err := ctx.DiscoveryHelper.ResourceFor(schema.ParseGroupKind(resource).WithVersion(""))
	// 	if err != nil {
	// 		return errors.Wrapf(err, "failed to resolve resource into complete group/version/resource: %v", resource)
	// 	}

	// }
	// unmarshal items into Unstructured
	// get applicable actions for the group
	// apply actions to the Unstructured items

	return nil
}

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
