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
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	batchv1 "k8s.io/api/batch/v1"
	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"sigs.k8s.io/controller-runtime/pkg/client"

	velerov1api "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	"github.com/vmware-tanzu/velero/pkg/util/kube"
)

const (
	RepositoryNameLabel              = "velero.io/repo-name"
	GlobalKeyForRepoMaintenanceJobCM = "global"
)

type JobConfigs struct {
	// LoadAffinities is the config for repository maintenance job load affinity.
	LoadAffinities []*kube.LoadAffinity `json:"loadAffinity,omitempty"`

	// PodResources is the config for the CPU and memory resources setting.
	PodResources *kube.PodResources `json:"podResources,omitempty"`
}

func GenerateJobName(repo string) string {
	millisecond := time.Now().UTC().UnixMilli() // millisecond

	jobName := fmt.Sprintf("%s-maintain-job-%d", repo, millisecond)
	if len(jobName) > 63 { // k8s job name length limit
		jobName = fmt.Sprintf("repo-maintain-job-%d", millisecond)
	}

	return jobName
}

// DeleteOldMaintenanceJobs deletes old maintenance jobs and keeps the latest N jobs
func DeleteOldMaintenanceJobs(cli client.Client, repo string, keep int) error {
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

func WaitForJobComplete(ctx context.Context, client client.Client, job *batchv1.Job) error {
	return wait.PollUntilContextCancel(ctx, time.Second, true, func(ctx context.Context) (bool, error) {
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

func GetMaintenanceResultFromJob(cli client.Client, job *batchv1.Job) (string, error) {
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
	pod := podList.Items[0]

	statuses := pod.Status.ContainerStatuses
	if len(statuses) == 0 {
		return "", fmt.Errorf("no container statuses found for job %s", job.Name)
	}

	// we only have one maintenance container
	terminated := statuses[0].State.Terminated
	if terminated == nil {
		return "", fmt.Errorf("container for job %s is not terminated", job.Name)
	}

	return terminated.Message, nil
}

func GetLatestMaintenanceJob(cli client.Client, ns string) (*batchv1.Job, error) {
	// Get the maintenance job list by label
	jobList := &batchv1.JobList{}
	err := cli.List(context.TODO(), jobList, &client.ListOptions{
		Namespace: ns,
	},
		&client.HasLabels{RepositoryNameLabel},
	)

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

// GetMaintenanceJobConfig is called to get the Maintenance Job Config for the
// BackupRepository specified by the repo parameter.
//
// Params:
//
//	ctx: the Go context used for controller-runtime client.
//	client: the controller-runtime client.
//	logger: the logger.
//	veleroNamespace: the Velero-installed namespace. It's used to retrieve the BackupRepository.
//	repoMaintenanceJobConfig: the repository maintenance job ConfigMap name.
//	repo: the BackupRepository needs to run the maintenance Job.
func GetMaintenanceJobConfig(
	ctx context.Context,
	client client.Client,
	logger logrus.FieldLogger,
	veleroNamespace string,
	repoMaintenanceJobConfig string,
	repo *velerov1api.BackupRepository,
) (*JobConfigs, error) {
	var cm v1.ConfigMap
	if err := client.Get(
		ctx,
		types.NamespacedName{
			Namespace: veleroNamespace,
			Name:      repoMaintenanceJobConfig,
		},
		&cm,
	); err != nil {
		if apierrors.IsNotFound(err) {
			return nil, nil
		} else {
			return nil, errors.Wrapf(
				err,
				"fail to get repo maintenance job configs %s", repoMaintenanceJobConfig)
		}
	}

	if cm.Data == nil {
		return nil, errors.Errorf("data is not available in config map %s", repoMaintenanceJobConfig)
	}

	// Generate the BackupRepository key.
	// If using the BackupRepository name as the is more intuitive,
	// but the BackupRepository generation is dynamic. We cannot assume
	// they are ready when installing Velero.
	// Instead we use the volume source namespace, BSL name, and the uploader
	// type to represent the BackupRepository. The combination of those three
	// keys can identify a unique BackupRepository.
	repoJobConfigKey := repo.Spec.VolumeNamespace + "-" +
		repo.Spec.BackupStorageLocation + "-" + repo.Spec.RepositoryType

	var result *JobConfigs
	if _, ok := cm.Data[repoJobConfigKey]; ok {
		logger.Debugf("Find the repo maintenance config %s for repo %s", repoJobConfigKey, repo.Name)
		result = new(JobConfigs)
		if err := json.Unmarshal([]byte(cm.Data[repoJobConfigKey]), result); err != nil {
			return nil, errors.Wrapf(
				err,
				"fail to unmarshal configs from %s's key %s",
				repoMaintenanceJobConfig,
				repoJobConfigKey)
		}
	}

	if _, ok := cm.Data[GlobalKeyForRepoMaintenanceJobCM]; ok {
		logger.Debugf("Find the global repo maintenance config for repo %s", repo.Name)

		if result == nil {
			result = new(JobConfigs)
		}

		globalResult := new(JobConfigs)

		if err := json.Unmarshal([]byte(cm.Data[GlobalKeyForRepoMaintenanceJobCM]), globalResult); err != nil {
			return nil, errors.Wrapf(
				err,
				"fail to unmarshal configs from %s's key %s",
				repoMaintenanceJobConfig,
				GlobalKeyForRepoMaintenanceJobCM)
		}

		if result.PodResources == nil && globalResult.PodResources != nil {
			result.PodResources = globalResult.PodResources
		}

		if len(result.LoadAffinities) == 0 {
			result.LoadAffinities = globalResult.LoadAffinities
		}
	}

	return result, nil
}

// WaitIncompleteMaintenance checks all the incomplete maintenance jobs of the specified repo and wait for them to complete,
// and then return the maintenance jobs in the range of limit
func WaitIncompleteMaintenance(ctx context.Context, cli client.Client, repo *velerov1api.BackupRepository, limit int, log logrus.FieldLogger) ([]velerov1api.BackupRepositoryMaintenanceStatus, error) {
	jobList := &batchv1.JobList{}
	err := cli.List(context.TODO(), jobList, &client.ListOptions{
		Namespace: repo.Namespace,
	},
		client.MatchingLabels(map[string]string{RepositoryNameLabel: repo.Name}),
	)

	if err != nil {
		return nil, errors.Wrapf(err, "error listing maintenance job for repo %s", repo.Name)
	}

	if len(jobList.Items) == 0 {
		return nil, nil
	}

	history := []velerov1api.BackupRepositoryMaintenanceStatus{}

	for _, job := range jobList.Items {
		if job.Status.Succeeded == 0 && job.Status.Failed == 0 {
			log.Infof("Waiting for maintenance job %s to complete", job.Name)

			if err := WaitForJobComplete(ctx, cli, &job); err != nil {
				return nil, errors.Wrapf(err, "error waiting maintenance job[%s] complete", job.Name)
			}
		}

		result := velerov1api.BackupRepositoryMaintenanceSucceeded
		if job.Status.Failed > 0 {
			result = velerov1api.BackupRepositoryMaintenanceFailed
		}

		message, err := GetMaintenanceResultFromJob(cli, &job)
		if err != nil {
			return nil, errors.Wrapf(err, "error getting maintenance job[%s] result", job.Name)
		}

		history = append(history, velerov1api.BackupRepositoryMaintenanceStatus{
			Result:            result,
			StartTimestamp:    &metav1.Time{Time: job.Status.StartTime.Time},
			CompleteTimestamp: &metav1.Time{Time: job.Status.CompletionTime.Time},
			Message:           message,
		})
	}

	sort.Slice(history, func(i, j int) bool {
		return history[i].CompleteTimestamp.Time.After(history[j].CompleteTimestamp.Time)
	})

	startPos := len(history) - limit
	if startPos < 0 {
		startPos = 0
	}

	return history[startPos:], nil
}
