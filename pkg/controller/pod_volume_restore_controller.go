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
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"

	jsonpatch "github.com/evanphx/json-patch"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"

	corev1api "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	corev1informers "k8s.io/client-go/informers/core/v1"
	corev1listers "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"

	arkv1api "github.com/heptio/ark/pkg/apis/ark/v1"
	arkv1client "github.com/heptio/ark/pkg/generated/clientset/versioned/typed/ark/v1"
	informers "github.com/heptio/ark/pkg/generated/informers/externalversions/ark/v1"
	listers "github.com/heptio/ark/pkg/generated/listers/ark/v1"
	"github.com/heptio/ark/pkg/restic"
	"github.com/heptio/ark/pkg/util/boolptr"
	"github.com/heptio/ark/pkg/util/kube"
)

type podVolumeRestoreController struct {
	*genericController

	podVolumeRestoreClient arkv1client.PodVolumeRestoresGetter
	podVolumeRestoreLister listers.PodVolumeRestoreLister
	secretLister           corev1listers.SecretLister
	podLister              corev1listers.PodLister
	pvcLister              corev1listers.PersistentVolumeClaimLister
	nodeName               string

	processRestoreFunc func(*arkv1api.PodVolumeRestore) error
}

// NewPodVolumeRestoreController creates a new pod volume restore controller.
func NewPodVolumeRestoreController(
	logger logrus.FieldLogger,
	podVolumeRestoreInformer informers.PodVolumeRestoreInformer,
	podVolumeRestoreClient arkv1client.PodVolumeRestoresGetter,
	podInformer cache.SharedIndexInformer,
	secretInformer corev1informers.SecretInformer,
	pvcInformer corev1informers.PersistentVolumeClaimInformer,
	nodeName string,
) Interface {
	c := &podVolumeRestoreController{
		genericController:      newGenericController("pod-volume-restore", logger),
		podVolumeRestoreClient: podVolumeRestoreClient,
		podVolumeRestoreLister: podVolumeRestoreInformer.Lister(),
		podLister:              corev1listers.NewPodLister(podInformer.GetIndexer()),
		secretLister:           secretInformer.Lister(),
		pvcLister:              pvcInformer.Lister(),
		nodeName:               nodeName,
	}

	c.syncHandler = c.processQueueItem
	c.cacheSyncWaiters = append(
		c.cacheSyncWaiters,
		podVolumeRestoreInformer.Informer().HasSynced,
		secretInformer.Informer().HasSynced,
		podInformer.HasSynced,
		pvcInformer.Informer().HasSynced,
	)
	c.processRestoreFunc = c.processRestore

	podVolumeRestoreInformer.Informer().AddEventHandler(
		cache.ResourceEventHandlerFuncs{
			AddFunc: c.pvrHandler,
			UpdateFunc: func(_, obj interface{}) {
				c.pvrHandler(obj)
			},
		},
	)

	podInformer.AddEventHandler(
		cache.ResourceEventHandlerFuncs{
			AddFunc: c.podHandler,
			UpdateFunc: func(_, obj interface{}) {
				c.podHandler(obj)
			},
		},
	)

	return c
}

func (c *podVolumeRestoreController) pvrHandler(obj interface{}) {
	pvr := obj.(*arkv1api.PodVolumeRestore)
	log := c.logger.WithField("key", kube.NamespaceAndName(pvr))

	if !shouldEnqueuePVR(pvr, c.podLister, c.nodeName, log) {
		return
	}

	log.Debug("enqueueing")
	c.enqueue(obj)
}

func (c *podVolumeRestoreController) podHandler(obj interface{}) {
	pod := obj.(*corev1api.Pod)
	log := c.logger.WithField("key", kube.NamespaceAndName(pod))

	for _, pvr := range pvrsToEnqueueForPod(pod, c.podVolumeRestoreLister, c.nodeName, log) {
		c.enqueue(pvr)
	}
}

