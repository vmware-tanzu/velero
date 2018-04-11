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

package restore

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"sort"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"

	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/sets"
	corev1 "k8s.io/client-go/kubernetes/typed/core/v1"

	api "github.com/heptio/ark/pkg/apis/ark/v1"
	"github.com/heptio/ark/pkg/client"
	"github.com/heptio/ark/pkg/cloudprovider"
	"github.com/heptio/ark/pkg/discovery"
	arkv1client "github.com/heptio/ark/pkg/generated/clientset/versioned/typed/ark/v1"
	"github.com/heptio/ark/pkg/util/boolptr"
	"github.com/heptio/ark/pkg/util/collections"
	"github.com/heptio/ark/pkg/util/kube"
	"github.com/heptio/ark/pkg/util/logging"
)

// Restorer knows how to restore a backup.
type Restorer interface {
	// Restore restores the backup data from backupReader, returning warnings and errors.
	Restore(restore *api.Restore, backup *api.Backup, backupReader io.Reader, logFile io.Writer, actions []ItemAction) (api.RestoreResult, api.RestoreResult)
}

type gvString string
type kindString string

// kubernetesRestorer implements Restorer for restoring into a Kubernetes cluster.
type kubernetesRestorer struct {
	discoveryHelper    discovery.Helper
	dynamicFactory     client.DynamicFactory
	backupService      cloudprovider.BackupService
	snapshotService    cloudprovider.SnapshotService
	backupClient       arkv1client.BackupsGetter
	namespaceClient    corev1.NamespaceInterface
	resourcePriorities []string
	fileSystem         FileSystem
	logger             logrus.FieldLogger
}

