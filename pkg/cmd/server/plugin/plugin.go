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
	"fmt"
	"strings"

	plugin "github.com/hashicorp/go-plugin"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/heptio/ark/pkg/backup"
	"github.com/heptio/ark/pkg/client"
	"github.com/heptio/ark/pkg/cloudprovider"
	"github.com/heptio/ark/pkg/cloudprovider/aws"
	"github.com/heptio/ark/pkg/cloudprovider/azure"
	"github.com/heptio/ark/pkg/cloudprovider/gcp"
	"github.com/heptio/ark/pkg/cmd"
	arkplugin "github.com/heptio/ark/pkg/plugin"
	"github.com/heptio/ark/pkg/restore"
	"github.com/heptio/ark/pkg/util/logging"
)

func NewCommand(f client.Factory) *cobra.Command {
	logLevelFlag := logging.LogLevelFlag(logrus.InfoLevel)

	c := &cobra.Command{
		Use:    "run-plugin [KIND] [NAME]",
		Hidden: true,
		Short:  "INTERNAL COMMAND ONLY - not intended to be run directly by users",
		Args:   cobra.ExactArgs(2),
		Run: func(c *cobra.Command, args []string) {
			logLevel := logLevelFlag.Parse()
			logger := arkplugin.NewLogger(logLevel)

			kind := args[0]
			name := args[1]

			logger = logger.WithFields(logrus.Fields{"kind": kind, "name": name})

			serveConfig := &plugin.ServeConfig{
				HandshakeConfig: arkplugin.Handshake,
				GRPCServer:      plugin.DefaultGRPCServer,
			}

			logger.Debug("Executing run-plugin command")

			switch kind {
			case "cloudprovider":
				var (
					objectStore cloudprovider.ObjectStore
					blockStore  cloudprovider.BlockStore
				)

				switch name {
				case "aws":
					objectStore, blockStore = aws.NewObjectStore(), aws.NewBlockStore()
				case "azure":
					objectStore, blockStore = azure.NewObjectStore(), azure.NewBlockStore()
				case "gcp":
					objectStore, blockStore = gcp.NewObjectStore(), gcp.NewBlockStore(logger)
				default:
					logger.Fatal("Unrecognized plugin name")
				}

				serveConfig.Plugins = map[string]plugin.Plugin{
					string(arkplugin.PluginKindObjectStore): arkplugin.NewObjectStorePlugin(objectStore),
					string(arkplugin.PluginKindBlockStore):  arkplugin.NewBlockStorePlugin(blockStore),
				}
			case arkplugin.PluginKindBackupItemAction.String():
				var action backup.ItemAction

				switch name {
				case "pv":
					action = backup.NewBackupPVAction(logger)
				case "pod":
					action = backup.NewPodAction(logger)
				case "serviceaccount":
					clientset, err := f.KubeClient()
					cmd.CheckError(err)

					action, err = backup.NewServiceAccountAction(logger, clientset.RbacV1().ClusterRoleBindings())
					cmd.CheckError(err)
				default:
					logger.Fatal("Unrecognized plugin name")
				}

				serveConfig.Plugins = map[string]plugin.Plugin{
					kind: arkplugin.NewBackupItemActionPlugin(action),
				}
			case arkplugin.PluginKindRestoreItemAction.String():
				var action restore.ItemAction

				switch name {
				case "job":
					action = restore.NewJobAction(logger)
				case "pod":
					action = restore.NewPodAction(logger)
				case "svc":
					action = restore.NewServiceAction(logger)
				case "restic":
					action = restore.NewResticRestoreAction(logger)
				default:
					logger.Fatal("Unrecognized plugin name")
				}

				serveConfig.Plugins = map[string]plugin.Plugin{
					kind: arkplugin.NewRestoreItemActionPlugin(action),
				}
			default:
				logger.Fatal("Unsupported plugin kind")
			}

			plugin.Serve(serveConfig)
		},
	}

	c.Flags().Var(logLevelFlag, "log-level", fmt.Sprintf("the level at which to log. Valid values are %s.", strings.Join(logLevelFlag.AllowedValues(), ", ")))

	return c
}
