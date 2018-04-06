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

package backup

import (
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"

	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/heptio/ark/pkg/apis/ark/v1"
)

// podAction implements ItemAction.
type podAction struct {
	log logrus.FieldLogger
}

// NewPodAction creates a new ItemAction for pods.
func NewPodAction(log logrus.FieldLogger) ItemAction {
	return &podAction{log: log}
}

var pvcGroupResource = schema.GroupResource{Group: "", Resource: "persistentvolumeclaims"}

// AppliesTo returns a ResourceSelector that applies only to pods.
func (a *podAction) AppliesTo() (ResourceSelector, error) {
	return ResourceSelector{
		IncludedResources: []string{"pods"},
	}, nil
}

// Execute scans the pod's spec.volumes for persistentVolumeClaim volumes and returns a
// ResourceIdentifier list containing references to all of the persistentVolumeClaim volumes used by
// the pod. This ensures that when a pod is backed up, all referenced PVCs are backed up too.
func (a *podAction) Execute(item runtime.Unstructured, backup *v1.Backup) (runtime.Unstructured, []ResourceIdentifier, error) {
	a.log.Info("Executing podAction")
	defer a.log.Info("Done executing podAction")

	volumes, found := unstructured.NestedSlice(item.UnstructuredContent(), "spec", "volumes")
	if !found {
		a.log.Info("pod has no volumes")
		return item, nil, nil
	}

	metadata, err := meta.Accessor(item)
	if err != nil {
		return nil, nil, errors.Wrap(err, "unable to access pod metadata")
	}

	var (
		errs            []error
		additionalItems []ResourceIdentifier
	)

	for i := range volumes {
		volume, ok := volumes[i].(map[string]interface{})
		if !ok {
			errs = append(errs, errors.Errorf("unexpected type %T", volumes[i]))
			continue
		}

		claimName, found := unstructured.NestedString(volume, "persistentVolumeClaim", "claimName")
		if !found {
			continue
		}

		a.log.Infof("Adding pvc %s to additionalItems", claimName)

		additionalItems = append(additionalItems, ResourceIdentifier{
			GroupResource: pvcGroupResource,
			Namespace:     metadata.GetNamespace(),
			Name:          claimName,
		})
	}

	return item, additionalItems, nil
}
