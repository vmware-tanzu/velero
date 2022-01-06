/*
Copyright 2022 the Velero contributors.

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
	"fmt"
	"strings"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	appsv1api "k8s.io/api/apps/v1"
	corev1api "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	corev1client "k8s.io/client-go/kubernetes/typed/core/v1"

	"github.com/vmware-tanzu/velero/pkg/plugin/framework"
	"github.com/vmware-tanzu/velero/pkg/plugin/velero"
)

// ChangeImageRegistryAction updates a pod/deploy/statefulset/daemonset/replicaset image registry
// if a mapping is found in the plugin's config map
type ChangeImageRegistryAction struct {
	logger          logrus.FieldLogger
	configMapClient corev1client.ConfigMapInterface
}

// NewChangeImageRegistryAction is the constructor for ChangeImageRegistryAction.
func NewChangeImageRegistryAction(
	logger logrus.FieldLogger,
	configMapClient corev1client.ConfigMapInterface,
) *ChangeImageRegistryAction {
	return &ChangeImageRegistryAction{
		logger:          logger,
		configMapClient: configMapClient,
	}
}

// AppliesTo returns the resources that ChangeImageRegistryAction should
// be run for.
func (a *ChangeImageRegistryAction) AppliesTo() (velero.ResourceSelector, error) {
	return velero.ResourceSelector{
		IncludedResources: []string{"pods", "deployments", "statefulsets", "daemonsets", "replicasets"},
	}, nil
}

// Execute updates the item's container's image if a mapping is found
// in the config map for the plugin.
func (a *ChangeImageRegistryAction) Execute(input *velero.RestoreItemActionExecuteInput) (*velero.RestoreItemActionExecuteOutput, error) {
	a.logger.Info("Executing ChangeImageRegistryAction")
	defer a.logger.Info("Done executing ChangeImageRegistryAction")

	a.logger.Debug("Getting plugin config")
	config, err := getPluginConfig(framework.PluginKindRestoreItemAction, "velero.io/change-image-registry", a.configMapClient)
	if err != nil {
		return nil, err
	}

	if config == nil || len(config.Annotations) == 0 {
		a.logger.Debug("No image-registry mappings found")
		return velero.NewRestoreItemActionExecuteOutput(input.Item), nil
	}

	obj, ok := input.Item.(*unstructured.Unstructured)
	if !ok {
		return nil, errors.Errorf("object was of unexpected type %T", input.Item)
	}

	log := a.logger.WithFields(map[string]interface{}{
		"kind":      obj.GetKind(),
		"namespace": obj.GetNamespace(),
		"name":      obj.GetName(),
	})

	switch obj.GetKind() {
	case "Pod":
		if err = a.changeImageRegistryForPod(log, obj, config); err != nil {
			return nil, err
		}
	case "Deployment":
		if err = a.changeImageRegistryForDeploy(log, obj, config); err != nil {
			return nil, err
		}
	case "ReplicaSet":
		if err = a.changeImageRegistryForReplicaSet(log, obj, config); err != nil {
			return nil, err
		}
	case "StatefulSet":
		if err = a.changeImageRegistryForStatefulset(log, obj, config); err != nil {
			return nil, err
		}
	case "DaemonSet":
		if err = a.changeImageRegistryForDaemonset(log, obj, config); err != nil {
			return nil, err
		}
	default:
		notSupportedMsg := fmt.Sprintf("The kind %s image registry conversion is not supported.", obj.GetKind())
		log.Error(notSupportedMsg)
		return nil, errors.New(notSupportedMsg)
	}

	return velero.NewRestoreItemActionExecuteOutput(obj), nil
}

func (a *ChangeImageRegistryAction) changeImageRegistryForPod(logger *logrus.Entry, obj *unstructured.Unstructured, configmap *corev1api.ConfigMap) error {
	pod := new(corev1api.Pod)
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(obj.UnstructuredContent(), pod); err != nil {
		return err
	}

	for k, container := range pod.Spec.Containers {
		for inputOldImageRegistry, inputNewImageRegistry := range configmap.Annotations {
			newImage := getNewImage(container.Image, inputOldImageRegistry, inputNewImageRegistry)
			if newImage == "" {
				// If empty, do nothing.
				continue
			} else {
				pod.Spec.Containers[k].Image = newImage
				logger.Infof("change pod %s container %s image to %s", pod.Name, container.Name, newImage)			}
		}
	}

	newObj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(pod)
	if err != nil {
		return err
	}
	obj.Object = newObj

	return nil
}

func (a *ChangeImageRegistryAction) changeImageRegistryForDeploy(logger *logrus.Entry, obj *unstructured.Unstructured, configmap *corev1api.ConfigMap) error {
	deploy := new(appsv1api.Deployment)
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(obj.UnstructuredContent(), deploy); err != nil {
		return err
	}

	for k, container := range deploy.Spec.Template.Spec.Containers {
		for inputOldImageRegistry, inputNewImageRegistry := range configmap.Annotations {
			newImage := getNewImage(container.Image, inputOldImageRegistry, inputNewImageRegistry)
			if newImage == "" {
				// If empty, do nothing.
				continue
			} else {
				deploy.Spec.Template.Spec.Containers[k].Image = newImage
				logger.Infof("change deployment %s container %s image to %s", deploy.Name, container.Name, newImage)
			}
		}
	}

	newObj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(deploy)
	if err != nil {
		return err
	}
	obj.Object = newObj

	return nil
}

func (a *ChangeImageRegistryAction) changeImageRegistryForReplicaSet(logger *logrus.Entry, obj *unstructured.Unstructured, configmap *corev1api.ConfigMap) error {
	replicaset := new(appsv1api.ReplicaSet)
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(obj.UnstructuredContent(), replicaset); err != nil {
		return err
	}

	for k, container := range replicaset.Spec.Template.Spec.Containers {
		for inputOldImageRegistry, inputNewImageRegistry := range configmap.Annotations {
			newImage := getNewImage(container.Image, inputOldImageRegistry, inputNewImageRegistry)
			if newImage == "" {
				// If empty, do nothing.
				continue
			} else {
				replicaset.Spec.Template.Spec.Containers[k].Image = newImage
				logger.Infof("change replicaset %s container %s image to %s", replicaset.Name, container.Name, newImage)
			}
		}
	}

	newObj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(replicaset)
	if err != nil {
		return err
	}
	obj.Object = newObj

	return nil
}

func (a *ChangeImageRegistryAction) changeImageRegistryForStatefulset(logger *logrus.Entry, obj *unstructured.Unstructured, configmap *corev1api.ConfigMap) error {
	sts := new(appsv1api.StatefulSet)
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(obj.UnstructuredContent(), sts); err != nil {
		return err
	}

	for k, container := range sts.Spec.Template.Spec.Containers {
		for inputOldImageRegistry, inputNewImageRegistry := range configmap.Annotations {
			newImage := getNewImage(container.Image, inputOldImageRegistry, inputNewImageRegistry)
			if newImage == "" {
				// If empty, do nothing.
				continue
			} else {
				sts.Spec.Template.Spec.Containers[k].Image = newImage
				logger.Infof("change statefulset %s container %s image to %s", sts.Name, container.Name, newImage)
			}
		}
	}

	newObj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(sts)
	if err != nil {
		return err
	}
	obj.Object = newObj

	return nil
}

func (a *ChangeImageRegistryAction) changeImageRegistryForDaemonset(logger *logrus.Entry, obj *unstructured.Unstructured, configmap *corev1api.ConfigMap) error {
	ds := new(appsv1api.DaemonSet)
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(obj.UnstructuredContent(), ds); err != nil {
		return err
	}

	for i, container := range ds.Spec.Template.Spec.Containers {
		for inputOldImageRegistry, inputNewImageRegistry := range configmap.Annotations {
			newImage := getNewImage(container.Image, inputOldImageRegistry, inputNewImageRegistry)
			if newImage == "" {
				// If empty, do nothing.
				continue
			} else {
				ds.Spec.Template.Spec.Containers[i].Image = newImage
				logger.Infof("change daemonset %s container %s image to %s", ds.Name, container.Name, newImage)
			}
		}
	}

	newObj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(ds)
	if err != nil {
		return err
	}
	obj.Object = newObj

	return nil
}

// getNewImage returns the final expected image based on the original image passed in, combined with inputOldImageRegistry and inputNewImageRegistry.
// If none match, an empty string is returned.
func getNewImage(oldImage, inputOldImageRegistry, inputNewImageRegistry string) (newImage string) {
	const slash = "/"
	num := strings.LastIndex(oldImage, slash)

	// when inputOldImageRegistry or inputNewImageRegistry last character is "/", we should move it.
	if inputOldImageRegistry[len(inputOldImageRegistry)-1:] == slash {
		inputOldImageRegistry = inputOldImageRegistry[:len(inputOldImageRegistry)-1]
	}

	if inputNewImageRegistry[len(inputNewImageRegistry)-1:] == slash {
		inputNewImageRegistry = inputNewImageRegistry[:len(inputNewImageRegistry)-1]
	}

	var oldImageRegistry string
	if num >= 0 {
		oldImageRegistry = oldImage[0:num]
	}

	if oldImageRegistry == inputOldImageRegistry {
		var newImageName string
		if num < 0 {
			newImageName = slash + oldImage
		} else {
			newImageName = oldImage[num:]
		}
		return inputNewImageRegistry + newImageName
	} else if oldImageRegistry == "" && inputOldImageRegistry == "none" {
		return inputNewImageRegistry + slash + oldImage
	}

	return ""
}
