/*
Copyright the Velero contributors.

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

package velero

import (
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
)

// GetNodeSelectorFromVeleroServer get the node selector from the Velero server deployment
func GetNodeSelectorFromVeleroServer(deployment *appsv1.Deployment) map[string]string {
	return deployment.Spec.Template.Spec.NodeSelector
}

// GetTolerationsFromVeleroServer get the tolerations from the Velero server deployment
func GetTolerationsFromVeleroServer(deployment *appsv1.Deployment) []v1.Toleration {
	return deployment.Spec.Template.Spec.Tolerations
}

// GetAffinityFromVeleroServer get the affinity from the Velero server deployment
func GetAffinityFromVeleroServer(deployment *appsv1.Deployment) *v1.Affinity {
	return deployment.Spec.Template.Spec.Affinity
}

// GetEnvVarsFromVeleroServer get the environment variables from the Velero server deployment
func GetEnvVarsFromVeleroServer(deployment *appsv1.Deployment) []v1.EnvVar {
	for _, container := range deployment.Spec.Template.Spec.Containers {
		// We only have one container in the Velero server deployment
		return container.Env
	}
	return nil
}

// GetEnvFromSourcesFromVeleroServer get the environment sources from the Velero server deployment
func GetEnvFromSourcesFromVeleroServer(deployment *appsv1.Deployment) []v1.EnvFromSource {
	for _, container := range deployment.Spec.Template.Spec.Containers {
		// We only have one container in the Velero server deployment
		return container.EnvFrom
	}
	return nil
}

// GetVolumeMountsFromVeleroServer get the volume mounts from the Velero server deployment
func GetVolumeMountsFromVeleroServer(deployment *appsv1.Deployment) []v1.VolumeMount {
	for _, container := range deployment.Spec.Template.Spec.Containers {
		// We only have one container in the Velero server deployment
		return container.VolumeMounts
	}
	return nil
}

// GetVolumesFromVeleroServer get the volumes from the Velero server deployment
func GetVolumesFromVeleroServer(deployment *appsv1.Deployment) []v1.Volume {
	return deployment.Spec.Template.Spec.Volumes
}

// GetServiceAccountFromVeleroServer get the service account from the Velero server deployment
func GetServiceAccountFromVeleroServer(deployment *appsv1.Deployment) string {
	return deployment.Spec.Template.Spec.ServiceAccountName
}

// getVeleroServerImage get the image of the Velero server deployment
func GetVeleroServerImage(deployment *appsv1.Deployment) string {
	return deployment.Spec.Template.Spec.Containers[0].Image
}

// GetVeleroServerLables get the labels of the Velero server deployment
func GetVeleroServerLables(deployment *appsv1.Deployment) map[string]string {
	return deployment.Spec.Template.Labels
}

// GetVeleroServerAnnotations get the annotations of the Velero server deployment
func GetVeleroServerAnnotations(deployment *appsv1.Deployment) map[string]string {
	return deployment.Spec.Template.Annotations
}

// GetVeleroServerLabelValue returns the value of specified label of Velero server deployment
func GetVeleroServerLabelValue(deployment *appsv1.Deployment, key string) string {
	if deployment.Spec.Template.Labels == nil {
		return ""
	}

	return deployment.Spec.Template.Labels[key]
}
