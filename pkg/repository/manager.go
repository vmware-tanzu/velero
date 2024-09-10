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
	"time"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/vmware-tanzu/velero/internal/credentials"
	velerov1api "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	"github.com/vmware-tanzu/velero/pkg/repository/provider"
	"github.com/vmware-tanzu/velero/pkg/util/filesystem"
	"github.com/vmware-tanzu/velero/pkg/util/kube"
	"github.com/vmware-tanzu/velero/pkg/util/logging"
	veleroutil "github.com/vmware-tanzu/velero/pkg/util/velero"
)

// SnapshotIdentifier uniquely identifies a snapshot
// taken by Velero.
type SnapshotIdentifier struct {
	// VolumeNamespace is the namespace of the pod/volume that
	// the snapshot is for.
	VolumeNamespace string `json:"volumeNamespace"`

	// BackupStorageLocation is the backup's storage location
	// name.
	BackupStorageLocation string `json:"backupStorageLocation"`

	// SnapshotID is the short ID of the snapshot.
	SnapshotID string `json:"snapshotID"`

	// RepositoryType is the type of the repository where the
	// snapshot is stored
	RepositoryType string `json:"repositoryType"`

	// Source is the source of the data saved in the repo by the snapshot
	Source string `json:"source"`

	// UploaderType is the type of uploader which saved the snapshot data
	UploaderType string `json:"uploaderType"`

	// RepoIdentifier is the identifier of the repository where the
	// snapshot is stored
	RepoIdentifier string `json:"repoIdentifier"`
}

// Manager manages backup repositories.
type Manager interface {
	// InitRepo initializes a repo with the specified name and identifier.
	InitRepo(repo *velerov1api.BackupRepository) error

	// ConnectToRepo tries to connect to the specified repo, and returns an error if it fails.
	// This is intended to be used to ensure that the repo exists/can be authenticated to.
	ConnectToRepo(repo *velerov1api.BackupRepository) error

	// PrepareRepo tries to connect to the specific repo first, if it fails because of the
	// repo is not initialized, it turns to initialize the repo
	PrepareRepo(repo *velerov1api.BackupRepository) error

	// PruneRepo deletes unused data from a repo.
	PruneRepo(repo *velerov1api.BackupRepository) error

	// UnlockRepo removes stale locks from a repo.
	UnlockRepo(repo *velerov1api.BackupRepository) error

	// Forget removes a snapshot from the list of
	// available snapshots in a repo.
	Forget(context.Context, *velerov1api.BackupRepository, string) error

	// BatchForget removes a list of snapshots from the list of
	// available snapshots in a repo.
	BatchForget(context.Context, *velerov1api.BackupRepository, []string) []error

	// DefaultMaintenanceFrequency returns the default maintenance frequency from the specific repo
	DefaultMaintenanceFrequency(repo *velerov1api.BackupRepository) (time.Duration, error)
}

type manager struct {
	namespace string
	providers map[string]provider.Provider
	// client is the Velero controller manager's client.
	// It's limited to resources in the Velero namespace.
	client                    client.Client
	repoLocker                *RepoLocker
	repoEnsurer               *Ensurer
	fileSystem                filesystem.Interface
	repoMaintenanceJobConfig  string
	podResources              kube.PodResources
	keepLatestMaintenanceJobs int
	log                       logrus.FieldLogger
	logLevel                  logrus.Level
	logFormat                 *logging.FormatFlag
}

// NewManager create a new repository manager.
func NewManager(
	namespace string,
	client client.Client,
	repoLocker *RepoLocker,
	repoEnsurer *Ensurer,
	credentialFileStore credentials.FileStore,
	credentialSecretStore credentials.SecretStore,
	repoMaintenanceJobConfig string,
	podResources kube.PodResources,
	keepLatestMaintenanceJobs int,
	log logrus.FieldLogger,
	logLevel logrus.Level,
	logFormat *logging.FormatFlag,
) Manager {
	mgr := &manager{
		namespace:                 namespace,
		client:                    client,
		providers:                 map[string]provider.Provider{},
		repoLocker:                repoLocker,
		repoEnsurer:               repoEnsurer,
		fileSystem:                filesystem.NewFileSystem(),
		repoMaintenanceJobConfig:  repoMaintenanceJobConfig,
		podResources:              podResources,
		keepLatestMaintenanceJobs: keepLatestMaintenanceJobs,
		log:                       log,
		logLevel:                  logLevel,
		logFormat:                 logFormat,
	}

	mgr.providers[velerov1api.BackupRepositoryTypeRestic] = provider.NewResticRepositoryProvider(credentialFileStore, mgr.fileSystem, mgr.log)
	mgr.providers[velerov1api.BackupRepositoryTypeKopia] = provider.NewUnifiedRepoProvider(credentials.CredentialGetter{
		FromFile:   credentialFileStore,
		FromSecret: credentialSecretStore,
	}, velerov1api.BackupRepositoryTypeKopia, mgr.log)

	return mgr
}

