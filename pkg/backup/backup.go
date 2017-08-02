/*
Copyright 2017 Heptio Inc.

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
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/golang/glog"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	kuberrs "k8s.io/apimachinery/pkg/util/errors"

	api "github.com/heptio/ark/pkg/apis/ark/v1"
	"github.com/heptio/ark/pkg/client"
	"github.com/heptio/ark/pkg/discovery"
	"github.com/heptio/ark/pkg/util/collections"
)

// Backupper performs backups.
type Backupper interface {
	// Backup takes a backup using the specification in the api.Backup and writes backup data to the
	// given writers.
	Backup(backup *api.Backup, data io.Writer) error
}

// kubernetesBackupper implements Backupper.
type kubernetesBackupper struct {
	dynamicFactory  client.DynamicFactory
	discoveryHelper discovery.Helper
	actions         map[schema.GroupResource]Action
	itemBackupper   itemBackupper
}

var _ Backupper = &kubernetesBackupper{}

// Action is an actor that performs an operation on an individual item being backed up.
type Action interface {
	// Execute is invoked on an item being backed up. If an error is returned, the Backup is marked as
	// failed.
	Execute(item map[string]interface{}, backup *api.Backup) error
}

// NewKubernetesBackupper creates a new kubernetesBackupper.
func NewKubernetesBackupper(
	discoveryHelper discovery.Helper,
	dynamicFactory client.DynamicFactory,
	actions map[string]Action,
) (Backupper, error) {
	resolvedActions, err := resolveActions(discoveryHelper.Mapper(), actions)
	if err != nil {
		return nil, err
	}

	return &kubernetesBackupper{
		discoveryHelper: discoveryHelper,
		dynamicFactory:  dynamicFactory,
		actions:         resolvedActions,
		itemBackupper:   &realItemBackupper{},
	}, nil
}

// resolveActions resolves the string-based map of group-resources to actions and returns a map of
// schema.GroupResources to actions.
func resolveActions(mapper meta.RESTMapper, actions map[string]Action) (map[schema.GroupResource]Action, error) {
	ret := make(map[schema.GroupResource]Action)

	for resource, action := range actions {
		gr, err := resolveGroupResource(mapper, resource)
		if err != nil {
			return nil, err
		}
		ret[gr] = action
	}

	return ret, nil
}

// getResourceIncludesExcludes takes the lists of resources to include and exclude from the
// backup, uses the RESTMapper to resolve them to fully-qualified group-resource names, and returns
// an IncludesExcludes list.
func getResourceIncludesExcludes(mapper meta.RESTMapper, backup *api.Backup) *collections.IncludesExcludes {
	resources := collections.NewIncludesExcludes()

	resolve := func(list []string, allowAll bool, f func(string)) {
		for _, resource := range list {
			if allowAll && resource == "*" {
				f("*")
				return
			}
			gr, err := resolveGroupResource(mapper, resource)
			if err != nil {
				glog.Errorf("unable to include resource %q in backup: %v", resource, err)
				continue
			}
			f(gr.String())
		}
	}

	resolve(backup.Spec.IncludedResources, true, func(s string) { resources.Includes(s) })
	resolve(backup.Spec.ExcludedResources, false, func(s string) { resources.Excludes(s) })

	return resources
}

// resolveGroupResource uses the RESTMapper to resolve resource to a fully-qualified
// schema.GroupResource. If the RESTMapper is unable to do so, an error is returned instead.
func resolveGroupResource(mapper meta.RESTMapper, resource string) (schema.GroupResource, error) {
	gvr, err := mapper.ResourceFor(schema.ParseGroupResource(resource).WithVersion(""))
	if err != nil {
		return schema.GroupResource{}, err
	}
	return gvr.GroupResource(), nil
}

// getNamespaceIncludesExcludes returns an IncludesExcludes list containing which namespaces to
// include and exclude from the backup.
func getNamespaceIncludesExcludes(backup *api.Backup) *collections.IncludesExcludes {
	return collections.NewIncludesExcludes().Includes(backup.Spec.IncludedNamespaces...).Excludes(backup.Spec.ExcludedNamespaces...)
}

type backupContext struct {
	backup                    *api.Backup
	w                         tarWriter
	namespaceIncludesExcludes *collections.IncludesExcludes
	resourceIncludesExcludes  *collections.IncludesExcludes
	// deploymentsBackedUp marks whether we've seen and are backing up the deployments resource, from
	// either the apps or extensions api groups. We only want to back them up once, from whichever api
	// group we see first.
	deploymentsBackedUp bool
	// networkPoliciesBackedUp marks whether we've seen and are backing up the networkpolicies
	// resource, from either the networking.k8s.io or extensions api groups. We only want to back them
	// up once, from whichever api group we see first.
	networkPoliciesBackedUp bool
}

// Backup backs up the items specified in the Backup, placing them in a gzip-compressed tar file
// written to data. The finalized api.Backup is written to metadata.
func (kb *kubernetesBackupper) Backup(backup *api.Backup, data io.Writer) error {
	gzw := gzip.NewWriter(data)
	defer gzw.Close()

	tw := tar.NewWriter(gzw)
	defer tw.Close()

	var errs []error

	ctx := &backupContext{
		backup: backup,
		w:      tw,
		namespaceIncludesExcludes: getNamespaceIncludesExcludes(backup),
		resourceIncludesExcludes:  getResourceIncludesExcludes(kb.discoveryHelper.Mapper(), backup),
	}

	for _, group := range kb.discoveryHelper.Resources() {
		glog.V(2).Infof("Backing up group %q\n", group.GroupVersion)
		if err := kb.backupGroup(ctx, group); err != nil {
			errs = append(errs, err)
		}
	}

	return kuberrs.NewAggregate(errs)
}

type tarWriter interface {
	io.Closer
	Write([]byte) (int, error)
	WriteHeader(*tar.Header) error
}

// backupGroup backs up a single API group.
func (kb *kubernetesBackupper) backupGroup(ctx *backupContext, group *metav1.APIResourceList) error {
	var errs []error

	for _, resource := range group.APIResources {
		glog.V(2).Infof("Backing up resource %s/%s\n", group.GroupVersion, resource.Name)
		if err := kb.backupResource(ctx, group, resource); err != nil {
			errs = append(errs, err)
		}
	}

	return kuberrs.NewAggregate(errs)
}

const (
	appsDeploymentsResource           = "deployments.apps"
	extensionsDeploymentsResource     = "deployments.extensions"
	networkingNetworkPoliciesResource = "networkpolicies.networking.k8s.io"
	extensionsNetworkPoliciesResource = "networkpolicies.extensions"
)

// backupResource backs up all the objects for a given group-version-resource.
func (kb *kubernetesBackupper) backupResource(
	ctx *backupContext,
	group *metav1.APIResourceList,
	resource metav1.APIResource,
) error {
	var errs []error

	gv, err := schema.ParseGroupVersion(group.GroupVersion)
	if err != nil {
		return err
	}
	gvr := schema.GroupVersionResource{Group: gv.Group, Version: gv.Version}
	gr := schema.GroupResource{Group: gv.Group, Resource: resource.Name}
	grString := gr.String()

	if !ctx.resourceIncludesExcludes.ShouldInclude(grString) {
		glog.V(2).Infof("Not including resource %s\n", grString)
		return nil
	}

	if grString == appsDeploymentsResource || grString == extensionsDeploymentsResource {
		if ctx.deploymentsBackedUp {
			var other string
			if grString == appsDeploymentsResource {
				other = extensionsDeploymentsResource
			} else {
				other = appsDeploymentsResource
			}
			glog.V(4).Infof("Skipping resource %q because it's a duplicate of %q", grString, other)
			return nil
		}

		ctx.deploymentsBackedUp = true
	}

	if grString == networkingNetworkPoliciesResource || grString == extensionsNetworkPoliciesResource {
		if ctx.networkPoliciesBackedUp {
			var other string
			if grString == networkingNetworkPoliciesResource {
				other = extensionsNetworkPoliciesResource
			} else {
				other = networkingNetworkPoliciesResource
			}
			glog.V(4).Infof("Skipping resource %q because it's a duplicate of %q", grString, other)
			return nil
		}

		ctx.networkPoliciesBackedUp = true
	}

	var namespacesToList []string
	if resource.Namespaced {
		namespacesToList = getNamespacesToList(ctx.namespaceIncludesExcludes)
	} else {
		namespacesToList = []string{""}
	}
	for _, namespace := range namespacesToList {
		resourceClient, err := kb.dynamicFactory.ClientForGroupVersionResource(gvr, resource, namespace)
		if err != nil {
			return err
		}

		labelSelector := ""
		if ctx.backup.Spec.LabelSelector != nil {
			labelSelector = metav1.FormatLabelSelector(ctx.backup.Spec.LabelSelector)
		}
		unstructuredList, err := resourceClient.List(metav1.ListOptions{LabelSelector: labelSelector})
		if err != nil {
			return err
		}

		// do the backup
		items, err := meta.ExtractList(unstructuredList)
		if err != nil {
			return err
		}

		action := kb.actions[gr]

		for _, item := range items {
			unstructured, ok := item.(runtime.Unstructured)
			if !ok {
				errs = append(errs, fmt.Errorf("unexpected type %T", item))
				continue
			}

			obj := unstructured.UnstructuredContent()

			if err := kb.itemBackupper.backupItem(ctx, obj, grString, action); err != nil {
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

type itemBackupper interface {
	backupItem(ctx *backupContext, item map[string]interface{}, groupResource string, action Action) error
}

type realItemBackupper struct{}

// backupItem backs up an individual item to tarWriter. The item may be excluded based on the
// namespaces IncludesExcludes list.
func (*realItemBackupper) backupItem(ctx *backupContext, item map[string]interface{}, groupResource string, action Action) error {
	// Never save status
	delete(item, "status")

	metadata, err := collections.GetMap(item, "metadata")
	if err != nil {
		return err
	}

	name, err := collections.GetString(metadata, "name")
	if err != nil {
		return err
	}

	namespace, err := collections.GetString(metadata, "namespace")
	if err == nil {
		if !ctx.namespaceIncludesExcludes.ShouldInclude(namespace) {
			glog.V(2).Infof("Excluding item %s because namespace %s is excluded\n", name, namespace)
			return nil
		}
	}

	if action != nil {
		glog.V(4).Infof("Executing action on %s, ns=%s, name=%s", groupResource, namespace, name)
		action.Execute(item, ctx.backup)
	}

	glog.V(2).Infof("Backing up resource=%s, ns=%s, name=%s", groupResource, namespace, name)

	var filePath string
	if namespace != "" {
		filePath = strings.Join([]string{api.NamespaceScopedDir, namespace, groupResource, name + ".json"}, "/")
	} else {
		filePath = strings.Join([]string{api.ClusterScopedDir, groupResource, name + ".json"}, "/")
	}

	itemBytes, err := json.Marshal(item)
	if err != nil {
		return err
	}

	hdr := &tar.Header{
		Name:     filePath,
		Size:     int64(len(itemBytes)),
		Typeflag: tar.TypeReg,
		Mode:     0755,
		ModTime:  time.Now(),
	}

	if err := ctx.w.WriteHeader(hdr); err != nil {
		return err
	}

	if _, err := ctx.w.Write(itemBytes); err != nil {
		return err
	}

	return nil
}
