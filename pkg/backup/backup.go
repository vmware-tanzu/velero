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

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"

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
	// Backup takes a backup using the specification in the api.Backup and writes backup and log data
	// to the given writers.
	Backup(backup *api.Backup, backupFile, logFile io.Writer) error
}

// kubernetesBackupper implements Backupper.
type kubernetesBackupper struct {
	dynamicFactory  client.DynamicFactory
	discoveryHelper discovery.Helper
	actions         map[schema.GroupResource]Action
	itemBackupper   itemBackupper
}

var _ Backupper = &kubernetesBackupper{}

// ActionContext contains contextual information for actions.
type ActionContext struct {
	logger *logrus.Logger
}

func (ac ActionContext) infof(msg string, args ...interface{}) {
	ac.logger.Infof(msg, args...)
}

// Action is an actor that performs an operation on an individual item being backed up.
type Action interface {
	// Execute is invoked on an item being backed up. If an error is returned, the Backup is marked as
	// failed.
	Execute(ctx *backupContext, item map[string]interface{}, backupper itemBackupper) error
}

type itemKey struct {
	resource  string
	namespace string
	name      string
}

func (i *itemKey) String() string {
	return fmt.Sprintf("resource=%s,namespace=%s,name=%s", i.resource, i.namespace, i.name)
}

