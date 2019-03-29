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

	"github.com/pkg/errors"

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

// Install creates resources on the Kubernetes cluster.
// An unstructured list of resources is sent, one at a time, to the server. These are assumed to be in the preferred order already.
// An io.Writer can be used to output to a log or the console.
func Install(factory client.DynamicFactory, resources *unstructured.UnstructuredList, w io.Writer) error {

	for _, r := range resources.Items {
		id := fmt.Sprintf("%s/%s", r.GetKind(), r.GetName())

		// Helper to reduce boilerplate message about the same object
		log := func(f string, a ...interface{}) {
			format := strings.Join([]string{id, ": ", f, "\n"}, "")
			if len(a) > 0 {
				fmt.Fprintf(w, format, a)
			} else {
				fmt.Fprintf(w, format)
			}
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

		_, err = c.Create(&r)
		if apierrors.IsAlreadyExists(err) {
			log("already exists, proceeding")
			continue
		} else if err != nil {
			return errors.Wrapf(err, "Error creating resource %s", id)
		}

		log("created", id)
	}
	return nil
}
