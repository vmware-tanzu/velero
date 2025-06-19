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
	"fmt"
	"math"
	"strings"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	batchv1 "k8s.io/api/batch/v1"
	corev1api "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	velerov1api "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	"github.com/vmware-tanzu/velero/pkg/builder"
	"github.com/vmware-tanzu/velero/pkg/repository/provider"
	velerotest "github.com/vmware-tanzu/velero/pkg/test"
	"github.com/vmware-tanzu/velero/pkg/util/kube"
	"github.com/vmware-tanzu/velero/pkg/util/logging"

	appsv1api "k8s.io/api/apps/v1"
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
func TestDeleteOldJobs(t *testing.T) {
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
	err := DeleteOldJobs(cli, repo, keep)
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

	schemeFail := runtime.NewScheme()

	scheme := runtime.NewScheme()
	batchv1.AddToScheme(scheme)

	waitCompletionBackOff1 := wait.Backoff{
		Duration: time.Second,
		Steps:    math.MaxInt,
		Factor:   2,
		Cap:      time.Second * 12,
	}

	waitCompletionBackOff2 := wait.Backoff{
		Duration: time.Second,
		Steps:    math.MaxInt,
		Factor:   2,
		Cap:      time.Second * 2,
	}

	// Define test cases
	tests := []struct {
		description   string // Test case description
		kubeClientObj []runtime.Object
		runtimeScheme *runtime.Scheme
		jobStatus     batchv1.JobStatus // Job status to set for the test
		logBackOff    wait.Backoff
		updateAfter   time.Duration
		expectedLogs  int
		expectError   bool // Whether an error is expected
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
		{
			description:   "Log backoff not to cap",
			runtimeScheme: scheme,
			kubeClientObj: []runtime.Object{
				job,
			},
			logBackOff:   waitCompletionBackOff1,
			updateAfter:  time.Second * 8,
			expectedLogs: 3,
		},
		{
			description:   "Log backoff to cap",
			runtimeScheme: scheme,
			kubeClientObj: []runtime.Object{
				job,
			},
			logBackOff:   waitCompletionBackOff2,
			updateAfter:  time.Second * 6,
			expectedLogs: 3,
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

			buffer := []string{}
			logger := velerotest.NewMultipleLogger(&buffer)

			waitCompletionBackOff = tc.logBackOff

			if tc.updateAfter != 0 {
				go func() {
					time.Sleep(tc.updateAfter)

					original := job.DeepCopy()
					job.Status.Succeeded = 1
					err := fakeClient.Status().Patch(context.Background(), job, client.MergeFrom(original))
					require.NoError(t, err)
				}()
			}

			// Call the function
			_, err := waitForJobComplete(context.Background(), fakeClient, job.Namespace, job.Name, logger)

			// Check if the error matches the expectation
			if tc.expectError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}

			assert.LessOrEqual(t, len(buffer), tc.expectedLogs)
		})
	}
}

