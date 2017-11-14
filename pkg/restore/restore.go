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

package restore

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"

	"github.com/sirupsen/logrus"

	"k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/sets"
	corev1 "k8s.io/client-go/kubernetes/typed/core/v1"

	api "github.com/heptio/ark/pkg/apis/ark/v1"
	"github.com/heptio/ark/pkg/client"
	"github.com/heptio/ark/pkg/cloudprovider"
	"github.com/heptio/ark/pkg/discovery"
	arkv1client "github.com/heptio/ark/pkg/generated/clientset/versioned/typed/ark/v1"
	"github.com/heptio/ark/pkg/restore/restorers"
	"github.com/heptio/ark/pkg/util/collections"
	"github.com/heptio/ark/pkg/util/kube"
	"github.com/heptio/ark/pkg/util/logging"
)

// Restorer knows how to restore a backup.
type Restorer interface {
	// Restore restores the backup data from backupReader, returning warnings and errors.
	Restore(restore *api.Restore, backup *api.Backup, backupReader io.Reader, logFile io.Writer) (api.RestoreResult, api.RestoreResult)
}

var _ Restorer = &kubernetesRestorer{}

type gvString string
type kindString string

// kubernetesRestorer implements Restorer for restoring into a Kubernetes cluster.
type kubernetesRestorer struct {
	discoveryHelper    discovery.Helper
	dynamicFactory     client.DynamicFactory
	restorers          map[schema.GroupResource]restorers.ResourceRestorer
	backupService      cloudprovider.BackupService
	backupClient       arkv1client.BackupsGetter
	namespaceClient    corev1.NamespaceInterface
	resourcePriorities []string
	fileSystem         FileSystem
	logger             *logrus.Logger
}

// prioritizeResources returns an ordered, fully-resolved list of resources to restore based on
// the provided discovery helper, resource priorities, and included/excluded resources.
func prioritizeResources(helper discovery.Helper, priorities []string, includedResources *collections.IncludesExcludes, logger *logrus.Logger) ([]schema.GroupResource, error) {
	var ret []schema.GroupResource

	// set keeps track of resolved GroupResource names
	set := sets.NewString()

	// start by resolving priorities into GroupResources and adding them to ret
	for _, r := range priorities {
		gvr, _, err := helper.ResourceFor(schema.ParseGroupResource(r).WithVersion(""))
		if err != nil {
			return nil, err
		}
		gr := gvr.GroupResource()

		if !includedResources.ShouldInclude(gr.String()) {
			logger.WithField("groupResource", gr).Info("Not including resource")
			continue
		}
		ret = append(ret, gr)
		set.Insert(gr.String())
	}

	// go through everything we got from discovery and add anything not in "set" to byName
	var byName []schema.GroupResource
	for _, resourceGroup := range helper.Resources() {
		// will be something like storage.k8s.io/v1
		groupVersion, err := schema.ParseGroupVersion(resourceGroup.GroupVersion)
		if err != nil {
			return nil, err
		}

		for _, resource := range resourceGroup.APIResources {
			gr := groupVersion.WithResource(resource.Name).GroupResource()

			if !includedResources.ShouldInclude(gr.String()) {
				logger.WithField("groupResource", gr.String()).Debug("Not including resource")
				continue
			}

			if !set.Has(gr.String()) {
				byName = append(byName, gr)
			}
		}
	}

	// sort byName by name
	sort.Slice(byName, func(i, j int) bool {
		return byName[i].String() < byName[j].String()
	})

	// combine prioritized with by-name
	ret = append(ret, byName...)

	return ret, nil
}

