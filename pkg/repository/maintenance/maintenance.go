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
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	batchv1 "k8s.io/api/batch/v1"
	corev1api "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"sigs.k8s.io/controller-runtime/pkg/client"

	velerov1api "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	"github.com/vmware-tanzu/velero/pkg/util"
	"github.com/vmware-tanzu/velero/pkg/util/kube"

	appsv1api "k8s.io/api/apps/v1"

	veleroutil "github.com/vmware-tanzu/velero/pkg/util/velero"

	"github.com/vmware-tanzu/velero/pkg/util/logging"
)

const (
	RepositoryNameLabel              = "velero.io/repo-name"
	GlobalKeyForRepoMaintenanceJobCM = "global"
	TerminationLogIndicator          = "Repo maintenance error: "
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

// DeleteOldJobs deletes old maintenance jobs and keeps the latest N jobs
func DeleteOldJobs(cli client.Client, repo string, keep int) error {
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

var waitCompletionBackOff = wait.Backoff{
	Duration: time.Minute * 20,
	Steps:    math.MaxInt,
	Factor:   2,
	Cap:      time.Hour * 12,
}

// waitForJobComplete wait for completion of the specified job and update the latest job object
func waitForJobComplete(ctx context.Context, client client.Client, ns string, job string, logger logrus.FieldLogger) (*batchv1.Job, error) {
	var ret *batchv1.Job

	backOff := waitCompletionBackOff

	startTime := time.Now()
	nextCheckpoint := startTime.Add(backOff.Step())

	err := wait.PollUntilContextCancel(ctx, time.Second, true, func(ctx context.Context) (bool, error) {
		updated := &batchv1.Job{}
		err := client.Get(ctx, types.NamespacedName{Namespace: ns, Name: job}, updated)
		if err != nil && !apierrors.IsNotFound(err) {
			return false, err
		}

		ret = updated

		if updated.Status.Succeeded > 0 {
			return true, nil
		}

		if updated.Status.Failed > 0 {
			return true, nil
		}

		now := time.Now()
		if now.After(nextCheckpoint) {
			logger.Warnf("Repo maintenance job %s has lasted %v minutes", job, now.Sub(startTime).Minutes())
			nextCheckpoint = now.Add(backOff.Step())
		}

		return false, nil
	})

	return ret, err
}

func getResultFromJob(cli client.Client, job *batchv1.Job) (string, error) {
	// Get the maintenance job related pod by label selector
	podList := &corev1api.PodList{}
	err := cli.List(context.TODO(), podList, client.InNamespace(job.Namespace), client.MatchingLabels(map[string]string{"job-name": job.Name}))
	if err != nil {
		return "", err
	}

	if len(podList.Items) == 0 {
		return "", errors.Errorf("no pod found for job %s", job.Name)
	}

	// we only have one maintenance pod for the job
	pod := podList.Items[0]

	statuses := pod.Status.ContainerStatuses
	if len(statuses) == 0 {
		return "", errors.Errorf("no container statuses found for job %s", job.Name)
	}

	// we only have one maintenance container
	terminated := statuses[0].State.Terminated
	if terminated == nil {
		return "", errors.Errorf("container for job %s is not terminated", job.Name)
	}

	if terminated.Message == "" {
		return "", nil
	}

	idx := strings.Index(terminated.Message, TerminationLogIndicator)
	if idx == -1 {
		return "", errors.New("error to locate repo maintenance error indicator from termination message")
	}

	if idx+len(TerminationLogIndicator) >= len(terminated.Message) {
		return "", errors.New("nothing after repo maintenance error indicator in termination message")
	}

	return terminated.Message[idx+len(TerminationLogIndicator):], nil
}

// getJobConfig is called to get the Maintenance Job Config for the
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
func getJobConfig(
	ctx context.Context,
	client client.Client,
	logger logrus.FieldLogger,
	veleroNamespace string,
	repoMaintenanceJobConfig string,
	repo *velerov1api.BackupRepository,
) (*JobConfigs, error) {
	var cm corev1api.ConfigMap
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

// WaitJobComplete waits the completion of the specified maintenance job and return the BackupRepositoryMaintenanceStatus
func WaitJobComplete(cli client.Client, ctx context.Context, jobName, ns string, logger logrus.FieldLogger) (velerov1api.BackupRepositoryMaintenanceStatus, error) {
	log := logger.WithField("job name", jobName)

	maintenanceJob, err := waitForJobComplete(ctx, cli, ns, jobName, logger)
	if err != nil {
		return velerov1api.BackupRepositoryMaintenanceStatus{}, errors.Wrap(err, "error to wait for maintenance job complete")
	}

	log.Infof("Maintenance repo complete, succeeded %v, failed %v", maintenanceJob.Status.Succeeded, maintenanceJob.Status.Failed)

	result := ""
	if maintenanceJob.Status.Failed > 0 {
		if r, err := getResultFromJob(cli, maintenanceJob); err != nil {
			log.WithError(err).Warn("Failed to get maintenance job result")
			result = "Repo maintenance failed but result is not retrieveable"
		} else {
			result = r
		}
	}

	return composeStatusFromJob(maintenanceJob, result), nil
}

// WaitAllJobsComplete checks all the incomplete maintenance jobs of the specified repo and wait for them to complete,
// and then return the maintenance jobs' status in the range of limit
func WaitAllJobsComplete(ctx context.Context, cli client.Client, repo *velerov1api.BackupRepository, limit int, log logrus.FieldLogger) ([]velerov1api.BackupRepositoryMaintenanceStatus, error) {
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

	sort.Slice(jobList.Items, func(i, j int) bool {
		return jobList.Items[i].CreationTimestamp.Time.Before(jobList.Items[j].CreationTimestamp.Time)
	})

	history := []velerov1api.BackupRepositoryMaintenanceStatus{}

	startPos := len(jobList.Items) - limit
	if startPos < 0 {
		startPos = 0
	}

	for i := startPos; i < len(jobList.Items); i++ {
		job := &jobList.Items[i]

		if job.Status.Succeeded == 0 && job.Status.Failed == 0 {
			log.Infof("Waiting for maintenance job %s to complete", job.Name)

			updated, err := waitForJobComplete(ctx, cli, job.Namespace, job.Name, log)
			if err != nil {
				return nil, errors.Wrapf(err, "error waiting maintenance job[%s] complete", job.Name)
			}

			job = updated
		}

		message := ""
		if job.Status.Failed > 0 {
			if msg, err := getResultFromJob(cli, job); err != nil {
				log.WithError(err).Warnf("Failed to get result of maintenance job %s", job.Name)
				message = fmt.Sprintf("Repo maintenance failed but result is not retrieveable, err: %v", err)
			} else {
				message = msg
			}
		}

		history = append(history, composeStatusFromJob(job, message))
	}

	return history, nil
}

// StartNewJob creates a new maintenance job
func StartNewJob(cli client.Client, ctx context.Context, repo *velerov1api.BackupRepository, repoMaintenanceJobConfig string,
	podResources kube.PodResources, logLevel logrus.Level, logFormat *logging.FormatFlag, logger logrus.FieldLogger) (string, error) {
	bsl := &velerov1api.BackupStorageLocation{}
	if err := cli.Get(ctx, client.ObjectKey{Namespace: repo.Namespace, Name: repo.Spec.BackupStorageLocation}, bsl); err != nil {
		return "", errors.WithStack(err)
	}

	log := logger.WithFields(logrus.Fields{
		"BSL name":  bsl.Name,
		"repo type": repo.Spec.RepositoryType,
		"repo name": repo.Name,
		"repo UID":  repo.UID,
	})

	jobConfig, err := getJobConfig(
		ctx,
		cli,
		log,
		repo.Namespace,
		repoMaintenanceJobConfig,
		repo,
	)
	if err != nil {
		log.Warnf("Fail to find the ConfigMap %s to build maintenance job with error: %s. Use default value.",
			repo.Namespace+"/"+repoMaintenanceJobConfig,
			err.Error(),
		)
	}

	log.Info("Starting maintenance repo")

	maintenanceJob, err := buildJob(cli, ctx, repo, bsl.Name, jobConfig, podResources, logLevel, logFormat)
	if err != nil {
		return "", errors.Wrap(err, "error to build maintenance job")
	}

	log = log.WithField("job", fmt.Sprintf("%s/%s", maintenanceJob.Namespace, maintenanceJob.Name))

	if err := cli.Create(context.TODO(), maintenanceJob); err != nil {
		return "", errors.Wrap(err, "error to create maintenance job")
	}

	log.Info("Repo maintenance job started")

	return maintenanceJob.Name, nil
}

func buildJob(cli client.Client, ctx context.Context, repo *velerov1api.BackupRepository, bslName string, config *JobConfigs,
	podResources kube.PodResources, logLevel logrus.Level, logFormat *logging.FormatFlag) (*batchv1.Job, error) {
	// Get the Velero server deployment
	deployment := &appsv1api.Deployment{}
	err := cli.Get(ctx, types.NamespacedName{Name: "velero", Namespace: repo.Namespace}, deployment)
	if err != nil {
		return nil, err
	}

	// Get the environment variables from the Velero server deployment
	envVars := veleroutil.GetEnvVarsFromVeleroServer(deployment)

	// Get the referenced storage from the Velero server deployment
	envFromSources := veleroutil.GetEnvFromSourcesFromVeleroServer(deployment)

	// Get the volume mounts from the Velero server deployment
	volumeMounts := veleroutil.GetVolumeMountsFromVeleroServer(deployment)

	// Get the volumes from the Velero server deployment
	volumes := veleroutil.GetVolumesFromVeleroServer(deployment)

	// Get the service account from the Velero server deployment
	serviceAccount := veleroutil.GetServiceAccountFromVeleroServer(deployment)

	// Get image
	image := veleroutil.GetVeleroServerImage(deployment)

	// Set resource limits and requests
	cpuRequest := podResources.CPURequest
	memRequest := podResources.MemoryRequest
	cpuLimit := podResources.CPULimit
	memLimit := podResources.MemoryLimit
	if config != nil && config.PodResources != nil {
		cpuRequest = config.PodResources.CPURequest
		memRequest = config.PodResources.MemoryRequest
		cpuLimit = config.PodResources.CPULimit
		memLimit = config.PodResources.MemoryLimit
	}
	resources, err := kube.ParseResourceRequirements(cpuRequest, memRequest, cpuLimit, memLimit)
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse resource requirements for maintenance job")
	}

	podLabels := map[string]string{
		RepositoryNameLabel: repo.Name,
	}

	for _, k := range util.ThirdPartyLabels {
		if v := veleroutil.GetVeleroServerLabelValue(deployment, k); v != "" {
			podLabels[k] = v
		}
	}

	podAnnotations := map[string]string{}
	for _, k := range util.ThirdPartyAnnotations {
		if v := veleroutil.GetVeleroServerAnnotationValue(deployment, k); v != "" {
			podAnnotations[k] = v
		}
	}

	// Set arguments
	args := []string{"repo-maintenance"}
	args = append(args, fmt.Sprintf("--repo-name=%s", repo.Spec.VolumeNamespace))
	args = append(args, fmt.Sprintf("--repo-type=%s", repo.Spec.RepositoryType))
	args = append(args, fmt.Sprintf("--backup-storage-location=%s", bslName))
	args = append(args, fmt.Sprintf("--log-level=%s", logLevel.String()))
	args = append(args, fmt.Sprintf("--log-format=%s", logFormat.String()))

	// build the maintenance job
	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      GenerateJobName(repo.Name),
			Namespace: repo.Namespace,
			Labels: map[string]string{
				RepositoryNameLabel: repo.Name,
			},
		},
		Spec: batchv1.JobSpec{
			BackoffLimit: new(int32), // Never retry
			Template: corev1api.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Name:        "velero-repo-maintenance-pod",
					Labels:      podLabels,
					Annotations: podAnnotations,
				},
				Spec: corev1api.PodSpec{
					Containers: []corev1api.Container{
						{
							Name:  "velero-repo-maintenance-container",
							Image: image,
							Command: []string{
								"/velero",
							},
							Args:                     args,
							ImagePullPolicy:          corev1api.PullIfNotPresent,
							Env:                      envVars,
							EnvFrom:                  envFromSources,
							VolumeMounts:             volumeMounts,
							Resources:                resources,
							TerminationMessagePolicy: corev1api.TerminationMessageFallbackToLogsOnError,
						},
					},
					RestartPolicy:      corev1api.RestartPolicyNever,
					Volumes:            volumes,
					ServiceAccountName: serviceAccount,
					Tolerations: []corev1api.Toleration{
						{
							Key:      "os",
							Operator: "Equal",
							Effect:   "NoSchedule",
							Value:    "windows",
						},
					},
				},
			},
		},
	}

	if config != nil && len(config.LoadAffinities) > 0 {
		affinity := kube.ToSystemAffinity(config.LoadAffinities)
		job.Spec.Template.Spec.Affinity = affinity
	}

	return job, nil
}

func composeStatusFromJob(job *batchv1.Job, message string) velerov1api.BackupRepositoryMaintenanceStatus {
	result := velerov1api.BackupRepositoryMaintenanceSucceeded
	if job.Status.Failed > 0 {
		result = velerov1api.BackupRepositoryMaintenanceFailed
	}

	return velerov1api.BackupRepositoryMaintenanceStatus{
		Result:            result,
		StartTimestamp:    &job.CreationTimestamp,
		CompleteTimestamp: job.Status.CompletionTime,
		Message:           message,
	}
}
