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
	"context"
	"fmt"
	"time"

	schedulingv1 "k8s.io/api/scheduling/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
)

// CreatePriorityClass creates a priority class with the given name and value
func CreatePriorityClass(ctx context.Context, client TestClient, name string, value int32) error {
	if name == "" {
		return fmt.Errorf("priority class name cannot be empty")
	}
	if value < -1000000000 || value > 1000000000 {
		return fmt.Errorf("priority class value must be between -1000000000 and 1000000000")
	}

	priorityClass := &schedulingv1.PriorityClass{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Value:         value,
		GlobalDefault: false,
		Description:   fmt.Sprintf("Priority class for e2e testing - %s", name),
	}

	_, err := client.ClientGo.SchedulingV1().PriorityClasses().Create(ctx, priorityClass, metav1.CreateOptions{})
	return err
}

// DeletePriorityClass deletes a priority class with the given name
func DeletePriorityClass(ctx context.Context, client TestClient, name string) error {
	return client.ClientGo.SchedulingV1().PriorityClasses().Delete(ctx, name, metav1.DeleteOptions{})
}

// GetPriorityClass gets a priority class by name
func GetPriorityClass(ctx context.Context, client TestClient, name string) (*schedulingv1.PriorityClass, error) {
	return client.ClientGo.SchedulingV1().PriorityClasses().Get(ctx, name, metav1.GetOptions{})
}

// WaitForPriorityClassDelete waits for a priority class to be deleted
func WaitForPriorityClassDelete(ctx context.Context, client TestClient, name string) error {
	return wait.PollUntilContextTimeout(ctx, 5*time.Second, 1*time.Minute, true, func(ctx context.Context) (bool, error) {
		_, err := client.ClientGo.SchedulingV1().PriorityClasses().Get(ctx, name, metav1.GetOptions{})
		if apierrors.IsNotFound(err) {
			return true, nil
		}
		if err != nil {
			return false, err
		}
		return false, nil
	})
}

// VerifyPodPriorityClass verifies that pods matching the label selector have the expected priority class
func VerifyPodPriorityClass(ctx context.Context, client TestClient, namespace, labelSelector, expectedPriorityClass string) error {
	pods, err := client.ClientGo.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{
		LabelSelector: labelSelector,
	})
	if err != nil {
		return fmt.Errorf("failed to list pods: %v", err)
	}

	if len(pods.Items) == 0 {
		return fmt.Errorf("no pods found with label selector: %s", labelSelector)
	}

	for _, pod := range pods.Items {
		if pod.Spec.PriorityClassName != expectedPriorityClass {
			return fmt.Errorf("pod %s/%s has priority class %q, expected %q",
				pod.Namespace, pod.Name, pod.Spec.PriorityClassName, expectedPriorityClass)
		}
	}

	return nil
}

// VerifyDeploymentPriorityClass verifies that a deployment has the expected priority class
func VerifyDeploymentPriorityClass(ctx context.Context, client TestClient, namespace, name, expectedPriorityClass string) error {
	deployment, err := client.ClientGo.AppsV1().Deployments(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get deployment: %v", err)
	}

	if deployment.Spec.Template.Spec.PriorityClassName != expectedPriorityClass {
		return fmt.Errorf("deployment %s/%s has priority class %q, expected %q",
			namespace, name, deployment.Spec.Template.Spec.PriorityClassName, expectedPriorityClass)
	}

	return nil
}

// VerifyDaemonSetPriorityClass verifies that a daemonset has the expected priority class
func VerifyDaemonSetPriorityClass(ctx context.Context, client TestClient, namespace, name, expectedPriorityClass string) error {
	daemonset, err := client.ClientGo.AppsV1().DaemonSets(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get daemonset: %v", err)
	}

	if daemonset.Spec.Template.Spec.PriorityClassName != expectedPriorityClass {
		return fmt.Errorf("daemonset %s/%s has priority class %q, expected %q",
			namespace, name, daemonset.Spec.Template.Spec.PriorityClassName, expectedPriorityClass)
	}

	return nil
}

// VerifyJobPriorityClass verifies that a job has the expected priority class
func VerifyJobPriorityClass(ctx context.Context, client TestClient, namespace, labelSelector, expectedPriorityClass string) error {
	jobs, err := client.ClientGo.BatchV1().Jobs(namespace).List(ctx, metav1.ListOptions{
		LabelSelector: labelSelector,
	})
	if err != nil {
		return fmt.Errorf("failed to list jobs: %v", err)
	}

	if len(jobs.Items) == 0 {
		return fmt.Errorf("no jobs found with label selector: %s", labelSelector)
	}

	for _, job := range jobs.Items {
		if job.Spec.Template.Spec.PriorityClassName != expectedPriorityClass {
			return fmt.Errorf("job %s/%s has priority class %q, expected %q",
				job.Namespace, job.Name, job.Spec.Template.Spec.PriorityClassName, expectedPriorityClass)
		}
	}

	return nil
}
