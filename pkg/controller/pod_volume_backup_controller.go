/*
Copyright 2018 the Heptio Ark contributors.

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

package controller

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	jsonpatch "github.com/evanphx/json-patch"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	corev1informers "k8s.io/client-go/informers/core/v1"
	corev1listers "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"

	velerov1api "github.com/heptio/velero/pkg/apis/velero/v1"
	velerov1client "github.com/heptio/velero/pkg/generated/clientset/versioned/typed/velero/v1"
	informers "github.com/heptio/velero/pkg/generated/informers/externalversions/velero/v1"
	listers "github.com/heptio/velero/pkg/generated/listers/velero/v1"
	"github.com/heptio/velero/pkg/restic"
	veleroexec "github.com/heptio/velero/pkg/util/exec"
	"github.com/heptio/velero/pkg/util/filesystem"
	"github.com/heptio/velero/pkg/util/kube"
)

type podVolumeBackupController struct {
	*genericController

	podVolumeBackupClient velerov1client.PodVolumeBackupsGetter
	podVolumeBackupLister listers.PodVolumeBackupLister
	secretLister          corev1listers.SecretLister
	podLister             corev1listers.PodLister
	pvcLister             corev1listers.PersistentVolumeClaimLister
	backupLocationLister  listers.BackupStorageLocationLister
	nodeName              string

	processBackupFunc func(*velerov1api.PodVolumeBackup) error
	fileSystem        filesystem.Interface
}

// NewPodVolumeBackupController creates a new pod volume backup controller.
func NewPodVolumeBackupController(
	logger logrus.FieldLogger,
	podVolumeBackupInformer informers.PodVolumeBackupInformer,
	podVolumeBackupClient velerov1client.PodVolumeBackupsGetter,
	podInformer cache.SharedIndexInformer,
	secretInformer cache.SharedIndexInformer,
	pvcInformer corev1informers.PersistentVolumeClaimInformer,
	backupLocationInformer informers.BackupStorageLocationInformer,
	nodeName string,
) Interface {
	c := &podVolumeBackupController{
		genericController:     newGenericController("pod-volume-backup", logger),
		podVolumeBackupClient: podVolumeBackupClient,
		podVolumeBackupLister: podVolumeBackupInformer.Lister(),
		podLister:             corev1listers.NewPodLister(podInformer.GetIndexer()),
		secretLister:          corev1listers.NewSecretLister(secretInformer.GetIndexer()),
		pvcLister:             pvcInformer.Lister(),
		backupLocationLister:  backupLocationInformer.Lister(),
		nodeName:              nodeName,

		fileSystem: filesystem.NewFileSystem(),
	}

	c.syncHandler = c.processQueueItem
	c.cacheSyncWaiters = append(
		c.cacheSyncWaiters,
		podVolumeBackupInformer.Informer().HasSynced,
		podInformer.HasSynced,
		secretInformer.HasSynced,
		pvcInformer.Informer().HasSynced,
		backupLocationInformer.Informer().HasSynced,
	)
	c.processBackupFunc = c.processBackup

	podVolumeBackupInformer.Informer().AddEventHandler(
		cache.ResourceEventHandlerFuncs{
			AddFunc:    c.pvbHandler,
			UpdateFunc: func(_, obj interface{}) { c.pvbHandler(obj) },
		},
	)

	return c
}

func (c *podVolumeBackupController) pvbHandler(obj interface{}) {
	req := obj.(*velerov1api.PodVolumeBackup)

	// only enqueue items for this node
	if req.Spec.Node != c.nodeName {
		return
	}

	log := loggerForPodVolumeBackup(c.logger, req)

	if req.Status.Phase != "" && req.Status.Phase != velerov1api.PodVolumeBackupPhaseNew {
		log.Debug("Backup is not new, not enqueuing")
		return
	}

	log.Debug("Enqueueing")
	c.enqueue(obj)
}

func (c *podVolumeBackupController) processQueueItem(key string) error {
	log := c.logger.WithField("key", key)
	log.Debug("Running processQueueItem")

	ns, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		log.WithError(err).Error("error splitting queue key")
		return nil
	}

	req, err := c.podVolumeBackupLister.PodVolumeBackups(ns).Get(name)
	if apierrors.IsNotFound(err) {
		log.Debug("Unable to find PodVolumeBackup")
		return nil
	}
	if err != nil {
		return errors.Wrap(err, "error getting PodVolumeBackup")
	}

	// only process new items
	switch req.Status.Phase {
	case "", velerov1api.PodVolumeBackupPhaseNew:
	default:
		return nil
	}

	// Don't mutate the shared cache
	reqCopy := req.DeepCopy()
	return c.processBackupFunc(reqCopy)
}

func loggerForPodVolumeBackup(baseLogger logrus.FieldLogger, req *velerov1api.PodVolumeBackup) logrus.FieldLogger {
	log := baseLogger.WithFields(logrus.Fields{
		"namespace": req.Namespace,
		"name":      req.Name,
	})

	if len(req.OwnerReferences) == 1 {
		log = log.WithField("backup", fmt.Sprintf("%s/%s", req.Namespace, req.OwnerReferences[0].Name))
	}

	return log
}

func (c *podVolumeBackupController) processBackup(req *velerov1api.PodVolumeBackup) error {
	log := loggerForPodVolumeBackup(c.logger, req)

	log.Info("Backup starting")

	var err error

	// update status to InProgress
	req, err = c.patchPodVolumeBackup(req, updatePhaseFunc(velerov1api.PodVolumeBackupPhaseInProgress))
	if err != nil {
		log.WithError(err).Error("Error setting phase to InProgress")
		return errors.WithStack(err)
	}

	pod, err := c.podLister.Pods(req.Spec.Pod.Namespace).Get(req.Spec.Pod.Name)
	if err != nil {
		log.WithError(err).Errorf("Error getting pod %s/%s", req.Spec.Pod.Namespace, req.Spec.Pod.Name)
		return c.fail(req, errors.Wrap(err, "error getting pod").Error(), log)
	}

	volumeDir, err := kube.GetVolumeDirectory(pod, req.Spec.Volume, c.pvcLister)
	if err != nil {
		log.WithError(err).Error("Error getting volume directory name")
		return c.fail(req, errors.Wrap(err, "error getting volume directory name").Error(), log)
	}

	pathGlob := fmt.Sprintf("/host_pods/%s/volumes/*/%s", string(req.Spec.Pod.UID), volumeDir)
	log.WithField("pathGlob", pathGlob).Debug("Looking for path matching glob")

	path, err := singlePathMatch(pathGlob)
	if err != nil {
		log.WithError(err).Error("Error uniquely identifying volume path")
		return c.fail(req, errors.Wrap(err, "error getting volume path on host").Error(), log)
	}
	log.WithField("path", path).Debugf("Found path matching glob")

	// temp creds
	file, err := restic.TempCredentialsFile(c.secretLister, req.Namespace, req.Spec.Pod.Namespace, c.fileSystem)
	if err != nil {
		log.WithError(err).Error("Error creating temp restic credentials file")
		return c.fail(req, errors.Wrap(err, "error creating temp restic credentials file").Error(), log)
	}
	// ignore error since there's nothing we can do and it's a temp file.
	defer os.Remove(file)

	resticCmd := restic.BackupCommand(
		req.Spec.RepoIdentifier,
		file,
		path,
		req.Spec.Tags,
	)

	// if this is azure, set resticCmd.Env appropriately
	var env []string
	if strings.HasPrefix(req.Spec.RepoIdentifier, "azure") {
		if env, err = restic.AzureCmdEnv(c.backupLocationLister, req.Namespace, req.Spec.BackupStorageLocation); err != nil {
			return c.fail(req, errors.Wrap(err, "error setting restic cmd env").Error(), log)
		}
		resticCmd.Env = env
	}

	var stdout, stderr string

	if stdout, stderr, err = veleroexec.RunCommand(resticCmd.Cmd()); err != nil {
		log.WithError(errors.WithStack(err)).Errorf("Error running command=%s, stdout=%s, stderr=%s", resticCmd.String(), stdout, stderr)
		return c.fail(req, fmt.Sprintf("error running restic backup, stderr=%s: %s", stderr, err.Error()), log)
	}
	log.Debugf("Ran command=%s, stdout=%s, stderr=%s", resticCmd.String(), stdout, stderr)

	snapshotID, err := restic.GetSnapshotID(req.Spec.RepoIdentifier, file, req.Spec.Tags, env)
	if err != nil {
		log.WithError(err).Error("Error getting SnapshotID")
		return c.fail(req, errors.Wrap(err, "error getting snapshot id").Error(), log)
	}

	// update status to Completed with path & snapshot id
	req, err = c.patchPodVolumeBackup(req, func(r *velerov1api.PodVolumeBackup) {
		r.Status.Path = path
		r.Status.SnapshotID = snapshotID
		r.Status.Phase = velerov1api.PodVolumeBackupPhaseCompleted
	})
	if err != nil {
		log.WithError(err).Error("Error setting phase to Completed")
		return err
	}

	log.Info("Backup completed")

	return nil
}