func TestGetResultFromJob(t *testing.T) {
	// Set up test job
	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-job",
			Namespace: "default",
		},
	}

	// Set up test pod with no status
	pod := &corev1api.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pod",
			Namespace: "default",
			Labels:    map[string]string{"job-name": job.Name},
		},
	}

	// Create a fake Kubernetes client
	cli := fake.NewClientBuilder().Build()

	// test an error should be returned
	result, err := getResultFromJob(cli, job)
	require.EqualError(t, err, "no pod found for job test-job")
	assert.Empty(t, result)

	cli = fake.NewClientBuilder().WithObjects(job, pod).Build()

	// test an error should be returned
	result, err = getResultFromJob(cli, job)
	require.EqualError(t, err, "no container statuses found for job test-job")
	assert.Empty(t, result)

	// Set a non-terminated container status to the pod
	pod.Status = corev1api.PodStatus{
		ContainerStatuses: []corev1api.ContainerStatus{
			{
				State: corev1api.ContainerState{},
			},
		},
	}

	// Test an error should be returned
	cli = fake.NewClientBuilder().WithObjects(job, pod).Build()
	result, err = getResultFromJob(cli, job)
	require.EqualError(t, err, "container for job test-job is not terminated")
	assert.Empty(t, result)

	// Set a terminated container status to the pod
	pod.Status = corev1api.PodStatus{
		ContainerStatuses: []corev1api.ContainerStatus{
			{
				State: corev1api.ContainerState{
					Terminated: &corev1api.ContainerStateTerminated{},
				},
			},
		},
	}

	// This call should return the termination message with no error
	cli = fake.NewClientBuilder().WithObjects(job, pod).Build()
	result, err = getResultFromJob(cli, job)
	require.NoError(t, err)
	assert.Empty(t, result)

	// Set a terminated container status with invalidate message to the pod
	pod.Status = corev1api.PodStatus{
		ContainerStatuses: []corev1api.ContainerStatus{
			{
				State: corev1api.ContainerState{
					Terminated: &corev1api.ContainerStateTerminated{
						Message: "fake-message",
					},
				},
			},
		},
	}

	cli = fake.NewClientBuilder().WithObjects(job, pod).Build()
	result, err = getResultFromJob(cli, job)
	require.EqualError(t, err, "error to locate repo maintenance error indicator from termination message")
	assert.Empty(t, result)

	// Set a terminated container status with empty maintenance error to the pod
	pod.Status = corev1api.PodStatus{
		ContainerStatuses: []corev1api.ContainerStatus{
			{
				State: corev1api.ContainerState{
					Terminated: &corev1api.ContainerStateTerminated{
						Message: "Repo maintenance error: ",
					},
				},
			},
		},
	}

	cli = fake.NewClientBuilder().WithObjects(job, pod).Build()
	result, err = getResultFromJob(cli, job)
	require.EqualError(t, err, "nothing after repo maintenance error indicator in termination message")
	assert.Empty(t, result)

	// Set a terminated container status with maintenance error to the pod
	pod.Status = corev1api.PodStatus{
		ContainerStatuses: []corev1api.ContainerStatus{
			{
				State: corev1api.ContainerState{
					Terminated: &corev1api.ContainerStateTerminated{
						Message: "Repo maintenance error: fake-error",
					},
				},
			},
		},
	}

	cli = fake.NewClientBuilder().WithObjects(job, pod).Build()
	result, err = getResultFromJob(cli, job)
	require.NoError(t, err)
	assert.Equal(t, "fake-error", result)
}

