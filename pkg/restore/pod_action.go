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

	// if there are no volumes, then there can't be any volume mounts, so we're done.
	if len(pod.Spec.Volumes) == 0 {
		res, err := runtime.DefaultUnstructuredConverter.ToUnstructured(pod)
		if err != nil {
			return nil, nil, errors.WithStack(err)
		}
		return &unstructured.Unstructured{Object: res}, nil, nil
	}

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

// func (a *podAction) Execute(obj runtime.Unstructured, restore *api.Restore) (runtime.Unstructured, error, error) {
// 	unstructured.RemoveNestedField(obj.UnstructuredContent(), "spec", "nodeName")
// 	unstructured.RemoveNestedField(obj.UnstructuredContent(), "spec", "priority")

// 	// if there are no volumes, then there can't be any volume mounts, so we're done.
// 	res, found, err := unstructured.NestedFieldNoCopy(obj.UnstructuredContent(), "spec", "volumes")
// 	if err != nil {
// 		return nil, nil, errors.WithStack(err)
// 	}
// 	if !found {
// 		return obj, nil, nil
// 	}
// 	volumes, ok := res.([]interface{})
// 	if !ok {
// 		return nil, nil, errors.Errorf("unexpected type for .spec.volumes %T", res)
// 	}

// 	serviceAccountName, found, err := unstructured.NestedString(obj.UnstructuredContent(), "spec", "serviceAccountName")
// 	if err != nil {
// 		return nil, nil, errors.WithStack(err)
// 	}
// 	if !found {
// 		return nil, nil, errors.New(".spec.serviceAccountName not found")
// 	}

// 	prefix := serviceAccountName + "-token-"

// 	var preservedVolumes []interface{}
// 	for _, obj := range volumes {
// 		volume, ok := obj.(map[string]interface{})
// 		if !ok {
// 			return nil, nil, errors.Errorf("unexpected type for volume %T", obj)
// 		}

// 		name, found, err := unstructured.NestedString(volume, "name")
// 		if err != nil {
// 			return nil, nil, errors.WithStack(err)
// 		}
// 		if !found {
// 			return nil, nil, errors.New("no name found for volume")
// 		}

// 		if !strings.HasPrefix(name, prefix) {
// 			preservedVolumes = append(preservedVolumes, volume)
// 		}
// 	}

// 	if err := unstructured.SetNestedSlice(obj.UnstructuredContent(), preservedVolumes, "spec", "volumes"); err != nil {
// 		return nil, nil, errors.WithStack(err)
// 	}

// 	containers, err := nestedSliceRef(obj.UnstructuredContent(), "spec", "containers")
// 	if err != nil {
// 		return nil, nil, err
// 	}

// 	for _, obj := range containers {
// 		container, ok := obj.(map[string]interface{})
// 		if !ok {
// 			return nil, nil, errors.Errorf("unexpected type for container %T", obj)
// 		}

// 		volumeMounts, err := nestedSliceRef(container, "volumeMounts")
// 		if err != nil {
// 			return nil, nil, err
// 		}

// 		var preservedVolumeMounts []interface{}
// 		for _, obj := range volumeMounts {
// 			mount, ok := obj.(map[string]interface{})
// 			if !ok {
// 				return nil, nil, errors.Errorf("unexpected type for volume mount %T", obj)
// 			}

// 			name, found, err := unstructured.NestedString(mount, "name")
// 			if err != nil {
// 				return nil, nil, errors.WithStack(err)
// 			}
// 			if !found {
// 				return nil, nil, errors.New("no name found for volume mount")
// 			}

// 			if !strings.HasPrefix(name, prefix) {
// 				preservedVolumeMounts = append(preservedVolumeMounts, mount)
// 			}
// 		}

// 		container["volumeMounts"] = preservedVolumeMounts
// 	}

// 	res, found, err = unstructured.NestedFieldNoCopy(obj.UnstructuredContent(), "spec", "initContainers")
// 	if err != nil {
// 		return nil, nil, errors.WithStack(err)
// 	}
// 	if !found {
// 		return obj, nil, nil
// 	}
// 	initContainers, ok := res.([]interface{})
// 	if !ok {
// 		return nil, nil, errors.Errorf("unexpected type for .spec.initContainers %T", res)
// 	}

// 	for _, obj := range initContainers {
// 		initContainer, ok := obj.(map[string]interface{})
// 		if !ok {
// 			return nil, nil, errors.Errorf("unexpected type for init container %T", obj)
// 		}

// 		volumeMounts, err := nestedSliceRef(initContainer, "volumeMounts")
// 		if err != nil {
// 			return nil, nil, err
// 		}

// 		var preservedVolumeMounts []interface{}
// 		for _, obj := range volumeMounts {
// 			mount, ok := obj.(map[string]interface{})
// 			if !ok {
// 				return nil, nil, errors.Errorf("unexpected type for volume mount %T", obj)
// 			}

// 			name, found, err := unstructured.NestedString(mount, "name")
// 			if err != nil {
// 				return nil, nil, errors.WithStack(err)
// 			}
// 			if !found {
// 				return nil, nil, errors.New("no name found for volume mount")
// 			}

// 			if !strings.HasPrefix(name, prefix) {
// 				preservedVolumeMounts = append(preservedVolumeMounts, mount)
// 			}
// 		}

// 		initContainer["volumeMounts"] = preservedVolumeMounts
// 	}

// 	return obj, nil, nil
// }

// func nestedSliceRef(obj map[string]interface{}, fields ...string) ([]interface{}, error) {
// 	val, found, err := unstructured.NestedFieldNoCopy(obj, fields...)
// 	if err != nil {
// 		return nil, errors.WithStack(err)
// 	}
// 	if !found {
// 		return nil, errors.Errorf(".%s not found", strings.Join(fields, "."))
// 	}

// 	slice, ok := val.([]interface{})
// 	if !ok {
// 		return nil, errors.Errorf("unexpected type for .%s %T", strings.Join(fields, "."), val)
// 	}

// 	return slice, nil
// }
