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
	"context"

	appsv1api "k8s.io/api/apps/v1"
	corev1api "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"

	velerov1api "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	"github.com/vmware-tanzu/velero/pkg/nodeagent"
)

// GetNodeSelectorFromVeleroServer get the node selector from the Velero server deployment
func GetNodeSelectorFromVeleroServer(deployment *appsv1api.Deployment) map[string]string {
	return deployment.Spec.Template.Spec.NodeSelector
}

// GetTolerationsFromVeleroServer get the tolerations from the Velero server deployment
func GetTolerationsFromVeleroServer(deployment *appsv1api.Deployment) []corev1api.Toleration {
	return deployment.Spec.Template.Spec.Tolerations
}

// GetAffinityFromVeleroServer get the affinity from the Velero server deployment
func GetAffinityFromVeleroServer(deployment *appsv1api.Deployment) *corev1api.Affinity {
	return deployment.Spec.Template.Spec.Affinity
}

// GetEnvVarsFromVeleroServer get the environment variables from the Velero server deployment
func GetEnvVarsFromVeleroServer(deployment *appsv1api.Deployment) []corev1api.EnvVar {
	for _, container := range deployment.Spec.Template.Spec.Containers {
		// We only have one container in the Velero server deployment
		return container.Env
	}
	return nil
}

// GetEnvFromSourcesFromVeleroServer get the environment sources from the Velero server deployment
func GetEnvFromSourcesFromVeleroServer(deployment *appsv1api.Deployment) []corev1api.EnvFromSource {
	for _, container := range deployment.Spec.Template.Spec.Containers {
		// We only have one container in the Velero server deployment
		return container.EnvFrom
	}
	return nil
}

// GetVolumeMountsFromVeleroServer get the volume mounts from the Velero server deployment
func GetVolumeMountsFromVeleroServer(deployment *appsv1api.Deployment) []corev1api.VolumeMount {
	for _, container := range deployment.Spec.Template.Spec.Containers {
		// We only have one container in the Velero server deployment
		return container.VolumeMounts
	}
	return nil
}

// GetPodSecurityContextsFromVeleroServer get the pod security context from the Velero server deployment
func GetPodSecurityContextsFromVeleroServer(deployment *appsv1api.Deployment) *corev1api.PodSecurityContext {
	return deployment.Spec.Template.Spec.SecurityContext
}

// GetContainerSecurityContextsFromVeleroServer get the security context from the Velero server deployment
func GetContainerSecurityContextsFromVeleroServer(deployment *appsv1api.Deployment) *corev1api.SecurityContext {
	for _, container := range deployment.Spec.Template.Spec.Containers {
		// We only have one container in the Velero server deployment
		return container.SecurityContext
	}
	return nil
}

// GetVolumesFromVeleroServer get the volumes from the Velero server deployment
func GetVolumesFromVeleroServer(deployment *appsv1api.Deployment) []corev1api.Volume {
	return deployment.Spec.Template.Spec.Volumes
}

// GetServiceAccountFromVeleroServer get the service account from the Velero server deployment
func GetServiceAccountFromVeleroServer(deployment *appsv1api.Deployment) string {
	return deployment.Spec.Template.Spec.ServiceAccountName
}

// getVeleroServerImage get the image of the Velero server deployment
func GetVeleroServerImage(deployment *appsv1api.Deployment) string {
	return deployment.Spec.Template.Spec.Containers[0].Image
}

// GetVeleroServerLables get the labels of the Velero server deployment
func GetVeleroServerLables(deployment *appsv1api.Deployment) map[string]string {
	return deployment.Spec.Template.Labels
}

// GetVeleroServerAnnotations get the annotations of the Velero server deployment
func GetVeleroServerAnnotations(deployment *appsv1api.Deployment) map[string]string {
	return deployment.Spec.Template.Annotations
}

// GetVeleroServerLabelValue returns the value of specified label of Velero server deployment
func GetVeleroServerLabelValue(deployment *appsv1api.Deployment, key string) string {
	if deployment.Spec.Template.Labels == nil {
		return ""
	}

	return deployment.Spec.Template.Labels[key]
}

// GetVeleroServerAnnotationValue returns the value of specified annotation of Velero server deployment
func GetVeleroServerAnnotationValue(deployment *appsv1api.Deployment, key string) string {
	if deployment.Spec.Template.Annotations == nil {
		return ""
	}

	return deployment.Spec.Template.Annotations[key]
}

func BSLIsAvailable(bsl velerov1api.BackupStorageLocation) bool {
	return bsl.Status.Phase == velerov1api.BackupStorageLocationPhaseAvailable
}

// GetDataMoverPriorityClassName returns the priority class name for data mover pods from the node-agent-configmap
func GetDataMoverPriorityClassName(ctx context.Context, namespace string, kubeClient kubernetes.Interface, configName string) (string, error) {
	// Get from node-agent-configmap
	configs, err := nodeagent.GetConfigs(ctx, namespace, kubeClient, configName)
	if err == nil && configs != nil && configs.PriorityClassName != "" {
		return configs.PriorityClassName, nil
	}

	// Return empty string if not found in configmap
	return "", nil
}