// NewKubernetesRestorer creates a new kubernetesRestorer.
func NewKubernetesRestorer(
	discoveryHelper discovery.Helper,
	dynamicFactory client.DynamicFactory,
	customRestorers map[string]restorers.ResourceRestorer,
	backupService cloudprovider.BackupService,
	resourcePriorities []string,
	backupClient arkv1client.BackupsGetter,
	namespaceClient corev1.NamespaceInterface,
	logger *logrus.Logger,
) (Restorer, error) {
	r := make(map[schema.GroupResource]restorers.ResourceRestorer)
	for gr, restorer := range customRestorers {
		gvr, _, err := discoveryHelper.ResourceFor(schema.ParseGroupResource(gr).WithVersion(""))
		if err != nil {
			return nil, err
		}
		r[gvr.GroupResource()] = restorer
	}

	return &kubernetesRestorer{
		discoveryHelper:    discoveryHelper,
		dynamicFactory:     dynamicFactory,
		restorers:          r,
		backupService:      backupService,
		backupClient:       backupClient,
		namespaceClient:    namespaceClient,
		resourcePriorities: resourcePriorities,
		fileSystem:         &osFileSystem{},
		logger:             logger,
	}, nil
}

// Restore executes a restore into the target Kubernetes cluster according to the restore spec
// and using data from the provided backup/backup reader. Returns a warnings and errors RestoreResult,
// respectively, summarizing info about the restore.
func (kr *kubernetesRestorer) Restore(restore *api.Restore, backup *api.Backup, backupReader io.Reader, logFile io.Writer) (api.RestoreResult, api.RestoreResult) {
	// metav1.LabelSelectorAsSelector converts a nil LabelSelector to a
	// Nothing Selector, i.e. a selector that matches nothing. We want
	// a selector that matches everything. This can be accomplished by
	// passing a non-nil empty LabelSelector.
	ls := restore.Spec.LabelSelector
	if ls == nil {
		ls = &metav1.LabelSelector{}
	}

	selector, err := metav1.LabelSelectorAsSelector(ls)
	if err != nil {
		return api.RestoreResult{}, api.RestoreResult{Ark: []string{err.Error()}}
	}

	// get resource includes-excludes
	resourceIncludesExcludes := collections.GenerateIncludesExcludes(
		restore.Spec.IncludedResources,
		restore.Spec.ExcludedResources,
		func(item string) string {
			gvr, _, err := kr.discoveryHelper.ResourceFor(schema.ParseGroupResource(item).WithVersion(""))
			if err != nil {
				kr.logger.WithError(err).WithField("resource", item).Error("Unable to resolve resource")
				return ""
			}

			gr := gvr.GroupResource()
			return gr.String()
		},
	)

	prioritizedResources, err := prioritizeResources(kr.discoveryHelper, kr.resourcePriorities, resourceIncludesExcludes, kr.logger)
	if err != nil {
		return api.RestoreResult{}, api.RestoreResult{Ark: []string{err.Error()}}
	}

	gzippedLog := gzip.NewWriter(logFile)
	defer gzippedLog.Close()

	log := logrus.New()
	log.Out = gzippedLog
	log.Hooks.Add(&logging.ErrorLocationHook{})
	log.Hooks.Add(&logging.LogLocationHook{})

	ctx := &context{
		backup:               backup,
		backupReader:         backupReader,
		restore:              restore,
		prioritizedResources: prioritizedResources,
		selector:             selector,
		logger:               log,
		dynamicFactory:       kr.dynamicFactory,
		fileSystem:           kr.fileSystem,
		namespaceClient:      kr.namespaceClient,
		restorers:            kr.restorers,
	}

	return ctx.execute()
}

type context struct {
	backup               *api.Backup
	backupReader         io.Reader
	restore              *api.Restore
	prioritizedResources []schema.GroupResource
	selector             labels.Selector
	logger               *logrus.Logger
	dynamicFactory       client.DynamicFactory
	fileSystem           FileSystem
	namespaceClient      corev1.NamespaceInterface
	restorers            map[schema.GroupResource]restorers.ResourceRestorer
}

func (ctx *context) infof(msg string, args ...interface{}) {
	ctx.logger.Infof(msg, args...)
}

func (ctx *context) execute() (api.RestoreResult, api.RestoreResult) {
	ctx.infof("Starting restore of backup %s", kube.NamespaceAndName(ctx.backup))

	dir, err := ctx.unzipAndExtractBackup(ctx.backupReader)
	if err != nil {
		ctx.infof("error unzipping and extracting: %v", err)
		return api.RestoreResult{}, api.RestoreResult{Ark: []string{err.Error()}}
	}
	defer ctx.fileSystem.RemoveAll(dir)

	return ctx.restoreFromDir(dir)
}

