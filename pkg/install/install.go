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

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"

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
	"BackupStorageLocation":    "backupstoragelocations",
	"VolumeSnapshotLocation":   "volumesnapshotlocations",
}

// Install creates resources on the Kubernetes cluster.
func Install(factory client.DynamicFactory, resources *unstructured.UnstructuredList, logger *logrus.Logger) error {
	for _, r := range resources.Items {
		// TODO: do we want to use the logger, or just a print?
		//logger.WithField("resource", fmt.Sprintf("%s/%s", r.GetKind(), r.GetName())).Info("Creating resource")
		fmt.Printf("Attempting to create resource %s/%s\n", r.GetKind(), r.GetName())

		gvk := schema.FromAPIVersionAndKind(r.GetAPIVersion(), r.GetKind())

		apiResource := metav1.APIResource{
			Name:       kindToResource[r.GetKind()],
			Namespaced: (r.GetNamespace() != ""),
		}

		c, err := factory.ClientForGroupVersionResource(gvk.GroupVersion(), apiResource, r.GetNamespace())
		if err != nil {
			return errors.Wrapf(err, "Error creating client for resource %s/%s", r.GetKind(), r.GetName())
		}

		_, err = c.Create(&r)
		if apierrors.IsAlreadyExists(err) {
			fmt.Printf("Resource %s/%s already exists, proceeding\n", r.GetKind(), r.GetName())
		} else if err != nil {
			return errors.Wrapf(err, "Error creating resource %s/%s", r.GetKind(), r.GetName())
		}
		fmt.Printf("Created %s/%s\n", r.GetKind(), r.GetName())
	}
	return nil
}
