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

package actions

import (
	"context"
	"fmt"
	"strings"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	corev1client "k8s.io/client-go/kubernetes/typed/core/v1"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"

	veleroimage "github.com/vmware-tanzu/velero/internal/velero"
	velerov1api "github.com/vmware-tanzu/velero/pkg/apis/velero/v1"
	"github.com/vmware-tanzu/velero/pkg/builder"
	"github.com/vmware-tanzu/velero/pkg/label"
	"github.com/vmware-tanzu/velero/pkg/plugin/framework/common"
	"github.com/vmware-tanzu/velero/pkg/plugin/velero"
	"github.com/vmware-tanzu/velero/pkg/podvolume"
	"github.com/vmware-tanzu/velero/pkg/restorehelper"
	"github.com/vmware-tanzu/velero/pkg/util/kube"
)

const (
	defaultCPURequestLimit = "100m"
	defaultMemRequestLimit = "128Mi"
	defaultCommand         = "/velero-restore-helper"
)

type PodVolumeRestoreAction struct {
	logger   logrus.FieldLogger
	client   corev1client.ConfigMapInterface
	crClient ctrlclient.Client
}

func NewPodVolumeRestoreAction(logger logrus.FieldLogger, client corev1client.ConfigMapInterface, crClient ctrlclient.Client) *PodVolumeRestoreAction {
	return &PodVolumeRestoreAction{
		logger:   logger,
		client:   client,
		crClient: crClient,
	}
}

func (a *PodVolumeRestoreAction) AppliesTo() (velero.ResourceSelector, error) {
	return velero.ResourceSelector{
		IncludedResources: []string{"pods"},
	}, nil
}

func (a *PodVolumeRestoreAction) Execute(input *velero.RestoreItemActionExecuteInput) (*velero.RestoreItemActionExecuteOutput, error) {
	a.logger.Info("Executing PodVolumeRestoreAction")
	defer a.logger.Info("Done executing PodVolumeRestoreAction")

	var pod corev1.Pod
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(input.Item.UnstructuredContent(), &pod); err != nil {
		return nil, errors.Wrap(err, "unable to convert pod from runtime.Unstructured")
	}

	// At the point when this function is called, the namespace mapping for the restore
	// has not yet been applied to `input.Item` so we can't perform a reverse-lookup in
	// the namespace mapping in the restore spec. Instead, use the pod from the backup
	// so that if the mapping is applied earlier, we still use the correct namespace.
	var podFromBackup corev1.Pod
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(input.ItemFromBackup.UnstructuredContent(), &podFromBackup); err != nil {
		return nil, errors.Wrap(err, "unable to convert source pod from runtime.Unstructured")
	}

	log := a.logger.WithField("pod", kube.NamespaceAndName(&pod))

	opts := &ctrlclient.ListOptions{
		LabelSelector: label.NewSelectorForBackup(input.Restore.Spec.BackupName),
	}
	podVolumeBackupList := new(velerov1api.PodVolumeBackupList)
	if err := a.crClient.List(context.TODO(), podVolumeBackupList, opts); err != nil {
		return nil, errors.WithStack(err)
	}

	var podVolumeBackups []*velerov1api.PodVolumeBackup
	for i := range podVolumeBackupList.Items {
		podVolumeBackups = append(podVolumeBackups, &podVolumeBackupList.Items[i])
	}
	volumeSnapshots := podvolume.GetVolumeBackupsForPod(podVolumeBackups, &pod, podFromBackup.Namespace)
	if len(volumeSnapshots) == 0 {
		log.Debug("No pod volume backups found for pod")
		return velero.NewRestoreItemActionExecuteOutput(input.Item), nil
	}

	log.Info("Pod volume backups for pod found")

	// TODO we might want/need to get plugin config at the top of this method at some point; for now, wait
	// until we know we're doing a restore before getting config.
	log.Debugf("Getting plugin config")
	config, err := common.GetPluginConfig(common.PluginKindRestoreItemAction, "velero.io/pod-volume-restore", a.client)
	if err != nil {
		return nil, err
	}

	image := getImage(log, config)
	log.Infof("Using image %q", image)

	cpuRequest, memRequest := getResourceRequests(log, config)
	cpuLimit, memLimit := getResourceLimits(log, config)
	if cpuRequest == "" {
		cpuRequest = defaultCPURequestLimit
	}
	if cpuLimit == "" {
		cpuLimit = defaultCPURequestLimit
	}
	if memRequest == "" {
		memRequest = defaultMemRequestLimit
	}
	if memLimit == "" {
		memLimit = defaultMemRequestLimit
	}

	resourceReqs, err := kube.ParseResourceRequirements(cpuRequest, memRequest, cpuLimit, memLimit)
	if err != nil {
		log.Errorf("Using default resource values, couldn't parse resource requirements: %s.", err)
		resourceReqs, _ = kube.ParseResourceRequirements(
			defaultCPURequestLimit, defaultMemRequestLimit, // requests
			defaultCPURequestLimit, defaultMemRequestLimit, // limits
		)
	}

	runAsUser, runAsGroup, allowPrivilegeEscalation, secCtx := getSecurityContext(log, config)

	securityContext, err := kube.ParseSecurityContext(runAsUser, runAsGroup, allowPrivilegeEscalation, secCtx)
	if err != nil {
		log.Errorf("Using default securityContext values, couldn't parse securityContext requirements: %s.", err)
	}

	initContainerBuilder := newRestoreInitContainerBuilder(image, string(input.Restore.UID))
	initContainerBuilder.Resources(&resourceReqs)
	initContainerBuilder.SecurityContext(&securityContext)

	for volumeName := range volumeSnapshots {
		mount := &corev1.VolumeMount{
			Name:      volumeName,
			MountPath: "/restores/" + volumeName,
		}
		initContainerBuilder.VolumeMounts(mount)
	}
	initContainerBuilder.Command(getCommand(log, config))

	initContainer := *initContainerBuilder.Result()
	if len(pod.Spec.InitContainers) == 0 || (pod.Spec.InitContainers[0].Name != restorehelper.WaitInitContainer && pod.Spec.InitContainers[0].Name != restorehelper.WaitInitContainerLegacy) {
		pod.Spec.InitContainers = append([]corev1.Container{initContainer}, pod.Spec.InitContainers...)
	} else {
		pod.Spec.InitContainers[0] = initContainer
	}

	res, err := runtime.DefaultUnstructuredConverter.ToUnstructured(&pod)
	if err != nil {
		return nil, errors.Wrap(err, "unable to convert pod to runtime.Unstructured")
	}

	return velero.NewRestoreItemActionExecuteOutput(&unstructured.Unstructured{Object: res}), nil
}

