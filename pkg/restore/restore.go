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
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"sort"

	"github.com/golang/glog"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/sets"
	corev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/pkg/api/v1"

	api "github.com/heptio/ark/pkg/apis/ark/v1"
	"github.com/heptio/ark/pkg/client"
	"github.com/heptio/ark/pkg/cloudprovider"
	"github.com/heptio/ark/pkg/discovery"
	arkv1client "github.com/heptio/ark/pkg/generated/clientset/typed/ark/v1"
	"github.com/heptio/ark/pkg/restore/restorers"
	"github.com/heptio/ark/pkg/util/kube"
)

// Restorer knows how to restore a backup.
type Restorer interface {
	// Restore restores the backup data from backupReader, returning warnings and errors.
	Restore(restore *api.Restore, backup *api.Backup, backupReader io.Reader) (api.RestoreResult, api.RestoreResult)
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
}

// prioritizeResources takes a list of pre-prioritized resources and a full list of resources to restore,
// and returns an ordered list of GroupResource-resolved resources in the order that they should be
// restored.
func prioritizeResources(mapper meta.RESTMapper, priorities []string, resources []*metav1.APIResourceList) ([]schema.GroupResource, error) {
	var ret []schema.GroupResource

	// set keeps track of resolved GroupResource names
	set := sets.NewString()

	// start by resolving priorities into GroupResources and adding them to ret
	for _, r := range priorities {
		gr := schema.ParseGroupResource(r)
		gvr, err := mapper.ResourceFor(gr.WithVersion(""))
		if err != nil {
			return nil, err
		}
		gr = gvr.GroupResource()
		ret = append(ret, gr)
		set.Insert(gr.String())
	}

	// go through everything we got from discovery and add anything not in "set" to byName
	var byName []schema.GroupResource
	for _, resourceGroup := range resources {
		// will be something like storage.k8s.io/v1
		groupVersion, err := schema.ParseGroupVersion(resourceGroup.GroupVersion)
		if err != nil {
			return nil, err
		}

		for _, resource := range resourceGroup.APIResources {
			gr := groupVersion.WithResource(resource.Name).GroupResource()
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
) (Restorer, error) {
	mapper := discoveryHelper.Mapper()
	r := make(map[schema.GroupResource]restorers.ResourceRestorer)
	for gr, restorer := range customRestorers {
		gvr, err := mapper.ResourceFor(schema.ParseGroupResource(gr).WithVersion(""))
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
	}, nil
}

// Restore executes a restore into the target Kubernetes cluster according to the restore spec
// and using data from the provided backup/backup reader. Returns a warnings and errors RestoreResult,
// respectively, summarizing info about the restore.
func (kr *kubernetesRestorer) Restore(restore *api.Restore, backup *api.Backup, backupReader io.Reader) (api.RestoreResult, api.RestoreResult) {
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

	prioritizedResources, err := prioritizeResources(kr.discoveryHelper.Mapper(), kr.resourcePriorities, kr.discoveryHelper.Resources())
	if err != nil {
		return api.RestoreResult{}, api.RestoreResult{Ark: []string{err.Error()}}
	}

	dir, err := kr.unzipAndExtractBackup(backupReader)
	if err != nil {
		glog.Errorf("error unzipping and extracting: %v", err)
		return api.RestoreResult{}, api.RestoreResult{Ark: []string{err.Error()}}
	}
	defer kr.fileSystem.RemoveAll(dir)

	return kr.restoreFromDir(dir, restore, backup, prioritizedResources, selector)
}

// restoreFromDir executes a restore based on backup data contained within a local
// directory.
func (kr *kubernetesRestorer) restoreFromDir(
	dir string,
	restore *api.Restore,
	backup *api.Backup,
	prioritizedResources []schema.GroupResource,
	selector labels.Selector,
) (api.RestoreResult, api.RestoreResult) {
	warnings, errors := api.RestoreResult{}, api.RestoreResult{}

	// cluster-scoped
	clusterPath := path.Join(dir, api.ClusterScopedDir)
	exists, err := kr.fileSystem.DirExists(clusterPath)
	if err != nil {
		errors.Cluster = []string{err.Error()}
	}
	if exists {
		w, e := kr.restoreNamespace(restore, "", clusterPath, prioritizedResources, selector, backup)
		merge(&warnings, &w)
		merge(&errors, &e)
	}

	// namespace-scoped
	namespacesPath := path.Join(dir, api.NamespaceScopedDir)
	nses, err := kr.fileSystem.ReadDir(namespacesPath)
	if err != nil {
		addArkError(&errors, err)
		return warnings, errors
	}

	namespacesToRestore := sets.NewString(restore.Spec.Namespaces...)
	for _, ns := range nses {
		if !ns.IsDir() {
			continue
		}

		nsPath := path.Join(namespacesPath, ns.Name())

		if !namespacesToRestore.Has("*") && !namespacesToRestore.Has(ns.Name()) {
			glog.Infof("Skipping namespace %s", ns.Name())
			continue
		}
		w, e := kr.restoreNamespace(restore, ns.Name(), nsPath, prioritizedResources, selector, backup)
		merge(&warnings, &w)
		merge(&errors, &e)
	}

	return warnings, errors
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

// restoreNamespace restores the resources from a specified namespace directory in the backup,
// or from the cluster-scoped directory if no namespace is specified.
func (kr *kubernetesRestorer) restoreNamespace(
	restore *api.Restore,
	nsName string,
	nsPath string,
	prioritizedResources []schema.GroupResource,
	labelSelector labels.Selector,
	backup *api.Backup,
) (api.RestoreResult, api.RestoreResult) {
	warnings, errors := api.RestoreResult{}, api.RestoreResult{}

	if nsName == "" {
		glog.Info("Restoring cluster-scoped resources")
	} else {
		glog.Infof("Restoring namespace %s", nsName)
	}

	resourceDirs, err := kr.fileSystem.ReadDir(nsPath)
	if err != nil {
		addToResult(&errors, nsName, err)
		return warnings, errors
	}

	resourceDirsMap := make(map[string]os.FileInfo)

	for _, rscDir := range resourceDirs {
		rscName := rscDir.Name()
		resourceDirsMap[rscName] = rscDir
	}

	if nsName != "" {
		// fetch mapped NS name
		if target, ok := restore.Spec.NamespaceMapping[nsName]; ok {
			nsName = target
		}

		// ensure namespace exists
		ns := &v1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: nsName,
			},
		}

		if _, err := kube.EnsureNamespaceExists(ns, kr.namespaceClient); err != nil {
			addArkError(&errors, err)
			return warnings, errors
		}
	}

	for _, resource := range prioritizedResources {
		rscDir := resourceDirsMap[resource.String()]
		if rscDir == nil {
			continue
		}

		resourcePath := path.Join(nsPath, rscDir.Name())

		w, e := kr.restoreResourceForNamespace(nsName, resourcePath, labelSelector, restore, backup)
		merge(&warnings, &w)
		merge(&errors, &e)
	}

	return warnings, errors
}

// restoreResourceForNamespace restores the specified resource type for the specified
// namespace (or blank for cluster-scoped resources).
func (kr *kubernetesRestorer) restoreResourceForNamespace(
	namespace string,
	resourcePath string,
	labelSelector labels.Selector,
	restore *api.Restore,
	backup *api.Backup,
) (api.RestoreResult, api.RestoreResult) {
	warnings, errors := api.RestoreResult{}, api.RestoreResult{}
	resource := path.Base(resourcePath)

	glog.Infof("Restoring resource %v into namespace %v\n", resource, namespace)

	files, err := kr.fileSystem.ReadDir(resourcePath)
	if err != nil {
		addToResult(&errors, namespace, fmt.Errorf("error reading %q resource directory: %v", resource, err))
		return warnings, errors
	}
	if len(files) == 0 {
		return warnings, errors
	}

	var (
		resourceClient client.Dynamic
		restorer       restorers.ResourceRestorer
		waiter         *resourceWaiter
		groupResource  = schema.ParseGroupResource(path.Base(resourcePath))
	)

	for _, file := range files {
		fullPath := filepath.Join(resourcePath, file.Name())
		obj, err := kr.unmarshal(fullPath)
		if err != nil {
			addToResult(&errors, namespace, fmt.Errorf("error decoding %q: %v", fullPath, err))
			continue
		}

		if !labelSelector.Matches(labels.Set(obj.GetLabels())) {
			continue
		}

		if restorer == nil {
			// initialize client & restorer for this Resource. we need
			// metadata from an object to do this.
			glog.Infof("Getting client for %s", obj.GroupVersionKind().String())

			resource := metav1.APIResource{
				Namespaced: len(namespace) > 0,
				Name:       groupResource.Resource,
			}

			var err error
			resourceClient, err = kr.dynamicFactory.ClientForGroupVersionKind(obj.GroupVersionKind(), resource, namespace)
			if err != nil {
				addArkError(&errors, fmt.Errorf("error getting resource client for namespace %q, resource %q: %v", namespace, groupResource.String(), err))
				return warnings, errors
			}

			restorer = kr.restorers[groupResource]
			if restorer == nil {
				glog.Infof("Using default restorer for %s", groupResource.String())
				restorer = restorers.NewBasicRestorer(true)
			} else {
				glog.Infof("Using custom restorer for %s", groupResource.String())
			}

			if restorer.Wait() {
				itmWatch, err := resourceClient.Watch(metav1.ListOptions{})
				if err != nil {
					addArkError(&errors, fmt.Errorf("error watching for namespace %q, resource %q: %v", namespace, groupResource.String(), err))
					return warnings, errors
				}
				watchChan := itmWatch.ResultChan()
				defer itmWatch.Stop()

				waiter = newResourceWaiter(watchChan, restorer.Ready)
			}
		}

		if !restorer.Handles(obj, restore) {
			continue
		}

		if hasControllerOwner(obj.GetOwnerReferences()) {
			glog.V(4).Infof("%s/%s has a controller owner - skipping", obj.GetNamespace(), obj.GetName())
			continue
		}

		preparedObj, err := restorer.Prepare(obj, restore, backup)
		if err != nil {
			addToResult(&errors, namespace, fmt.Errorf("error preparing %s: %v", fullPath, err))
			continue
		}

		unstructuredObj, ok := preparedObj.(*unstructured.Unstructured)
		if !ok {
			addToResult(&errors, namespace, fmt.Errorf("%s: unexpected type %T", fullPath, preparedObj))
			continue
		}

		// necessary because we may have remapped the namespace
		unstructuredObj.SetNamespace(namespace)

		// add an ark-restore label to each resource for easy ID
		addLabel(unstructuredObj, api.RestoreLabelKey, restore.Name)

		glog.Infof("Restoring item %v", unstructuredObj.GetName())
		_, err = resourceClient.Create(unstructuredObj)
		if apierrors.IsAlreadyExists(err) {
			addToResult(&warnings, namespace, err)
			continue
		}
		if err != nil {
			glog.Errorf("error restoring %s: %v", unstructuredObj.GetName(), err)
			addToResult(&errors, namespace, fmt.Errorf("error restoring %s: %v", fullPath, err))
			continue
		}

		if waiter != nil {
			waiter.RegisterItem(unstructuredObj.GetName())
		}
	}

	if waiter != nil {
		if err := waiter.Wait(); err != nil {
			addArkError(&errors, fmt.Errorf("error waiting for all %s resources to be created in namespace %s: %v", groupResource.String(), namespace, err))
		}
	}

	return warnings, errors
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
func (kr *kubernetesRestorer) unmarshal(filePath string) (*unstructured.Unstructured, error) {
	var obj unstructured.Unstructured

	bytes, err := kr.fileSystem.ReadFile(filePath)
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
func (kr *kubernetesRestorer) unzipAndExtractBackup(src io.Reader) (string, error) {
	gzr, err := gzip.NewReader(src)
	if err != nil {
		glog.Errorf("error creating gzip reader: %v", err)
		return "", err
	}
	defer gzr.Close()

	return kr.readBackup(tar.NewReader(gzr))
}

// readBackup extracts a tar reader to a local directory/file tree within a
// temp directory.
func (kr *kubernetesRestorer) readBackup(tarRdr *tar.Reader) (string, error) {
	dir, err := kr.fileSystem.TempDir("", "")
	if err != nil {
		glog.Errorf("error creating temp dir: %v", err)
		return "", err
	}

	for {
		header, err := tarRdr.Next()

		if err == io.EOF {
			glog.Infof("end of tar")
			break
		}
		if err != nil {
			glog.Errorf("error reading tar: %v", err)
			return "", err
		}

		target := path.Join(dir, header.Name)

		switch header.Typeflag {
		case tar.TypeDir:
			err := kr.fileSystem.MkdirAll(target, header.FileInfo().Mode())
			if err != nil {
				glog.Errorf("mkdirall error: %v", err)
				return "", err
			}

		case tar.TypeReg:
			// make sure we have the directory created
			err := kr.fileSystem.MkdirAll(path.Dir(target), header.FileInfo().Mode())
			if err != nil {
				glog.Errorf("mkdirall error: %v", err)
				return "", err
			}

			// create the file
			file, err := kr.fileSystem.Create(target)
			if err != nil {
				return "", err
			}
			defer file.Close()

			if _, err := io.Copy(file, tarRdr); err != nil {
				glog.Errorf("error copying: %v", err)
				return "", err
			}
		}
	}

	return dir, nil
}
