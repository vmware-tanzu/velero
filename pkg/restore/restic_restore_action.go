/*
Copyright 2018, 2019 the Velero contributors.

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
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	corev1client "k8s.io/client-go/kubernetes/typed/core/v1"

	"github.com/heptio/velero/pkg/buildinfo"
	"github.com/heptio/velero/pkg/plugin/framework"
	"github.com/heptio/velero/pkg/plugin/velero"
	"github.com/heptio/velero/pkg/restic"
	"github.com/heptio/velero/pkg/util/kube"
)

const defaultImageBase = "gcr.io/heptio-images/velero-restic-restore-helper"

type ResticRestoreAction struct {
	logger logrus.FieldLogger
	client corev1client.ConfigMapInterface
}

func NewResticRestoreAction(logger logrus.FieldLogger, client corev1client.ConfigMapInterface) *ResticRestoreAction {
	return &ResticRestoreAction{
		logger: logger,
		client: client,
	}
}

func (a *ResticRestoreAction) AppliesTo() (velero.ResourceSelector, error) {
	return velero.ResourceSelector{
		IncludedResources: []string{"pods"},
	}, nil
}

func (a *ResticRestoreAction) Execute(input *velero.RestoreItemActionExecuteInput) (*velero.RestoreItemActionExecuteOutput, error) {
	a.logger.Info("Executing ResticRestoreAction")
	defer a.logger.Info("Done executing ResticRestoreAction")

	var pod corev1.Pod
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(input.Item.UnstructuredContent(), &pod); err != nil {
		return nil, errors.Wrap(err, "unable to convert pod from runtime.Unstructured")
	}

	log := a.logger.WithField("pod", kube.NamespaceAndName(&pod))

	volumeSnapshots := restic.GetPodSnapshotAnnotations(&pod)
	if len(volumeSnapshots) == 0 {
		log.Debug("No restic snapshot ID annotations found")
		return velero.NewRestoreItemActionExecuteOutput(input.Item), nil
	}

	log.Info("Restic snapshot ID annotations found")

	// TODO we might want/need to get plugin config at the top of this method at some point; for now, wait
	// until we know we're doing a restore before getting config.
	log.Debugf("Getting plugin config")
	config, err := getPluginConfig(framework.PluginKindRestoreItemAction, "velero.io/restic", a.client)
	if err != nil {
		return nil, err
	}

	image := getImage(log, config)
	log.Infof("Using image %q", image)

	initContainer := corev1.Container{
		Name:  restic.InitContainer,
		Image: image,
		Args:  []string{string(input.Restore.UID)},
		Env: []corev1.EnvVar{
			{
				Name: "POD_NAMESPACE",
				ValueFrom: &corev1.EnvVarSource{
					FieldRef: &corev1.ObjectFieldSelector{
						FieldPath: "metadata.namespace",
					},
				},
			},
			{
				Name: "POD_NAME",
				ValueFrom: &corev1.EnvVarSource{
					FieldRef: &corev1.ObjectFieldSelector{
						FieldPath: "metadata.name",
					},
				},
			},
		},
	}

	for volumeName := range volumeSnapshots {
		mount := corev1.VolumeMount{
			Name:      volumeName,
			MountPath: "/restores/" + volumeName,
		}
		initContainer.VolumeMounts = append(initContainer.VolumeMounts, mount)
	}

	if len(pod.Spec.InitContainers) == 0 || pod.Spec.InitContainers[0].Name != restic.InitContainer {
		pod.Spec.InitContainers = append([]corev1.Container{initContainer}, pod.Spec.InitContainers...)
	} else {
		pod.Spec.InitContainers[0] = initContainer
	}

	res, err := runtime.DefaultUnstructuredConverter.ToUnstructured(&pod)
	if err != nil {
		return nil, errors.Wrap(err, "unable to convert pod to runtime.Unstructured")
	}

	return velero.NewRestoreItemActionExecuteOutput(&unstructured.Unstructured{Object: res}), nil
}

func getImage(log logrus.FieldLogger, config *corev1.ConfigMap) string {
	if config == nil {
		log.Debug("No config found for plugin")
		return initContainerImage(defaultImageBase)
	}

	image := config.Data["image"]
	if image == "" {
		log.Debugf("No custom image configured")
		return initContainerImage(defaultImageBase)
	}

	log = log.WithField("image", image)

	parts := strings.Split(image, ":")
	switch {
	case len(parts) == 1:
		// tag-less image name: add tag
		log.Debugf("Plugin config contains image name without tag. Adding tag.")
		return initContainerImage(image)
	case len(parts) == 2:
		// tagged image name
		log.Debugf("Plugin config contains image name with tag")
		return image
	default:
		// unrecognized
		log.Warnf("Plugin config contains unparseable image name")
		return initContainerImage(defaultImageBase)
	}
}

// TODO eventually this can move to pkg/plugin/framework since it'll be used across multiple
// plugins.
func getPluginConfig(kind framework.PluginKind, name string, client corev1client.ConfigMapInterface) (*corev1.ConfigMap, error) {
	opts := metav1.ListOptions{
		// velero.io/plugin-config: true
		// velero.io/restic: RestoreItemAction
		LabelSelector: fmt.Sprintf("velero.io/plugin-config,%s=%s", name, kind),
	}

	list, err := client.List(opts)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	if len(list.Items) == 0 {
		return nil, nil
	}

	if len(list.Items) > 1 {
		var items []string
		for _, item := range list.Items {
			items = append(items, item.Name)
		}
		return nil, errors.Errorf("found more than one ConfigMap matching label selector %q: %v", opts.LabelSelector, items)
	}

	return &list.Items[0], nil
}

func initContainerImage(imageBase string) string {
	tag := buildinfo.Version
	if tag == "" {
		tag = "latest"
	}

	return fmt.Sprintf("%s:%s", imageBase, tag)
}
