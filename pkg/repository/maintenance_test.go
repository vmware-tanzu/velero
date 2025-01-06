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
	batchv1 "k8s.io/api/batch/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	velerov1api "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	"github.com/vmware-tanzu/velero/pkg/builder"
	"github.com/vmware-tanzu/velero/pkg/repository/provider"
	velerotest "github.com/vmware-tanzu/velero/pkg/test"
	"github.com/vmware-tanzu/velero/pkg/util/kube"
	"github.com/vmware-tanzu/velero/pkg/util/logging"

	appsv1 "k8s.io/api/apps/v1"
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
			jobName := GenerateJobName(tc.repo)

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
	err := DeleteOldMaintenanceJobs(cli, repo, keep)
	assert.NoError(t, err)

	// Get the remaining jobs
	jobList := &batchv1.JobList{}
	err = cli.List(context.TODO(), jobList, client.MatchingLabels(map[string]string{RepositoryNameLabel: repo}))
	assert.NoError(t, err)

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

	schemeFail := runtime.NewScheme()

	scheme := runtime.NewScheme()
	batchv1.AddToScheme(scheme)

	// Define test cases
	tests := []struct {
		description   string // Test case description
		kubeClientObj []runtime.Object
		runtimeScheme *runtime.Scheme
		jobStatus     batchv1.JobStatus // Job status to set for the test
		expectError   bool              // Whether an error is expected
	}{
		{
			description:   "wait error",
			runtimeScheme: schemeFail,
			expectError:   true,
		},
		{
			description:   "Job Succeeded",
			runtimeScheme: scheme,
			kubeClientObj: []runtime.Object{
				job,
			},
			jobStatus: batchv1.JobStatus{
				Succeeded: 1,
			},
			expectError: false,
		},
		{
			description:   "Job Failed",
			runtimeScheme: scheme,
			kubeClientObj: []runtime.Object{
				job,
			},
			jobStatus: batchv1.JobStatus{
				Failed: 1,
			},
			expectError: false,
		},
	}

	// Run tests
	for _, tc := range tests {
		t.Run(tc.description, func(t *testing.T) {
			// Set the job status
			job.Status = tc.jobStatus
			// Create a fake Kubernetes client
			fakeClientBuilder := fake.NewClientBuilder()
			fakeClientBuilder = fakeClientBuilder.WithScheme(tc.runtimeScheme)
			fakeClient := fakeClientBuilder.WithRuntimeObjects(tc.kubeClientObj...).Build()

			// Call the function
			_, err := waitForJobComplete(context.Background(), fakeClient, job.Namespace, job.Name)

			// Check if the error matches the expectation
			if tc.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
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

	// Set up test pod with no status
	pod := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pod",
			Namespace: "default",
			Labels:    map[string]string{"job-name": job.Name},
		},
	}

	// Create a fake Kubernetes client
	cli := fake.NewClientBuilder().WithObjects(job, pod).Build()

	// test an error should be returned
	result, err := getMaintenanceResultFromJob(cli, job)
	assert.Error(t, err)
	assert.Equal(t, "", result)

	// Set a non-terminated container status to the pod
	pod.Status = v1.PodStatus{
		ContainerStatuses: []v1.ContainerStatus{
			{
				State: v1.ContainerState{},
			},
		},
	}

	// Test an error should be returned
	cli = fake.NewClientBuilder().WithObjects(job, pod).Build()
	result, err = getMaintenanceResultFromJob(cli, job)
	assert.Error(t, err)
	assert.Equal(t, "", result)

	// Set a terminated container status to the pod
	pod.Status = v1.PodStatus{
		ContainerStatuses: []v1.ContainerStatus{
			{
				State: v1.ContainerState{
					Terminated: &v1.ContainerStateTerminated{
						Message: "test message",
					},
				},
			},
		},
	}

	// This call should return the termination message with no error
	cli = fake.NewClientBuilder().WithObjects(job, pod).Build()
	result, err = getMaintenanceResultFromJob(cli, job)
	assert.NoError(t, err)
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
	assert.NoError(t, err)

	// We expect the returned job to be the newer job
	assert.Equal(t, newerJob.Name, job.Name)
}

