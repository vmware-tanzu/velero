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

	jsonpatch "github.com/evanphx/json-patch"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	"github.com/heptio/velero/pkg/client"
	"github.com/heptio/velero/pkg/cmd"
	"github.com/heptio/velero/pkg/cmd/util/flag"
)

const (
	pluginsVolumeName = "plugins"
	veleroDeployment  = "velero"
	veleroContainer   = "velero"
)

func NewAddCommand(f client.Factory) *cobra.Command {
	var (
		imagePullPolicies   = []string{string(v1.PullAlways), string(v1.PullIfNotPresent), string(v1.PullNever)}
		imagePullPolicyFlag = flag.NewEnum(string(v1.PullIfNotPresent), imagePullPolicies...)
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

			veleroDeploy, err := kubeClient.AppsV1beta1().Deployments(f.Namespace()).Get(veleroDeployment, metav1.GetOptions{})
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

			veleroDeploy.Spec.Template.Spec.InitContainers = append(veleroDeploy.Spec.Template.Spec.InitContainers, plugin)

			// create & apply the patch
			updated, err := json.Marshal(veleroDeploy)
			cmd.CheckError(err)

			patchBytes, err := jsonpatch.CreateMergePatch(original, updated)
			cmd.CheckError(err)

			_, err = kubeClient.AppsV1beta1().Deployments(veleroDeploy.Namespace).Patch(veleroDeploy.Name, types.MergePatchType, patchBytes)
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
