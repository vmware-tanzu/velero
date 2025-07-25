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
	"encoding/json"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
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

// GetDataMoverPriorityClassName retrieves the priority class name for data mover pods from the node-agent-configmap
func GetDataMoverPriorityClassName(ctx context.Context, namespace string, kubeClient kubernetes.Interface, configName string) (string, error) {
	// configData is a minimal struct to parse only the priority class name from the ConfigMap
	type configData struct {
		PriorityClassName string `json:"priorityClassName,omitempty"`
	}

	// Get the ConfigMap
	cm, err := kubeClient.CoreV1().ConfigMaps(namespace).Get(ctx, configName, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			// ConfigMap not found is not an error, just return empty string
			return "", nil
		}
		return "", errors.Wrapf(err, "error getting node agent config map %s", configName)
	}

	if cm.Data == nil {
		// No data in ConfigMap, return empty string
		return "", nil
	}

	// Extract the first value from the ConfigMap data
	jsonString := ""
	for _, v := range cm.Data {
		jsonString = v
		break // Use the first value found
	}

	if jsonString == "" {
		// No data to parse, return empty string
		return "", nil
	}

	// Parse the JSON to extract priority class name
	var config configData
	if err := json.Unmarshal([]byte(jsonString), &config); err != nil {
		// Invalid JSON is not a critical error for priority class
		// Just return empty string to use default behavior
		//nolint:nilerr // Invalid JSON is not critical - return empty string to use default behavior
		return "", nil
	}

	return config.PriorityClassName, nil
}