func shouldProcessPod(pod *corev1api.Pod, nodeName string, log logrus.FieldLogger) bool {
	// if the pod lister being used is filtered to pods on this node, this is superfluous,
	// but retaining for safety.
	if pod.Spec.NodeName != nodeName {
		log.Debugf("Pod is scheduled on node %s, not enqueueing.", pod.Spec.NodeName)
		return false
	}

	// only process items for pods that have the restic initContainer running
	if !isPodWaiting(pod) {
		log.Debugf("Pod is not running restic initContainer, not enqueueing.")
		return false
	}

	return true
}

func shouldProcessPVR(pvr *arkv1api.PodVolumeRestore, log logrus.FieldLogger) bool {
	// only process new items
	if pvr.Status.Phase != "" && pvr.Status.Phase != arkv1api.PodVolumeRestorePhaseNew {
		log.Debugf("Item has phase %s, not enqueueing.", pvr.Status.Phase)
		return false
	}

	return true
}

func pvrsToEnqueueForPod(pod *corev1api.Pod, pvrLister listers.PodVolumeRestoreLister, nodeName string, log logrus.FieldLogger) []*arkv1api.PodVolumeRestore {
	if !shouldProcessPod(pod, nodeName, log) {
		return nil
	}

	selector, err := labels.Parse(fmt.Sprintf("%s=%s", arkv1api.PodUIDLabel, pod.UID))
	if err != nil {
		log.WithError(err).Error("Unable to parse label selector %s", fmt.Sprintf("%s=%s", arkv1api.PodUIDLabel, pod.UID))
		return nil
	}

	pvrs, err := pvrLister.List(selector)
	if err != nil {
		log.WithError(err).Error("Unable to list pod volume restores")
		return nil
	}

	var res []*arkv1api.PodVolumeRestore
	for i, pvr := range pvrs {
		if shouldProcessPVR(pvr, log) {
			res = append(res, pvrs[i])
		}
	}

	return res
}

func shouldEnqueuePVR(pvr *arkv1api.PodVolumeRestore, podLister corev1listers.PodLister, nodeName string, log logrus.FieldLogger) bool {
	if !shouldProcessPVR(pvr, log) {
		return false
	}

	pod, err := podLister.Pods(pvr.Spec.Pod.Namespace).Get(pvr.Spec.Pod.Name)
	if err != nil {
		log.WithError(err).Errorf("Unable to get item's pod %s/%s, not enqueueing.", pvr.Spec.Pod.Namespace, pvr.Spec.Pod.Name)
		return false
	}

	if !shouldProcessPod(pod, nodeName, log) {
		return false
	}

	return true
}

func isPodWaiting(pod *corev1api.Pod) bool {
	return len(pod.Spec.InitContainers) == 0 ||
		pod.Spec.InitContainers[0].Name != restic.InitContainer ||
		len(pod.Status.InitContainerStatuses) == 0 ||
		pod.Status.InitContainerStatuses[0].State.Running == nil
}

func (c *podVolumeRestoreController) processQueueItem(key string) error {
	log := c.logger.WithField("key", key)
	log.Debug("Running processItem")

	ns, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		log.WithError(errors.WithStack(err)).Error("error splitting queue key")
		return nil
	}

	req, err := c.podVolumeRestoreLister.PodVolumeRestores(ns).Get(name)
	if apierrors.IsNotFound(err) {
		log.Debug("Unable to find PodVolumeRestore")
		return nil
	}
	if err != nil {
		return errors.Wrap(err, "error getting PodVolumeRestore")
	}

	// Don't mutate the shared cache
	reqCopy := req.DeepCopy()
	return c.processRestoreFunc(reqCopy)
}

