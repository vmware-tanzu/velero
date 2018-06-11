/*
Copyright 2018 the Heptio Ark contributors.

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

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"

	api "github.com/heptio/ark/pkg/apis/ark/v1"
	"github.com/heptio/ark/pkg/buildinfo"
	"github.com/heptio/ark/pkg/restic"
	"github.com/heptio/ark/pkg/util/kube"
)

type resticRestoreAction struct {
	logger             logrus.FieldLogger
	initContainerImage string
}

func NewResticRestoreAction(logger logrus.FieldLogger) ItemAction {
	return &resticRestoreAction{
		logger:             logger,
		initContainerImage: initContainerImage(),
	}
}

func initContainerImage() string {
	tag := buildinfo.Version
	if tag == "" {
		tag = "latest"
	}

	// TODO allow full image URL to be overriden via CLI flag.
	return fmt.Sprintf("gcr.io/heptio-images/ark-restic-restore-helper:%s", tag)
}

func (a *resticRestoreAction) AppliesTo() (ResourceSelector, error) {
	return ResourceSelector{
		IncludedResources: []string{"pods"},
	}, nil
}

func (a *resticRestoreAction) Execute(obj runtime.Unstructured, restore *api.Restore) (runtime.Unstructured, error, error) {
	a.logger.Info("Executing resticRestoreAction")
	defer a.logger.Info("Done executing resticRestoreAction")

	var pod corev1.Pod
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(obj.UnstructuredContent(), &pod); err != nil {
		return nil, nil, errors.Wrap(err, "unable to convert pod from runtime.Unstructured")
	}

	log := a.logger.WithField("pod", kube.NamespaceAndName(&pod))

	volumeSnapshots := restic.GetPodSnapshotAnnotations(&pod)
	if len(volumeSnapshots) == 0 {
		log.Debug("No restic snapshot ID annotations found")
		return obj, nil, nil
	}

	log.Info("Restic snapshot ID annotations found")

	initContainer := corev1.Container{
		Name:  restic.InitContainer,
		Image: a.initContainerImage,
		Args:  []string{string(restore.UID)},
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

	if len(pod.Spec.InitContainers) == 0 || pod.Spec.InitContainers[0].Name != "restic-wait" {
		pod.Spec.InitContainers = append([]corev1.Container{initContainer}, pod.Spec.InitContainers...)
	} else {
		pod.Spec.InitContainers[0] = initContainer
	}

	res, err := runtime.DefaultUnstructuredConverter.ToUnstructured(&pod)
	if err != nil {
		return nil, nil, errors.Wrap(err, "unable to convert pod to runtime.Unstructured")
	}

	return &unstructured.Unstructured{Object: res}, nil, nil
}
