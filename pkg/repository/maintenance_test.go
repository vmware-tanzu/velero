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
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	velerov1api "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	"github.com/vmware-tanzu/velero/pkg/repository/provider"
	"github.com/vmware-tanzu/velero/pkg/util/logging"
)

func TestGenerateJobName1(t *testing.T) {
	testCases := []struct {
		repo          string
		expectedStart string
	}{
		{
			repo:          "example",
			expectedStart: "example-maintain-job-",
		},
		{
			repo:          strings.Repeat("a", 60),
			expectedStart: "repo-maintain-job-",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.repo, func(t *testing.T) {
			// Call the function to test
			jobName := generateJobName(tc.repo)

			// Check if the generated job name starts with the expected prefix
			if !strings.HasPrefix(jobName, tc.expectedStart) {
				t.Errorf("generated job name does not start with expected prefix")
			}

			// Check if the length of the generated job name exceeds the Kubernetes limit
			if len(jobName) > 63 {
				t.Errorf("generated job name exceeds Kubernetes limit")
			}
		})
	}
}
func TestDeleteOldMaintenanceJobs(t *testing.T) {
	// Set up test repo and keep value
	repo := "test-repo"
	keep := 2

	// Create some maintenance jobs for testing
	var objs []client.Object
	// Create a newer job
	newerJob := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "job1",
			Namespace: "default",
			Labels:    map[string]string{RepositoryNameLabel: repo},
		},
		Spec: batchv1.JobSpec{},
	}
	objs = append(objs, newerJob)
	// Create older jobs
	for i := 2; i <= 3; i++ {
		olderJob := &batchv1.Job{
			ObjectMeta: metav1.ObjectMeta{
				Name:      fmt.Sprintf("job%d", i),
				Namespace: "default",
				Labels:    map[string]string{RepositoryNameLabel: repo},
				CreationTimestamp: metav1.Time{
					Time: metav1.Now().Add(time.Duration(-24*i) * time.Hour),
				},
			},
			Spec: batchv1.JobSpec{},
		}
		objs = append(objs, olderJob)
	}
	// Create a fake Kubernetes client
	scheme := runtime.NewScheme()
	_ = batchv1.AddToScheme(scheme)
	cli := fake.NewClientBuilder().WithScheme(scheme).WithObjects(objs...).Build()

	// Call the function
	err := deleteOldMaintenanceJobs(cli, repo, keep)
	require.NoError(t, err)

	// Get the remaining jobs
	jobList := &batchv1.JobList{}
	err = cli.List(context.TODO(), jobList, client.MatchingLabels(map[string]string{RepositoryNameLabel: repo}))
	require.NoError(t, err)

	// We expect the number of jobs to be equal to 'keep'
	assert.Len(t, jobList.Items, keep)

	// We expect that the oldest jobs were deleted
	// Job3 should not be present in the remaining list
	assert.NotContains(t, jobList.Items, objs[2])

	// Job2 should also not be present in the remaining list
	assert.NotContains(t, jobList.Items, objs[1])
}

func TestWaitForJobComplete(t *testing.T) {
	// Set up test job
	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-job",
			Namespace: "default",
		},
		Status: batchv1.JobStatus{},
	}

	// Define test cases
	tests := []struct {
		description string            // Test case description
		jobStatus   batchv1.JobStatus // Job status to set for the test
		expectError bool              // Whether an error is expected
	}{
		{
			description: "Job Succeeded",
			jobStatus: batchv1.JobStatus{
				Succeeded: 1,
			},
			expectError: false,
		},
		{
			description: "Job Failed",
			jobStatus: batchv1.JobStatus{
				Failed: 1,
			},
			expectError: true,
		},
	}

	// Run tests
	for _, tc := range tests {
		t.Run(tc.description, func(t *testing.T) {
			// Set the job status
			job.Status = tc.jobStatus
			// Create a fake Kubernetes client
			cli := fake.NewClientBuilder().WithObjects(job).Build()
			// Call the function
			err := waitForJobComplete(context.Background(), cli, job)

			// Check if the error matches the expectation
			if tc.expectError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestGetMaintenanceResultFromJob(t *testing.T) {
	// Set up test job
	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-job",
			Namespace: "default",
		},
	}

	// Set up test pod
	pod := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pod",
			Namespace: "default",
			Labels:    map[string]string{"job-name": job.Name},
		},
		Status: v1.PodStatus{
			ContainerStatuses: []v1.ContainerStatus{
				{
					State: v1.ContainerState{
						Terminated: &v1.ContainerStateTerminated{
							Message: "test message",
						},
					},
				},
			},
		},
	}

	// Create a fake Kubernetes client
	cli := fake.NewClientBuilder().WithObjects(job, pod).Build()

	// Call the function
	result, err := getMaintenanceResultFromJob(cli, job)

	// Check if the result and error match the expectation
	require.NoError(t, err)
	assert.Equal(t, "test message", result)
}

