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

package maintenance

import (
	"context"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	appsv1api "k8s.io/api/apps/v1"
	corev1api "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	velerov1api "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	"github.com/vmware-tanzu/velero/pkg/util/kube"
	"github.com/vmware-tanzu/velero/pkg/util/logging"
)

func TestBuildJobWithPriorityClassName(t *testing.T) {
	testCases := []struct {
		name              string
		priorityClassName string
		expectedValue     string
	}{
		{
			name:              "with priority class name",
			priorityClassName: "high-priority",
			expectedValue:     "high-priority",
		},
		{
			name:              "without priority class name",
			priorityClassName: "",
			expectedValue:     "",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Add Velero API types to the scheme
			err := velerov1api.AddToScheme(scheme.Scheme)
			require.NoError(t, err)

			// Create a fake client
			client := fake.NewClientBuilder().WithScheme(scheme.Scheme).Build()

			// Create a deployment with the specified priority class name
			deployment := &appsv1api.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "velero",
					Namespace: "velero",
				},
				Spec: appsv1api.DeploymentSpec{
					Template: corev1api.PodTemplateSpec{
						Spec: corev1api.PodSpec{
							Containers: []corev1api.Container{
								{
									Name:  "velero",
									Image: "velero/velero:latest",
								},
							},
							PriorityClassName: tc.priorityClassName,
						},
					},
				},
			}

			// Create a backup repository
			repo := &velerov1api.BackupRepository{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-repo",
					Namespace: "velero",
				},
				Spec: velerov1api.BackupRepositorySpec{
					VolumeNamespace:       "velero",
					BackupStorageLocation: "default",
				},
			}

			// Create the deployment in the fake client
			err = client.Create(context.Background(), deployment)
			require.NoError(t, err)

			// Create minimal job configs and resources
			jobConfig := &JobConfigs{}
			podResources := kube.PodResources{
				CPURequest:    "100m",
				MemoryRequest: "128Mi",
				CPULimit:      "200m",
				MemoryLimit:   "256Mi",
			}
			logLevel := logrus.InfoLevel
			logFormat := logging.NewFormatFlag()
			logFormat.Set("text")

			// Call buildJob
			job, err := buildJob(client, context.Background(), repo, "default", jobConfig, podResources, logLevel, logFormat)
			require.NoError(t, err)

			// Verify the priority class name is set correctly
			assert.Equal(t, tc.expectedValue, job.Spec.Template.Spec.PriorityClassName)
		})
	}
}
