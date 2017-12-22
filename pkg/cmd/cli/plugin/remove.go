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

	"github.com/pkg/errors"
	"github.com/spf13/cobra"

	"k8s.io/api/apps/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/strategicpatch"

	"github.com/heptio/ark/pkg/client"
	"github.com/heptio/ark/pkg/cmd"
)

func NewRemoveCommand(f client.Factory) *cobra.Command {
	c := &cobra.Command{
		Use:   "remove [NAME | IMAGE]",
		Short: "Remove a plugin",
		Run: func(c *cobra.Command, args []string) {
			if len(args) != 1 {
				cmd.CheckError(errors.New("you must specify only one argument, the plugin container's name or image"))
			}

			kubeClient, err := f.KubeClient()
			if err != nil {
				cmd.CheckError(err)
			}

			arkDeploy, err := kubeClient.AppsV1beta1().Deployments(f.Namespace()).Get(arkDeployment, metav1.GetOptions{})
			if err != nil {
				cmd.CheckError(err)
			}

			original, err := json.Marshal(arkDeploy)
			cmd.CheckError(err)

			var (
				initContainers = arkDeploy.Spec.Template.Spec.InitContainers
				index          = -1
			)

			for x, container := range initContainers {
				if container.Name == args[0] || container.Image == args[0] {
					index = x
					break
				}
			}

			if index == -1 {
				cmd.CheckError(errors.Errorf("init container %s not found in Ark server deployment", args[0]))
			}

			arkDeploy.Spec.Template.Spec.InitContainers = append(initContainers[0:index], initContainers[index+1:]...)

			updated, err := json.Marshal(arkDeploy)
			cmd.CheckError(err)

			patchBytes, err := strategicpatch.CreateTwoWayMergePatch(original, updated, v1beta1.Deployment{})
			cmd.CheckError(err)

			_, err = kubeClient.AppsV1beta1().Deployments(arkDeploy.Namespace).Patch(arkDeploy.Name, types.StrategicMergePatchType, patchBytes)
			cmd.CheckError(err)
		},
	}

	return c
}