func (c *podVolumeRestoreController) processRestore(req *arkv1api.PodVolumeRestore) error {
	log := c.logger.WithFields(logrus.Fields{
		"namespace": req.Namespace,
		"name":      req.Name,
	})

	var err error

	// update status to InProgress
	req, err = c.patchPodVolumeRestore(req, updatePodVolumeRestorePhaseFunc(arkv1api.PodVolumeRestorePhaseInProgress))
	if err != nil {
		log.WithError(err).Error("Error setting phase to InProgress")
		return errors.WithStack(err)
	}

	pod, err := c.podLister.Pods(req.Spec.Pod.Namespace).Get(req.Spec.Pod.Name)
	if err != nil {
		log.WithError(err).Errorf("Error getting pod %s/%s", req.Spec.Pod.Namespace, req.Spec.Pod.Name)
		return c.failRestore(req, errors.Wrap(err, "error getting pod").Error(), log)
	}

	volumeDir, err := kube.GetVolumeDirectory(pod, req.Spec.Volume, c.pvcLister)
	if err != nil {
		log.WithError(err).Error("Error getting volume directory name")
		return c.failRestore(req, errors.Wrap(err, "error getting volume directory name").Error(), log)
	}

	credsFile, err := restic.TempCredentialsFile(c.secretLister, req.Spec.Pod.Namespace)
	if err != nil {
		log.WithError(err).Error("Error creating temp restic credentials file")
		return c.failRestore(req, errors.Wrap(err, "error creating temp restic credentials file").Error(), log)
	}
	// ignore error since there's nothing we can do and it's a temp file.
	defer os.Remove(credsFile)

	// execute the restore process
	if err := restorePodVolume(req, credsFile, volumeDir, log); err != nil {
		log.WithError(err).Error("Error restoring volume")
		return c.failRestore(req, errors.Wrap(err, "error restoring volume").Error(), log)
	}

	// update status to Completed
	if _, err = c.patchPodVolumeRestore(req, updatePodVolumeRestorePhaseFunc(arkv1api.PodVolumeRestorePhaseCompleted)); err != nil {
		log.WithError(err).Error("Error setting phase to Completed")
		return err
	}

	return nil
}