// prioritizeResources returns an ordered, fully-resolved list of resources to restore based on
// the provided discovery helper, resource priorities, and included/excluded resources.
func prioritizeResources(helper discovery.Helper, priorities []string, includedResources *collections.IncludesExcludes, logger logrus.FieldLogger) ([]schema.GroupResource, error) {
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
				logger.WithField("groupResource", gr.String()).Info("Not including resource")
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
	backupService cloudprovider.BackupService,
	snapshotService cloudprovider.SnapshotService,
	resourcePriorities []string,
	backupClient arkv1client.BackupsGetter,
	namespaceClient corev1.NamespaceInterface,
	logger logrus.FieldLogger,
) (Restorer, error) {
	return &kubernetesRestorer{
		discoveryHelper:    discoveryHelper,
		dynamicFactory:     dynamicFactory,
		backupService:      backupService,
		snapshotService:    snapshotService,
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
func (kr *kubernetesRestorer) Restore(restore *api.Restore, backup *api.Backup, backupReader io.Reader, logFile io.Writer, actions []ItemAction) (api.RestoreResult, api.RestoreResult) {
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

	gzippedLog := gzip.NewWriter(logFile)
	defer gzippedLog.Close()

	log := logrus.New()
	log.Out = gzippedLog
	log.Hooks.Add(&logging.ErrorLocationHook{})
	log.Hooks.Add(&logging.LogLocationHook{})

	// get resource includes-excludes
	resourceIncludesExcludes := getResourceIncludesExcludes(kr.discoveryHelper, restore.Spec.IncludedResources, restore.Spec.ExcludedResources)
	prioritizedResources, err := prioritizeResources(kr.discoveryHelper, kr.resourcePriorities, resourceIncludesExcludes, log)
	if err != nil {
		return api.RestoreResult{}, api.RestoreResult{Ark: []string{err.Error()}}
	}

	resolvedActions, err := resolveActions(actions, kr.discoveryHelper)
	if err != nil {
		return api.RestoreResult{}, api.RestoreResult{Ark: []string{err.Error()}}
	}

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
		actions:              resolvedActions,
		snapshotService:      kr.snapshotService,
		waitForPVs:           true,
	}

	return ctx.execute()
}

// getResourceIncludesExcludes takes the lists of resources to include and exclude, uses the
// discovery helper to resolve them to fully-qualified group-resource names, and returns an
// IncludesExcludes list.
func getResourceIncludesExcludes(helper discovery.Helper, includes, excludes []string) *collections.IncludesExcludes {
	resources := collections.GenerateIncludesExcludes(
		includes,
		excludes,
		func(item string) string {
			gvr, _, err := helper.ResourceFor(schema.ParseGroupResource(item).WithVersion(""))
			if err != nil {
				return ""
			}

			gr := gvr.GroupResource()
			return gr.String()
		},
	)

	return resources
}

type resolvedAction struct {
	ItemAction

	resourceIncludesExcludes  *collections.IncludesExcludes
	namespaceIncludesExcludes *collections.IncludesExcludes
	selector                  labels.Selector
}

func resolveActions(actions []ItemAction, helper discovery.Helper) ([]resolvedAction, error) {
	var resolved []resolvedAction

	for _, action := range actions {
		resourceSelector, err := action.AppliesTo()
		if err != nil {
			return nil, err
		}

		resources := getResourceIncludesExcludes(helper, resourceSelector.IncludedResources, resourceSelector.ExcludedResources)
		namespaces := collections.NewIncludesExcludes().Includes(resourceSelector.IncludedNamespaces...).Excludes(resourceSelector.ExcludedNamespaces...)

		selector := labels.Everything()
		if resourceSelector.LabelSelector != "" {
			if selector, err = labels.Parse(resourceSelector.LabelSelector); err != nil {
				return nil, err
			}
		}

		res := resolvedAction{
			ItemAction:                action,
			resourceIncludesExcludes:  resources,
			namespaceIncludesExcludes: namespaces,
			selector:                  selector,
		}

		resolved = append(resolved, res)
	}

	return resolved, nil
}

type context struct {
	backup               *api.Backup
	backupReader         io.Reader
	restore              *api.Restore
	prioritizedResources []schema.GroupResource
	selector             labels.Selector
	logger               logrus.FieldLogger
	dynamicFactory       client.DynamicFactory
	fileSystem           FileSystem
	namespaceClient      corev1.NamespaceInterface
	actions              []resolvedAction
	snapshotService      cloudprovider.SnapshotService
	waitForPVs           bool
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

	namespaceFilter := collections.NewIncludesExcludes().
		Includes(ctx.restore.Spec.IncludedNamespaces...).
		Excludes(ctx.restore.Spec.ExcludedNamespaces...)

	// Make sure the top level "resources" dir exists:
	resourcesDir := filepath.Join(dir, api.ResourcesDir)
	rde, err := ctx.fileSystem.DirExists(resourcesDir)
	if err != nil {
		addArkError(&errs, err)
		return warnings, errs
	}
	if !rde {
		addArkError(&errs, errors.New("backup does not contain top level resources directory"))
		return warnings, errs
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

	existingNamespaces := sets.NewString()

	for _, resource := range ctx.prioritizedResources {
		// we don't want to explicitly restore namespace API objs because we'll handle
		// them as a special case prior to restoring anything into them
		if resource.Group == "" && resource.Resource == "namespaces" {
			continue
		}

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

			// if we don't know whether this namespace exists yet, attempt to create
			// it in order to ensure it exists. Try to get it from the backup tarball
			// (in order to get any backed-up metadata), but if we don't find it there,
			// create a blank one.
			if !existingNamespaces.Has(mappedNsName) {
				logger := ctx.logger.WithField("namespace", nsName)
				ns := getNamespace(logger, filepath.Join(dir, api.ResourcesDir, "namespaces", api.ClusterScopedDir, nsName+".json"), mappedNsName)
				if _, err := kube.EnsureNamespaceExists(ns, ctx.namespaceClient); err != nil {
					addArkError(&errs, err)
					continue
				}

				// keep track of namespaces that we know exist so we don't
				// have to try to create them multiple times
				existingNamespaces.Insert(mappedNsName)
			}

			w, e := ctx.restoreResource(resource.String(), mappedNsName, nsPath)
			merge(&warnings, &w)
			merge(&errs, &e)
		}
	}

	return warnings, errs
}

// getNamespace returns a namespace API object that we should attempt to
// create before restoring anything into it. It will come from the backup
// tarball if it exists, else will be a new one. If from the tarball, it
// will retain its labels, annotations, and spec.
func getNamespace(logger logrus.FieldLogger, path, remappedName string) *v1.Namespace {
	var nsBytes []byte
	var err error

	if nsBytes, err = ioutil.ReadFile(path); err != nil {
		return &v1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: remappedName,
			},
		}
	}

	var backupNS v1.Namespace
	if err := json.Unmarshal(nsBytes, &backupNS); err != nil {
		logger.Warnf("Error unmarshalling namespace from backup, creating new one.")
		return &v1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: remappedName,
			},
		}
	}

	return &v1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name:        remappedName,
			Labels:      backupNS.Labels,
			Annotations: backupNS.Annotations,
		},
		Spec: backupNS.Spec,
	}
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
		resourceClient    client.Dynamic
		waiter            *resourceWaiter
		groupResource     = schema.ParseGroupResource(resource)
		applicableActions []resolvedAction
	)

	// pre-filter the actions based on namespace & resource includes/excludes since
	// these will be the same for all items being restored below
	for _, action := range ctx.actions {
		if !action.resourceIncludesExcludes.ShouldInclude(groupResource.String()) {
			continue
		}

		if namespace != "" && !action.namespaceIncludesExcludes.ShouldInclude(namespace) {
			continue
		}

		applicableActions = append(applicableActions, action)
	}

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

		if hasControllerOwner(obj.GetOwnerReferences()) {
			ctx.infof("%s/%s has a controller owner - skipping", obj.GetNamespace(), obj.GetName())
			continue
		}

		if resourceClient == nil {
			// initialize client for this Resource. we need
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
		}

		if groupResource.Group == "" && groupResource.Resource == "persistentvolumes" {
			// restore the PV from snapshot (if applicable)
			updatedObj, err := ctx.executePVAction(obj)
			if err != nil {
				addToResult(&errs, namespace, fmt.Errorf("error executing PVAction for %s: %v", fullPath, err))
				continue
			}
			obj = updatedObj

			// wait for the PV to be ready
			if ctx.waitForPVs {
				pvWatch, err := resourceClient.Watch(metav1.ListOptions{})
				if err != nil {
					addToResult(&errs, namespace, fmt.Errorf("error watching for namespace %q, resource %q: %v", namespace, &groupResource, err))
					return warnings, errs
				}

				waiter = newResourceWaiter(pvWatch, isPVReady)
				defer waiter.Stop()
			}
		}

		for _, action := range applicableActions {
			if !action.selector.Matches(labels.Set(obj.GetLabels())) {
				continue
			}

			ctx.infof("Executing item action for %v", &groupResource)

			if logSetter, ok := action.ItemAction.(logging.LogSetter); ok {
				logSetter.SetLog(ctx.logger)
			}

			updatedObj, warning, err := action.Execute(obj, ctx.restore)
			if warning != nil {
				addToResult(&warnings, namespace, fmt.Errorf("warning preparing %s: %v", fullPath, warning))
			}
			if err != nil {
				addToResult(&errs, namespace, fmt.Errorf("error preparing %s: %v", fullPath, err))
				continue
			}

			unstructuredObj, ok := updatedObj.(*unstructured.Unstructured)
			if !ok {
				addToResult(&errs, namespace, fmt.Errorf("%s: unexpected type %T", fullPath, updatedObj))
				continue
			}

			obj = unstructuredObj
		}

		// clear out non-core metadata fields & status
		if obj, err = resetMetadataAndStatus(obj); err != nil {
			addToResult(&errs, namespace, err)
			continue
		}

		// necessary because we may have remapped the namespace
		// if the namespace is blank, don't create the key
		if namespace != "" {
			obj.SetNamespace(namespace)
		}

		// add an ark-restore label to each resource for easy ID
		addLabel(obj, api.RestoreLabelKey, ctx.restore.Name)

		ctx.infof("Restoring %s: %v", obj.GroupVersionKind().Kind, obj.GetName())
		_, restoreErr := resourceClient.Create(obj)
		if apierrors.IsAlreadyExists(restoreErr) {
			equal := false
			if fromCluster, err := resourceClient.Get(obj.GetName(), metav1.GetOptions{}); err == nil {
				equal, err = objectsAreEqual(fromCluster, obj)
				// Log any errors trying to check equality
				if err != nil {
					ctx.infof("error checking %s against cluster: %v", obj.GetName(), err)
				}
			} else {
				ctx.infof("Error retrieving cluster version of %s: %v", obj.GetName(), err)
			}
			if !equal {
				e := errors.Errorf("not restored: %s and is different from backed up version.", restoreErr)
				addToResult(&warnings, namespace, e)
			}
			continue
		}
		// Error was something other than an AlreadyExists
		if restoreErr != nil {
			ctx.infof("error restoring %s: %v", obj.GetName(), err)
			addToResult(&errs, namespace, fmt.Errorf("error restoring %s: %v", fullPath, restoreErr))
			continue
		}

		if waiter != nil {
			waiter.RegisterItem(obj.GetName())
		}
	}

	if waiter != nil {
		if err := waiter.Wait(); err != nil {
			addArkError(&errs, fmt.Errorf("error waiting for all %v resources to be created in namespace %s: %v", &groupResource, namespace, err))
		}
	}

	return warnings, errs
}

