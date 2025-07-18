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

package install

import (
	"fmt"
	"strings"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	"github.com/vmware-tanzu/velero/internal/velero"
	"github.com/vmware-tanzu/velero/pkg/builder"
	"github.com/vmware-tanzu/velero/pkg/util/kube"
)

type podTemplateOption func(*podTemplateConfig)

type podTemplateConfig struct {
	image                           string
	envVars                         []corev1.EnvVar
	restoreOnly                     bool
	annotations                     map[string]string
	labels                          map[string]string
	resources                       corev1.ResourceRequirements
	withSecret                      bool
	defaultRepoMaintenanceFrequency time.Duration
	garbageCollectionFrequency      time.Duration
	podVolumeOperationTimeout       time.Duration
	plugins                         []string
	features                        []string
	defaultVolumesToFsBackup        bool
	serviceAccountName              string
	uploaderType                    string
	defaultSnapshotMoveData         bool
	privilegedNodeAgent             bool
	disableInformerCache            bool
	scheduleSkipImmediately         bool
	podResources                    kube.PodResources
	keepLatestMaintenanceJobs       int
	backupRepoConfigMap             string
	repoMaintenanceJobConfigMap     string
	nodeAgentConfigMap              string
	itemBlockWorkerCount            int
	forWindows                      bool
}

func WithImage(image string) podTemplateOption {
	return func(c *podTemplateConfig) {
		c.image = image
	}
}

func WithAnnotations(annotations map[string]string) podTemplateOption {
	return func(c *podTemplateConfig) {
		c.annotations = annotations
	}
}

func WithLabels(labels map[string]string) podTemplateOption {
	return func(c *podTemplateConfig) {
		c.labels = labels
	}
}

func WithEnvFromSecretKey(varName, secret, key string) podTemplateOption {
	return func(c *podTemplateConfig) {
		c.envVars = append(c.envVars, corev1.EnvVar{
			Name: varName,
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: secret,
					},
					Key: key,
				},
			},
		})
	}
}

func WithSecret(secretPresent bool) podTemplateOption {
	return func(c *podTemplateConfig) {
		c.withSecret = secretPresent
	}
}

func WithRestoreOnly(b bool) podTemplateOption {
	return func(c *podTemplateConfig) {
		c.restoreOnly = b
	}
}

func WithResources(resources corev1.ResourceRequirements) podTemplateOption {
	return func(c *podTemplateConfig) {
		c.resources = resources
	}
}

func WithDefaultRepoMaintenanceFrequency(val time.Duration) podTemplateOption {
	return func(c *podTemplateConfig) {
		c.defaultRepoMaintenanceFrequency = val
	}
}

func WithGarbageCollectionFrequency(val time.Duration) podTemplateOption {
	return func(c *podTemplateConfig) {
		c.garbageCollectionFrequency = val
	}
}

func WithPodVolumeOperationTimeout(val time.Duration) podTemplateOption {
	return func(c *podTemplateConfig) {
		c.podVolumeOperationTimeout = val
	}
}

func WithPlugins(plugins []string) podTemplateOption {
	return func(c *podTemplateConfig) {
		c.plugins = plugins
	}
}

func WithFeatures(features []string) podTemplateOption {
	return func(c *podTemplateConfig) {
		c.features = features
	}
}

func WithUploaderType(t string) podTemplateOption {
	return func(c *podTemplateConfig) {
		c.uploaderType = t
	}
}

func WithDefaultVolumesToFsBackup(b bool) podTemplateOption {
	return func(c *podTemplateConfig) {
		c.defaultVolumesToFsBackup = b
	}
}

func WithDefaultSnapshotMoveData(b bool) podTemplateOption {
	return func(c *podTemplateConfig) {
		c.defaultSnapshotMoveData = b
	}
}

func WithDisableInformerCache(b bool) podTemplateOption {
	return func(c *podTemplateConfig) {
		c.disableInformerCache = b
	}
}

func WithServiceAccountName(sa string) podTemplateOption {
	return func(c *podTemplateConfig) {
		c.serviceAccountName = sa
	}
}

func WithPrivilegedNodeAgent(b bool) podTemplateOption {
	return func(c *podTemplateConfig) {
		c.privilegedNodeAgent = b
	}
}

func WithNodeAgentConfigMap(nodeAgentConfigMap string) podTemplateOption {
	return func(c *podTemplateConfig) {
		c.nodeAgentConfigMap = nodeAgentConfigMap
	}
}

