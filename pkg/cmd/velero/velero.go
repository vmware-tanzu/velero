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

package velero

import (
	"flag"

	"github.com/spf13/cobra"

	"github.com/heptio/velero/pkg/client"
	"github.com/heptio/velero/pkg/cmd/cli/backup"
	"github.com/heptio/velero/pkg/cmd/cli/backuplocation"
	"github.com/heptio/velero/pkg/cmd/cli/bug"
	cliclient "github.com/heptio/velero/pkg/cmd/cli/client"
	"github.com/heptio/velero/pkg/cmd/cli/completion"
	"github.com/heptio/velero/pkg/cmd/cli/create"
	"github.com/heptio/velero/pkg/cmd/cli/delete"
	"github.com/heptio/velero/pkg/cmd/cli/describe"
	"github.com/heptio/velero/pkg/cmd/cli/get"
	"github.com/heptio/velero/pkg/cmd/cli/plugin"
	"github.com/heptio/velero/pkg/cmd/cli/restic"
	"github.com/heptio/velero/pkg/cmd/cli/restore"
	"github.com/heptio/velero/pkg/cmd/cli/schedule"
	"github.com/heptio/velero/pkg/cmd/cli/snapshotlocation"
	"github.com/heptio/velero/pkg/cmd/server"
	runplugin "github.com/heptio/velero/pkg/cmd/server/plugin"
	"github.com/heptio/velero/pkg/cmd/version"
)

func NewCommand(name string) *cobra.Command {
	c := &cobra.Command{
		Use:   name,
		Short: "Back up and restore Kubernetes cluster resources.",
		Long: `Velero is a tool for managing disaster recovery, specifically for Kubernetes
cluster resources. It provides a simple, configurable, and operationally robust
way to back up your application state and associated data.

If you're familiar with kubectl, Velero supports a similar model, allowing you to
execute commands such as 'velero get backup' and 'velero create schedule'. The same
operations can also be performed as 'velero backup get' and 'velero schedule create'.`,
	}

	f := client.NewFactory(name)
	f.BindFlags(c.PersistentFlags())

	c.AddCommand(
		backup.NewCommand(f),
		schedule.NewCommand(f),
		restore.NewCommand(f),
		server.NewCommand(),
		version.NewCommand(f),
		get.NewCommand(f),
		describe.NewCommand(f),
		create.NewCommand(f),
		runplugin.NewCommand(f),
		plugin.NewCommand(f),
		delete.NewCommand(f),
		cliclient.NewCommand(),
		completion.NewCommand(),
		restic.NewCommand(f),
		bug.NewCommand(),
		backuplocation.NewCommand(f),
		snapshotlocation.NewCommand(f),
	)

	// add the glog flags
	c.PersistentFlags().AddGoFlagSet(flag.CommandLine)

	// TODO: switch to a different logging library.
	// Work around https://github.com/golang/glog/pull/13.
	// See also https://github.com/kubernetes/kubernetes/issues/17162
	flag.CommandLine.Parse([]string{})

	return c
}
