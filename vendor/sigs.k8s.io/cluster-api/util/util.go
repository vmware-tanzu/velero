/*
Copyright 2017 The Kubernetes Authors.

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

package util

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"math/rand"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/blang/semver"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/version"
	"k8s.io/client-go/rest"
	"k8s.io/klog/klogr"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1alpha3"
	"sigs.k8s.io/cluster-api/util/annotations"
	"sigs.k8s.io/cluster-api/util/container"
	"sigs.k8s.io/cluster-api/util/predicates"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

const (
	// CharSet defines the alphanumeric set for random string generation
	CharSet = "0123456789abcdefghijklmnopqrstuvwxyz"
	// MachineListFormatDeprecationMessage notifies the user that the old
	// MachineList format is no longer supported
	MachineListFormatDeprecationMessage = "Your MachineList items must include Kind and APIVersion"
)

var (
	rnd                          = rand.New(rand.NewSource(time.Now().UnixNano()))
	ErrNoCluster                 = fmt.Errorf("no %q label present", clusterv1.ClusterLabelName)
	ErrUnstructuredFieldNotFound = fmt.Errorf("field not found")
	kubeSemver                   = regexp.MustCompile(`^v?(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)([-0-9a-zA-Z_\.+]*)?$`)
)

// ParseMajorMinorPatch returns a semver.Version from the string provided
// by looking only at major.minor.patch and stripping everything else out.
func ParseMajorMinorPatch(version string) (semver.Version, error) {
	groups := kubeSemver.FindStringSubmatch(version)
	if len(groups) < 4 {
		return semver.Version{}, errors.Errorf("failed to parse major.minor.patch from %q", version)
	}
	major, err := strconv.ParseUint(groups[1], 10, 64)
	if err != nil {
		return semver.Version{}, errors.Wrapf(err, "failed to parse major version from %q", version)
	}
	minor, err := strconv.ParseUint(groups[2], 10, 64)
	if err != nil {
		return semver.Version{}, errors.Wrapf(err, "failed to parse minor version from %q", version)
	}
	patch, err := strconv.ParseUint(groups[3], 10, 64)
	if err != nil {
		return semver.Version{}, errors.Wrapf(err, "failed to parse patch version from %q", version)
	}
	return semver.Version{
		Major: major,
		Minor: minor,
		Patch: patch,
	}, nil
}

// RandomString returns a random alphanumeric string.
func RandomString(n int) string {
	result := make([]byte, n)
	for i := range result {
		result[i] = CharSet[rnd.Intn(len(CharSet))]
	}
	return string(result)
}

// Ordinalize takes an int and returns the ordinalized version of it.
// Eg. 1 --> 1st, 103 --> 103rd
func Ordinalize(n int) string {
	m := map[int]string{
		0: "th",
		1: "st",
		2: "nd",
		3: "rd",
		4: "th",
		5: "th",
		6: "th",
		7: "th",
		8: "th",
		9: "th",
	}

	an := int(math.Abs(float64(n)))
	if an < 10 {
		return fmt.Sprintf("%d%s", n, m[an])
	}
	return fmt.Sprintf("%d%s", n, m[an%10])
}

// ModifyImageRepository takes an imageName (e.g., repository/image:tag), and returns an image name with updated repository
// Deprecated: Please use the functions in util/container
func ModifyImageRepository(imageName, repositoryName string) (string, error) {
	return container.ModifyImageRepository(imageName, repositoryName)
}

// ModifyImageTag takes an imageName (e.g., repository/image:tag), and returns an image name with updated tag
// Deprecated: Please use the functions in util/container
func ModifyImageTag(imageName, tagName string) (string, error) {
	return container.ModifyImageTag(imageName, tagName)
}

// ImageTagIsValid ensures that a given image tag is compliant with the OCI spec
// Deprecated: Please use the functions in util/container
func ImageTagIsValid(tagName string) bool {
	return container.ImageTagIsValid(tagName)
}

// GetMachinesForCluster returns a list of machines associated with the cluster.
func GetMachinesForCluster(ctx context.Context, c client.Client, cluster *clusterv1.Cluster) (*clusterv1.MachineList, error) {
	var machines clusterv1.MachineList
	if err := c.List(
		ctx,
		&machines,
		client.InNamespace(cluster.Namespace),
		client.MatchingLabels{
			clusterv1.ClusterLabelName: cluster.Name,
		},
	); err != nil {
		return nil, err
	}
	return &machines, nil
}

// SemverToOCIImageTag is a helper function that replaces all
// non-allowed symbols in tag strings with underscores.
// Image tag can only contain lowercase and uppercase letters, digits,
// underscores, periods and dashes.
// Current usage is for CI images where all of symbols except '+' are valid,
// but function is for generic usage where input can't be always pre-validated.
// Taken from k8s.io/cmd/kubeadm/app/util
// Deprecated: Please use the functions in util/container
func SemverToOCIImageTag(version string) string {
	return container.SemverToOCIImageTag(version)
}

// GetControlPlaneMachines returns a slice containing control plane machines.
func GetControlPlaneMachines(machines []*clusterv1.Machine) (res []*clusterv1.Machine) {
	for _, machine := range machines {
		if IsControlPlaneMachine(machine) {
			res = append(res, machine)
		}
	}
	return
}

// GetControlPlaneMachinesFromList returns a slice containing control plane machines.
func GetControlPlaneMachinesFromList(machineList *clusterv1.MachineList) (res []*clusterv1.Machine) {
	for i := 0; i < len(machineList.Items); i++ {
		machine := machineList.Items[i]
		if IsControlPlaneMachine(&machine) {
			res = append(res, &machine)
		}
	}
	return
}

// GetMachineIfExists gets a machine from the API server if it exists.
func GetMachineIfExists(c client.Client, namespace, name string) (*clusterv1.Machine, error) {
	if c == nil {
		// Being called before k8s is setup as part of control plane VM creation
		return nil, nil
	}

	// Machines are identified by name
	machine := &clusterv1.Machine{}
	err := c.Get(context.Background(), client.ObjectKey{Namespace: namespace, Name: name}, machine)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return nil, nil
		}
		return nil, err
	}

	return machine, nil
}

// IsControlPlaneMachine checks machine is a control plane node.
func IsControlPlaneMachine(machine *clusterv1.Machine) bool {
	_, ok := machine.ObjectMeta.Labels[clusterv1.MachineControlPlaneLabelName]
	return ok
}

// IsNodeReady returns true if a node is ready.
func IsNodeReady(node *v1.Node) bool {
	for _, condition := range node.Status.Conditions {
		if condition.Type == v1.NodeReady {
			return condition.Status == v1.ConditionTrue
		}
	}

	return false
}

// GetClusterFromMetadata returns the Cluster object (if present) using the object metadata.
func GetClusterFromMetadata(ctx context.Context, c client.Client, obj metav1.ObjectMeta) (*clusterv1.Cluster, error) {
	if obj.Labels[clusterv1.ClusterLabelName] == "" {
		return nil, errors.WithStack(ErrNoCluster)
	}
	return GetClusterByName(ctx, c, obj.Namespace, obj.Labels[clusterv1.ClusterLabelName])
}

// GetOwnerCluster returns the Cluster object owning the current resource.
func GetOwnerCluster(ctx context.Context, c client.Client, obj metav1.ObjectMeta) (*clusterv1.Cluster, error) {
	for _, ref := range obj.OwnerReferences {
		if ref.Kind != "Cluster" {
			continue
		}
		gv, err := schema.ParseGroupVersion(ref.APIVersion)
		if err != nil {
			return nil, errors.WithStack(err)
		}
		if gv.Group == clusterv1.GroupVersion.Group {
			return GetClusterByName(ctx, c, obj.Namespace, ref.Name)
		}
	}
	return nil, nil
}

// GetClusterByName finds and return a Cluster object using the specified params.
func GetClusterByName(ctx context.Context, c client.Client, namespace, name string) (*clusterv1.Cluster, error) {
	cluster := &clusterv1.Cluster{}
	key := client.ObjectKey{
		Namespace: namespace,
		Name:      name,
	}

	if err := c.Get(ctx, key, cluster); err != nil {
		return nil, err
	}

	return cluster, nil
}

// ObjectKey returns client.ObjectKey for the object.
func ObjectKey(object metav1.Object) client.ObjectKey {
	return client.ObjectKey{
		Namespace: object.GetNamespace(),
		Name:      object.GetName(),
	}
}

// ClusterToInfrastructureMapFunc returns a handler.ToRequestsFunc that watches for
// Cluster events and returns reconciliation requests for an infrastructure provider object.
func ClusterToInfrastructureMapFunc(gvk schema.GroupVersionKind) handler.ToRequestsFunc {
	return func(o handler.MapObject) []reconcile.Request {
		c, ok := o.Object.(*clusterv1.Cluster)
		if !ok {
			return nil
		}

		// Return early if the InfrastructureRef is nil.
		if c.Spec.InfrastructureRef == nil {
			return nil
		}
		gk := gvk.GroupKind()
		// Return early if the GroupKind doesn't match what we expect.
		infraGK := c.Spec.InfrastructureRef.GroupVersionKind().GroupKind()
		if gk != infraGK {
			return nil
		}

		return []reconcile.Request{
			{
				NamespacedName: client.ObjectKey{
					Namespace: c.Namespace,
					Name:      c.Spec.InfrastructureRef.Name,
				},
			},
		}
	}
}

// GetOwnerMachine returns the Machine object owning the current resource.
func GetOwnerMachine(ctx context.Context, c client.Client, obj metav1.ObjectMeta) (*clusterv1.Machine, error) {
	for _, ref := range obj.OwnerReferences {
		gv, err := schema.ParseGroupVersion(ref.APIVersion)
		if err != nil {
			return nil, err
		}
		if ref.Kind == "Machine" && gv.Group == clusterv1.GroupVersion.Group {
			return GetMachineByName(ctx, c, obj.Namespace, ref.Name)
		}
	}
	return nil, nil
}

// GetMachineByName finds and return a Machine object using the specified params.
func GetMachineByName(ctx context.Context, c client.Client, namespace, name string) (*clusterv1.Machine, error) {
	m := &clusterv1.Machine{}
	key := client.ObjectKey{Name: name, Namespace: namespace}
	if err := c.Get(ctx, key, m); err != nil {
		return nil, err
	}
	return m, nil
}

// MachineToInfrastructureMapFunc returns a handler.ToRequestsFunc that watches for
// Machine events and returns reconciliation requests for an infrastructure provider object.
func MachineToInfrastructureMapFunc(gvk schema.GroupVersionKind) handler.ToRequestsFunc {
	return func(o handler.MapObject) []reconcile.Request {
		m, ok := o.Object.(*clusterv1.Machine)
		if !ok {
			return nil
		}

		gk := gvk.GroupKind()
		// Return early if the GroupKind doesn't match what we expect.
		infraGK := m.Spec.InfrastructureRef.GroupVersionKind().GroupKind()
		if gk != infraGK {
			return nil
		}

		return []reconcile.Request{
			{
				NamespacedName: client.ObjectKey{
					Namespace: m.Namespace,
					Name:      m.Spec.InfrastructureRef.Name,
				},
			},
		}
	}
}

// HasOwnerRef returns true if the OwnerReference is already in the slice.
func HasOwnerRef(ownerReferences []metav1.OwnerReference, ref metav1.OwnerReference) bool {
	return indexOwnerRef(ownerReferences, ref) > -1
}

// EnsureOwnerRef makes sure the slice contains the OwnerReference.
func EnsureOwnerRef(ownerReferences []metav1.OwnerReference, ref metav1.OwnerReference) []metav1.OwnerReference {
	idx := indexOwnerRef(ownerReferences, ref)
	if idx == -1 {
		return append(ownerReferences, ref)
	}
	ownerReferences[idx] = ref
	return ownerReferences
}

// ReplaceOwnerRef re-parents an object from one OwnerReference to another
// It compares strictly based on UID to avoid reparenting across an intentional deletion: if an object is deleted
// and re-created with the same name and namespace, the only way to tell there was an in-progress deletion
// is by comparing the UIDs.
func ReplaceOwnerRef(ownerReferences []metav1.OwnerReference, source metav1.Object, target metav1.OwnerReference) []metav1.OwnerReference {
	fi := -1
	for index, r := range ownerReferences {
		if r.UID == source.GetUID() {
			fi = index
			ownerReferences[index] = target
			break
		}
	}
	if fi < 0 {
		ownerReferences = append(ownerReferences, target)
	}
	return ownerReferences
}

// RemoveOwnerRef returns the slice of owner references after removing the supplied owner ref
func RemoveOwnerRef(ownerReferences []metav1.OwnerReference, inputRef metav1.OwnerReference) []metav1.OwnerReference {
	if index := indexOwnerRef(ownerReferences, inputRef); index != -1 {
		return append(ownerReferences[:index], ownerReferences[index+1:]...)
	}
	return ownerReferences
}

// indexOwnerRef returns the index of the owner reference in the slice if found, or -1.
func indexOwnerRef(ownerReferences []metav1.OwnerReference, ref metav1.OwnerReference) int {
	for index, r := range ownerReferences {
		if referSameObject(r, ref) {
			return index
		}
	}
	return -1
}

// PointsTo returns true if any of the owner references point to the given target
// Deprecated: Use IsOwnedByObject to cover differences in API version or backup/restore that changed UIDs.
func PointsTo(refs []metav1.OwnerReference, target *metav1.ObjectMeta) bool {
	for _, ref := range refs {
		if ref.UID == target.UID {
			return true
		}
	}
	return false
}

// IsOwnedByObject returns true if any of the owner references point to the given target.
func IsOwnedByObject(obj metav1.Object, target controllerutil.Object) bool {
	for _, ref := range obj.GetOwnerReferences() {
		ref := ref
		if refersTo(&ref, target) {
			return true
		}
	}
	return false
}

// IsControlledBy differs from metav1.IsControlledBy in that it checks the group (but not version), kind, and name vs uid.
func IsControlledBy(obj metav1.Object, owner controllerutil.Object) bool {
	controllerRef := metav1.GetControllerOfNoCopy(obj)
	if controllerRef == nil {
		return false
	}
	return refersTo(controllerRef, owner)
}

// Returns true if a and b point to the same object.
func referSameObject(a, b metav1.OwnerReference) bool {
	aGV, err := schema.ParseGroupVersion(a.APIVersion)
	if err != nil {
		return false
	}

	bGV, err := schema.ParseGroupVersion(b.APIVersion)
	if err != nil {
		return false
	}

	return aGV.Group == bGV.Group && a.Kind == b.Kind && a.Name == b.Name
}

// Returns true if ref refers to obj.
func refersTo(ref *metav1.OwnerReference, obj controllerutil.Object) bool {
	refGv, err := schema.ParseGroupVersion(ref.APIVersion)
	if err != nil {
		return false
	}

	gvk := obj.GetObjectKind().GroupVersionKind()
	return refGv.Group == gvk.Group && ref.Kind == gvk.Kind && ref.Name == obj.GetName()
}

// UnstructuredUnmarshalField is a wrapper around json and unstructured objects to decode and copy a specific field
// value into an object.
func UnstructuredUnmarshalField(obj *unstructured.Unstructured, v interface{}, fields ...string) error {
	value, found, err := unstructured.NestedFieldNoCopy(obj.Object, fields...)
	if err != nil {
		return errors.Wrapf(err, "failed to retrieve field %q from %q", strings.Join(fields, "."), obj.GroupVersionKind())
	}
	if !found || value == nil {
		return ErrUnstructuredFieldNotFound
	}
	valueBytes, err := json.Marshal(value)
	if err != nil {
		return errors.Wrapf(err, "failed to json-encode field %q value from %q", strings.Join(fields, "."), obj.GroupVersionKind())
	}
	if err := json.Unmarshal(valueBytes, v); err != nil {
		return errors.Wrapf(err, "failed to json-decode field %q value from %q", strings.Join(fields, "."), obj.GroupVersionKind())
	}
	return nil
}

// HasOwner checks if any of the references in the passed list match the given apiVersion and one of the given kinds
func HasOwner(refList []metav1.OwnerReference, apiVersion string, kinds []string) bool {
	kMap := make(map[string]bool)
	for _, kind := range kinds {
		kMap[kind] = true
	}

	for _, mr := range refList {
		if mr.APIVersion == apiVersion && kMap[mr.Kind] {
			return true
		}
	}

	return false
}

var (
	// IsPaused returns true if the Cluster is paused or the object has the `paused` annotation.
	// Deprecated: use util/annotations/IsPaused instead
	IsPaused = annotations.IsPaused

	// HasPausedAnnotation returns true if the object has the `paused` annotation.
	// Deprecated: use util/annotations/HasPausedAnnotation instead
	HasPausedAnnotation = annotations.HasPausedAnnotation
)

// GetCRDWithContract retrieves a list of CustomResourceDefinitions from using controller-runtime Client,
// filtering with the `contract` label passed in.
// Returns the first CRD in the list that matches the GroupVersionKind, otherwise returns an error.
func GetCRDWithContract(ctx context.Context, c client.Client, gvk schema.GroupVersionKind, contract string) (*apiextensionsv1.CustomResourceDefinition, error) {
	crdList := &apiextensionsv1.CustomResourceDefinitionList{}
	for {
		if err := c.List(ctx, crdList, client.Continue(crdList.Continue), client.HasLabels{contract}); err != nil {
			return nil, errors.Wrapf(err, "failed to list CustomResourceDefinitions for %v", gvk)
		}

		for _, crd := range crdList.Items {
			if crd.Spec.Group == gvk.Group &&
				crd.Spec.Names.Kind == gvk.Kind {
				return crd.DeepCopy(), nil
			}
		}

		if crdList.Continue == "" {
			break
		}
	}

	return nil, errors.Errorf("failed to find a CustomResourceDefinition for %v with contract %q", gvk, contract)
}

// KubeAwareAPIVersions is a sortable slice of kube-like version strings.
//
// Kube-like version strings are starting with a v, followed by a major version,
// optional "alpha" or "beta" strings followed by a minor version (e.g. v1, v2beta1).
// Versions will be sorted based on GA/alpha/beta first and then major and minor
// versions. e.g. v2, v1, v1beta2, v1beta1, v1alpha1.
type KubeAwareAPIVersions []string

func (k KubeAwareAPIVersions) Len() int      { return len(k) }
func (k KubeAwareAPIVersions) Swap(i, j int) { k[i], k[j] = k[j], k[i] }
func (k KubeAwareAPIVersions) Less(i, j int) bool {
	return version.CompareKubeAwareVersionStrings(k[i], k[j]) < 0
}

// MachinesByCreationTimestamp sorts a list of Machine by creation timestamp, using their names as a tie breaker.
type MachinesByCreationTimestamp []*clusterv1.Machine

func (o MachinesByCreationTimestamp) Len() int      { return len(o) }
func (o MachinesByCreationTimestamp) Swap(i, j int) { o[i], o[j] = o[j], o[i] }
func (o MachinesByCreationTimestamp) Less(i, j int) bool {
	if o[i].CreationTimestamp.Equal(&o[j].CreationTimestamp) {
		return o[i].Name < o[j].Name
	}
	return o[i].CreationTimestamp.Before(&o[j].CreationTimestamp)
}

// WatchOnClusterPaused adds a conditional watch to the controlled given as input
// that sends watch notifications on any create or delete, and only updates
// that toggle Cluster.Spec.Cluster.
// Deprecated: Instead add the Watch directly and use predicates.ClusterUnpaused or
// predicates.ClusterUnpausedAndInfrastructureReady depending on your use case.
func WatchOnClusterPaused(c controller.Controller, mapFunc handler.Mapper) error {
	log := klogr.New().WithName("WatchOnClusterPaused")
	return c.Watch(
		&source.Kind{Type: &clusterv1.Cluster{}},
		&handler.EnqueueRequestsFromMapFunc{
			ToRequests: mapFunc,
		},
		predicates.ClusterUnpaused(log),
	)
}

// ClusterToObjectsMapper returns a mapper function that gets a cluster and lists all objects for the object passed in
// and returns a list of requests.
// NB: The objects are required to have `clusterv1.ClusterLabelName` applied.
func ClusterToObjectsMapper(c client.Client, ro runtime.Object, scheme *runtime.Scheme) (handler.Mapper, error) {
	if _, ok := ro.(metav1.ListInterface); !ok {
		return nil, errors.Errorf("expected a metav1.ListInterface, got %T instead", ro)
	}

	gvk, err := apiutil.GVKForObject(ro, scheme)
	if err != nil {
		return nil, err
	}

	return handler.ToRequestsFunc(func(o handler.MapObject) []ctrl.Request {
		cluster, ok := o.Object.(*clusterv1.Cluster)
		if !ok {
			return nil
		}

		list := &unstructured.UnstructuredList{}
		list.SetGroupVersionKind(gvk)
		if err := c.List(context.Background(), list, client.MatchingLabels{clusterv1.ClusterLabelName: cluster.Name}); err != nil {
			return nil
		}

		results := []ctrl.Request{}
		for _, obj := range list.Items {
			results = append(results, ctrl.Request{
				NamespacedName: client.ObjectKey{Namespace: obj.GetNamespace(), Name: obj.GetName()},
			})
		}
		return results

	}), nil
}

// ObjectReferenceToUnstructured converts an object reference to an unstructured object.
func ObjectReferenceToUnstructured(in corev1.ObjectReference) *unstructured.Unstructured {
	out := &unstructured.Unstructured{}
	out.SetKind(in.Kind)
	out.SetAPIVersion(in.APIVersion)
	out.SetNamespace(in.Namespace)
	out.SetName(in.Name)
	return out
}

// IsSupportedVersionSkew will return true if a and b are no more than one minor version off from each other.
func IsSupportedVersionSkew(a, b semver.Version) bool {
	if a.Major != b.Major {
		return false
	}
	if a.Minor > b.Minor {
		return a.Minor-b.Minor == 1
	}
	return b.Minor-a.Minor <= 1
}

// NewDelegatingClientFunc returns a manager.NewClientFunc to be used when creating
// a new controller runtime manager.
//
// A delegating client reads from the cache and writes directly to the server.
// This avoids getting unstructured objects directly from the server
//
// See issue: https://github.com/kubernetes-sigs/cluster-api/issues/1663
func ManagerDelegatingClientFunc(cache cache.Cache, config *rest.Config, options client.Options) (client.Client, error) {
	c, err := client.New(config, options)
	if err != nil {
		return nil, err
	}
	return &client.DelegatingClient{
		Reader:       cache,
		Writer:       c,
		StatusClient: c,
	}, nil
}