func (m *manager) InitRepo(repo *velerov1api.BackupRepository) error {
	m.repoLocker.LockExclusive(repo.Name)
	defer m.repoLocker.UnlockExclusive(repo.Name)

	prd, err := m.getRepositoryProvider(repo)
	if err != nil {
		return errors.WithStack(err)
	}
	param, err := m.assembleRepoParam(repo)
	if err != nil {
		return errors.WithStack(err)
	}
	return prd.InitRepo(context.Background(), param)
}

func (m *manager) ConnectToRepo(repo *velerov1api.BackupRepository) error {
	m.repoLocker.Lock(repo.Name)
	defer m.repoLocker.Unlock(repo.Name)

	prd, err := m.getRepositoryProvider(repo)
	if err != nil {
		return errors.WithStack(err)
	}
	param, err := m.assembleRepoParam(repo)
	if err != nil {
		return errors.WithStack(err)
	}
	return prd.ConnectToRepo(context.Background(), param)
}

func (m *manager) PrepareRepo(repo *velerov1api.BackupRepository) error {
	m.repoLocker.Lock(repo.Name)
	defer m.repoLocker.Unlock(repo.Name)

	prd, err := m.getRepositoryProvider(repo)
	if err != nil {
		return errors.WithStack(err)
	}
	param, err := m.assembleRepoParam(repo)
	if err != nil {
		return errors.WithStack(err)
	}
	return prd.PrepareRepo(context.Background(), param)
}

func (m *manager) PruneRepo(repo *velerov1api.BackupRepository) error {
	m.repoLocker.LockExclusive(repo.Name)
	defer m.repoLocker.UnlockExclusive(repo.Name)

	param, err := m.assembleRepoParam(repo)
	if err != nil {
		return errors.WithStack(err)
	}

	log := m.log.WithFields(logrus.Fields{
		"BSL name":  param.BackupLocation.Name,
		"repo type": param.BackupRepo.Spec.RepositoryType,
		"repo name": param.BackupRepo.Name,
		"repo UID":  param.BackupRepo.UID,
	})

	job, err := getLatestMaintenanceJob(m.client, m.namespace)
	if err != nil {
		return errors.WithStack(err)
	}

	if job != nil && job.Status.Succeeded == 0 && job.Status.Failed == 0 {
		log.Debugf("There already has a unfinished maintenance job %s/%s for repository %s, please wait for it to complete", job.Namespace, job.Name, param.BackupRepo.Name)
		return nil
	}

	jobConfig, err := getMaintenanceJobConfig(
		context.Background(),
		m.client,
		m.log,
		m.namespace,
		m.repoMaintenanceJobConfig,
		repo,
	)
	if err != nil {
		log.Infof("Cannot find the repo-maintenance-job-config ConfigMap: %s. Use default value.", err.Error())
	}

	log.Info("Start to maintenance repo")

	maintenanceJob, err := m.buildMaintenanceJob(
		jobConfig,
		param,
	)
	if err != nil {
		return errors.Wrap(err, "error to build maintenance job")
	}

	log = log.WithField("job", fmt.Sprintf("%s/%s", maintenanceJob.Namespace, maintenanceJob.Name))

	if err := m.client.Create(context.TODO(), maintenanceJob); err != nil {
		return errors.Wrap(err, "error to create maintenance job")
	}
	log.Debug("Creating maintenance job")

	defer func() {
		if err := deleteOldMaintenanceJobs(
			m.client,
			param.BackupRepo.Name,
			m.keepLatestMaintenanceJobs,
		); err != nil {
			log.WithError(err).Error("Failed to delete maintenance job")
		}
	}()

	var jobErr error
	if err := waitForJobComplete(context.TODO(), m.client, maintenanceJob); err != nil {
		log.WithError(err).Error("Error to wait for maintenance job complete")
		jobErr = err // we won't return here for job may failed by maintenance failure, we want return the actual error
	}

	result, err := getMaintenanceResultFromJob(m.client, maintenanceJob)
	if err != nil {
		return errors.Wrap(err, "error to get maintenance job result")
	}

	if result != "" {
		return errors.New(fmt.Sprintf("Maintenance job %s failed: %s", maintenanceJob.Name, result))
	}

	if jobErr != nil {
		return errors.Wrap(jobErr, "error to wait for maintenance job complete")
	}

	log.Info("Maintenance repo complete")
	return nil
}