// restoreFromDir executes a restore based on backup data contained within a local
// directory.
func (ctx *context) restoreFromDir(dir string) (api.RestoreResult, api.RestoreResult) {
	warnings, errs := api.RestoreResult{}, api.RestoreResult{}

	namespaceFilter := collections.NewIncludesExcludes().Includes(ctx.restore.Spec.IncludedNamespaces...).Excludes(ctx.restore.Spec.ExcludedNamespaces...)

	// Make sure the top level "resources" dir exists:
	resourcesDir := filepath.Join(dir, api.ResourcesDir)
	rde, err := ctx.fileSystem.DirExists(resourcesDir)
	if err != nil {
		addArkError(&errs, err)
		return warnings, errs
	}
	if !rde {
		addArkError(&errs, errors.New("backup does not contain top level resources directory"))
	}

	resourceDirs, err := ctx.fileSystem.ReadDir(resourcesDir)
	if err != nil {
		addArkError(&errs, err)
		return warnings, errs
	}

	resourceDirsMap := make(map[string]os.FileInfo)

	for _, rscDir := range resourceDirs {
		rscName := rscDir.Name()
		resourceDirsMap[rscName] = rscDir
	}

	for _, resource := range ctx.prioritizedResources {
		rscDir := resourceDirsMap[resource.String()]
		if rscDir == nil {
			continue
		}

		resourcePath := filepath.Join(resourcesDir, rscDir.Name())

		clusterSubDir := filepath.Join(resourcePath, api.ClusterScopedDir)
		clusterSubDirExists, err := ctx.fileSystem.DirExists(clusterSubDir)
		if err != nil {
			addArkError(&errs, err)
			return warnings, errs
		}
		if clusterSubDirExists {
			w, e := ctx.restoreResource(resource.String(), "", clusterSubDir)
			merge(&warnings, &w)
			merge(&errs, &e)
			continue
		}

		nsSubDir := filepath.Join(resourcePath, api.NamespaceScopedDir)
		nsSubDirExists, err := ctx.fileSystem.DirExists(nsSubDir)
		if err != nil {
			addArkError(&errs, err)
			return warnings, errs
		}
		if !nsSubDirExists {
			continue
		}

		nsDirs, err := ctx.fileSystem.ReadDir(nsSubDir)
		if err != nil {
			addArkError(&errs, err)
			return warnings, errs
		}

		for _, nsDir := range nsDirs {
			if !nsDir.IsDir() {
				continue
			}
			nsName := nsDir.Name()
			nsPath := filepath.Join(nsSubDir, nsName)

			if !namespaceFilter.ShouldInclude(nsName) {
				ctx.infof("Skipping namespace %s", nsName)
				continue
			}

			// fetch mapped NS name
			mappedNsName := nsName
			if target, ok := ctx.restore.Spec.NamespaceMapping[nsName]; ok {
				mappedNsName = target
			}

			// ensure namespace exists
			ns := &v1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: mappedNsName,
				},
			}
			if _, err := kube.EnsureNamespaceExists(ns, ctx.namespaceClient); err != nil {
				addArkError(&errs, err)
				continue
			}

			w, e := ctx.restoreResource(resource.String(), mappedNsName, nsPath)
			merge(&warnings, &w)
			merge(&errs, &e)
		}
	}

	return warnings, errs
}

// merge combines two RestoreResult objects into one
// by appending the corresponding lists to one another.
func merge(a, b *api.RestoreResult) {
	a.Cluster = append(a.Cluster, b.Cluster...)
	a.Ark = append(a.Ark, b.Ark...)
	for k, v := range b.Namespaces {
		if a.Namespaces == nil {
			a.Namespaces = make(map[string][]string)
		}
		a.Namespaces[k] = append(a.Namespaces[k], v...)
	}
}

// addArkError appends an error to the provided RestoreResult's Ark list.
func addArkError(r *api.RestoreResult, err error) {
	r.Ark = append(r.Ark, err.Error())
}

