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
	"sort"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/pkg/errors"

	"github.com/vmware-tanzu/velero/pkg/repository/provider"
	"github.com/vmware-tanzu/velero/pkg/util/kube"
	"github.com/vmware-tanzu/velero/pkg/util/logging"
	veleroutil "github.com/vmware-tanzu/velero/pkg/util/velero"
)

const RepositoryNameLabel = "velero.io/repo-name"
const DefaultKeepLatestMaitenanceJobs = 3
const DefaultMaintenanceJobCPURequest = "0"
const DefaultMaintenanceJobCPULimit = "0"
const DefaultMaintenanceJobMemRequest = "0"
const DefaultMaintenanceJobMemLimit = "0"

// MaintenanceConfig is the configuration for the repo maintenance job
type MaintenanceConfig struct {
	KeepLatestMaitenanceJobs int
	CPURequest               string
	MemRequest               string
	CPULimit                 string
	MemLimit                 string
	LogLevelFlag             *logging.LevelFlag
	FormatFlag               *logging.FormatFlag
}

func generateJobName(repo string) string {
	millisecond := time.Now().UTC().UnixMilli() // millisecond

	jobName := fmt.Sprintf("%s-maintain-job-%d", repo, millisecond)
	if len(jobName) > 63 { // k8s job name length limit
		jobName = fmt.Sprintf("repo-maintain-job-%d", millisecond)
	}

	return jobName
}

func buildMaintenanceJob(m MaintenanceConfig, param provider.RepoParam, cli client.Client, namespace string) (*batchv1.Job, error) {
	// Get the Velero server deployment
	deployment := &appsv1.Deployment{}
	err := cli.Get(context.TODO(), types.NamespacedName{Name: "velero", Namespace: namespace}, deployment)
	if err != nil {
		return nil, err
	}

	// Get the environment variables from the Velero server deployment
	envVars := veleroutil.GetEnvVarsFromVeleroServer(deployment)

	// Get the volume mounts from the Velero server deployment
	volumeMounts := veleroutil.GetVolumeMountsFromVeleroServer(deployment)

	// Get the volumes from the Velero server deployment
	volumes := veleroutil.GetVolumesFromVeleroServer(deployment)

	// Get the service account from the Velero server deployment
	serviceAccount := veleroutil.GetServiceAccountFromVeleroServer(deployment)

	// Get image
	image := veleroutil.GetVeleroServerImage(deployment)

	// Set resource limits and requests
	if m.CPURequest == "" {
		m.CPURequest = DefaultMaintenanceJobCPURequest
	}
	if m.MemRequest == "" {
		m.MemRequest = DefaultMaintenanceJobMemRequest
	}
	if m.CPULimit == "" {
		m.CPULimit = DefaultMaintenanceJobCPULimit
	}
	if m.MemLimit == "" {
		m.MemLimit = DefaultMaintenanceJobMemLimit
	}

	resources, err := kube.ParseResourceRequirements(m.CPURequest, m.MemRequest, m.CPULimit, m.MemLimit)
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse resource requirements for maintenance job")
	}

	// Set arguments
	args := []string{"repo-maintenance"}
	args = append(args, fmt.Sprintf("--repo-name=%s", param.BackupRepo.Spec.VolumeNamespace))
	args = append(args, fmt.Sprintf("--repo-type=%s", param.BackupRepo.Spec.RepositoryType))
	args = append(args, fmt.Sprintf("--backup-storage-location=%s", param.BackupLocation.Name))
	args = append(args, fmt.Sprintf("--log-level=%s", m.LogLevelFlag.String()))
	args = append(args, fmt.Sprintf("--log-format=%s", m.FormatFlag.String()))

	// build the maintenance job
	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      generateJobName(param.BackupRepo.Name),
			Namespace: param.BackupRepo.Namespace,
			Labels: map[string]string{
				RepositoryNameLabel: param.BackupRepo.Name,
			},
		},
		Spec: batchv1.JobSpec{
			BackoffLimit: new(int32), // Never retry
			Template: v1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Name: "velero-repo-maintenance-pod",
				},
				Spec: v1.PodSpec{
					Containers: []v1.Container{
						{
							Name:  "velero-repo-maintenance-container",
							Image: image,
							Command: []string{
								"/velero",
							},
							Args:            args,
							ImagePullPolicy: v1.PullIfNotPresent,
							Env:             envVars,
							VolumeMounts:    volumeMounts,
							Resources:       resources,
						},
					},
					RestartPolicy:      v1.RestartPolicyNever,
					Volumes:            volumes,
					ServiceAccountName: serviceAccount,
				},
			},
		},
	}

	if affinity := veleroutil.GetAffinityFromVeleroServer(deployment); affinity != nil {
		job.Spec.Template.Spec.Affinity = affinity
	}

	if tolerations := veleroutil.GetTolerationsFromVeleroServer(deployment); tolerations != nil {
		job.Spec.Template.Spec.Tolerations = tolerations
	}

	if nodeSelector := veleroutil.GetNodeSelectorFromVeleroServer(deployment); nodeSelector != nil {
		job.Spec.Template.Spec.NodeSelector = nodeSelector
	}

	if labels := veleroutil.GetVeleroServerLables(deployment); len(labels) > 0 {
		job.Spec.Template.Labels = labels
	}

	if annotations := veleroutil.GetVeleroServerAnnotations(deployment); len(annotations) > 0 {
		job.Spec.Template.Annotations = annotations
	}

	return job, nil
}