func WithScheduleSkipImmediately(b bool) podTemplateOption {
	return func(c *podTemplateConfig) {
		c.scheduleSkipImmediately = b
	}
}

func WithPodResources(podResources kube.PodResources) podTemplateOption {
	return func(c *podTemplateConfig) {
		c.podResources = podResources
	}
}

func WithKeepLatestMaintenanceJobs(keepLatestMaintenanceJobs int) podTemplateOption {
	return func(c *podTemplateConfig) {
		c.keepLatestMaintenanceJobs = keepLatestMaintenanceJobs
	}
}

func WithBackupRepoConfigMap(backupRepoConfigMap string) podTemplateOption {
	return func(c *podTemplateConfig) {
		c.backupRepoConfigMap = backupRepoConfigMap
	}
}
func WithRepoMaintenanceJobConfigMap(repoMaintenanceJobConfigMap string) podTemplateOption {
	return func(c *podTemplateConfig) {
		c.repoMaintenanceJobConfigMap = repoMaintenanceJobConfigMap
	}
}

func WithItemBlockWorkerCount(itemBlockWorkerCount int) podTemplateOption {
	return func(c *podTemplateConfig) {
		c.itemBlockWorkerCount = itemBlockWorkerCount
	}
}

func WithForWindows() podTemplateOption {
	return func(c *podTemplateConfig) {
		c.forWindows = true
	}
}