func TestGetLatestMaintenanceJob(t *testing.T) {
	// Set up test repo
	repo := "test-repo"

	// Create some maintenance jobs for testing
	var objs []client.Object
	// Create a newer job
	newerJob := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "job1",
			Namespace: "default",
			Labels:    map[string]string{RepositoryNameLabel: repo},
			CreationTimestamp: metav1.Time{
				Time: metav1.Now().Add(time.Duration(-24) * time.Hour),
			},
		},
		Spec: batchv1.JobSpec{},
	}
	objs = append(objs, newerJob)

	// Create an older job
	olderJob := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "job2",
			Namespace: "default",
			Labels:    map[string]string{RepositoryNameLabel: repo},
		},
		Spec: batchv1.JobSpec{},
	}
	objs = append(objs, olderJob)

	// Create a fake Kubernetes client
	scheme := runtime.NewScheme()
	_ = batchv1.AddToScheme(scheme)
	cli := fake.NewClientBuilder().WithScheme(scheme).WithObjects(objs...).Build()

	// Call the function
	job, err := getLatestMaintenanceJob(cli, "default")
	require.NoError(t, err)

	// We expect the returned job to be the newer job
	assert.Equal(t, newerJob.Name, job.Name)
}
func TestBuildMaintenanceJob(t *testing.T) {
	testCases := []struct {
		name            string
		m               MaintenanceConfig
		deploy          *appsv1.Deployment
		expectedJobName string
		expectedError   bool
	}{
		{
			name: "Valid maintenance job",
			m: MaintenanceConfig{
				CPURequest:   "100m",
				MemRequest:   "128Mi",
				CPULimit:     "200m",
				MemLimit:     "256Mi",
				LogLevelFlag: logging.LogLevelFlag(logrus.InfoLevel),
				FormatFlag:   logging.NewFormatFlag(),
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
			expectedJobName: "test-123-maintain-job",
			expectedError:   false,
		},
		{
			name: "Error getting Velero server deployment",
			m: MaintenanceConfig{
				CPURequest:   "100m",
				MemRequest:   "128Mi",
				CPULimit:     "200m",
				MemLimit:     "256Mi",
				LogLevelFlag: logging.LogLevelFlag(logrus.InfoLevel),
				FormatFlag:   logging.NewFormatFlag(),
			},
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

			// Call the function to test
			job, err := buildMaintenanceJob(tc.m, param, cli, "velero")

			// Check the error
			if tc.expectedError {
				require.Error(t, err)
				assert.Nil(t, job)
			} else {
				require.NoError(t, err)
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
						v1.ResourceCPU:    resource.MustParse(tc.m.CPURequest),
						v1.ResourceMemory: resource.MustParse(tc.m.MemRequest),
					},
					Limits: v1.ResourceList{
						v1.ResourceCPU:    resource.MustParse(tc.m.CPULimit),
						v1.ResourceMemory: resource.MustParse(tc.m.MemLimit),
					},
				}
				assert.Equal(t, expectedResources, container.Resources)

				// Check args
				expectedArgs := []string{
					"repo-maintenance",
					fmt.Sprintf("--repo-name=%s", param.BackupRepo.Spec.VolumeNamespace),
					fmt.Sprintf("--repo-type=%s", param.BackupRepo.Spec.RepositoryType),
					fmt.Sprintf("--backup-storage-location=%s", param.BackupLocation.Name),
					fmt.Sprintf("--log-level=%s", tc.m.LogLevelFlag.String()),
					fmt.Sprintf("--log-format=%s", tc.m.FormatFlag.String()),
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