// addToResult appends an error to the provided RestoreResult, either within
// the cluster-scoped list (if ns == "") or within the provided namespace's
// entry.
func addToResult(r *api.RestoreResult, ns string, e error) {
	if ns == "" {
		r.Cluster = append(r.Cluster, e.Error())
	} else {
		if r.Namespaces == nil {
			r.Namespaces = make(map[string][]string)
		}
		r.Namespaces[ns] = append(r.Namespaces[ns], e.Error())
	}
}

// restoreResource restores the specified cluster or namespace scoped resource. If namespace is
// empty we are restoring a cluster level resource, otherwise into the specified namespace.
func (ctx *context) restoreResource(resource, namespace, resourcePath string) (api.RestoreResult, api.RestoreResult) {
	warnings, errs := api.RestoreResult{}, api.RestoreResult{}

	if ctx.restore.Spec.IncludeClusterResources != nil && !*ctx.restore.Spec.IncludeClusterResources && namespace == "" {
		ctx.infof("Skipping resource %s because it's cluster-scoped", resource)
		return warnings, errs
	}

	if namespace != "" {
		ctx.infof("Restoring resource '%s' into namespace '%s' from: %s", resource, namespace, resourcePath)
	} else {
		ctx.infof("Restoring cluster level resource '%s' from: %s", resource, resourcePath)
	}

	files, err := ctx.fileSystem.ReadDir(resourcePath)
	if err != nil {
		addToResult(&errs, namespace, fmt.Errorf("error reading %q resource directory: %v", resource, err))
		return warnings, errs
	}
	if len(files) == 0 {
		return warnings, errs
	}

	var (
		resourceClient client.Dynamic
		restorer       restorers.ResourceRestorer
		waiter         *resourceWaiter
		groupResource  = schema.ParseGroupResource(resource)
	)

	for _, file := range files {
		fullPath := filepath.Join(resourcePath, file.Name())
		obj, err := ctx.unmarshal(fullPath)
		if err != nil {
			addToResult(&errs, namespace, fmt.Errorf("error decoding %q: %v", fullPath, err))
			continue
		}

		if !ctx.selector.Matches(labels.Set(obj.GetLabels())) {
			continue
		}

		if restorer == nil {
			// initialize client & restorer for this Resource. we need
			// metadata from an object to do this.
			ctx.infof("Getting client for %v", obj.GroupVersionKind())

			resource := metav1.APIResource{
				Namespaced: len(namespace) > 0,
				Name:       groupResource.Resource,
			}

			var err error
			resourceClient, err = ctx.dynamicFactory.ClientForGroupVersionResource(obj.GroupVersionKind().GroupVersion(), resource, namespace)
			if err != nil {
				addArkError(&errs, fmt.Errorf("error getting resource client for namespace %q, resource %q: %v", namespace, &groupResource, err))
				return warnings, errs
			}

			restorer = ctx.restorers[groupResource]
			if restorer == nil {
				ctx.infof("Using default restorer for %v", &groupResource)
				restorer = restorers.NewBasicRestorer(true)
			} else {
				ctx.infof("Using custom restorer for %v", &groupResource)
			}

			if restorer.Wait() {
				itmWatch, err := resourceClient.Watch(metav1.ListOptions{})
				if err != nil {
					addArkError(&errs, fmt.Errorf("error watching for namespace %q, resource %q: %v", namespace, &groupResource, err))
					return warnings, errs
				}
				watchChan := itmWatch.ResultChan()
				defer itmWatch.Stop()

				waiter = newResourceWaiter(watchChan, restorer.Ready)
			}
		}

		if !restorer.Handles(obj, ctx.restore) {
			continue
		}

		if hasControllerOwner(obj.GetOwnerReferences()) {
			ctx.infof("%s/%s has a controller owner - skipping", obj.GetNamespace(), obj.GetName())
			continue
		}

		preparedObj, warning, err := restorer.Prepare(obj, ctx.restore, ctx.backup)
		if warning != nil {
			addToResult(&warnings, namespace, fmt.Errorf("warning preparing %s: %v", fullPath, warning))
		}
		if err != nil {
			addToResult(&errs, namespace, fmt.Errorf("error preparing %s: %v", fullPath, err))
			continue
		}

		unstructuredObj, ok := preparedObj.(*unstructured.Unstructured)
		if !ok {
			addToResult(&errs, namespace, fmt.Errorf("%s: unexpected type %T", fullPath, preparedObj))
			continue
		}

		// necessary because we may have remapped the namespace
		unstructuredObj.SetNamespace(namespace)

		// add an ark-restore label to each resource for easy ID
		addLabel(unstructuredObj, api.RestoreLabelKey, ctx.restore.Name)

		ctx.infof("Restoring %s: %v", obj.GroupVersionKind().Kind, unstructuredObj.GetName())
		_, err = resourceClient.Create(unstructuredObj)
		if apierrors.IsAlreadyExists(err) {
			addToResult(&warnings, namespace, err)
			continue
		}
		if err != nil {
			ctx.infof("error restoring %s: %v", unstructuredObj.GetName(), err)
			addToResult(&errs, namespace, fmt.Errorf("error restoring %s: %v", fullPath, err))
			continue
		}

		if waiter != nil {
			waiter.RegisterItem(unstructuredObj.GetName())
		}
	}

	if waiter != nil {
		if err := waiter.Wait(); err != nil {
			addArkError(&errs, fmt.Errorf("error waiting for all %v resources to be created in namespace %s: %v", &groupResource, namespace, err))
		}
	}

	return warnings, errs
}

