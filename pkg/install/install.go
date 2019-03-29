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

package install

import (
	"fmt"
	"io"
	"strings"
	"time"

	appsv1beta1 "k8s.io/api/apps/v1beta1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/wait"

	"github.com/pkg/errors"

	apiextv1beta1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/heptio/velero/pkg/client"
)

// kindToResource translates a Kind (mixed case, singular) to a Resource (lowercase, plural) string.
// This is to accomodate the dynamic client's need for an APIResource, as the Unstructured objects do not have easy helpers for this information.
var kindToResource = map[string]string{
	"CustomResourceDefinition": "customresourcedefinitions",
	"Namespace":                "namespaces",
	"ClusterRoleBinding":       "clusterrolebindings",
	"ServiceAccount":           "serviceaccounts",
	"Deployment":               "deployments",
	"DaemonSet":                "daemonsets",
	"Secret":                   "secrets",
	"BackupStorageLocation":    "backupstoragelocations",
	"VolumeSnapshotLocation":   "volumesnapshotlocations",
}

// ResourceGroup represents a collection of kubernetes objects with a common ready conditon
type ResourceGroup struct {
	Resources []*unstructured.Unstructured
	Ready     func(client.DynamicFactory) (bool, error)
}

// crdIsReady checks a CRD to see if it's ready, so that objects may be created from it.
func crdIsReady(crd apiextv1beta1.CustomResourceDefinition) bool {
	var isEstablished, namesAccepted bool
	for _, cond := range crd.Status.Conditions {
		if cond.Type == apiextv1beta1.Established {
			isEstablished = true
		}
		if cond.Type == apiextv1beta1.NamesAccepted {
			namesAccepted = true
		}
	}

	return (isEstablished && namesAccepted)
}

// crdsAreReady polls the API server to see if the BackupStorageLocation and VolumeSnapshotLocation CRDs are ready to create objects.
func crdsAreReady(factory client.DynamicFactory) (bool, error) {
	gvk := schema.FromAPIVersionAndKind(apiextv1beta1.SchemeGroupVersion.String(), "CustomResourceDefinition")
	apiResource := metav1.APIResource{
		Name:       "customresourcedefinitons",
		Namespaced: false,
	}
	c, err := factory.ClientForGroupVersionResource(gvk.GroupVersion(), apiResource, "")
	if err != nil {
		return false, errors.Wrapf(err, "Error creating client for CustomResourceDefinition polling")
	}
	var isReady bool
	err = wait.PollImmediate(time.Second, time.Minute, func() (bool, error) {
		unstructuredBSL, err := c.Get("backupstoragelocations.velero.io", metav1.GetOptions{})
		if apierrors.IsNotFound(err) {
			return false, nil
		} else if err != nil {
			return false, errors.Wrap(err, "error waiting for backupstoragelocations to be ready")
		}

		bsl := new(apiextv1beta1.CustomResourceDefinition)
		if err := runtime.DefaultUnstructuredConverter.FromUnstructured(unstructuredBSL.Object, bsl); err != nil {
			return false, errors.Wrap(err, "error converting backupstoragelocation from unstructured")
		}

		unstructuredVSL, err := c.Get("volumesnapshotlocations.velero.io", metav1.GetOptions{})
		if apierrors.IsNotFound(err) {
			return false, nil
		} else if err != nil {
			return false, errors.Wrap(err, "error waiting for volumesnapshotlocations to be ready")
		}

		vsl := new(apiextv1beta1.CustomResourceDefinition)
		if err := runtime.DefaultUnstructuredConverter.FromUnstructured(unstructuredVSL.Object, vsl); err != nil {
			return false, errors.Wrap(err, "error converting volumesnapshotlocation from unstructured")
		}

		isReady = (crdIsReady(*bsl) && crdIsReady(*vsl))

		return true, nil
	})
	return isReady, err
}

