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
	Execute(ctx ActionContext, item map[string]interface{}, backup *api.Backup) error
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
		gr, err := helper.ResolveGroupResource(resource)
		if err != nil {
			return nil, err
		}
		ret[gr] = action
	}

	return ret, nil
}

// getResourceIncludesExcludes takes the lists of resources to include and exclude from the
// backup, uses the discovery helper to resolve them to fully-qualified group-resource names, and returns
// an IncludesExcludes list.
func (ctx *backupContext) getResourceIncludesExcludes(helper discovery.Helper, includes, excludes []string) *collections.IncludesExcludes {
	return collections.GenerateIncludesExcludes(
		includes,
		excludes,
		func(item string) string {
			gr, err := helper.ResolveGroupResource(item)
			if err != nil {
				ctx.infof("Unable to resolve resource %q: %v", item, err)
				return ""
			}

			return gr.String()
		},
	)
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
	var errs []error

	for _, resource := range group.APIResources {
		ctx.infof("Processing resource %s/%s", group.GroupVersion, resource.Name)
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
		return errors.Wrapf(err, "error parsing GroupVersion %s", group.GroupVersion)
	}
	gvr := schema.GroupVersionResource{Group: gv.Group, Version: gv.Version}
	gr := schema.GroupResource{Group: gv.Group, Resource: resource.Name}
	grString := gr.String()

	if !ctx.resourceIncludesExcludes.ShouldInclude(grString) {
		ctx.infof("Resource %s is excluded", grString)
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
			ctx.infof("Skipping resource %q because it's a duplicate of %q", grString, other)
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
			ctx.infof("Skipping resource %q because it's a duplicate of %q", grString, other)
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
			return errors.WithStack(err)
		}

		// do the backup
		items, err := meta.ExtractList(unstructuredList)
		if err != nil {
			return errors.WithStack(err)
		}

		action := kb.actions[gr]

		for _, item := range items {
			unstructured, ok := item.(runtime.Unstructured)
			if !ok {
				errs = append(errs, errors.Errorf("unexpected type %T", item))
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
			ctx.infof("Excluding item %s because namespace %s is excluded", name, namespace)
			return nil
		}
	}

	if action != nil {
		ctx.infof("Executing action on %s, ns=%s, name=%s", groupResource, namespace, name)

		actionCtx := ActionContext{logger: ctx.logger}
		if err := action.Execute(actionCtx, item, ctx.backup); err != nil {
			return err
		}
	}

	ctx.infof("Backing up resource=%s, ns=%s, name=%s", groupResource, namespace, name)

	var filePath string
	if namespace != "" {
		filePath = strings.Join([]string{api.NamespaceScopedDir, namespace, groupResource, name + ".json"}, "/")
	} else {
		filePath = strings.Join([]string{api.ClusterScopedDir, groupResource, name + ".json"}, "/")
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