func restorePodVolume(req *arkv1api.PodVolumeRestore, credsFile, volumeDir string, log logrus.FieldLogger) error {
	resticCmd := restic.RestoreCommand(
		req.Spec.RepoPrefix,
		req.Spec.Pod.Namespace,
		credsFile,
		string(req.Spec.Pod.UID),
		req.Spec.SnapshotID,
	)

	var (
		stdout, stderr string
		err            error
	)

	// First restore the backed-up volume into a staging area, under /restores. This is necessary because restic backups
	// are stored with the absolute path of the backed-up directory, and restic doesn't allow you to adjust this path
	// when restoring, only to choose a different parent directory. So, for example, if you backup /foo/bar/volume, when
	// restoring, you can't restore to /baz/volume. You may restore to /baz/foo/bar/volume, though. The net result of
	// all this is that we can't restore directly into the new volume's directory, because the path is entirely different
	// than the backed-up one.
	if stdout, stderr, err = runCommand(resticCmd.Cmd()); err != nil {
		return errors.Wrapf(err, "error running restic restore, cmd=%s, stdout=%s, stderr=%s", resticCmd.String(), stdout, stderr)
	}
	log.Debugf("Ran command=%s, stdout=%s, stderr=%s", resticCmd.String(), stdout, stderr)

	// Now, get the full path of the restored volume in the staging directory, which will
	// look like:
	// 		/restores/<new-pod-uid>/host_pods/<backed-up-pod-uid>/volumes/<volume-plugin-name>/<volume-dir>
	restorePath, err := singlePathMatch(fmt.Sprintf("/restores/%s/host_pods/*/volumes/*/%s", string(req.Spec.Pod.UID), volumeDir))
	if err != nil {
		return errors.Wrap(err, "error identifying path of restore staging directory")
	}

	// Also get the full path of the new volume's directory (as mounted in the daemonset pod), which
	// will look like:
	// 		/host_pods/<new-pod-uid>/volumes/<volume-plugin-name>/<volume-dir>
	volumePath, err := singlePathMatch(fmt.Sprintf("/host_pods/%s/volumes/*/%s", string(req.Spec.Pod.UID), volumeDir))
	if err != nil {
		return errors.Wrap(err, "error identifying path of volume")
	}

	// Move the contents of the staging directory into the new volume directory to finalize the restore. This
	// is being executed through a shell because attempting to do the same thing in go (via os.Rename()) is
	// giving errors about renames not being allowed across filesystem layers in a container. This is occurring
	// whether /restores is part of the writeable container layer, or is an emptyDir volume mount. This may
	// be solvable but using the shell works so not investigating further.
	if _, stderr, err := runCommand(exec.Command("/bin/sh", "-c", fmt.Sprintf("mv %s/* %s/", restorePath, volumePath))); err != nil {
		return errors.Wrapf(err, "error moving contents of restore staging directory into volume, stderr=%s", stderr)
	}

	// The staging directory should be empty at this point since we moved everything, but
	// make sure.
	if err := os.RemoveAll(restorePath); err != nil {
		return errors.Wrap(err, "error removing restore staging directory and its contents")
	}

	var restoreUID types.UID
	for _, owner := range req.OwnerReferences {
		if boolptr.IsSetToTrue(owner.Controller) {
			restoreUID = owner.UID
			break
		}
	}

	// Create the .ark directory within the volume dir so we can write a done file
	// for this restore.
	if err := os.MkdirAll(filepath.Join(volumePath, ".ark"), 0755); err != nil {
		return errors.Wrap(err, "error creating .ark directory for done file")
	}

	// TODO remove any done files from previous ark restores from .ark

	// Write a done file with name=<restore-uid> into the just-created .ark dir
	// within the volume. The ark restic init container on the pod is waiting
	// for this file to exist in each restored volume before completing.
	if err := ioutil.WriteFile(filepath.Join(volumePath, ".ark", string(restoreUID)), nil, 0644); err != nil {
		return errors.Wrap(err, "error writing done file")
	}

	return nil
}

func (c *podVolumeRestoreController) patchPodVolumeRestore(req *arkv1api.PodVolumeRestore, mutate func(*arkv1api.PodVolumeRestore)) (*arkv1api.PodVolumeRestore, error) {
	// Record original json
	oldData, err := json.Marshal(req)
	if err != nil {
		return nil, errors.Wrap(err, "error marshalling original PodVolumeRestore")
	}

	// Mutate
	mutate(req)

	// Record new json
	newData, err := json.Marshal(req)
	if err != nil {
		return nil, errors.Wrap(err, "error marshalling updated PodVolumeRestore")
	}

	patchBytes, err := jsonpatch.CreateMergePatch(oldData, newData)
	if err != nil {
		return nil, errors.Wrap(err, "error creating json merge patch for PodVolumeRestore")
	}

	req, err = c.podVolumeRestoreClient.PodVolumeRestores(req.Namespace).Patch(req.Name, types.MergePatchType, patchBytes)
	if err != nil {
		return nil, errors.Wrap(err, "error patching PodVolumeRestore")
	}

	return req, nil
}

func (c *podVolumeRestoreController) failRestore(req *arkv1api.PodVolumeRestore, msg string, log logrus.FieldLogger) error {
	if _, err := c.patchPodVolumeRestore(req, func(pvr *arkv1api.PodVolumeRestore) {
		pvr.Status.Phase = arkv1api.PodVolumeRestorePhaseFailed
		pvr.Status.Message = msg
	}); err != nil {
		log.WithError(err).Error("Error setting phase to Failed")
		return err
	}
	return nil
}

func updatePodVolumeRestorePhaseFunc(phase arkv1api.PodVolumeRestorePhase) func(r *arkv1api.PodVolumeRestore) {
	return func(r *arkv1api.PodVolumeRestore) {
		r.Status.Phase = phase
	}
}
