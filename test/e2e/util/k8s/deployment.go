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

package k8s

import (
	"fmt"
	"time"

	"golang.org/x/net/context"
	apps "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	clientset "k8s.io/client-go/kubernetes"
)

const (
	// Poll is how often to Poll pods, nodes and claims.
	PollInterval = 2 * time.Second
	PollTimeout  = 15 * time.Minute
)

// newDeployment returns a RollingUpdate Deployment with a fake container image
func NewDeployment(name, ns string, replicas int32, labels map[string]string) *apps.Deployment {
	return &apps.Deployment{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Deployment",
			APIVersion: "apps/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Namespace: ns,
			Name:      name,
			Labels:    labels,
		},
		Spec: apps.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{MatchLabels: labels},
			Strategy: apps.DeploymentStrategy{
				Type:          apps.RollingUpdateDeploymentStrategyType,
				RollingUpdate: new(apps.RollingUpdateDeployment),
			},
			Template: v1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
				},
				Spec: v1.PodSpec{
					Containers: []v1.Container{
						{
							Name:    name,
							Image:   "busybox:latest",
							Command: []string{"sleep", "1000000"},
						},
					},
				},
			},
		},
	}
}

func CreateDeployment(c clientset.Interface, ns string, deployment *apps.Deployment) (*apps.Deployment, error) {
	return c.AppsV1().Deployments(ns).Create(context.TODO(), deployment, metav1.CreateOptions{})
}

func DownwardAPIVolumeBaseDeployment(name, ns string, replicas int32, labels map[string]string) *apps.Deployment {
	deploy := NewDeployment(name, ns, replicas, labels)
	vl := v1.Volume{
		Name: "podinfo",
		VolumeSource: v1.VolumeSource{
			Projected: &v1.ProjectedVolumeSource{
				Sources: []v1.VolumeProjection{
					{
						DownwardAPI: &v1.DownwardAPIProjection{
							Items: []v1.DownwardAPIVolumeFile{
								{
									Path: "labels",
									FieldRef: &v1.ObjectFieldSelector{
										APIVersion: "v1",
										FieldPath:  "metadata.labels",
									},
								},
							},
						},
					},
				},
			},
		},
	}
	deploy.Spec.Template.Spec.Volumes = append(deploy.Spec.Template.Spec.Volumes, vl)

	deploy.Spec.Template.Spec.Containers = []v1.Container{
		{
			Name:    "client-container",
			Image:   "busybox:latest",
			Command: []string{"sh", "-c", "while true; do if [[ -e /etc/podinfo/labels ]]; then cat /etc/podinfo/labels; fi; sleep 5; done"},
			VolumeMounts: []v1.VolumeMount{
				{
					Name:      "podinfo",
					MountPath: "/etc/podinfo",
					ReadOnly:  false,
				},
			},
		},
	}
	return deploy
}

// WaitForReadyDeployment waits for number of ready replicas to equal number of replicas.
func WaitForReadyDeployment(c clientset.Interface, ns, name string) error {
	if err := wait.PollImmediate(PollInterval, PollTimeout, func() (bool, error) {
		deployment, err := c.AppsV1().Deployments(ns).Get(context.TODO(), name, metav1.GetOptions{})
		if err != nil {
			return false, fmt.Errorf("failed to get deployment %q: %v", name, err)
		}
		return deployment.Status.ReadyReplicas == *deployment.Spec.Replicas, nil
	}); err != nil {
		return fmt.Errorf("failed to wait for .readyReplicas to equal .replicas: %v", err)
	}
	return nil
}