func getCommand(log logrus.FieldLogger, config *corev1.ConfigMap) []string {
	if config == nil {
		log.Debug("No config found for plugin")
		return []string{defaultCommand}
	}

	if config.Data["command"] == "" {
		log.Debugf("No custom command configured")
		return []string{defaultCommand}
	}

	log.Debugf("Using custom command %s", config.Data["command"])
	return []string{config.Data["command"]}
}

func getImage(log logrus.FieldLogger, config *corev1.ConfigMap) string {
	if config == nil {
		log.Debug("No config found for plugin")
		return veleroimage.DefaultRestoreHelperImage()
	}

	image := config.Data["image"]
	if image == "" {
		log.Debugf("No custom image configured")
		return veleroimage.DefaultRestoreHelperImage()
	}

	log = log.WithField("image", image)

	parts := strings.Split(image, "/")

	if len(parts) == 1 {
		defaultImage := veleroimage.DefaultRestoreHelperImage()
		// Image supplied without registry part
		log.Infof("Plugin config contains image name without registry name. Using default init container image: %q", defaultImage)
		return defaultImage
	}

	if !(strings.Contains(parts[len(parts)-1], ":")) {
		tag := veleroimage.ImageTag()
		// tag-less image name: add default image tag for this version of Velero
		log.Infof("Plugin config contains image name without tag. Adding tag: %q", tag)
		return fmt.Sprintf("%s:%s", image, tag)
	}
	// tagged image name
	log.Debugf("Plugin config contains image name with tag")
	return image
}

// getResourceRequests extracts the CPU and memory requests from a ConfigMap.
// The 0 values are valid if the keys are not present
func getResourceRequests(log logrus.FieldLogger, config *corev1.ConfigMap) (string, string) {
	if config == nil {
		log.Debug("No config found for plugin")
		return "", ""
	}

	return config.Data["cpuRequest"], config.Data["memRequest"]
}

// getResourceLimits extracts the CPU and memory limits from a ConfigMap.
// The 0 values are valid if the keys are not present
func getResourceLimits(log logrus.FieldLogger, config *corev1.ConfigMap) (string, string) {
	if config == nil {
		log.Debug("No config found for plugin")
		return "", ""
	}

	return config.Data["cpuLimit"], config.Data["memLimit"]
}

// getSecurityContext extracts securityContext runAsUser, runAsGroup, allowPrivilegeEscalation, and securityContext from a ConfigMap.
func getSecurityContext(log logrus.FieldLogger, config *corev1.ConfigMap) (string, string, string, string) {
	if config == nil {
		log.Debug("No config found for plugin")
		return "", "", "", ""
	}

	return config.Data["secCtxRunAsUser"],
		config.Data["secCtxRunAsGroup"],
		config.Data["secCtxAllowPrivilegeEscalation"],
		config.Data["secCtx"]
}

func newRestoreInitContainerBuilder(image, restoreUID string) *builder.ContainerBuilder {
	return builder.ForContainer(restorehelper.WaitInitContainer, image).
		Args(restoreUID).
		Env([]*corev1.EnvVar{
			{
				Name: "POD_NAMESPACE",
				ValueFrom: &corev1.EnvVarSource{
					FieldRef: &corev1.ObjectFieldSelector{
						FieldPath: "metadata.namespace",
					},
				},
			},
			{
				Name: "POD_NAME",
				ValueFrom: &corev1.EnvVarSource{
					FieldRef: &corev1.ObjectFieldSelector{
						FieldPath: "metadata.name",
					},
				},
			},
		}...)
}
