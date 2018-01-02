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

package kube

import (
	"fmt"

	"github.com/pkg/errors"

	"k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/discovery"
	corev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/kubernetes/pkg/util/version"
)

// NamespaceAndName returns a string in the format <namespace>/<name>
func NamespaceAndName(objMeta metav1.Object) string {
	if objMeta.GetNamespace() == "" {
		return objMeta.GetName()
	}
	return fmt.Sprintf("%s/%s", objMeta.GetNamespace(), objMeta.GetName())
}

// EnsureNamespaceExists attempts to create the provided Kubernetes namespace. It returns two values:
// a bool indicating whether or not the namespace was created, and an error if the create failed
// for a reason other than that the namespace already exists. Note that in the case where the
// namespace already exists, this function will return (false, nil).
func EnsureNamespaceExists(namespace *v1.Namespace, client corev1.NamespaceInterface) (bool, error) {
	if _, err := client.Create(namespace); err == nil {
		return true, nil
	} else if apierrors.IsAlreadyExists(err) {
		return false, nil
	} else {
		return false, errors.Wrapf(err, "error creating namespace %s", namespace.Name)
	}
}

func ServerVersion(client discovery.DiscoveryInterface) (*version.Version, error) {
	versionInfo, err := client.ServerVersion()
	if err != nil {
		return nil, errors.Wrap(err, "error getting server version")
	}

	semVer, err := version.ParseSemantic(versionInfo.String())
	if err != nil {
		return nil, errors.Wrap(err, "error parsing server version")
	}

	return semVer, err
}
