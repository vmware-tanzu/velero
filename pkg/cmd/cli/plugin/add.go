/*
Copyright 2020 the Velero contributors.

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

package plugin

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	jsonpatch "github.com/evanphx/json-patch"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	corev1api "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	"github.com/vmware-tanzu/velero/pkg/builder"
	"github.com/vmware-tanzu/velero/pkg/client"
	"github.com/vmware-tanzu/velero/pkg/cmd"
	"github.com/vmware-tanzu/velero/pkg/cmd/util/flag"
)

const (
	pluginsVolumeName = "plugins"
	veleroContainer   = "velero"
)

func NewAddCommand(f client.Factory) *cobra.Command {
	var (
		imagePullPolicies   = []string{string(corev1api.PullAlways), string(corev1api.PullIfNotPresent), string(corev1api.PullNever)}
		imagePullPolicyFlag = flag.NewEnum(string(corev1api.PullIfNotPresent), imagePullPolicies...)
	)

	c := &cobra.Command{
		Use:   "add IMAGE",
		Short: "Add a plugin",
		Args:  cobra.ExactArgs(1),
		Run: func(c *cobra.Command, args []string) {
			kubeClient, err := f.KubeClient()
			if err != nil {
				cmd.CheckError(err)
			}

			veleroDeploy, err := veleroDeployment(context.TODO(), kubeClient, f.Namespace())
			if err != nil {
				cmd.CheckError(err)
			}

			original, err := json.Marshal(veleroDeploy)
			cmd.CheckError(err)

			// ensure the plugins volume & mount exist
			volumeExists := false
			for _, volume := range veleroDeploy.Spec.Template.Spec.Volumes {
				if volume.Name == pluginsVolumeName {
					volumeExists = true
					break
				}
			}

			if !volumeExists {
				volume := corev1api.Volume{
					Name: pluginsVolumeName,
					VolumeSource: corev1api.VolumeSource{
						EmptyDir: &corev1api.EmptyDirVolumeSource{},
					},
				}

				volumeMount := corev1api.VolumeMount{
					Name:      pluginsVolumeName,
					MountPath: "/plugins",
				}

				veleroDeploy.Spec.Template.Spec.Volumes = append(veleroDeploy.Spec.Template.Spec.Volumes, volume)

				containers := veleroDeploy.Spec.Template.Spec.Containers
				containerIndex := -1
				for x, container := range containers {
					if container.Name == veleroContainer {
						containerIndex = x
						break
					}
				}

				if containerIndex < 0 {
					cmd.CheckError(errors.New("velero container not found in velero deployment"))
				}

				containers[containerIndex].VolumeMounts = append(containers[containerIndex].VolumeMounts, volumeMount)
			}

			// add the plugin as an init container
			plugin := *builder.ForPluginContainer(args[0], corev1api.PullPolicy(imagePullPolicyFlag.String())).Result()

			veleroDeploy.Spec.Template.Spec.InitContainers = append(veleroDeploy.Spec.Template.Spec.InitContainers, plugin)

			// create & apply the patch
			updated, err := json.Marshal(veleroDeploy)
			cmd.CheckError(err)

			patchBytes, err := jsonpatch.CreateMergePatch(original, updated)
			cmd.CheckError(err)

			_, err = kubeClient.AppsV1().Deployments(veleroDeploy.Namespace).Patch(context.TODO(), veleroDeploy.Name, types.MergePatchType, patchBytes, metav1.PatchOptions{})
			cmd.CheckError(err)
		},
	}

	c.Flags().Var(imagePullPolicyFlag, "image-pull-policy", fmt.Sprintf("The imagePullPolicy for the plugin container. Valid values are %s.", strings.Join(imagePullPolicies, ", ")))

	return c
}