// deleteOldMaintenanceJobs deletes old maintenance jobs and keeps the latest N jobs
func deleteOldMaintenanceJobs(cli client.Client, repo string, keep int) error {
	// Get the maintenance job list by label
	jobList := &batchv1.JobList{}
	err := cli.List(context.TODO(), jobList, client.MatchingLabels(map[string]string{RepositoryNameLabel: repo}))
	if err != nil {
		return err
	}

	// Delete old maintenance jobs
	if len(jobList.Items) > keep {
		sort.Slice(jobList.Items, func(i, j int) bool {
			return jobList.Items[i].CreationTimestamp.Before(&jobList.Items[j].CreationTimestamp)
		})
		for i := 0; i < len(jobList.Items)-keep; i++ {
			err = cli.Delete(context.TODO(), &jobList.Items[i], client.PropagationPolicy(metav1.DeletePropagationBackground))
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func waitForJobComplete(ctx context.Context, client client.Client, job *batchv1.Job) error {
	return wait.PollUntilContextCancel(ctx, 1, true, func(ctx context.Context) (bool, error) {
		err := client.Get(ctx, types.NamespacedName{Namespace: job.Namespace, Name: job.Name}, job)
		if err != nil && !apierrors.IsNotFound(err) {
			return false, err
		}

		if job.Status.Succeeded > 0 {
			return true, nil
		}

		if job.Status.Failed > 0 {
			return true, fmt.Errorf("maintenance job %s/%s failed", job.Namespace, job.Name)
		}
		return false, nil
	})
}

func getMaintenanceResultFromJob(cli client.Client, job *batchv1.Job) (string, error) {
	// Get the maintenance job related pod by label selector
	podList := &v1.PodList{}
	err := cli.List(context.TODO(), podList, client.InNamespace(job.Namespace), client.MatchingLabels(map[string]string{"job-name": job.Name}))
	if err != nil {
		return "", err
	}

	if len(podList.Items) == 0 {
		return "", fmt.Errorf("no pod found for job %s", job.Name)
	}

	// we only have one maintenance pod for the job
	return podList.Items[0].Status.ContainerStatuses[0].State.Terminated.Message, nil
}

func GetLatestMaintenanceJob(cli client.Client, repo string) (*batchv1.Job, error) {
	// Get the maintenance job list by label
	jobList := &batchv1.JobList{}
	err := cli.List(context.TODO(), jobList, client.MatchingLabels(map[string]string{RepositoryNameLabel: repo}))
	if err != nil {
		return nil, err
	}

	if len(jobList.Items) == 0 {
		return nil, nil
	}

	// Get the latest maintenance job
	sort.Slice(jobList.Items, func(i, j int) bool {
		return jobList.Items[i].CreationTimestamp.Time.After(jobList.Items[j].CreationTimestamp.Time)
	})
	return &jobList.Items[0], nil
}
