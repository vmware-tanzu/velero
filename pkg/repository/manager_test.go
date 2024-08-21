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

package repository

import (
	"fmt"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	kbclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	velerov1 "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	velerov1api "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	"github.com/vmware-tanzu/velero/pkg/repository/provider"
	"github.com/vmware-tanzu/velero/pkg/util/kube"
	"github.com/vmware-tanzu/velero/pkg/util/logging"
)

func TestGetRepositoryProvider(t *testing.T) {
	var fakeClient kbclient.Client
	mgr := NewManager("", fakeClient, nil, nil, nil, nil, "", kube.PodResources{}, 3, nil, logrus.InfoLevel, nil).(*manager)
	repo := &velerov1.BackupRepository{}

	// empty repository type
	provider, err := mgr.getRepositoryProvider(repo)
	require.NoError(t, err)
	assert.NotNil(t, provider)

	// valid repository type
	repo.Spec.RepositoryType = velerov1.BackupRepositoryTypeRestic
	provider, err = mgr.getRepositoryProvider(repo)
	require.NoError(t, err)
	assert.NotNil(t, provider)

	// invalid repository type
	repo.Spec.RepositoryType = "unknown"
	_, err = mgr.getRepositoryProvider(repo)
	require.Error(t, err)
}

func TestBuildMaintenanceJob(t *testing.T) {
	testCases := []struct {
		name            string
		m               *JobConfigs
		deploy          *appsv1.Deployment
		logLevel        logrus.Level
		logFormat       *logging.FormatFlag
		expectedJobName string
		expectedError   bool
	}{
		{
			name: "Valid maintenance job",
			m: &JobConfigs{
				PodResources: &kube.PodResources{
					CPURequest:    "100m",
					MemoryRequest: "128Mi",
					CPULimit:      "200m",
					MemoryLimit:   "256Mi",
				},
			},
			deploy: &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "velero",
					Namespace: "velero",
				},
				Spec: appsv1.DeploymentSpec{
					Template: v1.PodTemplateSpec{
						Spec: v1.PodSpec{
							Containers: []v1.Container{
								{
									Name:  "velero-repo-maintenance-container",
									Image: "velero-image",
								},
							},
						},
					},
				},
			},
			logLevel:        logrus.InfoLevel,
			logFormat:       logging.NewFormatFlag(),
			expectedJobName: "test-123-maintain-job",
			expectedError:   false,
		},
		{
			name: "Error getting Velero server deployment",
			m: &JobConfigs{
				PodResources: &kube.PodResources{
					CPURequest:    "100m",
					MemoryRequest: "128Mi",
					CPULimit:      "200m",
					MemoryLimit:   "256Mi",
				},
			},
			logLevel:        logrus.InfoLevel,
			logFormat:       logging.NewFormatFlag(),
			expectedJobName: "",
			expectedError:   true,
		},
	}

	param := provider.RepoParam{
		BackupRepo: &velerov1api.BackupRepository{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "velero",
				Name:      "test-123",
			},
			Spec: velerov1api.BackupRepositorySpec{
				VolumeNamespace: "test-123",
				RepositoryType:  "kopia",
			},
		},
		BackupLocation: &velerov1api.BackupStorageLocation{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "velero",
				Name:      "test-location",
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create a fake clientset with resources
			objs := []runtime.Object{param.BackupLocation, param.BackupRepo}

			if tc.deploy != nil {
				objs = append(objs, tc.deploy)
			}
			scheme := runtime.NewScheme()
			_ = appsv1.AddToScheme(scheme)
			_ = velerov1api.AddToScheme(scheme)
			cli := fake.NewClientBuilder().WithScheme(scheme).WithRuntimeObjects(objs...).Build()

			mgr := NewManager(
				"velero",
				cli,
				nil,
				nil,
				nil,
				nil,
				"",
				kube.PodResources{},
				3,
				nil,
				logrus.InfoLevel,
				logging.NewFormatFlag(),
			).(*manager)

			// Call the function to test
			job, err := mgr.buildMaintenanceJob(tc.m, param)

			// Check the error
			if tc.expectedError {
				assert.Error(t, err)
				assert.Nil(t, job)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, job)
				assert.Contains(t, job.Name, tc.expectedJobName)
				assert.Equal(t, param.BackupRepo.Namespace, job.Namespace)
				assert.Equal(t, param.BackupRepo.Name, job.Labels[RepositoryNameLabel])

				// Check container
				assert.Len(t, job.Spec.Template.Spec.Containers, 1)
				container := job.Spec.Template.Spec.Containers[0]
				assert.Equal(t, "velero-repo-maintenance-container", container.Name)
				assert.Equal(t, "velero-image", container.Image)
				assert.Equal(t, v1.PullIfNotPresent, container.ImagePullPolicy)

				// Check resources
				expectedResources := v1.ResourceRequirements{
					Requests: v1.ResourceList{
						v1.ResourceCPU:    resource.MustParse(tc.m.PodResources.CPURequest),
						v1.ResourceMemory: resource.MustParse(tc.m.PodResources.MemoryRequest),
					},
					Limits: v1.ResourceList{
						v1.ResourceCPU:    resource.MustParse(tc.m.PodResources.CPULimit),
						v1.ResourceMemory: resource.MustParse(tc.m.PodResources.MemoryLimit),
					},
				}
				assert.Equal(t, expectedResources, container.Resources)

				// Check args
				expectedArgs := []string{
					"repo-maintenance",
					fmt.Sprintf("--repo-name=%s", param.BackupRepo.Spec.VolumeNamespace),
					fmt.Sprintf("--repo-type=%s", param.BackupRepo.Spec.RepositoryType),
					fmt.Sprintf("--backup-storage-location=%s", param.BackupLocation.Name),
					fmt.Sprintf("--log-level=%s", tc.logLevel.String()),
					fmt.Sprintf("--log-format=%s", tc.logFormat.String()),
				}
				assert.Equal(t, expectedArgs, container.Args)

				// Check affinity
				assert.Nil(t, job.Spec.Template.Spec.Affinity)

				// Check tolerations
				assert.Nil(t, job.Spec.Template.Spec.Tolerations)

				// Check node selector
				assert.Nil(t, job.Spec.Template.Spec.NodeSelector)
			}
		})
	}
}