func deploymentIsReady(factory client.DynamicFactory) (bool, error) {
	gvk := schema.FromAPIVersionAndKind(appsv1beta1.SchemeGroupVersion.String(), "Deployment")
	apiResource := metav1.APIResource{
		Name:       "deployments",
		Namespaced: true,
	}
	c, err := factory.ClientForGroupVersionResource(gvk.GroupVersion(), apiResource, "velero")
	if err != nil {
		return false, errors.Wrapf(err, "Error creating client for deployment polling")
	}
	// declare this variable out of scope so we can return it
	var isReady bool
	err = wait.PollImmediate(time.Second, time.Minute, func() (bool, error) {
		unstructuredDeployment, err := c.Get("velero", metav1.GetOptions{})
		if apierrors.IsNotFound(err) {
			return false, nil
		} else if err != nil {
			return false, errors.Wrap(err, "error waiting for deployment to be ready")
		}

		deploy := new(appsv1beta1.Deployment)
		if err := runtime.DefaultUnstructuredConverter.FromUnstructured(unstructuredDeployment.Object, deploy); err != nil {
			return false, errors.Wrap(err, "error converting deployment from unstructured")
		}

		for _, cond := range deploy.Status.Conditions {
			if cond.Type == appsv1beta1.DeploymentAvailable {
				isReady = true
			}
		}
		return true, nil
	})
	return isReady, err
}

// GroupResources groups resources into ResourcesGroups based on whether the resources are CustomResourceDefinitions or other types of kubernetes objects
// This is useful to wait for readiness before creating CRD objects
func GroupResources(resources *unstructured.UnstructuredList) []*ResourceGroup {
	crdObjs := new(ResourceGroup)
	crdObjs.Ready = crdsAreReady
	otherObjs := new(ResourceGroup)
	otherObjs.Ready = deploymentIsReady

	for _, r := range resources.Items {
		fmt.Printf("Sorting %s/%s\n", r.GetKind(), r.GetName())
		if r.GetKind() == "CustomResourceDefinition" {
			crdObjs.Resources = append(crdObjs.Resources, &r)
			fmt.Printf("Put %s/%s in crdObjs\n", r.GetKind(), r.GetName())
			fmt.Printf("crdObjs looks like:\n")
			for _, c := range crdObjs.Resources {
				fmt.Printf("%+v\n", c)
				fmt.Printf("> %s/%s\n", c.GetKind(), c.GetName())
			}
			continue
		}
		otherObjs.Resources = append(otherObjs.Resources, &r)
		fmt.Printf("Put %s/%s in otherObjs\n", r.GetKind(), r.GetName())
		fmt.Printf("otherObjs looks like:\n")
		for _, c := range otherObjs.Resources {
			fmt.Printf("%+v\n", c)
			fmt.Printf("> %s/%s\n", c.GetKind(), c.GetName())
		}
	}
	fmt.Printf("%d Otherobjects after sorting\n", len(otherObjs.Resources))
	fmt.Printf("%d CRDobjects after sorting\n", len(crdObjs.Resources))
	// for _, c := range crdObjs.Resources {
	// 	fmt.Printf("%+v\n", c)
	// 	fmt.Printf("> %s/%s\n", c.GetKind(), c.GetName())
	// }

	return []*ResourceGroup{crdObjs, otherObjs}
}

// Install creates resources on the Kubernetes cluster.
// An unstructured list of resources is sent, one at a time, to the server. These are assumed to be in the preferred order already.
// An io.Writer can be used to output to a log or the console.
func Install(factory client.DynamicFactory, resources *unstructured.UnstructuredList, w io.Writer) error {
	// Loop over our items on a per-group basis.
	for _, group := range GroupResources(resources) {
		for _, r := range group.Resources {
			fmt.Printf("%s/%s\n", r.GetKind(), r.GetName())
		}
		for _, r := range group.Resources {
			id := fmt.Sprintf("%s/%s", r.GetKind(), r.GetName())

			// Helper to reduce boilerplate message about the same object
			log := func(f string, a ...interface{}) {
				format := strings.Join([]string{id, ": ", f, "\n"}, "")
				fmt.Fprintf(w, format, a...)
			}
			log("attempting to create resource")

			gvk := schema.FromAPIVersionAndKind(r.GetAPIVersion(), r.GetKind())

			apiResource := metav1.APIResource{
				Name:       kindToResource[r.GetKind()],
				Namespaced: (r.GetNamespace() != ""),
			}

			c, err := factory.ClientForGroupVersionResource(gvk.GroupVersion(), apiResource, r.GetNamespace())
			if err != nil {
				return errors.Wrapf(err, "Error creating client for resource %s", id)
			}

			_, err = c.Create(r)
			if apierrors.IsAlreadyExists(err) {
				log("already exists, proceeding")
				continue
			} else if err != nil {
				return errors.Wrapf(err, "Error creating resource %s", id)
			}

			log("created")
		}
		fmt.Fprint(w, "Waiting for resources to be ready in cluster...")
		_, err := group.Ready(factory)
		// Covers the err.WaitForTimeout case
		if err != nil {
			return err
		}
	}
	return nil
}
