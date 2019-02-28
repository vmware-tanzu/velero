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
	"strings"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"

	api "github.com/heptio/velero/pkg/apis/velero/v1"
)

type podAction struct {
	logger logrus.FieldLogger
}

func NewPodAction(logger logrus.FieldLogger) ItemAction {
	return &podAction{logger: logger}
}

func (a *podAction) AppliesTo() (ResourceSelector, error) {
	return ResourceSelector{
		IncludedResources: []string{"pods"},
	}, nil
}

func (a *podAction) Execute(obj runtime.Unstructured, restore *api.Restore) (runtime.Unstructured, error, error) {
	pod := new(v1.Pod)
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(obj.UnstructuredContent(), pod); err != nil {
		return nil, nil, errors.WithStack(err)
	}

	pod.Spec.NodeName = ""
	pod.Spec.Priority = nil

	serviceAccountTokenPrefix := pod.Spec.ServiceAccountName + "-token-"

	var preservedVolumes []v1.Volume
	for _, vol := range pod.Spec.Volumes {
		if !strings.HasPrefix(vol.Name, serviceAccountTokenPrefix) {
			preservedVolumes = append(preservedVolumes, vol)
		}
	}
	pod.Spec.Volumes = preservedVolumes

	for i, container := range pod.Spec.Containers {
		var preservedVolumeMounts []v1.VolumeMount
		for _, mount := range container.VolumeMounts {
			if !strings.HasPrefix(mount.Name, serviceAccountTokenPrefix) {
				preservedVolumeMounts = append(preservedVolumeMounts, mount)
			}
		}
		pod.Spec.Containers[i].VolumeMounts = preservedVolumeMounts
	}

	for i, container := range pod.Spec.InitContainers {
		var preservedVolumeMounts []v1.VolumeMount
		for _, mount := range container.VolumeMounts {
			if !strings.HasPrefix(mount.Name, serviceAccountTokenPrefix) {
				preservedVolumeMounts = append(preservedVolumeMounts, mount)
			}
		}
		pod.Spec.InitContainers[i].VolumeMounts = preservedVolumeMounts
	}

	res, err := runtime.DefaultUnstructuredConverter.ToUnstructured(pod)
	if err != nil {
		return nil, nil, errors.WithStack(err)
	}

	return &unstructured.Unstructured{Object: res}, nil, nil
}
