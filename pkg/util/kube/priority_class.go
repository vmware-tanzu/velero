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

package kube

import (
	"context"

	"github.com/sirupsen/logrus"
	schedulingv1 "k8s.io/api/scheduling/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// ValidatePriorityClass checks if the specified priority class exists in the cluster
// Returns true if the priority class exists or if priorityClassName is empty
// Returns false if the priority class doesn't exist or validation fails
// Logs warnings when the priority class doesn't exist
func ValidatePriorityClass(ctx context.Context, kubeClient kubernetes.Interface, priorityClassName string, logger logrus.FieldLogger) bool {
	if priorityClassName == "" {
		return true
	}

	_, err := kubeClient.SchedulingV1().PriorityClasses().Get(ctx, priorityClassName, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			logger.Warnf("Priority class %q not found in cluster. Pod creation may fail if the priority class doesn't exist when pods are scheduled.", priorityClassName)
		} else {
			logger.WithError(err).Warnf("Failed to validate priority class %q", priorityClassName)
		}
		return false
	}
	logger.Infof("Validated priority class %q exists in cluster", priorityClassName)
	return true
}

// ValidatePriorityClassWithClient checks if the specified priority class exists in the cluster using controller-runtime client
// Returns nil if the priority class exists or if priorityClassName is empty
// Returns error if the priority class doesn't exist or validation fails
func ValidatePriorityClassWithClient(ctx context.Context, cli client.Client, priorityClassName string) error {
	if priorityClassName == "" {
		return nil
	}

	priorityClass := &schedulingv1.PriorityClass{}
	err := cli.Get(ctx, types.NamespacedName{Name: priorityClassName}, priorityClass)
	return err
}
