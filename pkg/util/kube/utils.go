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

package kube

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/pkg/errors"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	corev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/pkg/api/v1"

	"github.com/heptio/ark/pkg/util/collections"
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

var ebsVolumeIDRegex = regexp.MustCompile("vol-.*")

var supportedVolumeTypes = map[string]string{
	"awsElasticBlockStore": "volumeID",
	"gcePersistentDisk":    "pdName",
	"azureDisk":            "diskName",
}

// GetVolumeID looks for a supported PV source within the provided PV unstructured
// data. It returns the appropriate volume ID field if found. If the PV source
// is supported but a volume ID cannot be found, an error is returned; if the PV
// source is not supported, zero values are returned.
func GetVolumeID(pv map[string]interface{}) (string, error) {
	spec, err := collections.GetMap(pv, "spec")
	if err != nil {
		return "", err
	}

	for volumeType, volumeIDKey := range supportedVolumeTypes {
		if pvSource, err := collections.GetMap(spec, volumeType); err == nil {
			volumeID, err := collections.GetString(pvSource, volumeIDKey)
			if err != nil {
				return "", err
			}

			if volumeType == "awsElasticBlockStore" {
				return ebsVolumeIDRegex.FindString(volumeID), nil
			}

			return volumeID, nil
		}
	}

	return "", nil
}

// GetPVSource looks for a supported PV source within the provided PV spec data.
// It returns the name of the PV source type and the unstructured source data if
// one is found, or zero values otherwise.
func GetPVSource(spec map[string]interface{}) (string, map[string]interface{}) {
	for volumeType := range supportedVolumeTypes {
		if pvSource, found := spec[volumeType]; found {
			return volumeType, pvSource.(map[string]interface{})
		}
	}

	return "", nil
}

// SetVolumeID looks for a supported PV source within the provided PV spec data.
// If sets the appropriate ID field(s) within the source if found, and returns an
// error if a supported PV source is not found.
func SetVolumeID(spec map[string]interface{}, volumeID string) error {
	sourceType, source := GetPVSource(spec)
	if sourceType == "" {
		return errors.New("persistent volume source is not compatible")
	}

	// for azureDisk, we need to do a find-replace within the diskURI (if it exists)
	// to switch the old disk name with the new.
	if sourceType == "azureDisk" {
		uri, err := collections.GetString(source, "diskURI")
		if err == nil {
			priorVolumeID, err := collections.GetString(source, supportedVolumeTypes["azureDisk"])
			if err != nil {
				return err
			}

			source["diskURI"] = strings.Replace(uri, priorVolumeID, volumeID, -1)
		}
	}

	source[supportedVolumeTypes[sourceType]] = volumeID

	return nil
}
