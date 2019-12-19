/*
Copyright 2017, 2019 the Velero contributors.

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
	go_context "context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"path/filepath"
	"sort"
	"strings"
	"time"

	uuid "github.com/gofrs/uuid"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	kubeerrs "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/wait"
	corev1 "k8s.io/client-go/kubernetes/typed/core/v1"

	velerov1api "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	"github.com/vmware-tanzu/velero/pkg/archive"
	"github.com/vmware-tanzu/velero/pkg/client"
	"github.com/vmware-tanzu/velero/pkg/discovery"
	listers "github.com/vmware-tanzu/velero/pkg/generated/listers/velero/v1"
	"github.com/vmware-tanzu/velero/pkg/kuberesource"
	"github.com/vmware-tanzu/velero/pkg/label"
	"github.com/vmware-tanzu/velero/pkg/plugin/velero"
	"github.com/vmware-tanzu/velero/pkg/restic"
	"github.com/vmware-tanzu/velero/pkg/util/boolptr"
	"github.com/vmware-tanzu/velero/pkg/util/collections"
	"github.com/vmware-tanzu/velero/pkg/util/filesystem"
	"github.com/vmware-tanzu/velero/pkg/util/kube"
	velerosync "github.com/vmware-tanzu/velero/pkg/util/sync"
	"github.com/vmware-tanzu/velero/pkg/volume"
)

type VolumeSnapshotterGetter interface {
	GetVolumeSnapshotter(name string) (velero.VolumeSnapshotter, error)
}

type Request struct {
	*velerov1api.Restore

	Log              logrus.FieldLogger
	Backup           *velerov1api.Backup
	PodVolumeBackups []*velerov1api.PodVolumeBackup
	VolumeSnapshots  []*volume.Snapshot
	BackupReader     io.Reader
}

// Restorer knows how to restore a backup.
type Restorer interface {
	// Restore restores the backup data from backupReader, returning warnings and errors.
	Restore(req Request,
		actions []velero.RestoreItemAction,
		snapshotLocationLister listers.VolumeSnapshotLocationLister,
		volumeSnapshotterGetter VolumeSnapshotterGetter,
	) (Result, Result)
}

// kubernetesRestorer implements Restorer for restoring into a Kubernetes cluster.
type kubernetesRestorer struct {
	discoveryHelper            discovery.Helper
	dynamicFactory             client.DynamicFactory
	namespaceClient            corev1.NamespaceInterface
	resticRestorerFactory      restic.RestorerFactory
	resticTimeout              time.Duration
	resourceTerminatingTimeout time.Duration
	resourcePriorities         []string
	fileSystem                 filesystem.Interface
	pvRenamer                  func(string) (string, error)
	logger                     logrus.FieldLogger
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
	resourcePriorities []string,
	namespaceClient corev1.NamespaceInterface,
	resticRestorerFactory restic.RestorerFactory,
	resticTimeout time.Duration,
	resourceTerminatingTimeout time.Duration,
	logger logrus.FieldLogger,
) (Restorer, error) {
	return &kubernetesRestorer{
		discoveryHelper:            discoveryHelper,
		dynamicFactory:             dynamicFactory,
		namespaceClient:            namespaceClient,
		resticRestorerFactory:      resticRestorerFactory,
		resticTimeout:              resticTimeout,
		resourceTerminatingTimeout: resourceTerminatingTimeout,
		resourcePriorities:         resourcePriorities,
		logger:                     logger,
		pvRenamer: func(string) (string, error) {
			veleroCloneUuid, err := uuid.NewV4()
			if err != nil {
				return "", errors.WithStack(err)
			}
			veleroCloneName := "velero-clone-" + veleroCloneUuid.String()
			return veleroCloneName, nil
		},
		fileSystem: filesystem.NewFileSystem(),
	}, nil
}

// Restore executes a restore into the target Kubernetes cluster according to the restore spec
// and using data from the provided backup/backup reader. Returns a warnings and errors RestoreResult,
// respectively, summarizing info about the restore.
func (kr *kubernetesRestorer) Restore(
	req Request,
	actions []velero.RestoreItemAction,
	snapshotLocationLister listers.VolumeSnapshotLocationLister,
	volumeSnapshotterGetter VolumeSnapshotterGetter,
) (Result, Result) {
	// metav1.LabelSelectorAsSelector converts a nil LabelSelector to a
	// Nothing Selector, i.e. a selector that matches nothing. We want
	// a selector that matches everything. This can be accomplished by
	// passing a non-nil empty LabelSelector.
	ls := req.Restore.Spec.LabelSelector
	if ls == nil {
		ls = &metav1.LabelSelector{}
	}

	selector, err := metav1.LabelSelectorAsSelector(ls)
	if err != nil {
		return Result{}, Result{Velero: []string{err.Error()}}
	}

	// get resource includes-excludes
	resourceIncludesExcludes := getResourceIncludesExcludes(kr.discoveryHelper, req.Restore.Spec.IncludedResources, req.Restore.Spec.ExcludedResources)
	prioritizedResources, err := prioritizeResources(kr.discoveryHelper, kr.resourcePriorities, resourceIncludesExcludes, req.Log)
	if err != nil {
		return Result{}, Result{Velero: []string{err.Error()}}
	}

	// get namespace includes-excludes
	namespaceIncludesExcludes := collections.NewIncludesExcludes().
		Includes(req.Restore.Spec.IncludedNamespaces...).
		Excludes(req.Restore.Spec.ExcludedNamespaces...)

	resolvedActions, err := resolveActions(actions, kr.discoveryHelper)
	if err != nil {
		return Result{}, Result{Velero: []string{err.Error()}}
	}

	podVolumeTimeout := kr.resticTimeout
	if val := req.Restore.Annotations[velerov1api.PodVolumeOperationTimeoutAnnotation]; val != "" {
		parsed, err := time.ParseDuration(val)
		if err != nil {
			req.Log.WithError(errors.WithStack(err)).Errorf("Unable to parse pod volume timeout annotation %s, using server value.", val)
		} else {
			podVolumeTimeout = parsed
		}
	}

	ctx, cancelFunc := go_context.WithTimeout(go_context.Background(), podVolumeTimeout)
	defer cancelFunc()

	var resticRestorer restic.Restorer
	if kr.resticRestorerFactory != nil {
		resticRestorer, err = kr.resticRestorerFactory.NewRestorer(ctx, req.Restore)
		if err != nil {
			return Result{}, Result{Velero: []string{err.Error()}}
		}
	}

	pvRestorer := &pvRestorer{
		logger:                  req.Log,
		backup:                  req.Backup,
		snapshotVolumes:         req.Backup.Spec.SnapshotVolumes,
		restorePVs:              req.Restore.Spec.RestorePVs,
		volumeSnapshots:         req.VolumeSnapshots,
		volumeSnapshotterGetter: volumeSnapshotterGetter,
		snapshotLocationLister:  snapshotLocationLister,
	}

	restoreCtx := &context{
		backup:                     req.Backup,
		backupReader:               req.BackupReader,
		restore:                    req.Restore,
		resourceIncludesExcludes:   resourceIncludesExcludes,
		namespaceIncludesExcludes:  namespaceIncludesExcludes,
		prioritizedResources:       prioritizedResources,
		selector:                   selector,
		log:                        req.Log,
		dynamicFactory:             kr.dynamicFactory,
		fileSystem:                 kr.fileSystem,
		namespaceClient:            kr.namespaceClient,
		actions:                    resolvedActions,
		volumeSnapshotterGetter:    volumeSnapshotterGetter,
		resticRestorer:             resticRestorer,
		pvsToProvision:             sets.NewString(),
		pvRestorer:                 pvRestorer,
		volumeSnapshots:            req.VolumeSnapshots,
		podVolumeBackups:           req.PodVolumeBackups,
		resourceTerminatingTimeout: kr.resourceTerminatingTimeout,
		resourceClients:            make(map[resourceClientKey]client.Dynamic),
		restoredItems:              make(map[velero.ResourceIdentifier]struct{}),
		renamedPVs:                 make(map[string]string),
		pvRenamer:                  kr.pvRenamer,
	}

	return restoreCtx.execute()
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
	velero.RestoreItemAction

	resourceIncludesExcludes  *collections.IncludesExcludes
	namespaceIncludesExcludes *collections.IncludesExcludes
	selector                  labels.Selector
}

func resolveActions(actions []velero.RestoreItemAction, helper discovery.Helper) ([]resolvedAction, error) {
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
			RestoreItemAction:         action,
			resourceIncludesExcludes:  resources,
			namespaceIncludesExcludes: namespaces,
			selector:                  selector,
		}

		resolved = append(resolved, res)
	}

	return resolved, nil
}

type context struct {
	backup                     *velerov1api.Backup
	backupReader               io.Reader
	restore                    *velerov1api.Restore
	restoreDir                 string
	resourceIncludesExcludes   *collections.IncludesExcludes
	namespaceIncludesExcludes  *collections.IncludesExcludes
	prioritizedResources       []schema.GroupResource
	selector                   labels.Selector
	log                        logrus.FieldLogger
	dynamicFactory             client.DynamicFactory
	fileSystem                 filesystem.Interface
	namespaceClient            corev1.NamespaceInterface
	actions                    []resolvedAction
	volumeSnapshotterGetter    VolumeSnapshotterGetter
	resticRestorer             restic.Restorer
	globalWaitGroup            velerosync.ErrorGroup
	pvsToProvision             sets.String
	pvRestorer                 PVRestorer
	volumeSnapshots            []*volume.Snapshot
	podVolumeBackups           []*velerov1api.PodVolumeBackup
	resourceTerminatingTimeout time.Duration
	resourceClients            map[resourceClientKey]client.Dynamic
	restoredItems              map[velero.ResourceIdentifier]struct{}
	renamedPVs                 map[string]string
	pvRenamer                  func(string) (string, error)
}

type resourceClientKey struct {
	resource  schema.GroupResource
	namespace string
}

func (ctx *context) execute() (Result, Result) {
	warnings, errs := Result{}, Result{}

	ctx.log.Infof("Starting restore of backup %s", kube.NamespaceAndName(ctx.backup))

	dir, err := archive.NewExtractor(ctx.log, ctx.fileSystem).UnzipAndExtractBackup(ctx.backupReader)
	if err != nil {
		ctx.log.Infof("error unzipping and extracting: %v", err)
		addVeleroError(&errs, err)
		return warnings, errs
	}
	defer ctx.fileSystem.RemoveAll(dir)

	// need to set this for additionalItems to be restored
	ctx.restoreDir = dir

	backupResources, err := archive.NewParser(ctx.log, ctx.fileSystem).Parse(ctx.restoreDir)
	if err != nil {
		addVeleroError(&errs, errors.Wrap(err, "error parsing backup contents"))
		return warnings, errs
	}

	existingNamespaces := sets.NewString()

	for _, resource := range ctx.prioritizedResources {
		// we don't want to explicitly restore namespace API objs because we'll handle
		// them as a special case prior to restoring anything into them
		if resource == kuberesource.Namespaces {
			continue
		}

		resourceList := backupResources[resource.String()]
		if resourceList == nil {
			continue
		}

		for namespace, items := range resourceList.ItemsByNamespace {
			if namespace != "" && !ctx.namespaceIncludesExcludes.ShouldInclude(namespace) {
				ctx.log.Infof("Skipping namespace %s", namespace)
				continue
			}

			// get target namespace to restore into, if different
			// from source namespace
			targetNamespace := namespace
			if target, ok := ctx.restore.Spec.NamespaceMapping[namespace]; ok {
				targetNamespace = target
			}

			// if we don't know whether this namespace exists yet, attempt to create
			// it in order to ensure it exists. Try to get it from the backup tarball
			// (in order to get any backed-up metadata), but if we don't find it there,
			// create a blank one.
			if namespace != "" && !existingNamespaces.Has(targetNamespace) {
				logger := ctx.log.WithField("namespace", namespace)
				ns := getNamespace(logger, getItemFilePath(ctx.restoreDir, "namespaces", "", namespace), targetNamespace)
				if _, err := kube.EnsureNamespaceExistsAndIsReady(ns, ctx.namespaceClient, ctx.resourceTerminatingTimeout); err != nil {
					addVeleroError(&errs, err)
					continue
				}

				// keep track of namespaces that we know exist so we don't
				// have to try to create them multiple times
				existingNamespaces.Insert(targetNamespace)
			}

			w, e := ctx.restoreResource(resource.String(), targetNamespace, namespace, items)
			merge(&warnings, &w)
			merge(&errs, &e)
		}
	}

	// TODO timeout?
	ctx.log.Debug("Waiting on global wait group")
	waitErrs := ctx.globalWaitGroup.Wait()
	ctx.log.Debug("Done waiting on global wait group")

	for _, err := range waitErrs {
		// TODO not ideal to be adding these to Velero-level errors
		// rather than a specific namespace, but don't have a way
		// to track the namespace right now.
		errs.Velero = append(errs.Velero, err.Error())
	}

	return warnings, errs
}

func getItemFilePath(rootDir, groupResource, namespace, name string) string {
	switch namespace {
	case "":
		return filepath.Join(rootDir, velerov1api.ResourcesDir, groupResource, velerov1api.ClusterScopedDir, name+".json")
	default:
		return filepath.Join(rootDir, velerov1api.ResourcesDir, groupResource, velerov1api.NamespaceScopedDir, namespace, name+".json")
	}
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
func merge(a, b *Result) {
	a.Cluster = append(a.Cluster, b.Cluster...)
	a.Velero = append(a.Velero, b.Velero...)
	for k, v := range b.Namespaces {
		if a.Namespaces == nil {
			a.Namespaces = make(map[string][]string)
		}
		a.Namespaces[k] = append(a.Namespaces[k], v...)
	}
}

// addVeleroError appends an error to the provided RestoreResult's Velero list.
func addVeleroError(r *Result, err error) {
	r.Velero = append(r.Velero, err.Error())
}

// addToResult appends an error to the provided RestoreResult, either within
// the cluster-scoped list (if ns == "") or within the provided namespace's
// entry.
func addToResult(r *Result, ns string, e error) {
	if ns == "" {
		r.Cluster = append(r.Cluster, e.Error())
	} else {
		if r.Namespaces == nil {
			r.Namespaces = make(map[string][]string)
		}
		r.Namespaces[ns] = append(r.Namespaces[ns], e.Error())
	}
}

func (ctx *context) getApplicableActions(groupResource schema.GroupResource, namespace string) []resolvedAction {
	var actions []resolvedAction
	for _, action := range ctx.actions {
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

func (ctx *context) shouldRestore(name string, pvClient client.Dynamic) (bool, error) {
	pvLogger := ctx.log.WithField("pvName", name)

	var shouldRestore bool
	err := wait.PollImmediate(time.Second, ctx.resourceTerminatingTimeout, func() (bool, error) {
		unstructuredPV, err := pvClient.Get(name, metav1.GetOptions{})
		if apierrors.IsNotFound(err) {
			pvLogger.Debug("PV not found, safe to restore")
			// PV not found, can safely exit loop and proceed with restore.
			shouldRestore = true
			return true, nil
		}
		if err != nil {
			return false, errors.Wrapf(err, "could not retrieve in-cluster copy of PV %s", name)
		}

		clusterPV := new(v1.PersistentVolume)
		if err := runtime.DefaultUnstructuredConverter.FromUnstructured(unstructuredPV.Object, clusterPV); err != nil {
			return false, errors.Wrap(err, "error converting PV from unstructured")
		}

		if clusterPV.Status.Phase == v1.VolumeReleased || clusterPV.DeletionTimestamp != nil {
			// PV was found and marked for deletion, or it was released; wait for it to go away.
			pvLogger.Debugf("PV found, but marked for deletion, waiting")
			return false, nil
		}

		// Check for the namespace and PVC to see if anything that's referencing the PV is deleting.
		// If either the namespace or PVC is in a deleting/terminating state, wait for them to finish before
		// trying to restore the PV
		// Not doing so may result in the underlying PV disappearing but not restoring due to timing issues,
		// then the PVC getting restored and showing as lost.
		if clusterPV.Spec.ClaimRef == nil {
			pvLogger.Debugf("PV is not marked for deletion and is not claimed by a PVC")
			return true, nil
		}

		namespace := clusterPV.Spec.ClaimRef.Namespace
		pvcName := clusterPV.Spec.ClaimRef.Name

		// Have to create the PVC client here because we don't know what namespace we're using til we get to this point.
		// Using a dynamic client since it's easier to mock for testing
		pvcResource := metav1.APIResource{Name: "persistentvolumeclaims", Namespaced: true}
		pvcClient, err := ctx.dynamicFactory.ClientForGroupVersionResource(schema.GroupVersion{Group: "", Version: "v1"}, pvcResource, namespace)
		if err != nil {
			return false, errors.Wrapf(err, "error getting pvc client")
		}

		pvc, err := pvcClient.Get(pvcName, metav1.GetOptions{})
		if apierrors.IsNotFound(err) {
			pvLogger.Debugf("PVC %s for PV not found, waiting", pvcName)
			// PVC wasn't found, but the PV still exists, so continue to wait.
			return false, nil
		}
		if err != nil {
			return false, errors.Wrapf(err, "error getting claim %s for persistent volume", pvcName)
		}

		if pvc != nil && pvc.GetDeletionTimestamp() != nil {
			pvLogger.Debugf("PVC for PV marked for deletion, waiting")
			// PVC is still deleting, continue to wait.
			return false, nil
		}

		// Check the namespace associated with the claimRef to see if it's deleting/terminating before proceeding
		ns, err := ctx.namespaceClient.Get(namespace, metav1.GetOptions{})
		if apierrors.IsNotFound(err) {
			pvLogger.Debugf("namespace %s for PV not found, waiting", namespace)
			// namespace not found but the PV still exists, so continue to wait
			return false, nil
		}
		if err != nil {
			return false, errors.Wrapf(err, "error getting namespace %s associated with PV %s", namespace, name)
		}

		if ns != nil && (ns.GetDeletionTimestamp() != nil || ns.Status.Phase == v1.NamespaceTerminating) {
			pvLogger.Debugf("namespace %s associated with PV is deleting, waiting", namespace)
			// namespace is in the process of deleting, keep looping
			return false, nil
		}

		// None of the PV, PVC, or NS are marked for deletion, break the loop.
		pvLogger.Debug("PV, associated PVC and namespace are not marked for deletion")
		return true, nil
	})

	if err == wait.ErrWaitTimeout {
		pvLogger.Debug("timeout reached waiting for persistent volume to delete")
	}

	return shouldRestore, err
}

// restoreResource restores the specified cluster or namespace scoped resource. If namespace is
// empty we are restoring a cluster level resource, otherwise into the specified namespace.
func (ctx *context) restoreResource(resource, targetNamespace, originalNamespace string, items []string) (Result, Result) {
	warnings, errs := Result{}, Result{}

	if targetNamespace == "" && boolptr.IsSetToFalse(ctx.restore.Spec.IncludeClusterResources) {
		ctx.log.Infof("Skipping resource %s because it's cluster-scoped", resource)
		return warnings, errs
	}

	if targetNamespace == "" && !boolptr.IsSetToTrue(ctx.restore.Spec.IncludeClusterResources) && !ctx.namespaceIncludesExcludes.IncludeEverything() {
		ctx.log.Infof("Skipping resource %s because it's cluster-scoped and only specific namespaces are included in the restore", resource)
		return warnings, errs
	}

	if targetNamespace != "" {
		ctx.log.Infof("Restoring resource '%s' into namespace '%s'", resource, targetNamespace)
	} else {
		ctx.log.Infof("Restoring cluster level resource '%s'", resource)
	}

	if len(items) == 0 {
		return warnings, errs
	}

	groupResource := schema.ParseGroupResource(resource)

	for _, item := range items {
		itemPath := getItemFilePath(ctx.restoreDir, resource, originalNamespace, item)

		obj, err := ctx.unmarshal(itemPath)
		if err != nil {
			addToResult(&errs, targetNamespace, fmt.Errorf("error decoding %q: %v", strings.Replace(itemPath, ctx.restoreDir+"/", "", -1), err))
			continue
		}

		if !ctx.selector.Matches(labels.Set(obj.GetLabels())) {
			continue
		}

		w, e := ctx.restoreItem(obj, groupResource, targetNamespace)
		merge(&warnings, &w)
		merge(&errs, &e)
	}

	return warnings, errs
}

func (ctx *context) getResourceClient(groupResource schema.GroupResource, obj *unstructured.Unstructured, namespace string) (client.Dynamic, error) {
	key := resourceClientKey{
		resource:  groupResource,
		namespace: namespace,
	}

	if client, ok := ctx.resourceClients[key]; ok {
		return client, nil
	}

	// initialize client for this Resource. we need
	// metadata from an object to do this.
	ctx.log.Infof("Getting client for %v", obj.GroupVersionKind())

	resource := metav1.APIResource{
		Namespaced: len(namespace) > 0,
		Name:       groupResource.Resource,
	}

	client, err := ctx.dynamicFactory.ClientForGroupVersionResource(obj.GroupVersionKind().GroupVersion(), resource, namespace)
	if err != nil {
		return nil, err
	}

	ctx.resourceClients[key] = client
	return client, nil
}

func getResourceID(groupResource schema.GroupResource, namespace, name string) string {
	if namespace == "" {
		return fmt.Sprintf("%s/%s", groupResource.String(), name)
	}

	return fmt.Sprintf("%s/%s/%s", groupResource.String(), namespace, name)
}

func (ctx *context) restoreItem(obj *unstructured.Unstructured, groupResource schema.GroupResource, namespace string) (Result, Result) {
	warnings, errs := Result{}, Result{}
	resourceID := getResourceID(groupResource, namespace, obj.GetName())

	// Check if group/resource should be restored. We need to do this here since
	// this method may be getting called for an additional item which is a group/resource
	// that's excluded.
	if !ctx.resourceIncludesExcludes.ShouldInclude(groupResource.String()) {
		ctx.log.WithFields(logrus.Fields{
			"namespace":     obj.GetNamespace(),
			"name":          obj.GetName(),
			"groupResource": groupResource.String(),
		}).Info("Not restoring item because resource is excluded")
		return warnings, errs
	}

	// Check if namespace/cluster-scoped resource should be restored. We need
	// to do this here since this method may be getting called for an additional
	// item which is in a namespace that's excluded, or which is cluster-scoped
	// and should be excluded. Note that we're checking the object's namespace (
	// via obj.GetNamespace()) instead of the namespace parameter, because we want
	// to check the *original* namespace, not the remapped one if it's been remapped.
	if namespace != "" {
		if !ctx.namespaceIncludesExcludes.ShouldInclude(obj.GetNamespace()) {
			ctx.log.WithFields(logrus.Fields{
				"namespace":     obj.GetNamespace(),
				"name":          obj.GetName(),
				"groupResource": groupResource.String(),
			}).Info("Not restoring item because namespace is excluded")
			return warnings, errs
		}
	} else {
		if boolptr.IsSetToFalse(ctx.restore.Spec.IncludeClusterResources) {
			ctx.log.WithFields(logrus.Fields{
				"namespace":     obj.GetNamespace(),
				"name":          obj.GetName(),
				"groupResource": groupResource.String(),
			}).Info("Not restoring item because it's cluster-scoped")
			return warnings, errs
		}
	}

	// make a copy of object retrieved from backup
	// to make it available unchanged inside restore actions
	itemFromBackup := obj.DeepCopy()

	complete, err := isCompleted(obj, groupResource)
	if err != nil {
		addToResult(&errs, namespace, fmt.Errorf("error checking completion of %q: %v", resourceID, err))
		return warnings, errs
	}
	if complete {
		ctx.log.Infof("%s is complete - skipping", kube.NamespaceAndName(obj))
		return warnings, errs
	}

	name := obj.GetName()

	// Check if we've already restored this
	itemKey := velero.ResourceIdentifier{
		GroupResource: groupResource,
		Namespace:     namespace,
		Name:          name,
	}
	if _, exists := ctx.restoredItems[itemKey]; exists {
		ctx.log.Infof("Skipping %s because it's already been restored.", resourceID)
		return warnings, errs
	}
	ctx.restoredItems[itemKey] = struct{}{}

	// TODO: move to restore item action if/when we add a ShouldRestore() method to the interface
	if groupResource == kuberesource.Pods && obj.GetAnnotations()[v1.MirrorPodAnnotationKey] != "" {
		ctx.log.Infof("Not restoring pod because it's a mirror pod")
		return warnings, errs
	}

	resourceClient, err := ctx.getResourceClient(groupResource, obj, namespace)
	if err != nil {
		addVeleroError(&errs, fmt.Errorf("error getting resource client for namespace %q, resource %q: %v", namespace, &groupResource, err))
		return warnings, errs
	}

	if groupResource == kuberesource.PersistentVolumes {
		switch {
		case hasSnapshot(name, ctx.volumeSnapshots):
			shouldRenamePV, err := shouldRenamePV(ctx, obj, resourceClient)
			if err != nil {
				addToResult(&errs, namespace, err)
				return warnings, errs
			}

			var shouldRestoreSnapshot bool
			if !shouldRenamePV {
				// Check if the PV exists in the cluster before attempting to create
				// a volume from the snapshot, in order to avoid orphaned volumes (GH #609)
				shouldRestoreSnapshot, err = ctx.shouldRestore(name, resourceClient)
				if err != nil {
					addToResult(&errs, namespace, errors.Wrapf(err, "error waiting on in-cluster persistentvolume %s", name))
					return warnings, errs
				}
			} else {
				// if we're renaming the PV, we're going to give it a new random name,
				// so we can assume it doesn't already exist in the cluster and therefore
				// we should proceed with restoring from snapshot.
				shouldRestoreSnapshot = true
			}

			if shouldRestoreSnapshot {
				// even if we're renaming the PV, obj still has the old name here, because the pvRestorer
				// uses the original name to look up metadata about the snapshot.
				ctx.log.Infof("Restoring persistent volume from snapshot.")
				updatedObj, err := ctx.pvRestorer.executePVAction(obj)
				if err != nil {
					addToResult(&errs, namespace, fmt.Errorf("error executing PVAction for %s: %v", resourceID, err))
					return warnings, errs
				}
				obj = updatedObj
			}

			if shouldRenamePV {
				// give obj a new name, and record the mapping between the old and new names
				oldName := obj.GetName()
				newName, err := ctx.pvRenamer(oldName)
				if err != nil {
					addToResult(&errs, namespace, errors.Wrapf(err, "error renaming PV"))
					return warnings, errs
				}

				ctx.renamedPVs[oldName] = newName
				obj.SetName(newName)

				// add the original PV name as an annotation
				annotations := obj.GetAnnotations()
				if annotations == nil {
					annotations = map[string]string{}
				}
				annotations["velero.io/original-pv-name"] = oldName
				obj.SetAnnotations(annotations)
			}

		case hasResticBackup(obj, ctx):
			ctx.log.Infof("Dynamically re-provisioning persistent volume because it has a restic backup to be restored.")
			ctx.pvsToProvision.Insert(name)

			// return early because we don't want to restore the PV itself, we want to dynamically re-provision it.
			return warnings, errs

		case hasDeleteReclaimPolicy(obj.Object):
			ctx.log.Infof("Dynamically re-provisioning persistent volume because it doesn't have a snapshot and its reclaim policy is Delete.")
			ctx.pvsToProvision.Insert(name)

			// return early because we don't want to restore the PV itself, we want to dynamically re-provision it.
			return warnings, errs

		default:
			ctx.log.Infof("Restoring persistent volume as-is because it doesn't have a snapshot and its reclaim policy is not Delete.")

			// we call the pvRestorer here to clear out the PV's claimRef, so it can be re-claimed
			// when its PVC is restored.
			updatedObj, err := ctx.pvRestorer.executePVAction(obj)
			if err != nil {
				addToResult(&errs, namespace, fmt.Errorf("error executing PVAction for %s: %v", resourceID, err))
				return warnings, errs
			}
			obj = updatedObj
		}
	}

	// clear out non-core metadata fields & status
	if obj, err = resetMetadataAndStatus(obj); err != nil {
		addToResult(&errs, namespace, err)
		return warnings, errs
	}

	for _, action := range ctx.getApplicableActions(groupResource, namespace) {
		if !action.selector.Matches(labels.Set(obj.GetLabels())) {
			return warnings, errs
		}

		ctx.log.Infof("Executing item action for %v", &groupResource)

		executeOutput, err := action.Execute(&velero.RestoreItemActionExecuteInput{
			Item:           obj,
			ItemFromBackup: itemFromBackup,
			Restore:        ctx.restore,
		})
		if err != nil {
			addToResult(&errs, namespace, fmt.Errorf("error preparing %s: %v", resourceID, err))
			return warnings, errs
		}

		if executeOutput.SkipRestore {
			ctx.log.Infof("Skipping restore of %s: %v because a registered plugin discarded it", obj.GroupVersionKind().Kind, name)
			return warnings, errs
		}
		unstructuredObj, ok := executeOutput.UpdatedItem.(*unstructured.Unstructured)
		if !ok {
			addToResult(&errs, namespace, fmt.Errorf("%s: unexpected type %T", resourceID, executeOutput.UpdatedItem))
			return warnings, errs
		}

		obj = unstructuredObj

		for _, additionalItem := range executeOutput.AdditionalItems {
			itemPath := getItemFilePath(ctx.restoreDir, additionalItem.GroupResource.String(), additionalItem.Namespace, additionalItem.Name)

			if _, err := ctx.fileSystem.Stat(itemPath); err != nil {
				ctx.log.WithError(err).WithFields(logrus.Fields{
					"additionalResource":          additionalItem.GroupResource.String(),
					"additionalResourceNamespace": additionalItem.Namespace,
					"additionalResourceName":      additionalItem.Name,
				}).Warn("unable to restore additional item")
				addToResult(&warnings, additionalItem.Namespace, err)

				continue
			}

			additionalResourceID := getResourceID(additionalItem.GroupResource, additionalItem.Namespace, additionalItem.Name)
			additionalObj, err := ctx.unmarshal(itemPath)
			if err != nil {
				addToResult(&errs, namespace, errors.Wrapf(err, "error restoring additional item %s", additionalResourceID))
			}

			additionalItemNamespace := additionalItem.Namespace
			if additionalItemNamespace != "" {
				if remapped, ok := ctx.restore.Spec.NamespaceMapping[additionalItemNamespace]; ok {
					additionalItemNamespace = remapped
				}
			}

			w, e := ctx.restoreItem(additionalObj, additionalItem.GroupResource, additionalItemNamespace)
			merge(&warnings, &w)
			merge(&errs, &e)
		}
	}

	// This comes after running item actions because we have built-in actions that restore
	// a PVC's associated PV (if applicable). As part of the PV being restored, the 'pvsToProvision'
	// set may be inserted into, and this needs to happen *before* running the following block of logic.
	//
	// The side effect of this is that it's impossible for a user to write a restore item action that
	// adjusts this behavior (i.e. of resetting the PVC for dynamic provisioning if it claims a PV with
	// a reclaim policy of Delete and no snapshot). If/when that becomes an issue for users, we can
	// revisit. This would be easier with a multi-pass restore process.
	if groupResource == kuberesource.PersistentVolumeClaims {
		pvc := new(v1.PersistentVolumeClaim)
		if err := runtime.DefaultUnstructuredConverter.FromUnstructured(obj.UnstructuredContent(), pvc); err != nil {
			addToResult(&errs, namespace, err)
			return warnings, errs
		}

		if pvc.Spec.VolumeName != "" && ctx.pvsToProvision.Has(pvc.Spec.VolumeName) {
			ctx.log.Infof("Resetting PersistentVolumeClaim %s/%s for dynamic provisioning because its PV %v has a reclaim policy of Delete", namespace, name, pvc.Spec.VolumeName)

			// use the unstructured helpers here since we're only deleting and
			// the unstructured converter will add back (empty) fields for metadata
			// and status that we removed earlier.
			unstructured.RemoveNestedField(obj.Object, "spec", "volumeName")
			annotations := obj.GetAnnotations()
			delete(annotations, "pv.kubernetes.io/bind-completed")
			delete(annotations, "pv.kubernetes.io/bound-by-controller")
			obj.SetAnnotations(annotations)
		}

		if newName, ok := ctx.renamedPVs[pvc.Spec.VolumeName]; ok {
			ctx.log.Infof("Updating persistent volume claim %s/%s to reference renamed persistent volume (%s -> %s)", namespace, name, pvc.Spec.VolumeName, newName)
			if err := unstructured.SetNestedField(obj.Object, newName, "spec", "volumeName"); err != nil {
				addToResult(&errs, namespace, err)
				return warnings, errs
			}
		}
	}

	// necessary because we may have remapped the namespace
	// if the namespace is blank, don't create the key
	originalNamespace := obj.GetNamespace()
	if namespace != "" {
		obj.SetNamespace(namespace)
	}

	// label the resource with the restore's name and the restored backup's name
	// for easy identification of all cluster resources created by this restore
	// and which backup they came from
	addRestoreLabels(obj, ctx.restore.Name, ctx.restore.Spec.BackupName)

	ctx.log.Infof("Attempting to restore %s: %v", obj.GroupVersionKind().Kind, name)
	createdObj, restoreErr := resourceClient.Create(obj)
	if apierrors.IsAlreadyExists(restoreErr) {
		fromCluster, err := resourceClient.Get(name, metav1.GetOptions{})
		if err != nil {
			ctx.log.Infof("Error retrieving cluster version of %s: %v", kube.NamespaceAndName(obj), err)
			addToResult(&warnings, namespace, err)
			return warnings, errs
		}
		// Remove insubstantial metadata
		fromCluster, err = resetMetadataAndStatus(fromCluster)
		if err != nil {
			ctx.log.Infof("Error trying to reset metadata for %s: %v", kube.NamespaceAndName(obj), err)
			addToResult(&warnings, namespace, err)
			return warnings, errs
		}

		// We know the object from the cluster won't have the backup/restore name labels, so
		// copy them from the object we attempted to restore.
		labels := obj.GetLabels()
		addRestoreLabels(fromCluster, labels[velerov1api.RestoreNameLabel], labels[velerov1api.BackupNameLabel])

		if !equality.Semantic.DeepEqual(fromCluster, obj) {
			switch groupResource {
			case kuberesource.ServiceAccounts:
				desired, err := mergeServiceAccounts(fromCluster, obj)
				if err != nil {
					ctx.log.Infof("error merging secrets for ServiceAccount %s: %v", kube.NamespaceAndName(obj), err)
					addToResult(&warnings, namespace, err)
					return warnings, errs
				}

				patchBytes, err := generatePatch(fromCluster, desired)
				if err != nil {
					ctx.log.Infof("error generating patch for ServiceAccount %s: %v", kube.NamespaceAndName(obj), err)
					addToResult(&warnings, namespace, err)
					return warnings, errs
				}

				if patchBytes == nil {
					// In-cluster and desired state are the same, so move on to the next item
					return warnings, errs
				}

				_, err = resourceClient.Patch(name, patchBytes)
				if err != nil {
					addToResult(&warnings, namespace, err)
				} else {
					ctx.log.Infof("ServiceAccount %s successfully updated", kube.NamespaceAndName(obj))
				}
			default:
				e := errors.Errorf("could not restore, %s. Warning: the in-cluster version is different than the backed-up version.", restoreErr)
				addToResult(&warnings, namespace, e)
			}
			return warnings, errs
		}

		ctx.log.Infof("Restore of %s, %v skipped: it already exists in the cluster and is the same as the backed up version", obj.GroupVersionKind().Kind, name)
		return warnings, errs
	}

	// Error was something other than an AlreadyExists
	if restoreErr != nil {
		ctx.log.Infof("error restoring %s: %v", name, restoreErr)
		addToResult(&errs, namespace, fmt.Errorf("error restoring %s: %v", resourceID, restoreErr))
		return warnings, errs
	}

	if groupResource == kuberesource.Pods && len(restic.GetVolumeBackupsForPod(ctx.podVolumeBackups, obj)) > 0 {
		restorePodVolumeBackups(ctx, createdObj, originalNamespace)
	}

	return warnings, errs
}

// shouldRenamePV returns a boolean indicating whether a persistent volume should be given a new name
// before being restored, or an error if this cannot be determined. A persistent volume will be
// given a new name if and only if (a) a PV with the original name already exists in-cluster, and
// (b) in the backup, the PV is claimed by a PVC in a namespace that's being remapped during the
// restore.
func shouldRenamePV(ctx *context, obj *unstructured.Unstructured, client client.Dynamic) (bool, error) {
	if len(ctx.restore.Spec.NamespaceMapping) == 0 {
		ctx.log.Debugf("Persistent volume does not need to be renamed because restore is not remapping any namespaces")
		return false, nil
	}

	pv := new(v1.PersistentVolume)
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(obj.Object, pv); err != nil {
		return false, errors.Wrapf(err, "error converting persistent volume to structured")
	}

	if pv.Spec.ClaimRef == nil {
		ctx.log.Debugf("Persistent volume does not need to be renamed because it's not claimed")
		return false, nil
	}

	if _, ok := ctx.restore.Spec.NamespaceMapping[pv.Spec.ClaimRef.Namespace]; !ok {
		ctx.log.Debugf("Persistent volume does not need to be renamed because it's not claimed by a PVC in a namespace that's being remapped")
		return false, nil
	}

	_, err := client.Get(pv.Name, metav1.GetOptions{})
	switch {
	case apierrors.IsNotFound(err):
		ctx.log.Debugf("Persistent volume does not need to be renamed because it does not exist in the cluster")
		return false, nil
	case err != nil:
		return false, errors.Wrapf(err, "error checking if persistent volume exists in the cluster")
	}

	// no error returned: the PV was found in-cluster, so we need to rename it
	return true, nil
}

// restorePodVolumeBackups restores the PodVolumeBackups for the given restored pod
func restorePodVolumeBackups(ctx *context, createdObj *unstructured.Unstructured, originalNamespace string) {
	if ctx.resticRestorer == nil {
		ctx.log.Warn("No restic restorer, not restoring pod's volumes")
	} else {
		ctx.globalWaitGroup.GoErrorSlice(func() []error {
			pod := new(v1.Pod)
			if err := runtime.DefaultUnstructuredConverter.FromUnstructured(createdObj.UnstructuredContent(), &pod); err != nil {
				ctx.log.WithError(err).Error("error converting unstructured pod")
				return []error{err}
			}

			data := restic.RestoreData{
				Restore:          ctx.restore,
				Pod:              pod,
				PodVolumeBackups: ctx.podVolumeBackups,
				SourceNamespace:  originalNamespace,
				BackupLocation:   ctx.backup.Spec.StorageLocation,
			}
			if errs := ctx.resticRestorer.RestorePodVolumes(data); errs != nil {
				ctx.log.WithError(kubeerrs.NewAggregate(errs)).Error("unable to successfully complete restic restores of pod's volumes")
				return errs
			}

			return nil
		})
	}
}

func hasSnapshot(pvName string, snapshots []*volume.Snapshot) bool {
	for _, snapshot := range snapshots {
		if snapshot.Spec.PersistentVolumeName == pvName {
			return true
		}
	}

	return false
}

func hasResticBackup(unstructuredPV *unstructured.Unstructured, ctx *context) bool {
	if len(ctx.podVolumeBackups) == 0 {
		return false
	}

	pv := new(v1.PersistentVolume)
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(unstructuredPV.Object, pv); err != nil {
		ctx.log.WithError(err).Warnf("Unable to convert PV from unstructured to structured")
		return false
	}

	if pv.Spec.ClaimRef == nil {
		return false
	}

	var found bool
	for _, pvb := range ctx.podVolumeBackups {
		if pvb.Spec.Pod.Namespace == pv.Spec.ClaimRef.Namespace && pvb.GetAnnotations()[restic.PVCNameAnnotation] == pv.Spec.ClaimRef.Name {
			found = true
			break
		}
	}

	return found
}

func hasDeleteReclaimPolicy(obj map[string]interface{}) bool {
	policy, _, _ := unstructured.NestedString(obj, "spec", "persistentVolumeReclaimPolicy")
	return policy == string(v1.PersistentVolumeReclaimDelete)
}

func resetMetadataAndStatus(obj *unstructured.Unstructured) (*unstructured.Unstructured, error) {
	res, ok := obj.Object["metadata"]
	if !ok {
		return nil, errors.New("metadata not found")
	}
	metadata, ok := res.(map[string]interface{})
	if !ok {
		return nil, errors.Errorf("metadata was of type %T, expected map[string]interface{}", res)
	}

	for k := range metadata {
		switch k {
		case "name", "namespace", "labels", "annotations":
		default:
			delete(metadata, k)
		}
	}

	// Never restore status
	delete(obj.UnstructuredContent(), "status")

	return obj, nil
}

// addRestoreLabels labels the provided object with the restore name and
// the restored backup's name.
func addRestoreLabels(obj metav1.Object, restoreName, backupName string) {
	labels := obj.GetLabels()

	if labels == nil {
		labels = make(map[string]string)
	}

	labels[velerov1api.BackupNameLabel] = label.GetValidName(backupName)
	labels[velerov1api.RestoreNameLabel] = label.GetValidName(restoreName)

	obj.SetLabels(labels)
}

// isCompleted returns whether or not an object is considered completed.
// Used to identify whether or not an object should be restored. Only Jobs or Pods are considered
func isCompleted(obj *unstructured.Unstructured, groupResource schema.GroupResource) (bool, error) {
	switch groupResource {
	case kuberesource.Pods:
		phase, _, err := unstructured.NestedString(obj.UnstructuredContent(), "status", "phase")
		if err != nil {
			return false, errors.WithStack(err)
		}
		if phase == string(v1.PodFailed) || phase == string(v1.PodSucceeded) {
			return true, nil
		}

	case kuberesource.Jobs:
		ct, found, err := unstructured.NestedString(obj.UnstructuredContent(), "status", "completionTime")
		if err != nil {
			return false, errors.WithStack(err)
		}
		if found && ct != "" {
			return true, nil
		}
	}
	// Assume any other resource isn't complete and can be restored
	return false, nil
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