func (ctx *context) executePVAction(obj *unstructured.Unstructured) (*unstructured.Unstructured, error) {
	pvName := obj.GetName()
	if pvName == "" {
		return nil, errors.New("PersistentVolume is missing its name")
	}

	spec, err := collections.GetMap(obj.UnstructuredContent(), "spec")
	if err != nil {
		return nil, err
	}

	delete(spec, "claimRef")
	delete(spec, "storageClassName")

	if boolptr.IsSetToFalse(ctx.backup.Spec.SnapshotVolumes) {
		// The backup had snapshots disabled, so we can return early
		return obj, nil
	}

	if boolptr.IsSetToFalse(ctx.restore.Spec.RestorePVs) {
		// The restore has pv restores disabled, so we can return early
		return obj, nil
	}

	// If we can't find a snapshot record for this particular PV, it most likely wasn't a PV that Ark
	// could snapshot, so return early instead of trying to restore from a snapshot.
	backupInfo, found := ctx.backup.Status.VolumeBackups[pvName]
	if !found {
		return obj, nil
	}

	// Past this point, we expect to be doing a restore

	if ctx.snapshotService == nil {
		return nil, errors.New("you must configure a persistentVolumeProvider to restore PersistentVolumes from snapshots")
	}

	ctx.infof("restoring PersistentVolume %s from SnapshotID %s", pvName, backupInfo.SnapshotID)
	volumeID, err := ctx.snapshotService.CreateVolumeFromSnapshot(backupInfo.SnapshotID, backupInfo.Type, backupInfo.AvailabilityZone, backupInfo.Iops)
	if err != nil {
		return nil, err
	}
	ctx.infof("successfully restored PersistentVolume %s from snapshot", pvName)

	updated1, err := ctx.snapshotService.SetVolumeID(obj, volumeID)
	if err != nil {
		return nil, err
	}

	updated2, ok := updated1.(*unstructured.Unstructured)
	if !ok {
		return nil, errors.Errorf("unexpected type %T", updated1)
	}

	return updated2, nil
}

