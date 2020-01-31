/*
Copyright 2017 the Velero contributors.

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

	"github.com/vmware-tanzu/velero/pkg/plugin/velero"
)

type PodAction struct {
	logger logrus.FieldLogger
}

func NewPodAction(logger logrus.FieldLogger) *PodAction {
	return &PodAction{logger: logger}
}

func (a *PodAction) AppliesTo() (velero.ResourceSelector, error) {
	return velero.ResourceSelector{
		IncludedResources: []string{"pods"},
	}, nil
}

func (a *PodAction) Execute(input *velero.RestoreItemActionExecuteInput) (*velero.RestoreItemActionExecuteOutput, error) {
	pod := new(v1.Pod)
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(input.Item.UnstructuredContent(), pod); err != nil {
		return nil, errors.WithStack(err)
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
		return nil, errors.WithStack(err)
	}

	return velero.NewRestoreItemActionExecuteOutput(&unstructured.Unstructured{Object: res}), nil
}