func TestGetMaintenanceJobConfig(t *testing.T) {
	ctx := context.Background()
	logger := logrus.New()
	veleroNamespace := "velero"
	repoMaintenanceJobConfig := "repo-maintenance-job-config"
	repo := &velerov1api.BackupRepository{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: veleroNamespace,
			Name:      repoMaintenanceJobConfig,
		},
		Spec: velerov1api.BackupRepositorySpec{
			BackupStorageLocation: "default",
			RepositoryType:        "kopia",
			VolumeNamespace:       "test",
		},
	}

	testCases := []struct {
		name           string
		repoJobConfig  *v1.ConfigMap
		expectedConfig *JobConfigs
		expectedError  error
	}{
		{
			name:           "Config not exist",
			expectedConfig: nil,
			expectedError:  nil,
		},
		{
			name: "Invalid JSON",
			repoJobConfig: &v1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: veleroNamespace,
					Name:      repoMaintenanceJobConfig,
				},
				Data: map[string]string{
					"test-default-kopia": "{\"cpuRequest:\"100m\"}",
				},
			},
			expectedConfig: nil,
			expectedError:  fmt.Errorf("fail to unmarshal configs from %s", repoMaintenanceJobConfig),
		},
		{
			name: "Find config specific for BackupRepository",
			repoJobConfig: &v1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: veleroNamespace,
					Name:      repoMaintenanceJobConfig,
				},
				Data: map[string]string{
					"test-default-kopia": "{\"podResources\":{\"cpuRequest\":\"100m\",\"cpuLimit\":\"200m\",\"memoryRequest\":\"100Mi\",\"memoryLimit\":\"200Mi\"},\"loadAffinity\":[{\"nodeSelector\":{\"matchExpressions\":[{\"key\":\"cloud.google.com/machine-family\",\"operator\":\"In\",\"values\":[\"e2\"]}]}}]}",
				},
			},
			expectedConfig: &JobConfigs{
				PodResources: &kube.PodResources{
					CPURequest:    "100m",
					CPULimit:      "200m",
					MemoryRequest: "100Mi",
					MemoryLimit:   "200Mi",
				},
				LoadAffinities: []*kube.LoadAffinity{
					{
						NodeSelector: metav1.LabelSelector{
							MatchExpressions: []metav1.LabelSelectorRequirement{
								{
									Key:      "cloud.google.com/machine-family",
									Operator: metav1.LabelSelectorOpIn,
									Values:   []string{"e2"},
								},
							},
						},
					},
				},
			},
			expectedError: nil,
		},
		{
			name: "Find config specific for global",
			repoJobConfig: &v1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: veleroNamespace,
					Name:      repoMaintenanceJobConfig,
				},
				Data: map[string]string{
					GlobalKeyForRepoMaintenanceJobCM: "{\"podResources\":{\"cpuRequest\":\"50m\",\"cpuLimit\":\"100m\",\"memoryRequest\":\"50Mi\",\"memoryLimit\":\"100Mi\"},\"loadAffinity\":[{\"nodeSelector\":{\"matchExpressions\":[{\"key\":\"cloud.google.com/machine-family\",\"operator\":\"In\",\"values\":[\"n2\"]}]}}]}",
				},
			},
			expectedConfig: &JobConfigs{
				PodResources: &kube.PodResources{
					CPURequest:    "50m",
					CPULimit:      "100m",
					MemoryRequest: "50Mi",
					MemoryLimit:   "100Mi",
				},
				LoadAffinities: []*kube.LoadAffinity{
					{
						NodeSelector: metav1.LabelSelector{
							MatchExpressions: []metav1.LabelSelectorRequirement{
								{
									Key:      "cloud.google.com/machine-family",
									Operator: metav1.LabelSelectorOpIn,
									Values:   []string{"n2"},
								},
							},
						},
					},
				},
			},
			expectedError: nil,
		},
		{
			name: "Specific config supersede global config",
			repoJobConfig: &v1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: veleroNamespace,
					Name:      repoMaintenanceJobConfig,
				},
				Data: map[string]string{
					GlobalKeyForRepoMaintenanceJobCM: "{\"podResources\":{\"cpuRequest\":\"50m\",\"cpuLimit\":\"100m\",\"memoryRequest\":\"50Mi\",\"memoryLimit\":\"100Mi\"},\"loadAffinity\":[{\"nodeSelector\":{\"matchExpressions\":[{\"key\":\"cloud.google.com/machine-family\",\"operator\":\"In\",\"values\":[\"n2\"]}]}}]}",
					"test-default-kopia":             "{\"podResources\":{\"cpuRequest\":\"100m\",\"cpuLimit\":\"200m\",\"memoryRequest\":\"100Mi\",\"memoryLimit\":\"200Mi\"},\"loadAffinity\":[{\"nodeSelector\":{\"matchExpressions\":[{\"key\":\"cloud.google.com/machine-family\",\"operator\":\"In\",\"values\":[\"e2\"]}]}}]}",
				},
			},
			expectedConfig: &JobConfigs{
				PodResources: &kube.PodResources{
					CPURequest:    "100m",
					CPULimit:      "200m",
					MemoryRequest: "100Mi",
					MemoryLimit:   "200Mi",
				},
				LoadAffinities: []*kube.LoadAffinity{
					{
						NodeSelector: metav1.LabelSelector{
							MatchExpressions: []metav1.LabelSelectorRequirement{
								{
									Key:      "cloud.google.com/machine-family",
									Operator: metav1.LabelSelectorOpIn,
									Values:   []string{"e2"},
								},
							},
						},
					},
				},
			},
			expectedError: nil,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var fakeClient client.Client
			if tc.repoJobConfig != nil {
				fakeClient = velerotest.NewFakeControllerRuntimeClient(t, tc.repoJobConfig)
			} else {
				fakeClient = velerotest.NewFakeControllerRuntimeClient(t)
			}

			jobConfig, err := getMaintenanceJobConfig(
				ctx,
				fakeClient,
				logger,
				veleroNamespace,
				repoMaintenanceJobConfig,
				repo,
			)

			if tc.expectedError != nil {
				require.ErrorContains(t, err, tc.expectedError.Error())
			} else {
				require.NoError(t, err)
			}
			require.Equal(t, tc.expectedConfig, jobConfig)
		})
	}
}

