/*
Copyright 2017 the Velero contributors.

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
	"strings"

	jsonpatch "github.com/evanphx/json-patch/v5"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	"github.com/vmware-tanzu/velero/pkg/cmd/cli/serverstatus"
	"github.com/vmware-tanzu/velero/pkg/client"
	"github.com/vmware-tanzu/velero/pkg/cmd"
)

func NewRemoveCommand(f client.Factory) *cobra.Command {
	c := &cobra.Command{
		Use:   "remove [NAME | IMAGE]",
		Short: "Remove a plugin",
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

			// If the argument looks like a plugin name (contains '/'), try to resolve
			// it via the server status. If the plugin is built-in, refuse removal.
			arg := args[0]

			if strings.Contains(arg, "/") {
				// retrieve server status
				serverStatusGetter := &serverstatus.DefaultServerStatusGetter{
					Namespace: f.Namespace(),
					Context:   context.TODO(),
				}

				kbClient, err := f.KubebuilderClient()
				if err == nil {
					serverStatus, serr := serverStatusGetter.GetServerStatus(kbClient)
					if serr == nil {
						// find plugin
						for _, p := range serverStatus.Status.Plugins {
							if p.Name == arg {
								if p.BuiltIn {
									cmd.CheckError(errors.Errorf("plugin %s is built-in and cannot be removed", arg))
								}
								// plugin exists and is not built-in; try to map to an init container
								// using heuristics below
								break
							}
						}
					}
				}

			}

			var (
				initContainers = veleroDeploy.Spec.Template.Spec.InitContainers
				index          = -1
			)

			// Find matching init container. Accept exact name or image match. If the
			// arg was a plugin name, try some heuristics to match the init container.
			for x, container := range initContainers {
				if container.Name == arg || container.Image == arg {
					index = x
					break
				}
			}

			if index == -1 && strings.Contains(arg, "/") {
				// heuristics: sanitized plugin name may match container name
				sanitized := strings.NewReplacer("/", "-", "_", "-", ".", "-").Replace(arg)
				var candidates []int
				last := arg[strings.LastIndex(arg, "/")+1:]
				for x, container := range initContainers {
					if container.Name == sanitized || strings.Contains(container.Image, last) || strings.Contains(container.Name, last) {
						candidates = append(candidates, x)
					}
				}
				if len(candidates) == 1 {
					index = candidates[0]
				} else if len(candidates) > 1 {
					names := []string{}
					for _, i := range candidates {
						names = append(names, initContainers[i].Name+"("+initContainers[i].Image+")")
					}
					cmd.CheckError(errors.Errorf("multiple init containers match plugin %s: %s; please specify image or init container name", arg, strings.Join(names, ", ")))
				}
			}

			if index == -1 {
				cmd.CheckError(errors.Errorf("init container %s not found in Velero server deployment", arg))
			}

			veleroDeploy.Spec.Template.Spec.InitContainers = append(initContainers[0:index], initContainers[index+1:]...)

			updated, err := json.Marshal(veleroDeploy)
			cmd.CheckError(err)

			patchBytes, err := jsonpatch.CreateMergePatch(original, updated)
			cmd.CheckError(err)

			_, err = kubeClient.AppsV1().Deployments(veleroDeploy.Namespace).Patch(context.TODO(), veleroDeploy.Name, types.MergePatchType, patchBytes, metav1.PatchOptions{})
			cmd.CheckError(err)
		},
	}

	return c
}
