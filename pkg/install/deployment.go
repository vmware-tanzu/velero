package install

import (
	"strings"

	appsv1beta1 "k8s.io/api/apps/v1beta1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type deploymentOption func(*deploymentConfig)

type deploymentConfig struct {
	image                    string
	withoutCredentialsVolume bool
}

func WithImage(image string) deploymentOption {
	return func(c *deploymentConfig) {
		c.image = image
	}
}

func WithoutCredentialsVolume() deploymentOption {
	return func(c *deploymentConfig) {
		c.withoutCredentialsVolume = true
	}
}

func Deployment(namespace string, opts ...deploymentOption) *appsv1beta1.Deployment {
	c := &deploymentConfig{
		image: "gcr.io/heptio-images/ark:latest",
	}

	for _, opt := range opts {
		opt(c)
	}

	pullPolicy := corev1.PullAlways
	imageParts := strings.Split(c.image, ":")
	if len(imageParts) == 2 && imageParts[1] != "latest" {
		pullPolicy = corev1.PullIfNotPresent

	}

	deployment := &appsv1beta1.Deployment{
		ObjectMeta: objectMeta(namespace, "ark"),
		Spec: appsv1beta1.DeploymentSpec{
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels(),
				},
				Spec: corev1.PodSpec{
					RestartPolicy:      corev1.RestartPolicyAlways,
					ServiceAccountName: "ark",
					Containers: []corev1.Container{
						{
							Name:            "ark",
							Image:           c.image,
							ImagePullPolicy: pullPolicy,
							Command: []string{
								"/ark",
							},
							Args: []string{
								"server",
							},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "plugins",
									MountPath: "/plugins",
								},
							},
							Env: []corev1.EnvVar{
								{
									Name:  "GOOGLE_APPLICATION_CREDENTIALS",
									Value: "/credentials/cloud",
								},
								{
									Name:  "AWS_SHARED_CREDENTIALS_FILE",
									Value: "/credentials/cloud",
								},
							},
						},
					},
					Volumes: []corev1.Volume{
						{
							Name: "plugins",
							VolumeSource: corev1.VolumeSource{
								EmptyDir: &corev1.EmptyDirVolumeSource{},
							},
						},
					},
				},
			},
		},
	}

	if !c.withoutCredentialsVolume {
		deployment.Spec.Template.Spec.Volumes = append(
			deployment.Spec.Template.Spec.Volumes,
			corev1.Volume{
				Name: "cloud-credentials",
				VolumeSource: corev1.VolumeSource{
					Secret: &corev1.SecretVolumeSource{
						SecretName: "cloud-credentials",
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
	}

	return deployment
}
