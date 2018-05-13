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
	"github.com/spf13/cobra"

	"github.com/heptio/ark/pkg/backup"
	"github.com/heptio/ark/pkg/client"
	"github.com/heptio/ark/pkg/cloudprovider"
	"github.com/heptio/ark/pkg/cloudprovider/aws"
	"github.com/heptio/ark/pkg/cloudprovider/azure"
	"github.com/heptio/ark/pkg/cloudprovider/gcp"
	arkplugin "github.com/heptio/ark/pkg/plugin"
	"github.com/heptio/ark/pkg/restore"
)

func NewCommand(f client.Factory) *cobra.Command {
	logger := arkplugin.NewLogger()

	c := &cobra.Command{
		Use:    "run-plugin",
		Hidden: true,
		Short:  "INTERNAL COMMAND ONLY - not intended to be run directly by users",
		Run: func(c *cobra.Command, args []string) {
			logger.Debug("Executing run-plugin command")

			arkplugin.NewSimpleServer().
				RegisterObjectStores(map[string]func() cloudprovider.ObjectStore{
					"aws": func() cloudprovider.ObjectStore {
						return aws.NewObjectStore(logger)
					},
					"azure": azure.NewObjectStore,
					"gcp": func() cloudprovider.ObjectStore {
						return gcp.NewObjectStore(logger)
					},
				}).
				RegisterBlockStores(map[string]func() cloudprovider.BlockStore{
					"aws":   aws.NewBlockStore,
					"azure": azure.NewBlockStore,
					"gcp": func() cloudprovider.BlockStore {
						return gcp.NewBlockStore(logger)
					},
				}).
				RegisterBackupItemActions(map[string]func() backup.ItemAction{
					"pv": func() backup.ItemAction {
						return backup.NewBackupPVAction(logger)
					},
					"pod": func() backup.ItemAction {
						return backup.NewPodAction(logger)
					},
					"serviceaccount": func() backup.ItemAction {
						clientset, err := f.KubeClient()
						if err != nil {
							panic(err)
						}

						action, err := backup.NewServiceAccountAction(logger, clientset.RbacV1().ClusterRoleBindings())
						if err != nil {
							panic(err)
						}

						return action
					},
				}).
				RegisterRestoreItemActions(map[string]func() restore.ItemAction{
					"job": func() restore.ItemAction {
						return restore.NewJobAction(logger)
					},
					"pod": func() restore.ItemAction {
						return restore.NewPodAction(logger)
					},
					"service": func() restore.ItemAction {
						return restore.NewServiceAction(logger)
					},
				}).
				Serve()
		},
	}

	return c
}