// addLabel applies the specified key/value to an object as a label.
func addLabel(obj *unstructured.Unstructured, key string, val string) {
	labels := obj.GetLabels()

	if labels == nil {
		labels = make(map[string]string)
	}

	labels[key] = val

	obj.SetLabels(labels)
}

// hasControllerOwner returns whether or not an object has a controller
// owner ref. Used to identify whether or not an object should be explicitly
// recreated during a restore.
func hasControllerOwner(refs []metav1.OwnerReference) bool {
	for _, ref := range refs {
		if ref.Controller != nil && *ref.Controller {
			return true
		}
	}
	return false
}

// unmarshal reads the specified file, unmarshals the JSON contained within it
// and returns an Unstructured object.
func (ctx *context) unmarshal(filePath string) (*unstructured.Unstructured, error) {
	var obj unstructured.Unstructured

	bytes, err := ctx.fileSystem.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	err = json.Unmarshal(bytes, &obj)
	if err != nil {
		return nil, err
	}

	return &obj, nil
}

// unzipAndExtractBackup extracts a reader on a gzipped tarball to a local temp directory
func (ctx *context) unzipAndExtractBackup(src io.Reader) (string, error) {
	gzr, err := gzip.NewReader(src)
	if err != nil {
		ctx.infof("error creating gzip reader: %v", err)
		return "", err
	}
	defer gzr.Close()

	return ctx.readBackup(tar.NewReader(gzr))
}

// readBackup extracts a tar reader to a local directory/file tree within a
// temp directory.
func (ctx *context) readBackup(tarRdr *tar.Reader) (string, error) {
	dir, err := ctx.fileSystem.TempDir("", "")
	if err != nil {
		ctx.infof("error creating temp dir: %v", err)
		return "", err
	}

	for {
		header, err := tarRdr.Next()

		if err == io.EOF {
			break
		}
		if err != nil {
			ctx.infof("error reading tar: %v", err)
			return "", err
		}

		target := filepath.Join(dir, header.Name)

		switch header.Typeflag {
		case tar.TypeDir:
			err := ctx.fileSystem.MkdirAll(target, header.FileInfo().Mode())
			if err != nil {
				ctx.infof("mkdirall error: %v", err)
				return "", err
			}

		case tar.TypeReg:
			// make sure we have the directory created
			err := ctx.fileSystem.MkdirAll(filepath.Dir(target), header.FileInfo().Mode())
			if err != nil {
				ctx.infof("mkdirall error: %v", err)
				return "", err
			}

			// create the file
			file, err := ctx.fileSystem.Create(target)
			if err != nil {
				return "", err
			}
			defer file.Close()

			if _, err := io.Copy(file, tarRdr); err != nil {
				ctx.infof("error copying: %v", err)
				return "", err
			}
		}
	}

	return dir, nil
}
