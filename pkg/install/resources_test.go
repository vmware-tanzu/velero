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
	"testing"

	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestAllResourcesWithPriorityClassName(t *testing.T) {
	testCases := []struct {
		name              string
		priorityClassName string
		useNodeAgent      bool
	}{
		{
			name:              "with priority class name and node agent",
			priorityClassName: "high-priority",
			useNodeAgent:      true,
		},
		{
			name:              "with priority class name without node agent",
			priorityClassName: "high-priority",
			useNodeAgent:      false,
		},
		{
			name:              "without priority class name with node agent",
			priorityClassName: "",
			useNodeAgent:      true,
		},
		{
			name:              "without priority class name without node agent",
			priorityClassName: "",
			useNodeAgent:      false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create VeleroOptions with the priority class name
			options := &VeleroOptions{
				Namespace:         "velero",
				UseNodeAgent:      tc.useNodeAgent,
				PriorityClassName: tc.priorityClassName,
			}

			// Generate all resources
			resources := AllResources(options)

			// Find the deployment and verify priority class name
			deploymentFound := false
			daemonsetFound := false

			for i := range resources.Items {
				item := resources.Items[i]

				// Check deployment
				if item.GetKind() == "Deployment" && item.GetName() == "velero" {
					deploymentFound = true

					// Extract priority class name from the unstructured object
					priorityClassName, found, err := unstructured.NestedString(
						item.Object,
						"spec", "template", "spec", "priorityClassName",
					)

					assert.NoError(t, err)
					if tc.priorityClassName != "" {
						assert.True(t, found, "priorityClassName should be set")
						assert.Equal(t, tc.priorityClassName, priorityClassName)
					} else {
						// If no priority class name was provided, it might not be set at all
						if found {
							assert.Equal(t, "", priorityClassName)
						}
					}
				}

				// Check daemonset if node agent is enabled
				if tc.useNodeAgent && item.GetKind() == "DaemonSet" && item.GetName() == "node-agent" {
					daemonsetFound = true

					// Extract priority class name from the unstructured object
					priorityClassName, found, err := unstructured.NestedString(
						item.Object,
						"spec", "template", "spec", "priorityClassName",
					)

					assert.NoError(t, err)
					if tc.priorityClassName != "" {
						assert.True(t, found, "priorityClassName should be set")
						assert.Equal(t, tc.priorityClassName, priorityClassName)
					} else {
						// If no priority class name was provided, it might not be set at all
						if found {
							assert.Equal(t, "", priorityClassName)
						}
					}
				}
			}

			// Verify we found the deployment
			assert.True(t, deploymentFound, "Deployment should be present in resources")

			// Verify we found the daemonset if node agent is enabled
			if tc.useNodeAgent {
				assert.True(t, daemonsetFound, "DaemonSet should be present when UseNodeAgent is true")
			}
		})
	}
}