// NewKubernetesBackupper creates a new kubernetesBackupper.
func NewKubernetesBackupper(
	discoveryHelper discovery.Helper,
	dynamicFactory client.DynamicFactory,
	actions map[string]Action,
) (Backupper, error) {
	resolvedActions, err := resolveActions(discoveryHelper, actions)
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
func resolveActions(helper discovery.Helper, actions map[string]Action) (map[schema.GroupResource]Action, error) {
	ret := make(map[schema.GroupResource]Action)

	for resource, action := range actions {
		gvr, _, err := helper.ResourceFor(schema.ParseGroupResource(resource).WithVersion(""))
		if err != nil {
			return nil, err
		}
		ret[gvr.GroupResource()] = action
	}

	return ret, nil
}

// getResourceIncludesExcludes takes the lists of resources to include and exclude, uses the
// discovery helper to resolve them to fully-qualified group-resource names, and returns an
// IncludesExcludes list.
func (ctx *backupContext) getResourceIncludesExcludes(helper discovery.Helper, includes, excludes []string) *collections.IncludesExcludes {
	resources := collections.GenerateIncludesExcludes(
		includes,
		excludes,
		func(item string) string {
			gvr, _, err := helper.ResourceFor(schema.ParseGroupResource(item).WithVersion(""))
			if err != nil {
				ctx.infof("Unable to resolve resource %q: %v", item, err)
				return ""
			}

			gr := gvr.GroupResource()
			return gr.String()
		},
	)

	ctx.infof("Including resources: %v", strings.Join(resources.GetIncludes(), ", "))
	ctx.infof("Excluding resources: %v", strings.Join(resources.GetExcludes(), ", "))

	return resources
}

// getNamespaceIncludesExcludes returns an IncludesExcludes list containing which namespaces to
// include and exclude from the backup.
func getNamespaceIncludesExcludes(backup *api.Backup) *collections.IncludesExcludes {
	return collections.NewIncludesExcludes().Includes(backup.Spec.IncludedNamespaces...).Excludes(backup.Spec.ExcludedNamespaces...)
}

type backupContext struct {
	backup                    *api.Backup
	w                         tarWriter
	logger                    *logrus.Logger
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

	actions map[schema.GroupResource]Action

	// backedUpItems keeps track of items that have been backed up already.
	backedUpItems map[itemKey]struct{}

	dynamicFactory client.DynamicFactory

	discoveryHelper discovery.Helper
}

func (ctx *backupContext) infof(msg string, args ...interface{}) {
	ctx.logger.Infof(msg, args...)
}

// Backup backs up the items specified in the Backup, placing them in a gzip-compressed tar file
// written to backupFile. The finalized api.Backup is written to metadata.
func (kb *kubernetesBackupper) Backup(backup *api.Backup, backupFile, logFile io.Writer) error {
	gzippedData := gzip.NewWriter(backupFile)
	defer gzippedData.Close()

	tw := tar.NewWriter(gzippedData)
	defer tw.Close()

	gzippedLog := gzip.NewWriter(logFile)
	defer gzippedLog.Close()

	var errs []error

	log := logrus.New()
	log.Out = gzippedLog

	ctx := &backupContext{
		backup: backup,
		w:      tw,
		logger: log,
		namespaceIncludesExcludes: getNamespaceIncludesExcludes(backup),
		backedUpItems:             make(map[itemKey]struct{}),
		actions:                   kb.actions,
		dynamicFactory:            kb.dynamicFactory,
		discoveryHelper:           kb.discoveryHelper,
	}

	ctx.infof("Starting backup")

	ctx.resourceIncludesExcludes = ctx.getResourceIncludesExcludes(kb.discoveryHelper, backup.Spec.IncludedResources, backup.Spec.ExcludedResources)

	for _, group := range kb.discoveryHelper.Resources() {
		ctx.infof("Processing group %s", group.GroupVersion)
		if err := kb.backupGroup(ctx, group); err != nil {
			errs = append(errs, err)
		}
	}

	err := kuberrs.NewAggregate(errs)
	if err == nil {
		ctx.infof("Backup completed successfully")
	} else {
		ctx.infof("Backup completed with errors: %v", err)
	}

	return err
}

type tarWriter interface {
	io.Closer
	Write([]byte) (int, error)
	WriteHeader(*tar.Header) error
}

// backupGroup backs up a single API group.
func (kb *kubernetesBackupper) backupGroup(ctx *backupContext, group *metav1.APIResourceList) error {
	var (
		errs []error
		pv   *metav1.APIResource
	)

	processResource := func(resource metav1.APIResource) {
		ctx.infof("Processing resource %s/%s", group.GroupVersion, resource.Name)
		if err := kb.backupResource(ctx, group, resource); err != nil {
			errs = append(errs, err)
		}
	}

	for _, resource := range group.APIResources {
		// do PVs last because if we're also backing up PVCs, we want to backup
		// PVs within the scope of the PVCs (within the PVC action) to allow
		// for hooks to run
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
		return errors.Wrapf(err, "error parsing GroupVersion %s", group.GroupVersion)
	}
	gvr := schema.GroupVersionResource{Group: gv.Group, Version: gv.Version}
	gr := schema.GroupResource{Group: gv.Group, Resource: resource.Name}
	grString := gr.String()

	switch {
	case ctx.backup.Spec.IncludeClusterResources == nil:
		// when IncludeClusterResources == nil (auto), only directly
		// back up cluster-scoped resources if we're doing a full-cluster
		// (all namespaces) backup. Note that in the case of a subset of
		// namespaces being backed up, some related cluster-scoped resources
		// may still be backed up if triggered by a custom action (e.g. PVC->PV).
		if !resource.Namespaced && !ctx.namespaceIncludesExcludes.IncludeEverything() {
			ctx.infof("Skipping resource %s because it's cluster-scoped and only specific namespaces are included in the backup", grString)
			return nil
		}
	case *ctx.backup.Spec.IncludeClusterResources == false:
		if !resource.Namespaced {
			ctx.infof("Skipping resource %s because it's cluster-scoped", grString)
			return nil
		}
	case *ctx.backup.Spec.IncludeClusterResources == true:
		// include the resource, no action required
	}

	if !ctx.resourceIncludesExcludes.ShouldInclude(grString) {
		ctx.infof("Resource %s is excluded", grString)
		return nil
	}

	shouldBackup := func(gr, gr1, gr2 string, backedUp *bool) bool {
		// if it's neither of the specified dupe group-resources, back it up
		if gr != gr1 && gr != gr2 {
			return true
		}

		// if it hasn't been backed up yet, back it up
		if !*backedUp {
			*backedUp = true
			return true
		}

		// else, don't back it up, and log why
		var other string
		switch gr {
		case gr1:
			other = gr2
		case gr2:
			other = gr1
		}

		ctx.infof("Skipping resource %q because it's a duplicate of %q", gr, other)
		return false
	}

	if !shouldBackup(grString, appsDeploymentsResource, extensionsDeploymentsResource, &ctx.deploymentsBackedUp) {
		return nil
	}

	if !shouldBackup(grString, networkingNetworkPoliciesResource, extensionsNetworkPoliciesResource, &ctx.networkPoliciesBackedUp) {
		return nil
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

			obj := unstructured.UnstructuredContent()

			if err := kb.itemBackupper.backupItem(ctx, obj, gr); err != nil {
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
	backupItem(ctx *backupContext, item map[string]interface{}, groupResource schema.GroupResource) error
}

type realItemBackupper struct{}

// backupItem backs up an individual item to tarWriter. The item may be excluded based on the
// namespaces IncludesExcludes list.
func (ib *realItemBackupper) backupItem(ctx *backupContext, item map[string]interface{}, groupResource schema.GroupResource) error {
	name, err := collections.GetString(item, "metadata.name")
	if err != nil {
		return err
	}

	namespace, err := collections.GetString(item, "metadata.namespace")
	// a non-nil error is assumed to be due to a cluster-scoped item
	if err == nil && !ctx.namespaceIncludesExcludes.ShouldInclude(namespace) {
		ctx.infof("Excluding item %s because namespace %s is excluded", name, namespace)
		return nil
	}

	if namespace == "" && ctx.backup.Spec.IncludeClusterResources != nil && *ctx.backup.Spec.IncludeClusterResources == false {
		ctx.infof("Excluding item %s because resource %s is cluster-scoped and IncludeClusterResources is false", name, groupResource.String())
		return nil
	}

	if !ctx.resourceIncludesExcludes.ShouldInclude(groupResource.String()) {
		ctx.infof("Excluding item %s because resource %s is excluded", name, groupResource.String())
		return nil
	}

	key := itemKey{
		resource:  groupResource.String(),
		namespace: namespace,
		name:      name,
	}

	if _, exists := ctx.backedUpItems[key]; exists {
		ctx.infof("Skipping item %s because it's already been backed up.", name)
		return nil
	}
	ctx.backedUpItems[key] = struct{}{}

	// Never save status
	delete(item, "status")

	if action, hasAction := ctx.actions[groupResource]; hasAction {
		ctx.infof("Executing action on %s, ns=%s, name=%s", groupResource.String(), namespace, name)

		if err := action.Execute(ctx, item, ib); err != nil {
			return err
		}
	}

	ctx.infof("Backing up resource=%s, ns=%s, name=%s", groupResource.String(), namespace, name)

	var filePath string
	if namespace != "" {
		filePath = strings.Join([]string{api.NamespaceScopedDir, namespace, groupResource.String(), name + ".json"}, "/")
	} else {
		filePath = strings.Join([]string{api.ClusterScopedDir, groupResource.String(), name + ".json"}, "/")
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

	if err := ctx.w.WriteHeader(hdr); err != nil {
		return errors.WithStack(err)
	}

	if _, err := ctx.w.Write(itemBytes); err != nil {
		return errors.WithStack(err)
	}

	return nil
}
