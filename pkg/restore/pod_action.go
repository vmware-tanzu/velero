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

package restore

import (
	"regexp"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"

	corev1api "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"

	api "github.com/heptio/ark/pkg/apis/ark/v1"
)

type podAction struct {
	logger logrus.FieldLogger
}

func NewPodAction(logger logrus.FieldLogger) ItemAction {
	return &podAction{
		logger: logger,
	}
}

func (a *podAction) AppliesTo() (ResourceSelector, error) {
	return ResourceSelector{
		IncludedResources: []string{"pods"},
	}, nil
}

var (
	defaultTokenRegex = regexp.MustCompile("default-token-.*")
)

func (a *podAction) Execute(obj runtime.Unstructured, restore *api.Restore) (runtime.Unstructured, error, error) {
	pod := new(corev1api.Pod)
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(obj.UnstructuredContent(), pod); err != nil {
		return obj, nil, errors.WithStack(err)
	}

	a.logger.Debug("deleting spec.NodeName")
	pod.Spec.NodeName = ""

	a.logger.Debug("iterating over volumes")
	var volumesExDefaultTokens []corev1api.Volume
	for _, volume := range pod.Spec.Volumes {
		log := a.logger.WithField("volumeName", volume.Name)
		log.Debug("Checking volume")

		if defaultTokenRegex.MatchString(volume.Name) {
			log.Debug("Excluding volume")
			continue
		}

		log.Debug("Preserving volume")
		volumesExDefaultTokens = append(volumesExDefaultTokens, volume)
	}

	a.logger.Debug("Setting spec.volumes")
	pod.Spec.Volumes = volumesExDefaultTokens

	a.logger.Debug("iterating over containers")
	for _, container := range pod.Spec.Containers {
		log := a.logger.WithField("containerName", container.Name)
		var volumeMountsExDefaultTokens []corev1api.VolumeMount

		for _, mount := range container.VolumeMounts {
			log = log.WithField("volumeMount", mount.Name)
			log.Debug("Checking volumeMount")

			if defaultTokenRegex.MatchString(mount.Name) {
				log.Debug("Excluding volumeMount")
				continue
			}

			log.Debug("Preserving volumeMount")
			volumeMountsExDefaultTokens = append(volumeMountsExDefaultTokens, mount)
		}

		container.VolumeMounts = volumeMountsExDefaultTokens
	}

	res, err := runtime.DefaultUnstructuredConverter.ToUnstructured(pod)
	if err != nil {
		return obj, nil, err
	}

	return &unstructured.Unstructured{Object: res}, nil, nil
}