func (c *podVolumeBackupController) patchPodVolumeBackup(req *velerov1api.PodVolumeBackup, mutate func(*velerov1api.PodVolumeBackup)) (*velerov1api.PodVolumeBackup, error) {
	// Record original json
	oldData, err := json.Marshal(req)
	if err != nil {
		return nil, errors.Wrap(err, "error marshalling original PodVolumeBackup")
	}

	// Mutate
	mutate(req)

	// Record new json
	newData, err := json.Marshal(req)
	if err != nil {
		return nil, errors.Wrap(err, "error marshalling updated PodVolumeBackup")
	}

	patchBytes, err := jsonpatch.CreateMergePatch(oldData, newData)
	if err != nil {
		return nil, errors.Wrap(err, "error creating json merge patch for PodVolumeBackup")
	}

	req, err = c.podVolumeBackupClient.PodVolumeBackups(req.Namespace).Patch(req.Name, types.MergePatchType, patchBytes)
	if err != nil {
		return nil, errors.Wrap(err, "error patching PodVolumeBackup")
	}

	return req, nil
}

func (c *podVolumeBackupController) fail(req *velerov1api.PodVolumeBackup, msg string, log logrus.FieldLogger) error {
	if _, err := c.patchPodVolumeBackup(req, func(r *velerov1api.PodVolumeBackup) {
		r.Status.Phase = velerov1api.PodVolumeBackupPhaseFailed
		r.Status.Message = msg
	}); err != nil {
		log.WithError(err).Error("Error setting phase to Failed")
		return err
	}
	return nil
}

func updatePhaseFunc(phase velerov1api.PodVolumeBackupPhase) func(r *velerov1api.PodVolumeBackup) {
	return func(r *velerov1api.PodVolumeBackup) {
		r.Status.Phase = phase
	}
}

func singlePathMatch(path string) (string, error) {
	matches, err := filepath.Glob(path)
	if err != nil {
		return "", errors.WithStack(err)
	}

	if len(matches) != 1 {
		return "", errors.Errorf("expected one matching path, got %d", len(matches))
	}

	return matches[0], nil
}