func TestGetJobConfig(t *testing.T) {
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
		repoJobConfig  *corev1api.ConfigMap
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
			repoJobConfig: &corev1api.ConfigMap{
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
			repoJobConfig: &corev1api.ConfigMap{
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
			repoJobConfig: &corev1api.ConfigMap{
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
			repoJobConfig: &corev1api.ConfigMap{
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

			jobConfig, err := getJobConfig(
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

func TestWaitAllJobsComplete(t *testing.T) {
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

	jobPodSucceeded1 := builder.ForPod(veleroNamespace, "job1").Labels(map[string]string{"job-name": "job1"}).ContainerStatuses(&corev1api.ContainerStatus{
		State: corev1api.ContainerState{
			Terminated: &corev1api.ContainerStateTerminated{},
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
			StartTime: &metav1.Time{Time: now.Add(time.Hour)},
			Failed:    1,
		},
	}

	jobPodFailed1 := builder.ForPod(veleroNamespace, "job2").Labels(map[string]string{"job-name": "job2"}).ContainerStatuses(&corev1api.ContainerStatus{
		State: corev1api.ContainerState{
			Terminated: &corev1api.ContainerStateTerminated{
				Message: "Repo maintenance error: fake-message-2",
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

	jobPodSucceeded2 := builder.ForPod(veleroNamespace, "job3").Labels(map[string]string{"job-name": "job3"}).ContainerStatuses(&corev1api.ContainerStatus{
		State: corev1api.ContainerState{
			Terminated: &corev1api.ContainerStateTerminated{},
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

	jobPodSucceeded3 := builder.ForPod(veleroNamespace, "job4").Labels(map[string]string{"job-name": "job4"}).ContainerStatuses(&corev1api.ContainerStatus{
		State: corev1api.ContainerState{
			Terminated: &corev1api.ContainerStateTerminated{},
		},
	}).Result()

	schemeFail := runtime.NewScheme()

	scheme := runtime.NewScheme()
	batchv1.AddToScheme(scheme)
	corev1api.AddToScheme(scheme)

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
			name:          "get result error on succeeded job",
			ctx:           context.TODO(),
			runtimeScheme: scheme,
			kubeClientObj: []runtime.Object{
				jobSucceeded1,
			},
			expectedStatus: []velerov1api.BackupRepositoryMaintenanceStatus{
				{
					Result:            velerov1api.BackupRepositoryMaintenanceSucceeded,
					StartTimestamp:    &metav1.Time{Time: now},
					CompleteTimestamp: &metav1.Time{Time: now.Add(time.Hour)},
				},
			},
		},
		{
			name:          "get result error on failed job",
			ctx:           context.TODO(),
			runtimeScheme: scheme,
			kubeClientObj: []runtime.Object{
				jobFailed1,
			},
			expectedStatus: []velerov1api.BackupRepositoryMaintenanceStatus{
				{
					Result:         velerov1api.BackupRepositoryMaintenanceFailed,
					StartTimestamp: &metav1.Time{Time: now.Add(time.Hour)},
					Message:        "Repo maintenance failed but result is not retrieveable, err: no pod found for job job2",
				},
			},
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
					Result:         velerov1api.BackupRepositoryMaintenanceFailed,
					StartTimestamp: &metav1.Time{Time: now.Add(time.Hour)},
					Message:        "fake-message-2",
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
					Result:         velerov1api.BackupRepositoryMaintenanceFailed,
					StartTimestamp: &metav1.Time{Time: now.Add(time.Hour)},
					Message:        "fake-message-2",
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
					Result:         velerov1api.BackupRepositoryMaintenanceFailed,
					StartTimestamp: &metav1.Time{Time: now.Add(time.Hour)},
					Message:        "fake-message-2",
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

			history, err := WaitAllJobsComplete(test.ctx, fakeClient, repo, 3, velerotest.NewLogger())

			if test.expectedError != "" {
				require.EqualError(t, err, test.expectedError)
			} else {
				require.NoError(t, err)
			}

			assert.Len(t, history, len(test.expectedStatus))
			for i := 0; i < len(test.expectedStatus); i++ {
				assert.Equal(t, test.expectedStatus[i].Result, history[i].Result)
				assert.Equal(t, test.expectedStatus[i].Message, history[i].Message)
				assert.Equal(t, test.expectedStatus[i].StartTimestamp.Time, history[i].StartTimestamp.Time)

				if test.expectedStatus[i].CompleteTimestamp == nil {
					assert.Nil(t, history[i].CompleteTimestamp)
				} else {
					assert.Equal(t, test.expectedStatus[i].CompleteTimestamp.Time, history[i].CompleteTimestamp.Time)
				}
			}
		})
	}

	cancel()
}

func TestBuildJob(t *testing.T) {
	deploy := appsv1api.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "velero",
			Namespace: "velero",
		},
		Spec: appsv1api.DeploymentSpec{
			Template: corev1api.PodTemplateSpec{
				Spec: corev1api.PodSpec{
					Containers: []corev1api.Container{
						{
							Name:  "velero-repo-maintenance-container",
							Image: "velero-image",
							Env: []corev1api.EnvVar{
								{
									Name:  "test-name",
									Value: "test-value",
								},
							},
							EnvFrom: []corev1api.EnvFromSource{
								{
									ConfigMapRef: &corev1api.ConfigMapEnvSource{
										LocalObjectReference: corev1api.LocalObjectReference{
											Name: "test-configmap",
										},
									},
								},
								{
									SecretRef: &corev1api.SecretEnvSource{
										LocalObjectReference: corev1api.LocalObjectReference{
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
	}

	deploy2 := deploy
	deploy2.Spec.Template.Labels = map[string]string{"azure.workload.identity/use": "fake-label-value"}

	testCases := []struct {
		name             string
		m                *JobConfigs
		deploy           *appsv1api.Deployment
		logLevel         logrus.Level
		logFormat        *logging.FormatFlag
		thirdPartyLabel  map[string]string
		expectedJobName  string
		expectedError    bool
		expectedEnv      []corev1api.EnvVar
		expectedEnvFrom  []corev1api.EnvFromSource
		expectedPodLabel map[string]string
	}{
		{
			name: "Valid maintenance job without third party labels",
			m: &JobConfigs{
				PodResources: &kube.PodResources{
					CPURequest:    "100m",
					MemoryRequest: "128Mi",
					CPULimit:      "200m",
					MemoryLimit:   "256Mi",
				},
			},
			deploy:          &deploy,
			logLevel:        logrus.InfoLevel,
			logFormat:       logging.NewFormatFlag(),
			expectedJobName: "test-123-maintain-job",
			expectedError:   false,
			expectedEnv: []corev1api.EnvVar{
				{
					Name:  "test-name",
					Value: "test-value",
				},
			},
			expectedEnvFrom: []corev1api.EnvFromSource{
				{
					ConfigMapRef: &corev1api.ConfigMapEnvSource{
						LocalObjectReference: corev1api.LocalObjectReference{
							Name: "test-configmap",
						},
					},
				},
				{
					SecretRef: &corev1api.SecretEnvSource{
						LocalObjectReference: corev1api.LocalObjectReference{
							Name: "test-secret",
						},
					},
				},
			},
			expectedPodLabel: map[string]string{
				RepositoryNameLabel: "test-123",
			},
		},
		{
			name: "Valid maintenance job with third party labels",
			m: &JobConfigs{
				PodResources: &kube.PodResources{
					CPURequest:    "100m",
					MemoryRequest: "128Mi",
					CPULimit:      "200m",
					MemoryLimit:   "256Mi",
				},
			},
			deploy:          &deploy2,
			logLevel:        logrus.InfoLevel,
			logFormat:       logging.NewFormatFlag(),
			expectedJobName: "test-123-maintain-job",
			expectedError:   false,
			expectedEnv: []corev1api.EnvVar{
				{
					Name:  "test-name",
					Value: "test-value",
				},
			},
			expectedEnvFrom: []corev1api.EnvFromSource{
				{
					ConfigMapRef: &corev1api.ConfigMapEnvSource{
						LocalObjectReference: corev1api.LocalObjectReference{
							Name: "test-configmap",
						},
					},
				},
				{
					SecretRef: &corev1api.SecretEnvSource{
						LocalObjectReference: corev1api.LocalObjectReference{
							Name: "test-secret",
						},
					},
				},
			},
			expectedPodLabel: map[string]string{
				RepositoryNameLabel:           "test-123",
				"azure.workload.identity/use": "fake-label-value",
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
			_ = appsv1api.AddToScheme(scheme)
			_ = velerov1api.AddToScheme(scheme)
			cli := fake.NewClientBuilder().WithScheme(scheme).WithRuntimeObjects(objs...).Build()

			// Call the function to test
			job, err := buildJob(cli, context.TODO(), param.BackupRepo, param.BackupLocation.Name, tc.m, *tc.m.PodResources, tc.logLevel, tc.logFormat)

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

				assert.Equal(t, param.BackupRepo.Name, job.Spec.Template.ObjectMeta.Labels[RepositoryNameLabel])

				// Check container
				assert.Len(t, job.Spec.Template.Spec.Containers, 1)
				container := job.Spec.Template.Spec.Containers[0]
				assert.Equal(t, "velero-repo-maintenance-container", container.Name)
				assert.Equal(t, "velero-image", container.Image)
				assert.Equal(t, corev1api.PullIfNotPresent, container.ImagePullPolicy)

				// Check container env
				assert.Equal(t, tc.expectedEnv, container.Env)
				assert.Equal(t, tc.expectedEnvFrom, container.EnvFrom)

				// Check resources
				expectedResources := corev1api.ResourceRequirements{
					Requests: corev1api.ResourceList{
						corev1api.ResourceCPU:    resource.MustParse(tc.m.PodResources.CPURequest),
						corev1api.ResourceMemory: resource.MustParse(tc.m.PodResources.MemoryRequest),
					},
					Limits: corev1api.ResourceList{
						corev1api.ResourceCPU:    resource.MustParse(tc.m.PodResources.CPULimit),
						corev1api.ResourceMemory: resource.MustParse(tc.m.PodResources.MemoryLimit),
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

				assert.Equal(t, tc.expectedPodLabel, job.Spec.Template.Labels)
			}
		})
	}
}