func TestWaitAllMaintenanceJobComplete(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*2)

	veleroNamespace := "velero"
	repo := &velerov1api.BackupRepository{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: veleroNamespace,
			Name:      "fake-repo",
		},
		Spec: velerov1api.BackupRepositorySpec{
			BackupStorageLocation: "default",
			RepositoryType:        "kopia",
			VolumeNamespace:       "test",
		},
	}

	now := time.Now().Round(time.Second)

	jobOtherLabel := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "job1",
			Namespace:         veleroNamespace,
			Labels:            map[string]string{RepositoryNameLabel: "other-repo"},
			CreationTimestamp: metav1.Time{Time: now},
		},
	}

	jobIncomplete := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "job1",
			Namespace:         veleroNamespace,
			Labels:            map[string]string{RepositoryNameLabel: "fake-repo"},
			CreationTimestamp: metav1.Time{Time: now},
		},
	}

	jobSucceeded1 := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "job1",
			Namespace:         veleroNamespace,
			Labels:            map[string]string{RepositoryNameLabel: "fake-repo"},
			CreationTimestamp: metav1.Time{Time: now},
		},
		Status: batchv1.JobStatus{
			StartTime:      &metav1.Time{Time: now},
			CompletionTime: &metav1.Time{Time: now.Add(time.Hour)},
			Succeeded:      1,
		},
	}

	jobPodSucceeded1 := builder.ForPod(veleroNamespace, "job1").Labels(map[string]string{"job-name": "job1"}).ContainerStatuses(&v1.ContainerStatus{
		State: v1.ContainerState{
			Terminated: &v1.ContainerStateTerminated{},
		},
	}).Result()

	jobFailed1 := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "job2",
			Namespace:         veleroNamespace,
			Labels:            map[string]string{RepositoryNameLabel: "fake-repo"},
			CreationTimestamp: metav1.Time{Time: now.Add(time.Hour)},
		},
		Status: batchv1.JobStatus{
			StartTime:      &metav1.Time{Time: now.Add(time.Hour)},
			CompletionTime: &metav1.Time{Time: now.Add(time.Hour * 2)},
			Failed:         1,
		},
	}

	jobPodFailed1 := builder.ForPod(veleroNamespace, "job2").Labels(map[string]string{"job-name": "job2"}).ContainerStatuses(&v1.ContainerStatus{
		State: v1.ContainerState{
			Terminated: &v1.ContainerStateTerminated{
				Message: "fake-message-2",
			},
		},
	}).Result()

	jobSucceeded2 := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "job3",
			Namespace:         veleroNamespace,
			Labels:            map[string]string{RepositoryNameLabel: "fake-repo"},
			CreationTimestamp: metav1.Time{Time: now.Add(time.Hour * 2)},
		},
		Status: batchv1.JobStatus{
			StartTime:      &metav1.Time{Time: now.Add(time.Hour * 2)},
			CompletionTime: &metav1.Time{Time: now.Add(time.Hour * 3)},
			Succeeded:      1,
		},
	}

	jobPodSucceeded2 := builder.ForPod(veleroNamespace, "job3").Labels(map[string]string{"job-name": "job3"}).ContainerStatuses(&v1.ContainerStatus{
		State: v1.ContainerState{
			Terminated: &v1.ContainerStateTerminated{},
		},
	}).Result()

	jobSucceeded3 := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "job4",
			Namespace:         veleroNamespace,
			Labels:            map[string]string{RepositoryNameLabel: "fake-repo"},
			CreationTimestamp: metav1.Time{Time: now.Add(time.Hour * 3)},
		},
		Status: batchv1.JobStatus{
			StartTime:      &metav1.Time{Time: now.Add(time.Hour * 3)},
			CompletionTime: &metav1.Time{Time: now.Add(time.Hour * 4)},
			Succeeded:      1,
		},
	}

	jobPodSucceeded3 := builder.ForPod(veleroNamespace, "job4").Labels(map[string]string{"job-name": "job4"}).ContainerStatuses(&v1.ContainerStatus{
		State: v1.ContainerState{
			Terminated: &v1.ContainerStateTerminated{},
		},
	}).Result()

	schemeFail := runtime.NewScheme()

	scheme := runtime.NewScheme()
	batchv1.AddToScheme(scheme)
	v1.AddToScheme(scheme)

	testCases := []struct {
		name           string
		ctx            context.Context
		kubeClientObj  []runtime.Object
		runtimeScheme  *runtime.Scheme
		expectedStatus []velerov1api.BackupRepositoryMaintenanceStatus
		expectedError  string
	}{
		{
			name:          "list job error",
			runtimeScheme: schemeFail,
			expectedError: "error listing maintenance job for repo fake-repo: no kind is registered for the type v1.JobList in scheme \"pkg/runtime/scheme.go:100\"",
		},
		{
			name:          "job not exist",
			runtimeScheme: scheme,
		},
		{
			name:          "no matching job",
			runtimeScheme: scheme,
			kubeClientObj: []runtime.Object{
				jobOtherLabel,
			},
		},
		{
			name:          "wait complete error",
			ctx:           ctx,
			runtimeScheme: scheme,
			kubeClientObj: []runtime.Object{
				jobIncomplete,
			},
			expectedError: "error waiting maintenance job[job1] complete: context deadline exceeded",
		},
		{
			name:          "get result error",
			ctx:           context.TODO(),
			runtimeScheme: scheme,
			kubeClientObj: []runtime.Object{
				jobSucceeded1,
			},
			expectedError: "error getting maintenance job[job1] result: no pod found for job job1",
		},
		{
			name:          "less than limit",
			ctx:           context.TODO(),
			runtimeScheme: scheme,
			kubeClientObj: []runtime.Object{
				jobFailed1,
				jobSucceeded1,
				jobPodSucceeded1,
				jobPodFailed1,
			},
			expectedStatus: []velerov1api.BackupRepositoryMaintenanceStatus{
				{
					Result:            velerov1api.BackupRepositoryMaintenanceSucceeded,
					StartTimestamp:    &metav1.Time{Time: now},
					CompleteTimestamp: &metav1.Time{Time: now.Add(time.Hour)},
				},
				{
					Result:            velerov1api.BackupRepositoryMaintenanceFailed,
					StartTimestamp:    &metav1.Time{Time: now.Add(time.Hour)},
					CompleteTimestamp: &metav1.Time{Time: now.Add(time.Hour * 2)},
					Message:           "fake-message-2",
				},
			},
		},
		{
			name:          "equal to limit",
			ctx:           context.TODO(),
			runtimeScheme: scheme,
			kubeClientObj: []runtime.Object{
				jobSucceeded2,
				jobFailed1,
				jobSucceeded1,
				jobPodSucceeded1,
				jobPodFailed1,
				jobPodSucceeded2,
			},
			expectedStatus: []velerov1api.BackupRepositoryMaintenanceStatus{
				{
					Result:            velerov1api.BackupRepositoryMaintenanceSucceeded,
					StartTimestamp:    &metav1.Time{Time: now},
					CompleteTimestamp: &metav1.Time{Time: now.Add(time.Hour)},
				},
				{
					Result:            velerov1api.BackupRepositoryMaintenanceFailed,
					StartTimestamp:    &metav1.Time{Time: now.Add(time.Hour)},
					CompleteTimestamp: &metav1.Time{Time: now.Add(time.Hour * 2)},
					Message:           "fake-message-2",
				},
				{
					Result:            velerov1api.BackupRepositoryMaintenanceSucceeded,
					StartTimestamp:    &metav1.Time{Time: now.Add(time.Hour * 2)},
					CompleteTimestamp: &metav1.Time{Time: now.Add(time.Hour * 3)},
				},
			},
		},
		{
			name:          "more than limit",
			ctx:           context.TODO(),
			runtimeScheme: scheme,
			kubeClientObj: []runtime.Object{
				jobSucceeded3,
				jobSucceeded2,
				jobFailed1,
				jobSucceeded1,
				jobPodSucceeded1,
				jobPodFailed1,
				jobPodSucceeded2,
				jobPodSucceeded3,
			},
			expectedStatus: []velerov1api.BackupRepositoryMaintenanceStatus{
				{
					Result:            velerov1api.BackupRepositoryMaintenanceSucceeded,
					StartTimestamp:    &metav1.Time{Time: now},
					CompleteTimestamp: &metav1.Time{Time: now.Add(time.Hour)},
				},
				{
					Result:            velerov1api.BackupRepositoryMaintenanceFailed,
					StartTimestamp:    &metav1.Time{Time: now.Add(time.Hour)},
					CompleteTimestamp: &metav1.Time{Time: now.Add(time.Hour * 2)},
					Message:           "fake-message-2",
				},
				{
					Result:            velerov1api.BackupRepositoryMaintenanceSucceeded,
					StartTimestamp:    &metav1.Time{Time: now.Add(time.Hour * 2)},
					CompleteTimestamp: &metav1.Time{Time: now.Add(time.Hour * 3)},
				},
				{
					Result:            velerov1api.BackupRepositoryMaintenanceSucceeded,
					StartTimestamp:    &metav1.Time{Time: now.Add(time.Hour * 3)},
					CompleteTimestamp: &metav1.Time{Time: now.Add(time.Hour * 4)},
				},
			},
		},
	}

	for _, test := range testCases {
		t.Run(test.name, func(t *testing.T) {
			fakeClientBuilder := fake.NewClientBuilder()
			fakeClientBuilder = fakeClientBuilder.WithScheme(test.runtimeScheme)

			fakeClient := fakeClientBuilder.WithRuntimeObjects(test.kubeClientObj...).Build()

			history, err := WaitAllMaintenanceJobComplete(test.ctx, fakeClient, repo, 3, velerotest.NewLogger())

			if test.expectedError != "" {
				assert.EqualError(t, err, test.expectedError)
			} else {
				require.NoError(t, err)
			}

			assert.Len(t, history, len(test.expectedStatus))
			for i := 0; i < len(test.expectedStatus); i++ {
				assert.Equal(t, test.expectedStatus[i].Result, history[i].Result)
				assert.Equal(t, test.expectedStatus[i].Message, history[i].Message)
				assert.Equal(t, test.expectedStatus[i].StartTimestamp.Time, history[i].StartTimestamp.Time)
				assert.Equal(t, test.expectedStatus[i].CompleteTimestamp.Time, history[i].CompleteTimestamp.Time)
			}
		})
	}

	cancel()
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
		expectedEnv     []v1.EnvVar
		expectedEnvFrom []v1.EnvFromSource
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
									Env: []v1.EnvVar{
										{
											Name:  "test-name",
											Value: "test-value",
										},
									},
									EnvFrom: []v1.EnvFromSource{
										{
											ConfigMapRef: &v1.ConfigMapEnvSource{
												LocalObjectReference: v1.LocalObjectReference{
													Name: "test-configmap",
												},
											},
										},
										{
											SecretRef: &v1.SecretEnvSource{
												LocalObjectReference: v1.LocalObjectReference{
													Name: "test-secret",
												},
											},
										},
									},
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
			expectedEnv: []v1.EnvVar{
				{
					Name:  "test-name",
					Value: "test-value",
				},
			},
			expectedEnvFrom: []v1.EnvFromSource{
				{
					ConfigMapRef: &v1.ConfigMapEnvSource{
						LocalObjectReference: v1.LocalObjectReference{
							Name: "test-configmap",
						},
					},
				},
				{
					SecretRef: &v1.SecretEnvSource{
						LocalObjectReference: v1.LocalObjectReference{
							Name: "test-secret",
						},
					},
				},
			},
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

			// Call the function to test
			job, err := buildMaintenanceJob(cli, context.TODO(), param.BackupRepo, param.BackupLocation.Name, tc.m, *tc.m.PodResources, tc.logLevel, tc.logFormat)

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

				assert.Equal(t, param.BackupRepo.Name, job.Spec.Template.ObjectMeta.Labels[RepositoryNameLabel])

				// Check container
				assert.Len(t, job.Spec.Template.Spec.Containers, 1)
				container := job.Spec.Template.Spec.Containers[0]
				assert.Equal(t, "velero-repo-maintenance-container", container.Name)
				assert.Equal(t, "velero-image", container.Image)
				assert.Equal(t, v1.PullIfNotPresent, container.ImagePullPolicy)

				// Check container env
				assert.Equal(t, tc.expectedEnv, container.Env)
				assert.Equal(t, tc.expectedEnvFrom, container.EnvFrom)

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
