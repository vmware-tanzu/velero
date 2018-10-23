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
	"sort"
	"strings"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	kuberrs "k8s.io/apimachinery/pkg/util/errors"

	"github.com/heptio/ark/pkg/client"
	"github.com/heptio/ark/pkg/discovery"
	"github.com/heptio/ark/pkg/podexec"
	"github.com/heptio/ark/pkg/restic"
)

type groupBackupperFactory interface {
	newGroupBackupper(
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
	) groupBackupper
}

type defaultGroupBackupperFactory struct{}

func (f *defaultGroupBackupperFactory) newGroupBackupper(
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
) groupBackupper {
	return &defaultGroupBackupper{
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

		resourceBackupperFactory: &defaultResourceBackupperFactory{},
	}
}

type groupBackupper interface {
	backupGroup(group *metav1.APIResourceList) error
}

type defaultGroupBackupper struct {
	log                      logrus.FieldLogger
	backupRequest            *Request
	dynamicFactory           client.DynamicFactory
	discoveryHelper          discovery.Helper
	backedUpItems            map[itemKey]struct{}
	cohabitatingResources    map[string]*cohabitatingResource
	podCommandExecutor       podexec.PodCommandExecutor
	tarWriter                tarWriter
	resticBackupper          restic.Backupper
	resticSnapshotTracker    *pvcSnapshotTracker
	resourceBackupperFactory resourceBackupperFactory
	blockStoreGetter         BlockStoreGetter
}

// backupGroup backs up a single API group.
func (gb *defaultGroupBackupper) backupGroup(group *metav1.APIResourceList) error {
	var (
		errs []error
		log  = gb.log.WithField("group", group.GroupVersion)
		rb   = gb.resourceBackupperFactory.newResourceBackupper(
			log,
			gb.backupRequest,
			gb.dynamicFactory,
			gb.discoveryHelper,
			gb.backedUpItems,
			gb.cohabitatingResources,
			gb.podCommandExecutor,
			gb.tarWriter,
			gb.resticBackupper,
			gb.resticSnapshotTracker,
			gb.blockStoreGetter,
		)
	)

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
		if err := rb.backupResource(group, resource); err != nil {
			errs = append(errs, err)
		}
	}

	return kuberrs.NewAggregate(errs)
}

// sortCoreGroup sorts group as a coreGroup.
func sortCoreGroup(group *metav1.APIResourceList) {
	sort.Stable(coreGroup(group.APIResources))
}

// coreGroup is used to sort APIResources in the core API group. The sort order is pods, pvcs, pvs,
// then everything else.
type coreGroup []metav1.APIResource

func (c coreGroup) Len() int {
	return len(c)
}

func (c coreGroup) Less(i, j int) bool {
	return coreGroupResourcePriority(c[i].Name) < coreGroupResourcePriority(c[j].Name)
}

func (c coreGroup) Swap(i, j int) {
	c[j], c[i] = c[i], c[j]
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