func Deployment(namespace string, opts ...podTemplateOption) *appsv1.Deployment {
	// TODO: Add support for server args
	c := &podTemplateConfig{
		image: velero.DefaultVeleroImage(),
	}

	for _, opt := range opts {
		opt(c)
	}

	pullPolicy := corev1.PullAlways
	imageParts := strings.Split(c.image, ":")
	if len(imageParts) == 2 && imageParts[1] != "latest" {
		pullPolicy = corev1.PullIfNotPresent
	}

	args := []string{"server"}
	if len(c.features) > 0 {
		args = append(args, fmt.Sprintf("--features=%s", strings.Join(c.features, ",")))
	}

	if c.defaultVolumesToFsBackup {
		args = append(args, "--default-volumes-to-fs-backup=true")
	}

	if c.defaultSnapshotMoveData {
		args = append(args, "--default-snapshot-move-data=true")
	}

	if c.disableInformerCache {
		args = append(args, "--disable-informer-cache=true")
	}

	if c.scheduleSkipImmediately {
		args = append(args, "--schedule-skip-immediately=true")
	}

	if len(c.uploaderType) > 0 {
		args = append(args, fmt.Sprintf("--uploader-type=%s", c.uploaderType))
	}

	if c.restoreOnly {
		args = append(args, "--restore-only")
	}

	if c.defaultRepoMaintenanceFrequency > 0 {
		args = append(args, fmt.Sprintf("--default-repo-maintain-frequency=%v", c.defaultRepoMaintenanceFrequency))
	}

	if c.garbageCollectionFrequency > 0 {
		args = append(args, fmt.Sprintf("--garbage-collection-frequency=%v", c.garbageCollectionFrequency))
	}

	if c.podVolumeOperationTimeout > 0 {
		args = append(args, fmt.Sprintf("--fs-backup-timeout=%v", c.podVolumeOperationTimeout))
	}

	if c.keepLatestMaintenanceJobs > 0 {
		args = append(args, fmt.Sprintf("--keep-latest-maintenance-jobs=%d", c.keepLatestMaintenanceJobs))
	}

	if len(c.podResources.CPULimit) > 0 {
		args = append(args, fmt.Sprintf("--maintenance-job-cpu-limit=%s", c.podResources.CPULimit))
	}

	if len(c.podResources.CPURequest) > 0 {
		args = append(args, fmt.Sprintf("--maintenance-job-cpu-request=%s", c.podResources.CPURequest))
	}

	if len(c.podResources.MemoryLimit) > 0 {
		args = append(args, fmt.Sprintf("--maintenance-job-mem-limit=%s", c.podResources.MemoryLimit))
	}

	if len(c.podResources.MemoryRequest) > 0 {
		args = append(args, fmt.Sprintf("--maintenance-job-mem-request=%s", c.podResources.MemoryRequest))
	}

	if len(c.backupRepoConfigMap) > 0 {
		args = append(args, fmt.Sprintf("--backup-repository-configmap=%s", c.backupRepoConfigMap))
	}

	if len(c.repoMaintenanceJobConfigMap) > 0 {
		args = append(args, fmt.Sprintf("--repo-maintenance-job-configmap=%s", c.repoMaintenanceJobConfigMap))
	}

	if c.itemBlockWorkerCount > 0 {
		args = append(args, fmt.Sprintf("--item-block-worker-count=%d", c.itemBlockWorkerCount))
	}

	deployment := &appsv1.Deployment{
		ObjectMeta: objectMeta(namespace, "velero"),
		TypeMeta: metav1.TypeMeta{
			Kind:       "Deployment",
			APIVersion: appsv1.SchemeGroupVersion.String(),
		},
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"deploy": "velero"}},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels:      podLabels(c.labels, map[string]string{"deploy": "velero"}),
					Annotations: podAnnotations(c.annotations),
				},
				Spec: corev1.PodSpec{
					RestartPolicy:      corev1.RestartPolicyAlways,
					ServiceAccountName: c.serviceAccountName,
					NodeSelector: map[string]string{
						"kubernetes.io/os": "linux",
					},
					OS: &corev1.PodOS{
						Name: "linux",
					},
					Containers: []corev1.Container{
						{
							Name:            "velero",
							Image:           c.image,
							Ports:           containerPorts(),
							ImagePullPolicy: pullPolicy,
							Command: []string{
								"/velero",
							},
							Args: args,
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "plugins",
									MountPath: "/plugins",
								},
								{
									Name:      "scratch",
									MountPath: "/scratch",
								},
							},
							Env: []corev1.EnvVar{
								{
									Name:  "VELERO_SCRATCH_DIR",
									Value: "/scratch",
								},
								{
									Name: "VELERO_NAMESPACE",
									ValueFrom: &corev1.EnvVarSource{
										FieldRef: &corev1.ObjectFieldSelector{
											FieldPath: "metadata.namespace",
										},
									},
								},
								{
									Name:  "LD_LIBRARY_PATH",
									Value: "/plugins",
								},
							},
							Resources: c.resources,
						},
					},
					Volumes: []corev1.Volume{
						{
							Name: "plugins",
							VolumeSource: corev1.VolumeSource{
								EmptyDir: &corev1.EmptyDirVolumeSource{},
							},
						},
						{
							Name: "scratch",
							VolumeSource: corev1.VolumeSource{
								EmptyDir: new(corev1.EmptyDirVolumeSource),
							},
						},
					},
				},
			},
		},
	}

	if c.withSecret {
		deployment.Spec.Template.Spec.Volumes = append(
			deployment.Spec.Template.Spec.Volumes,
			corev1.Volume{
				Name: "cloud-credentials",
				VolumeSource: corev1.VolumeSource{
					Secret: &corev1.SecretVolumeSource{
						// read-only for Owner, Group, Public
						DefaultMode: ptr.To(int32(0444)),
						SecretName:  "cloud-credentials",
					},
				},
			},
		)

		deployment.Spec.Template.Spec.Containers[0].VolumeMounts = append(
			deployment.Spec.Template.Spec.Containers[0].VolumeMounts,
			corev1.VolumeMount{
				Name:      "cloud-credentials",
				MountPath: "/credentials",
			},
		)

		deployment.Spec.Template.Spec.Containers[0].Env = append(deployment.Spec.Template.Spec.Containers[0].Env, []corev1.EnvVar{
			{
				Name:  "GOOGLE_APPLICATION_CREDENTIALS",
				Value: "/credentials/cloud",
			},
			{
				Name:  "AWS_SHARED_CREDENTIALS_FILE",
				Value: "/credentials/cloud",
			},
			{
				Name:  "AZURE_CREDENTIALS_FILE",
				Value: "/credentials/cloud",
			},
			{
				Name:  "ALIBABA_CLOUD_CREDENTIALS_FILE",
				Value: "/credentials/cloud",
			},
		}...)
	}

	deployment.Spec.Template.Spec.Containers[0].Env = append(deployment.Spec.Template.Spec.Containers[0].Env, c.envVars...)

	if len(c.plugins) > 0 {
		for _, image := range c.plugins {
			container := *builder.ForPluginContainer(image, pullPolicy).Result()
			deployment.Spec.Template.Spec.InitContainers = append(deployment.Spec.Template.Spec.InitContainers, container)
		}
	}

	return deployment
}