func (m *manager) UnlockRepo(repo *velerov1api.BackupRepository) error {
	m.repoLocker.Lock(repo.Name)
	defer m.repoLocker.Unlock(repo.Name)

	prd, err := m.getRepositoryProvider(repo)
	if err != nil {
		return errors.WithStack(err)
	}
	param, err := m.assembleRepoParam(repo)
	if err != nil {
		return errors.WithStack(err)
	}
	return prd.EnsureUnlockRepo(context.Background(), param)
}

func (m *manager) Forget(ctx context.Context, repo *velerov1api.BackupRepository, snapshot string) error {
	m.repoLocker.LockExclusive(repo.Name)
	defer m.repoLocker.UnlockExclusive(repo.Name)

	prd, err := m.getRepositoryProvider(repo)
	if err != nil {
		return errors.WithStack(err)
	}
	param, err := m.assembleRepoParam(repo)
	if err != nil {
		return errors.WithStack(err)
	}

	if err := prd.BoostRepoConnect(context.Background(), param); err != nil {
		return errors.WithStack(err)
	}

	return prd.Forget(context.Background(), snapshot, param)
}

func (m *manager) BatchForget(ctx context.Context, repo *velerov1api.BackupRepository, snapshots []string) []error {
	m.repoLocker.LockExclusive(repo.Name)
	defer m.repoLocker.UnlockExclusive(repo.Name)

	prd, err := m.getRepositoryProvider(repo)
	if err != nil {
		return []error{errors.WithStack(err)}
	}
	param, err := m.assembleRepoParam(repo)
	if err != nil {
		return []error{errors.WithStack(err)}
	}

	if err := prd.BoostRepoConnect(context.Background(), param); err != nil {
		return []error{errors.WithStack(err)}
	}

	return prd.BatchForget(context.Background(), snapshots, param)
}

func (m *manager) DefaultMaintenanceFrequency(repo *velerov1api.BackupRepository) (time.Duration, error) {
	prd, err := m.getRepositoryProvider(repo)
	if err != nil {
		return 0, errors.WithStack(err)
	}

	param, err := m.assembleRepoParam(repo)
	if err != nil {
		return 0, errors.WithStack(err)
	}

	return prd.DefaultMaintenanceFrequency(context.Background(), param), nil
}

func (m *manager) getRepositoryProvider(repo *velerov1api.BackupRepository) (provider.Provider, error) {
	switch repo.Spec.RepositoryType {
	case "", velerov1api.BackupRepositoryTypeRestic:
		return m.providers[velerov1api.BackupRepositoryTypeRestic], nil
	case velerov1api.BackupRepositoryTypeKopia:
		return m.providers[velerov1api.BackupRepositoryTypeKopia], nil
	default:
		return nil, fmt.Errorf("failed to get provider for repository %s", repo.Spec.RepositoryType)
	}
}

func (m *manager) assembleRepoParam(repo *velerov1api.BackupRepository) (provider.RepoParam, error) {
	bsl := &velerov1api.BackupStorageLocation{}
	if err := m.client.Get(context.Background(), client.ObjectKey{Namespace: m.namespace, Name: repo.Spec.BackupStorageLocation}, bsl); err != nil {
		return provider.RepoParam{}, errors.WithStack(err)
	}
	return provider.RepoParam{
		BackupLocation: bsl,
		BackupRepo:     repo,
	}, nil
}

func (m *manager) buildMaintenanceJob(
	config *JobConfigs,
	param provider.RepoParam,
) (*batchv1.Job, error) {
	// Get the Velero server deployment
	deployment := &appsv1.Deployment{}
	err := m.client.Get(context.TODO(), types.NamespacedName{Name: "velero", Namespace: m.namespace}, deployment)
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
	cpuRequest := m.podResources.CPURequest
	memRequest := m.podResources.MemoryRequest
	cpuLimit := m.podResources.CPULimit
	memLimit := m.podResources.MemoryLimit
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

	// Set arguments
	args := []string{"repo-maintenance"}
	args = append(args, fmt.Sprintf("--repo-name=%s", param.BackupRepo.Spec.VolumeNamespace))
	args = append(args, fmt.Sprintf("--repo-type=%s", param.BackupRepo.Spec.RepositoryType))
	args = append(args, fmt.Sprintf("--backup-storage-location=%s", param.BackupLocation.Name))
	args = append(args, fmt.Sprintf("--log-level=%s", m.logLevel.String()))
	args = append(args, fmt.Sprintf("--log-format=%s", m.logFormat.String()))

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

	if config != nil && len(config.LoadAffinities) > 0 {
		affinity := kube.ToSystemAffinity(config.LoadAffinities)
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
