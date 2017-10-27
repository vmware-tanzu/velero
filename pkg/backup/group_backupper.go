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
	"strings"

	"github.com/heptio/ark/pkg/apis/ark/v1"
	"github.com/heptio/ark/pkg/client"
	"github.com/heptio/ark/pkg/discovery"
	"github.com/heptio/ark/pkg/util/collections"
	"github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	kuberrs "k8s.io/apimachinery/pkg/util/errors"
)

type groupBackupperFactory interface {
	newGroupBackupper(
		log *logrus.Entry,
		backup *v1.Backup,
		namespaces, resources *collections.IncludesExcludes,
		labelSelector string,
		dynamicFactory client.DynamicFactory,
		discoveryHelper discovery.Helper,
		backedUpItems map[itemKey]struct{},
		cohabitatingResources map[string]*cohabitatingResource,
		actions map[schema.GroupResource]Action,
		podCommandExecutor podCommandExecutor,
		tarWriter tarWriter,
		resourceHooks []resourceHook,
	) groupBackupper
}

type defaultGroupBackupperFactory struct{}

func (f *defaultGroupBackupperFactory) newGroupBackupper(
	log *logrus.Entry,
	backup *v1.Backup,
	namespaces, resources *collections.IncludesExcludes,
	labelSelector string,
	dynamicFactory client.DynamicFactory,
	discoveryHelper discovery.Helper,
	backedUpItems map[itemKey]struct{},
	cohabitatingResources map[string]*cohabitatingResource,
	actions map[schema.GroupResource]Action,
	podCommandExecutor podCommandExecutor,
	tarWriter tarWriter,
	resourceHooks []resourceHook,
) groupBackupper {
	return &defaultGroupBackupper{
		log:                   log,
		backup:                backup,
		namespaces:            namespaces,
		resources:             resources,
		labelSelector:         labelSelector,
		dynamicFactory:        dynamicFactory,
		discoveryHelper:       discoveryHelper,
		backedUpItems:         backedUpItems,
		cohabitatingResources: cohabitatingResources,
		actions:               actions,
		podCommandExecutor:    podCommandExecutor,
		tarWriter:             tarWriter,
		resourceHooks:         resourceHooks,

		resourceBackupperFactory: &defaultResourceBackupperFactory{},
	}
}

type groupBackupper interface {
	backupGroup(group *metav1.APIResourceList) error
}

type defaultGroupBackupper struct {
	log                      *logrus.Entry
	backup                   *v1.Backup
	namespaces, resources    *collections.IncludesExcludes
	labelSelector            string
	dynamicFactory           client.DynamicFactory
	discoveryHelper          discovery.Helper
	backedUpItems            map[itemKey]struct{}
	cohabitatingResources    map[string]*cohabitatingResource
	actions                  map[schema.GroupResource]Action
	podCommandExecutor       podCommandExecutor
	tarWriter                tarWriter
	resourceHooks            []resourceHook
	resourceBackupperFactory resourceBackupperFactory
}

// backupGroup backs up a single API group.
func (gb *defaultGroupBackupper) backupGroup(group *metav1.APIResourceList) error {
	var (
		errs []error
		pv   *metav1.APIResource
		log  = gb.log.WithField("group", group.GroupVersion)
		rb   = gb.resourceBackupperFactory.newResourceBackupper(
			log,
			gb.backup,
			gb.namespaces,
			gb.resources,
			gb.labelSelector,
			gb.dynamicFactory,
			gb.discoveryHelper,
			gb.backedUpItems,
			gb.cohabitatingResources,
			gb.actions,
			gb.podCommandExecutor,
			gb.tarWriter,
			gb.resourceHooks,
		)
	)

	log.Infof("Backing up group")

	processResource := func(resource metav1.APIResource) {
		if err := rb.backupResource(group, resource); err != nil {
			errs = append(errs, err)
		}
	}

	for _, resource := range group.APIResources {
		// do PVs last because if we're also backing up PVCs, we want to backup PVs within the scope of
		// the PVCs (within the PVC action) to allow for hooks to run
		if strings.ToLower(resource.Name) == "persistentvolumes" && strings.ToLower(group.GroupVersion) == "v1" {
			pvResource := resource
			pv = &pvResource
			continue
		}
		processResource(resource)
	}

	if pv != nil {
		processResource(*pv)
	}

	return kuberrs.NewAggregate(errs)
}
