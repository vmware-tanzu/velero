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
	JobSelectorKey = "job"
	// Poll is how often to Poll pods, nodes and claims.
	PollInterval = 2 * time.Second
	PollTimeout  = 15 * time.Minute
)

// newDeployment returns a RollingUpdate Deployment with a fake container image
func NewDeployment(name, ns string, replicas int32, labels map[string]string, containers []v1.Container) *apps.Deployment {
	if containers == nil {
		containers = []v1.Container{
			{
				Name:    fmt.Sprintf("container-%s", "busybox"),
				Image:   "gcr.io/velero-gcp/busybox:latest",
				Command: []string{"sleep", "1000000"},
			},
		}
	}
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
					Containers: containers,
				},
			},
		},
	}
}

func CreateDeploy(c clientset.Interface, ns string, deployment *apps.Deployment) error {
	_, err := c.AppsV1().Deployments(ns).Create(context.TODO(), deployment, metav1.CreateOptions{})
	return err
}
func CreateDeployment(c clientset.Interface, ns string, deployment *apps.Deployment) (*apps.Deployment, error) {
	return c.AppsV1().Deployments(ns).Create(context.TODO(), deployment, metav1.CreateOptions{})
}

func GetDeployment(c clientset.Interface, ns, name string) (*apps.Deployment, error) {
	return c.AppsV1().Deployments(ns).Get(context.TODO(), name, metav1.GetOptions{})
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
