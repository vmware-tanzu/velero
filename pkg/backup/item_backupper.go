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
	"archive/tar"
	"encoding/json"
	"path/filepath"
	"time"

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
)

type itemBackupperFactory interface {
	newItemBackupper(
		backup *api.Backup,
		namespaces, resources *collections.IncludesExcludes,
		backedUpItems map[itemKey]struct{},
		actions map[schema.GroupResource]Action,
		podCommandExecutor podCommandExecutor,
		tarWriter tarWriter,
		resourceHooks []resourceHook,
		dynamicFactory client.DynamicFactory,
		discoveryHelper discovery.Helper,
	) ItemBackupper
}

type defaultItemBackupperFactory struct{}

func (f *defaultItemBackupperFactory) newItemBackupper(
	backup *api.Backup,
	namespaces, resources *collections.IncludesExcludes,
	backedUpItems map[itemKey]struct{},
	actions map[schema.GroupResource]Action,
	podCommandExecutor podCommandExecutor,
	tarWriter tarWriter,
	resourceHooks []resourceHook,
	dynamicFactory client.DynamicFactory,
	discoveryHelper discovery.Helper,
) ItemBackupper {
	ib := &defaultItemBackupper{
		backup:          backup,
		namespaces:      namespaces,
		resources:       resources,
		backedUpItems:   backedUpItems,
		actions:         actions,
		tarWriter:       tarWriter,
		resourceHooks:   resourceHooks,
		dynamicFactory:  dynamicFactory,
		discoveryHelper: discoveryHelper,

		itemHookHandler: &defaultItemHookHandler{
			podCommandExecutor: podCommandExecutor,
		},
	}

	// this is for testing purposes
	ib.additionalItemBackupper = ib

	return ib
}

type ItemBackupper interface {
	backupItem(logger *logrus.Entry, obj runtime.Unstructured, groupResource schema.GroupResource) error
}

type defaultItemBackupper struct {
	backup          *api.Backup
	namespaces      *collections.IncludesExcludes
	resources       *collections.IncludesExcludes
	backedUpItems   map[itemKey]struct{}
	actions         map[schema.GroupResource]Action
	tarWriter       tarWriter
	resourceHooks   []resourceHook
	dynamicFactory  client.DynamicFactory
	discoveryHelper discovery.Helper

	itemHookHandler         itemHookHandler
	additionalItemBackupper ItemBackupper
}

var podsGroupResource = schema.GroupResource{Group: "", Resource: "pods"}
var namespacesGroupResource = schema.GroupResource{Group: "", Resource: "namespaces"}

// backupItem backs up an individual item to tarWriter. The item may be excluded based on the
// namespaces IncludesExcludes list.
func (ib *defaultItemBackupper) backupItem(logger *logrus.Entry, obj runtime.Unstructured, groupResource schema.GroupResource) error {
	metadata, err := meta.Accessor(obj)
	if err != nil {
		return err
	}

	namespace := metadata.GetNamespace()
	name := metadata.GetName()

	log := logger.WithField("name", name)
	if namespace != "" {
		log = log.WithField("namespace", namespace)
	}

	// NOTE: we have to re-check namespace & resource includes/excludes because it's possible that
	// backupItem can be invoked by a custom action.
	if namespace != "" && !ib.namespaces.ShouldInclude(namespace) {
		log.Info("Excluding item because namespace is excluded")
		return nil
	}

	// NOTE: we specifically allow namespaces to be backed up even if IncludeClusterResources is
	// false.
	if namespace == "" && groupResource != namespacesGroupResource && ib.backup.Spec.IncludeClusterResources != nil && !*ib.backup.Spec.IncludeClusterResources {
		log.Info("Excluding item because resource is cluster-scoped and backup.spec.includeClusterResources is false")
		return nil
	}

	if !ib.resources.ShouldInclude(groupResource.String()) {
		log.Info("Excluding item because resource is excluded")
		return nil
	}

	key := itemKey{
		resource:  groupResource.String(),
		namespace: namespace,
		name:      name,
	}

	if _, exists := ib.backedUpItems[key]; exists {
		log.Info("Skipping item because it's already been backed up.")
		return nil
	}
	ib.backedUpItems[key] = struct{}{}

	log.Info("Backing up resource")

	item := obj.UnstructuredContent()
	// Never save status
	delete(item, "status")

	if err := ib.itemHookHandler.handleHooks(log, groupResource, obj, ib.resourceHooks); err != nil {
		return err
	}

	if action, found := ib.actions[groupResource]; found {
		log.Info("Executing custom action")

		if additionalItemIdentifiers, err := action.Execute(log, obj, ib.backup); err == nil {
			for _, additionalItem := range additionalItemIdentifiers {
				gvr, resource, err := ib.discoveryHelper.ResourceFor(additionalItem.GroupResource.WithVersion(""))
				if err != nil {
					return err
				}

				client, err := ib.dynamicFactory.ClientForGroupVersionResource(gvr.GroupVersion(), resource, additionalItem.Namespace)
				if err != nil {
					return err
				}

				additionalItem, err := client.Get(additionalItem.Name, metav1.GetOptions{})
				if err != nil {
					return err
				}

				ib.additionalItemBackupper.backupItem(log, additionalItem, gvr.GroupResource())
			}
		} else {
			return errors.Wrap(err, "error executing custom action")
		}
	}

	var filePath string
	if namespace != "" {
		filePath = filepath.Join(api.ResourcesDir, groupResource.String(), api.NamespaceScopedDir, namespace, name+".json")
	} else {
		filePath = filepath.Join(api.ResourcesDir, groupResource.String(), api.ClusterScopedDir, name+".json")
	}

	itemBytes, err := json.Marshal(item)
	if err != nil {
		return errors.WithStack(err)
	}

	hdr := &tar.Header{
		Name:     filePath,
		Size:     int64(len(itemBytes)),
		Typeflag: tar.TypeReg,
		Mode:     0755,
		ModTime:  time.Now(),
	}

	if err := ib.tarWriter.WriteHeader(hdr); err != nil {
		return errors.WithStack(err)
	}

	if _, err := ib.tarWriter.Write(itemBytes); err != nil {
		return errors.WithStack(err)
	}

	return nil
}
