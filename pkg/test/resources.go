/*
Copyright 2019 the Velero contributors.

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

package test

import (
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
)

// APIResource stores information about a specific Kubernetes API
// resource.
type APIResource struct {
	Group      string
	Version    string
	Name       string
	ShortName  string
	Namespaced bool
	Items      []metav1.Object
}

// GVR returns a GroupVersionResource representing the resource.
func (r *APIResource) GVR() schema.GroupVersionResource {
	return schema.GroupVersionResource{
		Group:    r.Group,
		Version:  r.Version,
		Resource: r.Name,
	}
}

// Pods returns an APIResource describing core/v1's Pods.
func Pods(items ...metav1.Object) *APIResource {
	return &APIResource{
		Group:      "",
		Version:    "v1",
		Name:       "pods",
		ShortName:  "po",
		Namespaced: true,
		Items:      items,
	}
}

func PVCs(items ...metav1.Object) *APIResource {
	return &APIResource{
		Group:      "",
		Version:    "v1",
		Name:       "persistentvolumeclaims",
		ShortName:  "pvc",
		Namespaced: true,
		Items:      items,
	}
}

func PVs(items ...metav1.Object) *APIResource {
	return &APIResource{
		Group:      "",
		Version:    "v1",
		Name:       "persistentvolumes",
		ShortName:  "pv",
		Namespaced: false,
		Items:      items,
	}
}

func Secrets(items ...metav1.Object) *APIResource {
	return &APIResource{
		Group:      "",
		Version:    "v1",
		Name:       "secrets",
		ShortName:  "secrets",
		Namespaced: true,
		Items:      items,
	}
}

func Deployments(items ...metav1.Object) *APIResource {
	return &APIResource{
		Group:      "apps",
		Version:    "v1",
		Name:       "deployments",
		ShortName:  "deploy",
		Namespaced: true,
		Items:      items,
	}
}

func ExtensionsDeployments(items ...metav1.Object) *APIResource {
	return &APIResource{
		Group:      "extensions",
		Version:    "v1",
		Name:       "deployments",
		ShortName:  "deploy",
		Namespaced: true,
		Items:      items,
	}
}

func Namespaces(items ...metav1.Object) *APIResource {
	return &APIResource{
		Group:      "",
		Version:    "v1",
		Name:       "namespaces",
		ShortName:  "ns",
		Namespaced: false,
		Items:      items,
	}
}

func ServiceAccounts(items ...metav1.Object) *APIResource {
	return &APIResource{
		Group:      "",
		Version:    "v1",
		Name:       "serviceaccounts",
		ShortName:  "sa",
		Namespaced: true,
		Items:      items,
	}
}

type ObjectOpts func(metav1.Object)

func NewPod(ns, name string, opts ...ObjectOpts) *corev1.Pod {
	obj := &corev1.Pod{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Pod",
			APIVersion: "v1",
		},
		ObjectMeta: objectMeta(ns, name),
	}

	for _, opt := range opts {
		opt(obj)
	}

	return obj
}

func NewPVC(ns, name string, opts ...ObjectOpts) *corev1.PersistentVolumeClaim {
	obj := &corev1.PersistentVolumeClaim{
		TypeMeta: metav1.TypeMeta{
			Kind:       "PersistentVolumeClaim",
			APIVersion: "v1",
		},
		ObjectMeta: objectMeta(ns, name),
	}

	for _, opt := range opts {
		opt(obj)
	}

	return obj
}

func NewPV(name string, opts ...ObjectOpts) *corev1.PersistentVolume {
	obj := &corev1.PersistentVolume{
		TypeMeta: metav1.TypeMeta{
			Kind:       "PersistentVolume",
			APIVersion: "v1",
		},
		ObjectMeta: objectMeta("", name),
	}

	for _, opt := range opts {
		opt(obj)
	}

	return obj
}

func NewSecret(ns, name string, opts ...ObjectOpts) *corev1.Secret {
	obj := &corev1.Secret{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Secret",
			APIVersion: "v1",
		},
		ObjectMeta: objectMeta(ns, name),
	}

	for _, opt := range opts {
		opt(obj)
	}

	return obj
}

func NewDeployment(ns, name string, opts ...ObjectOpts) *appsv1.Deployment {
	obj := &appsv1.Deployment{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Deployment",
			APIVersion: "apps/v1",
		},
		ObjectMeta: objectMeta(ns, name),
	}

	for _, opt := range opts {
		opt(obj)
	}

	return obj
}

func NewServiceAccount(ns, name string, opts ...ObjectOpts) *corev1.ServiceAccount {
	obj := &corev1.ServiceAccount{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ServiceAccount",
			APIVersion: "v1",
		},
		ObjectMeta: objectMeta(ns, name),
	}

	for _, opt := range opts {
		opt(obj)
	}

	return obj
}

func NewNamespace(name string, opts ...ObjectOpts) *corev1.Namespace {
	obj := &corev1.Namespace{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Namespace",
			APIVersion: "v1",
		},
		ObjectMeta: objectMeta("", name),
	}

	for _, opt := range opts {
		opt(obj)
	}

	return obj
}

func NewConfigMap(ns, name string, opts ...ObjectOpts) *corev1.ConfigMap {
	obj := &corev1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ConfigMap",
			APIVersion: "v1",
		},
		ObjectMeta: objectMeta(ns, name),
	}

	for _, opt := range opts {
		opt(obj)
	}

	return obj
}

func NewStorageClass(name string, opts ...ObjectOpts) *storagev1.StorageClass {
	obj := &storagev1.StorageClass{
		TypeMeta: metav1.TypeMeta{
			Kind:       "StorageClass",
			APIVersion: "storage/v1",
		},
		ObjectMeta: objectMeta("", name),
	}

	for _, opt := range opts {
		opt(obj)
	}

	return obj
}

// VolumeOpts exists because corev1.Volume does not implement metav1.Object
type VolumeOpts func(*corev1.Volume)

func NewVolume(name string, opts ...VolumeOpts) *corev1.Volume {
	obj := &corev1.Volume{Name: name}

	for _, opt := range opts {
		opt(obj)
	}

	return obj
}

// ContainerOpts exists because corev1.Container does not implement metav1.Object
type ContainerOpts func(*corev1.Container)

func NewContainer(name string, opts ...ContainerOpts) *corev1.Container {
	obj := &corev1.Container{Name: name}

	for _, opt := range opts {
		opt(obj)
	}

	return obj
}

func objectMeta(ns, name string) metav1.ObjectMeta {
	return metav1.ObjectMeta{
		Namespace: ns,
		Name:      name,
	}
}

// WithLabels is a functional option that applies the specified
// label keys/values to an object.
func WithLabels(labels ...string) func(obj metav1.Object) {
	return func(obj metav1.Object) {
		objLabels := obj.GetLabels()
		if objLabels == nil {
			objLabels = make(map[string]string)
		}

		if len(labels)%2 != 0 {
			labels = append(labels, "")
		}

		for i := 0; i < len(labels); i += 2 {
			objLabels[labels[i]] = labels[i+1]
		}

		obj.SetLabels(objLabels)
	}
}

// WithAnnotations is a functional option that applies the specified
// annotation keys/values to an object.
func WithAnnotations(vals ...string) func(obj metav1.Object) {
	return func(obj metav1.Object) {
		objAnnotations := obj.GetAnnotations()
		if objAnnotations == nil {
			objAnnotations = make(map[string]string)
		}

		if len(vals)%2 != 0 {
			vals = append(vals, "")
		}

		for i := 0; i < len(vals); i += 2 {
			objAnnotations[vals[i]] = vals[i+1]
		}

		obj.SetAnnotations(objAnnotations)
	}
}

// WithClusterName is a functional option that applies the specified
// cluster name to an object.
func WithClusterName(val string) func(obj metav1.Object) {
	return func(obj metav1.Object) {
		obj.SetClusterName(val)
	}
}

// WithFinalizers is a functional option that applies the specified
// finalizers to an object.
func WithFinalizers(vals ...string) func(obj metav1.Object) {
	return func(obj metav1.Object) {
		obj.SetFinalizers(vals)
	}
}

// WithDeletionTimestamp is a functional option that applies the specified
// deletion timestamp to an object.
func WithDeletionTimestamp(val time.Time) func(obj metav1.Object) {
	return func(obj metav1.Object) {
		obj.SetDeletionTimestamp(&metav1.Time{Time: val})
	}
}

// WithUID is a functional option that applies the specified UID to an object.
func WithUID(val string) func(obj metav1.Object) {
	return func(obj metav1.Object) {
		obj.SetUID(types.UID(val))
	}
}

// WithReclaimPolicy is a functional option for persistent volumes that sets
// the specified reclaim policy. It panics if the object is not a persistent
// volume.
func WithReclaimPolicy(policy corev1.PersistentVolumeReclaimPolicy) func(obj metav1.Object) {
	return func(obj metav1.Object) {
		pv, ok := obj.(*corev1.PersistentVolume)
		if !ok {
			panic("WithReclaimPolicy is only valid for persistent volumes")
		}

		pv.Spec.PersistentVolumeReclaimPolicy = policy
	}
}

// WithClaimRef is a functional option for persistent volumes that sets the specified
// claim ref. It panics if the object is not a persistent volume.
func WithClaimRef(ns, name string) func(obj metav1.Object) {
	return func(obj metav1.Object) {
		pv, ok := obj.(*corev1.PersistentVolume)
		if !ok {
			panic("WithClaimRef is only valid for persistent volumes")
		}

		pv.Spec.ClaimRef = &corev1.ObjectReference{
			Namespace: ns,
			Name:      name,
		}
	}
}

// WithAWSEBSVolumeID is a functional option for persistent volumes that sets the specified
// AWS EBS volume ID. It panics if the object is not a persistent volume.
func WithAWSEBSVolumeID(volumeID string) func(obj metav1.Object) {
	return func(obj metav1.Object) {
		pv, ok := obj.(*corev1.PersistentVolume)
		if !ok {
			panic("WithClaimRef is only valid for persistent volumes")
		}

		if pv.Spec.AWSElasticBlockStore == nil {
			pv.Spec.AWSElasticBlockStore = new(corev1.AWSElasticBlockStoreVolumeSource)
		}

		pv.Spec.AWSElasticBlockStore.VolumeID = volumeID
	}
}

// WithCSI is a functional option for persistent volumes that sets the specified CSI driver name
// and volume handle. It panics if the object is not a persistent volume.
func WithCSI(driver, volumeHandle string) func(object metav1.Object) {
	return func(obj metav1.Object) {
		pv, ok := obj.(*corev1.PersistentVolume)
		if !ok {
			panic("WithCSI is only valid for persistent volumes")
		}
		if pv.Spec.CSI == nil {
			pv.Spec.CSI = new(corev1.CSIPersistentVolumeSource)
		}
		pv.Spec.CSI.Driver = driver
		pv.Spec.CSI.VolumeHandle = volumeHandle
	}
}

// WithPVName is a functional option for persistent volume claims that sets the specified
// persistent volume name. It panics if the object is not a persistent volume claim.
func WithPVName(name string) func(obj metav1.Object) {
	return func(obj metav1.Object) {
		pvc, ok := obj.(*corev1.PersistentVolumeClaim)
		if !ok {
			panic("WithPVName is only valid for persistent volume claims")
		}

		pvc.Spec.VolumeName = name
	}
}

// WithStorageClassName is a functional option for persistent volumes or
// persistent volume claims that sets the specified storage class name.
// It panics if the object is not a persistent volume or persistent volume
// claim.
func WithStorageClassName(name string) func(obj metav1.Object) {
	return func(obj metav1.Object) {
		switch obj.(type) {
		case *corev1.PersistentVolume:
			obj.(*corev1.PersistentVolume).Spec.StorageClassName = name
		case *corev1.PersistentVolumeClaim:
			obj.(*corev1.PersistentVolumeClaim).Spec.StorageClassName = &name
		default:
			panic("WithStorageClassName is only valid for persistent volumes and persistent volume claims")
		}
	}
}

// WithVolume is a functional option for pods that sets the specified
// volume on the pod's Spec. It panics if the object is not a pod.
func WithVolume(volume *corev1.Volume) func(obj metav1.Object) {
	return func(obj metav1.Object) {
		pod, ok := obj.(*corev1.Pod)
		if !ok {
			panic("WithVolume is only valid for pods")
		}
		pod.Spec.Volumes = append(pod.Spec.Volumes, *volume)
	}
}

// WithPVCSource is a functional option for volumes that creates a
// PersistentVolumeClaimVolumeSource with the specified name.
func WithPVCSource(claimName string) func(vol *corev1.Volume) {
	return func(vol *corev1.Volume) {
		vol.VolumeSource.PersistentVolumeClaim = &corev1.PersistentVolumeClaimVolumeSource{ClaimName: claimName}
	}
}

// WithCSISource is a functional option for volumes that creates a
// CSIVolumeSource with the specified driver name.
func WithCSISource(driverName string) func(vol *corev1.Volume) {
	return func(vol *corev1.Volume) {
		vol.VolumeSource.CSI = &corev1.CSIVolumeSource{Driver: driverName}
	}
}

// WithConfigMapData is a functional option for config maps that puts the specified
// values in the Data field. It panics if the object is not a config map.
func WithConfigMapData(vals ...string) func(obj metav1.Object) {
	return func(obj metav1.Object) {
		cm, ok := obj.(*corev1.ConfigMap)
		if !ok {
			panic("WithConfigMapData is only valid for config maps")
		}

		if cm.Data == nil {
			cm.Data = make(map[string]string)
		}

		if len(vals)%2 != 0 {
			vals = append(vals, "")
		}

		for i := 0; i < len(vals); i += 2 {
			cm.Data[vals[i]] = vals[i+1]
		}
	}
}

// WithInitContainer is a functional option that adds an initContainer to a pod.
// It panics if the object is not a pod.
func WithInitContainer(container *corev1.Container) func(obj metav1.Object) {
	return func(obj metav1.Object) {
		pod, ok := obj.(*corev1.Pod)
		if !ok {
			panic("WithInitContainer is only valid for pods")
		}

		pod.Spec.InitContainers = append(pod.Spec.InitContainers, *container)
	}
}