// objectsAreEqual takes two unstructured objects and checks for equality.
// The fromCluster object is mutated to remove any insubstantial runtime
// information that won't match
func objectsAreEqual(fromCluster, fromBackup *unstructured.Unstructured) (bool, error) {
	// Remove insubstantial metadata
	fromCluster, err := resetMetadataAndStatus(fromCluster)
	if err != nil {
		return false, err
	}

	// We know the cluster won't have the restore name label, so
	// copy it over from the backup
	restoreName := fromBackup.GetLabels()[api.RestoreLabelKey]
	addLabel(fromCluster, api.RestoreLabelKey, restoreName)

	// If there are no specific actions needed based on the type, simply check for equality.
	return equality.Semantic.DeepEqual(fromBackup, fromCluster), nil
}

func isPVReady(obj runtime.Unstructured) bool {
	phase, err := collections.GetString(obj.UnstructuredContent(), "status.phase")
	if err != nil {
		return false
	}

	return phase == string(v1.VolumeAvailable)
}

func resetMetadataAndStatus(obj *unstructured.Unstructured) (*unstructured.Unstructured, error) {
	metadata, err := collections.GetMap(obj.UnstructuredContent(), "metadata")
	if err != nil {
		return nil, err
	}

	for k := range metadata {
		switch k {
		case "name", "namespace", "labels", "annotations":
		default:
			delete(metadata, k)
		}
	}

	// this should never be backed up anyway, but remove it just
	// in case.
	delete(obj.UnstructuredContent(), "status")

	return obj, nil
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
