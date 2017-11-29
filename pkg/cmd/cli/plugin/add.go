/*
Copyright 2017 the Heptio Ark contributors.

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
	"encoding/json"
	"fmt"
	"strings"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"

	"k8s.io/api/apps/v1beta1"
	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/strategicpatch"

	ark "github.com/heptio/ark/pkg/apis/ark/v1"
	"github.com/heptio/ark/pkg/client"
	"github.com/heptio/ark/pkg/cmd"
	"github.com/heptio/ark/pkg/cmd/util/flag"
)

const (
	pluginsVolumeName = "plugins"
	arkDeployment     = "ark"
	arkContainer      = "ark"
)

func NewAddCommand(f client.Factory) *cobra.Command {
	var (
		imagePullPolicies   = []string{string(v1.PullAlways), string(v1.PullIfNotPresent), string(v1.PullNever)}
		imagePullPolicyFlag = flag.NewEnum(string(v1.PullIfNotPresent), imagePullPolicies...)
	)

	c := &cobra.Command{
		Use:   "add IMAGE",
		Short: "Add a plugin",
		Run: func(c *cobra.Command, args []string) {
			if len(args) != 1 {
				cmd.CheckError(errors.New("you must specify only one argument, the plugin container image"))
			}

			kubeClient, err := f.KubeClient()
			if err != nil {
				cmd.CheckError(err)
			}

			arkDeploy, err := kubeClient.AppsV1beta1().Deployments(ark.DefaultNamespace).Get(arkDeployment, metav1.GetOptions{})
			if err != nil {
				cmd.CheckError(err)
			}

			original, err := json.Marshal(arkDeploy)
			cmd.CheckError(err)

			// ensure the plugins volume & mount exist
			volumeExists := false
			for _, volume := range arkDeploy.Spec.Template.Spec.Volumes {
				if volume.Name == pluginsVolumeName {
					volumeExists = true
					break
				}
			}

			if !volumeExists {
				volume := v1.Volume{
					Name: pluginsVolumeName,
					VolumeSource: v1.VolumeSource{
						EmptyDir: &v1.EmptyDirVolumeSource{},
					},
				}

				volumeMount := v1.VolumeMount{
					Name:      pluginsVolumeName,
					MountPath: "/plugins",
				}

				arkDeploy.Spec.Template.Spec.Volumes = append(arkDeploy.Spec.Template.Spec.Volumes, volume)

				containers := arkDeploy.Spec.Template.Spec.Containers
				containerIndex := -1
				for x, container := range containers {
					if container.Name == arkContainer {
						containerIndex = x
						break
					}
				}

				if containerIndex < 0 {
					cmd.CheckError(errors.New("ark container not found in ark deployment"))
				}

				containers[containerIndex].VolumeMounts = append(containers[containerIndex].VolumeMounts, volumeMount)
			}

			// add the plugin as an init container
			plugin := v1.Container{
				Name:            getName(args[0]),
				Image:           args[0],
				ImagePullPolicy: v1.PullPolicy(imagePullPolicyFlag.String()),
				VolumeMounts: []v1.VolumeMount{
					{
						Name:      pluginsVolumeName,
						MountPath: "/target",
					},
				},
			}

			arkDeploy.Spec.Template.Spec.InitContainers = append(arkDeploy.Spec.Template.Spec.InitContainers, plugin)

			// create & apply the patch
			updated, err := json.Marshal(arkDeploy)
			cmd.CheckError(err)

			patchBytes, err := strategicpatch.CreateTwoWayMergePatch(original, updated, v1beta1.Deployment{})
			cmd.CheckError(err)

			_, err = kubeClient.AppsV1beta1().Deployments(ark.DefaultNamespace).Patch(arkDeploy.Name, types.StrategicMergePatchType, patchBytes)
			cmd.CheckError(err)
		},
	}

	c.Flags().Var(imagePullPolicyFlag, "image-pull-policy", fmt.Sprintf("the imagePullPolicy for the plugin container. Valid values are %s.", strings.Join(imagePullPolicies, ", ")))

	return c
}

// getName returns the 'name' component of a docker
// image (i.e. everything after the last '/' and before
// any subsequent ':')
func getName(image string) string {
	slashIndex := strings.LastIndex(image, "/")
	colonIndex := strings.LastIndex(image, ":")

	start := 0
	if slashIndex > 0 {
		start = slashIndex + 1
	}

	end := len(image)
	if colonIndex > slashIndex {
		end = colonIndex
	}

	return image[start:end]
}
