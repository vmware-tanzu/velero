/*
Copyright 2017 the Heptio Ark Contributors.

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
	plugin "github.com/hashicorp/go-plugin"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/heptio/ark/pkg/backup"
	"github.com/heptio/ark/pkg/cloudprovider"
	"github.com/heptio/ark/pkg/cloudprovider/aws"
	"github.com/heptio/ark/pkg/cloudprovider/azure"
	"github.com/heptio/ark/pkg/cloudprovider/gcp"
	arkplugin "github.com/heptio/ark/pkg/plugin"
	"github.com/heptio/ark/pkg/restore"
)

func NewCommand() *cobra.Command {
	logger := arkplugin.NewLogger()

	objectStores := map[string]cloudprovider.ObjectStore{
		"aws":   aws.NewObjectStore(),
		"gcp":   gcp.NewObjectStore(),
		"azure": azure.NewObjectStore(),
	}

	blockStores := map[string]cloudprovider.BlockStore{
		"aws":   aws.NewBlockStore(),
		"gcp":   gcp.NewBlockStore(),
		"azure": azure.NewBlockStore(),
	}

	backupItemActions := map[string]backup.ItemAction{
		"pv": backup.NewBackupPVAction(logger),
	}

	restoreItemActions := map[string]restore.ItemAction{
		"job": restore.NewJobAction(logger),
		"pod": restore.NewPodAction(logger),
		"svc": restore.NewServiceAction(logger),
	}

	c := &cobra.Command{
		Use:    "run-plugin [KIND] [NAME]",
		Hidden: true,
		Short:  "INTERNAL COMMAND ONLY - not intended to be run directly by users",
		Run: func(c *cobra.Command, args []string) {
			if len(args) != 2 {
				logger.Fatal("You must specify exactly two arguments, the plugin kind and the plugin name")
			}

			kind := args[0]
			name := args[1]

			logger = logger.WithFields(logrus.Fields{"kind": kind, "name": name})

			serveConfig := &plugin.ServeConfig{
				HandshakeConfig: arkplugin.Handshake,
				GRPCServer:      plugin.DefaultGRPCServer,
			}

			logger.Debugf("Executing run-plugin command")

			switch kind {
			case "cloudprovider":
				objectStore, found := objectStores[name]
				if !found {
					logger.Fatalf("Unrecognized plugin name")
				}

				blockStore, found := blockStores[name]
				if !found {
					logger.Fatalf("Unrecognized plugin name")
				}

				serveConfig.Plugins = map[string]plugin.Plugin{
					string(arkplugin.PluginKindObjectStore): arkplugin.NewObjectStorePlugin(objectStore),
					string(arkplugin.PluginKindBlockStore):  arkplugin.NewBlockStorePlugin(blockStore),
				}
			case arkplugin.PluginKindBackupItemAction.String():
				action, found := backupItemActions[name]
				if !found {
					logger.Fatalf("Unrecognized plugin name")
				}

				serveConfig.Plugins = map[string]plugin.Plugin{
					kind: arkplugin.NewBackupItemActionPlugin(action),
				}
			case arkplugin.PluginKindRestoreItemAction.String():
				action, found := restoreItemActions[name]
				if !found {
					logger.Fatalf("Unrecognized plugin name")
				}

				serveConfig.Plugins = map[string]plugin.Plugin{
					kind: arkplugin.NewRestoreItemActionPlugin(action),
				}
			default:
				logger.Fatalf("Unsupported plugin kind")
			}

			plugin.Serve(serveConfig)
		},
	}

	return c
}
